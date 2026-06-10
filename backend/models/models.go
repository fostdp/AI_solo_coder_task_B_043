package models

import "time"

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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type XRFMeasurement struct {
	ID                      int       `json:"id"`
	SiteID                  int       `json:"site_id"`
	SampleDepth             string    `json:"sample_depth"`
	MeasurementYear         int       `json:"measurement_year"`
	Pb                      float64   `json:"pb"`
	Zn                      float64   `json:"zn"`
	Cu                      float64   `json:"cu"`
	As                      float64   `json:"as"`
	Hg                      float64   `json:"hg"`
	Cd                      float64   `json:"cd"`
	PH                      float64   `json:"ph"`
	OrganicMatter           float64   `json:"organic_matter"`
	CationExchangeCapacity  float64   `json:"cation_exchange_capacity"`
	SoilType                string    `json:"soil_type"`
	Remark                  string    `json:"remark"`
	CreatedAt               time.Time `json:"created_at"`
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
	CreatedAt       time.Time `json:"created_at"`
}

type PollutionFingerprint struct {
	ID              int     `json:"id"`
	FingerprintName string  `json:"fingerprint_name"`
	MetalType       string  `json:"metal_type"`
	ProcessType     string  `json:"process_type"`
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
	ClusterID       int     `json:"cluster_id"`
	Description     string  `json:"description"`
}

type FingerprintMatchResult struct {
	SiteID         int                `json:"site_id"`
	SiteName       string             `json:"site_name"`
	SiteMetalType  string             `json:"site_metal_type"`
	MatchedFingerprint *PollutionFingerprint `json:"matched_fingerprint"`
	Similarity     float64            `json:"similarity"`
	Distance       float64            `json:"distance"`
	ClusterID      int                `json:"cluster_id"`
	SiteRatios     map[string]float64 `json:"site_ratios"`
}

type RemediationTechnology struct {
	ID                      int      `json:"id"`
	Name                    string   `json:"name"`
	Category                string   `json:"category"`
	ApplicableMetals        []string `json:"applicable_metals"`
	ApplicableSoilTypes     []string `json:"applicable_soil_types"`
	CostLow                 float64  `json:"cost_low"`
	CostHigh                float64  `json:"cost_high"`
	DurationMonthsLow       int      `json:"duration_months_low"`
	DurationMonthsHigh      int      `json:"duration_months_high"`
	RemediationEfficiency   float64  `json:"remediation_efficiency"`
	EnvironmentalImpactScore float64 `json:"environmental_impact_score"`
	ApplicabilityScore      float64  `json:"applicability_score"`
	SustainabilityScore     float64  `json:"sustainability_score"`
	Description             string   `json:"description"`
	Advantages              string   `json:"advantages"`
	Limitations             string   `json:"limitations"`
}

type TechnologyScore struct {
	RemediationTechnology
	TotalScore    float64            `json:"total_score"`
	SubScores     map[string]float64 `json:"sub_scores"`
	MatchedMetals int                `json:"matched_metals"`
}

type RemediationAssessment struct {
	SiteID                int                `json:"site_id"`
	SiteName              string             `json:"site_name"`
	PollutionIndex        float64            `json:"pollution_index"`
	EcoRiskIndex          float64            `json:"eco_risk_index"`
	TopTechnologies       []TechnologyScore  `json:"top_technologies"`
	DetectedMetals        []string           `json:"detected_metals"`
	MetalConcentrations   map[string]float64 `json:"metal_concentrations"`
	SoilType              string             `json:"soil_type"`
	SpeciationData        map[string]*MetalSpeciation `json:"speciation_data"`
}

type RiskStandard struct {
	ID                int     `json:"id"`
	StandardName      string  `json:"standard_name"`
	MetalType         string  `json:"metal_type"`
	ScreeningValue    float64 `json:"screening_value"`
	InterventionValue float64 `json:"intervention_value"`
	Unit              string  `json:"unit"`
	LandUseType       string  `json:"land_use_type"`
}

type Alert struct {
	ID              int       `json:"id"`
	SiteID          int       `json:"site_id"`
	MeasurementID   *int      `json:"measurement_id"`
	AlertType       string    `json:"alert_type"`
	MetalType       string    `json:"metal_type"`
	Concentration   float64   `json:"concentration"`
	Threshold       float64   `json:"threshold"`
	Severity        string    `json:"severity"`
	IsSent          bool      `json:"is_sent"`
	EmailRecipients []string  `json:"email_recipients"`
	Message         string    `json:"message"`
	CreatedAt       time.Time `json:"created_at"`
}

type TrendData struct {
	Year         int                 `json:"year"`
	Metals       map[string]float64  `json:"metals"`
	PollutionIndex float64           `json:"pollution_index"`
}

type PCAResult struct {
	SiteID       int       `json:"site_id"`
	SiteName     string    `json:"site_name"`
	PC1          float64   `json:"pc1"`
	PC2          float64   `json:"pc2"`
	PC3          float64   `json:"pc3"`
	ClusterID    int       `json:"cluster_id"`
	MetalType    string    `json:"metal_type"`
}
