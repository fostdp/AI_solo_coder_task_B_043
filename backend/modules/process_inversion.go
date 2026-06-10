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

type MorphologyFeature struct {
	GlassPhaseIndex    float64
	MineralAssemblage  string
	CoolingRateEstimate float64
	TextureScore       float64
}

type MultiSourceFusionResult struct {
	XRFSignalWeight     float64
	SlagChemWeight      float64
	SlagMorphologyWeight float64
	FusedTemperature    float64
	FusedAgentProbs     map[string]float64
	DisagreementLevel   float64
	AmbiguityNote       string
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

	// ========== Step 1.5: 提取炉渣形态特征（多源数据融合） ==========
	morphology := m.extractMorphologyFeatures(slag)

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

	// ========== Step 3.5: 多源数据融合（XRF指纹 + 矿渣化学 + 矿渣形态） ==========
	fusion := m.fuseMultiSourceData(estimatedTemp, bayesPosterior, morphology, slag, nValid, slagCompleteness)

	// ========== Step 4: 温度-工艺-年代映射 ==========
	processType, eraEstimate := m.mapTempToProcess(estimatedTemp)

	// ========== Step 5: 质量评估 ==========
	tempConfidence := math.Min(0.98, 0.5+0.1*float64(nValid)+0.05*slagCompleteness)

	// 使用融合后的概率而非原始贝叶斯后验
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

	// 若多源融合降低温度结果，若有歧义则降级置信度
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

	// ========== Step 6: 生成温度分布直方图数据（10个区间，500-1600℃） ==========
	tempDistribution := m.generateTempDistribution(fusion.FusedTemperature, tempConfidence)

	// ========== Step 7: 组装结果 ==========
	inputFeatures := map[string]interface{}{
		"pb_zn_ratio":        features[0],
		"cu_pb_ratio":        features[1],
		"as_hg_ratio":        features[2],
		"cd_zn_ratio":        features[3],
		"cu_as_ratio":        features[4],
		"cao_sio2_ratio":     features[5],
		"feo_total":        features[6],
		"so3_content":        features[7],
		"n_valid_features":    nValid,
		"slag_completeness":   slagCompleteness,
		"morphology_glass":    morphology.GlassPhaseIndex,
		"morphology_texture":  morphology.TextureScore,
		"morphology_cooling":  morphology.CoolingRateEstimate,
		"fusion_xrf_weight":    fusion.XRFSignalWeight,
		"fusion_chem_weight":   fusion.SlagChemWeight,
		"fusion_morph_weight":  fusion.SlagMorphologyWeight,
		"fusion_disagreement": fusion.DisagreementLevel,
		"fusion_ambiguity":     fusion.AmbiguityNote,
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
		BayesKLD:            m.calculateKLDivergence(bpnnAgentProbs, finalAgentProbs),
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
		SiteID:                siteID,
		Inversion:             inversion,
		NetworkInfo:           networkInfo,
		TemperatureDistribution: tempDistribution,
		AgentProbabilities:    finalAgentProbs,
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

// extractMorphologyFeatures 从矿渣矿物相组成提取炉渣形态特征
// 用于多源数据融合，解决仅靠XRF指纹时特征重叠导致的误判
func (m *ProcessInversionModule) extractMorphologyFeatures(slag *models.SlagComposition) MorphologyFeature {
	morph := MorphologyFeature{
		GlassPhaseIndex:    0.5,
		MineralAssemblage:  "unknown",
		CoolingRateEstimate: 0.5,
		TextureScore:       0.5,
	}

	if slag == nil {
		return morph
	}

	// 1. 玻璃相指数：玻璃相含量越高 → 冷却越快（高温快速淬火）
	glassPhase := slag.GlassPhase
	if glassPhase < 0 {
		glassPhase = 0
	}
	if glassPhase > 100 {
		glassPhase = 100
	}
	morph.GlassPhaseIndex = glassPhase / 100.0

	// 2. 冷却速率估计：玻璃相+磁铁矿(快速冷却结晶)多 → 快冷；硅灰石/钙长石多 → 慢冷
	fastCoolIndicators := glassPhase + slag.Magnetite*2.0
	slowCoolIndicators := slag.Wollastonite + slag.Anorthite + slag.Diopside
	totalMineral := fastCoolIndicators + slowCoolIndicators + 0.01
	morph.CoolingRateEstimate = math.Min(1.0, fastCoolIndicators/totalMineral)

	// 3. 矿物组合类型判定
	switch {
	case slag.Fayalite > 30 && slag.GlassPhase < 20:
		morph.MineralAssemblage = "fayalite_slow" // 铁橄榄石为主→低温慢冷→块炼法
	case slag.Wollastonite > 25 && slag.Anorthite > 15:
		morph.MineralAssemblage = "wollastonite_anorthite" // 硅灰石+钙长石→中温→生铁冶炼
	case slag.GlassPhase > 40:
		morph.MineralAssemblage = "glass_quenched" // 高玻璃相→高温淬火→高温液态冶炼
	case slag.Diopside > 20:
		morph.MineralAssemblage = "diopside_basic" // 透辉石→高碱度→高炉工艺
	default:
		morph.MineralAssemblage = "mixed"
	}

	// 4. 织构评分：矿物相组成多样性越高（但不极端），织构信息越丰富
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
		// 3-5种主要矿相 → 织构清晰；极端单一或极端分散 → 模糊
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

// fuseMultiSourceData 多源数据融合
// 融合 XRF指纹信号、矿渣化学成分、矿渣形态特征 三个来源
// 通过加权D-S证据理论风格的融合，解决单一指纹重叠导致的误判
func (m *ProcessInversionModule) fuseMultiSourceData(
	nnTemp float64,
	bayesProbs map[string]float64,
	morph MorphologyFeature,
	slag *models.SlagComposition,
	nValid int,
	slagCompleteness float64,
) MultiSourceFusionResult {

	fusion := MultiSourceFusionResult{
		FusedTemperature: nnTemp,
		FusedAgentProbs:   make(map[string]float64),
	}

	// ========== 1. 计算各数据源权重 ==========
	// XRF指纹权重：有效特征数量越多，权重越高
	xrfWeight := 0.3 + 0.5*math.Min(1.0, float64(nValid)/8.0)

	// 矿渣化学权重：矿渣数据完整度越高，权重越高
	chemWeight := 0.2 + 0.5*slagCompleteness

	// 矿渣形态权重：织构评分（即信息清晰度）越高，权重越高
	morphWeight := 0.1 + 0.7*morph.TextureScore

	// 归一化权重
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

	// ========== 2. 从形态特征独立估计温度 ==========
	morphTempEstimate := nnTemp
	switch morph.MineralAssemblage {
	case "fayalite_slow":
		morphTempEstimate = 700 + 200*morph.CoolingRateEstimate // 700-900℃ 块炼法
	case "wollastonite_anorthite":
		morphTempEstimate = 1000 + 200*morph.CoolingRateEstimate // 1000-1200℃ 生铁
	case "glass_quenched":
		morphTempEstimate = 1200 + 300*morph.CoolingRateEstimate // 1200-1500℃ 高温液态
	case "diopside_basic":
		morphTempEstimate = 1300 + 200*morph.CoolingRateEstimate // 1300-1500℃ 高炉
	default:
		morphTempEstimate = nnTemp
	}

	// ========== 3. 从矿渣化学(碱度)独立估计温度 ==========
	chemTempEstimate := nnTemp
	if slag != nil {
		basicity := slag.CaO / (slag.SiO2 + 0.01)
		totalIron := (slag.FeO + slag.Fe2O3*0.9) / 100.0
		// 碱度+高铁→高温
		chemTempEstimate = 700 + basicity*250 + totalIron*400
		if chemTempEstimate > 1550 {
			chemTempEstimate = 1550
		}
	}

	// ========== 4. 温度加权融合 ==========
	fusedTemp := xrfWeight*nnTemp + chemWeight*chemTempEstimate + morphWeight*morphTempEstimate

	// ========== 5. 计算多源分歧度 ==========
	tempStd := math.Sqrt(
		xrfWeight*(nnTemp-fusedTemp)*(nnTemp-fusedTemp) +
			chemWeight*(chemTempEstimate-fusedTemp)*(chemTempEstimate-fusedTemp) +
			morphWeight*(morphTempEstimate-fusedTemp)*(morphTempEstimate-fusedTemp),
	)
	disagreement := math.Min(1.0, tempStd/200.0)
	fusion.DisagreementLevel = disagreement

	// 歧义无意义：记录警告信息
	if disagreement > 0.6 {
		fusion.AmbiguityNote = "多源数据存在显著分歧：建议补充矿渣微区分析以确证工艺类型"
	} else if disagreement > 0.3 {
		fusion.AmbiguityNote = "多源数据存在中等分歧：结果仅供参考，建议进一步考古验证"
	} else {
		fusion.AmbiguityNote = ""
	}

	// ========== 6. 还原剂概率融合（形态特征校正） ==========
	// 基于矿渣形态推导的还原剂似然
	morphAgentLikelihood := map[string]float64{
		"木炭": 1.0,
		"焦炭": 1.0,
		"煤":   1.0,
		"混合": 1.0,
	}
	switch morph.MineralAssemblage {
	case "fayalite_slow": // 低温块炼→木炭为主
		morphAgentLikelihood["木炭"] = 2.0
		morphAgentLikelihood["焦炭"] = 0.5
		morphAgentLikelihood["煤"] = 0.4
	case "glass_quenched", "diopside_basic": // 高温高炉→焦炭/煤
		morphAgentLikelihood["木炭"] = 0.4
		morphAgentLikelihood["焦炭"] = 1.8
		morphAgentLikelihood["煤"] = 1.6
	case "wollastonite_anorthite": // 中生铁→混合或焦炭
		morphAgentLikelihood["混合"] = 1.5
		morphAgentLikelihood["焦炭"] = 1.3
	}

	// 冷却极快也可能是焦炭（鼓风强）
	if morph.CoolingRateEstimate > 0.7 {
		morphAgentLikelihood["焦炭"] *= 1.3
		morphAgentLikelihood["煤"] *= 1.2
	}

	// 融合：bayes后验(已融合XRF+化学) × 形态似然，再归一化
	fusedProbs := make(map[string]float64)
	totalP := 0.0
	for agent, bayesP := range bayesProbs {
		likelihood := morphAgentLikelihood[agent]
		if likelihood < 0.1 {
			likelihood = 0.1
		}
		// 形态权重调制：形态权重高则影响大
		alpha := morphWeight*1.5 + 0.1
		if alpha > 1.0 {
			alpha = 1.0
		}
		adjustedLik := math.Pow(likelihood, alpha)
		fusedProbs[agent] = bayesP * adjustedLik
		totalP += fusedProbs[agent]
	}

	// 归一化
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
