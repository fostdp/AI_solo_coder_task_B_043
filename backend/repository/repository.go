package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"

	"archaeology-pollution-system/database"
	"archaeology-pollution-system/models"
)

func GetAllSitesWithPollution(ctx context.Context) ([]models.SiteWithPollution, error) {
	query := `
		SELECT 
			s.id, s.name, s.country, s.metal_type, s.scale, s.era, s.description,
			ST_X(s.geom) as longitude, ST_Y(s.geom) as latitude,
			COALESCE(v.pollution_index, 0) as pollution_index,
			COALESCE(v.measurement_year, 0) as latest_year,
			COALESCE(v.pb, 0), COALESCE(v.zn, 0), COALESCE(v.cu, 0),
			COALESCE(v.as_val, 0), COALESCE(v.hg, 0), COALESCE(v.cd, 0)
		FROM sites s
		LEFT JOIN v_pollution_index v ON s.id = v.site_id
		ORDER BY s.id
	`

	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []models.SiteWithPollution
	for rows.Next() {
		var s models.SiteWithPollution
		err := rows.Scan(
			&s.ID, &s.Name, &s.Country, &s.MetalType, &s.Scale, &s.Era, &s.Description,
			&s.Longitude, &s.Latitude,
			&s.PollutionIndex, &s.LatestYear,
			&s.Pb, &s.Zn, &s.Cu, &s.As, &s.Hg, &s.Cd,
		)
		if err != nil {
			return nil, err
		}
		sites = append(sites, s)
	}
	return sites, nil
}

func GetSiteByID(ctx context.Context, id int) (*models.Site, error) {
	query := `
		SELECT id, name, country, metal_type, scale, era, description,
			ST_X(geom) as longitude, ST_Y(geom) as latitude, created_at, updated_at
		FROM sites WHERE id = $1
	`
	var s models.Site
	err := database.Pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.Country, &s.MetalType, &s.Scale, &s.Era, &s.Description,
		&s.Longitude, &s.Latitude, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func GetXRFMeasurementsBySite(ctx context.Context, siteID int, limitYears int) ([]models.XRFMeasurement, error) {
	query := `
		SELECT id, site_id, sample_depth, measurement_year,
			pb, zn, cu, as_val, hg, cd,
			ph, organic_matter, cation_exchange_capacity, soil_type, remark, created_at
		FROM xrf_measurements
		WHERE site_id = $1
		ORDER BY measurement_year DESC
		LIMIT $2
	`
	rows, err := database.Pool.Query(ctx, query, siteID, limitYears)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var measurements []models.XRFMeasurement
	for rows.Next() {
		var m models.XRFMeasurement
		var sampleDepth, soilType, remark sql.NullString
		var ph, organicMatter, cec sql.NullFloat64
		err := rows.Scan(
			&m.ID, &m.SiteID, &sampleDepth, &m.MeasurementYear,
			&m.Pb, &m.Zn, &m.Cu, &m.As, &m.Hg, &m.Cd,
			&ph, &organicMatter, &cec, &soilType, &remark, &m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		m.SampleDepth = sampleDepth.String
		m.SoilType = soilType.String
		m.Remark = remark.String
		m.PH = ph.Float64
		m.OrganicMatter = organicMatter.Float64
		m.CationExchangeCapacity = cec.Float64
		measurements = append(measurements, m)
	}
	return measurements, nil
}

func InsertXRFMeasurement(ctx context.Context, m *models.XRFMeasurement) error {
	query := `
		INSERT INTO xrf_measurements 
		(site_id, sample_depth, measurement_year, pb, zn, cu, as_val, hg, cd, ph, organic_matter, cation_exchange_capacity, soil_type, remark)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (site_id, measurement_year, sample_depth) DO UPDATE SET
			pb = EXCLUDED.pb, zn = EXCLUDED.zn, cu = EXCLUDED.cu, as_val = EXCLUDED.as_val,
			hg = EXCLUDED.hg, cd = EXCLUDED.cd, ph = EXCLUDED.ph,
			organic_matter = EXCLUDED.organic_matter,
			cation_exchange_capacity = EXCLUDED.cation_exchange_capacity,
			soil_type = EXCLUDED.soil_type, remark = EXCLUDED.remark,
			created_at = CURRENT_TIMESTAMP
		RETURNING id, created_at
	`
	err := database.Pool.QueryRow(ctx, query,
		m.SiteID, m.SampleDepth, m.MeasurementYear,
		m.Pb, m.Zn, m.Cu, m.As, m.Hg, m.Cd,
		m.PH, m.OrganicMatter, m.CationExchangeCapacity,
		m.SoilType, m.Remark,
	).Scan(&m.ID, &m.CreatedAt)
	return err
}

func GetAllPollutionFingerprints(ctx context.Context) ([]models.PollutionFingerprint, error) {
	query := `
		SELECT id, fingerprint_name, metal_type, process_type,
			pb_zn_ratio, cu_pb_ratio, as_hg_ratio, cd_zn_ratio, cu_as_ratio,
			pb206_pb207, pb208_pb207, pca_pc1, pca_pc2, pca_pc3, cluster_id, description
		FROM pollution_fingerprints
		ORDER BY id
	`
	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fingerprints []models.PollutionFingerprint
	for rows.Next() {
		var f models.PollutionFingerprint
		err := rows.Scan(
			&f.ID, &f.FingerprintName, &f.MetalType, &f.ProcessType,
			&f.PbZnRatio, &f.CuPbRatio, &f.AsHgRatio, &f.CdZnRatio, &f.CuAsRatio,
			&f.Pb206Pb207, &f.Pb208Pb207, &f.PCAPc1, &f.PCAPc2, &f.PCAPc3, &f.ClusterID, &f.Description,
		)
		if err != nil {
			return nil, err
		}
		fingerprints = append(fingerprints, f)
	}
	return fingerprints, nil
}

func GetAllRemediationTechnologies(ctx context.Context) ([]models.RemediationTechnology, error) {
	query := `
		SELECT id, name, category, applicable_metals, applicable_soil_types,
			cost_low, cost_high, duration_months_low, duration_months_high,
			remediation_efficiency, environmental_impact_score, applicability_score, sustainability_score,
			description, advantages, limitations
		FROM remediation_technologies
		ORDER BY id
	`
	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var technologies []models.RemediationTechnology
	for rows.Next() {
		var t models.RemediationTechnology
		err := rows.Scan(
			&t.ID, &t.Name, &t.Category, &t.ApplicableMetals, &t.ApplicableSoilTypes,
			&t.CostLow, &t.CostHigh, &t.DurationMonthsLow, &t.DurationMonthsHigh,
			&t.RemediationEfficiency, &t.EnvironmentalImpactScore, &t.ApplicabilityScore, &t.SustainabilityScore,
			&t.Description, &t.Advantages, &t.Limitations,
		)
		if err != nil {
			return nil, err
		}
		technologies = append(technologies, t)
	}
	return technologies, nil
}

func GetRiskStandards(ctx context.Context) ([]models.RiskStandard, error) {
	query := `
		SELECT id, standard_name, metal_type, screening_value, intervention_value, unit, land_use_type
		FROM risk_standards
		ORDER BY metal_type
	`
	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var standards []models.RiskStandard
	for rows.Next() {
		var s models.RiskStandard
		err := rows.Scan(
			&s.ID, &s.StandardName, &s.MetalType, &s.ScreeningValue, &s.InterventionValue, &s.Unit, &s.LandUseType,
		)
		if err != nil {
			return nil, err
		}
		standards = append(standards, s)
	}
	return standards, nil
}

func GetAlerts(ctx context.Context, limit int) ([]models.Alert, error) {
	query := `
		SELECT id, site_id, measurement_id, alert_type, metal_type,
			concentration, threshold, severity, is_sent, email_recipients, message, created_at
		FROM alerts
		ORDER BY created_at DESC
		LIMIT $1
	`
	rows, err := database.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var measurementID sql.NullInt64
		err := rows.Scan(
			&a.ID, &a.SiteID, &measurementID, &a.AlertType, &a.MetalType,
			&a.Concentration, &a.Threshold, &a.Severity, &a.IsSent, &a.EmailRecipients, &a.Message, &a.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if measurementID.Valid {
			id := int(measurementID.Int64)
			a.MeasurementID = &id
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func InsertAlert(ctx context.Context, alert *models.Alert) error {
	query := `
		INSERT INTO alerts (site_id, measurement_id, alert_type, metal_type, concentration, threshold, severity, is_sent, email_recipients, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	err := database.Pool.QueryRow(ctx, query,
		alert.SiteID, alert.MeasurementID, alert.AlertType, alert.MetalType,
		alert.Concentration, alert.Threshold, alert.Severity, alert.IsSent,
		alert.EmailRecipients, alert.Message,
	).Scan(&alert.ID, &alert.CreatedAt)
	return err
}

func UpdateAlertSent(ctx context.Context, alertID int) error {
	query := `UPDATE alerts SET is_sent = TRUE WHERE id = $1`
	_, err := database.Pool.Exec(ctx, query, alertID)
	return err
}

func GetMetalSpeciation(ctx context.Context, siteID int, year int) ([]models.MetalSpeciation, error) {
	query := `
		SELECT id, site_id, measurement_year, metal_type,
			exchangeable, carbonate_bound, fe_mn_oxide_bound, organic_bound, residual, created_at
		FROM metal_speciation
		WHERE site_id = $1 AND measurement_year = $2
	`
	rows, err := database.Pool.Query(ctx, query, siteID, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var speciations []models.MetalSpeciation
	for rows.Next() {
		var s models.MetalSpeciation
		err := rows.Scan(
			&s.ID, &s.SiteID, &s.MeasurementYear, &s.MetalType,
			&s.Exchangeable, &s.CarbonateBound, &s.FeMnOxideBound, &s.OrganicBound, &s.Residual, &s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		speciations = append(speciations, s)
	}
	return speciations, nil
}

func GetIsotopeRatios(ctx context.Context, siteID int, year int) (*models.IsotopeRatio, error) {
	query := `
		SELECT id, site_id, measurement_year,
			pb206_pb204, pb207_pb204, pb208_pb204, pb206_pb207, pb208_pb207,
			cu65_cu63, zn68_zn64, hg202_hg198, created_at
		FROM isotope_ratios
		WHERE site_id = $1 AND measurement_year = $2
	`
	var iso models.IsotopeRatio
	err := database.Pool.QueryRow(ctx, query, siteID, year).Scan(
		&iso.ID, &iso.SiteID, &iso.MeasurementYear,
		&iso.Pb206Pb204, &iso.Pb207Pb204, &iso.Pb208Pb204, &iso.Pb206Pb207, &iso.Pb208Pb207,
		&iso.Cu65Cu63, &iso.Zn68Zn64, &iso.Hg202Hg198, &iso.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &iso, err
}

func CalculatePollutionIndex(pb, zn, cu, as, hg, cd float64) float64 {
	standards := map[string]float64{
		"Pb": 800.0, "Zn": 5000.0, "Cu": 18000.0,
		"As": 250.0, "Hg": 38.0, "Cd": 47.0,
	}
	metals := map[string]float64{
		"Pb": pb, "Zn": zn, "Cu": cu,
		"As": as, "Hg": hg, "Cd": cd,
	}
	for metal, val := range metals {
		if val > 0 {
			sum += val / standards[metal]
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return math.Round(sum/float64(count)*10000) / 10000
}

// =========================================
// 兼容包装函数（新模块命名风格）
// =========================================

func GetSitesWithPollution(ctx context.Context) ([]models.SiteWithPollution, error) {
	return GetAllSitesWithPollution(ctx)
}

func GetSite(ctx context.Context, id int) (*models.Site, error) {
	return GetSiteByID(ctx, id)
}

func GetXRFMeasurements(ctx context.Context, siteID int, limitYears int) ([]models.XRFMeasurement, error) {
	return GetXRFMeasurementsBySite(ctx, siteID, limitYears)
}

func GetAllXRFMeasurements(ctx context.Context) ([]models.XRFMeasurement, error) {
	sites, err := GetAllSitesWithPollution(ctx)
	if err != nil {
		return nil, err
	}
	var all []models.XRFMeasurement
	for _, s := range sites {
		ms, err := GetXRFMeasurementsBySite(ctx, s.ID, 1)
		if err == nil && len(ms) > 0 {
			all = append(all, ms[0])
		}
	}
	return all, nil
}

func UpsertXRFMeasurement(ctx context.Context, m *models.XRFMeasurement) error {
	return InsertXRFMeasurement(ctx, m)
}

func GetAllFingerprints(ctx context.Context) ([]models.PollutionFingerprint, error) {
	return GetAllPollutionFingerprints(ctx)
}

func GetAllRemediationTechnologies(ctx context.Context) ([]models.RemediationTechnology, error) {
	rows, err := database.GetPool(ctx).Query(ctx, `
		SELECT id, tech_name, tech_type, description, applicable_metals,
	       remediation_efficiency, avg_cost_per_m3, avg_duration_months,
	       soil_types, environmental_impact_score, sustainability_score,
	       applicable_regions, advantages, limitations
		FROM remediation_technologies
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var techs []models.RemediationTechnology
	for rows.Next() {
		var t models.RemediationTechnology
		err := rows.Scan(
			&t.ID, &t.TechName, &t.TechType, &t.Description,
			&t.ApplicableMetals, &t.RemediationEfficiency, &t.AvgCostPerM3,
			&t.AvgDurationMonths, &t.SoilTypes, &t.EnvironmentalImpactScore,
			&t.SustainabilityScore, &t.ApplicableRegions, &t.Advantages,
			&t.Limitations)
		if err != nil {
			return nil, err
		}
		techs = append(techs, t)
	}
	return techs, nil
}

func GetAllRiskStandards(ctx context.Context) ([]models.RiskStandard, error) {
	return GetRiskStandards(ctx)
}

func GetAlerts(ctx context.Context, siteID int, limit int) ([]models.Alert, error) {
	query := `
		SELECT id, site_id, alert_type, metal_type, severity,
		       concentration, threshold, exceed_ratio,
		       pollution_index, eco_risk_index, message,
		       is_sent, is_resolved, email_recipients, created_at
		FROM alerts
	`
	args := []interface{}{}
	if siteID > 0 {
		query += " WHERE site_id = $1"
		args = append(args, siteID)
	}
	query += " ORDER BY created_at DESC LIMIT $2"
	args = append(args, limit)
	rows, err := database.GetPool(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		recipJSON := []byte{}
		err := rows.Scan(&a.ID, &a.SiteID, &a.AlertType, &a.MetalType,
			&a.Severity, &a.Concentration, &a.Threshold,
			&a.ExceedRatio, &a.PollutionIndex, &a.EcoRiskIndex,
			&a.Message, &a.IsSent, &a.IsResolved, &recipJSON, &a.CreatedAt)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(recipJSON, &a.EmailRecipients)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func CreateAlert(ctx context.Context, alert *models.Alert) (*models.Alert, error) {
	err := InsertAlert(ctx, alert)
	if err != nil {
		return nil, err
	}
	return alert, nil
}

func GetLatestIsotopeRatio(ctx context.Context, siteID int) (*models.IsotopeRatio, error) {
	return GetIsotopeRatios(ctx, siteID, 0)
}

func GetLatestMetalSpeciation(ctx context.Context, siteID int) (*models.MetalSpeciation, error) {
	specs, err := GetMetalSpeciation(ctx, siteID, 0)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	return &specs[0], nil
}

func SaveRemediationAssessment(ctx context.Context, a *models.RemediationAssessment) (int, error) {
	if a == nil {
		return 0, nil
	}
	techJSON, _ := json.Marshal(a.RecommendedTechs)
	concsJSON, _ := json.Marshal(a.MetalConcs)
	metalsJSON, _ := json.Marshal(a.DetectedMetals)
	var id int
	err := database.GetPool(ctx).QueryRow(ctx, `
		INSERT INTO remediation_assessments
		(site_id, detected_metals, metal_concentrations,
		 pollution_index, eco_risk_index, mobility_level,
		 recommended_techs, assessment_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, a.SiteID, metalsJSON, concsJSON, a.PollutionIndex,
		a.EcoRiskIndex, a.MobilityLevel, techJSON, a.AssessmentDate).Scan(&id)
	return id, err
}
