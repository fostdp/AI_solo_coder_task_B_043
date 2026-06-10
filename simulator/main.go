package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"
)

type Site struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	MetalType string  `json:"metal_type"`
	Scale     string  `json:"scale"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
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
	Remark                 string  `json:"remark"`
}

type MetalProfile struct {
	PbBase float64
	ZnBase float64
	CuBase float64
	AsBase float64
	HgBase float64
	CdBase float64
}

var metalProfiles = map[string]MetalProfile{
	"铜": {PbBase: 450, ZnBase: 800, CuBase: 12000, AsBase: 120, HgBase: 3, CdBase: 15},
	"铁": {PbBase: 280, ZnBase: 1500, CuBase: 600, AsBase: 90, HgBase: 2, CdBase: 8},
	"银": {PbBase: 3500, ZnBase: 600, CuBase: 350, AsBase: 180, HgBase: 45, CdBase: 12},
	"铅": {PbBase: 8000, ZnBase: 1200, CuBase: 200, AsBase: 80, HgBase: 5, CdBase: 25},
	"汞": {PbBase: 350, ZnBase: 400, CuBase: 180, AsBase: 250, HgBase: 320, CdBase: 5},
}

var scaleMultipliers = map[string]float64{
	"小型":   0.4,
	"中型":   0.7,
	"大型":   1.0,
	"超大型": 1.6,
}

var soilTypes = []string{"壤土", "砂壤土", "砂土", "粘土", "粉土"}

const apiBase = "http://localhost:8080/api"

func main() {
	log.Println("=============================================")
	log.Println("  XRF 数据模拟器 - 古代金属冶炼遗址监测系统")
	log.Println("=============================================")
	log.Println()

	waitForServer()

	sites := fetchSites()
	if len(sites) == 0 {
		log.Fatal("未获取到遗址数据，请确保数据库已初始化并导入了遗址数据")
	}
	log.Printf("成功获取 %d 个遗址信息\n", len(sites))

	currentYear := time.Now().Year()

	log.Println("\n[阶段1] 生成近10年历史数据 (2015~2024)...")
	for year := currentYear - 10; year < currentYear; year++ {
		log.Printf("  正在生成 %d 年数据...", year)
		success := 0
		for _, site := range sites {
			if submitMeasurement(site, year, false) {
				success++
			}
		}
		log.Printf("    完成: %d/%d 个遗址\n", success, len(sites))
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("\n[阶段2] 上传最新监测年度数据...")
	success := 0
	for _, site := range sites {
		if submitMeasurement(site, currentYear, true) {
			success++
		}
	}
	log.Printf("  完成: %d/%d 个遗址\n", success, len(sites))

	log.Println("\n[阶段3] 启动定期上报模式 (每30秒模拟一次年度数据上报)...")
	log.Println("  按 Ctrl+C 退出模拟器")
	log.Println()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	simulatedYear := currentYear + 1
	for range ticker.C {
		log.Printf("\n[%s] 模拟上报 %d 年度数据...", time.Now().Format("15:04:05"), simulatedYear)
		success := 0
		for _, site := range sites {
			if submitMeasurement(site, simulatedYear, true) {
				success++
			}
		}
		log.Printf("  完成: %d/%d 个遗址\n", success, len(sites))
		simulatedYear++
	}
}

func waitForServer() {
	log.Println("等待后端服务启动...")
	for i := 0; i < 30; i++ {
		resp, err := http.Get(apiBase + "/stats")
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
	resp, err := http.Get(apiBase + "/sites")
	if err != nil {
		log.Printf("获取遗址列表失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var sites []Site
	if err := json.NewDecoder(resp.Body).Decode(&sites); err != nil {
		log.Printf("解析遗址数据失败: %v", err)
		return nil
	}
	return sites
}

func submitMeasurement(site Site, year int, verbose bool) bool {
	profile := metalProfiles[site.MetalType]
	scaleMult := scaleMultipliers[site.Scale]
	if scaleMult == 0 {
		scaleMult = 1.0
	}

	yearFactor := 0.85 + 0.03*float64(year-2015)
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
		Remark:                 fmt.Sprintf("XRF自动监测数据_%d_%s", year, site.Name),
	}

	if randomFactor > 1.25 && rand.Float64() > 0.6 {
		measurement.Pb *= 1.8
		measurement.Cd *= 2.2
		measurement.As *= 1.5
		if verbose {
			log.Printf("    ⚠️  %s: 检测到异常高值 (Pb: %.1f mg/kg)", site.Name, measurement.Pb)
		}
	}

	body, _ := json.Marshal(measurement)
	resp, err := http.Post(apiBase+"/xrf", "application/json", bytes.NewBuffer(body))
	if err != nil {
		if verbose {
			log.Printf("    ❌ %s: 上报失败 - %v", site.Name, err)
		}
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		if verbose {
			log.Printf("    ✓ %s (%s): Pb=%.0f Cu=%.0f Hg=%.1f mg/kg",
				site.Name, site.MetalType, measurement.Pb, measurement.Cu, measurement.Hg)
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

func round4(val float64) float64 {
	return math.Round(val*10000) / 10000
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}
