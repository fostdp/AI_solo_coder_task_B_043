package services

import (
	"math"
	"sort"

	"archaeology-pollution-system/models"
)

// ========================================
// 多属性决策：熵权法 + AHP层次分析法 + TOPSIS + 组合权重
// ========================================

type MCDMService struct {
	ahpWeights map[string]float64
}

func NewMCDMService() *MCDMService {
	s := &MCDMService{}
	s.calculateAHPWeights()
	return s
}

// AHP层次分析法专家打分矩阵
// 7个属性：metal_coverage, efficiency, soil_applicability, cost, duration, environmental, sustainability
// Saaty 1-9标度法
var ahpMatrix = [][]float64{
	//     金属覆盖  效率    土壤   成本    周期   环境   可持续
	{1.0,   2.0,   3.0,   3.0,   4.0,   5.0,   5.0},   // 金属覆盖度
	{0.5,   1.0,   2.0,   2.0,   3.0,   4.0,   4.0},   // 修复效率
	{1.0/3, 0.5,   1.0,   1.0,   2.0,   3.0,   3.0},   // 土壤适用性
	{1.0/3, 0.5,   1.0,   1.0,   2.0,   2.0,   3.0},   // 成本经济性
	{0.25,  1.0/3, 0.5,   0.5,   1.0,   2.0,   2.0},   // 修复周期
	{0.2,   0.25,  1.0/3, 1.0/3, 0.5,   1.0,   2.0},   // 环境影响
	{0.2,   0.25,  1.0/3, 1.0/3, 0.5,   0.5,   1.0},   // 可持续性
}

var ahpCriteria = []string{
	"metal_coverage", "efficiency", "soil_applicability",
	"cost", "duration", "environmental", "sustainability",
}

// 计算AHP权重（几何平均法）
func (s *MCDMService) calculateAHPWeights() {
	n := len(ahpMatrix)
	weightVector := make([]float64, n)

	for i := 0; i < n; i++ {
		prod := 1.0
		for j := 0; j < n; j++ {
			prod *= ahpMatrix[i][j]
		}
		weightVector[i] = math.Pow(prod, 1.0/float64(n))
	}

	sum := 0.0
	for _, w := range weightVector {
		sum += w
	}
	for i := range weightVector {
		weightVector[i] /= sum
	}

	s.ahpWeights = make(map[string]float64)
	for i, c := range ahpCriteria {
		s.ahpWeights[c] = weightVector[i]
	}
}

// 获取AHP一致性比率CR（CR<0.1则判断矩阵一致可接受）
func (s *MCDMService) GetConsistencyRatio() float64 {
	n := len(ahpMatrix)
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = s.ahpWeights[ahpCriteria[i]]
	}

	lambdaMax := 0.0
	for i := 0; i < n; i++ {
		rowSum := 0.0
		for j := 0; j < n; j++ {
			rowSum += ahpMatrix[i][j] * weights[j]
		}
		lambdaMax += rowSum / weights[i]
	}
	lambdaMax /= float64(n)

	riTable := []float64{0, 0, 0.58, 0.90, 1.12, 1.24, 1.32, 1.41, 1.45, 1.49}
	ci := (lambdaMax - float64(n)) / float64(n-1)
	if n-1 >= len(riTable) || n-1 < 0 {
		return 1.0
	}
	ri := riTable[n-1]
	if ri == 0 {
		return 0
	}
	return ci / ri
}

// 获取AHP主观权重
func (s *MCDMService) GetAHPWeights() map[string]float64 {
	w := make(map[string]float64)
	for k, v := range s.ahpWeights {
		w[k] = v
	}
	return w
}

// 熵权法：基于数据离散程度自动计算客观权重
func (s *MCDMService) CalculateEntropyWeights(
	techs []models.RemediationTechnology,
	detectedMetals []string,
	soilType string,
	pollutionIndex float64,
) map[string]float64 {
	n := len(techs)
	criteria := ahpCriteria
	m := len(criteria)

	if n <= 1 {
		// 数据太少，返回等权重
		w := make(map[string]float64)
		for _, c := range criteria {
			w[c] = 1.0 / float64(m)
		}
		return w
	}

	D := make([][]float64, n)
	for i := range D {
		D[i] = make([]float64, m)
	}

	for i, t := range techs {
		scores := s.calculateRawScores(t, detectedMetals, soilType, pollutionIndex)
		for j, c := range criteria {
			D[i][j] = scores[c]
		}
	}

	// 按列归一化
	for j := 0; j < m; j++ {
		colSum := 0.0
		for i := 0; i < n; i++ {
			colSum += D[i][j]
		}
		if colSum > 0 {
			for i := 0; i < n; i++ {
				D[i][j] = D[i][j] / colSum
			}
		}
	}

	// 计算熵值
	k := 1.0 / math.Log(float64(n))
	entropy := make([]float64, m)
	for j := 0; j < m; j++ {
		e := 0.0
		for i := 0; i < n; i++ {
			if D[i][j] > 1e-10 {
				e -= D[i][j] * math.Log(D[i][j])
			}
		}
		entropy[j] = k * e
	}

	// 计算差异系数和权重
	d := make([]float64, m)
	totalD := 0.0
	for j := 0; j < m; j++ {
		d[j] = 1.0 - entropy[j]
		totalD += d[j]
	}

	weights := make(map[string]float64)
	for j, c := range criteria {
		if totalD > 1e-10 {
			weights[c] = d[j] / totalD
		} else {
			weights[c] = 1.0 / float64(m)
		}
	}

	return weights
}

// 计算各属性的原始得分 (0-100)
func (s *MCDMService) calculateRawScores(
	t models.RemediationTechnology,
	detectedMetals []string,
	soilType string,
	pollutionIndex float64,
) map[string]float64 {
	scores := make(map[string]float64)

	// 1. 金属覆盖度 (0-100)
	metalMatch := 0
	for _, m := range detectedMetals {
		for _, am := range t.ApplicableMetals {
			if am == m {
				metalMatch++
				break
			}
		}
	}
	metalCoverage := 0.0
	if len(detectedMetals) > 0 {
		metalCoverage = float64(metalMatch) / float64(len(detectedMetals)) * 100
	}
	scores["metal_coverage"] = metalCoverage

	// 2. 修复效率 (0-100)
	scores["efficiency"] = t.RemediationEfficiency

	// 3. 土壤适用性 (0-100)
	soilMatch := false
	if len(t.ApplicableSoilTypes) == 0 {
		soilMatch = true
	} else {
		for _, st := range t.ApplicableSoilTypes {
			if st == soilType || st == "各种土壤" {
				soilMatch = true
				break
			}
		}
	}
	if soilMatch {
		scores["soil_applicability"] = 100
	} else {
		scores["soil_applicability"] = 40
	}

	// 4. 成本经济性 (0-100，成本越低得分越高)
	avgCost := (t.CostLow + t.CostHigh) / 2
	costScore := 100.0
	if avgCost > 0 {
		costScore = math.Max(0, 100-(avgCost/15000)*100)
	}
	scores["cost"] = costScore

	// 5. 修复周期 (0-100，周期越短得分越高)
	avgDuration := float64(t.DurationMonthsLow+t.DurationMonthsHigh) / 2
	durationScore := 100.0
	if avgDuration > 0 {
		durationScore = math.Max(0, 100-(avgDuration/60)*100)
	}
	scores["duration"] = durationScore

	// 6. 环境影响 (0-100)
	scores["environmental"] = t.EnvironmentalImpactScore * 10

	// 7. 可持续性 (0-100)
	scores["sustainability"] = t.SustainabilityScore * 10

	return scores
}

// 组合权重：AHP主观权重 + 熵权法客观权重
// alpha: 主观权重占比 (0~1)
func (s *MCDMService) CalculateCombinedWeights(
	entropyWeights map[string]float64,
	alpha float64,
) map[string]float64 {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}

	combined := make(map[string]float64)
	total := 0.0

	for _, c := range ahpCriteria {
		w := alpha*s.ahpWeights[c] + (1-alpha)*entropyWeights[c]
		combined[c] = w
		total += w
	}

	if total > 1e-10 {
		for c := range combined {
			combined[c] /= total
		}
	}

	return combined
}

// 根据污染程度动态调整 alpha (主观权重占比)
// 污染越重，专家经验权重越高
func (s *MCDMService) GetDynamicAlpha(pollutionIndex float64) float64 {
	pi := math.Min(5.0, math.Max(0, pollutionIndex))
	alpha := 0.3 + (pi/5.0)*0.4 // 0.3 ~ 0.7
	return alpha
}

// 检测数据缺失情况
func (s *MCDMService) detectMissingData(techs []models.RemediationTechnology) map[string]bool {
	missing := make(map[string]bool)

	hasCost := false
	for _, t := range techs {
		if t.CostLow > 0 && t.CostHigh > 0 {
			hasCost = true
			break
		}
	}
	if !hasCost {
		missing["cost"] = true
	}

	hasDuration := false
	for _, t := range techs {
		if t.DurationMonthsLow > 0 && t.DurationMonthsHigh > 0 {
			hasDuration = true
			break
		}
	}
	if !hasDuration {
		missing["duration"] = true
	}

	return missing
}

// 数据缺失时自适应调整权重
func (s *MCDMService) AdjustWeightsForMissingData(
	weights map[string]float64,
	missingCriteria map[string]bool,
) map[string]float64 {
	adjusted := make(map[string]float64)
	totalValidWeight := 0.0

	for c, w := range weights {
		if !missingCriteria[c] {
			adjusted[c] = w
			totalValidWeight += w
		} else {
			adjusted[c] = 0
		}
	}

	if totalValidWeight > 1e-10 {
		for c := range adjusted {
			if adjusted[c] > 0 {
				adjusted[c] /= totalValidWeight
			}
		}
	}

	return adjusted
}

// ========================================
// TOPSIS 综合评分
// ========================================

func (s *MCDMService) ScoreTechnologies(
	techs []models.RemediationTechnology,
	detectedMetals []string,
	metalConc map[string]float64,
	speciation map[string]*models.MetalSpeciation,
	soilType string,
	pollutionIndex float64,
) []models.TechnologyScore {
	if len(techs) == 0 {
		return []models.TechnologyScore{}
	}

	// 1. 计算熵权法权重
	entropyWeights := s.CalculateEntropyWeights(techs, detectedMetals, soilType, pollutionIndex)

	// 2. 获取动态 alpha
	alpha := s.GetDynamicAlpha(pollutionIndex)

	// 3. 组合权重
	combinedWeights := s.CalculateCombinedWeights(entropyWeights, alpha)

	// 4. 检测数据缺失
	missingData := s.detectMissingData(techs)

	// 5. 自适应调整权重
	adjustedWeights := s.AdjustWeightsForMissingData(combinedWeights, missingData)

	// 6. 构建加权标准化决策矩阵
	n := len(techs)
	m := len(ahpCriteria)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, m)
	}

	rawScores := make([]map[string]float64, n)
	for i, t := range techs {
		rawScores[i] = s.calculateRawScores(t, detectedMetals, soilType, pollutionIndex)
		for j, c := range ahpCriteria {
			matrix[i][j] = rawScores[i][c] * adjustedWeights[c]
		}
	}

	// 7. TOPSIS：计算正负理想解
	positiveIdeal := make([]float64, m)
	negativeIdeal := make([]float64, m)
	for j := 0; j < m; j++ {
		maxVal := math.Inf(-1)
		minVal := math.Inf(1)
		for i := 0; i < n; i++ {
			if matrix[i][j] > maxVal {
				maxVal = matrix[i][j]
			}
			if matrix[i][j] < minVal {
				minVal = matrix[i][j]
			}
		}
		positiveIdeal[j] = maxVal
		negativeIdeal[j] = minVal
	}

	// 8. 计算各方案到正负理想解的距离
	scores := make([]models.TechnologyScore, n)
	for i, t := range techs {
		dPos := 0.0
		dNeg := 0.0
		for j := 0; j < m; j++ {
			dPos += math.Pow(matrix[i][j]-positiveIdeal[j], 2)
			dNeg += math.Pow(matrix[i][j]-negativeIdeal[j], 2)
		}
		dPos = math.Sqrt(dPos)
		dNeg = math.Sqrt(dNeg)

		closeness := 0.0
		if dPos+dNeg > 1e-10 {
			closeness = dNeg / (dPos + dNeg)
		}

		// 转换为0-100分
		totalScore := closeness * 100

		// 9. 领域知识加分项
		mobilityFactor := calculateMobilityFactorForMCDM(detectedMetals, speciation)
		if mobilityFactor > 0.5 {
			for _, st := range []string{"固化稳定化", "植物稳定修复"} {
				if t.Category == st {
					totalScore += 3
				}
			}
		}
		if metalConc["Hg"] > 38 {
			if t.Category == "热脱附" {
				totalScore += 5
			}
		}

		// 组装子分数（用于展示）
		subScores := make(map[string]float64)
		for c, val := range rawScores[i] {
			subScores[c] = math.Round(val*100) / 100
		}
		// 额外信息：主客观权重占比
		subScores["weight_ahp"] = math.Round(alpha*1000) / 10
		subScores["weight_entropy"] = math.Round((1-alpha)*1000) / 10

		matchedMetals := 0
		for _, metal := range detectedMetals {
			for _, am := range t.ApplicableMetals {
				if am == metal {
					matchedMetals++
					break
				}
			}
		}

		scores[i] = models.TechnologyScore{
			RemediationTechnology: t,
			TotalScore:            math.Round(totalScore*100) / 100,
			SubScores:             subScores,
			MatchedMetals:         matchedMetals,
		}
	}

	// 排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].TotalScore > scores[j].TotalScore
	})

	if len(scores) > 5 {
		scores = scores[:5]
	}

	return scores
}

func calculateMobilityFactorForMCDM(metals []string, speciation map[string]*models.MetalSpeciation) float64 {
	if len(speciation) == 0 {
		return 0.3
	}
	totalMobility := 0.0
	count := 0
	for _, m := range metals {
		if sp, ok := speciation[m]; ok {
			total := sp.Exchangeable + sp.CarbonateBound + sp.FeMnOxideBound + sp.OrganicBound + sp.Residual
			if total > 0 {
				mobile := (sp.Exchangeable + sp.CarbonateBound) / total
				totalMobility += mobile
				count++
			}
		}
	}
	if count == 0 {
		return 0.3
	}
	return totalMobility / float64(count)
}
