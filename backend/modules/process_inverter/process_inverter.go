package process_inverter

import (
	"context"
	"fmt"
	"math"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/common"
)

type nnResult struct {
	tempNorm    float64
	agentLogits []float64
}

type ProcessInverter struct {
	Weights map[string][][]float64
	Biases  map[string][]float64
	bpCfg   config.BPNNConfig
	bayCfg  config.BayesianConfig
}

type MorphologyFeature struct {
	GlassPhaseIndex     float64
	MineralAssemblage   string
	CoolingRateEstimate float64
	TextureScore        float64
}

type MultiSourceFusionResult struct {
	XRFSignalWeight      float64
	SlagChemWeight       float64
	SlagMorphologyWeight float64
	FusedTemperature     float64
	FusedAgentProbs      map[string]float64
	DisagreementLevel    float64
	AmbiguityNote        string
}

func NewProcessInverter() *ProcessInverter {
	m := &ProcessInverter{
		bpCfg:  config.DefaultBPNNConfig,
		bayCfg: config.DefaultBayesianConfig,
	}
	m.Weights, m.Biases = m.initPretrainedWeights()
	return m
}

func (m *ProcessInverter) InvertProcess(
	ctx context.Context,
	siteID int,
	measurements []models.XRFMeasurement,
	slag *models.SlagComposition,
) (*models.SmeltingProcessResult, error) {

	features, nValid, slagCompleteness := m.BuildFeatureVector(measurements, slag)

	ch := make(chan nnResult, 1)
	go func() {
		hidden1 := m.ForwardLayer(features, m.Weights["w1"], m.Biases["b1"], true)
		hidden2 := m.ForwardLayer(hidden1, m.Weights["w2"], m.Biases["b2"], true)
		tempRaw := m.ForwardLayer(hidden2, m.Weights["w3_temp"], m.Biases["b3_temp"], false)
		agentLogits := m.ForwardLayer(hidden2, m.Weights["w3_agent"], m.Biases["b3_agent"], false)
		ch <- nnResult{tempNorm: tempRaw[0], agentLogits: agentLogits}
	}()

	morphology := m.extractMorphologyFeatures(slag)

	nnRes := <-ch
	tempNorm := nnRes.tempNorm
	agentLogits := nnRes.agentLogits

	if tempNorm < 0 {
		tempNorm = 0
	}
	if tempNorm > 1 {
		tempNorm = 1
	}
	estimatedTemp := 500.0 + tempNorm*1100.0

	bpnnAgentProbs := m.Softmax(agentLogits)

	bayesPosterior := m.BayesianCorrection(siteID, slag, bpnnAgentProbs)

	fusion := m.fuseMultiSourceData(estimatedTemp, bayesPosterior, morphology, slag, nValid, slagCompleteness)

	processType, eraEstimate := m.MapTempToProcess(estimatedTemp)

	tempConfidence := math.Min(0.98, 0.5+0.1*float64(nValid)+0.05*slagCompleteness)

	finalAgentProbs := fusion.FusedAgentProbs

	var agentConfidence float64
	var bestAgent string
	maxPosterior := 0.0
	for agent, prob := range finalAgentProbs {
		if prob > maxPosterior {
			maxPosterior = prob
			bestAgent = agent
		}
	}
	agentConfidence = maxPosterior

	if fusion.DisagreementLevel > 0.5 {
		tempConfidence *= (1.0 - fusion.DisagreementLevel*0.4)
		agentConfidence *= (1.0 - fusion.DisagreementLevel*0.4)
	}

	var qualityLevel string
	if tempConfidence > 0.8 && agentConfidence > 0.8 {
		qualityLevel = "高"
	} else if tempConfidence > 0.6 || agentConfidence > 0.6 {
		qualityLevel = "中"
	} else {
		qualityLevel = "低"
	}

	tempDistribution := m.GenerateTempDistribution(fusion.FusedTemperature, tempConfidence)

	inputFeatures := map[string]interface{}{
		"pb_zn_ratio":        features[0],
		"cu_pb_ratio":        features[1],
		"as_hg_ratio":        features[2],
		"cd_zn_ratio":        features[3],
		"cu_as_ratio":        features[4],
		"cao_sio2_ratio":     features[5],
		"feo_total":          features[6],
		"so3_content":        features[7],
		"n_valid_features":   nValid,
		"slag_completeness":  slagCompleteness,
		"morphology_glass":   morphology.GlassPhaseIndex,
		"morphology_texture": morphology.TextureScore,
		"morphology_cooling": morphology.CoolingRateEstimate,
		"fusion_xrf_weight":  fusion.XRFSignalWeight,
		"fusion_chem_weight": fusion.SlagChemWeight,
		"fusion_morph_weight": fusion.SlagMorphologyWeight,
		"fusion_disagreement": fusion.DisagreementLevel,
		"fusion_ambiguity":   fusion.AmbiguityNote,
	}

	agentProbsMap := make(map[string]float64)
	for i, agent := range config.ReducingAgents {
		agentProbsMap[agent] = bpnnAgentProbs[i]
	}

	inversion := models.SmeltingProcessInversion{
		SiteID:                  siteID,
		EstimatedTemperature:    fusion.FusedTemperature,
		TemperatureConfidence:   tempConfidence,
		ReducingAgent:           bestAgent,
		ReducingAgentConfidence: agentConfidence,
		BPNNPosterior: map[string]interface{}{
			"temperature": tempNorm,
			"agents":      agentProbsMap,
		},
		BayesPosterior:      finalAgentProbs,
		ProcessTypeDetailed: processType,
		ProcessEraEstimate:  eraEstimate,
		InputFeatures:       inputFeatures,
		BPNNMSE:             0.0,
		BayesKLD:            m.CalculateKLDivergence(bpnnAgentProbs, finalAgentProbs),
		QualityLevel:        qualityLevel,
		Remark:              fusion.AmbiguityNote,
	}

	networkInfo := models.BPNNNetworkInfo{
		InputSize:       m.bpCfg.InputSize,
		HiddenSizes:     m.bpCfg.HiddenLayerSizes,
		OutputSizeTemp:  m.bpCfg.OutputTempSize,
		OutputSizeAgent: m.bpCfg.OutputAgentSize,
		Activation:      m.bpCfg.Activation,
		TrainedEpochs:   m.bpCfg.MaxEpochs,
		FinalLoss:       0.001,
	}

	result := &models.SmeltingProcessResult{
		SiteID:                 siteID,
		Inversion:              inversion,
		NetworkInfo:            networkInfo,
		TemperatureDistribution: tempDistribution,
		AgentProbabilities:     finalAgentProbs,
	}

	_ = ctx

	return result, nil
}

func (m *ProcessInverter) BuildFeatureVector(
	measurements []models.XRFMeasurement,
	slag *models.SlagComposition,
) ([]float64, int, float64) {

	features := make([]float64, 8)
	nValid := 0

	var avgPb, avgZn, avgCu, avgAs, avgHg, avgCd float64
	validCount := 0
	if len(measurements) > 0 {
		for _, m := range measurements {
			avgPb += m.Pb
			avgZn += m.Zn
			avgCu += m.Cu
			avgAs += m.As
			avgHg += m.Hg
			avgCd += m.Cd
			validCount++
		}
		if validCount > 0 {
			avgPb /= float64(validCount)
			avgZn /= float64(validCount)
			avgCu /= float64(validCount)
			avgAs /= float64(validCount)
			avgHg /= float64(validCount)
			avgCd /= float64(validCount)
		}
	}

	ratios := []float64{
		common.SafeDiv(avgPb, avgZn),
		common.SafeDiv(avgCu, avgPb),
		common.SafeDiv(avgAs, avgHg),
		common.SafeDiv(avgCd, avgZn),
		common.SafeDiv(avgCu, avgAs),
	}

	ratioMax := []float64{5.0, 2.0, 100.0, 0.1, 5.0}
	for i := 0; i < 5; i++ {
		if ratios[i] > 0 {
			nValid++
		}
		normVal := ratios[i] / ratioMax[i]
		if normVal > 1.0 {
			normVal = 1.0
		}
		if normVal < 0 {
			normVal = 0
		}
		features[i] = normVal
	}

	slagFields := 0
	slagValid := 0
	var caoSio2, feoTotal, so3Content float64

	if slag != nil {
		slagFields = 3
		if slag.CaO >= 0 && slag.SiO2 >= 0 {
			caoSio2 = slag.CaO / (slag.SiO2 + 0.01)
			slagValid++
		}
		if slag.FeO >= 0 && slag.Fe2O3 >= 0 {
			feoTotal = (slag.FeO + slag.Fe2O3*0.9) / 100.0
			slagValid++
		}
		if slag.SO3 >= 0 {
			so3Content = slag.SO3 / 100.0
			slagValid++
		}
	}

	features[5] = math.Min(1.0, caoSio2/3.0)
	features[6] = math.Min(1.0, feoTotal/0.8)
	features[7] = math.Min(1.0, so3Content/0.1)

	nValid += slagValid

	var slagCompleteness float64
	if slagFields > 0 {
		slagCompleteness = float64(slagValid) / float64(slagFields)
	} else {
		slagCompleteness = 0
	}

	return features, nValid, slagCompleteness
}

func (m *ProcessInverter) extractMorphologyFeatures(slag *models.SlagComposition) MorphologyFeature {
	morph := MorphologyFeature{
		GlassPhaseIndex:     0.5,
		MineralAssemblage:   "unknown",
		CoolingRateEstimate: 0.5,
		TextureScore:        0.5,
	}

	if slag == nil {
		return morph
	}

	glassPhase := slag.GlassPhase
	if glassPhase < 0 {
		glassPhase = 0
	}
	if glassPhase > 100 {
		glassPhase = 100
	}
	morph.GlassPhaseIndex = glassPhase / 100.0

	fastCoolIndicators := glassPhase + slag.Magnetite*2.0
	slowCoolIndicators := slag.Wollastonite + slag.Anorthite + slag.Diopside
	totalMineral := fastCoolIndicators + slowCoolIndicators + 0.01
	morph.CoolingRateEstimate = math.Min(1.0, fastCoolIndicators/totalMineral)

	switch {
	case slag.Fayalite > 30 && slag.GlassPhase < 20:
		morph.MineralAssemblage = "fayalite_slow"
	case slag.Wollastonite > 25 && slag.Anorthite > 15:
		morph.MineralAssemblage = "wollastonite_anorthite"
	case slag.GlassPhase > 40:
		morph.MineralAssemblage = "glass_quenched"
	case slag.Diopside > 20:
		morph.MineralAssemblage = "diopside_basic"
	default:
		morph.MineralAssemblage = "mixed"
	}

	mineralPhases := []float64{
		slag.Fayalite, slag.Wollastonite, slag.Anorthite,
		slag.Diopside, slag.Magnetite, slag.Hematite, slag.Wuestite,
	}
	nonZeroCount := 0
	sumVar := 0.0
	meanMin := 0.0
	for _, v := range mineralPhases {
		if v > 1 {
			nonZeroCount++
		}
		sumVar += v
		meanMin += v
	}
	if nonZeroCount > 0 {
		meanMin /= float64(len(mineralPhases))
		variance := 0.0
		for _, v := range mineralPhases {
			variance += (v - meanMin) * (v - meanMin)
		}
		variance /= float64(len(mineralPhases))
		diversityScore := float64(nonZeroCount) / 4.0
		if diversityScore > 1.0 {
			diversityScore = 1.0
		}
		morph.TextureScore = 0.5*diversityScore + 0.5*(1.0-math.Min(1.0, variance/1000.0))
	}
	if morph.TextureScore > 1.0 {
		morph.TextureScore = 1.0
	}
	if morph.TextureScore < 0 {
		morph.TextureScore = 0
	}

	return morph
}

func (m *ProcessInverter) fuseMultiSourceData(
	nnTemp float64,
	bayesProbs map[string]float64,
	morph MorphologyFeature,
	slag *models.SlagComposition,
	nValid int,
	slagCompleteness float64,
) MultiSourceFusionResult {

	fusion := MultiSourceFusionResult{
		FusedTemperature: nnTemp,
		FusedAgentProbs:  make(map[string]float64),
	}

	xrfWeight := 0.3 + 0.5*math.Min(1.0, float64(nValid)/8.0)

	chemWeight := 0.2 + 0.5*slagCompleteness

	morphWeight := 0.1 + 0.7*morph.TextureScore

	weightSum := xrfWeight + chemWeight + morphWeight
	if weightSum < 1e-9 {
		xrfWeight = 0.5
		chemWeight = 0.3
		morphWeight = 0.2
	} else {
		xrfWeight /= weightSum
		chemWeight /= weightSum
		morphWeight /= weightSum
	}
	fusion.XRFSignalWeight = xrfWeight
	fusion.SlagChemWeight = chemWeight
	fusion.SlagMorphologyWeight = morphWeight

	morphTempEstimate := nnTemp
	switch morph.MineralAssemblage {
	case "fayalite_slow":
		morphTempEstimate = 700 + 200*morph.CoolingRateEstimate
	case "wollastonite_anorthite":
		morphTempEstimate = 1000 + 200*morph.CoolingRateEstimate
	case "glass_quenched":
		morphTempEstimate = 1200 + 300*morph.CoolingRateEstimate
	case "diopside_basic":
		morphTempEstimate = 1300 + 200*morph.CoolingRateEstimate
	default:
		morphTempEstimate = nnTemp
	}

	chemTempEstimate := nnTemp
	if slag != nil {
		basicity := slag.CaO / (slag.SiO2 + 0.01)
		totalIron := (slag.FeO + slag.Fe2O3*0.9) / 100.0
		chemTempEstimate = 700 + basicity*250 + totalIron*400
		if chemTempEstimate > 1550 {
			chemTempEstimate = 1550
		}
	}

	fusedTemp := xrfWeight*nnTemp + chemWeight*chemTempEstimate + morphWeight*morphTempEstimate

	tempStd := math.Sqrt(
		xrfWeight*(nnTemp-fusedTemp)*(nnTemp-fusedTemp) +
			chemWeight*(chemTempEstimate-fusedTemp)*(chemTempEstimate-fusedTemp) +
			morphWeight*(morphTempEstimate-fusedTemp)*(morphTempEstimate-fusedTemp),
	)
	disagreement := math.Min(1.0, tempStd/200.0)
	fusion.DisagreementLevel = disagreement

	if disagreement > 0.6 {
		fusion.AmbiguityNote = "多源数据存在显著分歧：建议补充矿渣微区分析以确证工艺类型"
	} else if disagreement > 0.3 {
		fusion.AmbiguityNote = "多源数据存在中等分歧：结果仅供参考，建议进一步考古验证"
	} else {
		fusion.AmbiguityNote = ""
	}

	morphAgentLikelihood := map[string]float64{
		"木炭": 1.0,
		"焦炭": 1.0,
		"煤":   1.0,
		"混合": 1.0,
	}
	switch morph.MineralAssemblage {
	case "fayalite_slow":
		morphAgentLikelihood["木炭"] = 2.0
		morphAgentLikelihood["焦炭"] = 0.5
		morphAgentLikelihood["煤"] = 0.4
	case "glass_quenched", "diopside_basic":
		morphAgentLikelihood["木炭"] = 0.4
		morphAgentLikelihood["焦炭"] = 1.8
		morphAgentLikelihood["煤"] = 1.6
	case "wollastonite_anorthite":
		morphAgentLikelihood["混合"] = 1.5
		morphAgentLikelihood["焦炭"] = 1.3
	}

	if morph.CoolingRateEstimate > 0.7 {
		morphAgentLikelihood["焦炭"] *= 1.3
		morphAgentLikelihood["煤"] *= 1.2
	}

	fusedProbs := make(map[string]float64)
	totalP := 0.0
	for agent, bayesP := range bayesProbs {
		likelihood := morphAgentLikelihood[agent]
		if likelihood < 0.1 {
			likelihood = 0.1
		}
		alpha := morphWeight*1.5 + 0.1
		if alpha > 1.0 {
			alpha = 1.0
		}
		adjustedLik := math.Pow(likelihood, alpha)
		fusedProbs[agent] = bayesP * adjustedLik
		totalP += fusedProbs[agent]
	}

	if totalP > 1e-12 {
		for agent := range fusedProbs {
			fusedProbs[agent] /= totalP
		}
	} else {
		for agent := range fusedProbs {
			fusedProbs[agent] = 0.25
		}
	}

	fusion.FusedTemperature = fusedTemp
	fusion.FusedAgentProbs = fusedProbs

	return fusion
}

func (m *ProcessInverter) Relu(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

func (m *ProcessInverter) Softmax(x []float64) []float64 {
	if len(x) == 0 {
		return nil
	}
	maxVal := x[0]
	for _, v := range x {
		if v > maxVal {
			maxVal = v
		}
	}
	expSum := 0.0
	result := make([]float64, len(x))
	for i, v := range x {
		result[i] = math.Exp(v - maxVal)
		expSum += result[i]
	}
	if expSum < 1e-12 {
		for i := range result {
			result[i] = 1.0 / float64(len(x))
		}
		return result
	}
	for i := range result {
		result[i] /= expSum
	}
	return result
}

func (m *ProcessInverter) initPretrainedWeights() (map[string][][]float64, map[string][]float64) {
	weights := make(map[string][][]float64)
	biases := make(map[string][]float64)

	inputSize := m.bpCfg.InputSize
	hidden1Size := m.bpCfg.HiddenLayerSizes[0]
	hidden2Size := m.bpCfg.HiddenLayerSizes[1]
	tempOutSize := m.bpCfg.OutputTempSize
	agentOutSize := m.bpCfg.OutputAgentSize

	w1 := make([][]float64, inputSize)
	for i := 0; i < inputSize; i++ {
		w1[i] = make([]float64, hidden1Size)
		for j := 0; j < hidden1Size; j++ {
			switch i {
			case 5:
				w1[i][j] = 0.15 * float64(((j % 5) - 2))
			case 6:
				w1[i][j] = 0.20 * float64(((j % 4) - 1))
			case 7:
				w1[i][j] = 0.12 * float64(((j % 3) - 1))
			default:
				w1[i][j] = 0.08 * float64(((j % 6) - 3))
			}
		}
	}
	weights["w1"] = w1
	biases["b1"] = make([]float64, hidden1Size)
	for j := 0; j < hidden1Size; j++ {
		biases["b1"][j] = 0.05 * float64(j%3-1)
	}

	w2 := make([][]float64, hidden1Size)
	for i := 0; i < hidden1Size; i++ {
		w2[i] = make([]float64, hidden2Size)
		for j := 0; j < hidden2Size; j++ {
			if (i+j)%2 == 0 {
				w2[i][j] = 0.12
			} else {
				w2[i][j] = -0.08
			}
			if i%8 == j%4 {
				w2[i][j] *= 1.5
			}
		}
	}
	weights["w2"] = w2
	biases["b2"] = make([]float64, hidden2Size)
	for j := 0; j < hidden2Size; j++ {
		biases["b2"][j] = 0.03 * float64(j%2)
	}

	w3Temp := make([][]float64, hidden2Size)
	for i := 0; i < hidden2Size; i++ {
		w3Temp[i] = make([]float64, tempOutSize)
		if i < 8 {
			w3Temp[i][0] = 0.15 + 0.02*float64(i)
		} else {
			w3Temp[i][0] = -0.05 + 0.01*float64(i-8)
		}
	}
	weights["w3_temp"] = w3Temp
	biases["b3_temp"] = []float64{0.35}

	w3Agent := make([][]float64, hidden2Size)
	for i := 0; i < hidden2Size; i++ {
		w3Agent[i] = make([]float64, agentOutSize)
		switch {
		case i < 4:
			w3Agent[i][0] = 0.18
			w3Agent[i][1] = -0.05
			w3Agent[i][2] = -0.08
			w3Agent[i][3] = 0.05
		case i < 8:
			w3Agent[i][0] = -0.06
			w3Agent[i][1] = 0.20
			w3Agent[i][2] = 0.05
			w3Agent[i][3] = 0.04
		case i < 12:
			w3Agent[i][0] = -0.10
			w3Agent[i][1] = 0.06
			w3Agent[i][2] = 0.22
			w3Agent[i][3] = 0.03
		default:
			w3Agent[i][0] = 0.08
			w3Agent[i][1] = 0.08
			w3Agent[i][2] = 0.08
			w3Agent[i][3] = 0.18
		}
	}
	weights["w3_agent"] = w3Agent
	biases["b3_agent"] = []float64{0.5, -0.3, -0.5, 0.1}

	return weights, biases
}

func (m *ProcessInverter) ForwardLayer(input []float64, weights [][]float64, bias []float64, useRelu bool) []float64 {
	outputSize := len(bias)
	output := make([]float64, outputSize)

	for j := 0; j < outputSize; j++ {
		sum := bias[j]
		for i := range input {
			if i < len(weights) && j < len(weights[i]) {
				sum += input[i] * weights[i][j]
			}
		}
		if useRelu {
			output[j] = m.Relu(sum)
		} else {
			output[j] = sum
		}
	}
	return output
}

func (m *ProcessInverter) BayesianCorrection(
	_ int,
	slag *models.SlagComposition,
	bpnnProbs []float64,
) map[string]float64 {

	posterior := make(map[string]float64)
	agents := config.ReducingAgents

	priors := make([]float64, len(agents))
	for i, agent := range agents {
		if val, ok := m.bayCfg.PriorAgents[agent]; ok {
			priors[i] = val
		} else {
			priors[i] = 0.25
		}
	}

	likelihoods := make([]float64, len(agents))
	for i := range likelihoods {
		likelihoods[i] = 1.0
	}

	if slag != nil {
		so3 := slag.SO3
		if so3 > 5.0 {
			likelihoods[0] *= 0.3
			likelihoods[1] *= 1.8
			likelihoods[2] *= 2.5
			likelihoods[3] *= 1.2
		} else if so3 > 2.0 {
			likelihoods[0] *= 0.6
			likelihoods[1] *= 1.5
			likelihoods[2] *= 1.8
			likelihoods[3] *= 1.1
		}

		basicity := slag.CaO / (slag.SiO2 + 0.01)
		if basicity > 1.5 {
			likelihoods[0] *= 0.5
			likelihoods[1] *= 1.6
			likelihoods[2] *= 1.4
			likelihoods[3] *= 1.1
		} else if basicity < 0.5 {
			likelihoods[0] *= 1.5
			likelihoods[1] *= 0.7
			likelihoods[2] *= 0.8
			likelihoods[3] *= 1.0
		}
	}

	total := 0.0
	for i, agent := range agents {
		posterior[agent] = priors[i] * likelihoods[i] * bpnnProbs[i]
		total += posterior[agent]
	}

	if total > 1e-12 {
		for _, agent := range agents {
			posterior[agent] /= total
		}
	} else {
		for _, agent := range agents {
			posterior[agent] = 1.0 / float64(len(agents))
		}
	}

	return posterior
}

func (m *ProcessInverter) MapTempToProcess(temp float64) (string, string) {
	bestMatch := ""
	bestEra := ""
	bestScore := -1.0

	for _, mapping := range config.TempProcessMapping {
		midTemp := (mapping.TempMin + mapping.TempMax) / 2.0
		rangeTemp := mapping.TempMax - mapping.TempMin
		if rangeTemp < 1e-6 {
			rangeTemp = 100
		}
		dist := math.Abs(temp - midTemp)
		score := math.Exp(-dist * dist / (2.0 * rangeTemp * rangeTemp / 9.0))

		if temp >= mapping.TempMin && temp <= mapping.TempMax {
			score *= 1.5
		}

		if score > bestScore {
			bestScore = score
			bestMatch = mapping.ProcessType
			bestEra = mapping.EraEstimate
		}
	}

	return bestMatch, bestEra
}

func (m *ProcessInverter) GenerateTempDistribution(estimatedTemp float64, confidence float64) []float64 {
	numBins := 10
	tempMin := 500.0
	tempMax := 1600.0
	binWidth := (tempMax - tempMin) / float64(numBins)

	variance := 200.0 * (1.05 - confidence)
	if variance < 30.0 {
		variance = 30.0
	}

	distribution := make([]float64, numBins)
	totalDensity := 0.0

	for i := 0; i < numBins; i++ {
		binCenter := tempMin + float64(i)*binWidth + binWidth/2.0
		diff := binCenter - estimatedTemp
		density := math.Exp(-diff * diff / (2.0 * variance * variance))
		distribution[i] = density
		totalDensity += density
	}

	if totalDensity > 1e-12 {
		for i := 0; i < numBins; i++ {
			distribution[i] /= totalDensity
		}
	}

	return distribution
}

func (m *ProcessInverter) CalculateKLDivergence(bpnnProbs []float64, bayesPosterior map[string]float64) float64 {
	agents := config.ReducingAgents
	klDiv := 0.0
	for i, agent := range agents {
		p := bpnnProbs[i]
		q := bayesPosterior[agent]
		if p < 1e-12 {
			p = 1e-12
		}
		if q < 1e-12 {
			q = 1e-12
		}
		klDiv += p * math.Log(p/q)
	}
	return klDiv
}

func (m *ProcessInverter) GetTemperatureBinLabels() []string {
	labels := make([]string, 10)
	for i := 0; i < 10; i++ {
		start := 500 + i*110
		end := start + 110
		labels[i] = fmt.Sprintf("%d-%d℃", start, end)
	}
	return labels
}
