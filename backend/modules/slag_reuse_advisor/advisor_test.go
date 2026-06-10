package slag_reuse_advisor_test

import (
	"math"
	"testing"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/common"
	"archaeology-pollution-system/modules/slag_reuse_advisor"
)

func TestNewSlagReuseAdvisor(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()
	if m == nil {
		t.Fatal("NewSlagReuseAdvisor returned nil")
	}
	if m.Std.CementS75Activity28dMin <= 0 {
		t.Error("cement standard not loaded")
	}
	if m.Std.RoadCBRGrade3Min <= 0 {
		t.Error("road standard not loaded")
	}
	if m.Std.LeachingPbMax <= 0 {
		t.Error("leaching standard not loaded")
	}
}

func TestGradeFromValue(t *testing.T) {
	tests := []struct {
		value  float64
		g1     float64
		g2     float64
		g3     float64
		expect string
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
		result := common.GradeFromValue(tt.value, tt.g1, tt.g2, tt.g3)
		if result != tt.expect {
			t.Errorf("GradeFromValue(%.1f, %.1f, %.1f, %.1f) = %s, want %s",
				tt.value, tt.g1, tt.g2, tt.g3, result, tt.expect)
		}
	}
}

func TestAssessCementBlended(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("high quality slag", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           15,
			CaO:             40,
			FeO:             8,
			Fe2O3:           3,
			MgO:             8,
			GlassPhase:      85,
			LossOnIgnition:  0.5,
			SpecificSurface: 450,
		}
		checks, score, grade, feasibility, _ := m.AssessCementBlended(slag)
		if len(checks) != 6 {
			t.Errorf("check count=%d, want 6", len(checks))
		}
		if score <= 0 || score > 100 {
			t.Errorf("score %.2f out of [0,100]", score)
		}
		if grade == "" {
			t.Error("grade is empty")
		}
		if feasibility == "" {
			t.Error("feasibility is empty")
		}
		t.Logf("high quality: score=%.2f, grade=%s, feasibility=%s", score, grade, feasibility)
		for _, c := range checks {
			t.Logf("  %s: %.2f / %.2f, pass=%v", c.Item, c.Value, c.StandardLimit, c.Pass)
		}
	})

	t.Run("low activity slag", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            50,
			Al2O3:           20,
			CaO:             10,
			FeO:             25,
			Fe2O3:           10,
			MgO:             5,
			GlassPhase:      20,
			LossOnIgnition:  5.0,
			SpecificSurface: 200,
		}
		checks, score, grade, feasibility, _ := m.AssessCementBlended(slag)
		if len(checks) != 6 {
			t.Errorf("check count=%d, want 6", len(checks))
		}
		t.Logf("low activity: score=%.2f, grade=%s, feasibility=%s", score, grade, feasibility)
	})

	t.Run("activity index verification", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:       30,
			Al2O3:      10,
			CaO:        35,
			FeO:        5,
			Fe2O3:      2,
			GlassPhase: 60,
		}
		checks, _, _, _, _ := m.AssessCementBlended(slag)
		var act28d float64
		for _, c := range checks {
			if c.Item == "活性指数28d" {
				act28d = c.Value
				break
			}
		}
		expected28d := 30 + 1.5*slag.GlassPhase + 20*math.Min((slag.CaO+slag.MgO)/(slag.SiO2+slag.Al2O3), 1.0)
		if math.Abs(act28d-expected28d) > 0.1 {
			t.Errorf("28d activity=%.2f, want %.2f", act28d, expected28d)
		}
	})

	t.Run("high glass phase vs low glass phase", func(t *testing.T) {
		highGlass := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           12,
			CaO:             40,
			GlassPhase:      90,
			LossOnIgnition:  0.3,
			SpecificSurface: 450,
		}
		lowGlass := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           12,
			CaO:             40,
			GlassPhase:      30,
			LossOnIgnition:  0.3,
			SpecificSurface: 450,
		}
		_, highScore, _, _, _ := m.AssessCementBlended(highGlass)
		_, lowScore, _, _, _ := m.AssessCementBlended(lowGlass)
		if highScore <= lowScore {
			t.Errorf("high glass score(%.2f) should be > low glass score(%.2f)", highScore, lowScore)
		}
	})
}

func TestAssessRoadBase(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("hard slag grade 1 road base", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:           40,
			Al2O3:          15,
			FeO:            5,
			Fe2O3:          3,
			Fayalite:       15,
			Wollastonite:   10,
			GlassPhase:     40,
			LossOnIgnition: 0.5,
			Density:        3.5,
		}
		checks, score, grade, feasibility, _ := m.AssessRoadBase(slag)
		if len(checks) != 5 {
			t.Errorf("check count=%d, want 5", len(checks))
		}
		t.Logf("hard slag road base: score=%.2f, grade=%s, feasibility=%s", score, grade, feasibility)
		for _, c := range checks {
			t.Logf("  %s: %.2f / %.2f, grade=%s, pass=%v",
				c.Item, c.Value, c.StandardLimit, c.Grade, c.Pass)
		}
	})

	t.Run("soft slag not feasible", func(t *testing.T) {
		slag := &models.SlagComposition{
			GlassPhase:     90,
			LossOnIgnition: 8.0,
			SiO2:           25,
		}
		_, score, _, feasibility, _ := m.AssessRoadBase(slag)
		t.Logf("soft slag road base: score=%.2f, feasibility=%s", score, feasibility)
	})

	t.Run("CBR calculation verification", func(t *testing.T) {
		slag := &models.SlagComposition{
			Fayalite:       20,
			Wollastonite:   15,
			GlassPhase:     50,
			LossOnIgnition: 1.0,
		}
		checks, _, _, _, _ := m.AssessRoadBase(slag)
		var cbr float64
		for _, c := range checks {
			if c.Item == "CBR值" {
				cbr = c.Value
				break
			}
		}
		expectedCBR := 100 + 2.5*(100-slag.Fayalite-slag.Wollastonite-slag.GlassPhase) - 10*slag.LossOnIgnition
		if math.Abs(cbr-expectedCBR) > 0.1 {
			t.Errorf("CBR=%.2f, want %.2f", cbr, expectedCBR)
		}
	})
}

func TestAssessOtherUses(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("completeness of multi-use assessment", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:       35,
			Al2O3:      15,
			CaO:        30,
			FeO:        10,
			MgO:        5,
			PbLeaching: 0.5,
			CdLeaching: 0.05,
			AsLeaching: 0.2,
			HgLeaching: 0.01,
			CrLeaching: 0.3,
			NiLeaching: 0.1,
			GlassPhase: 60,
			Density:    3.2,
		}
		otherUses := m.AssessOtherUses(slag)

		requiredKeys := []string{"concrete_aggregate", "glass_ceramic", "soil_amendment", "metal_recovery"}
		for _, key := range requiredKeys {
			if _, ok := otherUses[key]; !ok {
				t.Errorf("missing %s use assessment", key)
			}
		}

		for key, val := range otherUses {
			if sub, ok := val.(map[string]interface{}); ok {
				if _, hasScore := sub["score"]; !hasScore {
					t.Errorf("%s assessment missing score field", key)
				}
			} else {
				t.Errorf("%s assessment is not map type", key)
			}
		}

		t.Log("other uses assessment:")
		for key, val := range otherUses {
			if sub, ok := val.(map[string]interface{}); ok {
				if score, ok := sub["score"].(float64); ok {
					t.Logf("  %s: score=%.1f", key, score)
				}
			}
		}
	})

	t.Run("high Fe slag metal recovery value", func(t *testing.T) {
		highFe := &models.SlagComposition{
			FeO:        40,
			PbLeaching: 2.0,
			GlassPhase: 30,
		}
		lowFe := &models.SlagComposition{
			FeO:        5,
			PbLeaching: 0.1,
			GlassPhase: 30,
		}
		highUses := m.AssessOtherUses(highFe)
		lowUses := m.AssessOtherUses(lowFe)

		highScore := 0.0
		lowScore := 0.0
		if sub, ok := highUses["metal_recovery"].(map[string]interface{}); ok {
			highScore = sub["score"].(float64)
		}
		if sub, ok := lowUses["metal_recovery"].(map[string]interface{}); ok {
			lowScore = sub["score"].(float64)
		}

		if highScore <= lowScore {
			t.Errorf("high Fe metal recovery score(%.1f) should be > low Fe(%.1f)", highScore, lowScore)
		}
	})
}

func TestAssessLeachingRisk(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("low leaching = low risk", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 0.5,
			CdLeaching: 0.05,
			AsLeaching: 0.3,
			HgLeaching: 0.01,
			CrLeaching: 0.2,
			NiLeaching: 0.1,
		}
		riskLevel, details := m.AssessLeachingRisk(slag)
		if riskLevel != "低风险" {
			t.Errorf("low leaching risk='%s', want '低风险'", riskLevel)
		}
		if details == nil {
			t.Error("details should not be nil")
		}
		if details["exceed_count"] != 0 {
			t.Errorf("exceed count=%v, want 0", details["exceed_count"])
		}
		t.Logf("low leaching: risk=%s, exceed=%v", riskLevel, details["exceed_count"])
	})

	t.Run("multiple exceed = high/extreme risk", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 5.0,
			CdLeaching: 1.0,
			AsLeaching: 3.0,
			HgLeaching: 0.1,
			CrLeaching: 10.0,
			NiLeaching: 10.0,
		}
		riskLevel, details := m.AssessLeachingRisk(slag)
		if riskLevel != "极高风险" && riskLevel != "高风险" {
			t.Errorf("multi-exceed risk='%s', want '高风险' or '极高风险'", riskLevel)
		}
		exceedCount, _ := details["exceed_count"].(int)
		if exceedCount < 5 {
			t.Errorf("exceed count=%d, should >= 5", exceedCount)
		}
		t.Logf("multi-exceed: risk=%s, exceed=%d", riskLevel, exceedCount)
	})

	t.Run("severe Hg/As = extreme risk", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 0.1,
			CdLeaching: 0.01,
			AsLeaching: 10.0,
			HgLeaching: 0.5,
			CrLeaching: 0.1,
			NiLeaching: 0.1,
		}
		riskLevel, details := m.AssessLeachingRisk(slag)
		severeHgAs, _ := details["severe_hg_as"].(bool)
		if !severeHgAs {
			t.Error("severe_hg_as should be true for severe Hg/As exceedance")
		}
		if riskLevel != "极高风险" {
			t.Errorf("severe Hg/As risk='%s', want '极高风险'", riskLevel)
		}
		t.Logf("severe Hg/As: risk=%s, severe_hg_as=%v", riskLevel, severeHgAs)
	})

	t.Run("single exceed = medium risk", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 4.0,
			CdLeaching: 0.01,
			AsLeaching: 0.1,
			HgLeaching: 0.01,
			CrLeaching: 0.1,
			NiLeaching: 0.1,
		}
		riskLevel, _ := m.AssessLeachingRisk(slag)
		if riskLevel != "中风险" {
			t.Errorf("single exceed risk='%s', want '中风险'", riskLevel)
		}
	})

	t.Run("at standard limit = pass", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: m.Std.LeachingPbMax,
			CdLeaching: m.Std.LeachingCdMax,
			AsLeaching: m.Std.LeachingAsMax,
			HgLeaching: m.Std.LeachingHgMax,
			CrLeaching: m.Std.LeachingCrMax,
			NiLeaching: m.Std.LeachingNiMax,
		}
		riskLevel, details := m.AssessLeachingRisk(slag)
		exceedCount, _ := details["exceed_count"].(int)
		if exceedCount != 0 {
			t.Errorf("at limit exceed count=%d, should be 0 (pass)", exceedCount)
		}
		t.Logf("at limit: risk=%s, exceed=%d", riskLevel, exceedCount)
	})
}

func TestDecideRecommendation(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("low leaching + high cement = cement priority", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 50.0},
		}
		recommended, details := m.DecideRecommendation(85, 60, otherUses, "低风险", "S95")
		if recommended != "优先水泥混合材" {
			t.Errorf("recommendation='%s', want '优先水泥混合材'", recommended)
		}
		if len(details["reasons"].([]string)) == 0 {
			t.Error("reasons empty")
		}
		t.Logf("recommendation: %s", recommended)
		for _, r := range details["reasons"].([]string) {
			t.Logf("  - %s", r)
		}
	})

	t.Run("low leaching + low cement + high road = road priority", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 60.0},
		}
		recommended, _ := m.DecideRecommendation(40, 80, otherUses, "低风险", "不合格")
		if recommended != "优先道路基层材料" {
			t.Errorf("recommendation='%s', want '优先道路基层材料'", recommended)
		}
		t.Logf("recommendation: %s", recommended)
	})

	t.Run("low leaching + low scores + high glass = glass ceramic", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 90.0},
		}
		recommended, _ := m.DecideRecommendation(30, 50, otherUses, "低风险", "不合格")
		if recommended != "推荐微晶玻璃/铸石" {
			t.Errorf("recommendation='%s', want '推荐微晶玻璃/铸石'", recommended)
		}
		t.Logf("recommendation: %s", recommended)
	})

	t.Run("high leaching = stabilization", func(t *testing.T) {
		otherUses := map[string]interface{}{}
		recommended, details := m.DecideRecommendation(70, 80, otherUses, "高风险", "S75")
		if recommended != "稳定化处理→安全填埋/资源化" {
			t.Errorf("high leaching recommendation='%s', want '稳定化处理→安全填埋/资源化'", recommended)
		}
		decPath, _ := details["decision_path"].(map[string]interface{})
		if decPath["leaching_risk"] != "高风险" {
			t.Error("decision path leaching risk incorrect")
		}
		t.Logf("high leaching recommendation: %s", recommended)
	})

	t.Run("all low = metal recovery + landfill", func(t *testing.T) {
		otherUses := map[string]interface{}{
			"glass_ceramic": map[string]interface{}{"score": 30.0},
		}
		recommended, _ := m.DecideRecommendation(30, 40, otherUses, "低风险", "不合格")
		if recommended != "有价金属回收+填埋" {
			t.Errorf("all low recommendation='%s', want '有价金属回收+填埋'", recommended)
		}
		t.Logf("all low recommendation: %s", recommended)
	})
}

func TestGenerateProcessFlow(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("cement blended flow", func(t *testing.T) {
		flow := m.GenerateProcessFlow("优先水泥混合材", "低风险")
		if len(flow) < 3 {
			t.Errorf("flow steps=%d, should >= 3", len(flow))
		}
		hasStep1 := false
		for _, step := range flow {
			if step["step"].(int) == 1 {
				hasStep1 = true
			}
		}
		if !hasStep1 {
			t.Error("missing step 1")
		}
		t.Logf("cement flow: %d steps", len(flow))
		for _, step := range flow {
			t.Logf("  step%d: %s (%.0f yuan/ton)", step["step"].(int), step["desc"].(string), step["cost"].(float64))
		}
	})

	t.Run("stabilization flow", func(t *testing.T) {
		flow := m.GenerateProcessFlow("稳定化处理→安全填埋/资源化", "高风险")
		if len(flow) < 4 {
			t.Errorf("stabilization flow steps=%d, should >= 4", len(flow))
		}
		hasStabilize := false
		for _, step := range flow {
			if step["desc"].(string) == "药剂稳定化处理" {
				hasStabilize = true
			}
		}
		if !hasStabilize {
			t.Error("stabilization flow should include chemical stabilization step")
		}
	})

	t.Run("road base flow", func(t *testing.T) {
		flow := m.GenerateProcessFlow("优先道路基层材料", "低风险")
		if len(flow) < 3 {
			t.Errorf("road flow steps=%d, should >= 3", len(flow))
		}
	})

	t.Run("glass ceramic flow", func(t *testing.T) {
		flow := m.GenerateProcessFlow("推荐微晶玻璃/铸石", "低风险")
		if len(flow) < 4 {
			t.Errorf("glass ceramic flow steps=%d, should >= 4", len(flow))
		}
	})

	t.Run("default flow (metal recovery + landfill)", func(t *testing.T) {
		flow := m.GenerateProcessFlow("未知方案", "低风险")
		if len(flow) == 0 {
			t.Error("default flow should not be empty")
		}
	})

	t.Run("high leaching cement flow should have stabilization", func(t *testing.T) {
		flow := m.GenerateProcessFlow("优先水泥混合材", "高风险")
		hasStabilize := false
		for _, step := range flow {
			desc, ok := step["desc"].(string)
			if ok && desc == "稳定化预处理" {
				hasStabilize = true
				break
			}
		}
		if !hasStabilize {
			t.Error("high leaching cement flow should include stabilization preprocessing")
		}
	})

	t.Run("low leaching cement flow should not have stabilization", func(t *testing.T) {
		flow := m.GenerateProcessFlow("优先水泥混合材", "低风险")
		for _, step := range flow {
			desc, ok := step["desc"].(string)
			if ok && (desc == "稳定化预处理" || desc == "药剂稳定化处理") {
				t.Error("low leaching flow should not include stabilization step")
			}
		}
	})
}

func TestAssessRecycle(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("complete assessment result", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           15,
			CaO:             40,
			FeO:             8,
			MgO:             8,
			GlassPhase:      85,
			LossOnIgnition:  0.5,
			SpecificSurface: 450,
			MeasurementYear: 2023,
			PbLeaching:      0.5,
			CdLeaching:      0.05,
			AsLeaching:      0.3,
			HgLeaching:      0.01,
			CrLeaching:      0.2,
			NiLeaching:      0.1,
			Fayalite:        10,
			Wollastonite:    10,
			Density:         3.2,
		}
		result, err := m.AssessRecycle(nil, 1, slag)
		if err != nil {
			t.Fatalf("AssessRecycle returned error: %v", err)
		}
		if result == nil {
			t.Fatal("AssessRecycle returned nil result")
		}
		if result.Assessment == nil {
			t.Error("Assessment should not be nil")
		}
		if result.SiteID != 1 {
			t.Errorf("SiteID=%d, want 1", result.SiteID)
		}
		if len(result.CementChecks) != 6 {
			t.Errorf("CementChecks count=%d, want 6", len(result.CementChecks))
		}
		if len(result.RoadChecks) != 5 {
			t.Errorf("RoadChecks count=%d, want 5", len(result.RoadChecks))
		}
		if result.ProcessFlow == nil {
			t.Error("ProcessFlow should not be nil")
		}
		if result.AcceleratedTests == nil {
			t.Error("AcceleratedTests should not be nil")
		}
		if result.ConservativeEstimate == nil {
			t.Error("ConservativeEstimate should not be nil")
		}
		if result.Composition != slag {
			t.Error("Composition should match input slag")
		}
		t.Logf("AssessRecycle: siteID=%d, cementScore=%.2f, roadScore=%.2f, recommended=%s",
			result.SiteID, result.Assessment.CementBlendedScore, result.Assessment.RoadBaseScore, result.Assessment.RecommendedUse)
	})

	t.Run("assessment scores within range", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            30,
			Al2O3:           10,
			CaO:             35,
			GlassPhase:      60,
			LossOnIgnition:  1.0,
			SpecificSurface: 400,
			MeasurementYear: 2023,
			PbLeaching:      0.3,
			CdLeaching:      0.02,
			AsLeaching:      0.1,
			HgLeaching:      0.005,
			CrLeaching:      0.1,
			NiLeaching:      0.05,
		}
		result, _ := m.AssessRecycle(nil, 2, slag)
		if result.Assessment.CementBlendedScore < 0 || result.Assessment.CementBlendedScore > 100 {
			t.Errorf("cement score %.2f out of [0,100]", result.Assessment.CementBlendedScore)
		}
		if result.Assessment.RoadBaseScore < 0 || result.Assessment.RoadBaseScore > 100 {
			t.Errorf("road score %.2f out of [0,100]", result.Assessment.RoadBaseScore)
		}
	})
}

func TestComputeConservativeEstimate(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("complete data low risk = no conservative", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           15,
			CaO:             40,
			FeO:             8,
			GlassPhase:      85,
			SpecificSurface: 450,
			LossOnIgnition:  0.5,
			PbLeaching:      0.5,
			CdLeaching:      0.05,
			AsLeaching:      0.3,
			HgLeaching:      0.01,
			CrLeaching:      0.2,
			NiLeaching:      0.1,
		}
		accel := m.SimulateAcceleratedAging(slag, 85, 80, "低风险")
		conservative := m.ComputeConservativeEstimate(slag, 85, 80, "低风险", "S95", accel)
		if conservative == nil {
			t.Fatal("ComputeConservativeEstimate returned nil")
		}
		if conservative.UseConservativeLimits {
			t.Error("complete data + low risk should not use conservative limits")
		}
		if conservative.SafetyMarginPct <= 0 {
			t.Error("safety margin should be positive")
		}
		t.Logf("conservative: use=%v, margin=%.0f%%, cement=%.2f, road=%.2f",
			conservative.UseConservativeLimits, conservative.SafetyMarginPct,
			conservative.CementScoreConservative, conservative.RoadScoreConservative)
	})

	t.Run("high leaching risk = conservative limits", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:            35,
			Al2O3:           15,
			CaO:             40,
			FeO:             8,
			GlassPhase:      50,
			SpecificSurface: 350,
			LossOnIgnition:  2.0,
			PbLeaching:      5.0,
			CdLeaching:      1.0,
			AsLeaching:      3.0,
			HgLeaching:      0.5,
			CrLeaching:      10.0,
			NiLeaching:      8.0,
		}
		accel := m.SimulateAcceleratedAging(slag, 60, 50, "高风险")
		conservative := m.ComputeConservativeEstimate(slag, 60, 50, "高风险", "S75", accel)
		if !conservative.UseConservativeLimits {
			t.Error("high leaching risk should use conservative limits")
		}
		if conservative.CementScoreConservative >= 60 {
			t.Errorf("conservative cement score %.2f should be < original 60", conservative.CementScoreConservative)
		}
		if conservative.RoadScoreConservative >= 50 {
			t.Errorf("conservative road score %.2f should be < original 50", conservative.RoadScoreConservative)
		}
		if len(conservative.UncertaintyFactors) < 2 {
			t.Errorf("uncertainty factors count=%d, should >= 2", len(conservative.UncertaintyFactors))
		}
		t.Logf("conservative: use=%v, margin=%.0f%%, cement=%.2f, road=%.2f, factors=%v",
			conservative.UseConservativeLimits, conservative.SafetyMarginPct,
			conservative.CementScoreConservative, conservative.RoadScoreConservative,
			conservative.UncertaintyFactors)
	})

	t.Run("missing data fields = uncertainty factor", func(t *testing.T) {
		slag := &models.SlagComposition{
			PbLeaching: 0.5,
			CdLeaching: 0.05,
			AsLeaching: 0.3,
			HgLeaching: 0.01,
			CrLeaching: 0.2,
			NiLeaching: 0.1,
		}
		accel := m.SimulateAcceleratedAging(slag, 40, 30, "低风险")
		conservative := m.ComputeConservativeEstimate(slag, 40, 30, "低风险", "不合格", accel)
		hasMissingDataFactor := false
		for _, f := range conservative.UncertaintyFactors {
			if len(f) > 0 {
				hasMissingDataFactor = true
				break
			}
		}
		if !hasMissingDataFactor {
			t.Error("missing data should produce at least one uncertainty factor")
		}
	})
}

func TestSlagReuseAdvisorAcceleratedAging(t *testing.T) {
	m := slag_reuse_advisor.NewSlagReuseAdvisor()

	t.Run("returns non-nil report", func(t *testing.T) {
		slag := &models.SlagComposition{
			SiO2:           35,
			Al2O3:          15,
			CaO:            40,
			FeO:            8,
			GlassPhase:     70,
			LossOnIgnition: 1.0,
			PbLeaching:     0.5,
			CdLeaching:     0.05,
			AsLeaching:     0.3,
			HgLeaching:     0.01,
			CrLeaching:     0.2,
			NiLeaching:     0.1,
			Fayalite:       10,
			Wollastonite:   10,
		}
		report := m.SimulateAcceleratedAging(slag, 80, 75, "低风险")
		if report == nil {
			t.Fatal("SimulateAcceleratedAging returned nil")
		}
		if report.CementLongTerm == nil {
			t.Error("CementLongTerm should not be nil")
		}
		if report.RoadDurability == nil {
			t.Error("RoadDurability should not be nil")
		}
		if report.LeachingLongTerm == nil {
			t.Error("LeachingLongTerm should not be nil")
		}
		if report.AgingFactor <= 0 {
			t.Errorf("AgingFactor=%.4f, should be > 0", report.AgingFactor)
		}
		if report.TestDurationDays <= 0 {
			t.Errorf("TestDurationDays=%d, should be > 0", report.TestDurationDays)
		}
		if report.EquivalentYears <= 0 {
			t.Errorf("EquivalentYears=%.1f, should be > 0", report.EquivalentYears)
		}
		t.Logf("accelerated aging: agingFactor=%.4f, testDays=%d, equivYears=%.1f",
			report.AgingFactor, report.TestDurationDays, report.EquivalentYears)
		t.Logf("  cement: act90d=%.2f, act1yr=%.2f, lossRate=%.2f%%",
			report.CementLongTerm.Activity90d, report.CementLongTerm.Activity1yr,
			report.CementLongTerm.StrengthLossRate)
		t.Logf("  road: freeze100=%.2f, residualCBR=%.2f, grade=%s",
			report.RoadDurability.FreezeThaw100Cycles, report.RoadDurability.ResidualCBR,
			report.RoadDurability.LongTermGrade)
		t.Logf("  leaching: riskAfterAging=%s", report.LeachingLongTerm.RiskLevelAfterAging)
	})

	t.Run("low glass phase = faster aging", func(t *testing.T) {
		lowGlass := &models.SlagComposition{
			GlassPhase:     20,
			LossOnIgnition: 3.0,
			PbLeaching:     0.1,
			CdLeaching:     0.01,
			AsLeaching:     0.1,
			HgLeaching:     0.005,
			CrLeaching:     0.1,
			NiLeaching:     0.05,
		}
		highGlass := &models.SlagComposition{
			GlassPhase:     90,
			LossOnIgnition: 0.3,
			PbLeaching:     0.1,
			CdLeaching:     0.01,
			AsLeaching:     0.1,
			HgLeaching:     0.005,
			CrLeaching:     0.1,
			NiLeaching:     0.05,
		}
		lowReport := m.SimulateAcceleratedAging(lowGlass, 50, 50, "低风险")
		highReport := m.SimulateAcceleratedAging(highGlass, 80, 75, "低风险")
		if lowReport.AgingFactor <= highReport.AgingFactor {
			t.Errorf("low glass agingFactor(%.4f) should be > high glass(%.4f)",
				lowReport.AgingFactor, highReport.AgingFactor)
		}
	})
}
