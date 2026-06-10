package modules

import (
	"math"
	"testing"

	"archaeology-pollution-system/models"
)

func TestNewSlagRecycleModule(t *testing.T) {
	m := NewSlagRecycleModule()
	if m == nil {
		t.Fatal("NewSlagRecycleModule 返回 nil")
	}
	if m.std.CementS75Activity28dMin <= 0 {
		t.Error("水泥标准未正确加载")
	}
	if m.std.RoadCBRGrade3Min <= 0 {
		t.Error("路基标准未正确加载")
	}
	if m.std.LeachingPbMax <= 0 {
		t.Error("浸出标准未正确加载")
	}
}

func TestGradeFromValue(t *testing.T) {
	tests := []struct {
		value   float64
		g1      float64
		g2      float64
		g3      float64
		expect  string
	}{
		{10, 26, 30, 35, "一级"},
		{26, 26, 30, 35, "一级"},
		{28, 26, 30, 35, "二级"},
		{30, 26, 30, 35, "二级"},
		{32, 26, 30, 35, "三级"},
		{35, 26, 30, 35, "三级"},
		{40, 26, 30, 35, "不合格"},
		{0, 9, 12, 15, "一级"},
		{9, 9, 12, 15, "一级"},
		{10, 9, 12, 15, "二级"},
	}

	for _, tt := range tests {
		result := gradeFromValue(tt.value, tt.g1, tt.g2, tt.g3)
		if result != tt.expect {
			t.Errorf("gradeFromValue(%.1f, %.1f, %.1f, %.1f) = %s，期望 %s",
				tt.value, tt.g1, tt.g2, tt.g3, result, tt.expect)
		}
	}
}

func TestAssessCementBlended(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("优质矿渣 → S95/S105级", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:          35,
			Al2O3:        15,
			CaO:          40,
			FeO:          8,
			Fe2O3:         3,
			MgO:           8,
			GlassPhase:    85,
			LossOnIgnition: 0.5,
			SpecificSurface: 450,
		}
		checks, score, grade, feasibility, _ := m.assessCementBlended(slag)
		if len(checks) != 6 {
			t.Errorf("检查项数=%d，期望6", len(checks))
		}
		if score <= 0 || score > 100 {
			t.Errorf("评分%.2f 超出 [0,100] 范围", score)
		}
		if grade == "" {
			t.Error("等级为空")
		}
		if feasibility == "" {
			t.Error("可行性结论为空")
		}
		t.Logf("优质矿渣: 评分=%.2f, 等级=%s, 可行性=%s", score, grade, feasibility)
		for _, c := range checks {
			t.Logf("  %s: %.2f / %.2f, 通过=%v", c.Item, c.Value, c.StandardLimit, c.Pass)
		}
	})

	t.Run("低活性矿渣 → 不合格", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:          50,
			Al2O3:        20,
			CaO:          10,
			FeO:          25,
			Fe2O3:        10,
			MgO:           5,
			GlassPhase:    20,
			LossOnIgnition: 5.0,
			SpecificSurface: 200,
		}
		checks, score, grade, feasibility, _ := m.assessCementBlended(slag)
		if len(checks) != 6 {
			t.Errorf("检查项数=%d，期望6", len(checks))
		}
		t.Logf("低活性矿渣: 评分=%.2f, 等级=%s, 可行性=%s", score, grade, feasibility)
	})

	t.Run("活性指数计算验证", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:          30,
			Al2O3:        10,
			CaO:          35,
			FeO:          5,
			Fe2O3:         2,
			GlassPhase:    60,
		}
		checks, _, _, _, _ := m.assessCementBlended(slag)
		var act28d float64
		for _, c := range checks {
			if c.Item == "活性指数28d" {
				act28d = c.Value
				break
			}
		}
		expected28d := 30 + 1.5*slag.GlassPhase + 20*math.Min((slag.CaO+slag.MgO)/(slag.SiO2+slag.Al2O3), 1.0)
		if math.Abs(act28d-expected28d) > 0.1 {
			t.Errorf("28d活性指数=%.2f，期望%.2f", act28d, expected28d)
		}
	})

	t.Run("高玻璃相 → 高活性", func(t *testing.T) {
		highGlass := &models.SlagComposition{
			SiO2:          35,
			Al2O3:        12,
			CaO:          40,
			GlassPhase:    90,
			LossOnIgnition: 0.3,
			SpecificSurface: 450,
		}
		lowGlass := &models.SlagComposition{
			SiO2:          35,
			Al2O3:        12,
			CaO:          40,
			GlassPhase:    30,
			LossOnIgnition: 0.3,
			SpecificSurface: 450,
		}
		_, highScore, _, _, _ := m.assessCementBlended(highGlass)
		_, lowScore, _, _, _ := m.assessCementBlended(lowGlass)
		if highScore <= lowScore {
			t.Errorf("高玻璃相评分(%.2f)应高于低玻璃相评分(%.2f)", highScore, lowScore)
		}
	})
}

func TestAssessRoadBase(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("坚硬矿渣 → 一级路基", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:          40,
			Al2O3:        15,
			FeO:          5,
			Fe2O3:         3,
			Fayalite:     15,
			Wollastonite:  10,
			GlassPhase:    40,
			LossOnIgnition: 0.5,
			Density:       3.5,
		}
		checks, score, grade, feasibility, _ := m.assessRoadBase(slag)
		if len(checks) != 5 {
			t.Errorf("检查项数=%d，期望5", len(checks))
		}
		t.Logf("坚硬矿渣路基: 评分=%.2f, 等级=%s, 可行性=%s", score, grade, feasibility)
		for _, c := range checks {
			t.Logf("  %s: %.2f / %.2f, 等级=%s, 通过=%v",
				c.Item, c.Value, c.StandardLimit, c.Grade, c.Pass)
		}
	})

	t.Run("软质矿渣 → 不可行", func(t *testing.T) {
		slag := &models.SlagComposition{
			GlassPhase:    90,
			LossOnIgnition: 8.0,
			SiO2:          25,
		}
		_, score, _, feasibility, _ := m.assessRoadBase(slag)
		t.Logf("软质矿渣路基: 评分=%.2f, 可行性=%s", score, feasibility)
	})

	t.Run("CBR值计算验证", func(t *testing.T) {
		slag := &models.SlagComposition{
			Fayalite:     20,
			Wollastonite:  15,
			GlassPhase:    50,
			LossOnIgnition: 1.0,
		}
		checks, _, _, _, _ := m.assessRoadBase(slag)
		var cbr float64
		for _, c := range checks {
			if c.Item == "CBR值" {
				cbr = c.Value
				break
			}
		}
		expectedCBR := 100 + 2.5*(100-slag.Fayalite-slag.Wollastonite-slag.GlassPhase) - 10*slag.LossOnIgnition
		if math.Abs(cbr-expectedCBR) > 0.1 {
			t.Errorf("CBR=%.2f，期望%.2f", cbr, expectedCBR)
		}
	})
}

func TestAssessOtherUses(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("多用途评估完整性", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:          35,
			Al2O3:        15,
			CaO:          30,
			FeO:          10,
			MgO:           5,
			PbLeaching:    0.5,
			CdLeaching:    0.05,
			AsLeaching:    0.2,
			HgLeaching:    0.01,
			CrLeaching:    0.3,
			NiLeaching:    0.1,
			GlassPhase:    60,
			Density:       3.2,
		}
		otherUses := m.assessOtherUses(slag)

		requiredKeys := []string{"concrete_aggregate", "glass_ceramic", "soil_amendment", "metal_recovery"}
		for _, key := range requiredKeys {
			if _, ok := otherUses[key]; !ok {
				t.Errorf("缺少 %s 用途评估", key)
			}
		}

		for key, val := range otherUses {
			if m, ok := val.(map[string]interface{}); ok {
				if _, hasScore := m["score"]; !hasScore {
					t.Errorf("%s 评估缺少 score 字段", key)
				}
			} else {
				t.Errorf("%s 评估不是 map 类型", key)
			}
		}

		t.Log("其他用途评估:")
		for key, val := range otherUses {
			if m, ok := val.(map[string]interface{}); ok {
				if score, ok := m["score"].(float64); ok {
					t.Logf("  %s: 评分=%.1f", key, score)
				}
			}
		}
	})

	t.Run("高铁矿渣 → 金属回收价值高", func(t *testing.T) {
		highFe := &models.SlagComposition{
			FeO:        40,
			PbLeaching: 2.0,
			GlassPhase:  30,
		}
		lowFe := &models.SlagComposition{
			FeO:        5,
			PbLeaching: 0.1,
			GlassPhase:  30,
		}
		highUses := m.assessOtherUses(highFe)
		lowUses := m.assessOtherUses(lowFe)

		highScore := 0.0
		lowScore := 0.0
		if m, ok := highUses["metal_recovery"].(map[string]interface{}); ok {
			highScore = m["score"].(float64)
		}
		if m, ok := lowUses["metal_recovery"].(map[string]interface{}); ok {
			lowScore = m["score"].(float64)
		}

		if highScore <= lowScore {
			t.Errorf("高铁矿渣金属回收评分(%.1f)应高于低铁矿渣(%.1f)", highScore, lowScore)
		}
	})
}

func TestAssessLeachingRisk(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("低浸出 → 低风险", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 0.5,
			CdLeaching: 0.05,
			AsLeaching: 0.3,
			HgLeaching: 0.01,
			CrLeaching: 0.2,
			NiLeaching: 0.1,
		}
		riskLevel, details := m.assessLeachingRisk(slag)
		if riskLevel != "低风险" {
			t.Errorf("低浸出风险等级='%s'，期望'低风险'", riskLevel)
		}
		if details == nil {
			t.Error("详情不应为nil")
		}
		if details["exceed_count"] != 0 {
			t.Errorf("超标数=%v，期望0", details["exceed_count"])
		}
		t.Logf("低浸出: 风险=%s, 超标数=%v", riskLevel, details["exceed_count"])
	})

	t.Run("多种金属超标 → 高/极高风险", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 5.0,
			CdLeaching: 1.0,
			AsLeaching: 3.0,
			HgLeaching: 0.1,
			CrLeaching: 10.0,
			NiLeaching: 10.0,
		}
		riskLevel, details := m.assessLeachingRisk(slag)
		if riskLevel != "极高风险" && riskLevel != "高风险" {
			t.Errorf("多超标风险等级='%s'，期望'高风险'或'极高风险'", riskLevel)
		}
		exceedCount, _ := details["exceed_count"].(int)
		if exceedCount < 5 {
			t.Errorf("超标数=%d，应≥5", exceedCount)
		}
		t.Logf("多超标: 风险=%s, 超标数=%d", riskLevel, exceedCount)
	})

	t.Run("Hg/As严重超标 → 极高风险", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 0.1,
			CdLeaching: 0.01,
			AsLeaching: 10.0,
			HgLeaching: 0.5,
			CrLeaching: 0.1,
			NiLeaching: 0.1,
		}
		riskLevel, details := m.assessLeachingRisk(slag)
		severeHgAs, _ := details["severe_hg_as"].(bool)
		if !severeHgAs {
			t.Error("Hg/As严重超标时 severe_hg_as 应为 true")
		}
		if riskLevel != "极高风险" {
			t.Errorf("Hg/As严重超标风险='%s'，期望'极高风险'", riskLevel)
		}
		t.Logf("Hg/As严重超标: 风险=%s, severe_hg_as=%v", riskLevel, severeHgAs)
	})

	t.Run("单种金属超标 → 中风险", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 4.0,
			CdLeaching: 0.01,
			AsLeaching: 0.1,
			HgLeaching: 0.01,
			CrLeaching: 0.1,
			NiLeaching: 0.1,
		}
		riskLevel, _ := m.assessLeachingRisk(slag)
		if riskLevel != "中风险" {
			t.Errorf("单超标风险='%s'，期望'中风险'", riskLevel)
		}
	})

	t.Run("浸出标准验证", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: m.std.LeachingPbMax,
			CdLeaching: m.std.LeachingCdMax,
			AsLeaching: m.std.LeachingAsMax,
			HgLeaching: m.std.LeachingHgMax,
			CrLeaching: m.std.LeachingCrMax,
			NiLeaching: m.std.LeachingNiMax,
		}
		riskLevel, details := m.assessLeachingRisk(slag)
		exceedCount, _ := details["exceed_count"].(int)
		if exceedCount != 0 {
			t.Errorf("刚好等于标准值时超标数=%d，应按通过算(0)", exceedCount)
		}
		t.Logf("标准值处: 风险=%s, 超标数=%d", riskLevel, exceedCount)
	})
}

func TestDecideRecommendation(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("低浸出+高水泥评分 → 优先水泥", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 50.0},
		}
		recommended, details := m.decideRecommendation(85, 60, otherUses, "低风险", "S95")
		if recommended != "优先水泥混合材" {
			t.Errorf("推荐='%s'，期望'优先水泥混合材'", recommended)
		}
		if len(details["reasons"].([]string)) == 0 {
			t.Error("推荐理由为空")
		}
		t.Logf("推荐: %s", recommended)
		for _, r := range details["reasons"].([]string) {
			t.Logf("  - %s", r)
		}
	})

	t.Run("低浸出+低水泥+高路基 → 优先道路", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 60.0},
		}
		recommended, _ := m.decideRecommendation(40, 80, otherUses, "低风险", "不合格")
		if recommended != "优先道路基层材料" {
			t.Errorf("推荐='%s'，期望'优先道路基层材料'", recommended)
		}
		t.Logf("推荐: %s", recommended)
	})

	t.Run("低浸出+低水泥+低路基+高玻璃 → 微晶玻璃", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 90.0},
		}
		recommended, _ := m.decideRecommendation(30, 50, otherUses, "低风险", "不合格")
		if recommended != "推荐微晶玻璃/铸石" {
			t.Errorf("推荐='%s'，期望'推荐微晶玻璃/铸石'", recommended)
		}
		t.Logf("推荐: %s", recommended)
	})

	t.Run("高浸出 → 稳定化处理", func(t *testing.T) {
		otherUses := map[string]interface{}{}
		recommended, details := m.decideRecommendation(70, 80, otherUses, "高风险", "S75")
		if recommended != "稳定化处理→安全填埋/资源化" {
			t.Errorf("高浸出推荐='%s'，期望'稳定化处理→安全填埋/资源化'", recommended)
		}
		decPath, _ := details["decision_path"].(map[string]interface{})
		if decPath["leaching_risk"] != "高风险" {
			t.Error("决策路径中浸出风险不正确")
		}
		t.Logf("高浸出推荐: %s", recommended)
	})

	t.Run("都不达标 → 金属回收+填埋", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 30.0},
		}
		recommended, _ := m.decideRecommendation(30, 40, otherUses, "低风险", "不合格")
		if recommended != "有价金属回收+填埋" {
			t.Errorf("都不达标推荐='%s'，期望'有价金属回收+填埋'", recommended)
		}
		t.Logf("都不达标推荐: %s", recommended)
	})
}

func TestGenerateProcessFlow(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("水泥混合材流程", func(t *testing.T) {
		flow := m.generateProcessFlow("优先水泥混合材", "低风险")
		if len(flow) < 3 {
			t.Errorf("流程步骤数=%d，应≥3", len(flow))
		}
		hasStep1 := false
		for _, step := range flow {
			if step["step"].(int) == 1 {
				hasStep1 = true
			}
		}
		if !hasStep1 {
			t.Error("缺少步骤1")
		}
		t.Logf("水泥混合材流程: %d 步", len(flow))
		for _, step := range flow {
			t.Logf("  步骤%d: %s (%.0f元/吨)", step["step"].(int), step["desc"].(string), step["cost"].(float64))
		}
	})

	t.Run("稳定化流程", func(t *testing.T) {
		flow := m.generateProcessFlow("稳定化处理→安全填埋/资源化", "高风险")
		if len(flow) < 4 {
			t.Errorf("稳定化流程步骤数=%d，应≥4", len(flow))
		}
		hasStabilize := false
		for _, step := range flow {
			if step["desc"].(string) == "药剂稳定化处理" {
				hasStabilize = true
			}
		}
		if !hasStabilize {
			t.Error("稳定化流程应包含药剂稳定化处理步骤")
		}
	})

	t.Run("道路基层流程", func(t *testing.T) {
		flow := m.generateProcessFlow("优先道路基层材料", "低风险")
		if len(flow) < 3 {
			t.Errorf("路基流程步骤数=%d，应≥3", len(flow))
		}
	})

	t.Run("微晶玻璃流程", func(t *testing.T) {
		flow := m.generateProcessFlow("推荐微晶玻璃/铸石", "低风险")
		if len(flow) < 4 {
			t.Errorf("微晶玻璃流程步骤数=%d，应≥4", len(flow))
		}
	})

	t.Run("默认流程（金属回收+填埋）", func(t *testing.T) {
		flow := m.generateProcessFlow("未知方案", "低风险")
		if len(flow) == 0 {
			t.Error("默认流程不应为空")
		}
	})

	t.Run("高浸出水泥流程应含稳定化步骤", func(t *testing.T) {
		flow := m.generateProcessFlow("优先水泥混合材", "高风险")
		hasStabilize := false
		for _, step := range flow {
			desc, ok := step["desc"].(string)
			if ok && desc == "稳定化预处理" {
				hasStabilize = true
				break
			}
		}
		if !hasStabilize {
			t.Error("高浸出的水泥流程应包含稳定化预处理")
		}
	})

	t.Run("低浸出水泥流程不应有稳定化", func(t *testing.T) {
		flow := m.generateProcessFlow("优先水泥混合材", "低风险")
		for _, step := range flow {
			desc, ok := step["desc"].(string)
			if ok && (desc == "稳定化预处理" || desc == "药剂稳定化处理") {
				t.Error("低浸出流程不应包含稳定化步骤")
			}
		}
	})
}

func TestSlagTypeApplicability(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("铜渣 vs 铁渣 适用性差异", func(t *testing.T) {
		copperSlag := &models.SlagComposition{
			SiO2:          35,
			Al2O3:        8,
			CaO:          10,
			FeO:          45,
			Fe2O3:         5,
			GlassPhase:    80,
			LossOnIgnition: 0.5,
			SpecificSurface: 400,
			PbLeaching:    0.3,
			CdLeaching:    0.02,
			AsLeaching:    0.1,
			HgLeaching:    0.005,
			CrLeaching:    0.1,
			NiLeaching:    0.05,
		}
		ironSlag := &models.SlagComposition{
			SiO2:          32,
			Al2O3:        15,
			CaO:          45,
			FeO:          20,
			Fe2O3:         8,
			GlassPhase:    40,
			LossOnIgnition: 1.0,
			SpecificSurface: 350,
			PbLeaching:    0.1,
			CdLeaching:    0.01,
			AsLeaching:    0.05,
			HgLeaching:    0.002,
			CrLeaching:    0.05,
			NiLeaching:    0.02,
		}

		_, cuCementScore, cuCementGrade, cuCementFeas, _ := m.assessCementBlended(copperSlag)
		_, feCementScore, feCementGrade, feCementFeas, _ := m.assessCementBlended(ironSlag)
		t.Logf("铜渣水泥: 评分=%.2f, 等级=%s, 可行性=%s", cuCementScore, cuCementGrade, cuCementFeas)
		t.Logf("铁渣水泥: 评分=%.2f, 等级=%s, 可行性=%s", feCementScore, feCementGrade, feCementFeas)

		_, cuRoadScore, cuRoadGrade, cuRoadFeas, _ := m.assessRoadBase(copperSlag)
		_, feRoadScore, feRoadGrade, feRoadFeas, _ := m.assessRoadBase(ironSlag)
		t.Logf("铜渣路基: 评分=%.2f, 等级=%s, 可行性=%s", cuRoadScore, cuRoadGrade, cuRoadFeas)
		t.Logf("铁渣路基: 评分=%.2f, 等级=%s, 可行性=%s", feRoadScore, feRoadGrade, feRoadFeas)

		cuOther := m.assessOtherUses(copperSlag)
		feOther := m.assessOtherUses(ironSlag)
		if gc, ok := cuOther["glass_ceramic"].(map[string]interface{}); ok {
			t.Logf("铜渣微晶玻璃评分: %.1f", gc["score"])
		}
		if gc, ok := feOther["glass_ceramic"].(map[string]interface{}); ok {
			t.Logf("铁渣微晶玻璃评分: %.1f", gc["score"])
		}
	})
}

func TestBuildingMaterialStandards(t *testing.T) {
	m := NewSlagRecycleModule()

	t.Run("水泥标准验证", func(t *testing.T) {
		if m.std.CementS75Activity7dMin <= 0 {
			t.Error("S75 7d活性标准缺失")
		}
		if m.std.CementS75Activity28dMin <= 0 {
			t.Error("S75 28d活性标准缺失")
		}
		if m.std.CementFlowRatioMin <= 0 {
			t.Error("流动度比标准缺失")
		}
		if m.std.CementLossOnIgnitionMax <= 0 {
			t.Error("烧失量标准缺失")
		}
		if m.std.CementFinenessMin <= 0 {
			t.Error("比表面积标准缺失")
		}
		t.Logf("水泥标准: S75-7d=%.1f, S75-28d=%.1f, 流动度≥%.1f, 烧失≤%.1f, 比表≥%.1f",
			m.std.CementS75Activity7dMin, m.std.CementS75Activity28dMin,
			m.std.CementFlowRatioMin, m.std.CementLossOnIgnitionMax, m.std.CementFinenessMin)
	})

	t.Run("路基标准验证", func(t *testing.T) {
		if m.std.RoadCBRGrade1Min <= 0 {
			t.Error("一级CBR标准缺失")
		}
		if m.std.RoadCrushValueMax <= 0 {
			t.Error("压碎值标准缺失")
		}
		t.Logf("路基标准: CBR一级≥%.1f, 压碎值≤%.1f, 塑性指数≤%.1f",
			m.std.RoadCBRGrade1Min, m.std.RoadCrushValueMax, m.std.RoadPlasticityIdxMax)
	})

	t.Run("浸出标准验证", func(t *testing.T) {
		if m.std.LeachingPbMax <= 0 {
			t.Error("Pb浸出标准缺失")
		}
		if m.std.LeachingCdMax <= 0 {
			t.Error("Cd浸出标准缺失")
		}
		if m.std.LeachingHgMax <= 0 {
			t.Error("Hg浸出标准缺失")
		}
		t.Logf("浸出标准: Pb≤%.2f, Cd≤%.2f, As≤%.2f, Hg≤%.3f, Cr≤%.2f, Ni≤%.2f",
			m.std.LeachingPbMax, m.std.LeachingCdMax, m.std.LeachingAsMax,
			m.std.LeachingHgMax, m.std.LeachingCrMax, m.std.LeachingNiMax)
	})
}
