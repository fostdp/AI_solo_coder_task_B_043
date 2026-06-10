package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// =========================================
// 配置结构
// =========================================

type Config struct {
	APIBase         string
	NumSites        int
	ReportInterval  int    // 持续上报间隔（秒）
	StartYear       int
	HistoryYears    int
	Continuous      bool
	Verbose         bool
	ProfileOverride string // 污染特征注入
}

// =========================================
// 数据结构
// =========================================

type Site struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	MetalType string  `json:"metal_type"`
	Scale     string  `json:"scale"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Country   string  `json:"country"`
}

type XRFMeasurement struct {
	SiteID                 int     `json:"site_id"`
	SampleDepth            string  `json:"sample_depth"`
	MeasurementYear        int     `json:"measurement_year"`
	Pb                     float64 `json:"pb"`
	Zn                     float64 `json:"zn"`
	Cu                     float64 `json:"cu"`
	As                     float64 `json:"as"`
	Hg                     float64 `json:"hg"`
	Cd                     float64 `json:"cd"`
	PH                     float64 `json:"ph"`
	OrganicMatter          float64 `json:"organic_matter"`
	CationExchangeCapacity float64 `json:"cation_exchange_capacity"`
	SoilType               string  `json:"soil_type"`
	SoilMoisture           float64 `json:"soil_moisture"`
	Remark                 string  `json:"remark"`
}

type MetalProfile struct {
	PbBase float64 `json:"pb_base"`
	ZnBase float64 `json:"zn_base"`
	CuBase float64 `json:"cu_base"`
	AsBase float64 `json:"as_base"`
	HgBase float64 `json:"hg_base"`
	CdBase float64 `json:"cd_base"`
}

// =========================================
// 全局变量
// =========================================

var (
	// 不同冶炼金属的污染特征基线
	metalProfiles = map[string]MetalProfile{
		"铜": {PbBase: 450, ZnBase: 800, CuBase: 12000, AsBase: 120, HgBase: 3, CdBase: 15},
		"铁": {PbBase: 280, ZnBase: 1500, CuBase: 600, AsBase: 90, HgBase: 2, CdBase: 8},
		"银": {PbBase: 3500, ZnBase: 600, CuBase: 350, AsBase: 180, HgBase: 45, CdBase: 12},
		"铅": {PbBase: 8000, ZnBase: 1200, CuBase: 200, AsBase: 80, HgBase: 5, CdBase: 25},
		"汞": {PbBase: 350, ZnBase: 400, CuBase: 180, AsBase: 250, HgBase: 320, CdBase: 5},
	}

	scaleMultipliers = map[string]float64{
		"小型":   0.4,
		"中型":   0.7,
		"大型":   1.0,
		"超大型": 1.6,
	}

	soilTypes = []string{"壤土", "砂壤土", "砂土", "粘土", "粉土"}

	// 30个内置遗址（与 init.sql 一一对应）
	builtinSites = []Site{
		// 铜冶炼遗址 (9)
		{1, "蒂尔曼遗址", "铜", "超大型", 35.45, 30.32, "约旦"},
		{2, "提姆纳河谷", "铜", "大型", 34.98, 29.73, "以色列"},
		{3, "法尤姆遗址", "铜", "中型", 30.83, 29.31, "埃及"},
		{4, "萨尔茨堡附近矿区", "铜", "大型", 13.05, 47.80, "奥地利"},
		{5, "法伦铜矿", "铜", "超大型", 15.62, 60.60, "瑞典"},
		{6, "基律纳矿区", "铜", "大型", 20.22, 67.85, "瑞典"},
		{7, "大冶铜绿山", "铜", "超大型", 114.93, 30.09, "中国"},
		{8, "瑞昌铜岭", "铜", "大型", 115.62, 29.68, "中国"},
		{9, "中条山矿区", "铜", "大型", 111.55, 35.41, "中国"},
		// 铁冶炼遗址 (9)
		{10, "科里亚遗址", "铁", "大型", 8.78, 9.83, "尼日利亚"},
		{11, "梅罗伊遗址", "铁", "超大型", 33.78, 16.93, "苏丹"},
		{12, "德尔菲遗址", "铁", "中型", 22.50, 38.48, "希腊"},
		{13, "鲁尔工业区遗址", "铁", "超大型", 7.15, 51.43, "德国"},
		{14, "铁桥谷", "铁", "大型", -2.48, 52.62, "英国"},
		{15, "赫兰钢铁遗址", "铁", "大型", 7.57, 51.26, "德国"},
		{16, "徐州利国驿", "铁", "超大型", 117.46, 34.37, "中国"},
		{17, "巩义铁生沟", "铁", "大型", 113.02, 34.70, "中国"},
		{18, "南阳宛城冶铁", "铁", "大型", 112.55, 33.01, "中国"},
		// 银冶炼遗址 (5)
		{19, "拉乌里科查遗址", "银", "超大型", -76.58, -14.72, "秘鲁"},
		{20, "萨卡特卡斯", "银", "大型", -102.58, 22.77, "墨西哥"},
		{21, "波托西", "银", "超大型", -65.76, -19.59, "玻利维亚"},
		{22, "弗莱贝格", "银", "大型", 13.34, 50.92, "德国"},
		{23, "库特纳霍拉", "银", "大型", 15.26, 49.95, "捷克"},
		// 铅冶炼遗址 (3)
		{24, "萨德伯里", "铅", "中型", -1.98, 52.89, "英国"},
		{25, "门迪普山区", "铅", "大型", -2.75, 51.28, "英国"},
		{26, "洛林矿区", "铅", "大型", 6.18, 49.11, "法国"},
		// 汞冶炼遗址 (4)
		{27, "阿尔马登", "汞", "超大型", -4.84, 38.77, "西班牙"},
		{28, "伊德里亚", "汞", "大型", 14.03, 46.00, "斯洛文尼亚"},
		{29, "新阿尔马登", "汞", "大型", -121.05, 37.13, "美国"},
		{30, "万山汞矿", "汞", "超大型", 109.20, 27.53, "中国"},
	}
)

var cfg Config

// =========================================
// 主函数
// =========================================

func main() {
	loadConfig()
	printBanner()

	// 等待后端就绪
	waitForServer()

	// 获取遗址列表（优先从后端获取，失败则使用内置）
	sites := fetchSites()
	if len(sites) == 0 {
		log.Println("[WARN] 无法从后端获取遗址，使用内置 30 个遗址配置")
		sites = builtinSites
	}

	// 限制遗址数量
	if cfg.NumSites > 0 && cfg.NumSites < len(sites) {
		sites = sites[:cfg.NumSites]
	}
	log.Printf("使用 %d 个遗址进行模拟\n", len(sites))

	currentYear := time.Now().Year()

	// 阶段1：生成历史数据
	if cfg.HistoryYears > 0 {
		log.Printf("\n[阶段1] 生成 %d 年历史数据 (%d~%d)...",
			cfg.HistoryYears, cfg.StartYear, cfg.StartYear+cfg.HistoryYears-1)
		for year := cfg.StartYear; year < cfg.StartYear+cfg.HistoryYears; year++ {
			success := 0
			for _, site := range sites {
				if submitMeasurement(site, year, cfg.Verbose) {
					success++
				}
			}
			log.Printf("  %d 年: %d/%d 个遗址\n", year, success, len(sites))
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 阶段2：上传最新年度数据
	log.Println("\n[阶段2] 上传最新监测年度数据...")
	success := 0
	for _, site := range sites {
		if submitMeasurement(site, currentYear, cfg.Verbose) {
			success++
		}
	}
	log.Printf("  完成: %d/%d 个遗址\n", success, len(sites))

	// 阶段3：持续上报模式
	if cfg.Continuous {
		log.Printf("\n[阶段3] 启动定期上报模式 (每 %d 秒模拟一次年度数据上报)...",
			cfg.ReportInterval)
		log.Println("  按 Ctrl+C 退出模拟器")
		log.Println()

		ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
		defer ticker.Stop()

		simulatedYear := currentYear + 1
		for range ticker.C {
			log.Printf("\n[%s] 模拟上报 %d 年度数据...",
				time.Now().Format("15:04:05"), simulatedYear)
			success := 0
			for _, site := range sites {
				if submitMeasurement(site, simulatedYear, cfg.Verbose) {
					success++
				}
			}
			log.Printf("  完成: %d/%d 个遗址\n", success, len(sites))
			simulatedYear++
		}
	} else {
		log.Println("\n[完成] 非持续模式，模拟器退出")
	}
}

// =========================================
// 配置加载（环境变量优先）
// =========================================

func loadConfig() {
	cfg = Config{
		APIBase:         getEnv("API_BASE", "http://localhost:8080"),
		NumSites:        getEnvInt("SIM_NUM_SITES", 0), // 0 = 全部
		ReportInterval:  getEnvInt("SIM_REPORT_INTERVAL", 30),
		StartYear:       getEnvInt("SIM_START_YEAR", 2015),
		HistoryYears:    getEnvInt("SIM_HISTORY_YEARS", 10),
		Continuous:      getEnvBool("SIM_CONTINUOUS", true),
		Verbose:         getEnvBool("SIM_VERBOSE", true),
		ProfileOverride: getEnv("SIM_PROFILE_OVERRIDES", ""),
	}

	// 应用污染特征注入
	applyProfileOverrides(cfg.ProfileOverride)
}

// applyProfileOverrides 解析并应用污染特征覆盖
// 格式: 金属类型_属性=值,多个用逗号分隔
// 示例: 铜_Pb=600,铜_Cu=15000,汞_Hg=400
func applyProfileOverrides(override string) {
	if override == "" {
		return
	}

	log.Printf("[配置] 应用污染特征注入: %s\n", override)
	pairs := strings.Split(override, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		valStr := strings.TrimSpace(kv[1])

		// 解析 金属_属性
		parts := strings.SplitN(key, "_", 2)
		if len(parts) != 2 {
			log.Printf("  [WARN] 无效的配置键: %s (格式应为 金属_属性)\n", key)
			continue
		}
		metalType := parts[0]
		attrName := strings.ToUpper(parts[1])

		profile, ok := metalProfiles[metalType]
		if !ok {
			log.Printf("  [WARN] 未知金属类型: %s\n", metalType)
			continue
		}

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			log.Printf("  [WARN] 无效数值: %s\n", valStr)
			continue
		}

		// 应用到对应字段
		switch attrName {
		case "PB", "PBBASE":
			profile.PbBase = val
		case "ZN", "ZNBASE":
			profile.ZnBase = val
		case "CU", "CUBASE":
			profile.CuBase = val
		case "AS", "ASBASE":
			profile.AsBase = val
		case "HG", "HGBASE":
			profile.HgBase = val
		case "CD", "CDBASE":
			profile.CdBase = val
		default:
			log.Printf("  [WARN] 未知属性: %s\n", attrName)
			continue
		}

		metalProfiles[metalType] = profile
		log.Printf("  ✓ %s.%s = %.2f\n", metalType, attrName, val)
	}
}

// =========================================
// 辅助函数
// =========================================

func printBanner() {
	log.Println("=============================================")
	log.Println("  XRF 数据模拟器 - 古代金属冶炼遗址监测系统")
	log.Println("=============================================")
	log.Printf("  API 地址:       %s", cfg.APIBase)
	log.Printf("  遗址数量:       %d (0=全部)", cfg.NumSites)
	log.Printf("  历史起始年:     %d", cfg.StartYear)
	log.Printf("  历史年数:       %d", cfg.HistoryYears)
	log.Printf("  持续上报:       %v", cfg.Continuous)
	log.Printf("  上报间隔:       %d 秒", cfg.ReportInterval)
	log.Printf("  详细日志:       %v", cfg.Verbose)
	if cfg.ProfileOverride != "" {
		log.Printf("  特征注入:       %s", cfg.ProfileOverride)
	}
	log.Println("=============================================")
	log.Println()
}

func waitForServer() {
	log.Println("等待后端服务启动...")
	apiBase := strings.TrimSuffix(cfg.APIBase, "/")
	for i := 0; i < 60; i++ {
		resp, err := http.Get(apiBase + "/api/stats")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Println("后端服务已就绪 ✓")
			return
		}
		time.Sleep(1 * time.Second)
		fmt.Print(".")
	}
	log.Println("\n警告: 无法连接后端服务，但将继续运行...")
}

func fetchSites() []Site {
	apiBase := strings.TrimSuffix(cfg.APIBase, "/")
	resp, err := http.Get(apiBase + "/api/sites")
	if err != nil {
		log.Printf("获取遗址列表失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Data []Site `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("解析遗址数据失败: %v", err)
		return nil
	}
	return result.Data
}

func submitMeasurement(site Site, year int, verbose bool) bool {
	profile := metalProfiles[site.MetalType]
	scaleMult := scaleMultipliers[site.Scale]
	if scaleMult == 0 {
		scaleMult = 1.0
	}

	// 年份因素：随时间推移略有增长
	yearFactor := 0.85 + 0.03*float64(year-cfg.StartYear)
	// 随机因素：±20%
	randomFactor := 0.8 + rand.Float64()*0.4

	measurement := XRFMeasurement{
		SiteID:                 site.ID,
		SampleDepth:            "0-30cm",
		MeasurementYear:        year,
		Pb:                     round4(profile.PbBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		Zn:                     round4(profile.ZnBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		Cu:                     round4(profile.CuBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		As:                     round4(profile.AsBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		Hg:                     round4(profile.HgBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		Cd:                     round4(profile.CdBase * scaleMult * yearFactor * (0.85 + rand.Float64()*0.3)),
		PH:                     round2(6.0 + rand.Float64()*2.5),
		OrganicMatter:          round4(1.5 + rand.Float64()*3.5),
		CationExchangeCapacity: round4(8.0 + rand.Float64()*25.0),
		SoilType:               soilTypes[rand.Intn(len(soilTypes))],
		SoilMoisture:           round2(15 + rand.Float64()*20),
		Remark:                 fmt.Sprintf("XRF自动监测数据_%d_%s", year, site.Name),
	}

	// 随机异常高值（10%概率）
	if randomFactor > 1.18 && rand.Float64() > 0.7 {
		measurement.Pb *= 1.8
		measurement.Cd *= 2.2
		measurement.As *= 1.5
		if verbose {
			log.Printf("    ⚠️  %s: 检测到异常高值 (Pb: %.1f mg/kg)", site.Name, measurement.Pb)
		}
	}

	apiBase := strings.TrimSuffix(cfg.APIBase, "/")
	url := fmt.Sprintf("%s/api/sites/%d/xrf", apiBase, site.ID)

	body, _ := json.Marshal(measurement)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		if verbose {
			log.Printf("    ❌ %s: 上报失败 - %v", site.Name, err)
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		if verbose {
			log.Printf("    ✓ %s (%s/%s): Pb=%.0f Cu=%.0f Hg=%.1f mg/kg",
				site.Name, site.MetalType, site.Scale,
				measurement.Pb, measurement.Cu, measurement.Hg)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if alerts, ok := result["alerts"].([]interface{}); ok && len(alerts) > 0 && verbose {
			log.Printf("       🔔 触发 %d 条告警", len(alerts))
		}
		return true
	}

	if verbose {
		log.Printf("    ❌ %s: HTTP %d", site.Name, resp.StatusCode)
	}
	return false
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}

func round4(val float64) float64 {
	return math.Round(val*10000) / 10000
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}
