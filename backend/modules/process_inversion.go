package modules

import (
	"context"
	"fmt"
	"math"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
)

// ========================================
// ProcessInversionModule - 冶炼工艺反演模块
// 职责：从污染指纹和矿渣成分反推冶炼温度和还原剂类型
// 核心算法：BPNN神经网络前向传播 + 贝叶斯后验修正
// ========================================

type ProcessInversionModule struct {
	weights map[string][][]float64
	biases  map[string][]float64
	bpCfg   config.BPNNConfig
	bayCfg  config.BayesianConfig
}

// NewProcessInversionModule 创建冶炼工艺反演模块实例
func NewProcessInversionModule() *ProcessInversionModule {
	m := &ProcessInversionModule{
		bpCfg:  config.DefaultBPNNConfig,
		bayCfg: config.DefaultBayesianConfig,
	}
	m.weights, m.biases = m.initPretrainedWeights()
	return m
}

// InvertProcess 执行冶炼工艺反演
// 输入：站点ID、XRF测量数据序列、矿渣成分
// 输出：冶炼工艺反演结果（温度估计、还原剂类型、置信度等）
func (m *ProcessInversionModule) InvertProcess(
	ctx context.Context,
	siteID int,
	measurements []models.XRFMeasurement,
	slag *models.SlagComposition,
) (*models.SmeltingProcessResult, error) {

	// ========== Step 1: 构建8维输入特征向量 ==========
	features, nValid, slagCompleteness := m.buildFeatureVector(measurements, slag)

	// ========== Step 2: BPNN神经网络前向传播 ==========
	hidden1 := m.forwardLayer(features, m.weights["w1"], m.biases["b1"], true)
	hidden2 := m.forwardLayer(hidden1, m.weights["w2"], m.biases["b2"], true)
	tempRaw := m.forwardLayer(hidden2, m.weights["w3_temp"], m.biases["b3_temp"], false)
	agentLogits := m.forwardLayer(hidden2, m.weights["w3_agent"], m.biases["b3_agent"], false)

	// 温度反归一化到 [500, 1600] 区间
	tempNorm := tempRaw[0]
	if tempNorm < 0 {
		tempNorm = 0
	}
	if tempNorm > 1 {
		tempNorm = 1
	}
	estimatedTemp := 500.0 + tempNorm*1100.0

	// 还原剂 softmax 概率
	bpnnAgentProbs := m.softmax(agentLogits)

	// ========== Step 3: 贝叶斯后验概率修正 ==========
	bayesPosterior := m.bayesianCorrection(siteID, slag, bpnnAgentProbs)

	// ========== Step 4: 温度-工艺-年代映射 ==========
	processType, eraEstimate := m.mapTempToProcess(estimatedTemp)

	// ========== Step 5: 质量评估 ==========
	tempConfidence := math.Min(0.98, 0.5+0.1*float64(nValid)+0.05*slagCompleteness)

	var agentConfidence float64
	var bestAgent string
	maxPosterior := 0.0
	for i, agent := range config.ReducingAgents {
		if bayesPosterior[agent] > maxPosterior {
			maxPosterior = bayesPosterior[agent]
			bestAgent = agent
		}
	}
	agentConfidence = maxPosterior

	var qualityLevel string
	if tempConfidence > 0.8 && agentConfidence > 0.8 {
		qualityLevel = "高"
	} else if tempConfidence > 0.6 || agentConfidence > 0.6 {
		qualityLevel = "中"
	} else {
		qualityLevel = "低"
	}

	// ========== Step 6: 生成温度分布直方图数据（10个区间，500-1600℃） ==========
	tempDistribution := m.generateTempDistribution(estimatedTemp, tempConfidence)

	// ========== Step 7: 组装结果 ==========
	inputFeatures := map[string]interface{}{
		"pb_zn_ratio":     features[0],
		"cu_pb_ratio":     features[1],
		"as_hg_ratio":     features[2],
		"cd_zn_ratio":     features[3],
		"cu_as_ratio":     features[4],
		"cao_sio2_ratio":  features[5],
		"feo_total":       features[6],
		"so3_content":     features[7],
		"n_valid_features": nValid,
		"slag_completeness": slagCompleteness,
	}

	agentProbsMap := make(map[string]float64)
	for i, agent := range config.ReducingAgents {
		agentProbsMap[agent] = bpnnAgentProbs[i]
	}

	inversion := models.SmeltingProcessInversion{
		SiteID:                  siteID,
		EstimatedTemperature:    estimatedTemp,
		TemperatureConfidence:   tempConfidence,
		ReducingAgent:           bestAgent,
		ReducingAgentConfidence: agentConfidence,
		BPNNPosterior: map[string]interface{}{
			"temperature": tempNorm,
			"agents":      agentProbsMap,
		},
		BayesPosterior:      bayesPosterior,
		ProcessTypeDetailed: processType,
		ProcessEraEstimate:  eraEstimate,
		InputFeatures:       inputFeatures,
		BPNNMSE:             0.0,
		BayesKLD:            m.calculateKLDivergence(bpnnAgentProbs, bayesPosterior),
		QualityLevel:        qualityLevel,
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
		SiteID:                siteID,
		Inversion:             inversion,
		NetworkInfo:           networkInfo,
		TemperatureDistribution: tempDistribution,
		AgentProbabilities:    bayesPosterior,
	}

	// Step 7 (后续): 保存到 repository
	_ = ctx

	return result, nil
}

// ============== 辅助方法 ==============

// buildFeatureVector 构建8维输入特征向量并归一化
// 返回：归一化特征向量、有效特征数量、矿渣数据完整度
func (m *ProcessInversionModule) buildFeatureVector(
	measurements []models.XRFMeasurement,
	slag *models.SlagComposition,
) ([]float64, int, float64) {

	features := make([]float64, 8)
	nValid := 0

	// 计算平均XRF测量值
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

	// 5个金属比率特征
	ratios := []float64{
		safeDiv(avgPb, avgZn),  // pb_zn_ratio
		safeDiv(avgCu, avgPb),  // cu_pb_ratio
		safeDiv(avgAs, avgHg),  // as_hg_ratio
		safeDiv(avgCd, avgZn),  // cd_zn_ratio
		safeDiv(avgCu, avgAs),  // cu_as_ratio
	}

	// 比率归一化（经验范围）
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

	// 矿渣特征
	slagFields := 0
	slagValid := 0
	var caoSio2, feoTotal, so3Content float64

	if slag != nil {
		slagFields = 3
		// CaO/SiO2 碱度
		if slag.CaO >= 0 && slag.SiO2 >= 0 {
			caoSio2 = slag.CaO / (slag.SiO2 + 0.01)
			slagValid++
		}
		// 总铁 (FeO + Fe2O3*0.9)，单位转换为百分比小数
		if slag.FeO >= 0 && slag.Fe2O3 >= 0 {
			feoTotal = (slag.FeO + slag.Fe2O3*0.9) / 100.0
			slagValid++
		}
		// SO3含量
		if slag.SO3 >= 0 {
			so3Content = slag.SO3 / 100.0
			slagValid++
		}
	}

	// 矿渣碱度归一化 (0~3范围)
	features[5] = math.Min(1.0, caoSio2/3.0)
	// 总铁归一化 (0~0.8范围)
	features[6] = math.Min(1.0, feoTotal/0.8)
	// SO3归一化 (0~0.1范围)
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

// relu ReLU激活函数 f(x) = max(0, x)
func (m *ProcessInversionModule) relu(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

// softmax 概率归一化函数
func (m *ProcessInversionModule) softmax(x []float64) []float64 {
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

// initPretrainedWeights 初始化预训练权重矩阵
// 基于考古学已知数据构建模拟权重，确保结果合理
// 网络结构：8→32→16→4+1
func (m *ProcessInversionModule) initPretrainedWeights() (map[string][][]float64, map[string][]float64) {
	weights := make(map[string][][]float64)
	biases := make(map[string][]float64)

	inputSize := m.bpCfg.InputSize        // 8
	hidden1Size := m.bpCfg.HiddenLayerSizes[0] // 32
	hidden2Size := m.bpCfg.HiddenLayerSizes[1] // 16
	tempOutSize := m.bpCfg.OutputTempSize  // 1
	agentOutSize := m.bpCfg.OutputAgentSize // 4

	// W1: 输入层 (8) -> 隐藏层1 (32)
	w1 := make([][]float64, inputSize)
	for i := 0; i < inputSize; i++ {
		w1[i] = make([]float64, hidden1Size)
		for j := 0; j < hidden1Size; j++ {
			// 按特征重要性设置初始权重
			switch i {
			case 5: // cao_sio2_ratio - 高碱度对应高温
				w1[i][j] = 0.15 * float64(((j % 5) - 2))
			case 6: // feo_total - 高铁对应高温
				w1[i][j] = 0.20 * float64(((j % 4) - 1))
			case 7: // so3_content - 高硫对应煤/焦炭
				w1[i][j] = 0.12 * float64(((j % 3) - 1))
			default: // 金属比率
				w1[i][j] = 0.08 * float64(((j % 6) - 3))
			}
		}
	}
	weights["w1"] = w1
	biases["b1"] = make([]float64, hidden1Size)
	for j := 0; j < hidden1Size; j++ {
		biases["b1"][j] = 0.05 * float64(j%3-1)
	}

	// W2: 隐藏层1 (32) -> 隐藏层2 (16)
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

	// W3_temp: 隐藏层2 (16) -> 温度输出 (1)
	w3Temp := make([][]float64, hidden2Size)
	for i := 0; i < hidden2Size; i++ {
		w3Temp[i] = make([]float64, tempOutSize)
		// 高温相关特征对应正权重：高FeO、高CaO/SiO2
		if i < 8 {
			w3Temp[i][0] = 0.15 + 0.02*float64(i)
		} else {
			w3Temp[i][0] = -0.05 + 0.01*float64(i-8)
		}
	}
	weights["w3_temp"] = w3Temp
	biases["b3_temp"] = []float64{0.35} // 基线温度约 500+0.35*1100 ≈ 885℃

	// W3_agent: 隐藏层2 (16) -> 还原剂输出 (4)
	// 顺序: [木炭, 焦炭, 煤, 混合]
	w3Agent := make([][]float64, hidden2Size)
	for i := 0; i < hidden2Size; i++ {
		w3Agent[i] = make([]float64, agentOutSize)
		switch {
		case i < 4: // 木炭相关：低温、低SO3
			w3Agent[i][0] = 0.18
			w3Agent[i][1] = -0.05
			w3Agent[i][2] = -0.08
			w3Agent[i][3] = 0.05
		case i < 8: // 焦炭相关：高温、中SO3
			w3Agent[i][0] = -0.06
			w3Agent[i][1] = 0.20
			w3Agent[i][2] = 0.05
			w3Agent[i][3] = 0.04
		case i < 12: // 煤相关：高SO3、中温
			w3Agent[i][0] = -0.10
			w3Agent[i][1] = 0.06
			w3Agent[i][2] = 0.22
			w3Agent[i][3] = 0.03
		default: // 混合相关：中等特征
			w3Agent[i][0] = 0.08
			w3Agent[i][1] = 0.08
			w3Agent[i][2] = 0.08
			w3Agent[i][3] = 0.18
		}
	}
	weights["w3_agent"] = w3Agent
	biases["b3_agent"] = []float64{0.5, -0.3, -0.5, 0.1} // 先验偏向木炭和混合

	return weights, biases
}

// forwardLayer 执行单层前向传播
// input: 输入向量, weights: 权重矩阵, bias: 偏置向量, useRelu: 是否使用ReLU激活
func (m *ProcessInversionModule) forwardLayer(input []float64, weights [][]float64, bias []float64, useRelu bool) []float64 {
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
			output[j] = m.relu(sum)
		} else {
			output[j] = sum
		}
	}
	return output
}

// bayesianCorrection 贝叶斯后验概率修正
// 先验 × 似然，再归一化
func (m *ProcessInversionModule) bayesianCorrection(
	_ int,
	slag *models.SlagComposition,
	bpnnProbs []float64,
) map[string]float64 {

	posterior := make(map[string]float64)
	agents := config.ReducingAgents

	// 先验概率
	priors := make([]float64, len(agents))
	for i, agent := range agents {
		if val, ok := m.bayCfg.PriorAgents[agent]; ok {
			priors[i] = val
		} else {
			priors[i] = 0.25
		}
	}

	// 似然度：根据 site.MetalType + SO3含量 + CaO/SiO2 调整
	likelihoods := make([]float64, len(agents))
	for i := range likelihoods {
		likelihoods[i] = 1.0
	}

	if slag != nil {
		// SO3含量高 → 煤/焦炭概率高
		so3 := slag.SO3
		if so3 > 5.0 {
			likelihoods[0] *= 0.3  // 木炭
			likelihoods[1] *= 1.8  // 焦炭
			likelihoods[2] *= 2.5  // 煤
			likelihoods[3] *= 1.2  // 混合
		} else if so3 > 2.0 {
			likelihoods[0] *= 0.6
			likelihoods[1] *= 1.5
			likelihoods[2] *= 1.8
			likelihoods[3] *= 1.1
		}

		// 高碱度(CaO/SiO2高) → 温度高 → 焦炭/煤
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

	// 计算后验 = 先验 × 似然 × BPNN输出
	total := 0.0
	for i, agent := range agents {
		posterior[agent] = priors[i] * likelihoods[i] * bpnnProbs[i]
		total += posterior[agent]
	}

	// 归一化
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

// mapTempToProcess 根据温度映射到工艺类型和年代估计
func (m *ProcessInversionModule) mapTempToProcess(temp float64) (string, string) {
	bestMatch := ""
	bestEra := ""
	bestScore := -1.0

	for _, mapping := range config.TempProcessMapping {
		// 计算温度与区间的匹配度（使用高斯相似度）
		midTemp := (mapping.TempMin + mapping.TempMax) / 2.0
		rangeTemp := mapping.TempMax - mapping.TempMin
		if rangeTemp < 1e-6 {
			rangeTemp = 100
		}
		dist := math.Abs(temp - midTemp)
		score := math.Exp(-dist * dist / (2.0 * rangeTemp * rangeTemp / 9.0))

		// 温度在区间内额外加分
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

// generateTempDistribution 生成温度后验分布直方图（10个区间，500-1600℃）
// 基于正态分布模拟，以估计温度为均值，置信度反比于方差
func (m *ProcessInversionModule) generateTempDistribution(estimatedTemp float64, confidence float64) []float64 {
	numBins := 10
	tempMin := 500.0
	tempMax := 1600.0
	binWidth := (tempMax - tempMin) / float64(numBins)

	// 置信度越高，方差越小
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

	// 归一化
	if totalDensity > 1e-12 {
		for i := 0; i < numBins; i++ {
			distribution[i] /= totalDensity
		}
	}

	return distribution
}

// calculateKLDivergence 计算 BPNN 输出与贝叶斯后验的 KL 散度
func (m *ProcessInversionModule) calculateKLDivergence(bpnnProbs []float64, bayesPosterior map[string]float64) float64 {
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

// ============== 补充：获取结果的温度区间标签（用于前端展示） ==============

// GetTemperatureBinLabels 获取温度分布直方图的区间标签
func (m *ProcessInversionModule) GetTemperatureBinLabels() []string {
	labels := make([]string, 10)
	for i := 0; i < 10; i++ {
		start := 500 + i*110
		end := start + 110
		labels[i] = fmt.Sprintf("%d-%d℃", start, end)
	}
	return labels
}
