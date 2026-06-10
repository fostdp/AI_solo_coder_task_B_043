package models

import "time"

// =========================================
// 核心数据模型（含新旧API兼容字段）
// =========================================

type Site struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Country     string    `json:"country"`
	MetalType   string    `json:"metal_type"`
	Scale       string    `json:"scale"`
	Era         string    `json:"era"`
	Description string    `json:"description"`
	Longitude   float64   `json:"longitude"`
	Latitude    float64   `json:"latitude"`
	SoilType    string    `json:"soil_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type XRFMeasurement struct {
	ID                     int       `json:"id"`
	SiteID                 int       `json:"site_id"`
	SampleDepth            string    `json:"sample_depth"`
	MeasurementYear        int       `json:"measurement_year"`
	Pb                     float64   `json:"pb"`
	Zn                     float64   `json:"zn"`
	Cu                     float64   `json:"cu"`
	As                     float64   `json:"as"`
	Hg                     float64   `json:"hg"`
	Cd                     float64   `json:"cd"`
	PH                     float64   `json:"ph"`
	OrganicMatter          float64   `json:"organic_matter"`
	CationExchangeCapacity float64   `json:"cation_exchange_capacity"`
	CEC                    float64   `json:"cec"` // alias
	SoilType               string    `json:"soil_type"`
	SoilMoisture           float64   `json:"soil_moisture"`
	PollutionIndex         float64   `json:"pollution_index"`
	MeasurementDate        time.Time `json:"measurement_date"`
	Remark                 string    `json:"remark"`
	CreatedAt              time.Time `json:"created_at"`
}

type SiteWithPollution struct {
	Site
	PollutionIndex float64 `json:"pollution_index"`
	LatestYear     int     `json:"latest_year"`
	Pb             float64 `json:"pb"`
	Zn             float64 `json:"zn"`
	Cu             float64 `json:"cu"`
	As             float64 `json:"as"`
	Hg             float64 `json:"hg"`
	Cd             float64 `json:"cd"`
}

type IsotopeRatio struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementYear int       `json:"measurement_year"`
	Pb206Pb204      float64   `json:"pb206_pb204"`
	Pb207Pb204      float64   `json:"pb207_pb204"`
	Pb208Pb204      float64   `json:"pb208_pb204"`
	Pb206Pb207      float64   `json:"pb206_pb207"`
	Pb208Pb207      float64   `json:"pb208_pb207"`
	Cu65Cu63        float64   `json:"cu65_cu63"`
	Zn68Zn64        float64   `json:"zn68_zn64"`
	Hg202Hg198      float64   `json:"hg202_hg198"`
	CreatedAt       time.Time `json:"created_at"`
}

type MetalSpeciation struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementYear int       `json:"measurement_year"`
	MetalType       string    `json:"metal_type"`
	Exchangeable    float64   `json:"exchangeable"`
	CarbonateBound  float64   `json:"carbonate_bound"`
	FeMnOxideBound  float64   `json:"fe_mn_oxide_bound"`
	OrganicBound    float64   `json:"organic_bound"`
	Residual        float64   `json:"residual"`
	SoilType        string    `json:"soil_type"`
	CreatedAt       time.Time `json:"created_at"`
}

type PollutionFingerprint struct {
	ID              int     `json:"id"`
	FingerprintName string  `json:"fingerprint_name"`
	Name            string  `json:"name"` // alias
	MetalType       string  `json:"metal_type"`
	ProcessType     string  `json:"process_type"`
	Region          string  `json:"region"`
	PbZnRatio       float64 `json:"pb_zn_ratio"`
	CuPbRatio       float64 `json:"cu_pb_ratio"`
	AsHgRatio       float64 `json:"as_hg_ratio"`
	CdZnRatio       float64 `json:"cd_zn_ratio"`
	CuAsRatio       float64 `json:"cu_as_ratio"`
	Pb206Pb207      float64 `json:"pb206_pb207"`
	Pb208Pb207      float64 `json:"pb208_pb207"`
	PCAPc1          float64 `json:"pca_pc1"`
	PCAPc2          float64 `json:"pca_pc2"`
	PCAPc3          float64 `json:"pca_pc3"`
	PC1             float64 `json:"pc1"` // alias
	PC2             float64 `json:"pc2"`
	PC3             float64 `json:"pc3"`
	ClusterID       int     `json:"cluster_id"`
	Description     string  `json:"description"`
}

// ============== 指纹匹配 ==============

// Fingerprint 前端展示用的精简指纹结构
type Fingerprint struct {
	FingerprintID int               `json:"fingerprint_id"`
	MetalType     string            `json:"metal_type"`
	ProcessType   string            `json:"process_type"`
	Region        string            `json:"region"`
	Description   string            `json:"description"`
	Similarity    float64           `json:"similarity"`
	Distance      float64           `json:"distance"`
	Ratios        map[string]float64 `json:"ratios"`
	PCAProjection []float64         `json:"pca_projection"`
	ClusterID     int               `json:"cluster_id"`
}

// FingerprintMatchResult 指纹匹配结果
type FingerprintMatchResult struct {
	SiteID            int                     `json:"site_id,omitempty"`
	SiteName          string                  `json:"site_name,omitempty"`
	BestMatch         *Fingerprint            `json:"best_match"`
	Matches           []Fingerprint           `json:"matches"`
	Similarity        float64                 `json:"similarity"`
	Distance          float64                 `json:"distance"`
	SiteRatios        map[string]float64      `json:"site_ratios,omitempty"`
	ClusterID         int                     `json:"cluster_id,omitempty"`
	MatchedFingerprint *PollutionFingerprint  `json:"matched_fingerprint,omitempty"` // alias
}

// ============== 修复技术 ==============

type RemediationTechnology struct {
	ID                      int      `json:"id"`
	TechName                string   `json:"tech_name"`
	Name                    string   `json:"name"` // alias
	TechType                string   `json:"tech_type"`
	Category                string   `json:"category"`
	Description             string   `json:"description"`
	ApplicableMetals        string   `json:"applicable_metals"` // comma-separated
	ApplicableMetalsList    []string `json:"applicable_metals_list,omitempty"`
	ApplicableSoilTypes     []string `json:"applicable_soil_types"`
	SoilTypes               string   `json:"soil_types"` // comma-separated
	AvgCostPerM3            float64  `json:"avg_cost_per_m3"`
	CostLow                 float64  `json:"cost_low"`
	CostHigh                float64  `json:"cost_high"`
	AvgDurationMonths       int      `json:"avg_duration_months"`
	DurationMonthsLow       int      `json:"duration_months_low"`
	DurationMonthsHigh      int      `json:"duration_months_high"`
	RemediationEfficiency   float64  `json:"remediation_efficiency"`
	EnvironmentalImpactScore float64 `json:"environmental_impact_score"`
	SustainabilityScore     float64  `json:"sustainability_score"`
	ApplicabilityScore      float64  `json:"applicability_score"`
	ApplicableRegions       string   `json:"applicable_regions"`
	Advantages              string   `json:"advantages"`
	Limitations             string   `json:"limitations"`
}

// TechnologyScore 技术评分
type TechnologyScore struct {
	ID               int               `json:"id"`
	Rank             int               `json:"rank"`
	TechName         string            `json:"tech_name"`
	TechType         string            `json:"tech_type"`
	Description      string            `json:"description"`
	FinalScore       float64           `json:"final_score"`
	TotalScore       float64           `json:"total_score"` // alias
	Closeness        float64           `json:"closeness"`
	MetalCoverage    float64           `json:"metal_coverage"`
	EfficiencyScore  float64           `json:"efficiency_score"`
	SoilScore        float64           `json:"soil_score"`
	CostScore        float64           `json:"cost_score"`
	DurationScore    float64           `json:"duration_score"`
	EnvScore         float64           `json:"env_score"`
	SustainScore     float64           `json:"sustain_score"`
	SubScores        map[string]float64 `json:"sub_scores"`
	WeightsUsed      map[string]float64 `json:"weights_used"`
	AlphaUsed        float64           `json:"alpha_used"`
	MatchedMetals    int               `json:"matched_metals"`
	*RemediationTechnology `json:",inline,omitempty"`
}

// RemediationAssessment 修复评估结果
type RemediationAssessment struct {
	ID                  int                    `json:"id"`
	SiteID              int                    `json:"site_id"`
	SiteName            string                 `json:"site_name"`
	PollutionIndex      float64                `json:"pollution_index"`
	EcoRiskIndex        float64                `json:"eco_risk_index"`
	MobilityLevel       string                 `json:"mobility_level"`
	RecommendedTechs    []TechnologyScore      `json:"recommended_techs"`
	TopTechnologies     []TechnologyScore      `json:"top_technologies"` // alias
	DetectedMetals      []string               `json:"detected_metals"`
	MetalConcs          map[string]float64     `json:"metal_concs"`
	MetalConcentrations map[string]float64     `json:"metal_concentrations"` // alias
	SoilType            string                 `json:"soil_type"`
	SpeciationData      map[string]*MetalSpeciation `json:"speciation_data"`
	AssessmentDate      string                 `json:"assessment_date"`
}

// ============== 风险标准 ==============

type RiskStandard struct {
	ID                int     `json:"id"`
	StandardName      string  `json:"standard_name"`
	MetalType         string  `json:"metal_type"`
	ScreeningValue    float64 `json:"screening_value"`
	InterventionValue float64 `json:"intervention_value"`
	Unit              string  `json:"unit"`
	LandUseType       string  `json:"land_use_type"`
}

// ============== 告警 ==============

type Alert struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementID   *int      `json:"measurement_id"`
	MeasurementYear int       `json:"measurement_year"`
	AlertType       string    `json:"alert_type"`
	MetalType       string    `json:"metal_type"`
	Concentration   float64   `json:"concentration"`
	Threshold       float64   `json:"threshold"`
	ExceedRatio     float64   `json:"exceed_ratio"`
	Severity        string    `json:"severity"`
	PollutionIndex  float64   `json:"pollution_index"`
	EcoRiskIndex    float64   `json:"eco_risk_index"`
	IsSent          bool      `json:"is_sent"`
	IsResolved      bool      `json:"is_resolved"`
	EmailRecipients []string  `json:"email_recipients"`
	Message         string    `json:"message"`
	CreatedAt       time.Time `json:"created_at"`
}

// ============== 趋势数据 ==============

type TrendData struct {
	Year            int                 `json:"year"`
	Pb              float64             `json:"pb"`
	Zn              float64             `json:"zn"`
	Cu              float64             `json:"cu"`
	As              float64             `json:"as"`
	Hg              float64             `json:"hg"`
	Cd              float64             `json:"cd"`
	PollutionIndex  float64             `json:"pollution_index"`
	PH              float64             `json:"ph"`
	OrganicMatter   float64             `json:"organic_matter"`
	CEC             float64             `json:"cec"`
	SoilMoisture    float64             `json:"soil_moisture"`
	MeasurementDate time.Time           `json:"measurement_date"`
	// alias for legacy
	Metals          map[string]float64  `json:"metals,omitempty"`
}

// ============== PCA 结果 ==============

type PCAResult struct {
	SiteID       int       `json:"site_id"`
	SiteName     string    `json:"site_name"`
	PC1          float64   `json:"pc1"`
	PC2          float64   `json:"pc2"`
	PC3          float64   `json:"pc3"`
	ClusterID    int       `json:"cluster_id"`
	MetalType    string    `json:"metal_type"`
}

// PCAResultWithQuality 带质量评估的PCA结果
type PCAResultWithQuality struct {
	Projections        [][]float64 `json:"projections"`
	Labels             []int       `json:"labels"`
	K                  int         `json:"k"`
	ExplainedVariance  []float64   `json:"explained_variance"`
	CumulativeVariance []float64   `json:"cumulative_variance"`
	SilhouetteScore    float64     `json:"silhouette_score"`
	BootstrapStability float64     `json:"bootstrap_stability"`
	GapStatistic       float64     `json:"gap_statistic"`
	SSE                float64     `json:"sse"`
	// alias: full site list with per-site pc
	SiteResults        []PCAResult `json:"site_results,omitempty"`
}

// Stats 系统概览统计（辅助结构）
type StatsData struct {
	Counts             map[string]int `json:"counts"`
	SeverityBreakdown  map[string]int `json:"severity_breakdown"`
	TechnologiesCount  int            `json:"technologies_count"`
	FingerprintsCount  int            `json:"fingerprints_count"`
	PendingAlerts      int            `json:"pending_alerts"`
}

// ============== 农田土壤安全评估（新增） ==============

type FarmGeoAccumulationResult struct {
	Metal          string  `json:"metal"`
	Concentration  float64 `json:"concentration"`
	Igeo           float64 `json:"igeo"`
	Level          int     `json:"level"`
	LevelDesc      string  `json:"level_desc"`
}

type FarmSampleGeoAccumulation struct {
	SampleName   string                      `json:"sample_name"`
	MetalResults []FarmGeoAccumulationResult `json:"metal_results"`
	MaxIgeo      float64                     `json:"max_igeo"`
	MaxIgeoMetal string                      `json:"max_igeo_metal"`
}

type FarmSampleEcoRiskResult struct {
	SampleName  string             `json:"sample_name"`
	RI          float64            `json:"ri"`
	RiskLevel   string             `json:"risk_level"`
	MetalEri    map[string]float64 `json:"metal_eri"`
	MaxEri      float64            `json:"max_eri"`
	MaxEriMetal string             `json:"max_eri_metal"`
}

type FarmDistanceDecayResult struct {
	DistanceLabel string  `json:"distance_label"`
	SampleCount   int     `json:"sample_count"`
	AvgIgeo       float64 `json:"avg_igeo"`
	AvgRI         float64 `json:"avg_ri"`
	MaxIgeo       float64 `json:"max_igeo"`
	MaxRI         float64 `json:"max_ri"`
}

type FarmCropPrediction struct {
	Metal             string  `json:"metal"`
	SoilConcentration float64 `json:"soil_concentration"`
	BCF               float64 `json:"bcf"`
	PredictedCropConc float64 `json:"predicted_crop_conc"`
	FoodLimit         float64 `json:"food_limit"`
	ExceedRatio       float64 `json:"exceed_ratio"`
	IsExceed          bool    `json:"is_exceed"`
	IsClose           bool    `json:"is_close"`
}

type FarmCropRecommendation struct {
	LandUseType     string                 `json:"land_use_type"`
	RiskLevel       string                 `json:"risk_level"`
	RiskColor       string                 `json:"risk_color"`
	Recommendations []string               `json:"recommendations"`
	Predictions     []FarmCropPrediction   `json:"predictions"`
	ExceedCount     int                    `json:"exceed_count"`
	CloseCount      int                    `json:"close_count"`
	TotalMetals     int                    `json:"total_metals"`
}

type FarmSafetyAssessmentResult struct {
	SiteID              int                           `json:"site_id"`
	SiteName            string                        `json:"site_name"`
	AssessmentDate      string                        `json:"assessment_date"`
	SampleResults       []FarmSampleGeoAccumulation   `json:"sample_results"`
	EcoRiskResults      []FarmSampleEcoRiskResult     `json:"eco_risk_results"`
	DistanceDecay       []FarmDistanceDecayResult     `json:"distance_decay"`
	CropRecommendations []FarmCropRecommendation      `json:"crop_recommendations"`
	OverallRiskLevel    string                        `json:"overall_risk_level"`
	OverallRiskColor    string                        `json:"overall_risk_color"`
	MaxIgeo             float64                       `json:"max_igeo"`
	MaxEri              float64                       `json:"max_eri"`
	TotalRI             float64                       `json:"total_ri"`
	Summary             string                        `json:"summary"`
}

// ============== 矿渣成分 ==============

type SlagComposition struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementYear int       `json:"measurement_year"`
	SampleDepth     string    `json:"sample_depth"`
	SiO2            float64   `json:"sio2"`
	Al2O3           float64   `json:"al2o3"`
	CaO             float64   `json:"cao"`
	FeO             float64   `json:"feo"`
	Fe2O3           float64   `json:"fe2o3"`
	MgO             float64   `json:"mgo"`
	MnO             float64   `json:"mno"`
	P2O5            float64   `json:"p2o5"`
	SO3             float64   `json:"so3"`
	K2O             float64   `json:"k2o"`
	Na2O            float64   `json:"na2o"`
	TiO2            float64   `json:"tio2"`
	Fayalite        float64   `json:"fayalite"`
	Wollastonite    float64   `json:"wollastonite"`
	Anorthite       float64   `json:"anorthite"`
	Diopside        float64   `json:"diopside"`
	Magnetite       float64   `json:"magnetite"`
	Hematite        float64   `json:"hematite"`
	Wuestite        float64   `json:"wuestite"`
	GlassPhase      float64   `json:"glass_phase"`
	OtherMinerals   float64   `json:"other_minerals"`
	PbLeaching      float64   `json:"pb_leaching"`
	CdLeaching      float64   `json:"cd_leaching"`
	AsLeaching      float64   `json:"as_leaching"`
	HgLeaching      float64   `json:"hg_leaching"`
	CrLeaching      float64   `json:"cr_leaching"`
	NiLeaching      float64   `json:"ni_leaching"`
	Density         float64   `json:"density"`
	SpecificSurface float64   `json:"specific_surface"`
	LossOnIgnition  float64   `json:"loss_on_ignition"`
	Remark          string    `json:"remark"`
	CreatedAt       time.Time `json:"created_at"`
}

// ============== 农田土壤 ==============

type FarmlandSoil struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementYear int       `json:"measurement_year"`
	DistanceFromSite int      `json:"distance_from_site"`
	Direction       string    `json:"direction"`
	LandUseType     string    `json:"land_use_type"`
	Pb              float64   `json:"pb"`
	Zn              float64   `json:"zn"`
	Cu              float64   `json:"cu"`
	As              float64   `json:"as" gorm:"column:as_"`
	Hg              float64   `json:"hg"`
	Cd              float64   `json:"cd"`
	Cr              float64   `json:"cr"`
	Ni              float64   `json:"ni"`
	PH              *float64  `json:"ph"`
	OrganicMatter   *float64  `json:"organic_matter"`
	CEC             *float64  `json:"cec"`
	SoilType        string    `json:"soil_type"`
	MainCrops       []string  `json:"main_crops"`
	CreatedAt       time.Time `json:"created_at"`
}

// ============== 冶炼工艺反演 ==============

type SmeltingProcessInversion struct {
	ID                      int                    `json:"id"`
	SiteID                  int                    `json:"site_id"`
	MeasurementYear         int                    `json:"measurement_year"`
	EstimatedTemperature    float64                `json:"estimated_temperature"`
	TemperatureConfidence   float64                `json:"temperature_confidence"`
	ReducingAgent           string                 `json:"reducing_agent"`
	ReducingAgentConfidence float64                `json:"reducing_agent_confidence"`
	BPNNPosterior           map[string]interface{} `json:"bpnn_posterior"`
	BayesPosterior          map[string]float64     `json:"bayes_posterior"`
	ProcessTypeDetailed     string                 `json:"process_type_detailed"`
	ProcessEraEstimate      string                 `json:"process_era_estimate"`
	InputFeatures           map[string]interface{} `json:"input_features"`
	BPNNMSE                 float64                `json:"bpnn_mse"`
	BayesKLD                float64                `json:"bayes_kld"`
	QualityLevel            string                 `json:"quality_level"`
	Remark                  string                 `json:"remark"`
	CreatedAt               time.Time              `json:"created_at"`
}

// ============== 资源化评估 ==============

type ResourceUtilizationAssessment struct {
	ID                      int                    `json:"id"`
	SiteID                  int                    `json:"site_id"`
	MeasurementYear         int                    `json:"measurement_year"`
	CementBlendedFeasibility string                `json:"cement_blended_feasibility"`
	CementBlendedScore      float64                `json:"cement_blended_score"`
	CementBlendedGrade      string                 `json:"cement_blended_grade"`
	CementDetails           map[string]interface{} `json:"cement_details"`
	RoadBaseFeasibility     string                 `json:"road_base_feasibility"`
	RoadBaseScore           float64                `json:"road_base_score"`
	RoadBaseGrade           string                 `json:"road_base_grade"`
	RoadDetails             map[string]interface{} `json:"road_details"`
	OtherUses               map[string]interface{} `json:"other_uses"`
	LeachingRiskLevel       string                 `json:"leaching_risk_level"`
	LeachingRiskDetails     map[string]interface{} `json:"leaching_risk_details"`
	RecommendedUse          string                 `json:"recommended_use"`
	UtilizationPlan         map[string]interface{} `json:"utilization_plan"`
	CreatedAt               time.Time              `json:"created_at"`
}

// ============== 农田安全结果相关 ==============

type GeoAccumulation struct {
	Metal         string  `json:"metal"`
	Concentration float64 `json:"concentration"`
	Background    float64 `json:"background"`
	Igeo          float64 `json:"igeo"`
	Level         string  `json:"level"`
	Description   string  `json:"description"`
}

type CropRecommendation struct {
	Crop                      string  `json:"crop"`
	RiskLevel                 string  `json:"risk_level"`
	Reason                    string  `json:"reason"`
	Advice                    string  `json:"advice"`
	BiomassAccumulationFactor float64 `json:"biomass_accumulation_factor"`
}

type FarmSafetyAssessment struct {
	SiteID              int                      `json:"site_id"`
	SiteName            string                   `json:"site_name"`
	Year                int                      `json:"year"`
	TotalSamples        int                      `json:"total_samples"`
	AverageIgeo         map[string]float64       `json:"average_igeo"`
	AverageRI           float64                  `json:"average_ri"`
	RiskLevel           string                   `json:"risk_level"`
	DistanceRisks       []map[string]interface{} `json:"distance_risks"`
	CropRecommendations []CropRecommendation     `json:"crop_recommendations"`
	Details             []FarmlandSoil           `json:"details"`
}

// ============== 冶炼工艺反演结果 ==============

type BPNNNetworkInfo struct {
	InputSize        int     `json:"input_size"`
	HiddenSizes      []int   `json:"hidden_sizes"`
	OutputSizeTemp   int     `json:"output_size_temp"`
	OutputSizeAgent  int     `json:"output_size_agent"`
	Activation       string  `json:"activation"`
	TrainedEpochs    int     `json:"trained_epochs"`
	FinalLoss        float64 `json:"final_loss"`
}

type SmeltingProcessResult struct {
	SiteID               int                    `json:"site_id"`
	SiteName             string                 `json:"site_name"`
	Inversion            SmeltingProcessInversion `json:"inversion"`
	NetworkInfo          BPNNNetworkInfo        `json:"network_info"`
	TemperatureDistribution []float64           `json:"temperature_distribution"`
	AgentProbabilities   map[string]float64     `json:"agent_probabilities"`
}

// ============== 时间线对比 ==============

type TimelinePeak struct {
	SiteID     int     `json:"site_id"`
	SiteName   string  `json:"site_name"`
	PeakYear   int     `json:"peak_year"`
	PeakValue  float64 `json:"peak_value"`
	MetalType  string  `json:"metal_type"`
	Confidence float64 `json:"confidence"`
}

type CivilizationEpoch struct {
	EpochName      string   `json:"epoch_name"`
	YearRange      string   `json:"year_range"`
	YearStart      int      `json:"year_start"`
	YearEnd        int      `json:"year_end"`
	KeySites       []string `json:"key_sites"`
	KeyTechnology  string   `json:"key_technology"`
	Description    string   `json:"description"`
}

type TimelineComparisonResult struct {
	Sites             []interface{}            `json:"sites"`
	Peaks             []TimelinePeak           `json:"peaks"`
	CivilizationEpochs []CivilizationEpoch     `json:"civilization_epochs"`
	GlobalTrend       map[string][]float64     `json:"global_trend"`
}

// ============== 矿渣详细结果 ==============

type CementStandardCheck struct {
	Item          string  `json:"item"`
	Value         float64 `json:"value"`
	StandardLimit float64 `json:"standard_limit"`
	Pass          bool    `json:"pass"`
	Note          string  `json:"note"`
}

type RoadStandardCheck struct {
	Item          string  `json:"item"`
	Value         float64 `json:"value"`
	StandardLimit float64 `json:"standard_limit"`
	Pass          bool    `json:"pass"`
	Grade         string  `json:"grade"`
}

type SlagRecycleResult struct {
	SiteID        int                      `json:"site_id"`
	SiteName      string                   `json:"site_name"`
	Composition   *SlagComposition         `json:"composition"`
	Assessment    *ResourceUtilizationAssessment `json:"assessment"`
	CementChecks  []CementStandardCheck    `json:"cement_checks"`
	RoadChecks    []RoadStandardCheck      `json:"road_checks"`
	ProcessFlow   []map[string]interface{} `json:"process_flow"`
}
