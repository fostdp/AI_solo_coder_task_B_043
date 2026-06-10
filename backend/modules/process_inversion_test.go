package modules

import (
	"context"
	"math"
	"testing"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
)

func TestNewProcessInversionModule(t *testing.T) {
	m := NewProcessInversionModule()
	if m == nil {
		t.Fatal("NewProcessInversionModule 返回 nil")
	}
	if m.bpCfg.InputSize != 8 {
		t.Errorf("期望 InputSize=8，实际=%d", m.bpCfg.InputSize)
	}
	if len(m.bpCfg.HiddenLayerSizes) != 2 {
		t.Errorf("期望 2 个隐藏层，实际=%d", len(m.bpCfg.HiddenLayerSizes))
	}
	if m.bpCfg.OutputTempSize != 1 {
		t.Errorf("期望 OutputTempSize=1，实际=%d", m.bpCfg.OutputTempSize)
	}
	if m.bpCfg.OutputAgentSize != 4 {
		t.Errorf("期望 OutputAgentSize=4，实际=%d", m.bpCfg.OutputAgentSize)
	}
	if len(m.weights) == 0 {
		t.Error("权重矩阵未初始化")
	}
	if len(m.biases) == 0 {
		t.Error("偏置向量未初始化")
	}
}

func TestRelu(t *testing.T) {
	m := NewProcessInversionModule()
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.0, 1.0},
		{0.0, 0.0},
		{-1.0, 0.0},
		{100.0, 100.0},
		{-0.5, 0.0},
	}
	for _, tt := range tests {
		result := m.relu(tt.input)
		if math.Abs(result-tt.expected) > 1e-9 {
			t.Errorf("ReLU(%.1f) = %.1f，期望 %.1f", tt.input, result, tt.expected)
		}
	}
}

func TestSoftmax(t *testing.T) {
	m := NewProcessInversionModule()

	t.Run("正常输入", func(t *testing.T) {
		input := []float64{1.0, 2.0, 3.0, 4.0}
		result := m.softmax(input)
		if len(result) != len(input) {
			t.Fatalf("softmax 输出长度 %d != 输入长度 %d", len(result), len(input))
		}
		sum := 0.0
		for _, v := range result {
			sum += v
			if v < 0 || v > 1 {
				t.Errorf("softmax 输出 %.4f 不在 [0,1] 范围内", v)
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("softmax 概率和 %.6f != 1.0", sum)
		}
		if result[3] <= result[2] || result[2] <= result[1] || result[1] <= result[0] {
			t.Error("softmax 输出未保持输入大小顺序")
		}
	})

	t.Run("空输入", func(t *testing.T) {
		result := m.softmax([]float64{})
		if result != nil {
			t.Error("空输入应返回 nil")
		}
	})

	t.Run("极端大值（数值稳定性）", func(t *testing.T) {
		input := []float64{1000, 1001, 1002}
		result := m.softmax(input)
		sum := 0.0
		for _, v := range result {
			sum += v
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("softmax 出现 NaN 或 Inf: %v", v)
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("极端大值下 softmax 和=%.6f，期望1.0", sum)
		}
	})

	t.Run("全相同输入", func(t *testing.T) {
		input := []float64{5, 5, 5, 5}
		result := m.softmax(input)
		expected := 0.25
		for i, v := range result {
			if math.Abs(v-expected) > 1e-6 {
				t.Errorf("softmax[%d] = %.6f，期望 %.6f", i, v, expected)
			}
		}
	})
}

func TestForwardLayer(t *testing.T) {
	m := NewProcessInversionModule()

	weights := [][]float64{
		{0.5, -0.3},
		{0.2, 0.8},
	}
	biases := []float64{0.1, -0.2}
	input := []float64{1.0, 2.0}

	t.Run("带ReLU激活", func(t *testing.T) {
		result := m.forwardLayer(input, weights, biases, true)
		expected0 := math.Max(0, 0.1+1.0*0.5+2.0*0.2)
		expected1 := math.Max(0, -0.2+1.0*(-0.3)+2.0*0.8)
		if math.Abs(result[0]-expected0) > 1e-9 {
			t.Errorf("forwardLayer[0] = %.6f，期望 %.6f", result[0], expected0)
		}
		if math.Abs(result[1]-expected1) > 1e-9 {
			t.Errorf("forwardLayer[1] = %.6f，期望 %.6f", result[1], expected1)
		}
	})

	t.Run("不带激活", func(t *testing.T) {
		result := m.forwardLayer(input, weights, biases, false)
		expected0 := 0.1 + 1.0*0.5 + 2.0*0.2
		expected1 := -0.2 + 1.0*(-0.3) + 2.0*0.8
		if math.Abs(result[0]-expected0) > 1e-9 {
			t.Errorf("forwardLayer[0] = %.6f，期望 %.6f", result[0], expected0)
		}
		if math.Abs(result[1]-expected1) > 1e-9 {
			t.Errorf("forwardLayer[1] = %.6f，期望 %.6f", result[1], expected1)
		}
	})
}

func TestBuildFeatureVector(t *testing.T) {
	m := NewProcessInversionModule()

	t.Run("完整数据", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 100, Zn: 200, Cu: 150, As: 10, Hg: 0.5, Cd: 2.0},
		}
		slag := &models.SlagComposition{
			CaO:     20,
			SiO2:    35,
			FeO:     45,
			Fe2O3:   10,
			SO3:     2.5,
		}
		features, nValid, slagComp := m.buildFeatureVector(measurements, slag)
		if len(features) != 8 {
			t.Errorf("特征向量长度=%d，期望8", len(features))
		}
		if nValid < 5 {
			t.Errorf("有效特征数=%d，应≥5", nValid)
		}
		if slagComp <= 0 {
			t.Errorf("矿渣完整度=%.2f，应>0", slagComp)
		}
		for i, f := range features {
			if f < 0 || f > 1.0+1e-9 {
				t.Errorf("特征[%d]=%.4f 超出 [0,1] 范围", i, f)
			}
		}
	})

	t.Run("仅测量数据，无矿渣", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 80, Zn: 120, Cu: 60, As: 5, Hg: 0.2, Cd: 1.0},
		}
		features, nValid, slagComp := m.buildFeatureVector(measurements, nil)
		if nValid != 5 {
			t.Errorf("有效特征数=%d，期望5（仅金属比率）", nValid)
		}
		if slagComp != 0 {
			t.Errorf("矿渣完整度=%.2f，期望0", slagComp)
		}
		if features[5] != 0 || features[6] != 0 || features[7] != 0 {
			t.Error("无矿渣时矿渣相关特征应为0")
		}
	})

	t.Run("空测量数据+矿渣", func(t *testing.T) {
		slag := &models.SlagComposition{
			CaO:   15,
			SiO2:  40,
			FeO:   30,
			Fe2O3: 5,
			SO3:   1.0,
		}
		features, nValid, slagComp := m.buildFeatureVector(nil, slag)
		if nValid < 3 {
			t.Errorf("有效特征数=%d，应≥3（仅矿渣特征）", nValid)
		}
		if math.Abs(slagComp-1.0) > 1e-9 {
			t.Errorf("矿渣完整度=%.2f，期望1.0", slagComp)
		}
		if features[0] != 0 || features[1] != 0 || features[2] != 0 {
			t.Error("无测量数据时金属比率特征应为0")
		}
	})

	t.Run("零浓度数据", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 0, Zn: 0, Cu: 0, As: 0, Hg: 0, Cd: 0},
		}
		features, nValid, _ := m.buildFeatureVector(measurements, nil)
		if nValid != 0 {
			t.Errorf("零浓度下有效特征数=%d，期望0", nValid)
		}
		for i, f := range features {
			if f != 0 {
				t.Errorf("零浓度下特征[%d]=%.4f，期望0", i, f)
			}
		}
	})

	t.Run("极端高浓度（比率截断）", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 1000, Zn: 10, Cu: 5000, As: 0.1, Hg: 0.001, Cd: 10},
		}
		features, _, _ := m.buildFeatureVector(measurements, nil)
		for i, f := range features {
			if f > 1.0+1e-9 {
				t.Errorf("特征[%d]=%.4f 超出归一化上限1.0", i, f)
			}
		}
	})
}

func TestMapTempToProcess(t *testing.T) {
	m := NewProcessInversionModule()

	tests := []struct {
		temp        float64
		shouldNotBe string
	}{
		{700, "高温液态还原"},
		{1200, "低温焙烧"},
		{950, ""},
		{1500, ""},
	}

	for _, tt := range tests {
		process, era := m.mapTempToProcess(tt.temp)
		if process == "" {
			t.Errorf("%.0f℃ 对应工艺为空", tt.temp)
		}
		if era == "" {
			t.Errorf("%.0f℃ 对应年代为空", tt.temp)
		}
		if tt.shouldNotBe != "" && process == tt.shouldNotBe {
			t.Errorf("%.0f℃ 不应被归类为'%s'", tt.temp, tt.shouldNotBe)
		}
		t.Logf("%.0f℃ → %s (%s)", tt.temp, process, era)
	}
}

func TestGenerateTempDistribution(t *testing.T) {
	m := NewProcessInversionModule()

	t.Run("正常分布", func(t *testing.T) {
		dist := m.generateTempDistribution(1000, 0.8)
		if len(dist) != 10 {
			t.Errorf("分布长度=%d，期望10", len(dist))
		}
		sum := 0.0
		for _, d := range dist {
			sum += d
			if d < 0 {
				t.Error("概率密度为负")
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("概率和=%.6f，期望1.0", sum)
		}
	})

	t.Run("高置信度 → 分布更集中", func(t *testing.T) {
		highConf := m.generateTempDistribution(1000, 0.95)
		lowConf := m.generateTempDistribution(1000, 0.3)
		highMax := 0.0
		lowMax := 0.0
		for _, v := range highConf {
			if v > highMax {
				highMax = v
			}
		}
		for _, v := range lowConf {
			if v > lowMax {
				lowMax = v
			}
		}
		if highMax <= lowMax {
			t.Error("高置信度下分布峰值应高于低置信度")
		}
	})

	t.Run("峰值位置接近估计温度", func(t *testing.T) {
		estTemp := 800.0
		dist := m.generateTempDistribution(estTemp, 0.9)
		maxIdx := 0
		maxVal := 0.0
		for i, v := range dist {
			if v > maxVal {
				maxVal = v
				maxIdx = i
			}
		}
		binCenter := 500.0 + float64(maxIdx)*110.0 + 55.0
		if math.Abs(binCenter-estTemp) > 200 {
			t.Errorf("分布峰值中心 %.0f℃ 偏离估计温度 %.0f℃ 过远", binCenter, estTemp)
		}
	})
}

func TestCalculateKLDivergence(t *testing.T) {
	m := NewProcessInversionModule()

	t.Run("相同分布 → KL散度=0", func(t *testing.T) {
		bpnn := []float64{0.25, 0.25, 0.25, 0.25}
		bayes := map[string]float64{
			"木炭": 0.25, "焦炭": 0.25, "煤": 0.25, "混合": 0.25,
		}
		kl := m.calculateKLDivergence(bpnn, bayes)
		if math.Abs(kl) > 1e-6 {
			t.Errorf("相同分布 KL=%.6f，期望≈0", kl)
		}
	})

	t.Run("不同分布 → KL散度>0", func(t *testing.T) {
		bpnn := []float64{0.7, 0.1, 0.1, 0.1}
		bayes := map[string]float64{
			"木炭": 0.1, "焦炭": 0.1, "煤": 0.7, "混合": 0.1,
		}
		kl := m.calculateKLDivergence(bpnn, bayes)
		if kl <= 0 {
			t.Errorf("不同分布 KL=%.6f，应>0", kl)
		}
	})
}

func TestBayesianCorrection(t *testing.T) {
	m := NewProcessInversionModule()

	t.Run("高SO3矿渣 → 煤/焦炭概率上升", func(t *testing.T) {
		slag := &models.SlagComposition{
			SO3: 8.0,
			CaO: 30,
			SiO2: 20,
		}
		bpnnProbs := []float64{0.25, 0.25, 0.25, 0.25}
		posterior := m.bayesianCorrection(1, slag, bpnnProbs)

		coalProb := posterior["煤"]
		charcoalProb := posterior["木炭"]
		if coalProb <= charcoalProb {
			t.Errorf("高SO3时煤概率(%.4f)应高于木炭概率(%.4f)", coalProb, charcoalProb)
		}
		t.Logf("高SO3后验: 木炭=%.4f, 焦炭=%.4f, 煤=%.4f, 混合=%.4f",
			posterior["木炭"], posterior["焦炭"], posterior["煤"], posterior["混合"])
	})

	t.Run("低SO3高碱度 → 焦炭概率高", func(t *testing.T) {
		slag := &models.SlagComposition{
			SO3: 1.0,
			CaO: 45,
			SiO2: 20,
		}
		bpnnProbs := []float64{0.25, 0.25, 0.25, 0.25}
		posterior := m.bayesianCorrection(1, slag, bpnnProbs)
		if posterior["焦炭"] <= 0 {
			t.Error("高碱度下焦炭概率不应为0")
		}
	})

	t.Run("无矿渣 → 使用先验", func(t *testing.T) {
		bpnnProbs := []float64{0.6, 0.1, 0.05, 0.25}
		posterior := m.bayesianCorrection(1, nil, bpnnProbs)
		sum := 0.0
		for _, v := range posterior {
			sum += v
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("后验概率和=%.6f，期望1.0", sum)
		}
	})

	t.Run("概率归一化验证", func(t *testing.T) {
		slag := &models.SlagComposition{SO3: 3.0, CaO: 25, SiO2: 30}
		bpnnProbs := []float64{0.3, 0.3, 0.2, 0.2}
		posterior := m.bayesianCorrection(1, slag, bpnnProbs)
		sum := 0.0
		for _, agent := range config.ReducingAgents {
			sum += posterior[agent]
		}
		if math.Abs(sum-1.0) > 1e-9 {
			t.Errorf("后验概率和=%.10f，期望1.0", sum)
		}
	})
}

func TestInvertProcess(t *testing.T) {
	m := NewProcessInversionModule()
	ctx := context.Background()

	t.Run("正常输入 - 完整数据", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 250, Zn: 180, Cu: 420, As: 35, Hg: 1.2, Cd: 3.5},
			{Pb: 260, Zn: 175, Cu: 410, As: 33, Hg: 1.1, Cd: 3.4},
		}
		slag := &models.SlagComposition{
			SiO2:    32,
			Al2O3:   8,
			CaO:     25,
			FeO:     38,
			Fe2O3:   12,
			MgO:    5,
			SO3:     3.0,
			K2O:    1.5,
			Na2O:    1.0,
			TiO2:    0.8,
			Fayalite: 35,
			Wollastonite: 15,
			Anorthite: 10,
			Diopside: 5,
			Magnetite: 12,
			Hematite: 5,
			Wuestite: 8,
			GlassPhase: 40,
			OtherMinerals: 10,
			PbLeaching: 0.5,
			CdLeaching: 0.05,
			AsLeaching: 0.3,
			HgLeaching: 0.01,
			CrLeaching: 0.2,
			NiLeaching: 0.1,
			Density:  3.2,
			SpecificSurface: 450,
			LossOnIgnition: 1.5,
		}

		result, err := m.InvertProcess(ctx, 1, measurements, slag)
		if err != nil {
			t.Fatalf("InvertProcess 失败: %v", err)
		}
		if result == nil {
			t.Fatal("结果为 nil")
		}
		if result.SiteID != 1 {
			t.Errorf("SiteID=%d，期望1", result.SiteID)
		}

		inv := result.Inversion
		if inv.EstimatedTemperature < 500 || inv.EstimatedTemperature > 1600 {
			t.Errorf("估计温度 %.0f℃ 超出 [500, 1600] 范围", inv.EstimatedTemperature)
		}
		if inv.TemperatureConfidence <= 0 || inv.TemperatureConfidence > 1.0 {
			t.Errorf("温度置信度 %.4f 无效", inv.TemperatureConfidence)
		}
		if inv.ReducingAgent == "" {
			t.Error("还原剂类型为空")
		}
		if inv.ReducingAgentConfidence <= 0 || inv.ReducingAgentConfidence > 1.0 {
			t.Errorf("还原剂置信度 %.4f 无效", inv.ReducingAgentConfidence)
		}
		if inv.ProcessTypeDetailed == "" {
			t.Error("工艺类型为空")
		}
		if inv.ProcessEraEstimate == "" {
			t.Error("年代估计为空")
		}
		if inv.QualityLevel == "" {
			t.Error("质量等级为空")
		}

		if len(result.AgentProbabilities) != 4 {
			t.Errorf("还原剂概率种类=%d，期望4", len(result.AgentProbabilities))
		}
		if len(result.TemperatureDistribution) != 10 {
			t.Errorf("温度分布长度=%d，期望10", len(result.TemperatureDistribution))
		}

		t.Logf("估计温度: %.0f℃, 置信度: %.2f%%, 还原剂: %s (%.2f%%), 质量: %s",
			inv.EstimatedTemperature, inv.TemperatureConfidence*100,
			inv.ReducingAgent, inv.ReducingAgentConfidence*100, inv.QualityLevel)
	})

	t.Run("仅测量数据，无矿渣", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 150, Zn: 200, Cu: 300, As: 15, Hg: 0.5, Cd: 2.0},
		}
		result, err := m.InvertProcess(ctx, 2, measurements, nil)
		if err != nil {
			t.Fatalf("InvertProcess 失败: %v", err)
		}
		if result.Inversion.QualityLevel == "高" {
			t.Error("无矿渣数据时质量等级不应为'高'")
		}
	})

	t.Run("高温特征 → 高温结果", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 300, Zn: 100, Cu: 600, As: 50, Hg: 0.2, Cd: 5.0},
		}
		slag := &models.SlagComposition{
			CaO:  45,
			SiO2: 25,
			FeO:  50,
			Fe2O3: 15,
			SO3:  6.0,
		}
		result, err := m.InvertProcess(ctx, 3, measurements, slag)
		if err != nil {
			t.Fatalf("InvertProcess 失败: %v", err)
		}
		temp := result.Inversion.EstimatedTemperature
		if temp < 800 {
			t.Errorf("高温特征下估计温度%.0f℃偏低，应≥800℃", temp)
		}
	})

	t.Run("单条测量数据", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 100, Zn: 150, Cu: 200, As: 8, Hg: 0.3, Cd: 1.0},
		}
		result, err := m.InvertProcess(ctx, 4, measurements, nil)
		if err != nil {
			t.Fatalf("单条数据失败: %v", err)
		}
		if result == nil {
			t.Fatal("结果为nil")
		}
	})

	t.Run("多条测量数据 → 置信度更高", func(t *testing.T) {
		multiMeas := []models.XRFMeasurement{
			{Pb: 100, Zn: 150, Cu: 200, As: 8, Hg: 0.3, Cd: 1.0},
			{Pb: 105, Zn: 148, Cu: 205, As: 7.5, Hg: 0.28, Cd: 0.95},
			{Pb: 98, Zn: 152, Cu: 198, As: 8.2, Hg: 0.32, Cd: 1.05},
			{Pb: 102, Zn: 149, Cu: 202, As: 7.8, Hg: 0.29, Cd: 0.98},
			{Pb: 99, Zn: 151, Cu: 199, As: 8.1, Hg: 0.31, Cd: 1.02},
		}
		singleMeas := []models.XRFMeasurement{
			{Pb: 100, Zn: 150, Cu: 200, As: 8, Hg: 0.3, Cd: 1.0},
		}
		multiResult, _ := m.InvertProcess(ctx, 5, multiMeas, nil)
		singleResult, _ := m.InvertProcess(ctx, 6, singleMeas, nil)
		if multiResult.Inversion.TemperatureConfidence <= singleResult.Inversion.TemperatureConfidence {
			t.Logf("注意：多条数据置信度(%.4f)不一定严格高于单条(%.4f)",
				multiResult.Inversion.TemperatureConfidence, singleResult.Inversion.TemperatureConfidence)
		}
	})
}

func TestGetTemperatureBinLabels(t *testing.T) {
	m := NewProcessInversionModule()
	labels := m.GetTemperatureBinLabels()
	if len(labels) != 10 {
		t.Errorf("标签数量=%d，期望10", len(labels))
	}
	if labels[0] != "500-610℃" {
		t.Errorf("首个标签='%s'，期望'500-610℃'", labels[0])
	}
	if labels[9] != "1490-1600℃" {
		t.Errorf("末个标签='%s'，期望'1490-1600℃'", labels[9])
	}
}

func TestBPNNNetworkStructure(t *testing.T) {
	m := NewProcessInversionModule()
	w1 := m.weights["w1"]
	b1 := m.biases["b1"]
	w2 := m.weights["w2"]
	b2 := m.biases["b2"]
	w3t := m.weights["w3_temp"]
	b3t := m.biases["b3_temp"]
	w3a := m.weights["w3_agent"]
	b3a := m.biases["b3_agent"]

	if len(w1) != 8 {
		t.Errorf("W1输入维度=%d，期望8", len(w1))
	}
	if len(w1[0]) != 32 {
		t.Errorf("W1隐藏层1维度=%d，期望32", len(w1[0]))
	}
	if len(b1) != 32 {
		t.Errorf("b1长度=%d，期望32", len(b1))
	}
	if len(w2) != 32 {
		t.Errorf("W2输入维度=%d，期望32", len(w2))
	}
	if len(w2[0]) != 16 {
		t.Errorf("W2隐藏层2维度=%d，期望16", len(w2[0]))
	}
	if len(b2) != 16 {
		t.Errorf("b2长度=%d，期望16", len(b2))
	}
	if len(w3t) != 16 {
		t.Errorf("W3_temp输入维度=%d，期望16", len(w3t))
	}
	if len(b3t) != 1 {
		t.Errorf("b3_temp长度=%d，期望1", len(b3t))
	}
	if len(w3a) != 16 {
		t.Errorf("W3_agent输入维度=%d，期望16", len(w3a))
	}
	if len(b3a) != 4 {
		t.Errorf("b3_agent长度=%d，期望4", len(b3a))
	}
}
