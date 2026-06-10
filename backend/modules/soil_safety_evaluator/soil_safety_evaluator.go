package soil_safety_evaluator

import (
	"context"
	"fmt"
	"math"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/common"
)

type KrigingParams struct {
	Nugget float64
	Sill   float64
	Range  float64
	Model  string
}

type KrigingService struct {
	params KrigingParams
}

func NewKrigingService() *KrigingService {
	return &KrigingService{
		params: KrigingParams{Nugget: 10.0, Sill: 200.0, Range: 1500.0, Model: "spherical"},
	}
}

func SphericalVariogram(h float64, params KrigingParams) float64 {
	if h <= 0 {
		return params.Nugget
	}
	if h >= params.Range {
		return params.Nugget + params.Sill
	}
	ratio := h / params.Range
	return params.Nugget + params.Sill*(1.5*ratio-0.5*ratio*ratio*ratio)
}

func (ks *KrigingService) EstimateVariogram(farmlands []models.FarmlandSoil, calcRI func(models.FarmlandSoil) (float64, map[string]float64, float64, string)) KrigingParams {
	n := len(farmlands)
	if n < 2 {
		return KrigingParams{Nugget: 10.0, Sill: 200.0, Range: 1500.0, Model: "spherical"}
	}

	type pair struct {
		dist  float64
		gamma float64
	}
	pairs := make([]pair, 0, n*(n-1)/2)
	ris := make([]float64, n)
	for i, fl := range farmlands {
		ri, _, _, _ := calcRI(fl)
		ris[i] = ri
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			d := math.Abs(float64(farmlands[i].DistanceFromSite - farmlands[j].DistanceFromSite))
			if d < 1 {
				d = 1
			}
			diff := ris[i] - ris[j]
			gamma := 0.5 * diff * diff
			pairs = append(pairs, pair{d, gamma})
		}
	}

	meanRI := common.MeanFloat64(ris)
	variance := 0.0
	for _, ri := range ris {
		variance += (ri - meanRI) * (ri - meanRI)
	}
	variance /= float64(n)
	sill := variance * 1.2
	if sill < 50 {
		sill = 50
	}

	nugget := 0.0
	minDist := 1e9
	for _, p := range pairs {
		if p.dist < float64(minDist) {
			minDist = int(p.dist)
			nugget = p.gamma
		}
	}
	if nugget > sill*0.5 {
		nugget = sill * 0.3
	}
	if nugget < 0 {
		nugget = 0
	}

	rangeEst := 1500.0
	maxPairDist := 0.0
	for _, p := range pairs {
		if p.dist > maxPairDist {
			maxPairDist = p.dist
		}
	}
	if maxPairDist > 300 {
		rangeEst = maxPairDist * 0.7
	}
	if rangeEst < 500 {
		rangeEst = 500
	}

	return KrigingParams{
		Nugget: nugget,
		Sill:   sill,
		Range:  rangeEst,
		Model:  "spherical",
	}
}

func (ks *KrigingService) OrdinaryKriging(predDist int, direction string, farmlands []models.FarmlandSoil, params KrigingParams, calcRI func(models.FarmlandSoil) (float64, map[string]float64, float64, string)) (float64, float64) {
	n := len(farmlands)
	if n == 0 {
		return 0, 1000
	}
	if n == 1 {
		ri, _, _, _ := calcRI(farmlands[0])
		d := math.Abs(float64(predDist - farmlands[0].DistanceFromSite))
		std := math.Sqrt(SphericalVariogram(d, params))
		return ri, std
	}

	matrixSize := n + 1
	K := make([][]float64, matrixSize)
	for i := range K {
		K[i] = make([]float64, matrixSize)
	}

	values := make([]float64, n)
	for i, fl := range farmlands {
		ri, _, _, _ := calcRI(fl)
		values[i] = ri
	}

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			d := math.Abs(float64(farmlands[i].DistanceFromSite - farmlands[j].DistanceFromSite))
			K[i][j] = SphericalVariogram(d, params)
		}
		K[i][n] = 1.0
		K[n][i] = 1.0
	}
	K[n][n] = 0.0

	b := make([]float64, matrixSize)
	for i := 0; i < n; i++ {
		d := math.Abs(float64(predDist - farmlands[i].DistanceFromSite))
		b[i] = SphericalVariogram(d, params)
	}
	b[n] = 1.0

	weights, ok := common.GaussElimination(K, b)
	if !ok {
		return ks.InverseDistanceWeighting(predDist, farmlands, calcRI)
	}

	predicted := 0.0
	for i := 0; i < n; i++ {
		predicted += weights[i] * values[i]
	}
	if predicted < 0 {
		predicted = 0
	}

	krigingVariance := 0.0
	for i := 0; i < n; i++ {
		d := math.Abs(float64(predDist - farmlands[i].DistanceFromSite))
		krigingVariance += weights[i] * SphericalVariogram(d, params)
	}
	krigingVariance += weights[n]
	if krigingVariance < 0 {
		krigingVariance = SphericalVariogram(0, params)
	}
	stdErr := math.Sqrt(krigingVariance)

	return predicted, stdErr
}

func (ks *KrigingService) InverseDistanceWeighting(predDist int, farmlands []models.FarmlandSoil, calcRI func(models.FarmlandSoil) (float64, map[string]float64, float64, string)) (float64, float64) {
	weightedSum := 0.0
	weightTotal := 0.0
	power := 2.0
	maxWeight := 0.0

	for _, fl := range farmlands {
		d := math.Abs(float64(predDist - fl.DistanceFromSite))
		if d < 1 {
			d = 1
		}
		w := 1.0 / math.Pow(d, power)
		ri, _, _, _ := calcRI(fl)
		weightedSum += w * ri
		weightTotal += w
		if w > maxWeight {
			maxWeight = w
		}
	}

	predicted := 0.0
	if weightTotal > 0 {
		predicted = weightedSum / weightTotal
	}

	stdErr := 50.0
	if maxWeight > 0 {
		stdErr = 100.0 / math.Sqrt(maxWeight)
	}
	if stdErr > 500 {
		stdErr = 500
	}

	return predicted, stdErr
}

func (ks *KrigingService) Interpolate(farmlands []models.FarmlandSoil, calcRI func(models.FarmlandSoil) (float64, map[string]float64, float64, string)) *models.SpatialUncertaintyReport {
	if len(farmlands) == 0 {
		return nil
	}

	params := ks.EstimateVariogram(farmlands, calcRI)

	gridDistances := []int{100, 500, 1000, 1500, 2000, 2500, 3000}
	directions := []string{"北", "东北", "东", "东南", "南", "西南", "西", "西北"}

	predictionGrid := make([]map[string]interface{}, 0, len(gridDistances)*len(directions))
	stdErrorGrid := make([]map[string]interface{}, 0, len(gridDistances)*len(directions))
	interpolatedRI := make([]float64, 0, len(gridDistances))
	interpolatedRI95CI := make([][]float64, 0, len(gridDistances))

	classifyRI := func(ri float64) string {
		switch {
		case ri < 150:
			return "低风险"
		case ri < 300:
			return "中等风险"
		case ri < 600:
			return "较高风险"
		default:
			return "极高风险"
		}
	}

	totalStdErr := 0.0
	pointCount := 0

	for _, dist := range gridDistances {
		riAtDist := make([]float64, 0, len(directions))
		stdAtDist := make([]float64, 0, len(directions))

		for _, dir := range directions {
			predRI, predStd := ks.OrdinaryKriging(dist, dir, farmlands, params, calcRI)

			predictionGrid = append(predictionGrid, map[string]interface{}{
				"distance_m":     dist,
				"direction":      dir,
				"predicted_ri":   common.Round(predRI, 2),
				"predicted_level": classifyRI(predRI),
			})

			stdErrorGrid = append(stdErrorGrid, map[string]interface{}{
				"distance_m": dist,
				"direction":  dir,
				"std_error":  common.Round(predStd, 2),
				"ci95_low":   common.Round(math.Max(0, predRI-1.96*predStd), 2),
				"ci95_high":  common.Round(predRI+1.96*predStd, 2),
				"cv":         common.Round(predStd/(predRI+0.01), 4),
			})

			riAtDist = append(riAtDist, predRI)
			stdAtDist = append(stdAtDist, predStd)
			totalStdErr += predStd
			pointCount++
		}

		meanRI := common.MeanFloat64(riAtDist)
		meanStd := common.MeanFloat64(stdAtDist)
		interpolatedRI = append(interpolatedRI, common.Round(meanRI, 2))
		interpolatedRI95CI = append(interpolatedRI95CI, []float64{
			common.Round(math.Max(0, meanRI-1.96*meanStd), 2),
			common.Round(meanRI+1.96*meanStd, 2),
		})
	}

	avgStdErr := 0.0
	if pointCount > 0 {
		avgStdErr = totalStdErr / float64(pointCount)
	}

	sparsityWarning := false
	dataQualityNote := ""

	uniqueDistances := make(map[int]bool)
	for _, fl := range farmlands {
		uniqueDistances[fl.DistanceFromSite] = true
	}
	effectiveCount := len(uniqueDistances)
	if effectiveCount < 4 {
		sparsityWarning = true
		dataQualityNote = fmt.Sprintf("样点空间分布稀疏（仅%d个不同距离），克里金预测不确定度较大，建议加密采样至8个以上均匀分布样点", effectiveCount)
	}

	maxDist := 0
	for _, fl := range farmlands {
		if fl.DistanceFromSite > maxDist {
			maxDist = fl.DistanceFromSite
		}
	}
	if maxDist > 0 && float64(maxDist) < params.Range*0.7 {
		if dataQualityNote == "" {
			dataQualityNote = fmt.Sprintf("样点最大距离(%dm)小于空间变程(%.0fm)的70%%，建议补充远距采样以提升外推可靠性", maxDist, params.Range)
			sparsityWarning = true
		} else {
			dataQualityNote += fmt.Sprintf("；样点最大距离(%dm)小于变程(%.0fm)的70%%，远距外推不可靠", maxDist, params.Range)
		}
	}

	if !sparsityWarning {
		dataQualityNote = "样点空间分布充分，克里金插值可靠性良好"
	}

	return &models.SpatialUncertaintyReport{
		InterpolationMethod:   "OrdinaryKriging_SphericalModel",
		KrigingNugget:         common.Round(params.Nugget, 4),
		KrigingSill:           common.Round(params.Sill, 4),
		KrigingRange:          common.Round(params.Range, 1),
		KrigingModel:          params.Model,
		PredictionGrid:        predictionGrid,
		StdErrorGrid:          stdErrorGrid,
		SampleSparsityWarning: sparsityWarning,
		EffectiveSampleCount:  effectiveCount,
		AvgPredictionStdErr:   common.Round(avgStdErr, 4),
		DataQualityNote:       dataQualityNote,
		InterpolatedRI:        interpolatedRI,
		InterpolatedRI95CI:    interpolatedRI95CI,
	}
}

type SoilSafetyEvaluator struct {
	geoCfg         config.GeoAccumulationConfig
	riCfg          config.EcoRiskRIConfig
	cropCfg        config.CropBioaccumulationConfig
	geoLevels      []struct {
		Min, Max    float64
		Level       string
		Description string
	}
	krigingService *KrigingService
}

func NewSoilSafetyEvaluator() *SoilSafetyEvaluator {
	return &SoilSafetyEvaluator{
		geoCfg:         config.DefaultGeoAccumulationConfig,
		riCfg:          config.DefaultEcoRiskRIConfig,
		cropCfg:        config.DefaultCropBioaccumulation,
		geoLevels:      config.GeoAccumulationLevels,
		krigingService: NewKrigingService(),
	}
}

func (fsm *SoilSafetyEvaluator) AssessFarmSafety(ctx context.Context, siteID int, farmlands []models.FarmlandSoil) (*models.FarmSafetyAssessmentResult, error) {
	if len(farmlands) == 0 {
		return nil, fmt.Errorf("no farmland soil samples provided")
	}

	sampleResults := make([]models.FarmSampleGeoAccumulation, 0, len(farmlands))
	ecoRiskResults := make([]models.FarmSampleEcoRiskResult, 0, len(farmlands))
	cropRecommendations := make([]models.FarmCropRecommendation, 0, len(farmlands))
	distanceGroups := make(map[string][]models.FarmlandSoil)

	var maxIgeo, maxEri, totalRI float64

	for _, fl := range farmlands {
		sampleIgeo, sampleMaxIgeo, sampleMaxMetal := fsm.calcSampleIgeo(fl)
		sampleName := fmt.Sprintf("样点%d", fl.ID)
		if fl.Direction != "" {
			sampleName = fl.Direction
		}
		sampleResults = append(sampleResults, models.FarmSampleGeoAccumulation{
			SampleName:   sampleName,
			MetalResults: sampleIgeo,
			MaxIgeo:      sampleMaxIgeo,
			MaxIgeoMetal: sampleMaxMetal,
		})

		if sampleMaxIgeo > maxIgeo {
			maxIgeo = sampleMaxIgeo
		}

		ri, metalEri, sampleMaxEri, sampleMaxEriMetal := fsm.calcRI(fl)
		riskLevel := fsm.classifyRI(ri)
		ecoRiskResults = append(ecoRiskResults, models.FarmSampleEcoRiskResult{
			SampleName:  sampleName,
			RI:          common.Round(ri, 2),
			RiskLevel:   riskLevel,
			MetalEri:    metalEri,
			MaxEri:      common.Round(sampleMaxEri, 2),
			MaxEriMetal: sampleMaxEriMetal,
		})

		totalRI += ri
		if sampleMaxEri > maxEri {
			maxEri = sampleMaxEri
		}

		cropRec := fsm.assessCropRisk(fl, fl.LandUseType)
		cropRecommendations = append(cropRecommendations, cropRec)

		distLabel := fsm.formatDistanceLabel(float64(fl.DistanceFromSite))
		distanceGroups[distLabel] = append(distanceGroups[distLabel], fl)
	}

	distanceDecay := fsm.calcDistanceDecay(distanceGroups)

	spatialUncertainty := fsm.performKrigingInterpolation(farmlands)

	overallRiskLevel, overallRiskColor := fsm.classifyOverallRisk(maxIgeo, totalRI)

	if spatialUncertainty != nil && spatialUncertainty.SampleSparsityWarning {
		switch overallRiskLevel {
		case "低风险":
			overallRiskLevel = "低风险（数据稀疏保守估计）"
		case "中风险":
			overallRiskLevel = "中风险（数据稀疏保守估计）"
		}
	}

	summary := fsm.generateSummary(len(farmlands), maxIgeo, totalRI, overallRiskLevel)

	return &models.FarmSafetyAssessmentResult{
		SiteID:              siteID,
		AssessmentDate:      time.Now().Format("2006-01-02"),
		SampleResults:       sampleResults,
		EcoRiskResults:      ecoRiskResults,
		DistanceDecay:       distanceDecay,
		CropRecommendations: cropRecommendations,
		OverallRiskLevel:    overallRiskLevel,
		OverallRiskColor:    overallRiskColor,
		MaxIgeo:             common.Round(maxIgeo, 4),
		MaxEri:              common.Round(maxEri, 2),
		TotalRI:             common.Round(totalRI, 2),
		Summary:             summary,
		SpatialUncertainty:  spatialUncertainty,
	}, nil
}

func (fsm *SoilSafetyEvaluator) calcIgeo(metal string, conc float64) (float64, int, string) {
	bg, ok := fsm.geoCfg.BackgroundValues[metal]
	if !ok || bg <= 0 {
		return 0, 0, "无背景值数据"
	}
	if conc <= 0 {
		return math.Inf(-1), 0, fsm.geoLevels[0].Description
	}

	k := fsm.geoCfg.CorrectionFactor
	if k == 0 {
		k = 1.5
	}
	denominator := k * bg
	if denominator <= 0 {
		return 0, 0, "参数错误"
	}

	ratio := conc / denominator
	if ratio <= 0 {
		return math.Inf(-1), 0, fsm.geoLevels[0].Description
	}

	igeo := math.Log2(ratio)

	for i, lvl := range fsm.geoLevels {
		if igeo >= lvl.Min && igeo < lvl.Max {
			return common.Round(igeo, 4), i, lvl.Description
		}
	}

	lastIdx := len(fsm.geoLevels) - 1
	return common.Round(igeo, 4), lastIdx, fsm.geoLevels[lastIdx].Description
}

func (fsm *SoilSafetyEvaluator) calcSampleIgeo(fl models.FarmlandSoil) ([]models.FarmGeoAccumulationResult, float64, string) {
	metals := map[string]float64{
		"Pb": fl.Pb, "Zn": fl.Zn, "Cu": fl.Cu, "As": fl.As,
		"Hg": fl.Hg, "Cd": fl.Cd, "Cr": fl.Cr, "Ni": fl.Ni,
	}

	results := make([]models.FarmGeoAccumulationResult, 0, len(metals))
	var maxIgeo float64 = math.Inf(-1)
	maxMetal := ""

	for metal, conc := range metals {
		igeo, level, desc := fsm.calcIgeo(metal, conc)
		results = append(results, models.FarmGeoAccumulationResult{
			Metal:         metal,
			Concentration: conc,
			Igeo:          igeo,
			Level:         level,
			LevelDesc:     desc,
		})
		if igeo > maxIgeo {
			maxIgeo = igeo
			maxMetal = metal
		}
	}

	return results, common.Round(maxIgeo, 4), maxMetal
}

func (fsm *SoilSafetyEvaluator) calcRI(fl models.FarmlandSoil) (float64, map[string]float64, float64, string) {
	metals := map[string]float64{
		"Pb": fl.Pb, "Zn": fl.Zn, "Cu": fl.Cu, "As": fl.As,
		"Hg": fl.Hg, "Cd": fl.Cd, "Cr": fl.Cr, "Ni": fl.Ni,
	}

	metalEri := make(map[string]float64)
	var ri float64
	var maxEri float64
	maxEriMetal := ""

	for metal, conc := range metals {
		tri, ok1 := fsm.riCfg.ToxicFactors[metal]
		bn, ok2 := fsm.riCfg.RefValues[metal]
		if !ok1 || !ok2 || bn <= 0 {
			continue
		}
		cfi := conc / bn
		eri := tri * cfi
		metalEri[metal] = common.Round(eri, 2)
		ri += eri
		if eri > maxEri {
			maxEri = eri
			maxEriMetal = metal
		}
	}

	return common.Round(ri, 2), metalEri, common.Round(maxEri, 2), maxEriMetal
}

func (fsm *SoilSafetyEvaluator) classifyRI(ri float64) string {
	switch {
	case ri < 150:
		return "低风险"
	case ri < 300:
		return "中等风险"
	case ri < 600:
		return "较高风险"
	default:
		return "极高风险"
	}
}

func (fsm *SoilSafetyEvaluator) formatDistanceLabel(dist float64) string {
	switch {
	case dist <= 500:
		return "≤500米"
	case dist <= 1000:
		return "500-1000米"
	case dist <= 2000:
		return "1000-2000米"
	default:
		return "≥2000米"
	}
}

func (fsm *SoilSafetyEvaluator) calcDistanceDecay(groups map[string][]models.FarmlandSoil) []models.FarmDistanceDecayResult {
	labels := []string{"≤500米", "500-1000米", "1000-2000米", "≥2000米"}
	results := make([]models.FarmDistanceDecayResult, 0, len(labels))

	for _, label := range labels {
		samples, ok := groups[label]
		if !ok || len(samples) == 0 {
			continue
		}

		var sumIgeo, sumRI, maxIgeo, maxRI float64
		for _, s := range samples {
			_, sampleMaxIgeo, _ := fsm.calcSampleIgeo(s)
			sampleRI, _, _, _ := fsm.calcRI(s)
			sumIgeo += sampleMaxIgeo
			sumRI += sampleRI
			if sampleMaxIgeo > maxIgeo {
				maxIgeo = sampleMaxIgeo
			}
			if sampleRI > maxRI {
				maxRI = sampleRI
			}
		}

		results = append(results, models.FarmDistanceDecayResult{
			DistanceLabel: label,
			SampleCount:   len(samples),
			AvgIgeo:       common.Round(sumIgeo/float64(len(samples)), 4),
			AvgRI:         common.Round(sumRI/float64(len(samples)), 2),
			MaxIgeo:       common.Round(maxIgeo, 4),
			MaxRI:         common.Round(maxRI, 2),
		})
	}

	return results
}

func (fsm *SoilSafetyEvaluator) assessCropRisk(fl models.FarmlandSoil, landUseType string) models.FarmCropRecommendation {
	metals := []string{"Pb", "Zn", "Cu", "As", "Hg", "Cd", "Cr", "Ni"}
	soilConcs := map[string]float64{
		"Pb": fl.Pb, "Zn": fl.Zn, "Cu": fl.Cu, "As": fl.As,
		"Hg": fl.Hg, "Cd": fl.Cd, "Cr": fl.Cr, "Ni": fl.Ni,
	}

	bcfMap, bcfOk := fsm.cropCfg.BCF[landUseType]
	if !bcfOk {
		bcfMap, bcfOk = fsm.cropCfg.BCF["旱地"]
	}

	predictions := make([]models.FarmCropPrediction, 0, len(metals))
	exceedCount := 0
	closeCount := 0

	closeRatio := 0.7

	for _, metal := range metals {
		conc := soilConcs[metal]
		bcf := 0.0
		if bcfOk {
			bcf = bcfMap[metal]
		}
		limit := fsm.cropCfg.FoodSafetyLimit[metal]

		predictedConc := conc * bcf
		exceedRatio := 0.0
		isExceed := false
		isClose := false

		if limit > 0 {
			exceedRatio = predictedConc / limit
			isExceed = exceedRatio > 1.0
			isClose = !isExceed && exceedRatio >= closeRatio
		}

		if isExceed {
			exceedCount++
		} else if isClose {
			closeCount++
		}

		predictions = append(predictions, models.FarmCropPrediction{
			Metal:             metal,
			SoilConcentration: conc,
			BCF:               bcf,
			PredictedCropConc: common.Round(predictedConc, 6),
			FoodLimit:         limit,
			ExceedRatio:       common.Round(exceedRatio, 4),
			IsExceed:          isExceed,
			IsClose:           isClose,
		})
	}

	var riskLevel, riskColor string
	var recommendations []string

	if exceedCount > 0 {
		riskLevel = "高风险"
		riskColor = "red"
		recommendations = []string{
			"严禁种植食用作物",
			"建议改为工业用地、绿化用地或造林用地",
			"需进行土壤污染风险评估和修复治理",
		}
	} else if closeCount >= 1 && closeCount <= 2 {
		riskLevel = "中等风险"
		riskColor = "yellow"
		recommendations = []string{
			"建议选用低富集作物品种替代",
			"可采取土壤改良措施（如钝化剂、有机物料）",
			"加强农产品质量监测",
		}
	} else {
		riskLevel = "低风险"
		riskColor = "green"
		recommendations = []string{
			"土壤环境质量良好，推荐正常种植",
			"建议定期进行土壤环境监测",
		}
	}

	return models.FarmCropRecommendation{
		LandUseType:     landUseType,
		RiskLevel:       riskLevel,
		RiskColor:       riskColor,
		Recommendations: recommendations,
		Predictions:     predictions,
		ExceedCount:     exceedCount,
		CloseCount:      closeCount,
		TotalMetals:     len(metals),
	}
}

func (fsm *SoilSafetyEvaluator) classifyOverallRisk(maxIgeo, totalRI float64) (string, string) {
	switch {
	case maxIgeo >= 5 || totalRI >= 600:
		return "高风险", "red"
	case maxIgeo >= 3 || totalRI >= 300:
		return "较高风险", "orange"
	case maxIgeo >= 1 || totalRI >= 150:
		return "中风险", "yellow"
	default:
		return "低风险", "green"
	}
}

func (fsm *SoilSafetyEvaluator) generateSummary(sampleCount int, maxIgeo, totalRI float64, riskLevel string) string {
	return fmt.Sprintf("本次评估共分析 %d 个农田土壤样点，最大地积累指数 Igeo = %.4f，总潜在生态风险指数 RI = %.2f，综合风险等级为【%s】。",
		sampleCount, maxIgeo, totalRI, riskLevel)
}

func (fsm *SoilSafetyEvaluator) performKrigingInterpolation(farmlands []models.FarmlandSoil) *models.SpatialUncertaintyReport {
	return fsm.krigingService.Interpolate(farmlands, fsm.calcRI)
}

func (fsm *SoilSafetyEvaluator) GeoCfg() config.GeoAccumulationConfig {
	return fsm.geoCfg
}

func (fsm *SoilSafetyEvaluator) SetGeoCfg(cfg config.GeoAccumulationConfig) {
	fsm.geoCfg = cfg
}

func (fsm *SoilSafetyEvaluator) RICfg() config.EcoRiskRIConfig {
	return fsm.riCfg
}

func (fsm *SoilSafetyEvaluator) GeoLevels() []struct {
	Min, Max    float64
	Level       string
	Description string
} {
	return fsm.geoLevels
}

func (fsm *SoilSafetyEvaluator) CropCfg() config.CropBioaccumulationConfig {
	return fsm.cropCfg
}

func (fsm *SoilSafetyEvaluator) CalcIgeo(metal string, conc float64) (float64, int, string) {
	return fsm.calcIgeo(metal, conc)
}

func (fsm *SoilSafetyEvaluator) CalcSampleIgeo(fl models.FarmlandSoil) ([]models.FarmGeoAccumulationResult, float64, string) {
	return fsm.calcSampleIgeo(fl)
}

func (fsm *SoilSafetyEvaluator) CalcRI(fl models.FarmlandSoil) (float64, map[string]float64, float64, string) {
	return fsm.calcRI(fl)
}

func (fsm *SoilSafetyEvaluator) ClassifyRI(ri float64) string {
	return fsm.classifyRI(ri)
}

func (fsm *SoilSafetyEvaluator) FormatDistanceLabel(dist float64) string {
	return fsm.formatDistanceLabel(dist)
}

func (fsm *SoilSafetyEvaluator) CalcDistanceDecay(groups map[string][]models.FarmlandSoil) []models.FarmDistanceDecayResult {
	return fsm.calcDistanceDecay(groups)
}

func (fsm *SoilSafetyEvaluator) AssessCropRisk(fl models.FarmlandSoil, landUseType string) models.FarmCropRecommendation {
	return fsm.assessCropRisk(fl, landUseType)
}

func (fsm *SoilSafetyEvaluator) ClassifyOverallRisk(maxIgeo, totalRI float64) (string, string) {
	return fsm.classifyOverallRisk(maxIgeo, totalRI)
}

func (fsm *SoilSafetyEvaluator) GenerateSummary(sampleCount int, maxIgeo, totalRI float64, riskLevel string) string {
	return fsm.generateSummary(sampleCount, maxIgeo, totalRI, riskLevel)
}
