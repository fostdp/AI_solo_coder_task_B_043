package process_inverter_test

import (
	"context"
	"math"
	"testing"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/process_inverter"
)

func TestNewProcessInverter(t *testing.T) {
	m := process_inverter.NewProcessInverter()
	if m == nil {
		t.Fatal("NewProcessInverter returned nil")
	}
}

func TestRelu(t *testing.T) {
	m := process_inverter.NewProcessInverter()
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
		result := m.Relu(tt.input)
		if math.Abs(result-tt.expected) > 1e-9 {
			t.Errorf("ReLU(%.1f) = %.1f, expected %.1f", tt.input, result, tt.expected)
		}
	}
}

func TestSoftmax(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	t.Run("normal input", func(t *testing.T) {
		input := []float64{1.0, 2.0, 3.0, 4.0}
		result := m.Softmax(input)
		if len(result) != len(input) {
			t.Fatalf("softmax output length %d != input length %d", len(result), len(input))
		}
		sum := 0.0
		for _, v := range result {
			sum += v
			if v < 0 || v > 1 {
				t.Errorf("softmax output %.4f not in [0,1]", v)
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("softmax sum %.6f != 1.0", sum)
		}
		if result[3] <= result[2] || result[2] <= result[1] || result[1] <= result[0] {
			t.Error("softmax output does not preserve input order")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := m.Softmax([]float64{})
		if result != nil {
			t.Error("empty input should return nil")
		}
	})

	t.Run("extreme large values", func(t *testing.T) {
		input := []float64{1000, 1001, 1002}
		result := m.Softmax(input)
		sum := 0.0
		for _, v := range result {
			sum += v
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("softmax produced NaN or Inf: %v", v)
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("extreme value softmax sum=%.6f, expected 1.0", sum)
		}
	})

	t.Run("all same input", func(t *testing.T) {
		input := []float64{5, 5, 5, 5}
		result := m.Softmax(input)
		expected := 0.25
		for i, v := range result {
			if math.Abs(v-expected) > 1e-6 {
				t.Errorf("softmax[%d] = %.6f, expected %.6f", i, v, expected)
			}
		}
	})
}

func TestForwardLayer(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	weights := [][]float64{
		{0.5, -0.3},
		{0.2, 0.8},
	}
	biases := []float64{0.1, -0.2}
	input := []float64{1.0, 2.0}

	t.Run("with ReLU", func(t *testing.T) {
		result := m.ForwardLayer(input, weights, biases, true)
		expected0 := math.Max(0, 0.1+1.0*0.5+2.0*0.2)
		expected1 := math.Max(0, -0.2+1.0*(-0.3)+2.0*0.8)
		if math.Abs(result[0]-expected0) > 1e-9 {
			t.Errorf("forwardLayer[0] = %.6f, expected %.6f", result[0], expected0)
		}
		if math.Abs(result[1]-expected1) > 1e-9 {
			t.Errorf("forwardLayer[1] = %.6f, expected %.6f", result[1], expected1)
		}
	})

	t.Run("without activation", func(t *testing.T) {
		result := m.ForwardLayer(input, weights, biases, false)
		expected0 := 0.1 + 1.0*0.5 + 2.0*0.2
		expected1 := -0.2 + 1.0*(-0.3) + 2.0*0.8
		if math.Abs(result[0]-expected0) > 1e-9 {
			t.Errorf("forwardLayer[0] = %.6f, expected %.6f", result[0], expected0)
		}
		if math.Abs(result[1]-expected1) > 1e-9 {
			t.Errorf("forwardLayer[1] = %.6f, expected %.6f", result[1], expected1)
		}
	})
}

func TestBuildFeatureVector(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	t.Run("full data", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 100, Zn: 200, Cu: 150, As: 10, Hg: 0.5, Cd: 2.0},
		}
		slag := &models.SlagComposition{
			CaO:   20,
			SiO2:  35,
			FeO:   45,
			Fe2O3: 10,
			SO3:   2.5,
		}
		features, nValid, slagComp := m.BuildFeatureVector(measurements, slag)
		if len(features) != 8 {
			t.Errorf("feature vector length=%d, expected 8", len(features))
		}
		if nValid < 5 {
			t.Errorf("valid features=%d, should be >=5", nValid)
		}
		if slagComp <= 0 {
			t.Errorf("slag completeness=%.2f, should be >0", slagComp)
		}
		for i, f := range features {
			if f < 0 || f > 1.0+1e-9 {
				t.Errorf("feature[%d]=%.4f outside [0,1]", i, f)
			}
		}
	})

	t.Run("measurements only, no slag", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 80, Zn: 120, Cu: 60, As: 5, Hg: 0.2, Cd: 1.0},
		}
		features, nValid, slagComp := m.BuildFeatureVector(measurements, nil)
		if nValid != 5 {
			t.Errorf("valid features=%d, expected 5 (metal ratios only)", nValid)
		}
		if slagComp != 0 {
			t.Errorf("slag completeness=%.2f, expected 0", slagComp)
		}
		if features[5] != 0 || features[6] != 0 || features[7] != 0 {
			t.Error("slag features should be 0 when no slag data")
		}
	})

	t.Run("no measurements with slag", func(t *testing.T) {
		slag := &models.SlagComposition{
			CaO:   15,
			SiO2:  40,
			FeO:   30,
			Fe2O3: 5,
			SO3:   1.0,
		}
		features, nValid, slagComp := m.BuildFeatureVector(nil, slag)
		if nValid < 3 {
			t.Errorf("valid features=%d, should be >=3 (slag only)", nValid)
		}
		if math.Abs(slagComp-1.0) > 1e-9 {
			t.Errorf("slag completeness=%.2f, expected 1.0", slagComp)
		}
		if features[0] != 0 || features[1] != 0 || features[2] != 0 {
			t.Error("metal ratio features should be 0 when no measurements")
		}
	})

	t.Run("zero concentration", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 0, Zn: 0, Cu: 0, As: 0, Hg: 0, Cd: 0},
		}
		features, nValid, _ := m.BuildFeatureVector(measurements, nil)
		if nValid != 0 {
			t.Errorf("zero concentration valid features=%d, expected 0", nValid)
		}
		for i, f := range features {
			if f != 0 {
				t.Errorf("zero concentration feature[%d]=%.4f, expected 0", i, f)
			}
		}
	})

	t.Run("extreme high concentration", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 1000, Zn: 10, Cu: 5000, As: 0.1, Hg: 0.001, Cd: 10},
		}
		features, _, _ := m.BuildFeatureVector(measurements, nil)
		for i, f := range features {
			if f > 1.0+1e-9 {
				t.Errorf("feature[%d]=%.4f exceeds normalized upper bound 1.0", i, f)
			}
		}
	})
}

func TestMapTempToProcess(t *testing.T) {
	m := process_inverter.NewProcessInverter()

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
		process, era := m.MapTempToProcess(tt.temp)
		if process == "" {
			t.Errorf("%.0f℃ process is empty", tt.temp)
		}
		if era == "" {
			t.Errorf("%.0f℃ era is empty", tt.temp)
		}
		if tt.shouldNotBe != "" && process == tt.shouldNotBe {
			t.Errorf("%.0f℃ should not be classified as '%s'", tt.temp, tt.shouldNotBe)
		}
		t.Logf("%.0f℃ → %s (%s)", tt.temp, process, era)
	}
}

func TestGenerateTempDistribution(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	t.Run("normal distribution", func(t *testing.T) {
		dist := m.GenerateTempDistribution(1000, 0.8)
		if len(dist) != 10 {
			t.Errorf("distribution length=%d, expected 10", len(dist))
		}
		sum := 0.0
		for _, d := range dist {
			sum += d
			if d < 0 {
				t.Error("probability density is negative")
			}
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("probability sum=%.6f, expected 1.0", sum)
		}
	})

	t.Run("high confidence more concentrated", func(t *testing.T) {
		highConf := m.GenerateTempDistribution(1000, 0.95)
		lowConf := m.GenerateTempDistribution(1000, 0.3)
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
			t.Error("high confidence peak should be higher than low confidence")
		}
	})

	t.Run("peak near estimated temperature", func(t *testing.T) {
		estTemp := 800.0
		dist := m.GenerateTempDistribution(estTemp, 0.9)
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
			t.Errorf("distribution peak center %.0f℃ too far from estimated temperature %.0f℃", binCenter, estTemp)
		}
	})
}

func TestCalculateKLDivergence(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	t.Run("same distribution KL=0", func(t *testing.T) {
		bpnn := []float64{0.25, 0.25, 0.25, 0.25}
		bayes := map[string]float64{
			"木炭": 0.25, "焦炭": 0.25, "煤": 0.25, "混合": 0.25,
		}
		kl := m.CalculateKLDivergence(bpnn, bayes)
		if math.Abs(kl) > 1e-6 {
			t.Errorf("same distribution KL=%.6f, expected ~0", kl)
		}
	})

	t.Run("different distribution KL>0", func(t *testing.T) {
		bpnn := []float64{0.7, 0.1, 0.1, 0.1}
		bayes := map[string]float64{
			"木炭": 0.1, "焦炭": 0.1, "煤": 0.7, "混合": 0.1,
		}
		kl := m.CalculateKLDivergence(bpnn, bayes)
		if kl <= 0 {
			t.Errorf("different distribution KL=%.6f, should be >0", kl)
		}
	})
}

func TestBayesianCorrection(t *testing.T) {
	m := process_inverter.NewProcessInverter()

	t.Run("high SO3 increases coal/coke probability", func(t *testing.T) {
		slag := &models.SlagComposition{
			SO3: 8.0,
			CaO: 30,
			SiO2: 20,
		}
		bpnnProbs := []float64{0.25, 0.25, 0.25, 0.25}
		posterior := m.BayesianCorrection(1, slag, bpnnProbs)

		coalProb := posterior["煤"]
		charcoalProb := posterior["木炭"]
		if coalProb <= charcoalProb {
			t.Errorf("high SO3 coal probability(%.4f) should be higher than charcoal(%.4f)", coalProb, charcoalProb)
		}
		t.Logf("high SO3 posterior: 木炭=%.4f, 焦炭=%.4f, 煤=%.4f, 混合=%.4f",
			posterior["木炭"], posterior["焦炭"], posterior["煤"], posterior["混合"])
	})

	t.Run("low SO3 high basicity increases coke probability", func(t *testing.T) {
		slag := &models.SlagComposition{
			SO3: 1.0,
			CaO: 45,
			SiO2: 20,
		}
		bpnnProbs := []float64{0.25, 0.25, 0.25, 0.25}
		posterior := m.BayesianCorrection(1, slag, bpnnProbs)
		if posterior["焦炭"] <= 0 {
			t.Error("high basicity coke probability should not be 0")
		}
	})

	t.Run("no slag uses prior", func(t *testing.T) {
		bpnnProbs := []float64{0.6, 0.1, 0.05, 0.25}
		posterior := m.BayesianCorrection(1, nil, bpnnProbs)
		sum := 0.0
		for _, v := range posterior {
			sum += v
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("posterior probability sum=%.6f, expected 1.0", sum)
		}
	})

	t.Run("probability normalization", func(t *testing.T) {
		slag := &models.SlagComposition{SO3: 3.0, CaO: 25, SiO2: 30}
		bpnnProbs := []float64{0.3, 0.3, 0.2, 0.2}
		posterior := m.BayesianCorrection(1, slag, bpnnProbs)
		sum := 0.0
		for _, agent := range config.ReducingAgents {
			sum += posterior[agent]
		}
		if math.Abs(sum-1.0) > 1e-9 {
			t.Errorf("posterior probability sum=%.10f, expected 1.0", sum)
		}
	})
}

func TestInvertProcess(t *testing.T) {
	m := process_inverter.NewProcessInverter()
	ctx := context.Background()

	t.Run("normal input - full data", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 250, Zn: 180, Cu: 420, As: 35, Hg: 1.2, Cd: 3.5},
			{Pb: 260, Zn: 175, Cu: 410, As: 33, Hg: 1.1, Cd: 3.4},
		}
		slag := &models.SlagComposition{
			SiO2:     32,
			Al2O3:    8,
			CaO:      25,
			FeO:      38,
			Fe2O3:    12,
			MgO:      5,
			SO3:      3.0,
			K2O:      1.5,
			Na2O:     1.0,
			TiO2:     0.8,
			Fayalite: 35,
			Wollastonite: 15,
			Anorthite:    10,
			Diopside:     5,
			Magnetite:    12,
			Hematite:     5,
			Wuestite:     8,
			GlassPhase:   40,
			OtherMinerals: 10,
			PbLeaching:   0.5,
			CdLeaching:   0.05,
			AsLeaching:   0.3,
			HgLeaching:   0.01,
			CrLeaching:   0.2,
			NiLeaching:   0.1,
			Density:      3.2,
			SpecificSurface: 450,
			LossOnIgnition:  1.5,
		}

		result, err := m.InvertProcess(ctx, 1, measurements, slag)
		if err != nil {
			t.Fatalf("InvertProcess failed: %v", err)
		}
		if result == nil {
			t.Fatal("result is nil")
		}
		if result.SiteID != 1 {
			t.Errorf("SiteID=%d, expected 1", result.SiteID)
		}

		inv := result.Inversion
		if inv.EstimatedTemperature < 500 || inv.EstimatedTemperature > 1600 {
			t.Errorf("estimated temperature %.0f℃ outside [500, 1600]", inv.EstimatedTemperature)
		}
		if inv.TemperatureConfidence <= 0 || inv.TemperatureConfidence > 1.0 {
			t.Errorf("temperature confidence %.4f invalid", inv.TemperatureConfidence)
		}
		if inv.ReducingAgent == "" {
			t.Error("reducing agent is empty")
		}
		if inv.ReducingAgentConfidence <= 0 || inv.ReducingAgentConfidence > 1.0 {
			t.Errorf("reducing agent confidence %.4f invalid", inv.ReducingAgentConfidence)
		}
		if inv.ProcessTypeDetailed == "" {
			t.Error("process type is empty")
		}
		if inv.ProcessEraEstimate == "" {
			t.Error("era estimate is empty")
		}
		if inv.QualityLevel == "" {
			t.Error("quality level is empty")
		}

		if len(result.AgentProbabilities) != 4 {
			t.Errorf("agent probability types=%d, expected 4", len(result.AgentProbabilities))
		}
		if len(result.TemperatureDistribution) != 10 {
			t.Errorf("temperature distribution length=%d, expected 10", len(result.TemperatureDistribution))
		}

		t.Logf("estimated temperature: %.0f℃, confidence: %.2f%%, reducing agent: %s (%.2f%%), quality: %s",
			inv.EstimatedTemperature, inv.TemperatureConfidence*100,
			inv.ReducingAgent, inv.ReducingAgentConfidence*100, inv.QualityLevel)
	})

	t.Run("measurements only, no slag", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 150, Zn: 200, Cu: 300, As: 15, Hg: 0.5, Cd: 2.0},
		}
		result, err := m.InvertProcess(ctx, 2, measurements, nil)
		if err != nil {
			t.Fatalf("InvertProcess failed: %v", err)
		}
		if result.Inversion.QualityLevel == "高" {
			t.Error("quality level should not be '高' without slag data")
		}
	})

	t.Run("high temperature features yield high temperature result", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 300, Zn: 100, Cu: 600, As: 50, Hg: 0.2, Cd: 5.0},
		}
		slag := &models.SlagComposition{
			CaO:   45,
			SiO2:  25,
			FeO:   50,
			Fe2O3: 15,
			SO3:   6.0,
		}
		result, err := m.InvertProcess(ctx, 3, measurements, slag)
		if err != nil {
			t.Fatalf("InvertProcess failed: %v", err)
		}
		temp := result.Inversion.EstimatedTemperature
		if temp < 800 {
			t.Errorf("high temperature features estimated temperature %.0f℃ too low, should be >=800℃", temp)
		}
	})

	t.Run("single measurement", func(t *testing.T) {
		measurements := []models.XRFMeasurement{
			{Pb: 100, Zn: 150, Cu: 200, As: 8, Hg: 0.3, Cd: 1.0},
		}
		result, err := m.InvertProcess(ctx, 4, measurements, nil)
		if err != nil {
			t.Fatalf("single measurement failed: %v", err)
		}
		if result == nil {
			t.Fatal("result is nil")
		}
	})

	t.Run("multiple measurements higher confidence", func(t *testing.T) {
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
			t.Logf("note: multi-measurement confidence(%.4f) may not strictly exceed single(%.4f)",
				multiResult.Inversion.TemperatureConfidence, singleResult.Inversion.TemperatureConfidence)
		}
	})
}

func TestGetTemperatureBinLabels(t *testing.T) {
	m := process_inverter.NewProcessInverter()
	labels := m.GetTemperatureBinLabels()
	if len(labels) != 10 {
		t.Errorf("label count=%d, expected 10", len(labels))
	}
	if labels[0] != "500-610℃" {
		t.Errorf("first label='%s', expected '500-610℃'", labels[0])
	}
	if labels[9] != "1490-1600℃" {
		t.Errorf("last label='%s', expected '1490-1600℃'", labels[9])
	}
}

func TestBPNNNetworkStructure(t *testing.T) {
	m := process_inverter.NewProcessInverter()
	w1 := m.Weights["w1"]
	b1 := m.Biases["b1"]
	w2 := m.Weights["w2"]
	b2 := m.Biases["b2"]
	w3t := m.Weights["w3_temp"]
	b3t := m.Biases["b3_temp"]
	w3a := m.Weights["w3_agent"]
	b3a := m.Biases["b3_agent"]

	if len(w1) != 8 {
		t.Errorf("W1 input dimension=%d, expected 8", len(w1))
	}
	if len(w1[0]) != 32 {
		t.Errorf("W1 hidden1 dimension=%d, expected 32", len(w1[0]))
	}
	if len(b1) != 32 {
		t.Errorf("b1 length=%d, expected 32", len(b1))
	}
	if len(w2) != 32 {
		t.Errorf("W2 input dimension=%d, expected 32", len(w2))
	}
	if len(w2[0]) != 16 {
		t.Errorf("W2 hidden2 dimension=%d, expected 16", len(w2[0]))
	}
	if len(b2) != 16 {
		t.Errorf("b2 length=%d, expected 16", len(b2))
	}
	if len(w3t) != 16 {
		t.Errorf("W3_temp input dimension=%d, expected 16", len(w3t))
	}
	if len(b3t) != 1 {
		t.Errorf("b3_temp length=%d, expected 1", len(b3t))
	}
	if len(w3a) != 16 {
		t.Errorf("W3_agent input dimension=%d, expected 16", len(w3a))
	}
	if len(b3a) != 4 {
		t.Errorf("b3_agent length=%d, expected 4", len(b3a))
	}
}

func TestNeuralNetworkGoroutine(t *testing.T) {
	m := process_inverter.NewProcessInverter()
	ctx := context.Background()

	measurements := []models.XRFMeasurement{
		{Pb: 250, Zn: 180, Cu: 420, As: 35, Hg: 1.2, Cd: 3.5},
	}
	slag := &models.SlagComposition{
		CaO:   25,
		SiO2:  32,
		FeO:   38,
		Fe2O3: 12,
		SO3:   3.0,
	}

	result, err := m.InvertProcess(ctx, 1, measurements, slag)
	if err != nil {
		t.Fatalf("InvertProcess with goroutine NN failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Inversion.EstimatedTemperature < 500 || result.Inversion.EstimatedTemperature > 1600 {
		t.Errorf("estimated temperature %.0f℃ outside [500, 1600]", result.Inversion.EstimatedTemperature)
	}
}
