package main

import (
	"log"
	"net/http"
	_ "net/http/pprof" // 自动注册 pprof 到 DefaultServeMux

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/database"
	"archaeology-pollution-system/handlers"
	"archaeology-pollution-system/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// =========================================
// 系统入口 — 模块化架构 + 工程化监控
//
// 模块划分（通过 EventBus Channel 解耦通信）：
//   XRFReceiver         - 数据接收与入库
//   FingerprintAnalyzer - PCA聚类 + 指纹识别
//   RemediationAdvisor  - AHP + 熵权法 + TOPSIS 多属性决策
//   AlarmMailer         - 告警检测 + 聚合邮件
//
// 监控端口：
//   :8080 - 业务 API
//   :6060 - pprof 性能剖析（/debug/pprof）
//   :2112 - Prometheus 抓取（/metrics，Go 生态标准端口）
//
// 事件流：
//   XRF数据上报 → [XRFReceived] → 自动触发指纹/修复/告警模块
// =========================================

func main() {
	config.LoadConfig()

	database.Connect()
	defer database.Close()

	// 启动 pprof 性能剖析（单独端口，不影响业务）
	// 访问: http://localhost:6060/debug/pprof/
	go func() {
		log.Printf("[pprof] Performance profiling available on :6060/debug/pprof/")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("[pprof] WARNING: Failed to start pprof server: %v", err)
		}
	}()

	// 启动 Prometheus 指标抓取（单独端口，Go 生态标准端口 2112）
	// 访问: http://localhost:2112/metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Printf("[prometheus] Metrics endpoint available on :2112/metrics")
		if err := http.ListenAndServe(":2112", mux); err != nil {
			log.Printf("[prometheus] WARNING: Failed to start metrics server: %v", err)
		}
	}()

	// 初始化Handler — 内部创建4个业务模块 + EventBus
	handler := handlers.NewHandler()

	r := gin.Default()

	// Prometheus 指标中间件（全局生效）
	r.Use(middleware.PrometheusMetrics())

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 健康检查（供 K8s/Docker 就绪探针）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": config.AppConfig.ServerPort,
		})
	})

	api := r.Group("/api")
	{
		// ====== 系统概览 ======
		api.GET("/stats", handler.GetStats)

		// ====== 遗址管理 ======
		api.GET("/sites", handler.GetSites)
		api.GET("/sites/:id", handler.GetSite)
		api.GET("/sites/:id/trend", handler.GetSiteTrend)

		// ====== XRF数据（XRFReceiver模块） ======
		api.POST("/sites/:id/xrf", handler.CreateXRFMeasurement)
		api.POST("/xrf", func(c *gin.Context) {
			c.JSON(400, gin.H{"error": "use POST /api/sites/:id/xrf"})
		})

		// ====== 指纹识别（FingerprintAnalyzer模块） ======
		api.GET("/fingerprints", handler.GetStats)
		api.GET("/fingerprint/pca", handler.PerformPCA)
		api.GET("/pca", handler.PerformPCA)
		api.GET("/sites/:id/fingerprint", handler.MatchFingerprint)

		// ====== 修复技术（RemediationAdvisor模块） ======
		api.GET("/technologies", handler.GetRemediationTechnologies)
		api.GET("/remediation/technologies", handler.GetRemediationTechnologies)
		api.GET("/sites/:id/remediation", handler.AssessRemediation)

		// ====== 风险标准 ======
		api.GET("/standards", func(c *gin.Context) {
			c.JSON(200, gin.H{"data": config.PollutionStandards})
		})

		// ====== 告警系统（AlarmMailer模块） ======
		api.GET("/alerts", handler.GetAlerts)
		api.POST("/sites/:id/check-alerts", handler.CheckAndCreateAlerts)
		api.POST("/alerts/flush", handler.SendPendingAlerts)

		// ====== 冶炼工艺反演（ProcessInversionModule） ======
		api.GET("/sites/:id/smelting-inversion", handler.InvertSmeltingProcess)

		// ====== 农田土壤安全评估（FarmSafetyModule） ======
		api.GET("/sites/:id/farm-safety", handler.AssessFarmSafety)

		// ====== 矿渣资源化利用（SlagRecycleModule） ======
		api.GET("/sites/:id/slag-recycle", handler.AssessSlagRecycle)

		// ====== 多遗址时间线对比（TimelineCompareModule） ======
		api.GET("/timeline/compare", handler.CompareTimelines)
	}

	r.Static("/frontend", "./../frontend")
	r.StaticFile("/", "./../frontend/index.html")

	log.Printf("============================================")
	log.Printf(" Archaeology Pollution System Starting")
	log.Printf(" Port: %s (API)", config.AppConfig.ServerPort)
	log.Printf(" Port: 6060 (pprof)")
	log.Printf(" Port: 2112 (prometheus metrics)")
	log.Printf(" Modules: XRFReceiver | FingerprintAnalyzer | RemediationAdvisor | AlarmMailer")
	log.Printf(" New Modules: ProcessInversion | FarmSafety | SlagRecycle | TimelineCompare")
	log.Printf(" Communication: EventBus (Go Channels)")
	log.Printf("============================================")

	if err := r.Run(":" + config.AppConfig.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
