package config

// ========================================
// 算法参数配置 — 所有硬编码参数集中管理
// 修改配置无需改动业务代码
// ========================================

// ====== 污染指数计算 ======
var PollutionStandards = map[string]float64{
	"Pb": 800.0,
	"Zn": 5000.0,
	"Cu": 18000.0,
	"As": 250.0,
	"Hg": 38.0,
	"Cd": 47.0,
}

// 重金属检测阈值（占标准的比例，超过视为检测到）
var MetalDetectionThresholdRatio = 0.1

// ====== PCA + 聚类 ======
type PCAConfig struct {
	NumComponents      int     // 主成分数量
	KMin               int     // 最小聚类数
	KMax               int     // 最大聚类数
	MaxIterations      int     // K-Means最大迭代
	ConvergenceEps     float64 // 收敛阈值
	NumBootstraps      int     // Bootstrap抽样次数
	NumGapReferences   int     // Gap Statistic参考分布次数
	ElbowWeight        float64 // 手肘法权重
	SilhouetteWeight   float64 // 轮廓系数权重
	GapStatisticWeight float64 // Gap统计量权重
	JaccardThreshold   float64 // 聚类稳定性阈值
}

var DefaultPCAConfig = PCAConfig{
	NumComponents:      3,
	KMin:               2,
	KMax:               8,
	MaxIterations:      200,
	ConvergenceEps:     1e-6,
	NumBootstraps:      100,
	NumGapReferences:   50,
	ElbowWeight:        0.3,
	SilhouetteWeight:   0.4,
	GapStatisticWeight: 0.3,
	JaccardThreshold:   0.6,
}

// ====== 指纹匹配 ======
type FingerprintConfig struct {
	RatioWeights    map[string]float64 // 重金属比率权重
	IsotopeWeights  map[string]float64 // 同位素权重
	SimilarityBias  float64            // 相似度分母偏移
	LogTransform    bool               // 是否对数变换
}

var DefaultFingerprintConfig = FingerprintConfig{
	RatioWeights: map[string]float64{
		"pb_zn_ratio":  2.0,
		"cu_pb_ratio":  2.0,
		"as_hg_ratio":  1.5,
		"cd_zn_ratio":  1.0,
		"cu_as_ratio":  1.5,
	},
	IsotopeWeights: map[string]float64{
		"pb206_pb207": 2.5,
		"pb208_pb207": 2.5,
	},
	SimilarityBias: 1.0,
	LogTransform:   true,
}

// ====== AHP 多属性决策 ======
type AHPConfig struct {
	JudgmentMatrix [][]float64 // 判断矩阵（Saaty 1-9标度）
	Criteria       []string    // 属性名
	RITable        []float64   // RI随机一致性指标表
	CRThreshold    float64     // 一致性比率阈值
}

var DefaultAHPConfig = AHPConfig{
	JudgmentMatrix: [][]float64{
		//     金属覆盖  效率    土壤   成本    周期   环境   可持续
		{1.0, 2.0, 3.0, 3.0, 4.0, 5.0, 5.0},
		{0.5, 1.0, 2.0, 2.0, 3.0, 4.0, 4.0},
		{1.0 / 3, 0.5, 1.0, 1.0, 2.0, 3.0, 3.0},
		{1.0 / 3, 0.5, 1.0, 1.0, 2.0, 2.0, 3.0},
		{0.25, 1.0 / 3, 0.5, 0.5, 1.0, 2.0, 2.0},
		{0.2, 0.25, 1.0 / 3, 1.0 / 3, 0.5, 1.0, 2.0},
		{0.2, 0.25, 1.0 / 3, 1.0 / 3, 0.5, 0.5, 1.0},
	},
	Criteria: []string{
		"metal_coverage", "efficiency", "soil_applicability",
		"cost", "duration", "environmental", "sustainability",
	},
	RITable:     []float64{0, 0, 0.58, 0.90, 1.12, 1.24, 1.32, 1.41, 1.45, 1.49},
	CRThreshold: 0.1,
}

// ====== 组合权重动态调整 ======
type AlphaConfig struct {
	MinAlpha       float64 // 最小主观权重占比
	MaxAlpha       float64 // 最大主观权重占比
	MaxPIForAlpha  float64 // 达到最大alpha时的PI值
}

var DefaultAlphaConfig = AlphaConfig{
	MinAlpha:      0.3,
	MaxAlpha:      0.7,
	MaxPIForAlpha: 5.0,
}

// ====== MCDM 打分基准 ======
type ScoreBenchmarkConfig struct {
	CostBenchmarkYuanPerM3     float64 // 成本基准（元/m³）
	DurationBenchmarkMonths    float64 // 周期基准（月）
	UnmatchedSoilPenaltyScore  float64 // 土壤不匹配惩罚分
	MobilityHighBonusTechs     []string // 迁移性高时加分技术
	MobilityBonusPoints        float64  // 迁移性加分值
	HgHighTriggerMgPerKg       float64  // Hg严重超标阈值
	HgHighBonusTech            string   // Hg超标加分技术
	HgHighBonusPoints          float64  // Hg加分值
}

var DefaultScoreBenchmark = ScoreBenchmarkConfig{
	CostBenchmarkYuanPerM3:    15000.0,
	DurationBenchmarkMonths:   60.0,
	UnmatchedSoilPenaltyScore: 40.0,
	MobilityHighBonusTechs:    []string{"固化稳定化", "植物稳定修复"},
	MobilityBonusPoints:       3.0,
	HgHighTriggerMgPerKg:      38.0,
	HgHighBonusTech:           "热脱附",
	HgHighBonusPoints:         5.0,
}

// ====== 生态风险指数 ======
type EcoRiskConfig struct {
	ToxicFactors map[string]float64 // 毒性系数
	RefValues    map[string]float64 // 参考背景值
}

var DefaultEcoRiskConfig = EcoRiskConfig{
	ToxicFactors: map[string]float64{
		"Pb": 5, "Zn": 1, "Cu": 5, "As": 10, "Hg": 40, "Cd": 30,
	},
	RefValues: map[string]float64{
		"Pb": 35, "Zn": 80, "Cu": 35, "As": 15, "Hg": 0.25, "Cd": 0.5,
	},
}

// ====== 告警系统 ======
type AlertConfig struct {
	AggregateFlushPeriodMinutes int // 聚合发送周期（分钟）
	ExceedRatioForHighLevel     float64 // 升级到高级别的超标倍数
	HighEcoRiskThreshold        float64 // 生态风险告警阈值
	MediumPollutionThreshold    float64 // 生态风险PI阈值
}

var DefaultAlertConfig = AlertConfig{
	AggregateFlushPeriodMinutes: 30,
	ExceedRatioForHighLevel:     1.5,
	HighEcoRiskThreshold:        150.0,
	MediumPollutionThreshold:    2.0,
}

// ====== 事件总线 Channel 缓冲 ======
var EventBusChannelBufferSize = 100
