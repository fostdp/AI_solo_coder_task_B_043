package services

import (
	"context"
	"math"
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

func (s *FingerprintService) PerformPCA(ctx context.Context, siteData []models.SiteWithPollution) ([]models.PCAResult, error) {
	if len(siteData) == 0 {
		return []models.PCAResult{}, nil
	}

	n := len(siteData)
	features := 6
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

	var pc stat.PC
	if ok := pc.PrincipalComponents(X, nil); !ok {
		return nil, nil
	}

	k := 3
	proj := mat.NewDense(n, k, nil)
	for i := 0; i < n; i++ {
		pcVec := pc.VectorsTo(nil)
		for j := 0; j < k; j++ {
			sum := 0.0
			for f := 0; f < features; f++ {
				sum += X.At(i, f) * pcVec.At(f, j)
			}
			proj.Set(i, j, sum)
		}
	}

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

	s.performKMeans(results, 8)

	return results, nil
}

func (s *FingerprintService) performKMeans(points []models.PCAResult, k int) {
	if len(points) == 0 || k <= 0 {
		return
	}
	if len(points) < k {
		k = len(points)
	}

	centroids := make([]struct{ x, y, z float64 }, k)
	for i := 0; i < k; i++ {
		idx := i * len(points) / k
		centroids[i] = struct{ x, y, z float64 }{points[idx].PC1, points[idx].PC2, points[idx].PC3}
	}

	for iter := 0; iter < 100; iter++ {
		for i, p := range points {
			minDist := math.Inf(1)
			bestCluster := 0
			for c, cent := range centroids {
				dist := math.Pow(p.PC1-cent.x, 2) + math.Pow(p.PC2-cent.y, 2) + math.Pow(p.PC3-cent.z, 2)
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
				if math.Abs(centroids[c].x-nx) > 1e-6 || math.Abs(centroids[c].y-ny) > 1e-6 || math.Abs(centroids[c].z-nz) > 1e-6 {
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

type RemediationService struct{}

func NewRemediationService() *RemediationService {
	return &RemediationService{}
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

	scoredTechs := s.scoreTechnologies(technologies, detectedMetals, metalConc, speciationMap, m.SoilType, pollutionIndex)

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

func (s *RemediationService) scoreTechnologies(
	techs []models.RemediationTechnology,
	detectedMetals []string,
	metalConc map[string]float64,
	speciation map[string]*models.MetalSpeciation,
	soilType string,
	pollutionIndex float64,
) []models.TechnologyScore {
	scored := make([]models.TechnologyScore, 0, len(techs))

	for _, t := range techs {
		metalMatch := 0
		for _, m := range detectedMetals {
			for _, am := range t.ApplicableMetals {
				if am == m {
					metalMatch++
					break
				}
			}
		}
		metalCoverage := 0.0
		if len(detectedMetals) > 0 {
			metalCoverage = float64(metalMatch) / float64(len(detectedMetals))
		}

		soilMatch := false
		if len(t.ApplicableSoilTypes) == 0 {
			soilMatch = true
		} else {
			for _, st := range t.ApplicableSoilTypes {
				if st == soilType || st == "各种土壤" {
					soilMatch = true
					break
				}
			}
		}

		mobilityFactor := s.calculateMobilityFactor(detectedMetals, speciation)

		subScores := make(map[string]float64)

		subScores["metal_coverage"] = metalCoverage * 100

		subScores["efficiency"] = t.RemediationEfficiency

		subScores["soil_applicability"] = 0.0
		if soilMatch {
			subScores["soil_applicability"] = 100
		}

		costScore := 100.0
		avgCost := (t.CostLow + t.CostHigh) / 2
		if avgCost > 0 {
			costScore = math.Max(0, 100-(avgCost/15000)*100)
		}
		subScores["cost"] = costScore

		durationScore := 100.0
		avgDuration := float64(t.DurationMonthsLow+t.DurationMonthsHigh) / 2
		if avgDuration > 0 {
			durationScore = math.Max(0, 100-(avgDuration/60)*100)
		}
		subScores["duration"] = durationScore

		subScores["environmental"] = t.EnvironmentalImpactScore * 100 / 10
		subScores["sustainability"] = t.SustainabilityScore * 100 / 10

		urgencyFactor := math.Min(1.0, pollutionIndex/2.0)
		speedWeight := 0.15 + urgencyFactor*0.2
		costWeight := 0.25 - urgencyFactor*0.1

		totalScore := subScores["metal_coverage"]*0.30 +
			subScores["efficiency"]*0.20 +
			subScores["soil_applicability"]*0.10 +
			subScores["cost"]*costWeight +
			subScores["duration"]*speedWeight +
			subScores["environmental"]*0.10 +
			subScores["sustainability"]*(0.25-costWeight+0.15-speedWeight)

		if mobilityFactor > 0.5 {
			for _, st := range []string{"固化稳定化", "植物稳定修复"} {
				if t.Category == st {
					totalScore += 5
				}
			}
		}

		if metalConc["Hg"] > 38 {
			if t.Category == "热脱附" {
				totalScore += 10
			}
		}

		scored = append(scored, models.TechnologyScore{
			RemediationTechnology: t,
			TotalScore:            math.Round(totalScore*100) / 100,
			SubScores:             subScores,
			MatchedMetals:         metalMatch,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].TotalScore > scored[j].TotalScore
	})

	if len(scored) > 5 {
		scored = scored[:5]
	}

	return scored
}

func (s *RemediationService) calculateMobilityFactor(metals []string, speciation map[string]*models.MetalSpeciation) float64 {
	if len(speciation) == 0 {
		return 0.3
	}
	totalMobility := 0.0
	count := 0
	for _, m := range metals {
		if sp, ok := speciation[m]; ok {
			total := sp.Exchangeable + sp.CarbonateBound + sp.FeMnOxideBound + sp.OrganicBound + sp.Residual
			if total > 0 {
				mobile := (sp.Exchangeable + sp.CarbonateBound) / total
				totalMobility += mobile
				count++
			}
		}
	}
	if count == 0 {
		return 0.3
	}
	return totalMobility / float64(count)
}
