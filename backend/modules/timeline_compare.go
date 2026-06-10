package modules

import (
	"context"
	"math"
	"sort"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
)

type TimelineCompareModule struct {
	bus *EventBus
}

type ClimateZoneProfile struct {
	Name            string
	LatitudeMin     float64
	LatitudeMax     float64
	MeanTempC       float64
	MeanRainMM      float64
	TypicalSoilPH   float64
	LeachingBase    float64 // 基础淋溶速率
	RetentionBase   float64 // 基础持留率
}

var climateZoneProfiles = []ClimateZoneProfile{
	{"热带季风", 0, 23.5, 25.0, 1500, 5.5, 1.4, 0.5},
	{"亚热带湿润", 23.5, 35, 17.0, 1200, 6.0, 1.2, 0.6},
	{"温带半湿润", 35, 45, 10.0, 700, 7.0, 0.9, 0.8},
	{"温带干旱", 35, 50, 8.0, 300, 8.0, 0.5, 1.1},
	{"寒温带", 45, 70, 0.0, 500, 6.5, 0.6, 1.0},
	{"地中海气候", 30, 45, 16.0, 600, 7.5, 0.8, 0.9},
	{"热带沙漠", 15, 30, 28.0, 100, 8.5, 0.3, 1.3},
}

type CivilizationEpochExtended struct {
	EpochName     string   `json:"epoch_name"`
	YearRange     string   `json:"year_range"`
	YearStart     int      `json:"year_start"`
	YearEnd       int      `json:"year_end"`
	KeySites      []string `json:"key_sites"`
	KeyTechnology string   `json:"key_technology"`
	Description   string   `json:"description"`
	PeakCount     int      `json:"peak_count"`
	AvgIntensity  float64  `json:"avg_intensity"`
}

type peakCluster struct {
	CenterYear   int
	AvgIntensity float64
	Peaks        []models.TimelinePeak
}

func NewTimelineCompareModule() *TimelineCompareModule {
	return &TimelineCompareModule{
		bus: GetEventBus(),
	}
}

func (t *TimelineCompareModule) CompareTimelines(
	ctx context.Context,
	siteIDs []int,
	allMeasurements map[int][]models.TrendData,
) (*models.TimelineComparisonResult, error) {

	sites := make([]interface{}, 0, len(siteIDs))
	siteInfoMap := make(map[int]*models.Site)
	for _, id := range siteIDs {
		site, err := repository.GetSite(ctx, id)
		if err == nil && site != nil {
			sites = append(sites, site)
			siteInfoMap[id] = site
		}
	}

	// ========== Step 0: 计算各遗址气候校正因子 ==========
	climateReport := t.computeClimateCorrection(siteIDs, siteInfoMap, allMeasurements)

	// 应用气候校正到测量数据（修正指纹漂移）
	correctedMeasurements := t.applyClimateCorrection(allMeasurements, climateReport)

	alignedYears, normalizedData := t.normalizeAndInterpolate(siteIDs, correctedMeasurements)

	allPeaks := t.detectPeaks(siteIDs, alignedYears, normalizedData, siteInfoMap)
	_ = t.clusterPeaks(allPeaks)

	epochsExtended := t.mapPeaksToEpochs(allPeaks, siteInfoMap)

	epochs := make([]models.CivilizationEpoch, 0, len(epochsExtended))
	for _, ep := range epochsExtended {
		epochs = append(epochs, models.CivilizationEpoch{
			EpochName:     ep.EpochName,
			YearRange:     ep.YearRange,
			YearStart:     ep.YearStart,
			YearEnd:       ep.YearEnd,
			KeySites:      ep.KeySites,
			KeyTechnology: ep.KeyTechnology,
			Description:   ep.Description,
		})
	}
	_ = epochsExtended

	globalTrend := t.computeGlobalTrend(siteIDs, alignedYears, normalizedData, siteInfoMap)

	result := &models.TimelineComparisonResult{
		Sites:              sites,
		Peaks:              allPeaks,
		CivilizationEpochs: epochs,
		GlobalTrend:        globalTrend,
		ClimateCorrection:  climateReport,
	}

	return result, nil
}

func (t *TimelineCompareModule) normalizeAndInterpolate(
	siteIDs []int,
	allMeasurements map[int][]models.TrendData,
) ([]int, map[int]map[int]float64) {

	yearSet := make(map[int]bool)
	for _, id := range siteIDs {
		if measurements, ok := allMeasurements[id]; ok {
			for _, m := range measurements {
				yearSet[m.Year] = true
			}
		}
	}

	alignedYears := make([]int, 0, len(yearSet))
	for y := range yearSet {
		alignedYears = append(alignedYears, y)
	}
	sort.Ints(alignedYears)

	normalizedData := make(map[int]map[int]float64)

	for _, id := range siteIDs {
		measurements, ok := allMeasurements[id]
		if !ok || len(measurements) == 0 {
			continue
		}

		sort.Slice(measurements, func(i, j int) bool {
			return measurements[i].Year < measurements[j].Year
		})

		rawData := make(map[int]float64)
		maxPI := 0.0
		for _, m := range measurements {
			rawData[m.Year] = m.PollutionIndex
			if m.PollutionIndex > maxPI {
				maxPI = m.PollutionIndex
			}
		}

		siteYears := make([]int, 0, len(measurements))
		for _, m := range measurements {
			siteYears = append(siteYears, m.Year)
		}
		sort.Ints(siteYears)

		interpolated := make(map[int]float64)
		if len(siteYears) == 0 {
			continue
		}

		for _, targetYear := range alignedYears {
			if val, exists := rawData[targetYear]; exists {
				if maxPI > 0 {
					interpolated[targetYear] = val / maxPI
				} else {
					interpolated[targetYear] = 0
				}
				continue
			}

			var leftYear, rightYear int
			var hasLeft, hasRight bool

			for _, y := range siteYears {
				if y < targetYear {
					leftYear = y
					hasLeft = true
				} else if y > targetYear {
					rightYear = y
					hasRight = true
					break
				}
			}

			var interpolatedVal float64
			if hasLeft && hasRight {
				leftVal := rawData[leftYear]
				rightVal := rawData[rightYear]
				ratio := float64(targetYear-leftYear) / float64(rightYear-leftYear)
				interpolatedVal = leftVal + ratio*(rightVal-leftVal)
			} else if hasLeft {
				interpolatedVal = rawData[leftYear]
			} else if hasRight {
				interpolatedVal = rawData[rightYear]
			} else {
				interpolatedVal = 0
			}

			if maxPI > 0 {
				interpolated[targetYear] = interpolatedVal / maxPI
			} else {
				interpolated[targetYear] = 0
			}
		}

		normalizedData[id] = interpolated
	}

	return alignedYears, normalizedData
}

func (t *TimelineCompareModule) detectPeaks(
	siteIDs []int,
	alignedYears []int,
	normalizedData map[int]map[int]float64,
	siteInfoMap map[int]*models.Site,
) []models.TimelinePeak {

	allPeaks := make([]models.TimelinePeak, 0)

	for _, id := range siteIDs {
		siteData, ok := normalizedData[id]
		if !ok || len(siteData) < 3 {
			continue
		}

		values := make([]float64, len(alignedYears))
		for i, y := range alignedYears {
			values[i] = siteData[y]
		}

		mean, stddev := t.meanAndStddev(values)
		threshold := mean + 0.5*stddev

		siteName := ""
		metalType := ""
		if site, ok := siteInfoMap[id]; ok {
			siteName = site.Name
			metalType = site.MetalType
		}

		for i := 1; i < len(alignedYears)-1; i++ {
			y := values[i]
			prev := values[i-1]
			next := values[i+1]

			isLocalMax := y > prev && y > next
			isAboveThreshold := y > threshold

			if isLocalMax && isAboveThreshold {
				var confidence float64
				if stddev > 0 {
					confidence = (y - mean) / stddev
				}
				if confidence < 0 {
					confidence = 0
				}
				if confidence > 1 {
					confidence = 1
				}

				allPeaks = append(allPeaks, models.TimelinePeak{
					SiteID:     id,
					SiteName:   siteName,
					PeakYear:   alignedYears[i],
					PeakValue:  y,
					MetalType:  metalType,
					Confidence: confidence,
				})
			}
		}
	}

	return allPeaks
}

func (t *TimelineCompareModule) clusterPeaks(peaks []models.TimelinePeak) []peakCluster {
	if len(peaks) == 0 {
		return nil
	}

	sortedPeaks := make([]models.TimelinePeak, len(peaks))
	copy(sortedPeaks, peaks)
	sort.Slice(sortedPeaks, func(i, j int) bool {
		return sortedPeaks[i].PeakYear < sortedPeaks[j].PeakYear
	})

	clusters := make([]peakCluster, 0)
	clusterEps := 100

	currentCluster := peakCluster{
		CenterYear:   sortedPeaks[0].PeakYear,
		AvgIntensity: sortedPeaks[0].PeakValue,
		Peaks:        []models.TimelinePeak{sortedPeaks[0]},
	}

	for i := 1; i < len(sortedPeaks); i++ {
		peak := sortedPeaks[i]
		diff := peak.PeakYear - currentCluster.CenterYear

		if diff <= clusterEps {
			currentCluster.Peaks = append(currentCluster.Peaks, peak)

			sumYear := 0
			sumIntensity := 0.0
			for _, p := range currentCluster.Peaks {
				sumYear += p.PeakYear
				sumIntensity += p.PeakValue
			}
			currentCluster.CenterYear = sumYear / len(currentCluster.Peaks)
			currentCluster.AvgIntensity = sumIntensity / float64(len(currentCluster.Peaks))
		} else {
			clusters = append(clusters, currentCluster)
			currentCluster = peakCluster{
				CenterYear:   peak.PeakYear,
				AvgIntensity: peak.PeakValue,
				Peaks:        []models.TimelinePeak{peak},
			}
		}
	}
	clusters = append(clusters, currentCluster)

	return clusters
}

func (t *TimelineCompareModule) mapPeaksToEpochs(
	peaks []models.TimelinePeak,
	siteInfoMap map[int]*models.Site,
) []CivilizationEpochExtended {

	epochPeaks := make(map[int][]models.TimelinePeak)

	for _, peak := range peaks {
		for idx, epochConfig := range config.DefaultCivilizationEpochs.Epochs {
			if peak.PeakYear >= epochConfig.YearStart && peak.PeakYear < epochConfig.YearEnd {
				epochPeaks[idx] = append(epochPeaks[idx], peak)
				break
			}
		}
	}

	result := make([]CivilizationEpochExtended, 0)

	for idx, epochConfig := range config.DefaultCivilizationEpochs.Epochs {
		peaksInEpoch := epochPeaks[idx]

		keySitesSet := make(map[string]bool)
		sumIntensity := 0.0
		for _, p := range peaksInEpoch {
			if p.SiteName != "" {
				keySitesSet[p.SiteName] = true
			}
			sumIntensity += p.PeakValue
		}

		keySites := make([]string, 0, len(keySitesSet))
		for s := range keySitesSet {
			keySites = append(keySites, s)
		}
		sort.Strings(keySites)

		avgIntensity := 0.0
		if len(peaksInEpoch) > 0 {
			avgIntensity = sumIntensity / float64(len(peaksInEpoch))
		}

		yearRange := t.formatYearRange(epochConfig.YearStart, epochConfig.YearEnd)

		result = append(result, CivilizationEpochExtended{
			EpochName:     epochConfig.Name,
			YearRange:     yearRange,
			YearStart:     epochConfig.YearStart,
			YearEnd:       epochConfig.YearEnd,
			KeySites:      keySites,
			KeyTechnology: epochConfig.KeyTechnology,
			Description:   epochConfig.Description,
			PeakCount:     len(peaksInEpoch),
			AvgIntensity:  avgIntensity,
		})
	}

	return result
}

func (t *TimelineCompareModule) computeGlobalTrend(
	siteIDs []int,
	alignedYears []int,
	normalizedData map[int]map[int]float64,
	siteInfoMap map[int]*models.Site,
) map[string][]float64 {

	result := make(map[string][]float64)

	if len(alignedYears) == 0 {
		result["years"] = []float64{}
		result["all_sites_avg"] = []float64{}
		result["copper_sites"] = []float64{}
		result["iron_sites"] = []float64{}
		result["silver_sites"] = []float64{}
		result["lead_sites"] = []float64{}
		result["mercury_sites"] = []float64{}
		return result
	}

	minYear := alignedYears[0]
	maxYear := alignedYears[len(alignedYears)-1]
	if minYear > -3000 {
		minYear = -3000
	}
	if maxYear < 2000 {
		maxYear = 2000
	}

	gridYears := make([]int, 0)
	step := 50
	for y := minYear; y <= maxYear; y += step {
		gridYears = append(gridYears, y)
	}

	yearsFloat := make([]float64, len(gridYears))
	for i, y := range gridYears {
		yearsFloat[i] = float64(y)
	}
	result["years"] = yearsFloat

	metalGroups := map[string][]int{
		"copper":  {},
		"iron":    {},
		"silver":  {},
		"lead":    {},
		"mercury": {},
	}

	for _, id := range siteIDs {
		if site, ok := siteInfoMap[id]; ok {
			switch site.MetalType {
			case "Cu", "铜", "copper":
				metalGroups["copper"] = append(metalGroups["copper"], id)
			case "Fe", "铁", "iron":
				metalGroups["iron"] = append(metalGroups["iron"], id)
			case "Ag", "银", "silver":
				metalGroups["silver"] = append(metalGroups["silver"], id)
			case "Pb", "铅", "lead":
				metalGroups["lead"] = append(metalGroups["lead"], id)
			case "Hg", "汞", "mercury":
				metalGroups["mercury"] = append(metalGroups["mercury"], id)
			}
		}
	}

	result["all_sites_avg"] = t.computeGroupAverage(siteIDs, gridYears, alignedYears, normalizedData)
	result["copper_sites"] = t.computeGroupAverage(metalGroups["copper"], gridYears, alignedYears, normalizedData)
	result["iron_sites"] = t.computeGroupAverage(metalGroups["iron"], gridYears, alignedYears, normalizedData)
	result["silver_sites"] = t.computeGroupAverage(metalGroups["silver"], gridYears, alignedYears, normalizedData)
	result["lead_sites"] = t.computeGroupAverage(metalGroups["lead"], gridYears, alignedYears, normalizedData)
	result["mercury_sites"] = t.computeGroupAverage(metalGroups["mercury"], gridYears, alignedYears, normalizedData)

	return result
}

func (t *TimelineCompareModule) computeGroupAverage(
	siteIDs []int,
	gridYears []int,
	alignedYears []int,
	normalizedData map[int]map[int]float64,
) []float64 {

	result := make([]float64, len(gridYears))

	if len(siteIDs) == 0 {
		return result
	}

	for gi, targetYear := range gridYears {
		sum := 0.0
		count := 0

		for _, id := range siteIDs {
			siteData, ok := normalizedData[id]
			if !ok {
				continue
			}

			val, ok := t.interpolateAtYear(targetYear, alignedYears, siteData)
			if ok {
				sum += val
				count++
			}
		}

		if count > 0 {
			result[gi] = sum / float64(count)
		} else {
			result[gi] = 0
		}
	}

	return result
}

func (t *TimelineCompareModule) interpolateAtYear(
	targetYear int,
	alignedYears []int,
	siteData map[int]float64,
) (float64, bool) {

	if val, ok := siteData[targetYear]; ok {
		return val, true
	}

	var leftYear, rightYear int
	var leftVal, rightVal float64
	var hasLeft, hasRight bool

	for _, y := range alignedYears {
		if y < targetYear {
			if val, ok := siteData[y]; ok {
				leftYear = y
				leftVal = val
				hasLeft = true
			}
		} else if y > targetYear {
			if val, ok := siteData[y]; ok {
				rightYear = y
				rightVal = val
				hasRight = true
			}
			break
		}
	}

	if hasLeft && hasRight {
		ratio := float64(targetYear-leftYear) / float64(rightYear-leftYear)
		return leftVal + ratio*(rightVal-leftVal), true
	} else if hasLeft {
		return leftVal, true
	} else if hasRight {
		return rightVal, true
	}

	return 0, false
}

func (t *TimelineCompareModule) meanAndStddev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	stddev := math.Sqrt(variance)

	return mean, stddev
}

func (t *TimelineCompareModule) formatYearRange(start, end int) string {
	startStr := ""
	endStr := ""

	if start < 0 {
		startStr = "公元前" + intToStr(-start) + "年"
	} else {
		startStr = "公元" + intToStr(start) + "年"
	}

	if end < 0 {
		endStr = "公元前" + intToStr(-end) + "年"
	} else {
		endStr = "公元" + intToStr(end) + "年"
	}

	return startStr + " - " + endStr
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []rune{}
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		digits = append([]rune{rune('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]rune{'-'}, digits...)
	}
	return string(digits)
}

// computeClimateCorrection 计算各遗址的气候校正因子
// 解决不同区域气候导致的重金属指纹保留率差异和峰位漂移问题
// 原理：高温多雨→淋溶快→表观峰位偏晚、峰值偏低；干旱少雨→淋溶慢→表观峰位偏早、峰值偏高
func (t *TimelineCompareModule) computeClimateCorrection(
	siteIDs []int,
	siteInfo map[int]*models.Site,
	allMeasurements map[int][]models.TrendData,
) *models.ClimateCorrectionReport {

	factors := make(map[int]models.ClimateSiteFactor)
	climateZones := make(map[int]string)
	driftMagnitude := make(map[int]float64)

	for _, id := range siteIDs {
		site, ok := siteInfo[id]
		if !ok || site == nil {
			continue
		}

		// 基于纬度确定气候带（模拟，实际可接入真实气候数据库）
		lat := math.Abs(site.Latitude)
		var profile ClimateZoneProfile
		for _, p := range climateZoneProfiles {
			if lat >= p.LatitudeMin && lat < p.LatitudeMax {
				profile = p
				break
			}
		}
		if profile.Name == "" {
			profile = ClimateZoneProfile{"温带半湿润", 35, 45, 10.0, 700, 7.0, 0.9, 0.8}
		}
		climateZones[id] = profile.Name

		// 从实际测量数据中提取pH和有机质（如果有）
		avgPH := profile.TypicalSoilPH
		avgOM := 2.0
		measurements, hasData := allMeasurements[id]
		if hasData && len(measurements) > 0 {
			sumPH := 0.0
			sumOM := 0.0
			countPH := 0
			countOM := 0
			for _, m := range measurements {
				if m.PH > 0 {
					sumPH += m.PH
					countPH++
				}
				if m.OrganicMatter > 0 {
					sumOM += m.OrganicMatter
					countOM++
				}
			}
			if countPH > 0 {
				avgPH = sumPH / float64(countPH)
			}
			if countOM > 0 {
				avgOM = sumOM / float64(countOM)
			}
		}

		// ========== 计算淋溶速率 ==========
		// 温度每升高10℃，反应速率约加倍（Arrhenius）
		tempFactor := math.Pow(2.0, (profile.MeanTempC-10.0)/10.0)
		// 降雨量非线性影响：>1000mm后淋溶加速
		rainFactor := 0.5 + profile.MeanRainMM/1000.0
		if rainFactor < 0.3 {
			rainFactor = 0.3
		}
		// pH影响：酸性(pH<6)加速重金属溶出，碱性(pH>8)抑制
		phFactor := 1.0
		if avgPH < 6.0 {
			phFactor = 1.0 + (6.0-avgPH)*0.2
		} else if avgPH > 8.0 {
			phFactor = math.Max(0.3, 1.0-(avgPH-8.0)*0.2)
		}
		// 有机质络合：高OM降低淋溶
		omFactor := math.Max(0.5, 1.0-avgOM/10.0)

		leachingRate := profile.LeachingBase * tempFactor * rainFactor * phFactor * omFactor
		if leachingRate < 0.2 {
			leachingRate = 0.2
		}
		if leachingRate > 2.5 {
			leachingRate = 2.5
		}

		// ========== 计算持留因子（保留率）==========
		retentionFactor := profile.RetentionBase * (1.0 / leachingRate) * 0.8
		if retentionFactor < 0.3 {
			retentionFactor = 0.3
		}
		if retentionFactor > 1.5 {
			retentionFactor = 1.5
		}

		// ========== 计算指纹峰位偏移（年） ==========
		// 淋溶越快，重金属向下迁移越快→地层中保存的峰值年代偏年轻（偏晚）
		// 经验公式：淋溶速率每增加0.1，峰值偏移约-20年（偏晚20年）
		peakYearShift := int(-(leachingRate - 0.9) * 200)
		if peakYearShift > 300 {
			peakYearShift = 300
		}
		if peakYearShift < -300 {
			peakYearShift = -300
		}

		// ========== 计算峰值振幅衰减系数 ==========
		// 淋溶越快，峰值越被平滑、振幅越小
		amplitudeDamp := math.Max(0.3, 1.0-(leachingRate-0.5)*0.5)

		// ========== 综合校正系数 ==========
		// 以温带半湿润（leaching≈0.9）为基准
		overallCorrection := 1.0 / retentionFactor
		if overallCorrection < 0.5 {
			overallCorrection = 0.5
		}
		if overallCorrection > 2.0 {
			overallCorrection = 2.0
		}

		factors[id] = models.ClimateSiteFactor{
			SiteID:            id,
			ClimateZone:       profile.Name,
			MeanAnnualTempC:   profile.MeanTempC,
			MeanAnnualRainMM:  profile.MeanRainMM,
			SoilPH:            roundFloat(avgPH, 2),
			LeachingRate:      roundFloat(leachingRate, 4),
			RetentionFactor:   roundFloat(retentionFactor, 4),
			OverallCorrection: roundFloat(overallCorrection, 4),
			PeakYearShift:     peakYearShift,
			AmplitudeDamp:     roundFloat(amplitudeDamp, 4),
		}

		// 漂移量百分比（相对于基准）
		driftMagnitude[id] = roundFloat(math.Abs(1.0-overallCorrection)*100.0, 2)
	}

	// 校正后置信度：漂移量越大，置信度越低
	avgDrift := 0.0
	if len(driftMagnitude) > 0 {
		for _, v := range driftMagnitude {
			avgDrift += v
		}
		avgDrift /= float64(len(driftMagnitude))
	}
	confidenceAfterCorr := math.Max(0.3, 1.0-avgDrift/100.0*0.7)

	// 校正说明
	note := "基于纬度气候带+土壤pH+有机质估算淋溶速率，校正指纹峰位漂移和振幅衰减"
	if avgDrift > 30 {
		note += "；不同区域气候差异较大（平均漂移>30%），建议结合考古地层学交叉验证"
	} else if avgDrift > 15 {
		note += "；不同区域存在中等气候差异，校正结果可靠性中等"
	} else {
		note += "；区域气候较为均一，校正结果可靠性较高"
	}

	return &models.ClimateCorrectionReport{
		Method:              "ClimateZone_Latitudinal_LeachingModel",
		CorrectionFactors:   factors,
		ClimateZones:        climateZones,
		DriftMagnitude:      driftMagnitude,
		ConfidenceAfterCorr: roundFloat(confidenceAfterCorr, 4),
		CorrectionNote:      note,
	}
}

// applyClimateCorrection 将气候校正因子应用于原始测量数据
// 修正内容：1) 峰值年代偏移  2) 振幅恢复（反衰减）  3) 整体浓度按持留因子缩放
func (t *TimelineCompareModule) applyClimateCorrection(
	allMeasurements map[int][]models.TrendData,
	report *models.ClimateCorrectionReport,
) map[int][]models.TrendData {

	if report == nil {
		return allMeasurements
	}

	corrected := make(map[int][]models.TrendData)

	for siteID, measurements := range allMeasurements {
		factor, ok := report.CorrectionFactors[siteID]
		if !ok {
			corrected[siteID] = measurements
			continue
		}

		correctedData := make([]models.TrendData, len(measurements))
		for i, m := range measurements {
			// 峰位偏移：校正年份（向左/向右平移）
			correctedYear := m.Year - factor.PeakYearShift

			// 振幅恢复：除以衰减系数（恢复被淋溶抹平的峰值）
			amplitudeRestore := 1.0
			if factor.AmplitudeDamp > 0.1 {
				amplitudeRestore = 1.0 / factor.AmplitudeDamp
			}
			// 限制在合理范围
			if amplitudeRestore > 2.5 {
				amplitudeRestore = 2.5
			}
			if amplitudeRestore < 0.5 {
				amplitudeRestore = 0.5
			}

			// 整体浓度按持留因子缩放
			corrFactor := factor.OverallCorrection * 0.7 + amplitudeRestore * 0.3
			if corrFactor < 0.5 {
				corrFactor = 0.5
			}
			if corrFactor > 2.0 {
				corrFactor = 2.0
			}

			correctedData[i] = models.TrendData{
				Year:           correctedYear,
				Pb:             m.Pb * corrFactor,
				Zn:             m.Zn * corrFactor,
				Cu:             m.Cu * corrFactor,
				As:             m.As * corrFactor,
				Hg:             m.Hg * corrFactor,
				Cd:             m.Cd * corrFactor,
				PollutionIndex: m.PollutionIndex * corrFactor,
				PH:             m.PH,
				OrganicMatter:  m.OrganicMatter,
				CEC:            m.CEC,
				SoilMoisture:   m.SoilMoisture,
				MeasurementDate: m.MeasurementDate,
				Metals:         m.Metals,
			}
		}
		corrected[siteID] = correctedData
	}

	return corrected
}

// roundFloat 工具：四舍五入到指定小数位
func roundFloat(v float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Round(v*pow) / pow
}
