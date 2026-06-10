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

	alignedYears, normalizedData := t.normalizeAndInterpolate(siteIDs, allMeasurements)

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
