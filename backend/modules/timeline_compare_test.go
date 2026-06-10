package modules

import (
	"math"
	"testing"

	"archaeology-pollution-system/models"
)

func TestNewTimelineCompareModule(t *testing.T) {
	m := NewTimelineCompareModule()
	if m == nil {
		t.Fatal("NewTimelineCompareModule 返回 nil")
	}
	if m.bus == nil {
		t.Error("EventBus 未初始化")
	}
}

func TestMeanAndStddev(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("正常数据", func(t *testing.T) {
		values := []float64{1, 2, 3, 4, 5}
		mean, stddev := m.meanAndStddev(values)
		if math.Abs(mean-3.0) > 1e-9 {
			t.Errorf("均值=%.2f，期望3.0", mean)
		}
		expectedStd := math.Sqrt(2.0)
		if math.Abs(stddev-expectedStd) > 1e-9 {
			t.Errorf("标准差=%.6f，期望%.6f", stddev, expectedStd)
		}
	})

	t.Run("空数据", func(t *testing.T) {
		mean, stddev := m.meanAndStddev([]float64{})
		if mean != 0 {
			t.Errorf("空数据均值=%.2f，期望0", mean)
		}
		if stddev != 0 {
			t.Errorf("空数据标准差=%.2f，期望0", stddev)
		}
	})

	t.Run("单元素", func(t *testing.T) {
		mean, stddev := m.meanAndStddev([]float64{5})
		if mean != 5 {
			t.Errorf("单元素均值=%.2f，期望5", mean)
		}
		if stddev != 0 {
			t.Errorf("单元素标准差=%.2f，期望0", stddev)
		}
	})

	t.Run("全相同值", func(t *testing.T) {
		values := []float64{2, 2, 2, 2, 2}
		mean, stddev := m.meanAndStddev(values)
		if mean != 2 {
			t.Errorf("均值=%.2f，期望2", mean)
		}
		if stddev != 0 {
			t.Errorf("标准差=%.6f，期望0", stddev)
		}
	})
}

func TestNormalizeAndInterpolate(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("两站点数据对齐", func(t *testing.T) {
		siteIDs := []int{1, 2}
		allData := map[int][]models.TrendData{
			1: {
				{Year: -2000, PollutionIndex: 0.5},
				{Year: -1000, PollutionIndex: 1.5},
				{Year: 0, PollutionIndex: 2.0},
			},
			2: {
				{Year: -1500, PollutionIndex: 1.0},
				{Year: -500, PollutionIndex: 2.5},
				{Year: 500, PollutionIndex: 3.0},
			},
		}

		alignedYears, normalized := m.normalizeAndInterpolate(siteIDs, allData)

		if len(alignedYears) != 6 {
			t.Errorf("对齐后年份数=%d，期望6", len(alignedYears))
		}

		for i := 1; i < len(alignedYears); i++ {
			if alignedYears[i] <= alignedYears[i-1] {
				t.Error("对齐后的年份未递增排序")
				break
			}
		}

		if len(normalized) != 2 {
			t.Errorf("站点数据数=%d，期望2", len(normalized))
		}

		site1Data := normalized[1]
		maxPI := 0.0
		for _, v := range site1Data {
			if v > maxPI {
				maxPI = v
			}
		}
		if math.Abs(maxPI-1.0) > 1e-9 {
			t.Errorf("归一化后最大值=%.6f，期望1.0", maxPI)
		}

		t.Logf("对齐年份: %v", alignedYears)
		t.Logf("站点1归一化数据: %v", site1Data)
	})

	t.Run("单站点数据", func(t *testing.T) {
		siteIDs := []int{1}
		allData := map[int][]models.TrendData{
			1: {
				{Year: -1000, PollutionIndex: 1.0},
				{Year: 0, PollutionIndex: 2.0},
				{Year: 1000, PollutionIndex: 3.0},
			},
		}
		alignedYears, normalized := m.normalizeAndInterpolate(siteIDs, allData)
		if len(alignedYears) != 3 {
			t.Errorf("单站点对齐后年份数=%d，期望3", len(alignedYears))
		}
		if len(normalized) != 1 {
			t.Errorf("归一化后站点数=%d，期望1", len(normalized))
		}
	})

	t.Run("空数据", func(t *testing.T) {
		siteIDs := []int{1}
		allData := map[int][]models.TrendData{}
		alignedYears, normalized := m.normalizeAndInterpolate(siteIDs, allData)
		if len(alignedYears) != 0 {
			t.Errorf("空数据对齐年份数=%d，期望0", len(alignedYears))
		}
		if len(normalized) != 0 {
			t.Errorf("空数据归一化站点数=%d，期望0", len(normalized))
		}
	})

	t.Run("插值验证 - 线性插值", func(t *testing.T) {
		siteIDs := []int{1}
		allData := map[int][]models.TrendData{
			1: {
				{Year: 0, PollutionIndex: 0.0},
				{Year: 100, PollutionIndex: 1.0},
				{Year: 200, PollutionIndex: 0.5},
			},
			2: {
				{Year: 50, PollutionIndex: 0.0},
				{Year: 150, PollutionIndex: 2.0},
			},
		}
		_, normalized := m.normalizeAndInterpolate(siteIDs, allData)

		site1 := normalized[1]
		valAt50 := site1[50]
		if math.Abs(valAt50-0.25) > 1e-9 {
			t.Errorf("站点1第50年归一化值=%.6f，期望0.25 (0 + 50/100 * 0.5)", valAt50)
		}
	})

	t.Run("边界外推 - 使用端点值", func(t *testing.T) {
		siteIDs := []int{1, 2}
		allData := map[int][]models.TrendData{
			1: {
				{Year: 100, PollutionIndex: 0.5},
				{Year: 200, PollutionIndex: 1.5},
			},
			2: {
				{Year: 0, PollutionIndex: 0.2},
				{Year: 300, PollutionIndex: 2.0},
			},
		}
		_, normalized := m.normalizeAndInterpolate(siteIDs, allData)
		site1 := normalized[1]
		valAt0 := site1[0]
		if math.Abs(valAt0-0.25) > 1e-9 {
			t.Errorf("站点1第0年(左边界外)值=%.6f，期望0.25 (左端点值归一化后)", valAt0)
		}
	})
}

func TestDetectPeaks(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("单峰检测", func(t *testing.T) {
		siteIDs := []int{1}
		alignedYears := []int{0, 100, 200, 300, 400}
		normalized := map[int]map[int]float64{
			1: {
				0:   0.2,
				100: 0.5,
				200: 0.9,
				300: 0.4,
				400: 0.1,
			},
		}
		siteInfo := map[int]*models.Site{
			1: {ID: 1, Name: "测试遗址", MetalType: "铜"},
		}

		peaks := m.detectPeaks(siteIDs, alignedYears, normalized, siteInfo)
		if len(peaks) < 1 {
			t.Error("应检测到至少1个峰值")
		}
		for _, p := range peaks {
			t.Logf("峰值: 年份=%d, 强度=%.2f, 置信度=%.2f, 金属=%s",
				p.PeakYear, p.PeakValue, p.Confidence, p.MetalType)
		}
	})

	t.Run("多峰检测", func(t *testing.T) {
		siteIDs := []int{1}
		alignedYears := []int{0, 100, 200, 300, 400, 500, 600}
		normalized := map[int]map[int]float64{
			1: {
				0:   0.1,
				100: 0.8,
				200: 0.2,
				300: 0.9,
				400: 0.3,
				500: 0.7,
				600: 0.1,
			},
		}
		siteInfo := map[int]*models.Site{
			1: {ID: 1, Name: "测试遗址", MetalType: "铁"},
		}

		peaks := m.detectPeaks(siteIDs, alignedYears, normalized, siteInfo)
		t.Logf("检测到 %d 个峰值", len(peaks))
		for _, p := range peaks {
			t.Logf("  年份=%d, 强度=%.2f", p.PeakYear, p.PeakValue)
		}
	})

	t.Run("平坡无峰", func(t *testing.T) {
		siteIDs := []int{1}
		alignedYears := []int{0, 100, 200, 300}
		normalized := map[int]map[int]float64{
			1: {
				0:   0.5,
				100: 0.5,
				200: 0.5,
				300: 0.5,
			},
		}
		siteInfo := map[int]*models.Site{}

		peaks := m.detectPeaks(siteIDs, alignedYears, normalized, siteInfo)
		if len(peaks) > 0 {
			t.Errorf("平坡应无峰，但检测到%d个", len(peaks))
		}
	})

	t.Run("数据不足3个点", func(t *testing.T) {
		siteIDs := []int{1}
		alignedYears := []int{0, 100}
		normalized := map[int]map[int]float64{
			1: {
				0:   0.3,
				100: 0.7,
			},
		}
		siteInfo := map[int]*models.Site{}

		peaks := m.detectPeaks(siteIDs, alignedYears, normalized, siteInfo)
		if len(peaks) != 0 {
			t.Errorf("不足3个点应无峰，但检测到%d个", len(peaks))
		}
	})

	t.Run("峰值置信度范围 [0, 1]", func(t *testing.T) {
		siteIDs := []int{1}
		alignedYears := []int{0, 100, 200, 300, 400}
		normalized := map[int]map[int]float64{
			1: {
				0:   0.1,
				100: 0.3,
				200: 0.9,
				300: 0.2,
				400: 0.1,
			},
		}
		siteInfo := map[int]*models.Site{}

		peaks := m.detectPeaks(siteIDs, alignedYears, normalized, siteInfo)
		for _, p := range peaks {
			if p.Confidence < 0 || p.Confidence > 1 {
				t.Errorf("峰值置信度=%.4f，超出 [0,1] 范围", p.Confidence)
			}
		}
	})
}

func TestClusterPeaks(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("单峰聚类", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{PeakYear: 1000, PeakValue: 0.8},
		}
		clusters := m.clusterPeaks(peaks)
		if len(clusters) != 1 {
			t.Errorf("单峰聚类数=%d，期望1", len(clusters))
		}
		if clusters[0].CenterYear != 1000 {
			t.Errorf("聚类中心年份=%d，期望1000", clusters[0].CenterYear)
		}
	})

	t.Run("相近年份 → 同一聚类", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{PeakYear: 1000, PeakValue: 0.8},
			{PeakYear: 1050, PeakValue: 0.7},
			{PeakYear: 1080, PeakValue: 0.6},
		}
		clusters := m.clusterPeaks(peaks)
		if len(clusters) != 1 {
			t.Errorf("相近年份聚类数=%d，期望1", len(clusters))
		}
		if len(clusters[0].Peaks) != 3 {
			t.Errorf("聚类内峰数=%d，期望3", len(clusters[0].Peaks))
		}
	})

	t.Run("远隔年份 → 不同聚类", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{PeakYear: 500, PeakValue: 0.8},
			{PeakYear: 1500, PeakValue: 0.7},
			{PeakYear: 3000, PeakValue: 0.6},
		}
		clusters := m.clusterPeaks(peaks)
		if len(clusters) != 3 {
			t.Errorf("远隔年份聚类数=%d，期望3", len(clusters))
		}
	})

	t.Run("空输入", func(t *testing.T) {
		clusters := m.clusterPeaks([]models.TimelinePeak{})
		if clusters != nil {
			t.Error("空输入应返回nil")
		}
	})

	t.Run("聚类中心为平均年份", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{PeakYear: 1000, PeakValue: 0.8},
			{PeakYear: 1050, PeakValue: 0.8},
			{PeakYear: 1100, PeakValue: 0.8},
		}
		clusters := m.clusterPeaks(peaks)
		expectedCenter := (1000 + 1050 + 1100) / 3
		if clusters[0].CenterYear != expectedCenter {
			t.Errorf("聚类中心=%d，期望%d", clusters[0].CenterYear, expectedCenter)
		}
	})
}

func TestInterpolateAtYear(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("精确匹配年份", func(t *testing.T) {
		alignedYears := []int{0, 100, 200}
		siteData := map[int]float64{
			0:   0.0,
			100: 1.0,
			200: 0.5,
		}
		val, ok := m.interpolateAtYear(100, alignedYears, siteData)
		if !ok {
			t.Error("精确匹配应返回ok=true")
		}
		if math.Abs(val-1.0) > 1e-9 {
			t.Errorf("插值=%.6f，期望1.0", val)
		}
	})

	t.Run("线性插值", func(t *testing.T) {
		alignedYears := []int{0, 100, 200}
		siteData := map[int]float64{
			0:   0.0,
			100: 1.0,
			200: 0.0,
		}
		val, ok := m.interpolateAtYear(50, alignedYears, siteData)
		if !ok {
			t.Error("插值应成功")
		}
		if math.Abs(val-0.5) > 1e-9 {
			t.Errorf("中点插值=%.6f，期望0.5", val)
		}
	})

	t.Run("左边界外推", func(t *testing.T) {
		alignedYears := []int{100, 200}
		siteData := map[int]float64{
			100: 1.0,
			200: 2.0,
		}
		val, ok := m.interpolateAtYear(50, alignedYears, siteData)
		if !ok {
			t.Error("左边界外推应成功（返回左端点）")
		}
		if math.Abs(val-1.0) > 1e-9 {
			t.Errorf("左边界外插值=%.6f，期望1.0（左端点值）", val)
		}
	})

	t.Run("右边界外推", func(t *testing.T) {
		alignedYears := []int{100, 200}
		siteData := map[int]float64{
			100: 1.0,
			200: 2.0,
		}
		val, ok := m.interpolateAtYear(300, alignedYears, siteData)
		if !ok {
			t.Error("右边界外推应成功")
		}
		if math.Abs(val-2.0) > 1e-9 {
			t.Errorf("右边界外插值=%.6f，期望2.0（右端点值）", val)
		}
	})
}

func TestComputeGroupAverage(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("单组平均值", func(t *testing.T) {
		siteIDs := []int{1, 2}
		gridYears := []int{0, 50, 100}
		alignedYears := []int{0, 100}
		normalized := map[int]map[int]float64{
			1: {0: 0.2, 100: 0.8},
			2: {0: 0.4, 100: 0.6},
		}
		result := m.computeGroupAverage(siteIDs, gridYears, alignedYears, normalized)
		if len(result) != 3 {
			t.Errorf("结果长度=%d，期望3", len(result))
		}
		expected0 := (0.2 + 0.4) / 2.0
		if math.Abs(result[0]-expected0) > 1e-9 {
			t.Errorf("第0年平均=%.6f，期望%.6f", result[0], expected0)
		}
		t.Logf("组平均: %v", result)
	})

	t.Run("空站点组", func(t *testing.T) {
		siteIDs := []int{}
		gridYears := []int{0, 100}
		alignedYears := []int{0, 100}
		normalized := map[int]map[int]float64{}
		result := m.computeGroupAverage(siteIDs, gridYears, alignedYears, normalized)
		if len(result) != 2 {
			t.Errorf("空组结果长度=%d，期望2", len(result))
		}
		for _, v := range result {
			if v != 0 {
				t.Errorf("空组平均值=%.6f，期望0", v)
			}
		}
	})
}

func TestFormatYearRange(t *testing.T) {
	m := NewTimelineCompareModule()

	tests := []struct {
		start    int
		end      int
		contains string
	}{
		{-500, 0, "公元前"},
		{0, 500, "公元"},
		{-1000, -500, "公元前"},
		{1000, 1500, "公元"},
	}

	for _, tt := range tests {
		result := m.formatYearRange(tt.start, tt.end)
		if result == "" {
			t.Error("年代范围字符串为空")
		}
		t.Logf("%d - %d → %s", tt.start, tt.end, result)
	}
}

func TestMapPeaksToEpochs(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("峰值分配到正确文明时期", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{SiteID: 1, PeakYear: -3000, PeakValue: 0.6},
			{SiteID: 1, PeakYear: -1500, PeakValue: 0.8},
			{SiteID: 2, PeakYear: 500, PeakValue: 0.7},
			{SiteID: 3, PeakYear: 1800, PeakValue: 0.9},
		}
		siteInfo := map[int]*models.Site{
			1: {ID: 1, Name: "遗址1"},
			2: {ID: 2, Name: "遗址2"},
			3: {ID: 3, Name: "遗址3"},
		}

		epochs := m.mapPeaksToEpochs(peaks, siteInfo)
		if len(epochs) == 0 {
			t.Error("文明时期列表为空")
		}

		totalPeaks := 0
		for _, ep := range epochs {
			totalPeaks += ep.PeakCount
			t.Logf("%s: %d个峰值, 平均强度=%.2f", ep.EpochName, ep.PeakCount, ep.AvgIntensity)
		}
		if totalPeaks != 4 {
			t.Errorf("所有时期总峰值数=%d，期望4", totalPeaks)
		}
	})

	t.Run("无峰值 → 所有时期peak_count=0", func(t *testing.T) {
		peaks := []models.TimelinePeak{}
		siteInfo := map[int]*models.Site{}
		epochs := m.mapPeaksToEpochs(peaks, siteInfo)
		for _, ep := range epochs {
			if ep.PeakCount != 0 {
				t.Errorf("%s 峰值数=%d，期望0", ep.EpochName, ep.PeakCount)
			}
		}
	})
}

func TestComputeGlobalTrend(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("多站点全球趋势计算", func(t *testing.T) {
		siteIDs := []int{1, 2, 3}
		alignedYears := []int{-2000, -1000, 0, 1000}
		normalized := map[int]map[int]float64{
			1: {-2000: 0.3, -1000: 0.5, 0: 0.7, 1000: 0.9},
			2: {-2000: 0.1, -1000: 0.4, 0: 0.6, 1000: 0.8},
			3: {-2000: 0.2, -1000: 0.3, 0: 0.5, 1000: 0.7},
		}
		siteInfo := map[int]*models.Site{
			1: {ID: 1, MetalType: "铜"},
			2: {ID: 2, MetalType: "铁"},
			3: {ID: 3, MetalType: "铅"},
		}

		globalTrend := m.computeGlobalTrend(siteIDs, alignedYears, normalized, siteInfo)

		requiredKeys := []string{"years", "all_sites_avg", "copper_sites", "iron_sites",
			"silver_sites", "lead_sites", "mercury_sites"}
		for _, key := range requiredKeys {
			if _, ok := globalTrend[key]; !ok {
				t.Errorf("全球趋势缺少 %s", key)
			}
		}

		allAvg := globalTrend["all_sites_avg"]
		years := globalTrend["years"]
		if len(allAvg) != len(years) {
			t.Errorf("平均趋势长度=%d，年份长度=%d，应相等", len(allAvg), len(years))
		}

		t.Logf("全球趋势年份范围: %.0f - %.0f, 共%d个点",
			years[0], years[len(years)-1], len(years))
		t.Logf("铜站点数: %d, 铁站点数: %d, 铅站点数: %d",
			len(globalTrend["copper_sites"]) > 0,
			len(globalTrend["iron_sites"]) > 0,
			len(globalTrend["lead_sites"]) > 0)
	})

	t.Run("空数据 → 空趋势", func(t *testing.T) {
		siteIDs := []int{}
		alignedYears := []int{}
		normalized := map[int]map[int]float64{}
		siteInfo := map[int]*models.Site{}

		globalTrend := m.computeGlobalTrend(siteIDs, alignedYears, normalized, siteInfo)
		if len(globalTrend["years"]) != 0 {
			t.Errorf("空数据趋势年份数=%d，期望0", len(globalTrend["years"]))
		}
	})
}

func TestTimelineAlignmentAccuracy(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("不同时间分辨率站点对齐", func(t *testing.T) {
		siteIDs := []int{1, 2, 3}
		allData := map[int][]models.TrendData{
			1: {
				{Year: -3000, PollutionIndex: 0.2},
				{Year: -2000, PollutionIndex: 0.5},
				{Year: -1000, PollutionIndex: 0.8},
				{Year: 0, PollutionIndex: 1.0},
			},
			2: {
				{Year: -2500, PollutionIndex: 0.3},
				{Year: -1500, PollutionIndex: 0.6},
				{Year: -500, PollutionIndex: 0.9},
				{Year: 500, PollutionIndex: 0.7},
			},
			3: {
				{Year: -2000, PollutionIndex: 0.4},
				{Year: 0, PollutionIndex: 0.8},
				{Year: 2000, PollutionIndex: 0.5},
			},
		}

		alignedYears, normalized := m.normalizeAndInterpolate(siteIDs, allData)

		yearSet := make(map[int]bool)
		for _, y := range alignedYears {
			yearSet[y] = true
		}
		expectedYears := []int{-3000, -2500, -2000, -1500, -1000, -500, 0, 500, 2000}
		for _, y := range expectedYears {
			if !yearSet[y] {
				t.Errorf("对齐后应包含年份 %d", y)
			}
		}

		for id, data := range normalized {
			if len(data) != len(alignedYears) {
				t.Errorf("站点%d 数据点=%d，对齐年份数=%d，应相等",
					id, len(data), len(alignedYears))
			}
		}

		t.Logf("对齐后年份数: %d", len(alignedYears))
	})
}

func TestGlobalSmeltingHistoryInference(t *testing.T) {
	m := NewTimelineCompareModule()

	t.Run("多文明时期峰值分布合理性", func(t *testing.T) {
		peaks := []models.TimelinePeak{
			{SiteID: 1, SiteName: "美索不达米亚", PeakYear: -3500, PeakValue: 0.6, MetalType: "铜"},
			{SiteID: 2, SiteName: "古埃及", PeakYear: -3000, PeakValue: 0.7, MetalType: "铜"},
			{SiteID: 3, SiteName: "商周", PeakYear: -1200, PeakValue: 0.9, MetalType: "青铜"},
			{SiteID: 4, SiteName: "古希腊", PeakYear: -800, PeakValue: 0.8, MetalType: "铁"},
			{SiteID: 5, SiteName: "古罗马", PeakYear: 100, PeakValue: 0.85, MetalType: "铁"},
			{SiteID: 6, SiteName: "英国工业革命", PeakYear: 1850, PeakValue: 1.0, MetalType: "铁"},
		}
		siteInfo := map[int]*models.Site{
			1: {ID: 1, Name: "美索不达米亚"},
			2: {ID: 2, Name: "古埃及"},
			3: {ID: 3, Name: "商周"},
			4: {ID: 4, Name: "古希腊"},
			5: {ID: 5, Name: "古罗马"},
			6: {ID: 6, Name: "工业革命"},
		}

		epochs := m.mapPeaksToEpochs(peaks, siteInfo)

		t.Log("全球冶炼史脉络推断:")
		for _, ep := range epochs {
			if ep.PeakCount > 0 {
				t.Logf("  %s (%s): %d个冶炼高峰, 平均强度=%.2f",
					ep.EpochName, ep.YearRange, ep.PeakCount, ep.AvgIntensity)
				if len(ep.KeySites) > 0 {
					t.Logf("    关键遗址: %v", ep.KeySites)
				}
			}
		}

		hasCopperAge := false
		hasIronAge := false
		hasIndustrial := false
		for _, ep := range epochs {
			if ep.PeakCount > 0 {
				switch {
				case ep.YearStart < -2000:
					hasCopperAge = true
				case ep.YearStart < 0 && ep.YearEnd > -1000:
					hasIronAge = true
				case ep.YearStart > 1500:
					hasIndustrial = true
				}
			}
		}
		if !hasCopperAge {
			t.Log("注意：未检测到铜石并用/青铜时代峰值")
		}
		if !hasIronAge {
			t.Log("注意：未检测到铁器时代峰值")
		}
		if !hasIndustrial {
			t.Log("注意：未检测到工业革命时期峰值")
		}
	})
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
		{1000, "1000"},
	}

	for _, tt := range tests {
		result := intToStr(tt.n)
		if result != tt.expected {
			t.Errorf("intToStr(%d) = %s，期望 %s", tt.n, result, tt.expected)
		}
	}
}
