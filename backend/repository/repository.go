package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"

	"github.com/lib/pq"

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

func GetSlagComposition(ctx context.Context, siteID int, year int) (*models.SlagComposition, error) {
	query := `
		SELECT id, site_id, measurement_year, sample_depth,
			sio2, al2o3, cao, feo, fe2o3, mgo, mno, p2o5, so3, k2o, na2o, tio2,
			fayalite, wollastonite, anorthite, diopside, magnetite, hematite, wuestite,
			glass_phase, other_minerals,
			pb_leaching, cd_leaching, as_leaching, hg_leaching, cr_leaching, ni_leaching,
			density, specific_surface, loss_on_ignition, remark, created_at
		FROM slag_compositions
		WHERE site_id = $1
		ORDER BY measurement_year DESC
		LIMIT 1
	`
	var s models.SlagComposition
	var sampleDepth, remark sql.NullString
	err := database.Pool.QueryRow(ctx, query, siteID).Scan(
		&s.ID, &s.SiteID, &s.MeasurementYear, &sampleDepth,
		&s.SiO2, &s.Al2O3, &s.CaO, &s.FeO, &s.Fe2O3, &s.MgO, &s.MnO, &s.P2O5, &s.SO3, &s.K2O, &s.Na2O, &s.TiO2,
		&s.Fayalite, &s.Wollastonite, &s.Anorthite, &s.Diopside, &s.Magnetite, &s.Hematite, &s.Wuestite,
		&s.GlassPhase, &s.OtherMinerals,
		&s.PbLeaching, &s.CdLeaching, &s.AsLeaching, &s.HgLeaching, &s.CrLeaching, &s.NiLeaching,
		&s.Density, &s.SpecificSurface, &s.LossOnIgnition, &remark, &s.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.SampleDepth = sampleDepth.String
	s.Remark = remark.String
	return &s, nil
}

func GetSlagCompositionsBySite(ctx context.Context, siteID int, limit int) ([]models.SlagComposition, error) {
	query := `
		SELECT id, site_id, measurement_year, sample_depth,
			sio2, al2o3, cao, feo, fe2o3, mgo, mno, p2o5, so3, k2o, na2o, tio2,
			fayalite, wollastonite, anorthite, diopside, magnetite, hematite, wuestite,
			glass_phase, other_minerals,
			pb_leaching, cd_leaching, as_leaching, hg_leaching, cr_leaching, ni_leaching,
			density, specific_surface, loss_on_ignition, remark, created_at
		FROM slag_compositions
		WHERE site_id = $1
		ORDER BY measurement_year DESC
		LIMIT $2
	`
	rows, err := database.Pool.Query(ctx, query, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slags []models.SlagComposition
	for rows.Next() {
		var s models.SlagComposition
		var sampleDepth, remark sql.NullString
		err := rows.Scan(
			&s.ID, &s.SiteID, &s.MeasurementYear, &sampleDepth,
			&s.SiO2, &s.Al2O3, &s.CaO, &s.FeO, &s.Fe2O3, &s.MgO, &s.MnO, &s.P2O5, &s.SO3, &s.K2O, &s.Na2O, &s.TiO2,
			&s.Fayalite, &s.Wollastonite, &s.Anorthite, &s.Diopside, &s.Magnetite, &s.Hematite, &s.Wuestite,
			&s.GlassPhase, &s.OtherMinerals,
			&s.PbLeaching, &s.CdLeaching, &s.AsLeaching, &s.HgLeaching, &s.CrLeaching, &s.NiLeaching,
			&s.Density, &s.SpecificSurface, &s.LossOnIgnition, &remark, &s.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		s.SampleDepth = sampleDepth.String
		s.Remark = remark.String
		slags = append(slags, s)
	}
	return slags, nil
}

func GetFarmlandSoilsBySite(ctx context.Context, siteID int, limit int) ([]models.FarmlandSoil, error) {
	query := `
		SELECT id, site_id, measurement_year, distance_from_site, direction, land_use_type,
			pb, zn, cu, as_, hg, cd, cr, ni,
			ph, organic_matter, cec, soil_type, main_crops, created_at
		FROM farmland_soils
		WHERE site_id = $1
		ORDER BY measurement_year DESC, distance_from_site ASC
		LIMIT $2
	`
	rows, err := database.Pool.Query(ctx, query, siteID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var soils []models.FarmlandSoil
	for rows.Next() {
		var f models.FarmlandSoil
		var direction, landUseType, soilType sql.NullString
		var ph, organicMatter, cec sql.NullFloat64
		var mainCrops pq.StringArray
		err := rows.Scan(
			&f.ID, &f.SiteID, &f.MeasurementYear, &f.DistanceFromSite, &direction, &landUseType,
			&f.Pb, &f.Zn, &f.Cu, &f.As, &f.Hg, &f.Cd, &f.Cr, &f.Ni,
			&ph, &organicMatter, &cec, &soilType, &mainCrops, &f.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		f.Direction = direction.String
		f.LandUseType = landUseType.String
		f.SoilType = soilType.String
		if ph.Valid {
			v := ph.Float64
			f.PH = &v
		}
		if organicMatter.Valid {
			v := organicMatter.Float64
			f.OrganicMatter = &v
		}
		if cec.Valid {
			v := cec.Float64
			f.CEC = &v
		}
		f.MainCrops = []string(mainCrops)
		soils = append(soils, f)
	}
	return soils, nil
}

func GetSmeltingInversion(ctx context.Context, siteID int, year int) (*models.SmeltingProcessInversion, error) {
	query := `
		SELECT id, site_id, measurement_year,
			estimated_temperature, temperature_confidence,
			reducing_agent, reducing_agent_confidence,
			bpnn_posterior, bayes_posterior,
			process_type_detailed, process_era_estimate,
			input_features, bpnn_mse, bayes_kld,
			quality_level, remark, created_at
		FROM smelting_process_inversions
		WHERE site_id = $1 AND measurement_year = $2
	`
	var inv models.SmeltingProcessInversion
	var reducingAgent, processTypeDetailed, processEraEstimate, qualityLevel, remark sql.NullString
	var bpnnPosteriorJSON, bayesPosteriorJSON, inputFeaturesJSON []byte
	err := database.Pool.QueryRow(ctx, query, siteID, year).Scan(
		&inv.ID, &inv.SiteID, &inv.MeasurementYear,
		&inv.EstimatedTemperature, &inv.TemperatureConfidence,
		&reducingAgent, &inv.ReducingAgentConfidence,
		&bpnnPosteriorJSON, &bayesPosteriorJSON,
		&processTypeDetailed, &processEraEstimate,
		&inputFeaturesJSON, &inv.BPNNMSE, &inv.BayesKLD,
		&qualityLevel, &remark, &inv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	inv.ReducingAgent = reducingAgent.String
	inv.ProcessTypeDetailed = processTypeDetailed.String
	inv.ProcessEraEstimate = processEraEstimate.String
	inv.QualityLevel = qualityLevel.String
	inv.Remark = remark.String
	if len(bpnnPosteriorJSON) > 0 {
		json.Unmarshal(bpnnPosteriorJSON, &inv.BPNNPosterior)
	}
	if len(bayesPosteriorJSON) > 0 {
		json.Unmarshal(bayesPosteriorJSON, &inv.BayesPosterior)
	}
	if len(inputFeaturesJSON) > 0 {
		json.Unmarshal(inputFeaturesJSON, &inv.InputFeatures)
	}
	return &inv, nil
}

func SaveSmeltingInversion(ctx context.Context, inv *models.SmeltingProcessInversion) (int, error) {
	bpnnPosteriorJSON, _ := json.Marshal(inv.BPNNPosterior)
	bayesPosteriorJSON, _ := json.Marshal(inv.BayesPosterior)
	inputFeaturesJSON, _ := json.Marshal(inv.InputFeatures)

	query := `
		INSERT INTO smelting_process_inversions
		(site_id, measurement_year,
		 estimated_temperature, temperature_confidence,
		 reducing_agent, reducing_agent_confidence,
		 bpnn_posterior, bayes_posterior,
		 process_type_detailed, process_era_estimate,
		 input_features, bpnn_mse, bayes_kld,
		 quality_level, remark)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (site_id, measurement_year) DO UPDATE SET
			estimated_temperature = EXCLUDED.estimated_temperature,
			temperature_confidence = EXCLUDED.temperature_confidence,
			reducing_agent = EXCLUDED.reducing_agent,
			reducing_agent_confidence = EXCLUDED.reducing_agent_confidence,
			bpnn_posterior = EXCLUDED.bpnn_posterior,
			bayes_posterior = EXCLUDED.bayes_posterior,
			process_type_detailed = EXCLUDED.process_type_detailed,
			process_era_estimate = EXCLUDED.process_era_estimate,
			input_features = EXCLUDED.input_features,
			bpnn_mse = EXCLUDED.bpnn_mse,
			bayes_kld = EXCLUDED.bayes_kld,
			quality_level = EXCLUDED.quality_level,
			remark = EXCLUDED.remark,
			created_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	var id int
	err := database.Pool.QueryRow(ctx, query,
		inv.SiteID, inv.MeasurementYear,
		inv.EstimatedTemperature, inv.TemperatureConfidence,
		inv.ReducingAgent, inv.ReducingAgentConfidence,
		bpnnPosteriorJSON, bayesPosteriorJSON,
		inv.ProcessTypeDetailed, inv.ProcessEraEstimate,
		inputFeaturesJSON, inv.BPNNMSE, inv.BayesKLD,
		inv.QualityLevel, inv.Remark,
	).Scan(&id)
	return id, err
}

func GetResourceAssessment(ctx context.Context, siteID int, year int) (*models.ResourceUtilizationAssessment, error) {
	query := `
		SELECT id, site_id, measurement_year,
			cement_blended_feasibility, cement_blended_score, cement_blended_grade, cement_details,
			road_base_feasibility, road_base_score, road_base_grade, road_details,
			other_uses,
			leaching_risk_level, leaching_risk_details,
			recommended_use, utilization_plan, created_at
		FROM resource_utilization_assessments
		WHERE site_id = $1 AND measurement_year = $2
	`
	var a models.ResourceUtilizationAssessment
	var cementFeasibility, cementGrade, roadFeasibility, roadGrade sql.NullString
	var leachingRiskLevel, recommendedUse sql.NullString
	var cementDetailsJSON, roadDetailsJSON, otherUsesJSON, leachingRiskDetailsJSON, utilizationPlanJSON []byte
	err := database.Pool.QueryRow(ctx, query, siteID, year).Scan(
		&a.ID, &a.SiteID, &a.MeasurementYear,
		&cementFeasibility, &a.CementBlendedScore, &cementGrade, &cementDetailsJSON,
		&roadFeasibility, &a.RoadBaseScore, &roadGrade, &roadDetailsJSON,
		&otherUsesJSON,
		&leachingRiskLevel, &leachingRiskDetailsJSON,
		&recommendedUse, &utilizationPlanJSON, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.CementBlendedFeasibility = cementFeasibility.String
	a.CementBlendedGrade = cementGrade.String
	a.RoadBaseFeasibility = roadFeasibility.String
	a.RoadBaseGrade = roadGrade.String
	a.LeachingRiskLevel = leachingRiskLevel.String
	a.RecommendedUse = recommendedUse.String
	if len(cementDetailsJSON) > 0 {
		json.Unmarshal(cementDetailsJSON, &a.CementDetails)
	}
	if len(roadDetailsJSON) > 0 {
		json.Unmarshal(roadDetailsJSON, &a.RoadDetails)
	}
	if len(otherUsesJSON) > 0 {
		json.Unmarshal(otherUsesJSON, &a.OtherUses)
	}
	if len(leachingRiskDetailsJSON) > 0 {
		json.Unmarshal(leachingRiskDetailsJSON, &a.LeachingRiskDetails)
	}
	if len(utilizationPlanJSON) > 0 {
		json.Unmarshal(utilizationPlanJSON, &a.UtilizationPlan)
	}
	return &a, nil
}

func SaveResourceAssessment(ctx context.Context, a *models.ResourceUtilizationAssessment) (int, error) {
	cementDetailsJSON, _ := json.Marshal(a.CementDetails)
	roadDetailsJSON, _ := json.Marshal(a.RoadDetails)
	otherUsesJSON, _ := json.Marshal(a.OtherUses)
	leachingRiskDetailsJSON, _ := json.Marshal(a.LeachingRiskDetails)
	utilizationPlanJSON, _ := json.Marshal(a.UtilizationPlan)

	query := `
		INSERT INTO resource_utilization_assessments
		(site_id, measurement_year,
		 cement_blended_feasibility, cement_blended_score, cement_blended_grade, cement_details,
		 road_base_feasibility, road_base_score, road_base_grade, road_details,
		 other_uses,
		 leaching_risk_level, leaching_risk_details,
		 recommended_use, utilization_plan)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (site_id, measurement_year) DO UPDATE SET
			cement_blended_feasibility = EXCLUDED.cement_blended_feasibility,
			cement_blended_score = EXCLUDED.cement_blended_score,
			cement_blended_grade = EXCLUDED.cement_blended_grade,
			cement_details = EXCLUDED.cement_details,
			road_base_feasibility = EXCLUDED.road_base_feasibility,
			road_base_score = EXCLUDED.road_base_score,
			road_base_grade = EXCLUDED.road_base_grade,
			road_details = EXCLUDED.road_details,
			other_uses = EXCLUDED.other_uses,
			leaching_risk_level = EXCLUDED.leaching_risk_level,
			leaching_risk_details = EXCLUDED.leaching_risk_details,
			recommended_use = EXCLUDED.recommended_use,
			utilization_plan = EXCLUDED.utilization_plan,
			created_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	var id int
	err := database.Pool.QueryRow(ctx, query,
		a.SiteID, a.MeasurementYear,
		a.CementBlendedFeasibility, a.CementBlendedScore, a.CementBlendedGrade, cementDetailsJSON,
		a.RoadBaseFeasibility, a.RoadBaseScore, a.RoadBaseGrade, roadDetailsJSON,
		otherUsesJSON,
		a.LeachingRiskLevel, leachingRiskDetailsJSON,
		a.RecommendedUse, utilizationPlanJSON,
	).Scan(&id)
	return id, err
}

func GetAllTrendData(ctx context.Context) (map[int][]models.TrendData, error) {
	query := `
		SELECT site_id, measurement_year,
			pb, zn, cu, as_val, hg, cd,
			ph, organic_matter, cation_exchange_capacity
		FROM xrf_measurements
		ORDER BY site_id, measurement_year
	`
	rows, err := database.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]models.TrendData)
	for rows.Next() {
		var t models.TrendData
		var siteID int
		var ph, organicMatter, cec sql.NullFloat64
		err := rows.Scan(
			&siteID, &t.Year,
			&t.Pb, &t.Zn, &t.Cu, &t.As, &t.Hg, &t.Cd,
			&ph, &organicMatter, &cec,
		)
		if err != nil {
			return nil, err
		}
		t.PH = ph.Float64
		t.OrganicMatter = organicMatter.Float64
		t.CEC = cec.Float64
		t.PollutionIndex = CalculatePollutionIndex(t.Pb, t.Zn, t.Cu, t.As, t.Hg, t.Cd)
		result[siteID] = append(result[siteID], t)
	}
	return result, nil
}

func UpsertSlagComposition(ctx context.Context, s *models.SlagComposition) error {
	query := `
		INSERT INTO slag_compositions
		(site_id, measurement_year, sample_depth,
		 sio2, al2o3, cao, feo, fe2o3, mgo, mno, p2o5, so3, k2o, na2o, tio2,
		 fayalite, wollastonite, anorthite, diopside, magnetite, hematite, wuestite,
		 glass_phase, other_minerals,
		 pb_leaching, cd_leaching, as_leaching, hg_leaching, cr_leaching, ni_leaching,
		 density, specific_surface, loss_on_ignition, remark)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
		 $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30,
		 $31, $32, $33, $34, $35)
		ON CONFLICT (site_id, measurement_year, sample_depth) DO UPDATE SET
			sio2 = EXCLUDED.sio2, al2o3 = EXCLUDED.al2o3, cao = EXCLUDED.cao,
			feo = EXCLUDED.feo, fe2o3 = EXCLUDED.fe2o3, mgo = EXCLUDED.mgo,
			mno = EXCLUDED.mno, p2o5 = EXCLUDED.p2o5, so3 = EXCLUDED.so3,
			k2o = EXCLUDED.k2o, na2o = EXCLUDED.na2o, tio2 = EXCLUDED.tio2,
			fayalite = EXCLUDED.fayalite, wollastonite = EXCLUDED.wollastonite,
			anorthite = EXCLUDED.anorthite, diopside = EXCLUDED.diopside,
			magnetite = EXCLUDED.magnetite, hematite = EXCLUDED.hematite,
			wuestite = EXCLUDED.wuestite, glass_phase = EXCLUDED.glass_phase,
			other_minerals = EXCLUDED.other_minerals,
			pb_leaching = EXCLUDED.pb_leaching, cd_leaching = EXCLUDED.cd_leaching,
			as_leaching = EXCLUDED.as_leaching, hg_leaching = EXCLUDED.hg_leaching,
			cr_leaching = EXCLUDED.cr_leaching, ni_leaching = EXCLUDED.ni_leaching,
			density = EXCLUDED.density, specific_surface = EXCLUDED.specific_surface,
			loss_on_ignition = EXCLUDED.loss_on_ignition, remark = EXCLUDED.remark,
			created_at = CURRENT_TIMESTAMP
		RETURNING id, created_at
	`
	return database.Pool.QueryRow(ctx, query,
		s.SiteID, s.MeasurementYear, s.SampleDepth,
		s.SiO2, s.Al2O3, s.CaO, s.FeO, s.Fe2O3, s.MgO, s.MnO, s.P2O5, s.SO3, s.K2O, s.Na2O, s.TiO2,
		s.Fayalite, s.Wollastonite, s.Anorthite, s.Diopside, s.Magnetite, s.Hematite, s.Wuestite,
		s.GlassPhase, s.OtherMinerals,
		s.PbLeaching, s.CdLeaching, s.AsLeaching, s.HgLeaching, s.CrLeaching, s.NiLeaching,
		s.Density, s.SpecificSurface, s.LossOnIgnition, s.Remark,
	).Scan(&s.ID, &s.CreatedAt)
}

func UpsertFarmlandSoil(ctx context.Context, f *models.FarmlandSoil) error {
	var ph, organicMatter, cec sql.NullFloat64
	if f.PH != nil {
		ph = sql.NullFloat64{Float64: *f.PH, Valid: true}
	}
	if f.OrganicMatter != nil {
		organicMatter = sql.NullFloat64{Float64: *f.OrganicMatter, Valid: true}
	}
	if f.CEC != nil {
		cec = sql.NullFloat64{Float64: *f.CEC, Valid: true}
	}
	mainCrops := pq.StringArray(f.MainCrops)

	query := `
		INSERT INTO farmland_soils
		(site_id, measurement_year, distance_from_site, direction, land_use_type,
		 pb, zn, cu, as_, hg, cd, cr, ni,
		 ph, organic_matter, cec, soil_type, main_crops)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		 $14, $15, $16, $17, $18)
		ON CONFLICT (site_id, measurement_year, distance_from_site, direction) DO UPDATE SET
			land_use_type = EXCLUDED.land_use_type,
			pb = EXCLUDED.pb, zn = EXCLUDED.zn, cu = EXCLUDED.cu,
			as_ = EXCLUDED.as_, hg = EXCLUDED.hg, cd = EXCLUDED.cd,
			cr = EXCLUDED.cr, ni = EXCLUDED.ni,
			ph = EXCLUDED.ph, organic_matter = EXCLUDED.organic_matter,
			cec = EXCLUDED.cec, soil_type = EXCLUDED.soil_type,
			main_crops = EXCLUDED.main_crops,
			created_at = CURRENT_TIMESTAMP
		RETURNING id, created_at
	`
	return database.Pool.QueryRow(ctx, query,
		f.SiteID, f.MeasurementYear, f.DistanceFromSite, f.Direction, f.LandUseType,
		f.Pb, f.Zn, f.Cu, f.As, f.Hg, f.Cd, f.Cr, f.Ni,
		ph, organicMatter, cec, f.SoilType, mainCrops,
	).Scan(&f.ID, &f.CreatedAt)
}
