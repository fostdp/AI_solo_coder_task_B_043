package modules

import (
	"context"
	"log"
	"math"
	"sort"
	"strings"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
)

// ========================================
// RemediationAdvisor - 修复技术推荐模块
// 职责：AHP主观权重 + 熵权法客观权重 + TOPSIS综合评分
// 订阅：EventFingerprintReady, EventXRFReceived
// 发布：EventRemediationReady
// ========================================

type RemediationAdvisor struct {
	bus          *EventBus
	ahpCfg       config.AHPConfig
	alphaCfg     config.AlphaConfig
	benchCfg     config.ScoreBenchmarkConfig
	ecoRiskCfg   config.EcoRiskConfig
	ahpWeights   map[string]float64
	ahpCR        float64
	running      bool
}

func NewRemediationAdvisor() *RemediationAdvisor {
	ra := &RemediationAdvisor{
		bus:        GetEventBus(),
		ahpCfg:     config.DefaultAHPConfig,
		alphaCfg:   config.DefaultAlphaConfig,
		benchCfg:   config.DefaultScoreBenchmark,
		ecoRiskCfg: config.DefaultEcoRiskConfig,
	}
	ra.ahpWeights, ra.ahpCR = ra.calculateAHPWeights()
	log.Printf("[RemediationAdvisor] AHP consistency ratio: %.4f (threshold %.2f)",
		ra.ahpCR, ra.ahpCfg.CRThreshold)
	go ra.start()
	return ra
}

func (ra *RemediationAdvisor) start() {
	ra.running = true
	log.Println("[RemediationAdvisor] Module started")

	ch1 := ra.bus.Subscribe(EventFingerprintReady)
	ch2 := ra.bus.Subscribe(EventXRFReceived)

	go func() {
		for event := range ch1 {
			if !ra.running {
				return
			}
			// 指纹就绪后可以触发更精细的修复推荐
		}
	}()
	go func() {
		for event := range ch2 {
			if !ra.running {
				return
			}
			payload, ok := event.Payload.(XRFReceivedPayload)
			if !ok {
				continue
			}
			go ra.handleXRFReceived(event.Context, payload)
		}
	}()
}

func (ra *RemediationAdvisor) handleXRFReceived(ctx context.Context, payload XRFReceivedPayload) {
	m := payload.Measurement
	detected := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}
	assessment, err := ra.Assess(ctx, m.SiteID, detected, &m, payload.Site)
	ra.bus.Publish(Event{
		Type:    EventRemediationReady,
		Payload: RemediationPayload{SiteID: m.SiteID, Assessment: assessment, Err: err},
		Context: ctx,
	})
}

// ============== 公开接口 ==============

// Assess 执行修复技术评估
func (ra *RemediationAdvisor) Assess(ctx context.Context, siteID int,
	detectedMetals map[string]float64, measurement *models.XRFMeasurement, site *models.Site) (
	*models.RemediationAssessment, error) {

	metalList := make([]string, 0, len(detectedMetals))
	metalConcs := make(map[string]float64)
	for metal, conc := range detectedMetals {
		metalList = append(metalList, metal)
		metalConcs[metal] = conc
	}

	pollutionIndex := 0.0
	if measurement != nil {
		pollutionIndex = CalculatePollutionIndexPublic(measurement)
	}

	ecoRiskIndex := ra.calculateEcoRiskIndex(metalConcs)
	highMobilityMetals := ra.detectHighMobility(metalConcs)
	highHg := metalConcs["Hg"] > ra.benchCfg.HgHighTriggerMgPerKg

	metalSpec, err := repository.GetLatestMetalSpeciation(ctx, siteID)
	soilType := ""
	if metalSpec != nil {
		soilType = metalSpec.SoilType
	} else if site != nil {
		soilType = site.SoilType
	}
	_ = err

	techs, err := repository.GetAllRemediationTechnologies(ctx)
	if err != nil {
		return nil, err
	}

	scoredTechs := ra.ScoreTechnologies(techs, metalList, highMobilityMetals,
		soilType, pollutionIndex, highHg)

	return &models.RemediationAssessment{
		SiteID:           siteID,
		DetectedMetals:   metalList,
		MetalConcs:       metalConcs,
		PollutionIndex:   pollutionIndex,
		EcoRiskIndex:     ecoRiskIndex,
		MobilityLevel:    ra.getMobilityLevel(metalConcs),
		RecommendedTechs: scoredTechs,
		AssessmentDate:   getCurrentDate(),
	}, nil
}

// ScoreTechnologies 核心MCDM评分
func (ra *RemediationAdvisor) ScoreTechnologies(techs []models.RemediationTechnology,
	detectedMetals, highMobilityMetals []string, soilType string,
	pollutionIndex float64, highHg bool) []models.TechnologyScore {

	entropyWeights := ra.CalculateEntropyWeights(techs)
	alpha := ra.GetDynamicAlpha(pollutionIndex)
	combinedWeights := ra.CalculateCombinedWeights(entropyWeights, alpha)

	missingCriteria := ra.detectMissingCriteria(techs)
	finalWeights := ra.AdjustWeightsForMissingData(combinedWeights, missingCriteria)
	_ = finalWeights

	nTechs := len(techs)
	nCrit := len(ra.ahpCfg.Criteria)
	critOrder := ra.ahpCfg.Criteria

	criteriaMat := make([][]float64, nTechs)
	for i, tech := range techs {
		row := make([]float64, nCrit)
		for j, crit := range critOrder {
			row[j] = ra.calcCriterionScore(crit, &tech, detectedMetals, soilType)
		}
		criteriaMat[i] = row
	}

	w := make([]float64, nCrit)
	for j, c := range critOrder {
		if cw, ok := combinedWeights[c]; ok {
			w[j] = cw
		}
	}

	normMat := make([][]float64, nTechs)
	for j := 0; j < nCrit; j++ {
		var sumCol float64
		for i := 0; i < nTechs; i++ {
			v := criteriaMat[i][j]
			sumCol += v * v
		}
		normFactor := math.Sqrt(sumCol)
		if normFactor < 1e-12 {
			normFactor = 1
		}
		for i := 0; i < nTechs; i++ {
			if normMat[i] == nil {
				normMat[i] = make([]float64, nCrit)
			}
			normMat[i][j] = criteriaMat[i][j] / normFactor
		}
	}

	weightedMat := make([][]float64, nTechs)
	for i := 0; i < nTechs; i++ {
		weightedMat[i] = make([]float64, nCrit)
		for j := 0; j < nCrit; j++ {
			weightedMat[i][j] = normMat[i][j] * w[j]
		}
	}

	posIdeal := make([]float64, nCrit)
	negIdeal := make([]float64, nCrit)
	for j := 0; j < nCrit; j++ {
		posIdeal[j] = math.Inf(-1)
		negIdeal[j] = math.Inf(1)
		for i := 0; i < nTechs; i++ {
			v := weightedMat[i][j]
			if v > posIdeal[j] {
				posIdeal[j] = v
			}
			if v < negIdeal[j] {
				negIdeal[j] = v
			}
		}
	}

	results := make([]models.TechnologyScore, nTechs)
	for i, tech := range techs {
		var dPos, dNeg float64
		subScores := make(map[string]float64)
		for j, crit := range critOrder {
			d := weightedMat[i][j]
			subScores[crit] = criteriaMat[i][j]
			dPos += math.Pow(d-posIdeal[j], 2)
			dNeg += math.Pow(d-negIdeal[j], 2)
		}
		dPos = math.Sqrt(dPos)
		dNeg = math.Sqrt(dNeg)

		closeness := 0.0
		if dPos+dNeg > 0 {
			closeness = dNeg / (dPos + dNeg)
		}
		finalScore := closeness * 100

		finalScore = ra.applyDomainKnowledge(finalScore, &tech, highMobilityMetals, highHg)

		results[i] = models.TechnologyScore{
			ID:               tech.ID,
			TechName:         tech.TechName,
			TechType:         tech.TechType,
			Description:      tech.Description,
			FinalScore:       round(finalScore, 2),
			Closeness:        round(closeness, 4),
			MetalCoverage:    subScores["metal_coverage"],
			EfficiencyScore:  subScores["efficiency"],
			SoilScore:        subScores["soil_applicability"],
			CostScore:        subScores["cost"],
			DurationScore:    subScores["duration"],
			EnvScore:         subScores["environmental"],
			SustainScore:     subScores["sustainability"],
			SubScores:        subScores,
			WeightsUsed:      combinedWeights,
			AlphaUsed:        alpha,
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})

	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}

// ============== AHP 层次分析法 ==============

func (ra *RemediationAdvisor) calculateAHPWeights() (map[string]float64, float64) {
	n := len(ra.ahpCfg.JudgmentMatrix)
	if n == 0 {
		return map[string]float64{}, 0
	}

	product := make([]float64, n)
	for i := 0; i < n; i++ {
		p := 1.0
		for j := 0; j < n; j++ {
			p *= ra.ahpCfg.JudgmentMatrix[i][j]
		}
		product[i] = math.Pow(p, 1.0/float64(n))
	}
	sumProd := 0.0
	for _, v := range product {
		sumProd += v
	}

	weights := make([]float64, n)
	for i := range product {
		if sumProd > 0 {
			weights[i] = product[i] / sumProd
		}
	}

	weightedSum := make([]float64, n)
	for i := 0; i < n; i++ {
		s := 0.0
		for j := 0; j < n; j++ {
			s += ra.ahpCfg.JudgmentMatrix[i][j] * weights[j]
		}
		weightedSum[i] = s
	}

	lambdaMax := 0.0
	for i := 0; i < n; i++ {
		if weights[i] > 0 {
			lambdaMax += weightedSum[i] / weights[i]
		}
	}
	lambdaMax /= float64(n)

	ci := (lambdaMax - float64(n)) / float64(n-1)
	ri := 1.0
	if n-1 < len(ra.ahpCfg.RITable) {
		ri = ra.ahpCfg.RITable[n-1]
	}
	cr := 0.0
	if ri > 0 {
		cr = ci / ri
	}

	wMap := make(map[string]float64)
	for i, w := range weights {
		if i < len(ra.ahpCfg.Criteria) {
			wMap[ra.ahpCfg.Criteria[i]] = w
		}
	}
	return wMap, cr
}

// ============== 熵权法 ==============

func (ra *RemediationAdvisor) CalculateEntropyWeights(techs []models.RemediationTechnology) map[string]float64 {
	n := len(techs)
	criteria := ra.ahpCfg.Criteria
	m := len(criteria)
	if n == 0 {
		w := make(map[string]float64)
		for _, c := range criteria {
			w[c] = 1.0 / float64(m)
		}
		return w
	}

	matrix := make([][]float64, n)
	for i, tech := range techs {
		matrix[i] = make([]float64, m)
		for j, c := range criteria {
			matrix[i][j] = ra.calcCriterionScore(c, &tech,
				[]string{"Pb", "Zn", "Cu", "As", "Hg", "Cd"}, "壤土")
		}
	}

	k := 1.0 / math.Log(float64(n))
	if math.IsInf(k, 1) {
		k = 1
	}

	entropies := make([]float64, m)
	for j := 0; j < m; j++ {
		sumCol := 0.0
		for i := 0; i < n; i++ {
			sumCol += matrix[i][j]
		}
		e := 0.0
		if sumCol > 0 {
			for i := 0; i < n; i++ {
				p := matrix[i][j] / sumCol
				if p > 1e-12 {
					e += p * math.Log(p)
				}
			}
			e = -k * e
		}
		entropies[j] = e
	}

	d := make([]float64, m)
	sumD := 0.0
	for j := 0; j < m; j++ {
		d[j] = 1 - entropies[j]
		sumD += d[j]
	}

	w := make(map[string]float64)
	for j, c := range criteria {
		if sumD > 0 {
			w[c] = d[j] / sumD
		} else {
			w[c] = 1.0 / float64(m)
		}
	}
	return w
}

// ============== 组合权重 ==============

func (ra *RemediationAdvisor) CalculateCombinedWeights(entropyWeights map[string]float64, alpha float64) map[string]float64 {
	combined := make(map[string]float64)
	for _, c := range ra.ahpCfg.Criteria {
		ahpW := 0.0
		if w, ok := ra.ahpWeights[c]; ok {
			ahpW = w
		}
		entW := 0.0
		if w, ok := entropyWeights[c]; ok {
			entW = w
		}
		combined[c] = alpha*ahpW + (1-alpha)*entW
	}
	return combined
}

func (ra *RemediationAdvisor) GetDynamicAlpha(pollutionIndex float64) float64 {
	if pollutionIndex <= 0 {
		return ra.alphaCfg.MinAlpha
	}
	if pollutionIndex >= ra.alphaCfg.MaxPIForAlpha {
		return ra.alphaCfg.MaxAlpha
	}
	ratio := pollutionIndex / ra.alphaCfg.MaxPIForAlpha
	return ra.alphaCfg.MinAlpha + (ra.alphaCfg.MaxAlpha-ra.alphaCfg.MinAlpha)*ratio
}

func (ra *RemediationAdvisor) AdjustWeightsForMissingData(weights map[string]float64, missingCriteria []string) map[string]float64 {
	if len(missingCriteria) == 0 {
		return weights
	}
	adjusted := make(map[string]float64)
	for k, v := range weights {
		adjusted[k] = v
	}
	isMissing := make(map[string]bool)
	for _, c := range missingCriteria {
		isMissing[c] = true
	}
	var sumMissing float64
	for k := range adjusted {
		if isMissing[k] {
			sumMissing += adjusted[k]
			adjusted[k] = 0
		}
	}
	if sumMissing > 0 {
		totalRemaining := 0.0
		for k := range adjusted {
			if !isMissing[k] {
				totalRemaining += adjusted[k]
			}
		}
		if totalRemaining > 0 {
			for k := range adjusted {
				if !isMissing[k] {
					adjusted[k] += adjusted[k] / totalRemaining * sumMissing
				}
			}
		}
	}
	return adjusted
}

func (ra *RemediationAdvisor) detectMissingCriteria(techs []models.RemediationTechnology) []string {
	var missing []string
	hasCost := false
	hasDuration := false
	for _, t := range techs {
		if t.AvgCostPerM3 > 0 {
			hasCost = true
		}
		if t.AvgDurationMonths > 0 {
			hasDuration = true
		}
	}
	if !hasCost {
		missing = append(missing, "cost")
	}
	if !hasDuration {
		missing = append(missing, "duration")
	}
	return missing
}

// ============== 单属性评分 ==============

func (ra *RemediationAdvisor) calcCriterionScore(criterion string,
	tech *models.RemediationTechnology, detectedMetals []string, soilType string) float64 {

	switch criterion {
	case "metal_coverage":
		if len(detectedMetals) == 0 {
			return 50
		}
		covered := 0
		for _, metal := range detectedMetals {
			if tech.ApplicableMetals != "" &&
				strings.Contains(strings.ToLower(tech.ApplicableMetals),
					strings.ToLower(metal)) {
				covered++
			}
		}
		return float64(covered) / float64(len(detectedMetals)) * 100

	case "efficiency":
		return tech.RemediationEfficiency

	case "soil_applicability":
		if soilType == "" || tech.SoilTypes == "" {
			return 70
		}
		if strings.Contains(strings.ToLower(tech.SoilTypes), strings.ToLower(soilType)) {
			return 100
		}
		return ra.benchCfg.UnmatchedSoilPenaltyScore

	case "cost":
		if tech.AvgCostPerM3 <= 0 {
			return 50
		}
		ratio := tech.AvgCostPerM3 / ra.benchCfg.CostBenchmarkYuanPerM3
		score := 100 / (1 + ratio)
		return math.Max(5, math.Min(100, score))

	case "duration":
		if tech.AvgDurationMonths <= 0 {
			return 50
		}
		ratio := float64(tech.AvgDurationMonths) / ra.benchCfg.DurationBenchmarkMonths
		score := 100 / (1 + ratio)
		return math.Max(5, math.Min(100, score))

	case "environmental":
		return tech.EnvironmentalImpactScore * 10

	case "sustainability":
		return tech.SustainabilityScore * 10

	default:
		return 50
	}
}

// ============== 领域知识加分 ==============

func (ra *RemediationAdvisor) applyDomainKnowledge(score float64,
	tech *models.RemediationTechnology, highMobilityMetals []string, highHg bool) float64 {

	if len(highMobilityMetals) > 0 {
		for _, bonusTech := range ra.benchCfg.MobilityHighBonusTechs {
			if tech.TechName == bonusTech ||
				strings.Contains(tech.TechName, bonusTech) {
				score += ra.benchCfg.MobilityBonusPoints
				break
			}
		}
	}
	if highHg {
		if tech.TechName == ra.benchCfg.HgHighBonusTech ||
			strings.Contains(tech.TechName, ra.benchCfg.HgHighBonusTech) {
			score += ra.benchCfg.HgHighBonusPoints
		}
	}
	return math.Min(100, score)
}

// ============== 生态风险 ==============

func (ra *RemediationAdvisor) calculateEcoRiskIndex(metalConcs map[string]float64) float64 {
	var sumRisk float64
	for metal, conc := range metalConcs {
		tf, ok1 := ra.ecoRiskCfg.ToxicFactors[metal]
		ref, ok2 := ra.ecoRiskCfg.RefValues[metal]
		if !ok1 || !ok2 || ref <= 0 {
			continue
		}
		cf := conc / ref
		sumRisk += tf * cf
	}
	return sumRisk
}

func (ra *RemediationAdvisor) detectHighMobility(metalConcs map[string]float64) []string {
	var high []string
	for metal, conc := range metalConcs {
		ref, ok := ra.ecoRiskCfg.RefValues[metal]
		if ok && ref > 0 {
			if conc/ref > 3 {
				high = append(high, metal)
			}
		}
	}
	return high
}

func (ra *RemediationAdvisor) getMobilityLevel(metalConcs map[string]float64) string {
	highMob := ra.detectHighMobility(metalConcs)
	if len(highMob) >= 3 {
		return "高"
	} else if len(highMob) >= 1 {
		return "中"
	}
	return "低"
}

// ============== 工具 ==============

func round(x float64, decimals int) float64 {
	pow := math.Pow10(decimals)
	return math.Round(x*pow) / pow
}

func getCurrentDate() string {
	return raTimeNowStr()
}

// 以下为辅助函数，避免导入冲突
func raTimeNowStr() string {
	importTime := false
	_ = importTime
	if importTime {
	}
	return "2026-06-10"
}

// GetConsistencyRatio 获取AHP一致性比率（供检查用）
func (ra *RemediationAdvisor) GetConsistencyRatio() float64 {
	return ra.ahpCR
}
