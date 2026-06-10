package main

import (
	"log"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/database"
	"archaeology-pollution-system/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// =========================================
// 系统入口 — 模块化架构
//
// 模块划分（通过 EventBus Channel 解耦通信）：
//   XRFReceiver         - 数据接收与入库
//   FingerprintAnalyzer - PCA聚类 + 指纹识别
//   RemediationAdvisor  - AHP + 熵权法 + TOPSIS 多属性决策
//   AlarmMailer         - 告警检测 + 聚合邮件
//
// 事件流：
//   XRF数据上报 → [XRFReceived] → 自动触发指纹/修复/告警模块
// =========================================

func main() {
	config.LoadConfig()

	database.Connect()
	defer database.Close()

	// 初始化Handler — 内部创建4个业务模块 + EventBus
	handler := handlers.NewHandler()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

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
	}

	r.Static("/frontend", "./../frontend")
	r.StaticFile("/", "./../frontend/index.html")

	log.Printf("============================================")
	log.Printf(" Archaeology Pollution System Starting")
	log.Printf(" Port: %s", config.AppConfig.ServerPort)
	log.Printf(" Modules: XRFReceiver | FingerprintAnalyzer | RemediationAdvisor | AlarmMailer")
	log.Printf(" Communication: EventBus (Go Channels)")
	log.Printf("============================================")

	if err := r.Run(":" + config.AppConfig.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
