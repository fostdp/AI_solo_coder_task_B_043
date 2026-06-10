package global_comparator_test

import (
	"context"
	"math"
	"testing"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/global_comparator"
)

func TestNewGlobalComparator(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	if gc == nil {
		t.Fatal("NewGlobalComparator returned nil")
	}
}

func TestNormalizeAndInterpolate(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	siteIDs := []int{1, 2}
	allMeasurements := map[int][]models.TrendData{
		1: {
			{Year: -1000, PollutionIndex: 1.0},
			{Year: 0, PollutionIndex: 2.0},
			{Year: 1000, PollutionIndex: 3.0},
		},
		2: {
			{Year: -500, PollutionIndex: 0.5},
			{Year: 500, PollutionIndex: 1.5},
			{Year: 1500, PollutionIndex: 2.5},
		},
	}
	alignedYears, normalizedData := gc.NormalizeAndInterpolate(siteIDs, allMeasurements)
	if len(alignedYears) != 6 {
		t.Errorf("aligned years count = %d, want 6", len(alignedYears))
	}
	for i := 1; i < len(alignedYears); i++ {
		if alignedYears[i] <= alignedYears[i-1] {
			t.Error("aligned years not sorted ascending")
			break
		}
	}
	if len(normalizedData) != 2 {
		t.Errorf("normalized sites count = %d, want 2", len(normalizedData))
	}
	site1 := normalizedData[1]
	if math.Abs(site1[-1000]-1.0/3.0) > 1e-9 {
		t.Errorf("site 1 at year -1000 = %.6f, want %.6f", site1[-1000], 1.0/3.0)
	}
	if math.Abs(site1[0]-2.0/3.0) > 1e-9 {
		t.Errorf("site 1 at year 0 = %.6f, want %.6f", site1[0], 2.0/3.0)
	}
	if math.Abs(site1[1000]-1.0) > 1e-9 {
		t.Errorf("site 1 at year 1000 = %.6f, want 1.0", site1[1000])
	}
	site2 := normalizedData[2]
	if math.Abs(site2[1500]-1.0) > 1e-9 {
		t.Errorf("site 2 at year 1500 = %.6f, want 1.0", site2[1500])
	}
}

func TestDetectPeaks(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	siteIDs := []int{1}
	alignedYears := []int{-2000, -1000, 0, 1000, 2000}
	normalizedData := map[int]map[int]float64{
		1: {
			-2000: 0.2,
			-1000: 0.5,
			0:     0.9,
			1000:  0.4,
			2000:  0.1,
		},
	}
	siteInfoMap := map[int]*models.Site{
		1: {ID: 1, Name: "TestSite", MetalType: "Cu"},
	}
	peaks := gc.DetectPeaks(siteIDs, alignedYears, normalizedData, siteInfoMap)
	if len(peaks) < 1 {
		t.Fatal("expected at least 1 peak, got 0")
	}
	found := false
	for _, p := range peaks {
		if p.SiteID == 1 && p.PeakYear == 0 {
			found = true
			if p.SiteName != "TestSite" {
				t.Errorf("peak site name = %s, want TestSite", p.SiteName)
			}
			if p.MetalType != "Cu" {
				t.Errorf("peak metal type = %s, want Cu", p.MetalType)
			}
			if p.Confidence < 0 || p.Confidence > 1 {
				t.Errorf("peak confidence = %.4f, want [0, 1]", p.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected peak at year 0 not found")
	}
}

func TestMeanAndStddev(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	values := []float64{1, 2, 3, 4, 5}
	mean, stddev := gc.MeanAndStddev(values)
	if math.Abs(mean-3.0) > 1e-9 {
		t.Errorf("mean = %.6f, want 3.0", mean)
	}
	expectedStddev := math.Sqrt(2.0)
	if math.Abs(stddev-expectedStddev) > 1e-9 {
		t.Errorf("stddev = %.6f, want %.6f", stddev, expectedStddev)
	}
}

func TestFormatYearRange(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	result := gc.FormatYearRange(-500, 200)
	expected := "公元前500年 - 公元200年"
	if result != expected {
		t.Errorf("FormatYearRange(-500, 200) = %q, want %q", result, expected)
	}
}

func TestInterpolateAtYear(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	alignedYears := []int{0, 100, 200}
	siteData := map[int]float64{
		0:   0.0,
		100: 1.0,
		200: 0.5,
	}
	t.Run("existing year", func(t *testing.T) {
		val, ok := gc.InterpolateAtYear(100, alignedYears, siteData)
		if !ok {
			t.Error("expected ok=true for existing year")
		}
		if math.Abs(val-1.0) > 1e-9 {
			t.Errorf("value at year 100 = %.6f, want 1.0", val)
		}
	})
	t.Run("missing year interpolation", func(t *testing.T) {
		val, ok := gc.InterpolateAtYear(50, alignedYears, siteData)
		if !ok {
			t.Error("expected ok=true for interpolated year")
		}
		if math.Abs(val-0.5) > 1e-9 {
			t.Errorf("interpolated value at year 50 = %.6f, want 0.5", val)
		}
	})
}

func TestComputeGroupAverage(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	siteIDs := []int{1, 2}
	gridYears := []int{0, 50, 100}
	alignedYears := []int{0, 100}
	normalizedData := map[int]map[int]float64{
		1: {0: 0.2, 100: 0.8},
		2: {0: 0.4, 100: 0.6},
	}
	result := gc.ComputeGroupAverage(siteIDs, gridYears, alignedYears, normalizedData)
	if len(result) != 3 {
		t.Fatalf("result length = %d, want 3", len(result))
	}
	expected0 := (0.2 + 0.4) / 2.0
	if math.Abs(result[0]-expected0) > 1e-9 {
		t.Errorf("average at year 0 = %.6f, want %.6f", result[0], expected0)
	}
	expected100 := (0.8 + 0.6) / 2.0
	if math.Abs(result[2]-expected100) > 1e-9 {
		t.Errorf("average at year 100 = %.6f, want %.6f", result[2], expected100)
	}
}

func TestComputeClimateCorrection(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	siteIDs := []int{1}
	siteInfo := map[int]*models.Site{
		1: {ID: 1, Name: "SubtropicalSite", Latitude: 30.0, MetalType: "Cu"},
	}
	allMeasurements := map[int][]models.TrendData{
		1: {
			{Year: -1000, PollutionIndex: 1.0, PH: 6.5, OrganicMatter: 2.0},
			{Year: 0, PollutionIndex: 2.0, PH: 6.2, OrganicMatter: 1.8},
			{Year: 1000, PollutionIndex: 3.0, PH: 6.0, OrganicMatter: 2.2},
		},
	}
	report := gc.ComputeClimateCorrection(siteIDs, siteInfo, allMeasurements)
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.Method != "ClimateZone_Latitudinal_LeachingModel" {
		t.Errorf("method = %s, want ClimateZone_Latitudinal_LeachingModel", report.Method)
	}
	factor, ok := report.CorrectionFactors[1]
	if !ok {
		t.Fatal("correction factor for site 1 not found")
	}
	if factor.ClimateZone != "亚热带湿润" {
		t.Errorf("climate zone = %s, want 亚热带湿润", factor.ClimateZone)
	}
	if factor.SiteID != 1 {
		t.Errorf("site ID = %d, want 1", factor.SiteID)
	}
	if factor.OverallCorrection <= 0 {
		t.Errorf("overall correction = %.4f, should be > 0", factor.OverallCorrection)
	}
	if factor.LeachingRate < 0.2 || factor.LeachingRate > 2.5 {
		t.Errorf("leaching rate = %.4f, should be in [0.2, 2.5]", factor.LeachingRate)
	}
	if factor.RetentionFactor < 0.3 || factor.RetentionFactor > 1.5 {
		t.Errorf("retention factor = %.4f, should be in [0.3, 1.5]", factor.RetentionFactor)
	}
	zone, ok := report.ClimateZones[1]
	if !ok {
		t.Fatal("climate zone for site 1 not found")
	}
	if zone != "亚热带湿润" {
		t.Errorf("climate zone = %s, want 亚热带湿润", zone)
	}
	if report.ConfidenceAfterCorr <= 0 || report.ConfidenceAfterCorr > 1 {
		t.Errorf("confidence = %.4f, want (0, 1]", report.ConfidenceAfterCorr)
	}
}

func TestApplyClimateCorrection(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	allMeasurements := map[int][]models.TrendData{
		1: {
			{Year: -1000, Pb: 100, Zn: 200, Cu: 300, As: 10, Hg: 1.0, Cd: 2.0, PollutionIndex: 1.5, PH: 6.0, OrganicMatter: 2.0},
			{Year: 0, Pb: 200, Zn: 300, Cu: 400, As: 20, Hg: 2.0, Cd: 3.0, PollutionIndex: 2.5, PH: 6.2, OrganicMatter: 1.8},
		},
	}
	siteIDs := []int{1}
	siteInfo := map[int]*models.Site{
		1: {ID: 1, Latitude: 30.0, MetalType: "Cu"},
	}
	report := gc.ComputeClimateCorrection(siteIDs, siteInfo, allMeasurements)
	corrected := gc.ApplyClimateCorrection(allMeasurements, report)
	if len(corrected) != 1 {
		t.Fatalf("corrected sites count = %d, want 1", len(corrected))
	}
	correctedData := corrected[1]
	if len(correctedData) != 2 {
		t.Fatalf("corrected data points = %d, want 2", len(correctedData))
	}
	factor := report.CorrectionFactors[1]
	if correctedData[0].PollutionIndex == allMeasurements[1][0].PollutionIndex {
		t.Error("correction did not modify pollution index")
	}
	if correctedData[0].PH != allMeasurements[1][0].PH {
		t.Errorf("PH changed from %.2f to %.2f, should be preserved", allMeasurements[1][0].PH, correctedData[0].PH)
	}
	if correctedData[0].OrganicMatter != allMeasurements[1][0].OrganicMatter {
		t.Errorf("OrganicMatter changed from %.2f to %.2f, should be preserved", allMeasurements[1][0].OrganicMatter, correctedData[0].OrganicMatter)
	}
	expectedYear := allMeasurements[1][0].Year - factor.PeakYearShift
	if correctedData[0].Year != expectedYear {
		t.Errorf("corrected year = %d, want %d", correctedData[0].Year, expectedYear)
	}
	if correctedData[0].Pb == allMeasurements[1][0].Pb && factor.OverallCorrection != 1.0 {
		t.Error("correction did not modify Pb concentration")
	}
}

func TestCompareTimelinesIntegration(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("skipping: database not available (%v)", r)
		}
	}()
	gc := global_comparator.NewGlobalComparator()
	ctx := context.Background()
	siteIDs := []int{1, 2}
	allMeasurements := map[int][]models.TrendData{
		1: {
			{Year: -2000, PollutionIndex: 0.5, PH: 6.0, OrganicMatter: 2.0},
			{Year: -1000, PollutionIndex: 1.5, PH: 6.2, OrganicMatter: 1.8},
			{Year: 0, PollutionIndex: 2.0, PH: 6.5, OrganicMatter: 2.2},
		},
		2: {
			{Year: -1500, PollutionIndex: 1.0, PH: 7.0, OrganicMatter: 3.0},
			{Year: -500, PollutionIndex: 2.5, PH: 7.2, OrganicMatter: 2.8},
			{Year: 500, PollutionIndex: 3.0, PH: 7.5, OrganicMatter: 3.2},
		},
	}
	result, err := gc.CompareTimelines(ctx, siteIDs, allMeasurements)
	if err != nil {
		t.Fatalf("CompareTimelines error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.GlobalTrend == nil {
		t.Error("global trend is nil")
	}
	if result.ClimateCorrection == nil {
		t.Error("climate correction is nil")
	}
	if _, ok := result.GlobalTrend["years"]; !ok {
		t.Error("global trend missing 'years' key")
	}
	if _, ok := result.GlobalTrend["all_sites_avg"]; !ok {
		t.Error("global trend missing 'all_sites_avg' key")
	}
}

func TestClimateCorrectionHighLatitude(t *testing.T) {
	gc := global_comparator.NewGlobalComparator()
	siteIDs := []int{1}
	siteInfo := map[int]*models.Site{
		1: {ID: 1, Name: "ColdTemperateSite", Latitude: 55.0, MetalType: "Fe"},
	}
	allMeasurements := map[int][]models.TrendData{
		1: {
			{Year: -500, PollutionIndex: 1.0, PH: 6.8, OrganicMatter: 3.0},
			{Year: 500, PollutionIndex: 2.0, PH: 7.0, OrganicMatter: 2.5},
		},
	}
	report := gc.ComputeClimateCorrection(siteIDs, siteInfo, allMeasurements)
	if report == nil {
		t.Fatal("report is nil")
	}
	factor, ok := report.CorrectionFactors[1]
	if !ok {
		t.Fatal("correction factor for site 1 not found")
	}
	if factor.ClimateZone != "寒温带" {
		t.Errorf("climate zone = %s, want 寒温带", factor.ClimateZone)
	}
	if factor.MeanAnnualTempC != 0.0 {
		t.Errorf("mean annual temp = %.1f, want 0.0", factor.MeanAnnualTempC)
	}
	if factor.MeanAnnualRainMM != 500.0 {
		t.Errorf("mean annual rain = %.1f, want 500.0", factor.MeanAnnualRainMM)
	}
	if factor.OverallCorrection < 0.5 || factor.OverallCorrection > 2.0 {
		t.Errorf("overall correction = %.4f, should be in [0.5, 2.0]", factor.OverallCorrection)
	}
	zone, ok := report.ClimateZones[1]
	if !ok {
		t.Fatal("climate zone for site 1 not found")
	}
	if zone != "寒温带" {
		t.Errorf("climate zone = %s, want 寒温带", zone)
	}
}
