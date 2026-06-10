package config

import "math"

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

// ====== 潜在生态风险指数 (Hakanson RI) ======
type EcoRiskRIConfig struct {
	ToxicFactors map[string]float64
	RefValues    map[string]float64
}

var DefaultEcoRiskRIConfig = EcoRiskRIConfig{
	ToxicFactors: map[string]float64{
		"Hg": 40.0,
		"Cd": 30.0,
		"As": 10.0,
		"Pb": 5.0,
		"Cu": 5.0,
		"Zn": 1.0,
		"Cr": 2.0,
		"Ni": 5.0,
	},
	RefValues: map[string]float64{
		"Pb": 20.0,
		"Zn": 67.0,
		"Cu": 20.0,
		"As": 11.2,
		"Hg": 0.065,
		"Cd": 0.097,
		"Cr": 54.0,
		"Ni": 24.0,
	},
}

// ====== 地积累指数 Muller 分级 ======
type GeoAccumulationConfig struct {
	BackgroundValues map[string]float64
	CorrectionFactor float64
}

var DefaultGeoAccumulationConfig = GeoAccumulationConfig{
	BackgroundValues: map[string]float64{
		"Pb": 20.0,
		"Zn": 70.0,
		"Cu": 25.0,
		"As": 5.0,
		"Hg": 0.08,
		"Cd": 0.2,
		"Cr": 70.0,
		"Ni": 50.0,
	},
	CorrectionFactor: 1.5,
}

var GeoAccumulationLevels = []struct {
	Min, Max    float64
	Level       string
	Description string
}{
	{-10, 0, "0级", "清洁（无污染）"},
	{0, 1, "1级", "轻度污染"},
	{1, 2, "2级", "偏中度污染"},
	{2, 3, "3级", "中度污染"},
	{3, 4, "4级", "偏重污染"},
	{4, 5, "5级", "重度污染"},
	{5, 100, "6级", "极重度污染"},
}

// ====== BPNN 冶炼工艺反演神经网络 ======
type BPNNConfig struct {
	InputSize        int
	HiddenLayerSizes []int
	OutputTempSize   int
	OutputAgentSize  int
	LearningRate     float64
	MaxEpochs        int
	ConvergenceEps   float64
	Activation       string
	DropoutRate      float64
}

var DefaultBPNNConfig = BPNNConfig{
	InputSize:        8,
	HiddenLayerSizes: []int{32, 16},
	OutputTempSize:   1,
	OutputAgentSize:  4,
	LearningRate:     0.001,
	MaxEpochs:        2000,
	ConvergenceEps:   1e-5,
	Activation:       "relu",
	DropoutRate:      0.1,
}

var ReducingAgents = []string{"木炭", "焦炭", "煤", "混合"}

var TempProcessMapping = []struct {
	TempMin, TempMax float64
	ProcessType      string
	EraEstimate      string
}{
	{400, 700, "低温焙烧法", "公元前5000-2000年 新石器时代晚期/铜石并用时代"},
	{600, 900, "混汞法/辰砂焙烧", "公元前3000-公元1800年 早期文明-殖民时期"},
	{800, 1000, "灰吹法-贵金属精炼", "公元前2500-公元1800年 青铜时代-工业革命前"},
	{950, 1200, "坩埚还原熔炼法", "公元前3500-公元前1000年 青铜时代早期中期"},
	{1100, 1250, "竖炉熔炼法", "公元前2000-公元前500年 青铜时代晚期"},
	{1000, 1300, "块炼法-固态还原", "公元前1500-公元1000年 铁器时代早期中期"},
	{1300, 1600, "近代高炉法", "公元1700年至今 工业革命-现代"},
}

// ====== 贝叶斯推断配置 ======
type BayesianConfig struct {
	PriorTemperatures map[string]float64
	PriorAgents       map[string]float64
	NumSamples        int
	BurnIn            int
}

var DefaultBayesianConfig = BayesianConfig{
	PriorAgents: map[string]float64{
		"木炭": 0.60,
		"焦炭": 0.10,
		"煤":   0.05,
		"混合": 0.25,
	},
	NumSamples: 5000,
	BurnIn:     1000,
}

// ====== 矿渣建材标准 ======
type BuildingMaterialStandard struct {
	CementS95Activity7dMin  float64
	CementS95Activity28dMin float64
	CementS75Activity7dMin  float64
	CementS75Activity28dMin float64
	CementFlowRatioMin      float64
	CementWaterContentMax   float64
	CementLossOnIgnitionMax float64
	CementFinenessMin       float64

	RoadCBRGrade1Min       float64
	RoadCBRGrade2Min       float64
	RoadCBRGrade3Min       float64
	RoadCrushValueMax      float64
	RoadPlasticityIdxMax   float64
	RoadFreezeThawLossMax  float64
	RoadAbrasionMax        float64

	LeachingPbMax float64
	LeachingCdMax float64
	LeachingAsMax float64
	LeachingHgMax float64
	LeachingCrMax float64
	LeachingNiMax float64
}

var DefaultBuildingStandard = BuildingMaterialStandard{
	CementS95Activity7dMin:  75,
	CementS95Activity28dMin: 95,
	CementS75Activity7dMin:  55,
	CementS75Activity28dMin: 75,
	CementFlowRatioMin:      95,
	CementWaterContentMax:   1.0,
	CementLossOnIgnitionMax: 3.0,
	CementFinenessMin:       350,

	RoadCBRGrade1Min:       150,
	RoadCBRGrade2Min:       120,
	RoadCBRGrade3Min:       80,
	RoadCrushValueMax:      26,
	RoadPlasticityIdxMax:   9,
	RoadFreezeThawLossMax:  5,
	RoadAbrasionMax:        15,

	LeachingPbMax: 3.0,
	LeachingCdMax: 0.3,
	LeachingAsMax: 1.5,
	LeachingHgMax: 0.05,
	LeachingCrMax: 4.5,
	LeachingNiMax: 5.0,
}

// ====== 农作物重金属富集系数（BCF）& 种植建议 ======
type CropBioaccumulationConfig struct {
	BCF               map[string]map[string]float64
	FoodSafetyLimit   map[string]float64
	LowRiskMaxIgeo    float64
	MediumRiskMaxIgeo float64
}

var DefaultCropBioaccumulation = CropBioaccumulationConfig{
	BCF: map[string]map[string]float64{
		"水稻":   {"Pb": 0.01, "Zn": 0.08, "Cu": 0.05, "As": 0.06, "Hg": 0.02, "Cd": 0.15, "Cr": 0.08, "Ni": 0.03},
		"小麦":   {"Pb": 0.02, "Zn": 0.10, "Cu": 0.04, "As": 0.02, "Hg": 0.01, "Cd": 0.08, "Cr": 0.03, "Ni": 0.05},
		"玉米":   {"Pb": 0.005, "Zn": 0.05, "Cu": 0.03, "As": 0.01, "Hg": 0.005, "Cd": 0.02, "Cr": 0.01, "Ni": 0.02},
		"叶菜":   {"Pb": 0.08, "Zn": 0.25, "Cu": 0.15, "As": 0.10, "Hg": 0.08, "Cd": 0.20, "Cr": 0.12, "Ni": 0.10},
		"根茎类": {"Pb": 0.05, "Zn": 0.15, "Cu": 0.08, "As": 0.08, "Hg": 0.03, "Cd": 0.12, "Cr": 0.06, "Ni": 0.05},
		"瓜果":   {"Pb": 0.005, "Zn": 0.03, "Cu": 0.02, "As": 0.005, "Hg": 0.002, "Cd": 0.01, "Cr": 0.005, "Ni": 0.01},
		"果树":   {"Pb": 0.01, "Zn": 0.04, "Cu": 0.03, "As": 0.01, "Hg": 0.005, "Cd": 0.02, "Cr": 0.01, "Ni": 0.02},
		"茶叶":   {"Pb": 0.05, "Zn": 0.10, "Cu": 0.08, "As": 0.05, "Hg": 0.03, "Cd": 0.06, "Cr": 0.04, "Ni": 0.05},
		"烟草":   {"Pb": 0.10, "Zn": 0.30, "Cu": 0.20, "As": 0.15, "Hg": 0.08, "Cd": 0.25, "Cr": 0.10, "Ni": 0.08},
	},
	FoodSafetyLimit: map[string]float64{
		"Pb": 0.2, "Zn": 50.0, "Cu": 20.0, "As": 0.5, "Hg": 0.02, "Cd": 0.1, "Cr": 1.0, "Ni": 1.0,
	},
	LowRiskMaxIgeo:    1.0,
	MediumRiskMaxIgeo: 3.0,
}

// ====== 全球冶炼史文明年代划分 ======
type CivilizationEpochConfig struct {
	Epochs []struct {
		Name          string
		YearStart     int
		YearEnd       int
		KeyTechnology string
		KeyRegions    []string
		Description   string
	}
}

var DefaultCivilizationEpochs = CivilizationEpochConfig{
	Epochs: []struct {
		Name          string
		YearStart     int
		YearEnd       int
		KeyTechnology string
		KeyRegions    []string
		Description   string
	}{
		{"铜石并用时代", -5000, -3300, "天然铜冷锻/退火", []string{"安纳托利亚", "两河流域", "伊朗"}, "最早的铜器出现，低温加工天然铜"},
		{"青铜时代早期", -3300, -2000, "砷青铜/锡青铜铸造", []string{"美索不达米亚", "埃及", "印度河", "中国西北地区"}, "铜-砷/铜-锡合金冶炼，范铸法"},
		{"青铜时代鼎盛期", -2000, -1200, "失蜡法/大型范铸", []string{"殷商中国", "迈锡尼希腊", "新王国埃及", "米诺斯"}, "大型青铜器出现，锡料贸易网形成"},
		{"铁器时代早期", -1200, -500, "块炼法炼铁", []string{"赫梯", "亚述", "希腊黑暗时代", "中国西周"}, "铁的固态还原，铁器逐渐普及"},
		{"铁器时代鼎盛期", -500, 500, "生铁铸造/炼钢", []string{"中国秦汉", "罗马帝国", "印度孔雀王朝"}, "液态生铁，炒钢法/百炼钢"},
		{"中世纪冶炼", 500, 1500, "水力鼓风/竖炉大型化", []string{"中国唐宋", "阿拉伯帝国", "欧洲中世纪"}, "水力风箱，银铅矿灰吹法普及"},
		{"殖民时代", 1500, 1800, "混汞法提金/大规模开采", []string{"西班牙美洲殖民地", "中欧银矿", "西非"}, "汞齐法贵金属提取，跨大西洋金属贸易"},
		{"工业革命", 1800, 1900, "焦炭高炉/转炉炼钢", []string{"英国", "西欧", "美国东北"}, "现代冶金工业诞生，产量指数级增长"},
	},
}
