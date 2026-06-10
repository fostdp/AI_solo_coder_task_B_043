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

	overallRiskLevel, overallRiskColor := fsm.classifyOverallRisk(maxIgeo, totalRI)

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
