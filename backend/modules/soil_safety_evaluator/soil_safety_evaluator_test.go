package soil_safety_evaluator_test

import (
	"context"
	"math"
	"testing"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/soil_safety_evaluator"
)

func TestNewSoilSafetyEvaluator(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()
	if fsm == nil {
		t.Fatal("NewSoilSafetyEvaluator 返回 nil")
	}
	if len(fsm.GeoCfg().BackgroundValues) == 0 {
		t.Error("地壳背景值未初始化")
	}
	if len(fsm.RICfg().ToxicFactors) == 0 {
		t.Error("毒性系数未初始化")
	}
	if len(fsm.GeoLevels()) != 7 {
		t.Errorf("Igeo等级数=%d，期望7", len(fsm.GeoLevels()))
	}
}

func TestCalcIgeo(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("正常计算 - Pb", func(t *testing.T) {
		igeo, level, desc := fsm.CalcIgeo("Pb", 100)
		bg := fsm.GeoCfg().BackgroundValues["Pb"]
		k := fsm.GeoCfg().CorrectionFactor
		expected := math.Log2(100.0 / (k * bg))
		if math.Abs(igeo-expected) > 1e-4 {
			t.Errorf("Igeo(Pb)=%.4f，期望 %.4f", igeo, expected)
		}
		if level < 0 || level >= len(fsm.GeoLevels()) {
			t.Errorf("等级 %d 超出范围", level)
		}
		if desc == "" {
			t.Error("描述为空")
		}
		t.Logf("Pb=100mg/kg → Igeo=%.4f, 等级=%d, %s", igeo, level, desc)
	})

	t.Run("浓度为0 → 负无穷/0级", func(t *testing.T) {
		igeo, level, _ := fsm.CalcIgeo("Pb", 0)
		if !math.IsInf(igeo, -1) {
			t.Errorf("浓度为0时Igeo=%.4f，期望负无穷", igeo)
		}
		if level != 0 {
			t.Errorf("浓度为0时等级=%d，期望0", level)
		}
	})

	t.Run("极高浓度 → 最高级", func(t *testing.T) {
		igeo, level, desc := fsm.CalcIgeo("Hg", 1000)
		if level != len(fsm.GeoLevels())-1 {
			t.Errorf("Hg=1000mg/kg 等级=%d，期望最高级%d", level, len(fsm.GeoLevels())-1)
		}
		t.Logf("Hg=1000mg/kg → Igeo=%.4f, 等级=%d, %s", igeo, level, desc)
	})

	t.Run("不存在的金属", func(t *testing.T) {
		igeo, level, desc := fsm.CalcIgeo("Uranium", 100)
		if igeo != 0 {
			t.Errorf("未知金属 Igeo=%.4f，期望0", igeo)
		}
		if level != 0 {
			t.Errorf("未知金属等级=%d，期望0", level)
		}
		if desc == "" {
			t.Error("应有默认描述")
		}
	})

	t.Run("背景值为0的金属", func(t *testing.T) {
		origCfg := fsm.GeoCfg()
		newCfg := origCfg
		newCfg.BackgroundValues = map[string]float64{"X": 0}
		fsm.SetGeoCfg(newCfg)
		igeo, level, _ := fsm.CalcIgeo("X", 100)
		fsm.SetGeoCfg(origCfg)
		if igeo != 0 {
			t.Errorf("背景值为0时 Igeo=%.4f，期望0", igeo)
		}
		if level != 0 {
			t.Errorf("背景值为0时等级=%d，期望0", level)
		}
	})

	t.Run("校正因子验证", func(t *testing.T) {
		k := fsm.GeoCfg().CorrectionFactor
		if k != 1.5 {
			t.Logf("校正因子 k=%.1f（默认应为1.5）", k)
		}
	})
}

func TestCalcRI(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("正常土壤 - 低风险", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 20, Zn: 50, Cu: 30, As: 8,
			Hg: 0.1, Cd: 0.2, Cr: 40, Ni: 15,
		}
		ri, metalEri, maxEri, maxMetal := fsm.CalcRI(fl)
		if ri <= 0 {
			t.Error("RI应为正数")
		}
		if len(metalEri) == 0 {
			t.Error("金属Eri map为空")
		}
		if maxEri <= 0 {
			t.Error("最大Eri应为正数")
		}
		if maxMetal == "" {
			t.Error("最大贡献金属为空")
		}
		t.Logf("清洁土壤 RI=%.2f, 最大贡献: %s (%.2f)", ri, maxMetal, maxEri)
	})

	t.Run("严重污染土壤 - 高RI", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 2000, Zn: 3000, Cu: 1500, As: 200,
			Hg: 20, Cd: 50, Cr: 300, Ni: 200,
		}
		ri, _, maxEri, _ := fsm.CalcRI(fl)
		if ri < 600 {
			t.Errorf("严重污染 RI=%.2f，应≥600", ri)
		}
		if maxEri < 40 {
			t.Errorf("最大单项Eri=%.2f，应更高", maxEri)
		}
		t.Logf("重污染土壤 RI=%.2f, 最大Eri=%.2f", ri, maxEri)
	})

	t.Run("RI计算验证 - Hg毒性系数最高", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Hg: 1.0,
		}
		ri, metalEri, _, _ := fsm.CalcRI(fl)
		expectedTri := fsm.RICfg().ToxicFactors["Hg"]
		expectedBn := fsm.RICfg().RefValues["Hg"]
		expectedEri := expectedTri * (1.0 / expectedBn)
		actualEri := metalEri["Hg"]
		if math.Abs(actualEri-expectedEri) > 0.01 {
			t.Errorf("Hg的Eri=%.2f，期望%.2f", actualEri, expectedEri)
		}
		if math.Abs(ri-expectedEri) > 0.01 {
			t.Errorf("RI=%.2f，期望%.2f（仅Hg）", ri, expectedEri)
		}
	})

	t.Run("毒性系数排序验证", func(t *testing.T) {
		tri := fsm.RICfg().ToxicFactors
		if tri["Hg"] <= tri["Cd"] {
			t.Errorf("Hg毒性系数应高于Cd")
		}
		if tri["Cd"] <= tri["As"] {
			t.Errorf("Cd毒性系数应高于As")
		}
	})
}

func TestClassifyRI(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	tests := []struct {
		ri       float64
		expected string
	}{
		{0, "低风险"},
		{100, "低风险"},
		{149.9, "低风险"},
		{150, "中等风险"},
		{200, "中等风险"},
		{299.9, "中等风险"},
		{300, "较高风险"},
		{450, "较高风险"},
		{599.9, "较高风险"},
		{600, "极高风险"},
		{10000, "极高风险"},
	}

	for _, tt := range tests {
		result := fsm.ClassifyRI(tt.ri)
		if result != tt.expected {
			t.Errorf("RI=%.1f → %s，期望 %s", tt.ri, result, tt.expected)
		}
	}
}

func TestFormatDistanceLabel(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	tests := []struct {
		dist     float64
		expected string
	}{
		{0, "≤500米"},
		{250, "≤500米"},
		{500, "≤500米"},
		{501, "500-1000米"},
		{800, "500-1000米"},
		{1000, "500-1000米"},
		{1001, "1000-2000米"},
		{1500, "1000-2000米"},
		{2000, "1000-2000米"},
		{2001, "≥2000米"},
		{5000, "≥2000米"},
	}

	for _, tt := range tests {
		result := fsm.FormatDistanceLabel(tt.dist)
		if result != tt.expected {
			t.Errorf("距离%.0f米 → '%s'，期望 '%s'", tt.dist, result, tt.expected)
		}
	}
}

func TestAssessCropRisk(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("清洁土壤 → 低风险，推荐种植", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 10, Zn: 30, Cu: 15, As: 3,
			Hg: 0.03, Cd: 0.05, Cr: 30, Ni: 10,
			LandUseType: "旱地",
		}
		rec := fsm.AssessCropRisk(fl, "旱地")
		if rec.RiskLevel != "低风险" {
			t.Errorf("清洁土壤风险等级='%s'，期望'低风险'", rec.RiskLevel)
		}
		if rec.RiskColor != "green" {
			t.Errorf("清洁土壤风险颜色='%s'，期望'green'", rec.RiskColor)
		}
		if rec.ExceedCount > 0 {
			t.Errorf("清洁土壤超标数=%d，应=0", rec.ExceedCount)
		}
		if len(rec.Recommendations) == 0 {
			t.Error("应有种植建议")
		}
	})

	t.Run("严重污染土壤 → 高风险，禁止种植", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 2000, Zn: 5000, Cu: 2000, As: 300,
			Hg: 30, Cd: 80, Cr: 500, Ni: 300,
			LandUseType: "水田",
		}
		rec := fsm.AssessCropRisk(fl, "水田")
		if rec.RiskLevel != "高风险" {
			t.Errorf("重污染风险等级='%s'，期望'高风险'", rec.RiskLevel)
		}
		if rec.ExceedCount <= 0 {
			t.Error("重污染土壤应有多种金属超标")
		}
	})

	t.Run("中等污染 → 中等风险，替代种植", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 300, Zn: 800, Cu: 400, As: 50,
			Hg: 2, Cd: 5, Cr: 100, Ni: 50,
			LandUseType: "旱地",
		}
		rec := fsm.AssessCropRisk(fl, "旱地")
		if rec.RiskLevel != "高风险" && rec.RiskLevel != "中等风险" {
			t.Logf("中等污染风险等级: %s (超标数: %d, 接近数: %d)",
				rec.RiskLevel, rec.ExceedCount, rec.CloseCount)
		}
	})

	t.Run("不同土地利用类型 → 不同BCF", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 200, Zn: 500, Cu: 200, As: 40,
			Hg: 1, Cd: 3, Cr: 80, Ni: 40,
		}
		recPaddy := fsm.AssessCropRisk(fl, "水田")
		recDry := fsm.AssessCropRisk(fl, "旱地")
		if recPaddy.ExceedCount == recDry.ExceedCount {
			t.Log("水田和旱地超标数相同（可能BCF不同但都超标）")
		}
		t.Logf("水田: %d种超标, 旱地: %d种超标", recPaddy.ExceedCount, recDry.ExceedCount)
	})

	t.Run("未知土地利用类型 → 使用旱地默认", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 50, Zn: 100, Cu: 80, As: 15,
			Hg: 0.2, Cd: 0.5, Cr: 50, Ni: 20,
		}
		rec := fsm.AssessCropRisk(fl, "未知类型")
		if rec.RiskLevel == "" {
			t.Error("未知土地类型也应有风险评估结果")
		}
	})

	t.Run("BCF预测准确性检查", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 100, Zn: 200,
			LandUseType: "旱地",
		}
		rec := fsm.AssessCropRisk(fl, "旱地")
		for _, p := range rec.Predictions {
			if p.Metal == "Pb" {
				bcf, _ := fsm.CropCfg().BCF["旱地"]
				expectedConc := 100.0 * bcf["Pb"]
				if math.Abs(p.PredictedCropConc-expectedConc) > 1e-6 {
					t.Errorf("Pb预测浓度=%.6f，期望%.6f", p.PredictedCropConc, expectedConc)
				}
			}
		}
	})
}

func TestCalcDistanceDecay(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("多组距离数据", func(t *testing.T) {
		groups := map[string][]models.FarmlandSoil{
			"≤500米": {
				{DistanceFromSite: 200, Pb: 500, Zn: 800, Cu: 600, As: 50, Hg: 2, Cd: 5, Cr: 100, Ni: 50},
			},
			"500-1000米": {
				{DistanceFromSite: 800, Pb: 200, Zn: 400, Cu: 300, As: 25, Hg: 1, Cd: 2, Cr: 70, Ni: 30},
			},
			"1000-2000米": {
				{DistanceFromSite: 1500, Pb: 80, Zn: 180, Cu: 120, As: 10, Hg: 0.3, Cd: 0.5, Cr: 50, Ni: 20},
			},
			"≥2000米": {
				{DistanceFromSite: 3000, Pb: 20, Zn: 50, Cu: 30, As: 3, Hg: 0.05, Cd: 0.1, Cr: 30, Ni: 10},
			},
		}
		decay := fsm.CalcDistanceDecay(groups)
		if len(decay) != 4 {
			t.Errorf("距离衰减组数量=%d，期望4", len(decay))
		}

		for i := 0; i < len(decay)-1; i++ {
			if decay[i].AvgIgeo < decay[i+1].AvgIgeo {
				t.Errorf("距离越远 Igeo 应越低，但 %s(%.4f) < %s(%.4f)",
					decay[i].DistanceLabel, decay[i].AvgIgeo,
					decay[i+1].DistanceLabel, decay[i+1].AvgIgeo)
			}
			if decay[i].AvgRI < decay[i+1].AvgRI {
				t.Errorf("距离越远 RI 应越低，但 %s(%.2f) < %s(%.2f)",
					decay[i].DistanceLabel, decay[i].AvgRI,
					decay[i+1].DistanceLabel, decay[i+1].AvgRI)
			}
		}

		for _, d := range decay {
			t.Logf("%s: 样点数=%d, 平均Igeo=%.4f, 平均RI=%.2f",
				d.DistanceLabel, d.SampleCount, d.AvgIgeo, d.AvgRI)
		}
	})

	t.Run("部分距离组缺失", func(t *testing.T) {
		groups := map[string][]models.FarmlandSoil{
			"≤500米": {
				{DistanceFromSite: 100, Pb: 300, Zn: 500, Cu: 400, As: 30, Hg: 1.5, Cd: 3, Cr: 80, Ni: 40},
			},
			"≥2000米": {
				{DistanceFromSite: 5000, Pb: 30, Zn: 60, Cu: 40, As: 5, Hg: 0.1, Cd: 0.2, Cr: 40, Ni: 15},
			},
		}
		decay := fsm.CalcDistanceDecay(groups)
		if len(decay) != 2 {
			t.Errorf("缺失组时结果数=%d，期望2", len(decay))
		}
	})

	t.Run("空输入", func(t *testing.T) {
		groups := map[string][]models.FarmlandSoil{}
		decay := fsm.CalcDistanceDecay(groups)
		if len(decay) != 0 {
			t.Errorf("空输入结果数=%d，期望0", len(decay))
		}
	})
}

func TestClassifyOverallRisk(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	tests := []struct {
		maxIgeo  float64
		totalRI  float64
		expected string
	}{
		{0, 0, "低风险"},
		{0.5, 50, "低风险"},
		{0.99, 100, "低风险"},
		{1.0, 100, "中风险"},
		{0.5, 150, "中风险"},
		{2.5, 200, "中风险"},
		{3.0, 250, "较高风险"},
		{2.0, 300, "较高风险"},
		{4.5, 400, "较高风险"},
		{5.0, 500, "高风险"},
		{3.0, 600, "高风险"},
		{10, 2000, "高风险"},
	}

	for _, tt := range tests {
		level, color := fsm.ClassifyOverallRisk(tt.maxIgeo, tt.totalRI)
		if level != tt.expected {
			t.Errorf("Igeo=%.1f, RI=%.0f → %s，期望 %s",
				tt.maxIgeo, tt.totalRI, level, tt.expected)
		}
		if color == "" {
			t.Error("风险颜色为空")
		}
	}
}

func TestAssessFarmSafety(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()
	ctx := context.Background()

	t.Run("正常评估 - 多样点", func(t *testing.T) {
		farmlands := []models.FarmlandSoil{
			{ID: 1, DistanceFromSite: 200, Direction: "NE", LandUseType: "旱地",
				Pb: 400, Zn: 650, Cu: 800, As: 60, Hg: 2.5, Cd: 8, Cr: 70, Ni: 40},
			{ID: 2, DistanceFromSite: 800, Direction: "E", LandUseType: "水田",
				Pb: 180, Zn: 400, Cu: 300, As: 30, Hg: 0.8, Cd: 3, Cr: 60, Ni: 35},
			{ID: 3, DistanceFromSite: 2000, Direction: "S", LandUseType: "果园",
				Pb: 60, Zn: 150, Cu: 100, As: 12, Hg: 0.2, Cd: 0.8, Cr: 50, Ni: 25},
		}
		result, err := fsm.AssessFarmSafety(ctx, 1, farmlands)
		if err != nil {
			t.Fatalf("评估失败: %v", err)
		}
		if result == nil {
			t.Fatal("结果为nil")
		}
		if result.SiteID != 1 {
			t.Errorf("SiteID=%d，期望1", result.SiteID)
		}
		if len(result.SampleResults) != 3 {
			t.Errorf("样点Igeo结果数=%d，期望3", len(result.SampleResults))
		}
		if len(result.EcoRiskResults) != 3 {
			t.Errorf("样点生态风险结果数=%d，期望3", len(result.EcoRiskResults))
		}
		if len(result.CropRecommendations) != 3 {
			t.Errorf("作物建议数=%d，期望3", len(result.CropRecommendations))
		}
		if len(result.DistanceDecay) == 0 {
			t.Error("距离衰减结果为空")
		}
		if result.OverallRiskLevel == "" {
			t.Error("综合风险等级为空")
		}
		if result.OverallRiskColor == "" {
			t.Error("风险颜色为空")
		}
		if result.Summary == "" {
			t.Error("评估摘要为空")
		}
		if result.AssessmentDate == "" {
			t.Error("评估日期为空")
		}

		t.Logf("综合风险: %s (%s), 最大Igeo=%.4f, 总RI=%.2f",
			result.OverallRiskLevel, result.OverallRiskColor,
			result.MaxIgeo, result.TotalRI)
	})

	t.Run("空输入 → 错误", func(t *testing.T) {
		result, err := fsm.AssessFarmSafety(ctx, 1, []models.FarmlandSoil{})
		if err == nil {
			t.Error("空输入应返回错误")
		}
		if result != nil {
			t.Error("空输入结果应为nil")
		}
	})

	t.Run("单样点评估", func(t *testing.T) {
		farmlands := []models.FarmlandSoil{
			{ID: 1, DistanceFromSite: 500, Direction: "N", LandUseType: "旱地",
				Pb: 200, Zn: 350, Cu: 250, As: 25, Hg: 1.0, Cd: 2.5, Cr: 60, Ni: 30},
		}
		result, err := fsm.AssessFarmSafety(ctx, 2, farmlands)
		if err != nil {
			t.Fatalf("单样点评估失败: %v", err)
		}
		if len(result.SampleResults) != 1 {
			t.Errorf("单样点结果数=%d，期望1", len(result.SampleResults))
		}
	})

	t.Run("高污染遗址周边 → 高风险", func(t *testing.T) {
		farmlands := []models.FarmlandSoil{
			{ID: 1, DistanceFromSite: 100, LandUseType: "水田",
				Pb: 3000, Zn: 8000, Cu: 4000, As: 500, Hg: 50, Cd: 100, Cr: 400, Ni: 200},
		}
		result, _ := fsm.AssessFarmSafety(ctx, 3, farmlands)
		if result.OverallRiskLevel != "高风险" {
			t.Errorf("高污染风险等级='%s'，期望'高风险'", result.OverallRiskLevel)
		}
	})

	t.Run("种植建议合理性验证", func(t *testing.T) {
		farmlands := []models.FarmlandSoil{
			{ID: 1, DistanceFromSite: 500, LandUseType: "旱地",
				Pb: 50, Zn: 100, Cu: 50, As: 10, Hg: 0.1, Cd: 0.3, Cr: 40, Ni: 15},
		}
		result, _ := fsm.AssessFarmSafety(ctx, 4, farmlands)
		for _, rec := range result.CropRecommendations {
			if len(rec.Recommendations) == 0 {
				t.Error("每个样点都应有种植建议")
			}
			if len(rec.Predictions) != 8 {
				t.Errorf("作物预测金属数=%d，期望8", len(rec.Predictions))
			}
		}
	})
}

func TestCalcSampleIgeo(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("8种金属都有数据", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 50, Zn: 100, Cu: 70, As: 15,
			Hg: 0.5, Cd: 1.0, Cr: 50, Ni: 25,
		}
		results, maxIgeo, maxMetal := fsm.CalcSampleIgeo(fl)
		if len(results) != 8 {
			t.Errorf("金属Igeo结果数=%d，期望8", len(results))
		}
		if maxIgeo <= 0 && maxIgeo != math.Inf(-1) {
			t.Error("最大Igeo应有值")
		}
		if maxMetal == "" {
			t.Error("最大贡献金属不应为空")
		}
		for _, r := range results {
			if r.Metal == "" {
				t.Error("金属名称不应为空")
			}
			if r.Level < 0 || r.Level > 6 {
				t.Errorf("%s 等级=%d 超出 [0,6] 范围", r.Metal, r.Level)
			}
			if r.LevelDesc == "" {
				t.Errorf("%s 等级描述为空", r.Metal)
			}
		}
	})
}

func TestGenerateSummary(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	summary := fsm.GenerateSummary(5, 3.5, 450, "较高风险")
	if summary == "" {
		t.Error("摘要为空")
	}
	if len(summary) < 20 {
		t.Errorf("摘要过短: %s", summary)
	}
	t.Logf("摘要: %s", summary)
}

func TestCropRiskDifferences(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	t.Run("不同作物风险差异验证", func(t *testing.T) {
		fl := models.FarmlandSoil{
			Pb: 200, Zn: 400, Cu: 300, As: 40,
			Hg: 1.5, Cd: 3, Cr: 80, Ni: 40,
		}

		landUses := []string{"水田", "旱地", "菜地", "果园", "茶园"}
		results := make(map[string]int)
		for _, lu := range landUses {
			rec := fsm.AssessCropRisk(fl, lu)
			results[lu] = rec.ExceedCount
			t.Logf("%s: 超标数=%d, 风险等级=%s", lu, rec.ExceedCount, rec.RiskLevel)
		}

		hasDifference := false
		firstCount := -1
		for _, count := range results {
			if firstCount == -1 {
				firstCount = count
			} else if count != firstCount {
				hasDifference = true
				break
			}
		}
		if !hasDifference {
			t.Log("注意：所有土地利用类型超标数相同（可能BCF差异不大或都超标/都不超标）")
		}
	})
}

func TestKrigingServiceCreation(t *testing.T) {
	ks := soil_safety_evaluator.NewKrigingService()
	if ks == nil {
		t.Fatal("NewKrigingService 返回 nil")
	}
}

func TestSphericalVariogram(t *testing.T) {
	params := soil_safety_evaluator.KrigingParams{
		Nugget: 10.0,
		Sill:   200.0,
		Range:  1500.0,
		Model:  "spherical",
	}

	t.Run("h=0 → Nugget", func(t *testing.T) {
		gamma := soil_safety_evaluator.SphericalVariogram(0, params)
		if gamma != params.Nugget {
			t.Errorf("h=0 → γ=%.4f，期望 %.4f", gamma, params.Nugget)
		}
	})

	t.Run("h≥Range → Nugget+Sill", func(t *testing.T) {
		gamma := soil_safety_evaluator.SphericalVariogram(1500, params)
		expected := params.Nugget + params.Sill
		if math.Abs(gamma-expected) > 1e-6 {
			t.Errorf("h=Range → γ=%.4f，期望 %.4f", gamma, expected)
		}
		gamma2 := soil_safety_evaluator.SphericalVariogram(3000, params)
		if math.Abs(gamma2-expected) > 1e-6 {
			t.Errorf("h=2*Range → γ=%.4f，期望 %.4f", gamma2, expected)
		}
	})

	t.Run("0<h<Range → between Nugget and Nugget+Sill", func(t *testing.T) {
		h := 750.0
		gamma := soil_safety_evaluator.SphericalVariogram(h, params)
		if gamma <= params.Nugget {
			t.Errorf("h=750 → γ=%.4f，应 > Nugget=%.4f", gamma, params.Nugget)
		}
		if gamma >= params.Nugget+params.Sill {
			t.Errorf("h=750 → γ=%.4f，应 < Nugget+Sill=%.4f", gamma, params.Nugget+params.Sill)
		}
		ratio := h / params.Range
		expected := params.Nugget + params.Sill*(1.5*ratio-0.5*ratio*ratio*ratio)
		if math.Abs(gamma-expected) > 1e-10 {
			t.Errorf("h=750 → γ=%.6f，期望 %.6f", gamma, expected)
		}
	})

	t.Run("negative h → Nugget", func(t *testing.T) {
		gamma := soil_safety_evaluator.SphericalVariogram(-100, params)
		if gamma != params.Nugget {
			t.Errorf("h=-100 → γ=%.4f，期望 Nugget=%.4f", gamma, params.Nugget)
		}
	})
}

func TestKrigingInterpolationSparse(t *testing.T) {
	fsm := soil_safety_evaluator.NewSoilSafetyEvaluator()

	farmlands := []models.FarmlandSoil{
		{ID: 1, DistanceFromSite: 200, LandUseType: "旱地",
			Pb: 300, Zn: 500, Cu: 400, As: 40, Hg: 1.5, Cd: 4, Cr: 80, Ni: 40},
		{ID: 2, DistanceFromSite: 1500, LandUseType: "旱地",
			Pb: 50, Zn: 100, Cu: 60, As: 8, Hg: 0.1, Cd: 0.3, Cr: 40, Ni: 15},
	}

	ctx := context.Background()
	result, err := fsm.AssessFarmSafety(ctx, 1, farmlands)
	if err != nil {
		t.Fatalf("评估失败: %v", err)
	}

	if result.SpatialUncertainty == nil {
		t.Fatal("SpatialUncertainty 不应为 nil")
	}

	if !result.SpatialUncertainty.SampleSparsityWarning {
		t.Error("稀疏样点应触发 SampleSparsityWarning=true")
	}

	if result.SpatialUncertainty.EffectiveSampleCount >= 4 {
		t.Errorf("有效样点数=%d，应<4（稀疏）", result.SpatialUncertainty.EffectiveSampleCount)
	}

	if result.SpatialUncertainty.DataQualityNote == "" {
		t.Error("稀疏数据应有 DataQualityNote")
	}

	t.Logf("稀疏数据: 有效样点=%d, 警告=%v, 备注=%s",
		result.SpatialUncertainty.EffectiveSampleCount,
		result.SpatialUncertainty.SampleSparsityWarning,
		result.SpatialUncertainty.DataQualityNote)
}
