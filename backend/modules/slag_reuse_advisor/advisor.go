package slag_reuse_advisor

import (
	"context"
	"fmt"
	"math"
	"strings"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/common"
	"archaeology-pollution-system/repository"
)

type SlagReuseAdvisor struct {
	Std config.BuildingMaterialStandard
}

func NewSlagReuseAdvisor() *SlagReuseAdvisor {
	return &SlagReuseAdvisor{
		Std: config.DefaultBuildingStandard,
	}
}

func (m *SlagReuseAdvisor) AssessRecycle(ctx context.Context, siteID int, slag *models.SlagComposition) (*models.SlagRecycleResult, error) {
	site, err := repository.GetSiteByID(ctx, siteID)
	siteName := ""
	if err == nil && site != nil {
		siteName = site.Name
	}

	cementChecks, cementScore, cementGrade, cementFeasibility, cementDetails := m.AssessCementBlended(slag)
	roadChecks, roadScore, roadGrade, roadFeasibility, roadDetails := m.AssessRoadBase(slag)
	otherUses := m.AssessOtherUses(slag)
	leachingRisk, leachingDetails := m.AssessLeachingRisk(slag)

	acceleratedTests := m.SimulateAcceleratedAging(slag, cementScore, roadScore, leachingRisk)

	conservative := m.ComputeConservativeEstimate(
		slag, cementScore, roadScore, leachingRisk,
		cementGrade, acceleratedTests,
	)

	finalCementScore := cementScore
	finalRoadScore := roadScore
	if conservative.UseConservativeLimits {
		finalCementScore = conservative.CementScoreConservative
		finalRoadScore = conservative.RoadScoreConservative
		_, _, cementGrade, cementFeasibility, _ = m.assessCementBlendedWithScore(finalCementScore)
		_, _, roadGrade, roadFeasibility, _ = m.assessRoadBaseWithScore(finalRoadScore)
	}

	recommendedUse, utilizationPlan := m.DecideRecommendation(finalCementScore, finalRoadScore, otherUses, leachingRisk, cementGrade)
	processFlow := m.GenerateProcessFlow(recommendedUse, leachingRisk)

	assessment := &models.ResourceUtilizationAssessment{
		SiteID:                   siteID,
		MeasurementYear:          slag.MeasurementYear,
		CementBlendedFeasibility: cementFeasibility,
		CementBlendedScore:       common.Round(cementScore, 2),
		CementBlendedGrade:       cementGrade,
		CementDetails:            cementDetails,
		RoadBaseFeasibility:      roadFeasibility,
		RoadBaseScore:            common.Round(roadScore, 2),
		RoadBaseGrade:            roadGrade,
		RoadDetails:              roadDetails,
		OtherUses:                otherUses,
		LeachingRiskLevel:        leachingRisk,
		LeachingRiskDetails:      leachingDetails,
		RecommendedUse:           recommendedUse,
		UtilizationPlan:          utilizationPlan,
	}

	return &models.SlagRecycleResult{
		SiteID:               siteID,
		SiteName:             siteName,
		Composition:          slag,
		Assessment:           assessment,
		CementChecks:         cementChecks,
		RoadChecks:           roadChecks,
		ProcessFlow:          processFlow,
		AcceleratedTests:     acceleratedTests,
		ConservativeEstimate: conservative,
	}, nil
}

func (m *SlagReuseAdvisor) AssessCementBlended(slag *models.SlagComposition) ([]models.CementStandardCheck, float64, string, string, map[string]interface{}) {
	denomSiAl := slag.SiO2 + slag.Al2O3
	if denomSiAl <= 0 {
		denomSiAl = 1
	}
	basicity := (slag.CaO + slag.MgO) / denomSiAl

	act7d := 20 + 1.2*slag.GlassPhase + 15*basicity
	act28d := 30 + 1.5*slag.GlassPhase + 20*math.Min(basicity, 1.0)
	flowRatio := 90 + 0.2*slag.GlassPhase - 0.5*slag.LossOnIgnition
	waterContent := 0.1 + 0.05*slag.LossOnIgnition
	lossOnIgnition := slag.LossOnIgnition
	specificSurface := slag.SpecificSurface

	checks := []models.CementStandardCheck{
		{
			Item:          "活性指数7d",
			Value:         common.Round(act7d, 2),
			StandardLimit: m.Std.CementS75Activity7dMin,
			Pass:          act7d >= m.Std.CementS75Activity7dMin,
			Note:          "GB/T 18046-2017 S75≥55, S95≥75",
		},
		{
			Item:          "活性指数28d",
			Value:         common.Round(act28d, 2),
			StandardLimit: m.Std.CementS75Activity28dMin,
			Pass:          act28d >= m.Std.CementS75Activity28dMin,
			Note:          "GB/T 18046-2017 S75≥75, S95≥95",
		},
		{
			Item:          "流动度比",
			Value:         common.Round(flowRatio, 2),
			StandardLimit: m.Std.CementFlowRatioMin,
			Pass:          flowRatio >= m.Std.CementFlowRatioMin,
			Note:          "GB/T 18046-2017 ≥95%",
		},
		{
			Item:          "含水率",
			Value:         common.Round(waterContent, 3),
			StandardLimit: m.Std.CementWaterContentMax,
			Pass:          waterContent <= m.Std.CementWaterContentMax,
			Note:          "GB/T 18046-2017 ≤1.0%",
		},
		{
			Item:          "烧失量",
			Value:         common.Round(lossOnIgnition, 2),
			StandardLimit: m.Std.CementLossOnIgnitionMax,
			Pass:          lossOnIgnition <= m.Std.CementLossOnIgnitionMax,
			Note:          "GB/T 18046-2017 ≤3.0%",
		},
		{
			Item:          "比表面积",
			Value:         common.Round(specificSurface, 2),
			StandardLimit: m.Std.CementFinenessMin,
			Pass:          specificSurface >= m.Std.CementFinenessMin,
			Note:          "GB/T 18046-2017 ≥350m²/kg",
		},
	}

	scoreAct7d := math.Min(act7d/m.Std.CementS95Activity7dMin*100, 100)
	scoreAct28d := math.Min(act28d/m.Std.CementS95Activity28dMin*100, 100)
	scoreFlow := math.Min(flowRatio/m.Std.CementFlowRatioMin*100, 100)
	scoreWater := math.Max(0, (1-waterContent/m.Std.CementWaterContentMax)*100)
	scoreLoss := math.Max(0, (1-lossOnIgnition/m.Std.CementLossOnIgnitionMax)*100)
	scoreSurface := math.Min(specificSurface/m.Std.CementFinenessMin*100, 100)

	totalScore := scoreAct28d*0.4 + scoreAct7d*0.25 + scoreFlow*0.1 + scoreWater*0.1 + scoreLoss*0.1 + scoreSurface*0.05

	var grade, feasibility string
	if totalScore >= 85 {
		if act28d >= m.Std.CementS95Activity28dMin && act7d >= m.Std.CementS95Activity7dMin {
			grade = "S105/S95"
		} else {
			grade = "S95"
		}
		feasibility = "可行"
	} else if totalScore >= 60 {
		grade = "S75"
		feasibility = "条件可行"
	} else {
		grade = "不合格"
		feasibility = "不可行"
	}

	details := map[string]interface{}{
		"basicity":         common.Round(basicity, 4),
		"activity_7d":      common.Round(act7d, 2),
		"activity_28d":     common.Round(act28d, 2),
		"flow_ratio":       common.Round(flowRatio, 2),
		"water_content":    common.Round(waterContent, 3),
		"loss_ignition":    common.Round(lossOnIgnition, 2),
		"specific_surface": common.Round(specificSurface, 2),
		"sub_scores": map[string]float64{
			"act7d":   common.Round(scoreAct7d, 2),
			"act28d":  common.Round(scoreAct28d, 2),
			"flow":    common.Round(scoreFlow, 2),
			"water":   common.Round(scoreWater, 2),
			"loss":    common.Round(scoreLoss, 2),
			"surface": common.Round(scoreSurface, 2),
		},
	}

	return checks, totalScore, grade, feasibility, details
}

func (m *SlagReuseAdvisor) AssessRoadBase(slag *models.SlagComposition) ([]models.RoadStandardCheck, float64, string, string, map[string]interface{}) {
	cbr := 100 + 2.5*(100-slag.Fayalite-slag.Wollastonite-slag.GlassPhase) - 10*slag.LossOnIgnition
	crushValue := 8 + 0.2*(100-slag.GlassPhase)
	plasticityIdx := 2 + 0.05*(100-slag.SiO2)
	freezeThawLoss := 0.5 + 0.1*slag.LossOnIgnition
	abrasion := 5 + 0.15*(100 - (slag.SiO2 + slag.Al2O3))

	cbrGrade := "三级"
	if cbr >= m.Std.RoadCBRGrade1Min {
		cbrGrade = "一级"
	} else if cbr >= m.Std.RoadCBRGrade2Min {
		cbrGrade = "二级"
	}

	checks := []models.RoadStandardCheck{
		{
			Item:          "CBR值",
			Value:         common.Round(cbr, 2),
			StandardLimit: m.Std.RoadCBRGrade3Min,
			Pass:          cbr >= m.Std.RoadCBRGrade3Min,
			Grade:         cbrGrade,
		},
		{
			Item:          "压碎值",
			Value:         common.Round(crushValue, 2),
			StandardLimit: m.Std.RoadCrushValueMax,
			Pass:          crushValue <= m.Std.RoadCrushValueMax,
			Grade:         common.GradeFromValue(crushValue, 26, 30, 35),
		},
		{
			Item:          "塑性指数",
			Value:         common.Round(plasticityIdx, 2),
			StandardLimit: m.Std.RoadPlasticityIdxMax,
			Pass:          plasticityIdx <= m.Std.RoadPlasticityIdxMax,
			Grade:         common.GradeFromValue(plasticityIdx, 9, 12, 15),
		},
		{
			Item:          "冻融损失",
			Value:         common.Round(freezeThawLoss, 3),
			StandardLimit: m.Std.RoadFreezeThawLossMax,
			Pass:          freezeThawLoss <= m.Std.RoadFreezeThawLossMax,
			Grade:         common.GradeFromValue(freezeThawLoss, 5, 8, 12),
		},
		{
			Item:          "磨耗率",
			Value:         common.Round(abrasion, 2),
			StandardLimit: m.Std.RoadAbrasionMax,
			Pass:          abrasion <= m.Std.RoadAbrasionMax,
			Grade:         common.GradeFromValue(abrasion, 15, 20, 30),
		},
	}

	scoreCBR := math.Min(cbr/m.Std.RoadCBRGrade1Min*100, 100)
	scoreCrush := math.Max(0, (1-crushValue/35)*100)
	scorePI := math.Max(0, (1-plasticityIdx/15)*100)
	scoreFreeze := math.Max(0, (1-freezeThawLoss/12)*100)
	scoreAbrasion := math.Max(0, (1-abrasion/30)*100)

	totalScore := scoreCBR*0.4 + scoreCrush*0.25 + scorePI*0.15 + scoreFreeze*0.1 + scoreAbrasion*0.1

	overallGrade := "三级"
	feasibility := "条件可行"
	if totalScore >= 85 {
		overallGrade = "一级"
		feasibility = "可行"
	} else if totalScore >= 70 {
		overallGrade = "二级"
		feasibility = "可行"
	} else if totalScore < 60 {
		feasibility = "不可行"
	}

	details := map[string]interface{}{
		"cbr":              common.Round(cbr, 2),
		"crush_value":      common.Round(crushValue, 2),
		"plasticity_index": common.Round(plasticityIdx, 2),
		"freeze_thaw_loss": common.Round(freezeThawLoss, 3),
		"abrasion_rate":    common.Round(abrasion, 2),
		"sub_scores": map[string]float64{
			"cbr":      common.Round(scoreCBR, 2),
			"crush":    common.Round(scoreCrush, 2),
			"pi":       common.Round(scorePI, 2),
			"freeze":   common.Round(scoreFreeze, 2),
			"abrasion": common.Round(scoreAbrasion, 2),
		},
	}

	return checks, totalScore, overallGrade, feasibility, details
}

func (m *SlagReuseAdvisor) AssessOtherUses(slag *models.SlagComposition) map[string]interface{} {
	crushValue := 8 + 0.2*(100-slag.GlassPhase)
	leachingPass := slag.PbLeaching <= m.Std.LeachingPbMax &&
		slag.CdLeaching <= m.Std.LeachingCdMax &&
		slag.AsLeaching <= m.Std.LeachingAsMax &&
		slag.HgLeaching <= m.Std.LeachingHgMax &&
		slag.CrLeaching <= m.Std.LeachingCrMax &&
		slag.NiLeaching <= m.Std.LeachingNiMax

	aggDensityScore := math.Min(slag.Density/3.5*100, 100)
	aggCrushScore := math.Max(0, (1-crushValue/35)*100)
	aggLeachingScore := 0.0
	if leachingPass {
		aggLeachingScore = 100
	}
	aggregateScore := aggDensityScore*0.35 + aggCrushScore*0.35 + aggLeachingScore*0.3

	glassChemScore := math.Min((slag.SiO2+slag.Al2O3)/80*100, 100)
	glassColorScore := math.Min(slag.FeO/15*100, 100)
	glassCeramicScore := glassChemScore*0.7 + glassColorScore*0.3

	soilAlkScore := math.Min((slag.CaO+slag.MgO)/40*100, 100)
	soilLeachScore := 0.0
	if leachingPass {
		soilLeachScore = 100
	}
	soilAmendmentScore := soilAlkScore*0.6 + soilLeachScore*0.4

	pbRecycle := math.Min(slag.PbLeaching/5*100, 100)
	znRecycle := 30.0
	cuRecycle := 25.0
	feRecycle := math.Min(slag.FeO/30*100, 100)
	metalRecoveryScore := (pbRecycle + znRecycle + cuRecycle + feRecycle) / 4

	highMetals := []string{}
	if slag.PbLeaching > 1.0 {
		highMetals = append(highMetals, "Pb")
	}
	if slag.CrLeaching > 2.0 {
		highMetals = append(highMetals, "Cr")
	}
	if slag.NiLeaching > 2.0 {
		highMetals = append(highMetals, "Ni")
	}
	if slag.FeO > 20 {
		highMetals = append(highMetals, "Fe(可磁选)")
	}
	metalRecoveryNote := "有价金属含量较低"
	if len(highMetals) > 0 {
		metalRecoveryNote = "可回收: " + strings.Join(highMetals, ", ")
	}

	return map[string]interface{}{
		"concrete_aggregate": map[string]interface{}{
			"score":       common.Round(aggregateScore, 2),
			"density":     slag.Density,
			"crush_value": common.Round(crushValue, 2),
			"leaching_ok": leachingPass,
			"sub_scores": map[string]float64{
				"density":  common.Round(aggDensityScore, 2),
				"crush":    common.Round(aggCrushScore, 2),
				"leaching": common.Round(aggLeachingScore, 2),
			},
		},
		"glass_ceramic": map[string]interface{}{
			"score":            common.Round(glassCeramicScore, 2),
			"sio2_al2o3_total": common.Round(slag.SiO2+slag.Al2O3, 2),
			"feo_color":        common.Round(slag.FeO, 2),
			"can_cast_stone":   glassCeramicScore >= 70,
			"sub_scores": map[string]float64{
				"chemistry": common.Round(glassChemScore, 2),
				"color":     common.Round(glassColorScore, 2),
			},
		},
		"soil_amendment": map[string]interface{}{
			"score":       common.Round(soilAmendmentScore, 2),
			"cao_mgo":     common.Round(slag.CaO+slag.MgO, 2),
			"leaching_ok": leachingPass,
			"sub_scores": map[string]float64{
				"alkalinity": common.Round(soilAlkScore, 2),
				"leaching":   common.Round(soilLeachScore, 2),
			},
		},
		"metal_recovery": map[string]interface{}{
			"score":       common.Round(metalRecoveryScore, 2),
			"note":        metalRecoveryNote,
			"pb_leaching": common.Round(slag.PbLeaching, 4),
			"cr_leaching": common.Round(slag.CrLeaching, 4),
			"ni_leaching": common.Round(slag.NiLeaching, 4),
			"feo_content": common.Round(slag.FeO, 2),
		},
	}
}

func (m *SlagReuseAdvisor) AssessLeachingRisk(slag *models.SlagComposition) (string, map[string]interface{}) {
	leachingMetals := []struct {
		name  string
		value float64
		limit float64
	}{
		{"Pb", slag.PbLeaching, m.Std.LeachingPbMax},
		{"Cd", slag.CdLeaching, m.Std.LeachingCdMax},
		{"As", slag.AsLeaching, m.Std.LeachingAsMax},
		{"Hg", slag.HgLeaching, m.Std.LeachingHgMax},
		{"Cr", slag.CrLeaching, m.Std.LeachingCrMax},
		{"Ni", slag.NiLeaching, m.Std.LeachingNiMax},
	}

	exceedCount := 0
	metalResults := make([]map[string]interface{}, 0, len(leachingMetals))
	severeHgAs := false

	for _, mItem := range leachingMetals {
		exceed := mItem.value > mItem.limit
		if exceed {
			exceedCount++
		}
		severe := false
		if (mItem.name == "Hg" && mItem.value > mItem.limit*5) ||
			(mItem.name == "As" && mItem.value > mItem.limit*5) {
			severe = true
			if exceed {
				severeHgAs = true
			}
		}
		metalResults = append(metalResults, map[string]interface{}{
			"metal":        mItem.name,
			"value":        common.Round(mItem.value, 4),
			"limit":        mItem.limit,
			"exceed":       exceed,
			"exceed_ratio": common.Round(mItem.value/mItem.limit, 2),
			"severe":       severe,
		})
	}

	var riskLevel string
	switch {
	case exceedCount >= 5 || severeHgAs:
		riskLevel = "极高风险"
	case exceedCount >= 3:
		riskLevel = "高风险"
	case exceedCount >= 1:
		riskLevel = "中风险"
	default:
		riskLevel = "低风险"
	}

	details := map[string]interface{}{
		"standard":      "GB5085.3-2007 危险废物鉴别标准 浸出毒性鉴别",
		"exceed_count":  exceedCount,
		"severe_hg_as":  severeHgAs,
		"risk_level":    riskLevel,
		"metal_results": metalResults,
	}

	return riskLevel, details
}

func (m *SlagReuseAdvisor) DecideRecommendation(cementScore, roadScore float64, otherUses map[string]interface{}, leachingRisk, cementGrade string) (string, map[string]interface{}) {
	gcScore := 0.0
	if gc, ok := otherUses["glass_ceramic"].(map[string]interface{}); ok {
		if s, ok2 := gc["score"].(float64); ok2 {
			gcScore = s
		}
	}

	var recommended string
	var reasons []string
	alternatives := make([]string, 0)

	switch leachingRisk {
	case "极高风险", "高风险":
		recommended = "稳定化处理→安全填埋/资源化"
		reasons = append(reasons, "浸出毒性"+leachingRisk+"，需先进行稳定化处理")
		if cementScore >= 60 {
			alternatives = append(alternatives, "稳定化达标后可考虑水泥混合材")
		}
		if roadScore >= 70 {
			alternatives = append(alternatives, "稳定化达标后可考虑道路材料")
		}
	default:
		switch {
		case cementScore >= 60:
			recommended = "优先水泥混合材"
			reasons = append(reasons, fmt.Sprintf("水泥混合材评分%.1f≥60分，等级%s", cementScore, cementGrade))
			if roadScore >= 70 {
				alternatives = append(alternatives, "道路基层材料")
			}
			if gcScore >= 80 {
				alternatives = append(alternatives, "微晶玻璃/铸石")
			}
		case roadScore >= 70:
			recommended = "优先道路基层材料"
			reasons = append(reasons, fmt.Sprintf("路基材料评分%.1f≥70分", roadScore))
			if gcScore >= 80 {
				alternatives = append(alternatives, "微晶玻璃/铸石")
			}
			alternatives = append(alternatives, "有价金属回收")
		case gcScore >= 80:
			recommended = "推荐微晶玻璃/铸石"
			reasons = append(reasons, fmt.Sprintf("微晶玻璃评分%.1f≥80分，化学组成适宜", gcScore))
			alternatives = append(alternatives, "有价金属回收")
		default:
			recommended = "有价金属回收+填埋"
			reasons = append(reasons, "建材利用评分均不达标，建议提取有价金属后安全填埋")
		}
	}

	return recommended, map[string]interface{}{
		"recommended_use": recommended,
		"reasons":         reasons,
		"alternatives":    alternatives,
		"decision_path": map[string]interface{}{
			"leaching_risk": leachingRisk,
			"cement_score":  common.Round(cementScore, 2),
			"road_score":    common.Round(roadScore, 2),
			"glass_score":   common.Round(gcScore, 2),
		},
	}
}

func (m *SlagReuseAdvisor) GenerateProcessFlow(recommendedUse, leachingRisk string) []map[string]interface{} {
	flow := make([]map[string]interface{}, 0)

	needStabilization := leachingRisk == "极高风险" || leachingRisk == "高风险"

	switch recommendedUse {
	case "稳定化处理→安全填埋/资源化":
		flow = append(flow, map[string]interface{}{
			"step": 1, "desc": "矿渣破碎筛分", "cost": 15.0, "note": "破碎至粒径≤50mm",
		})
		stabilizeCost := 150.0
		if leachingRisk == "极高风险" {
			stabilizeCost = 300.0
		}
		flow = append(flow, map[string]interface{}{
			"step": 2, "desc": "药剂稳定化处理", "cost": stabilizeCost,
			"note": "采用水泥基+螯合剂联合稳定化，成本视污染程度而定",
		})
		flow = append(flow, map[string]interface{}{
			"step": 3, "desc": "养护固化", "cost": 20.0, "note": "养护7-28天至强度达标",
		})
		flow = append(flow, map[string]interface{}{
			"step": 4, "desc": "浸出毒性复检", "cost": 50.0, "note": "送检验证稳定化效果",
		})
		flow = append(flow, map[string]interface{}{
			"step": 5, "desc": "安全填埋/资源化利用", "cost": 80.0,
			"note": "达标后可填埋或进一步资源化，填埋场防渗层(约50元/吨)+长期监测",
		})

	case "优先水泥混合材":
		flow = append(flow, map[string]interface{}{
			"step": 1, "desc": "水淬粒化", "cost": 10.0, "note": "高温熔渣水淬，提高玻璃相含量",
		})
		if needStabilization {
			flow = append(flow, map[string]interface{}{
				"step": 2, "desc": "稳定化预处理", "cost": 80.0, "note": "重金属超标的情况下需先稳定化",
			})
		}
		flow = append(flow, map[string]interface{}{
			"step": 3, "desc": "干燥与粉磨", "cost": 40.0, "note": "粉磨至比表面积≥350m²/kg",
		})
		flow = append(flow, map[string]interface{}{
			"step": 4, "desc": "活性激发改性", "cost": 30.0, "note": "添加硫酸盐、碱激发剂提升活性",
		})
		flow = append(flow, map[string]interface{}{
			"step": 5, "desc": "质量检测与均化", "cost": 15.0, "note": "按GB/T 18046-2017检测活性指数、流动度等",
		})

	case "优先道路基层材料":
		flow = append(flow, map[string]interface{}{
			"step": 1, "desc": "破碎筛分", "cost": 15.0, "note": "按级配要求筛分(0-5mm, 5-20mm, 20-40mm)",
		})
		if needStabilization {
			flow = append(flow, map[string]interface{}{
				"step": 2, "desc": "水泥/石灰稳定化", "cost": 50.0, "note": "掺加3-8%水泥或石灰，改善工程性质并固化重金属",
			})
		}
		flow = append(flow, map[string]interface{}{
			"step": 3, "desc": "级配拌合", "cost": 10.0, "note": "按JTGT F20-2015要求设计级配",
		})
		flow = append(flow, map[string]interface{}{
			"step": 4, "desc": "压实成型", "cost": 5.0, "note": "现场压实度≥95%",
		})
		flow = append(flow, map[string]interface{}{
			"step": 5, "desc": "质量检测", "cost": 20.0, "note": "检测CBR、压实度、压碎值、浸出毒性",
		})

	case "推荐微晶玻璃/铸石":
		flow = append(flow, map[string]interface{}{
			"step": 1, "desc": "矿渣预处理", "cost": 20.0, "note": "破碎、除铁、筛分",
		})
		flow = append(flow, map[string]interface{}{
			"step": 2, "desc": "配料与熔融", "cost": 120.0, "note": "添加SiO2、CaO等调整成分，1400-1500℃熔融",
		})
		flow = append(flow, map[string]interface{}{
			"step": 3, "desc": "成型与晶化", "cost": 80.0, "note": "浇铸成型+可控晶化热处理",
		})
		flow = append(flow, map[string]interface{}{
			"step": 4, "desc": "退火与加工", "cost": 40.0, "note": "消除内应力，切割打磨成品",
		})
		flow = append(flow, map[string]interface{}{
			"step": 5, "desc": "质量检测", "cost": 30.0, "note": "检测抗压强度、耐磨耐腐蚀性",
		})

	default:
		flow = append(flow, map[string]interface{}{
			"step": 1, "desc": "有价金属回收", "cost": 60.0, "note": "磁选回收Fe，浮选/湿法提取Pb、Zn、Cu等",
		})
		flow = append(flow, map[string]interface{}{
			"step": 2, "desc": "尾渣稳定化", "cost": 100.0, "note": "提取金属后尾渣进行稳定化处理",
		})
		flow = append(flow, map[string]interface{}{
			"step": 3, "desc": "安全填埋", "cost": 80.0,
			"note": "防渗层(约50元/吨)+覆土+长期环境监测(约30元/吨·年)",
		})
	}

	return flow
}

func (m *SlagReuseAdvisor) assessCementBlendedWithScore(score float64) ([]models.CementStandardCheck, float64, string, string, map[string]interface{}) {
	var grade, feasibility string
	if score >= 85 {
		grade = "S95"
		feasibility = "可行"
	} else if score >= 60 {
		grade = "S75"
		feasibility = "条件可行"
	} else {
		grade = "不合格"
		feasibility = "不可行"
	}
	return nil, score, grade, feasibility, nil
}

func (m *SlagReuseAdvisor) assessRoadBaseWithScore(score float64) ([]models.RoadStandardCheck, float64, string, string, map[string]interface{}) {
	overallGrade := "三级"
	feasibility := "条件可行"
	if score >= 85 {
		overallGrade = "一级"
		feasibility = "可行"
	} else if score >= 70 {
		overallGrade = "二级"
		feasibility = "可行"
	} else if score < 60 {
		feasibility = "不可行"
	}
	return nil, score, overallGrade, feasibility, nil
}

func (m *SlagReuseAdvisor) SimulateAcceleratedAging(
	slag *models.SlagComposition,
	cementScore, roadScore float64,
	leachingRisk string,
) *models.AcceleratedTestReport {

	glassPhase := slag.GlassPhase
	if glassPhase < 0 {
		glassPhase = 0
	}
	if glassPhase > 100 {
		glassPhase = 100
	}

	agingFactor := 0.05 + (1.0-glassPhase/100.0)*0.15 + slag.LossOnIgnition*0.003
	if agingFactor < 0.02 {
		agingFactor = 0.02
	}
	if agingFactor > 0.3 {
		agingFactor = 0.3
	}

	testDurationDays := 90
	equivalentYears := 10.0

	act28d := 30 + 1.5*glassPhase
	act90d := act28d * (1.12 - agingFactor*0.5)
	act180d := act90d * (1.05 - agingFactor*0.3)
	act1yr := act180d * (1.02 - agingFactor*0.2)
	if act1yr < act28d*0.6 {
		act1yr = act28d * 0.6
	}
	strengthLossRate := (1.0 - act1yr/act28d) * 100
	if strengthLossRate < 0 {
		strengthLossRate = 0
	}

	expansionRate := 0.02 + slag.LossOnIgnition*0.002
	soundnessOK := expansionRate < 0.1

	baseCBR := 100 + 2.5*(100-slag.Fayalite-slag.Wollastonite-slag.GlassPhase) - 10*slag.LossOnIgnition
	baseCBR = math.Max(20, baseCBR)
	freezethawLoss100 := 2.0 + slag.LossOnIgnition*0.5 + (100.0-glassPhase)*0.03
	wetdryLoss50 := 1.5 + slag.LossOnIgnition*0.3
	abrasionLoss100k := 5.0 + (100.0-(slag.SiO2+slag.Al2O3))*0.2
	residualCBR := baseCBR * math.Max(0.4, 1.0-freezethawLoss100/100.0-wetdryLoss50/100.0)

	longTermGrade := "三级"
	if residualCBR >= 80 && freezethawLoss100 < 8 {
		longTermGrade = "一级"
	} else if residualCBR >= 60 && freezethawLoss100 < 12 {
		longTermGrade = "二级"
	}

	initialLeaching := map[string]float64{
		"Pb": slag.PbLeaching,
		"Cd": slag.CdLeaching,
		"As": slag.AsLeaching,
		"Hg": slag.HgLeaching,
		"Cr": slag.CrLeaching,
		"Ni": slag.NiLeaching,
	}

	mobilizationFactor := map[string]float64{
		"Pb": 1.3 + 0.01*slag.GlassPhase,
		"Cd": 1.5 + 0.01*slag.GlassPhase,
		"As": 1.2 + 0.008*slag.GlassPhase,
		"Hg": 1.1 + 0.005*slag.GlassPhase,
		"Cr": 1.25 + 0.008*slag.GlassPhase,
		"Ni": 1.35 + 0.009*slag.GlassPhase,
	}

	after1yr := make(map[string]float64)
	after10yr := make(map[string]float64)
	for metal, init := range initialLeaching {
		mf := mobilizationFactor[metal]
		after1yr[metal] = common.Round(init*math.Pow(mf, 0.1), 5)
		after10yr[metal] = common.Round(init*math.Pow(mf, 1.0), 5)
	}

	leachingLimits := map[string]float64{
		"Pb": m.Std.LeachingPbMax, "Cd": m.Std.LeachingCdMax,
		"As": m.Std.LeachingAsMax, "Hg": m.Std.LeachingHgMax,
		"Cr": m.Std.LeachingCrMax, "Ni": m.Std.LeachingNiMax,
	}
	exceedCount := 0
	severeHgAs := false
	for metal, v10 := range after10yr {
		limit := leachingLimits[metal]
		if limit > 0 && v10 > limit {
			exceedCount++
			if (metal == "Hg" || metal == "As") && v10 > limit*5 {
				severeHgAs = true
			}
		}
	}
	riskAfterAging := "低风险"
	switch {
	case exceedCount >= 5 || severeHgAs:
		riskAfterAging = "极高风险"
	case exceedCount >= 3:
		riskAfterAging = "高风险"
	case exceedCount >= 1:
		riskAfterAging = "中风险"
	}

	reliabilityNote := "模拟结果基于加速老化外推，建议补充实际长期耐久性试验验证"
	if agingFactor > 0.2 {
		reliabilityNote = "该矿渣老化速率较快，长期性能衰减风险较大，强烈建议开展实测试验"
	}

	return &models.AcceleratedTestReport{
		TestMethod: "Arrhenius加速老化 + 多场耦合模拟",
		TestConditions: map[string]interface{}{
			"temperature_c":         60,
			"relative_humidity_pct": 90,
			"freeze_thaw_cycles":    100,
			"wet_dry_cycles":        50,
			"aging_factor":          common.Round(agingFactor, 4),
		},
		CementLongTerm: &models.CementLongTermResult{
			Activity90d:      common.Round(act90d, 2),
			Activity180d:     common.Round(act180d, 2),
			Activity1yr:      common.Round(act1yr, 2),
			StrengthLossRate: common.Round(strengthLossRate, 2),
			SoundnessOK:      soundnessOK,
			ExpansionRatePct: common.Round(expansionRate, 4),
		},
		RoadDurability: &models.RoadDurabilityResult{
			FreezeThaw100Cycles: common.Round(freezethawLoss100, 2),
			WetDry50Cycles:      common.Round(wetdryLoss50, 2),
			Abrasion100000Pass:  common.Round(abrasionLoss100k, 2),
			ResidualCBR:         common.Round(residualCBR, 2),
			LongTermGrade:       longTermGrade,
		},
		LeachingLongTerm: &models.LeachingLongTermResult{
			After1yrWetDry:        after1yr,
			After10yrExtrapolated: after10yr,
			MobilizationFactor:    mobilizationFactor,
			RiskLevelAfterAging:   riskAfterAging,
		},
		AgingFactor:      common.Round(agingFactor, 4),
		TestDurationDays: testDurationDays,
		EquivalentYears:  equivalentYears,
		ReliabilityNote:  reliabilityNote,
	}
}

func (m *SlagReuseAdvisor) ComputeConservativeEstimate(
	slag *models.SlagComposition,
	cementScore, roadScore float64,
	leachingRisk, cementGrade string,
	accel *models.AcceleratedTestReport,
) *models.ConservativeEstimate {

	uncertaintyFactors := make([]string, 0)
	dataGapWarning := ""

	requiredFields := []float64{
		slag.SiO2, slag.Al2O3, slag.CaO, slag.FeO, slag.GlassPhase,
		slag.SpecificSurface, slag.LossOnIgnition,
	}
	missingFields := 0
	for _, f := range requiredFields {
		if f <= 0 {
			missingFields++
		}
	}
	if missingFields >= 3 {
		uncertaintyFactors = append(uncertaintyFactors,
			fmt.Sprintf("矿渣成分数据缺失%d项以上", missingFields))
	}

	if leachingRisk == "中风险" || leachingRisk == "高风险" || leachingRisk == "极高风险" {
		uncertaintyFactors = append(uncertaintyFactors,
			fmt.Sprintf("浸出毒性为%s，长期溶出存在不确定性", leachingRisk))
	}

	if accel != nil && accel.CementLongTerm != nil && accel.CementLongTerm.StrengthLossRate > 15 {
		uncertaintyFactors = append(uncertaintyFactors,
			fmt.Sprintf("预测1年强度损失率%.1f%%超过安全阈值15%%", accel.CementLongTerm.StrengthLossRate))
	}

	if accel != nil && accel.RoadDurability != nil && accel.RoadDurability.FreezeThaw100Cycles > 8 {
		uncertaintyFactors = append(uncertaintyFactors,
			fmt.Sprintf("预测100次冻融损失%.1f%%超过8%%阈值", accel.RoadDurability.FreezeThaw100Cycles))
	}

	if accel != nil && accel.LeachingLongTerm != nil &&
		(accel.LeachingLongTerm.RiskLevelAfterAging == "高风险" ||
			accel.LeachingLongTerm.RiskLevelAfterAging == "极高风险") {
		uncertaintyFactors = append(uncertaintyFactors,
			fmt.Sprintf("预测10年后浸出风险升级为%s", accel.LeachingLongTerm.RiskLevelAfterAging))
	}

	safetyMargin := 15.0
	useConservative := len(uncertaintyFactors) >= 2
	if len(uncertaintyFactors) >= 3 {
		safetyMargin = 25.0
	}
	if len(uncertaintyFactors) >= 4 {
		safetyMargin = 35.0
	}

	conservativeCementScore := math.Max(0, cementScore*(1.0-safetyMargin/100.0))
	conservativeRoadScore := math.Max(0, roadScore*(1.0-safetyMargin/100.0))

	conservativeRecommended := "有价金属回收+填埋"
	if leachingRisk == "低风险" || leachingRisk == "中风险" {
		if conservativeCementScore >= 60 {
			conservativeRecommended = "条件可行：水泥混合材（需长期监测）"
		} else if conservativeRoadScore >= 70 {
			conservativeRecommended = "条件可行：道路基层（需水泥稳定化）"
		}
	}

	precautionMeasures := []string{}
	if leachingRisk != "低风险" {
		precautionMeasures = append(precautionMeasures,
			"进行重金属稳定化预处理，确保浸出毒性长期达标")
	}
	if accel != nil && accel.CementLongTerm != nil && !accel.CementLongTerm.SoundnessOK {
		precautionMeasures = append(precautionMeasures,
			"补充安定性试验（雷氏夹法/试饼法），排除体积膨胀风险")
	}
	if len(uncertaintyFactors) > 0 {
		precautionMeasures = append(precautionMeasures,
			"建议开展3个月以上中试试验，验证实际工程性能")
		precautionMeasures = append(precautionMeasures,
			"工程应用时设置质量监测点，定期抽检活性指数与浸出毒性")
	}

	if len(uncertaintyFactors) > 0 {
		dataGapWarning = fmt.Sprintf("评估存在%d项不确定性因子，采用保守估计（安全余量%.0f%%）",
			len(uncertaintyFactors), safetyMargin)
	} else {
		dataGapWarning = "数据充分，评估结果可靠性高"
	}

	return &models.ConservativeEstimate{
		UseConservativeLimits:      useConservative,
		SafetyMarginPct:            safetyMargin,
		CementScoreConservative:    common.Round(conservativeCementScore, 2),
		RoadScoreConservative:      common.Round(conservativeRoadScore, 2),
		RecommendedUseConservative: conservativeRecommended,
		UncertaintyFactors:         uncertaintyFactors,
		PrecautionMeasures:         precautionMeasures,
		DataGapWarning:             dataGapWarning,
	}
}
