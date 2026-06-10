package main

import (
	"log"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/database"
	"archaeology-pollution-system/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadConfig()
	database.Connect()
	defer database.Close()

	h := handlers.NewHandler()

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
		api.GET("/stats", h.GetDashboardStats)

		api.GET("/sites", h.GetSites)
		api.GET("/sites/:id", h.GetSite)
		api.GET("/sites/:id/trend", h.GetSiteTrend)
		api.POST("/xrf", h.CreateXRFMeasurement)

		api.GET("/fingerprints", h.GetFingerprints)
		api.GET("/sites/:id/fingerprint", h.MatchFingerprint)
		api.GET("/pca", h.PerformPCA)

		api.GET("/technologies", h.GetTechnologies)
		api.GET("/sites/:id/remediation", h.AssessRemediation)

		api.GET("/standards", h.GetRiskStandards)
		api.GET("/alerts", h.GetAlerts)
	}

	r.Static("/frontend", "./../frontend")
	r.StaticFile("/", "./../frontend/index.html")

	log.Printf("Server starting on port %s...", config.AppConfig.ServerPort)
	if err := r.Run(":" + config.AppConfig.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
