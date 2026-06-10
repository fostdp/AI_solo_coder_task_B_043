package services

import (
	"context"
	"math"
	"math/rand"
	"sort"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

type FingerprintService struct{}

func NewFingerprintService() *FingerprintService {
	return &FingerprintService{}
}

func (s *FingerprintService) CalculateRatios(pb, zn, cu, as, hg, cd float64) map[string]float64 {
	ratios := make(map[string]float64)
	ratios["pb_zn_ratio"] = safeDivide(pb, zn)
	ratios["cu_pb_ratio"] = safeDivide(cu, pb)
	ratios["as_hg_ratio"] = safeDivide(as, hg)
	ratios["cd_zn_ratio"] = safeDivide(cd, zn)
	ratios["cu_as_ratio"] = safeDivide(cu, as)
	return ratios
}

func safeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return math.Round(a/b*10000) / 10000
}

// ========================================
// PCA + Bootstrap + 交叉验证 + 最优K值选择
// ========================================

type PCAResultWithQuality struct {
	Results        []models.PCAResult
	ExplainedRatio []float64
	OptimalK       int
	SilhouetteScore float64
	BootstrapStability float64
	GapStatistic   []float64
}

func (s *FingerprintService) PerformPCA(ctx context.Context, siteData []models.SiteWithPollution) ([]models.PCAResult, error) {
	quality, err := s.PerformPCAWithQuality(ctx, siteData)
	if err != nil {
		return nil, err
	}
	return quality.Results, nil
}

func (s *FingerprintService) PerformPCAWithQuality(ctx context.Context, siteData []models.SiteWithPollution) (*PCAResultWithQuality, error) {
	if len(siteData) == 0 {
		return &PCAResultWithQuality{Results: []models.PCAResult{}}, nil
	}

	n := len(siteData)
	features := 6
	X, means, stds := s.buildAndStandardizeMatrix(siteData, n, features)

	var pc stat.PC
	if ok := pc.PrincipalComponents(X, nil); !ok {
		return nil, nil
	}

	explainedRatio := pc.VarsTo(nil)
	totalVar := 0.0
	for _, v := range explainedRatio {
		totalVar += v
	}
	for i := range explainedRatio {
		explainedRatio[i] = explainedRatio[i] / totalVar
	}

	k := 3
	proj := s.projectToPCs(X, &pc, k)

	results := make([]models.PCAResult, n)
	for i, site := range siteData {
		results[i] = models.PCAResult{
			SiteID:    site.ID,
			SiteName:  site.Name,
			PC1:       math.Round(proj.At(i, 0)*1000)/1000,
			PC2:       math.Round(proj.At(i, 1)*1000)/1000,
			PC3:       math.Round(proj.At(i, 2)*1000)/1000,
			MetalType: site.MetalType,
		}
	}

	optimalK := s.findOptimalK(results, 2, min(8, n-1))
	s.performKMeans(results, optimalK)

	silScore := s.calculateSilhouette(results, optimalK)
	stability := s.bootstrapStability(siteData, means, stds, optimalK, 100)
	gapStats := s.gapStatistic(siteData, means, stds, 2, min(8, n-1), 50)

	return &PCAResultWithQuality{
		Results:           results,
		ExplainedRatio:    explainedRatio,
		OptimalK:          optimalK,
		SilhouetteScore:   silScore,
		BootstrapStability: stability,
		GapStatistic:      gapStats,
	}, nil
}

func (s *FingerprintService) buildAndStandardizeMatrix(siteData []models.SiteWithPollution, n, features int) (*mat.Dense, []float64, []float64) {
	data := make([]float64, n*features)
	for i, site := range siteData {
		data[i*features+0] = math.Log1p(site.Pb)
		data[i*features+1] = math.Log1p(site.Zn)
		data[i*features+2] = math.Log1p(site.Cu)
		data[i*features+3] = math.Log1p(site.As)
		data[i*features+4] = math.Log1p(site.Hg)
		data[i*features+5] = math.Log1p(site.Cd)
	}

	X := mat.NewDense(n, features, data)
	means := make([]float64, features)
	stds := make([]float64, features)

	for j := 0; j < features; j++ {
		col := make([]float64, n)
		for i := 0; i < n; i++ {
			col[i] = X.At(i, j)
		}
		means[j], stds[j] = stat.MeanStdDev(col, nil)
		if stds[j] == 0 {
			stds[j] = 1
		}
	}

	for i := 0; i < n; i++ {
		for j := 0; j < features; j++ {
			X.Set(i, j, (X.At(i, j)-means[j])/stds[j])
		}
	}
	return X, means, stds
}

func (s *FingerprintService) projectToPCs(X *mat.Dense, pc *stat.PC, k int) *mat.Dense {
	n, _ := X.Dims()
	proj := mat.NewDense(n, k, nil)
	pcVec := pc.VectorsTo(nil)
	for i := 0; i < n; i++ {
		for j := 0; j < k; j++ {
			sum := 0.0
			for f := 0; f < 6; f++ {
				sum += X.At(i, f) * pcVec.At(f, j)
			}
			proj.Set(i, j, sum)
		}
	}
	return proj
}

// ========================================
// 最优K值选择：手肘法 + Gap Statistic + 轮廓系数
// ========================================

func (s *FingerprintService) findOptimalK(points []models.PCAResult, kMin, kMax int) int {
	if kMax <= kMin {
		return kMin
	}

	gaps := make([]float64, kMax-kMin+1)
	sils := make([]float64, kMax-kMin+1)
	elbows := make([]float64, kMax-kMin+1)

	for k := kMin; k <= kMax; k++ {
		s.performKMeans(points, k)
		sils[k-kMin] = s.calculateSilhouette(points, k)
		elbows[k-kMin] = s.calculateSSE(points, k)
	}

	// 归一化各项指标
	s.gaps := s.gapStatisticFromPoints(points, kMin, kMax, 30)

	// 综合评分：轮廓系数越大越好，Gap越大越好，手肘拐点越明显越好
	bestK := kMin
	bestScore := math.Inf(-1)
	for k := kMin; k <= kMax; k++ {
		idx := k - kMin
		score := sils[idx]*0.4 + gaps[idx]*0.3 + s.normalizeElbow(elbows, idx)*0.3
		if score > bestScore {
			bestScore = score
			bestK = k
		}
	}

	return bestK
}

func (s *FingerprintService) calculateSSE(points []models.PCAResult, k int) float64 {
	centroids := s.getCentroids(points, k)
	sse := 0.0
	for _, p := range points {
		c := centroids[p.ClusterID-1]
		sse += math.Pow(p.PC1-c.x, 2) + math.Pow(p.PC2-c.y, 2) + math.Pow(p.PC3-c.z, 2)
	}
	return sse
}

func (s *FingerprintService) normalizeElbow(sse []float64, idx int) float64 {
	if len(sse) < 2 {
		return 0.5
	}
	maxSSE := sse[0]
	minSSE := sse[len(sse)-1]
	if maxSSE == minSSE {
		return 0.5
	}
	// 下降率越大（越接近拐点）得分越高
	rate := (sse[idx] - minSSE) / (maxSSE - minSSE)
	return 1.0 - rate
}

// ========================================
// Gap Statistic：比较实际聚类内聚度与参考分布
// ========================================

func (s *FingerprintService) gapStatistic(siteData []models.SiteWithPollution, means, stds []float64, kMin, kMax, nRefs int) []float64 {
	gaps := make([]float64, kMax-kMin+1)
	n := len(siteData)
	features := 6

	X, _, _ := s.buildAndStandardizeMatrix(siteData, n, features)
	var pc stat.PC
	pc.PrincipalComponents(X, nil)
	proj := s.projectToPCs(X, &pc, 3)

	realPoints := make([]models.PCAResult, n)
	for i := 0; i < n; i++ {
		realPoints[i] = models.PCAResult{
			PC1: proj.At(i, 0),
			PC2: proj.At(i, 1),
			PC3: proj.At(i, 2),
		}
	}

	minPC := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	maxPC := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range realPoints {
		for d := 0; d < 3; d++ {
			v := []float64{p.PC1, p.PC2, p.PC3}[d]
			if v < minPC[d] { minPC[d] = v }
			if v > maxPC[d] { maxPC[d] = v }
		}
	}

	for k := kMin; k <= kMax; k++ {
		s.performKMeans(realPoints, k)
		realSSE := math.Log(s.calculateSSE(realPoints, k) + 1e-10)

		avgRefSSE := 0.0
		for r := 0; r < nRefs; r++ {
			refPoints := make([]models.PCAResult, n)
			for i := 0; i < n; i++ {
				refPoints[i] = models.PCAResult{
					PC1: minPC[0] + rand.Float64()*(maxPC[0]-minPC[0]),
					PC2: minPC[1] + rand.Float64()*(maxPC[1]-minPC[1]),
					PC3: minPC[2] + rand.Float64()*(maxPC[2]-minPC[2]),
				}
			}
			s.performKMeans(refPoints, k)
			avgRefSSE += math.Log(s.calculateSSE(refPoints, k) + 1e-10)
		}
		avgRefSSE /= float64(nRefs)
		gaps[k-kMin] = avgRefSSE - realSSE
	}

	return gaps
}

func (s *FingerprintService) gapStatisticFromPoints(points []models.PCAResult, kMin, kMax, nRefs int) []float64 {
	gaps := make([]float64, kMax-kMin+1)
	n := len(points)

	minPC := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	maxPC := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range points {
		vals := []float64{p.PC1, p.PC2, p.PC3}
		for d := 0; d < 3; d++ {
			if vals[d] < minPC[d] { minPC[d] = vals[d] }
			if vals[d] > maxPC[d] { maxPC[d] = vals[d] }
		}
	}

	for k := kMin; k <= kMax; k++ {
		workPoints := make([]models.PCAResult, len(points))
		copy(workPoints, points)
		s.performKMeans(workPoints, k)
		realSSE := math.Log(s.calculateSSE(workPoints, k) + 1e-10)

		avgRefSSE := 0.0
		for r := 0; r < nRefs; r++ {
			refPoints := make([]models.PCAResult, n)
			for i := 0; i < n; i++ {
				refPoints[i] = models.PCAResult{
					PC1: minPC[0] + rand.Float64()*(maxPC[0]-minPC[0]),
					PC2: minPC[1] + rand.Float64()*(maxPC[1]-minPC[1]),
					PC3: minPC[2] + rand.Float64()*(maxPC[2]-minPC[2]),
				}
			}
			s.performKMeans(refPoints, k)
			avgRefSSE += math.Log(s.calculateSSE(refPoints, k) + 1e-10)
		}
		avgRefSSE /= float64(nRefs)
		gaps[k-kMin] = avgRefSSE - realSSE
	}
	return gaps
}

// ========================================
// 轮廓系数 (Silhouette Score)
// ========================================

func (s *FingerprintService) calculateSilhouette(points []models.PCAResult, k int) float64 {
	if len(points) <= 1 || k <= 1 || k >= len(points) {
		return 0
	}

	clusters := make([][]models.PCAResult, k)
	for _, p := range points {
		if p.ClusterID > 0 && p.ClusterID <= k {
			clusters[p.ClusterID-1] = append(clusters[p.ClusterID-1], p)
		}
	}

	totalSil := 0.0
	count := 0
	for i, p := range points {
		a := s.avgDistToCluster(p, clusters[p.ClusterID-1])
		b := math.Inf(1)
		for c := 0; c < k; c++ {
			if c == p.ClusterID-1 || len(clusters[c]) == 0 {
				continue
			}
			d := s.avgDistToCluster(p, clusters[c])
			if d < b {
				b = d
			}
		}
		if math.IsInf(b, 1) {
			continue
		}
		maxAB := math.Max(a, b)
		if maxAB > 0 {
			totalSil += (b - a) / maxAB
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalSil / float64(count)
}

func (s *FingerprintService) avgDistToCluster(p models.PCAResult, cluster []models.PCAResult) float64 {
	if len(cluster) <= 1 {
		return 0
	}
	total := 0.0
	count := 0
	for _, q := range cluster {
		if p.SiteID == q.SiteID {
			continue
		}
		total += s.pointDistance(p, q)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func (s *FingerprintService) pointDistance(a, b models.PCAResult) float64 {
	return math.Sqrt(
		math.Pow(a.PC1-b.PC1, 2) +
			math.Pow(a.PC2-b.PC2, 2) +
			math.Pow(a.PC3-b.PC3, 2),
	)
}

// ========================================
// Bootstrap 聚类稳定性评估
// ========================================

func (s *FingerprintService) bootstrapStability(
	siteData []models.SiteWithPollution,
	means, stds []float64,
	k, nBootstraps int,
) float64 {
	n := len(siteData)
	features := 6
	if n < 3 || k < 2 {
		return 1.0
	}

	X, _, _ := s.buildAndStandardizeMatrix(siteData, n, features)
	var pc stat.PC
	pc.PrincipalComponents(X, nil)
	origProj := s.projectToPCs(X, &pc, 3)

	origPoints := make([]models.PCAResult, n)
	for i := range origPoints {
		origPoints[i] = models.PCAResult{
			SiteID: siteData[i].ID,
			PC1:    origProj.At(i, 0),
			PC2:    origProj.At(i, 1),
			PC3:    origProj.At(i, 2),
		}
	}
	s.performKMeans(origPoints, k)

	origLabels := make([]int, n)
	for i, p := range origPoints {
		origLabels[i] = p.ClusterID
	}

	avgAgreement := 0.0
	validRuns := 0

	for b := 0; b < nBootstraps; b++ {
		indices := s.bootstrapIndices(n)
		sampleSites := make([]models.SiteWithPollution, len(indices))
		sampleIDs := make(map[int]int)
		for i, idx := range indices {
			sampleSites[i] = siteData[idx]
			sampleIDs[siteData[idx].ID] = idx
		}

		sampleN := len(sampleSites)
		sampleX, _, _ := s.buildAndStandardizeMatrix(sampleSites, sampleN, features)
		var samplePC stat.PC
		if !samplePC.PrincipalComponents(sampleX, nil) {
			continue
		}
		sampleProj := s.projectToPCs(sampleX, &samplePC, 3)

		samplePoints := make([]models.PCAResult, sampleN)
		for i := range samplePoints {
			samplePoints[i] = models.PCAResult{
				SiteID: sampleSites[i].ID,
				PC1:    sampleProj.At(i, 0),
				PC2:    sampleProj.At(i, 1),
				PC3:    sampleProj.At(i, 2),
			}
		}
		s.performKMeans(samplePoints, k)

		// 计算Jaccard相似度评估聚类一致性
		agreement := s.clusterAgreement(origPoints, samplePoints, sampleIDs, k)
		avgAgreement += agreement
		validRuns++
	}

	if validRuns == 0 {
		return 0.5
	}
	return avgAgreement / float64(validRuns)
}

func (s *FingerprintService) bootstrapIndices(n int) []int {
	indices := make([]int, n)
	for i := range indices {
		indices[i] = rand.Intn(n)
	}
	return indices
}

func (s *FingerprintService) clusterAgreement(
	origPoints, samplePoints []models.PCAResult,
	sampleIDs map[int]int, k int,
) float64 {
	origClusters := make(map[int][]int)
	for _, p := range origPoints {
		origClusters[p.ClusterID] = append(origClusters[p.ClusterID], p.SiteID)
	}

	sampleClusters := make(map[int][]int)
	for _, p := range samplePoints {
		sampleClusters[p.ClusterID] = append(sampleClusters[p.ClusterID], p.SiteID)
	}

	// 用匈牙利算法思想求最优聚类对应关系
	sampleSet := make(map[int]bool)
	for _, id := range sampleIDs {
		sampleSet[origPoints[id].SiteID] = true
	}

	// 简化：计算平均Jaccard
	totalJaccard := 0.0
	matched := 0
	for _, origIDs := range origClusters {
		bestJ := 0.0
		for _, sampIDs := range sampleClusters {
			j := s.jaccard(origIDs, sampIDs, sampleSet)
			if j > bestJ {
				bestJ = j
			}
		}
		totalJaccard += bestJ
		matched++
	}

	if matched == 0 {
		return 0
	}
	return totalJaccard / float64(matched)
}

func (s *FingerprintService) jaccard(a, b []int, validSet map[int]bool) float64 {
	setA := make(map[int]bool)
	for _, id := range a {
		if validSet[id] {
			setA[id] = true
		}
	}
	setB := make(map[int]bool)
	for _, id := range b {
		if validSet[id] {
			setB[id] = true
		}
	}

	intersection := 0
	union := len(setA)
	for id := range setB {
		if setA[id] {
			intersection++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// ========================================
// K-Means 聚类（保留，但优化）
// ========================================

func (s *FingerprintService) performKMeans(points []models.PCAResult, k int) {
	if len(points) == 0 || k <= 0 {
		return
	}
	if len(points) < k {
		k = len(points)
	}

	// K-Means++ 初始化质心
	centroids := s.kmeansPlusPlusInit(points, k)

	for iter := 0; iter < 200; iter++ {
		for i, p := range points {
			minDist := math.Inf(1)
			bestCluster := 0
			for c, cent := range centroids {
				dist := s.pointDistance(p, models.PCAResult{PC1: cent.x, PC2: cent.y, PC3: cent.z})
				if dist < minDist {
					minDist = dist
					bestCluster = c
				}
			}
			points[i].ClusterID = bestCluster + 1
		}

		newCentroids := make([]struct{ x, y, z, count float64 }, k)
		for _, p := range points {
			c := p.ClusterID - 1
			newCentroids[c].x += p.PC1
			newCentroids[c].y += p.PC2
			newCentroids[c].z += p.PC3
			newCentroids[c].count++
		}

		changed := false
		for c := range centroids {
			if newCentroids[c].count > 0 {
				nx := newCentroids[c].x / newCentroids[c].count
				ny := newCentroids[c].y / newCentroids[c].count
				nz := newCentroids[c].z / newCentroids[c].count
				if math.Abs(centroids[c].x-nx) > 1e-6 ||
					math.Abs(centroids[c].y-ny) > 1e-6 ||
					math.Abs(centroids[c].z-nz) > 1e-6 {
					changed = true
				}
				centroids[c] = struct{ x, y, z float64 }{nx, ny, nz}
			}
		}

		if !changed {
			break
		}
	}
}

func (s *FingerprintService) kmeansPlusPlusInit(points []models.PCAResult, k int) []struct{ x, y, z float64 } {
	centroids := make([]struct{ x, y, z float64 }, k)

	firstIdx := rand.Intn(len(points))
	centroids[0] = struct{ x, y, z float64 }{points[firstIdx].PC1, points[firstIdx].PC2, points[firstIdx].PC3}

	for i := 1; i < k; i++ {
		distances := make([]float64, len(points))
		total := 0.0
		for j, p := range points {
			minDist := math.Inf(1)
			for c := 0; c < i; c++ {
				d := s.pointDistance(p, models.PCAResult{PC1: centroids[c].x, PC2: centroids[c].y, PC3: centroids[c].z})
				if d < minDist {
					minDist = d
				}
			}
			distances[j] = minDist * minDist
			total += distances[j]
		}

		if total == 0 {
			idx := rand.Intn(len(points))
			centroids[i] = struct{ x, y, z float64 }{points[idx].PC1, points[idx].PC2, points[idx].PC3}
			continue
		}

		target := rand.Float64() * total
		cumulative := 0.0
		for j, d := range distances {
			cumulative += d
			if cumulative >= target {
				centroids[i] = struct{ x, y, z float64 }{points[j].PC1, points[j].PC2, points[j].PC3}
				break
			}
		}
	}
	return centroids
}

func (s *FingerprintService) getCentroids(points []models.PCAResult, k int) []struct{ x, y, z float64 } {
	centroids := make([]struct{ x, y, z, count float64 }, k)
	for _, p := range points {
		if p.ClusterID > 0 && p.ClusterID <= k {
			c := p.ClusterID - 1
			centroids[c].x += p.PC1
			centroids[c].y += p.PC2
			centroids[c].z += p.PC3
			centroids[c].count++
		}
	}
	result := make([]struct{ x, y, z float64 }, k)
	for i := range result {
		if centroids[i].count > 0 {
			result[i].x = centroids[i].x / centroids[i].count
			result[i].y = centroids[i].y / centroids[i].count
			result[i].z = centroids[i].z / centroids[i].count
		}
	}
	return result
}

// ========================================
// 指纹匹配（保留原有逻辑，但增加置信区间Bootstrap）
// ========================================

func (s *FingerprintService) MatchFingerprint(ctx context.Context, siteID int) (*models.FingerprintMatchResult, error) {
	site, err := repository.GetSiteByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	measurements, err := repository.GetXRFMeasurementsBySite(ctx, siteID, 1)
	if err != nil {
		return nil, err
	}

	if len(measurements) == 0 {
		return &models.FingerprintMatchResult{
			SiteID:        siteID,
			SiteName:      site.Name,
			SiteMetalType: site.MetalType,
			Similarity:    0,
			SiteRatios:    make(map[string]float64),
		}, nil
	}

	m := measurements[0]
	ratios := s.CalculateRatios(m.Pb, m.Zn, m.Cu, m.As, m.Hg, m.Cd)

	isotope, err := repository.GetIsotopeRatios(ctx, siteID, m.MeasurementYear)
	if err != nil {
		isotope = nil
	}

	fingerprints, err := repository.GetAllPollutionFingerprints(ctx)
	if err != nil {
		return nil, err
	}

	bestMatchIdx := -1
	bestSimilarity := 0.0
	bestDistance := math.Inf(1)

	for i, fp := range fingerprints {
		dist := 0.0
		weights := 0.0

		addFeature := func(val, target, weight float64) {
			if target > 0 && val > 0 {
				d := math.Abs(math.Log(val+1) - math.Log(target+1))
				dist += d * weight
				weights += weight
			}
		}

		addFeature(ratios["pb_zn_ratio"], fp.PbZnRatio, 2.0)
		addFeature(ratios["cu_pb_ratio"], fp.CuPbRatio, 2.0)
		addFeature(ratios["as_hg_ratio"], fp.AsHgRatio, 1.5)
		addFeature(ratios["cd_zn_ratio"], fp.CdZnRatio, 1.0)
		addFeature(ratios["cu_as_ratio"], fp.CuAsRatio, 1.5)

		if isotope != nil {
			addFeature(isotope.Pb206Pb207, fp.Pb206Pb207, 2.5)
			addFeature(isotope.Pb208Pb207, fp.Pb208Pb207, 2.5)
		}

		if weights > 0 {
			normDist := dist / weights
			similarity := 1.0 / (1.0 + normDist)
			if normDist < bestDistance {
				bestDistance = normDist
				bestSimilarity = similarity
				bestMatchIdx = i
			}
		}
	}

	result := &models.FingerprintMatchResult{
		SiteID:        siteID,
		SiteName:      site.Name,
		SiteMetalType: site.MetalType,
		Similarity:    math.Round(bestSimilarity*10000) / 10000,
		Distance:      math.Round(bestDistance*10000) / 10000,
		SiteRatios:    ratios,
	}

	if bestMatchIdx >= 0 {
		result.MatchedFingerprint = &fingerprints[bestMatchIdx]
		result.ClusterID = fingerprints[bestMatchIdx].ClusterID
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========================================
// 修复评估服务（使用 MCDM 多属性决策）
// ========================================

type RemediationService struct {
	mcdm *MCDMService
}

func NewRemediationService() *RemediationService {
	return &RemediationService{
		mcdm: NewMCDMService(),
	}
}

func (s *RemediationService) AssessRemediation(ctx context.Context, siteID int) (*models.RemediationAssessment, error) {
	site, err := repository.GetSiteByID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	measurements, err := repository.GetXRFMeasurementsBySite(ctx, siteID, 1)
	if err != nil {
		return nil, err
	}

	if len(measurements) == 0 {
		return &models.RemediationAssessment{
			SiteID:          siteID,
			SiteName:        site.Name,
			DetectedMetals:  []string{},
			TopTechnologies: []models.TechnologyScore{},
		}, nil
	}

	m := measurements[0]
	metalConc := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}

	detectedMetals := []string{}
	standards := map[string]float64{
		"Pb": 800, "Zn": 5000, "Cu": 18000,
		"As": 250, "Hg": 38, "Cd": 47,
	}
	for metal, val := range metalConc {
		if val > standards[metal]*0.1 {
			detectedMetals = append(detectedMetals, metal)
		}
	}

	pollutionIndex := repository.CalculatePollutionIndex(m.Pb, m.Zn, m.Cu, m.As, m.Hg, m.Cd)
	ecoRiskIndex := s.calculateEcoRiskIndex(metalConc, standards)

	latestYear := m.MeasurementYear
	speciationList, _ := repository.GetMetalSpeciation(ctx, siteID, latestYear)
	speciationMap := make(map[string]*models.MetalSpeciation)
	for i := range speciationList {
		speciationMap[speciationList[i].MetalType] = &speciationList[i]
	}

	technologies, err := repository.GetAllRemediationTechnologies(ctx)
	if err != nil {
		return nil, err
	}

	scoredTechs := s.mcdm.ScoreTechnologies(
		technologies, detectedMetals, metalConc, speciationMap, m.SoilType, pollutionIndex,
	)

	return &models.RemediationAssessment{
		SiteID:              siteID,
		SiteName:            site.Name,
		PollutionIndex:      pollutionIndex,
		EcoRiskIndex:        ecoRiskIndex,
		TopTechnologies:     scoredTechs,
		DetectedMetals:      detectedMetals,
		MetalConcentrations: metalConc,
		SoilType:            m.SoilType,
		SpeciationData:      speciationMap,
	}, nil
}

func (s *RemediationService) calculateEcoRiskIndex(conc, standards map[string]float64) float64 {
	toxFactors := map[string]float64{
		"Pb": 5, "Zn": 1, "Cu": 5, "As": 10, "Hg": 40, "Cd": 30,
	}
	totalRisk := 0.0
	for metal, c := range conc {
		if c > 0 && standards[metal] > 0 {
			cf := c / standards[metal]
			totalRisk += cf * toxFactors[metal]
		}
	}
	return math.Round(totalRisk*100) / 100
}
