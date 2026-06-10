package modules

import (
	"context"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// ========================================
// FingerprintAnalyzer - PCA聚类 + 指纹识别模块
// 职责：主成分分析、K-Means++聚类、质量评估、指纹库匹配
// 订阅：EventXRFReceived
// 发布：EventFingerprintReady
// ========================================

type FingerprintAnalyzer struct {
	bus      *EventBus
	cfg      config.PCAConfig
	fpCfg    config.FingerprintConfig
	running  bool
}

func NewFingerprintAnalyzer() *FingerprintAnalyzer {
	fa := &FingerprintAnalyzer{
		bus:   GetEventBus(),
		cfg:   config.DefaultPCAConfig,
		fpCfg: config.DefaultFingerprintConfig,
	}
	go fa.start()
	return fa
}

func (fa *FingerprintAnalyzer) start() {
	fa.running = true
	log.Println("[FingerprintAnalyzer] Module started")

	ch := fa.bus.Subscribe(EventXRFReceived)
	for event := range ch {
		if !fa.running {
			return
		}
		payload, ok := event.Payload.(XRFReceivedPayload)
		if !ok {
			continue
		}
		go fa.handleXRFReceived(event.Context, payload)
	}
}

func (fa *FingerprintAnalyzer) handleXRFReceived(ctx context.Context, payload XRFReceivedPayload) {
	m := payload.Measurement
	siteData, err := repository.GetAllXRFMeasurements(ctx)
	if err != nil {
		log.Printf("[FingerprintAnalyzer] Failed to get measurements: %v", err)
		return
	}
	result, err := fa.MatchFingerprint(ctx, m.SiteID, siteData, &m)
	fa.bus.Publish(Event{
		Type:    EventFingerprintReady,
		Payload: FingerprintPayload{SiteID: m.SiteID, Result: result, Err: err},
		Context: ctx,
	})
}

// ============== 公开接口 ==============

// PerformPCAWithQuality 执行PCA并返回质量评估
func (fa *FingerprintAnalyzer) PerformPCAWithQuality(siteData []models.XRFMeasurement) *models.PCAResultWithQuality {
	if len(siteData) < 2 {
		return nil
	}
	samples := make([]models.XRFMeasurement, len(siteData))
	copy(samples, siteData)

	n := len(samples)
	means := make([]float64, 6)
	stds := make([]float64, 6)
	data := fa.buildAndStandardizeMatrix(samples, means, stds)

	pc := fa.performPCA(data)
	nComp := fa.cfg.NumComponents
	if pc.Vecs == nil {
		nComp = 0
	}
	_ = nComp
	projected := fa.projectToPCs(data, pc, fa.cfg.NumComponents, means, stds)

	var pcaResult models.PCAResultWithQuality
	pcaResult.Projections = make([][]float64, n)
	for i := 0; i < n; i++ {
		pcaResult.Projections[i] = projected[i]
	}
	if pc.Vals != nil && len(pc.Vals) > 0 {
		totalVar := 0.0
		for _, v := range pc.Vals {
			totalVar += v
		}
		pcaResult.ExplainedVariance = make([]float64, len(pc.Vals))
		for i, v := range pc.Vals {
			if totalVar > 0 {
				pcaResult.ExplainedVariance[i] = v / totalVar
			}
		}
		cumulative := 0.0
		pcaResult.CumulativeVariance = make([]float64, len(pc.Vals))
		for i := range pc.Vals {
			cumulative += pcaResult.ExplainedVariance[i]
			pcaResult.CumulativeVariance[i] = cumulative
		}
	}

	optimalK, gaps, silhouettes, sse := fa.findOptimalK(projected, fa.cfg.KMin, fa.cfg.KMax)
	_ = gaps
	_ = silhouettes
	_ = sse
	labels, _, sseVal := fa.performKMeans(projected, optimalK, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)
	_ = sseVal
	pcaResult.Labels = labels
	pcaResult.K = optimalK
	pcaResult.SilhouetteScore = fa.calculateSilhouette(projected, labels, optimalK)
	pcaResult.BootstrapStability = fa.bootstrapStability(samples, means, stds, optimalK, fa.cfg.NumBootstraps)
	pcaResult.GapStatistic, _ = fa.gapStatistic(samples, means, stds, optimalK, optimalK, fa.cfg.NumGapReferences)

	return &pcaResult
}

// MatchFingerprint 匹配污染指纹
func (fa *FingerprintAnalyzer) MatchFingerprint(ctx context.Context, siteID int,
	siteData []models.XRFMeasurement, currentMeasurement *models.XRFMeasurement) (*models.FingerprintMatchResult, error) {

	siteRatios := fa.CalculateRatios(currentMeasurement)
	if currentMeasurement == nil && len(siteData) > 0 {
		siteRatios = fa.CalculateRatios(&siteData[len(siteData)-1])
	}
	isotopes, err := repository.GetLatestIsotopeRatio(ctx, siteID)
	if err != nil {
		isotopes = nil
	}

	fingerprints, err := repository.GetAllFingerprints(ctx)
	if err != nil {
		return nil, err
	}

	result := &models.FingerprintMatchResult{
		Matches: make([]models.Fingerprint, 0),
	}

	matches := make([]struct {
		fp   models.PollutionFingerprint
		dist float64
		sim  float64
	}, 0, len(fingerprints))

	for _, fp := range fingerprints {
		dist := fa.computeFingerprintDistance(siteRatios, isotopes, &fp)
		sim := fa.computeFingerprintSimilarity(dist)
		matches = append(matches, struct {
			fp   models.PollutionFingerprint
			dist float64
			sim  float64
		}{fp, dist, sim})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].sim > matches[j].sim
	})

	if len(matches) > 0 {
		result.BestMatch = models.Fingerprint{
			FingerprintID: matches[0].fp.ID,
			MetalType:     matches[0].fp.MetalType,
			ProcessType:   matches[0].fp.ProcessType,
			Region:        matches[0].fp.Region,
			Description:   matches[0].fp.Description,
			Similarity:    matches[0].sim,
			Distance:      matches[0].dist,
			Ratios: map[string]float64{
				"pb_zn_ratio": matches[0].fp.PbZnRatio,
				"cu_pb_ratio": matches[0].fp.CuPbRatio,
				"as_hg_ratio": matches[0].fp.AsHgRatio,
				"cd_zn_ratio": matches[0].fp.CdZnRatio,
				"cu_as_ratio": matches[0].fp.CuAsRatio,
			},
			PCAProjection: []float64{matches[0].fp.PC1, matches[0].fp.PC2, matches[0].fp.PC3},
			ClusterID:     matches[0].fp.ClusterID,
		}
		result.Similarity = matches[0].sim
		result.Distance = matches[0].dist

		for i := 0; i < len(matches) && i < 5; i++ {
			result.Matches = append(result.Matches, models.Fingerprint{
				FingerprintID: matches[i].fp.ID,
				MetalType:     matches[i].fp.MetalType,
				ProcessType:   matches[i].fp.ProcessType,
				Region:        matches[i].fp.Region,
				Description:   matches[i].fp.Description,
				Similarity:    matches[i].sim,
				Distance:      matches[i].dist,
				Ratios: map[string]float64{
					"pb_zn_ratio": matches[i].fp.PbZnRatio,
					"cu_pb_ratio": matches[i].fp.CuPbRatio,
				},
				ClusterID: matches[i].fp.ClusterID,
			})
		}
	}
	return result, nil
}

// CalculateRatios 计算5种重金属比率
func (fa *FingerprintAnalyzer) CalculateRatios(m *models.XRFMeasurement) map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}
	r := map[string]float64{}
	r["pb_zn_ratio"] = safeDiv(m.Pb, m.Zn)
	r["cu_pb_ratio"] = safeDiv(m.Cu, m.Pb)
	r["as_hg_ratio"] = safeDiv(m.As, m.Hg)
	r["cd_zn_ratio"] = safeDiv(m.Cd, m.Zn)
	r["cu_as_ratio"] = safeDiv(m.Cu, m.As)
	return r
}

// ============== PCA + 聚类核心 ==============

func (fa *FingerprintAnalyzer) buildAndStandardizeMatrix(data []models.XRFMeasurement, means, stds []float64) *mat.Dense {
	n := len(data)
	p := 6
	raw := mat.NewDense(n, p, nil)
	for i, m := range data {
		raw.Set(i, 0, math.Log1p(m.Pb))
		raw.Set(i, 1, math.Log1p(m.Zn))
		raw.Set(i, 2, math.Log1p(m.Cu))
		raw.Set(i, 3, math.Log1p(m.As))
		raw.Set(i, 4, math.Log1p(m.Hg))
		raw.Set(i, 5, math.Log1p(m.Cd))
	}
	for j := 0; j < p; j++ {
		sum := 0.0
		for i := 0; i < n; i++ {
			sum += raw.At(i, j)
		}
		means[j] = sum / float64(n)
	}
	for j := 0; j < p; j++ {
		var sq float64
		for i := 0; i < n; i++ {
			d := raw.At(i, j) - means[j]
			sq += d * d
		}
		v := sq / float64(n-1)
		if v < 1e-12 {
			v = 1
		}
		stds[j] = math.Sqrt(v)
	}
	stdData := mat.NewDense(n, p, nil)
	for i := 0; i < n; i++ {
		for j := 0; j < p; j++ {
			stdData.Set(i, j, (raw.At(i, j)-means[j])/stds[j])
		}
	}
	return stdData
}

func (fa *FingerprintAnalyzer) performPCA(data *mat.Dense) stat.PC {
	n, _ := data.Dims()
	var pc stat.PC
	if n >= 2 {
		pc = stat.PC{}
		ok := pc.PrincipalComponents(data, nil)
		if !ok {
			rows, cols := data.Dims()
			_ = rows
			pc.Vecs = mat.NewDense(cols, cols, nil)
			pc.Vals = make([]float64, cols)
			for i := 0; i < cols; i++ {
				pc.Vecs.Set(i, i, 1)
				pc.Vals[i] = 1.0
			}
		}
	}
	return pc
}

func (fa *FingerprintAnalyzer) projectToPCs(data *mat.Dense, pc stat.PC, k int, means, stds []float64) [][]float64 {
	n, p := data.Dims()
	if k > p {
		k = p
	}
	result := make([][]float64, n)
	for i := 0; i < n; i++ {
		result[i] = make([]float64, k)
		for j := 0; j < k; j++ {
			s := 0.0
			for d := 0; d < p; d++ {
				s += data.At(i, d) * pc.Vecs.At(d, j)
			}
			result[i][j] = s
		}
	}
	return result
}

func (fa *FingerprintAnalyzer) performKMeans(points [][]float64, k int, maxIter int, eps float64) ([]int, [][]float64, float64) {
	n := len(points)
	labels := make([]int, n)
	centroids := fa.kmeansPlusPlusInit(points, k)
	var prevCentroids [][]float64

	for iter := 0; iter < maxIter; iter++ {
		for i := 0; i < n; i++ {
			best := 0
			bestD := math.Inf(1)
			for c := 0; c < k; c++ {
				d := euclidean(points[i], centroids[c])
				if d < bestD {
					bestD = d
					best = c
				}
			}
			labels[i] = best
		}
		prevCentroids = make([][]float64, k)
		for c := 0; c < k; c++ {
			prevCentroids[c] = make([]float64, len(centroids[0]))
			copy(prevCentroids[c], centroids[c])
		}
		sums := make([][]float64, k)
		counts := make([]int, k)
		for c := range sums {
			sums[c] = make([]float64, len(centroids[0]))
		}
		for i, pt := range points {
			c := labels[i]
			counts[c]++
			for d := range pt {
				sums[c][d] += pt[d]
			}
		}
		for c := 0; c < k; c++ {
			if counts[c] > 0 {
				for d := range centroids[c] {
					centroids[c][d] = sums[c][d] / float64(counts[c])
				}
			}
		}
		if fa.centroidsConverged(prevCentroids, centroids, eps) {
			break
		}
	}

	var sse float64
	for i, pt := range points {
		d := euclidean(pt, centroids[labels[i]])
		sse += d * d
	}
	return labels, centroids, sse
}

func (fa *FingerprintAnalyzer) kmeansPlusPlusInit(points [][]float64, k int) [][]float64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := len(points)
	if n == 0 || k == 0 {
		return nil
	}
	centroids := make([][]float64, k)
	firstIdx := r.Intn(n)
	centroids[0] = make([]float64, len(points[0]))
	copy(centroids[0], points[firstIdx])

	distances := make([]float64, n)
	for i := 1; i < k; i++ {
		for j, pt := range points {
			minD := math.Inf(1)
			for c := 0; c < i; c++ {
				d := euclidean(pt, centroids[c])
				if d < minD {
					minD = d
				}
			}
			distances[j] = minD * minD
		}
		total := 0.0
		for _, d := range distances {
			total += d
		}
		if total == 0 {
			centroids[i] = make([]float64, len(points[0]))
			copy(centroids[i], points[r.Intn(n)])
			continue
		}
		target := r.Float64() * total
		cumulative := 0.0
		selected := 0
		for idx, d := range distances {
			cumulative += d
			if cumulative >= target {
				selected = idx
				break
			}
		}
		centroids[i] = make([]float64, len(points[0]))
		copy(centroids[i], points[selected])
	}
	return centroids
}

func (fa *FingerprintAnalyzer) centroidsConverged(prev, curr [][]float64, eps float64) bool {
	for i := range prev {
		if euclidean(prev[i], curr[i]) > eps {
			return false
		}
	}
	return true
}

// ============== 质量评估 ==============

func (fa *FingerprintAnalyzer) findOptimalK(points [][]float64, kMin, kMax int) (int, []float64, []float64, []float64) {
	n := len(points)
	if kMax > n {
		kMax = n
	}
	if kMin > kMax {
		return kMin, nil, nil, nil
	}

	sses := make([]float64, kMax+1)
	silhouettes := make([]float64, kMax+1)

	sseRates := make([]float64, kMax+1)

	for k := kMin; k <= kMax; k++ {
		labels, _, sse := fa.performKMeans(points, k, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)
		sses[k] = sse
		silhouettes[k] = fa.calculateSilhouette(points, labels, k)
	}
	for k := kMin + 1; k <= kMax; k++ {
		if sses[k-1] > 0 {
			sseRates[k] = (sses[k-1] - sses[k]) / sses[k-1]
		}
	}

	ratesNorm := fa.normalize(sseRates[kMin : kMax+1])
	silNorm := fa.normalize(silhouettes[kMin : kMax+1])

	bestIdx := 0
	bestScore := -1.0
	for i := kMin; i <= kMax; i++ {
		idx := i - kMin
		score := fa.cfg.ElbowWeight*ratesNorm[idx] +
			fa.cfg.SilhouetteWeight*silNorm[idx]
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx, sses, silhouettes, sses
}

func (fa *FingerprintAnalyzer) calculateSilhouette(points [][]float64, labels []int, k int) float64 {
	n := len(points)
	if n <= 1 || k <= 1 || k >= n {
		return 0
	}
	var total float64
	for i := 0; i < n; i++ {
		aCount := 0
		aSum := 0.0
		bMin := math.Inf(1)
		for c := 0; c < k; c++ {
			if c == labels[i] {
				for j := 0; j < n; j++ {
					if j != i && labels[j] == c {
						aSum += euclidean(points[i], points[j])
						aCount++
					}
				}
			} else {
				bCount := 0
				bSum := 0.0
				for j := 0; j < n; j++ {
					if labels[j] == c {
						bSum += euclidean(points[i], points[j])
						bCount++
					}
				}
				if bCount > 0 {
					avg := bSum / float64(bCount)
					if avg < bMin {
						bMin = avg
					}
				}
			}
		}
		var a float64
		if aCount > 0 {
			a = aSum / float64(aCount)
		}
		s := 0.0
		if math.Max(a, bMin) > 0 {
			s = (bMin - a) / math.Max(a, bMin)
		}
		total += s
	}
	return total / float64(n)
}

func (fa *FingerprintAnalyzer) gapStatistic(siteData []models.XRFMeasurement, means, stds []float64, k, kMax, nRefs int) (float64, []float64) {
	n := len(siteData)
	data := fa.buildAndStandardizeMatrix(siteData, means, stds)
	pc := fa.performPCA(data)
	projected := fa.projectToPCs(data, pc, fa.cfg.NumComponents, means, stds)
	_, _, realSSE := fa.performKMeans(projected, k, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)
	_ = kMax

	_, p := data.Dims()
	r := rand.New(rand.NewSource(42))
	logWkRefs := make([]float64, nRefs)
	minVals := make([]float64, p)
	maxVals := make([]float64, p)
	for j := 0; j < p; j++ {
		minVals[j] = math.Inf(1)
		maxVals[j] = math.Inf(-1)
		for i := 0; i < n; i++ {
			v := data.At(i, j)
			if v < minVals[j] {
				minVals[j] = v
			}
			if v > maxVals[j] {
				maxVals[j] = v
			}
		}
	}
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	for ref := 0; ref < nRefs; ref++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			refData := mat.NewDense(n, p, nil)
			for i := 0; i < n; i++ {
				for j := 0; j < p; j++ {
					refData.Set(i, j, minVals[j]+r.Float64()*(maxVals[j]-minVals[j]))
				}
			}
			refPC := fa.performPCA(refData)
			refProj := fa.projectToPCs(refData, refPC, fa.cfg.NumComponents, means, stds)
			_, _, sse := fa.performKMeans(refProj, k, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)
			mu.Lock()
			logWkRefs[idx] = math.Log(sse + 1e-10)
			mu.Unlock()
		}(ref)
	}
	wg.Wait()

	avgRef := 0.0
	for _, v := range logWkRefs {
		avgRef += v
	}
	avgRef /= float64(nRefs)
	gap := avgRef - math.Log(realSSE+1e-10)
	return gap, logWkRefs
}

func (fa *FingerprintAnalyzer) bootstrapStability(siteData []models.XRFMeasurement, means, stds []float64, k, nBootstraps int) float64 {
	n := len(siteData)
	if n < 4 {
		return 0.5
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	data := fa.buildAndStandardizeMatrix(siteData, means, stds)
	pc := fa.performPCA(data)
	projected := fa.projectToPCs(data, pc, fa.cfg.NumComponents, means, stds)
	origLabels, _, _ := fa.performKMeans(projected, k, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)

	var wg sync.WaitGroup
	mu := sync.Mutex{}
	var totalJaccard float64
	validCount := 0

	for b := 0; b < nBootstraps; b++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sample := make([]models.XRFMeasurement, n)
			indices := make([]int, n)
			for i := 0; i < n; i++ {
				idx := r.Intn(n)
				sample[i] = siteData[idx]
				indices[i] = idx
			}
			sMeans := make([]float64, 6)
			sStds := make([]float64, 6)
			sData := fa.buildAndStandardizeMatrix(sample, sMeans, sStds)
			sPC := fa.performPCA(sData)
			sProj := fa.projectToPCs(sData, sPC, fa.cfg.NumComponents, sMeans, sStds)
			sLabels, _, _ := fa.performKMeans(sProj, k, fa.cfg.MaxIterations, fa.cfg.ConvergenceEps)

			mu.Lock()
			if len(sLabels) == len(indices) {
				maxJac := 0.0
				for c := 0; c < k; c++ {
					origSet := map[int]bool{}
					for i, l := range origLabels {
						if l == c {
							origSet[i] = true
						}
					}
					for c2 := 0; c2 < k; c2++ {
						sSet := map[int]bool{}
						for i, l := range sLabels {
							if l == c2 {
								sSet[indices[i]] = true
							}
						}
						inter := 0
						union := 0
						for k := range origSet {
							if sSet[k] {
								inter++
							}
							union++
						}
						for k := range sSet {
							if !origSet[k] {
								union++
							}
						}
						if union > 0 {
							j := float64(inter) / float64(union)
							if j > maxJac {
								maxJac = j
							}
						}
					}
				}
				totalJaccard += maxJac
				validCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if validCount == 0 {
		return 0
	}
	return totalJaccard / float64(validCount)
}

// ============== 指纹匹配 ==============

func (fa *FingerprintAnalyzer) computeFingerprintDistance(siteRatios map[string]float64,
	isotopes *models.IsotopeRatio, fp *models.PollutionFingerprint) float64 {

	var totalDist float64
	var totalWeight float64

	for ratioName, w := range fa.fpCfg.RatioWeights {
		siteVal := siteRatios[ratioName]
		fpVal := 0.0
		switch ratioName {
		case "pb_zn_ratio":
			fpVal = fp.PbZnRatio
		case "cu_pb_ratio":
			fpVal = fp.CuPbRatio
		case "as_hg_ratio":
			fpVal = fp.AsHgRatio
		case "cd_zn_ratio":
			fpVal = fp.CdZnRatio
		case "cu_as_ratio":
			fpVal = fp.CuAsRatio
		}
		if fa.fpCfg.LogTransform {
			siteVal = math.Log1p(siteVal)
			fpVal = math.Log1p(fpVal)
		}
		totalDist += w * math.Abs(siteVal-fpVal)
		totalWeight += w
	}
	if isotopes != nil && isotopes.Pb206Pb207 > 0 && isotopes.Pb208Pb207 > 0 &&
		fp.Pb206Pb207 > 0 && fp.Pb208Pb207 > 0 {
		for name, w := range fa.fpCfg.IsotopeWeights {
			var sv, fv float64
			switch name {
			case "pb206_pb207":
				sv = isotopes.Pb206Pb207
				fv = fp.Pb206Pb207
			case "pb208_pb207":
				sv = isotopes.Pb208Pb207
				fv = fp.Pb208Pb207
			}
			totalDist += w * math.Abs(sv-fv)
			totalWeight += w
		}
	}
	if totalWeight == 0 {
		return math.Inf(1)
	}
	return totalDist / totalWeight
}

func (fa *FingerprintAnalyzer) computeFingerprintSimilarity(distance float64) float64 {
	return 1.0 / (fa.fpCfg.SimilarityBias + distance)
}

// ============== 工具 ==============

func euclidean(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		d := a[i] - b[i]
		s += d * d
	}
	return math.Sqrt(s)
}

func safeDiv(a, b float64) float64 {
	if b < 1e-6 {
		return 0
	}
	return a / b
}

func (fa *FingerprintAnalyzer) normalize(vals []float64) []float64 {
	minV := math.Inf(1)
	maxV := math.Inf(-1)
	for _, v := range vals {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	norm := make([]float64, len(vals))
	span := maxV - minV
	if span < 1e-12 {
		for i := range norm {
			norm[i] = 0.5
		}
		return norm
	}
	for i, v := range vals {
		norm[i] = (v - minV) / span
	}
	return norm
}

// ============== 兼容旧代码 ==============
func (fa *FingerprintAnalyzer) GetConfig() config.PCAConfig {
	return fa.cfg
}
