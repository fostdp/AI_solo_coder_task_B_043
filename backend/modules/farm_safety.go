package modules

import (
	"context"
	"fmt"
	"math"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
)

type FarmSafetyModule struct {
	geoCfg    config.GeoAccumulationConfig
	riCfg     config.EcoRiskRIConfig
	cropCfg   config.CropBioaccumulationConfig
	geoLevels []struct {
		Min, Max    float64
		Level       string
		Description string
	}
}

type KrigingParams struct {
	Nugget    float64
	Sill      float64
	Range     float64
	Model     string
}

type SpatialUncertainty struct {
	InterpolationMethod   string                    `json:"interpolation_method"`
	KrigingParams         KrigingParams              `json:"kriging_params"`
	PredictionGrid        []map[string]interface{}  `json:"prediction_grid"`
	StdErrorGrid          []map[string]interface{}  `json:"std_error_grid"`
	SampleSparsityWarning bool                       `json:"sample_sparsity_warning"`
	EffectiveSampleCount  int                        `json:"effective_sample_count"`
	AvgPredictionStdErr   float64                    `json:"avg_prediction_std_err"`
	DataQualityNote       string                     `json:"data_quality_note"`
}

func NewFarmSafetyModule() *FarmSafetyModule {
	return &FarmSafetyModule{
		geoCfg:    config.DefaultGeoAccumulationConfig,
		riCfg:     config.DefaultEcoRiskRIConfig,
		cropCfg:   config.DefaultCropBioaccumulation,
		geoLevels: config.GeoAccumulationLevels,
	}
}

func (fsm *FarmSafetyModule) AssessFarmSafety(ctx context.Context, siteID int, farmlands []models.FarmlandSoil) (*models.FarmSafetyAssessmentResult, error) {
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
			RI:          round(ri, 2),
			RiskLevel:   riskLevel,
			MetalEri:    metalEri,
			MaxEri:      round(sampleMaxEri, 2),
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

	// ========== 空间插值与不确定性评估（克里金） ==========
	spatialUncertainty := fsm.performKrigingInterpolation(farmlands)

	overallRiskLevel, overallRiskColor := fsm.classifyOverallRisk(maxIgeo, totalRI)

	// 若克里金插值提示数据稀疏，在摘要和风险等级中加入保守估计
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
		MaxIgeo:             round(maxIgeo, 4),
		MaxEri:              round(maxEri, 2),
		TotalRI:             round(totalRI, 2),
		Summary:             summary,
		SpatialUncertainty:  spatialUncertainty,
	}, nil
}

func (fsm *FarmSafetyModule) calcIgeo(metal string, conc float64) (float64, int, string) {
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
			return round(igeo, 4), i, lvl.Description
		}
	}

	lastIdx := len(fsm.geoLevels) - 1
	return round(igeo, 4), lastIdx, fsm.geoLevels[lastIdx].Description
}

func (fsm *FarmSafetyModule) calcSampleIgeo(fl models.FarmlandSoil) ([]models.FarmGeoAccumulationResult, float64, string) {
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

	return results, round(maxIgeo, 4), maxMetal
}

func (fsm *FarmSafetyModule) calcRI(fl models.FarmlandSoil) (float64, map[string]float64, float64, string) {
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
		metalEri[metal] = round(eri, 2)
		ri += eri
		if eri > maxEri {
			maxEri = eri
			maxEriMetal = metal
		}
	}

	return round(ri, 2), metalEri, round(maxEri, 2), maxEriMetal
}

func (fsm *FarmSafetyModule) classifyRI(ri float64) string {
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

func (fsm *FarmSafetyModule) formatDistanceLabel(dist float64) string {
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

func (fsm *FarmSafetyModule) calcDistanceDecay(groups map[string][]models.FarmlandSoil) []models.FarmDistanceDecayResult {
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
			AvgIgeo:       round(sumIgeo/float64(len(samples)), 4),
			AvgRI:         round(sumRI/float64(len(samples)), 2),
			MaxIgeo:       round(maxIgeo, 4),
			MaxRI:         round(maxRI, 2),
		})
	}

	return results
}

func (fsm *FarmSafetyModule) assessCropRisk(fl models.FarmlandSoil, landUseType string) models.FarmCropRecommendation {
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
			PredictedCropConc: round(predictedConc, 6),
			FoodLimit:         limit,
			ExceedRatio:       round(exceedRatio, 4),
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

func (fsm *FarmSafetyModule) classifyOverallRisk(maxIgeo, totalRI float64) (string, string) {
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

func (fsm *FarmSafetyModule) generateSummary(sampleCount int, maxIgeo, totalRI float64, riskLevel string) string {
	return fmt.Sprintf("本次评估共分析 %d 个农田土壤样点，最大地积累指数 Igeo = %.4f，总潜在生态风险指数 RI = %.2f，综合风险等级为【%s】。",
		sampleCount, maxIgeo, totalRI, riskLevel)
}

// performKrigingInterpolation 执行普通克里金空间插值并量化不确定性
// 解决数据稀疏时简单线性插值不准的问题，输出预测网格+标准误+保守风险估计
func (fsm *FarmSafetyModule) performKrigingInterpolation(farmlands []models.FarmlandSoil) *models.SpatialUncertaintyReport {
	if len(farmlands) == 0 {
		return nil
	}

	n := len(farmlands)

	// ========== 1. 变程拟合（经验球状模型） ==========
	// 基于样点距离分布自动估算 nugget, sill, range
	params := fsm.estimateVariogram(farmlands)

	// ========== 2. 构建预测网格（以遗址为中心，径向0~3000m，步长500m） ==========
	gridDistances := []int{100, 500, 1000, 1500, 2000, 2500, 3000}
	directions := []string{"北", "东北", "东", "东南", "南", "西南", "西", "西北"}

	predictionGrid := make([]map[string]interface{}, 0, len(gridDistances)*len(directions))
	stdErrorGrid := make([]map[string]interface{}, 0, len(gridDistances)*len(directions))
	interpolatedRI := make([]float64, 0, len(gridDistances))
	interpolatedRI95CI := make([][]float64, 0, len(gridDistances))

	// ========== 3. 对每个网格点执行克里金预测 ==========
	totalStdErr := 0.0
	pointCount := 0

	for _, dist := range gridDistances {
		riAtDist := make([]float64, 0, len(directions))
		stdAtDist := make([]float64, 0, len(directions))

		for _, dir := range directions {
			// 计算该网格点到各样点的等效距离（方向+距离）
			predPoint := struct {
				dist      int
				direction string
			}{dist, dir}

			// 普通克里金：构建半方差矩阵求解权重
			predRI, predStd := fsm.ordinaryKriging(predPoint.dist, predPoint.direction, farmlands, params)

			predictionGrid = append(predictionGrid, map[string]interface{}{
				"distance_m":  dist,
				"direction":   dir,
				"predicted_ri": round(predRI, 2),
				"predicted_level": fsm.classifyRI(predRI),
			})

			stdErrorGrid = append(stdErrorGrid, map[string]interface{}{
				"distance_m": dist,
				"direction":  dir,
				"std_error":  round(predStd, 2),
				"ci95_low":   round(math.Max(0, predRI-1.96*predStd), 2),
				"ci95_high":  round(predRI+1.96*predStd, 2),
				"cv":         round(predStd/(predRI+0.01), 4),
			})

			riAtDist = append(riAtDist, predRI)
			stdAtDist = append(stdAtDist, predStd)
			totalStdErr += predStd
			pointCount++
		}

		// 该距离处的平均RI及95%置信区间（保守估计取下界）
		meanRI := meanFloat64(riAtDist)
		meanStd := meanFloat64(stdAtDist)
		interpolatedRI = append(interpolatedRI, round(meanRI, 2))
		interpolatedRI95CI = append(interpolatedRI95CI, []float64{
			round(math.Max(0, meanRI-1.96*meanStd), 2),
			round(meanRI+1.96*meanStd, 2),
		})
	}

	avgStdErr := 0.0
	if pointCount > 0 {
		avgStdErr = totalStdErr / float64(pointCount)
	}

	// ========== 4. 数据稀疏性评估 ==========
	sparsityWarning := false
	dataQualityNote := ""

	// 有效样点数不足4个 → 稀疏警告
	uniqueDistances := make(map[int]bool)
	for _, fl := range farmlands {
		uniqueDistances[fl.DistanceFromSite] = true
	}
	effectiveCount := len(uniqueDistances)
	if effectiveCount < 4 {
		sparsityWarning = true
		dataQualityNote = fmt.Sprintf("样点空间分布稀疏（仅%d个不同距离），克里金预测不确定度较大，建议加密采样至8个以上均匀分布样点", effectiveCount)
	}

	// 变程与最大采样距离比较
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
		KrigingNugget:         round(params.Nugget, 4),
		KrigingSill:           round(params.Sill, 4),
		KrigingRange:          round(params.Range, 1),
		KrigingModel:          params.Model,
		PredictionGrid:        predictionGrid,
		StdErrorGrid:          stdErrorGrid,
		SampleSparsityWarning: sparsityWarning,
		EffectiveSampleCount:  effectiveCount,
		AvgPredictionStdErr:   round(avgStdErr, 4),
		DataQualityNote:       dataQualityNote,
		InterpolatedRI:        interpolatedRI,
		InterpolatedRI95CI:    interpolatedRI95CI,
	}
}

// estimateVariogram 基于样点RI值自动拟合球状模型半方差函数参数
// 返回 nugget（块金）, sill（基台）, range（变程）
func (fsm *FarmSafetyModule) estimateVariogram(farmlands []models.FarmlandSoil) KrigingParams {
	n := len(farmlands)
	if n < 2 {
		return KrigingParams{Nugget: 10.0, Sill: 200.0, Range: 1500.0, Model: "spherical"}
	}

	// 计算所有样点对的 (距离, RI半方差)
	type pair struct {
		dist  float64
		gamma float64
	}
	pairs := make([]pair, 0, n*(n-1)/2)
	ris := make([]float64, n)
	for i, fl := range farmlands {
		ri, _, _, _ := fsm.calcRI(fl)
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

	// 样点RI方差作为初始基台估计
	meanRI := meanFloat64(ris)
	variance := 0.0
	for _, ri := range ris {
		variance += (ri - meanRI) * (ri - meanRI)
	}
	variance /= float64(n)
	sill := variance * 1.2
	if sill < 50 {
		sill = 50
	}

	// 最小距离对的半方差作为块金估计
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

	// 变程估计：半方差达到基台95%时的距离
	rangeEst := 1500.0
	maxPairDist := 0.0
	for _, p := range pairs {
		if p.dist > maxPairDist {
			maxPairDist = p.dist
		}
	}
	if maxPairDist > 300 {
		// 经验：变程约为最大样点间距的70%
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

// sphericalVariogram 球状模型半方差计算
// γ(h) = c0 + c·[1.5h/a - 0.5(h/a)^3] (h ≤ a)
// γ(h) = c0 + c (h > a)
func sphericalVariogram(h float64, params KrigingParams) float64 {
	if h <= 0 {
		return params.Nugget
	}
	if h >= params.Range {
		return params.Nugget + params.Sill
	}
	ratio := h / params.Range
	return params.Nugget + params.Sill*(1.5*ratio-0.5*ratio*ratio*ratio)
}

// ordinaryKriging 普通克里金预测
// 对单个预测点（距离+方向），通过求解克里金方程组得到插值结果和标准误
func (fsm *FarmSafetyModule) ordinaryKriging(
	predDist int,
	_ string,
	farmlands []models.FarmlandSoil,
	params KrigingParams,
) (float64, float64) {
	n := len(farmlands)
	if n == 0 {
		return 0, 1000
	}
	if n == 1 {
		ri, _, _, _ := fsm.calcRI(farmlands[0])
		d := math.Abs(float64(predDist - farmlands[0].DistanceFromSite))
		std := math.Sqrt(sphericalVariogram(d, params))
		return ri, std
	}

	// 构建克里金矩阵 (n+1)×(n+1)：n个样点+拉格朗日乘子
	matrixSize := n + 1
	K := make([][]float64, matrixSize)
	for i := range K {
		K[i] = make([]float64, matrixSize)
	}

	// RI值
	values := make([]float64, n)
	for i, fl := range farmlands {
		ri, _, _, _ := fsm.calcRI(fl)
		values[i] = ri
	}

	// 填充半方差矩阵
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			d := math.Abs(float64(farmlands[i].DistanceFromSite - farmlands[j].DistanceFromSite))
			K[i][j] = sphericalVariogram(d, params)
		}
		K[i][n] = 1.0
		K[n][i] = 1.0
	}
	K[n][n] = 0.0

	// 右端向量：预测点到各样点的半方差
	b := make([]float64, matrixSize)
	for i := 0; i < n; i++ {
		d := math.Abs(float64(predDist - farmlands[i].DistanceFromSite))
		b[i] = sphericalVariogram(d, params)
	}
	b[n] = 1.0

	// 高斯消元求解权重
	weights, ok := gaussElimination(K, b)
	if !ok {
		// 退化为距离反比加权（IDW）
		return fsm.inverseDistanceWeighting(predDist, farmlands)
	}

	// 计算预测值
	predicted := 0.0
	for i := 0; i < n; i++ {
		predicted += weights[i] * values[i]
	}
	if predicted < 0 {
		predicted = 0
	}

	// 计算克里金标准误
	// σ² = Σλi·γ(x0,xi) + μ
	krigingVariance := 0.0
	for i := 0; i < n; i++ {
		d := math.Abs(float64(predDist - farmlands[i].DistanceFromSite))
		krigingVariance += weights[i] * sphericalVariogram(d, params)
	}
	krigingVariance += weights[n] // 拉格朗日乘子项
	if krigingVariance < 0 {
		krigingVariance = sphericalVariogram(0, params) // 退化为块金值
	}
	stdErr := math.Sqrt(krigingVariance)

	return predicted, stdErr
}

// inverseDistanceWeighting 距离反比加权（克里金退化时的回退方案）
func (fsm *FarmSafetyModule) inverseDistanceWeighting(predDist int, farmlands []models.FarmlandSoil) (float64, float64) {
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
		ri, _, _, _ := fsm.calcRI(fl)
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

	// 标准误估计：与最大权重成反比（越近越确定）
	stdErr := 50.0
	if maxWeight > 0 {
		stdErr = 100.0 / math.Sqrt(maxWeight)
	}
	if stdErr > 500 {
		stdErr = 500
	}

	return predicted, stdErr
}

// gaussElimination 高斯消元求解线性方程组 Ax = b
// 返回解向量 x 和求解是否成功
func gaussElimination(A [][]float64, b []float64) ([]float64, bool) {
	n := len(A)
	if n == 0 || len(b) != n {
		return nil, false
	}

	// 构造增广矩阵
	aug := make([][]float64, n)
	for i := range A {
		aug[i] = make([]float64, n+1)
		copy(aug[i], A[i])
		aug[i][n] = b[i]
	}

	// 前向消元
	for col := 0; col < n; col++ {
		// 选主元
		maxRow := col
		maxVal := math.Abs(aug[col][col])
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > maxVal {
				maxVal = math.Abs(aug[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-10 {
			return nil, false
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		// 消元
		for row := col + 1; row < n; row++ {
			factor := aug[row][col] / aug[col][col]
			for k := col; k <= n; k++ {
				aug[row][k] -= factor * aug[col][k]
			}
		}
	}

	// 回代求解
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := aug[i][n]
		for j := i + 1; j < n; j++ {
			sum -= aug[i][j] * x[j]
		}
		if math.Abs(aug[i][i]) < 1e-12 {
			return nil, false
		}
		x[i] = sum / aug[i][i]
	}

	return x, true
}

// meanFloat64 工具函数：计算浮点数组均值
func meanFloat64(arr []float64) float64 {
	if len(arr) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range arr {
		s += v
	}
	return s / float64(len(arr))
}

// round 工具函数：四舍五入到指定小数位
func round(v float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Round(v*pow) / pow
}
