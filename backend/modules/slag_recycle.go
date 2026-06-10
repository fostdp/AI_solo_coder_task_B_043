package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
)

type SlagRecycleModule struct {
	std config.BuildingMaterialStandard
}

func NewSlagRecycleModule() *SlagRecycleModule {
	return &SlagRecycleModule{
		std: config.DefaultBuildingStandard,
	}
}

func (m *SlagRecycleModule) AssessRecycle(ctx context.Context, siteID int, slag *models.SlagComposition) (*models.SlagRecycleResult, error) {
	site, err := repository.GetSiteByID(ctx, siteID)
	siteName := ""
	if err == nil && site != nil {
		siteName = site.Name
	}

	cementChecks, cementScore, cementGrade, cementFeasibility, cementDetails := m.assessCementBlended(slag)
	roadChecks, roadScore, roadGrade, roadFeasibility, roadDetails := m.assessRoadBase(slag)
	otherUses := m.assessOtherUses(slag)
	leachingRisk, leachingDetails := m.assessLeachingRisk(slag)
	recommendedUse, utilizationPlan := m.decideRecommendation(cementScore, roadScore, otherUses, leachingRisk, cementGrade)
	processFlow := m.generateProcessFlow(recommendedUse, leachingRisk)

	assessment := &models.ResourceUtilizationAssessment{
		SiteID:                   siteID,
		MeasurementYear:          slag.MeasurementYear,
		CementBlendedFeasibility: cementFeasibility,
		CementBlendedScore:       round(cementScore, 2),
		CementBlendedGrade:       cementGrade,
		CementDetails:            cementDetails,
		RoadBaseFeasibility:      roadFeasibility,
		RoadBaseScore:            round(roadScore, 2),
		RoadBaseGrade:            roadGrade,
		RoadDetails:              roadDetails,
		OtherUses:                otherUses,
		LeachingRiskLevel:        leachingRisk,
		LeachingRiskDetails:      leachingDetails,
		RecommendedUse:           recommendedUse,
		UtilizationPlan:          utilizationPlan,
	}

	return &models.SlagRecycleResult{
		SiteID:       siteID,
		SiteName:     siteName,
		Composition:  slag,
		Assessment:   assessment,
		CementChecks: cementChecks,
		RoadChecks:   roadChecks,
		ProcessFlow:  processFlow,
	}, nil
}

func (m *SlagRecycleModule) assessCementBlended(slag *models.SlagComposition) ([]models.CementStandardCheck, float64, string, string, map[string]interface{}) {
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
			Value:         round(act7d, 2),
			StandardLimit: m.std.CementS75Activity7dMin,
			Pass:          act7d >= m.std.CementS75Activity7dMin,
			Note:          "GB/T 18046-2017 S75≥55, S95≥75",
		},
		{
			Item:          "活性指数28d",
			Value:         round(act28d, 2),
			StandardLimit: m.std.CementS75Activity28dMin,
			Pass:          act28d >= m.std.CementS75Activity28dMin,
			Note:          "GB/T 18046-2017 S75≥75, S95≥95",
		},
		{
			Item:          "流动度比",
			Value:         round(flowRatio, 2),
			StandardLimit: m.std.CementFlowRatioMin,
			Pass:          flowRatio >= m.std.CementFlowRatioMin,
			Note:          "GB/T 18046-2017 ≥95%",
		},
		{
			Item:          "含水率",
			Value:         round(waterContent, 3),
			StandardLimit: m.std.CementWaterContentMax,
			Pass:          waterContent <= m.std.CementWaterContentMax,
			Note:          "GB/T 18046-2017 ≤1.0%",
		},
		{
			Item:          "烧失量",
			Value:         round(lossOnIgnition, 2),
			StandardLimit: m.std.CementLossOnIgnitionMax,
			Pass:          lossOnIgnition <= m.std.CementLossOnIgnitionMax,
			Note:          "GB/T 18046-2017 ≤3.0%",
		},
		{
			Item:          "比表面积",
			Value:         round(specificSurface, 2),
			StandardLimit: m.std.CementFinenessMin,
			Pass:          specificSurface >= m.std.CementFinenessMin,
			Note:          "GB/T 18046-2017 ≥350m²/kg",
		},
	}

	scoreAct7d := math.Min(act7d/m.std.CementS95Activity7dMin*100, 100)
	scoreAct28d := math.Min(act28d/m.std.CementS95Activity28dMin*100, 100)
	scoreFlow := math.Min(flowRatio/m.std.CementFlowRatioMin*100, 100)
	scoreWater := math.Max(0, (1-waterContent/m.std.CementWaterContentMax)*100)
	scoreLoss := math.Max(0, (1-lossOnIgnition/m.std.CementLossOnIgnitionMax)*100)
	scoreSurface := math.Min(specificSurface/m.std.CementFinenessMin*100, 100)

	totalScore := scoreAct28d*0.4 + scoreAct7d*0.25 + scoreFlow*0.1 + scoreWater*0.1 + scoreLoss*0.1 + scoreSurface*0.05

	var grade, feasibility string
	if totalScore >= 85 {
		if act28d >= m.std.CementS95Activity28dMin && act7d >= m.std.CementS95Activity7dMin {
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
		"basicity":         round(basicity, 4),
		"activity_7d":      round(act7d, 2),
		"activity_28d":     round(act28d, 2),
		"flow_ratio":       round(flowRatio, 2),
		"water_content":    round(waterContent, 3),
		"loss_ignition":    round(lossOnIgnition, 2),
		"specific_surface": round(specificSurface, 2),
		"sub_scores": map[string]float64{
			"act7d":   round(scoreAct7d, 2),
			"act28d":  round(scoreAct28d, 2),
			"flow":    round(scoreFlow, 2),
			"water":   round(scoreWater, 2),
			"loss":    round(scoreLoss, 2),
			"surface": round(scoreSurface, 2),
		},
	}

	return checks, totalScore, grade, feasibility, details
}

func (m *SlagRecycleModule) assessRoadBase(slag *models.SlagComposition) ([]models.RoadStandardCheck, float64, string, string, map[string]interface{}) {
	cbr := 100 + 2.5*(100-slag.Fayalite-slag.Wollastonite-slag.GlassPhase) - 10*slag.LossOnIgnition
	crushValue := 8 + 0.2*(100-slag.GlassPhase)
	plasticityIdx := 2 + 0.05*(100-slag.SiO2)
	freezeThawLoss := 0.5 + 0.1*slag.LossOnIgnition
	abrasion := 5 + 0.15*(100 - (slag.SiO2 + slag.Al2O3))

	cbrGrade := "三级"
	if cbr >= m.std.RoadCBRGrade1Min {
		cbrGrade = "一级"
	} else if cbr >= m.std.RoadCBRGrade2Min {
		cbrGrade = "二级"
	}

	checks := []models.RoadStandardCheck{
		{
			Item:          "CBR值",
			Value:         round(cbr, 2),
			StandardLimit: m.std.RoadCBRGrade3Min,
			Pass:          cbr >= m.std.RoadCBRGrade3Min,
			Grade:         cbrGrade,
		},
		{
			Item:          "压碎值",
			Value:         round(crushValue, 2),
			StandardLimit: m.std.RoadCrushValueMax,
			Pass:          crushValue <= m.std.RoadCrushValueMax,
			Grade:         gradeFromValue(crushValue, 26, 30, 35),
		},
		{
			Item:          "塑性指数",
			Value:         round(plasticityIdx, 2),
			StandardLimit: m.std.RoadPlasticityIdxMax,
			Pass:          plasticityIdx <= m.std.RoadPlasticityIdxMax,
			Grade:         gradeFromValue(plasticityIdx, 9, 12, 15),
		},
		{
			Item:          "冻融损失",
			Value:         round(freezeThawLoss, 3),
			StandardLimit: m.std.RoadFreezeThawLossMax,
			Pass:          freezeThawLoss <= m.std.RoadFreezeThawLossMax,
			Grade:         gradeFromValue(freezeThawLoss, 5, 8, 12),
		},
		{
			Item:          "磨耗率",
			Value:         round(abrasion, 2),
			StandardLimit: m.std.RoadAbrasionMax,
			Pass:          abrasion <= m.std.RoadAbrasionMax,
			Grade:         gradeFromValue(abrasion, 15, 20, 30),
		},
	}

	scoreCBR := math.Min(cbr/m.std.RoadCBRGrade1Min*100, 100)
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
		"cbr":              round(cbr, 2),
		"crush_value":      round(crushValue, 2),
		"plasticity_index": round(plasticityIdx, 2),
		"freeze_thaw_loss": round(freezeThawLoss, 3),
		"abrasion_rate":    round(abrasion, 2),
		"sub_scores": map[string]float64{
			"cbr":      round(scoreCBR, 2),
			"crush":    round(scoreCrush, 2),
			"pi":       round(scorePI, 2),
			"freeze":   round(scoreFreeze, 2),
			"abrasion": round(scoreAbrasion, 2),
		},
	}

	return checks, totalScore, overallGrade, feasibility, details
}

func (m *SlagRecycleModule) assessOtherUses(slag *models.SlagComposition) map[string]interface{} {
	crushValue := 8 + 0.2*(100-slag.GlassPhase)
	leachingPass := slag.PbLeaching <= m.std.LeachingPbMax &&
		slag.CdLeaching <= m.std.LeachingCdMax &&
		slag.AsLeaching <= m.std.LeachingAsMax &&
		slag.HgLeaching <= m.std.LeachingHgMax &&
		slag.CrLeaching <= m.std.LeachingCrMax &&
		slag.NiLeaching <= m.std.LeachingNiMax

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
			"score":       round(aggregateScore, 2),
			"density":     slag.Density,
			"crush_value": round(crushValue, 2),
			"leaching_ok": leachingPass,
			"sub_scores": map[string]float64{
				"density":  round(aggDensityScore, 2),
				"crush":    round(aggCrushScore, 2),
				"leaching": round(aggLeachingScore, 2),
			},
		},
		"glass_ceramic": map[string]interface{}{
			"score":            round(glassCeramicScore, 2),
			"sio2_al2o3_total": round(slag.SiO2+slag.Al2O3, 2),
			"feo_color":        round(slag.FeO, 2),
			"can_cast_stone":   glassCeramicScore >= 70,
			"sub_scores": map[string]float64{
				"chemistry": round(glassChemScore, 2),
				"color":     round(glassColorScore, 2),
			},
		},
		"soil_amendment": map[string]interface{}{
			"score":       round(soilAmendmentScore, 2),
			"cao_mgo":     round(slag.CaO+slag.MgO, 2),
			"leaching_ok": leachingPass,
			"sub_scores": map[string]float64{
				"alkalinity": round(soilAlkScore, 2),
				"leaching":   round(soilLeachScore, 2),
			},
		},
		"metal_recovery": map[string]interface{}{
			"score":       round(metalRecoveryScore, 2),
			"note":        metalRecoveryNote,
			"pb_leaching": round(slag.PbLeaching, 4),
			"cr_leaching": round(slag.CrLeaching, 4),
			"ni_leaching": round(slag.NiLeaching, 4),
			"feo_content": round(slag.FeO, 2),
		},
	}
}

func (m *SlagRecycleModule) assessLeachingRisk(slag *models.SlagComposition) (string, map[string]interface{}) {
	leachingMetals := []struct {
		name  string
		value float64
		limit float64
	}{
		{"Pb", slag.PbLeaching, m.std.LeachingPbMax},
		{"Cd", slag.CdLeaching, m.std.LeachingCdMax},
		{"As", slag.AsLeaching, m.std.LeachingAsMax},
		{"Hg", slag.HgLeaching, m.std.LeachingHgMax},
		{"Cr", slag.CrLeaching, m.std.LeachingCrMax},
		{"Ni", slag.NiLeaching, m.std.LeachingNiMax},
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
			"value":        round(mItem.value, 4),
			"limit":        mItem.limit,
			"exceed":       exceed,
			"exceed_ratio": round(mItem.value/mItem.limit, 2),
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

func (m *SlagRecycleModule) decideRecommendation(cementScore, roadScore float64, otherUses map[string]interface{}, leachingRisk, cementGrade string) (string, map[string]interface{}) {
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
			"cement_score":  round(cementScore, 2),
			"road_score":    round(roadScore, 2),
			"glass_score":   round(gcScore, 2),
		},
	}
}

func (m *SlagRecycleModule) generateProcessFlow(recommendedUse, leachingRisk string) []map[string]interface{} {
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

func gradeFromValue(value, g1Limit, g2Limit, g3Limit float64) string {
	if value <= g1Limit {
		return "一级"
	} else if value <= g2Limit {
		return "二级"
	} else if value <= g3Limit {
		return "三级"
	}
	return "不合格"
}

var _ = json.Marshal
