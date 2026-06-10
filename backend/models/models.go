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
