package handlers

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
	"archaeology-pollution-system/services"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	fingerprintService  *services.FingerprintService
	remediationService  *services.RemediationService
	alertService        *services.AlertService
}

func NewHandler() *Handler {
	return &Handler{
		fingerprintService: services.NewFingerprintService(),
		remediationService: services.NewRemediationService(),
		alertService:       services.NewAlertService(),
	}
}

func (h *Handler) GetSites(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sites, err := repository.GetAllSitesWithPollution(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sites)
}

func (h *Handler) GetSite(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid site ID"})
		return
	}

	site, err := repository.GetSiteByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found"})
		return
	}
	c.JSON(http.StatusOK, site)
}

func (h *Handler) GetSiteTrend(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid site ID"})
		return
	}

	measurements, err := repository.GetXRFMeasurementsBySite(ctx, id, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sort.Slice(measurements, func(i, j int) bool {
		return measurements[i].MeasurementYear < measurements[j].MeasurementYear
	})

	trendData := make([]models.TrendData, 0, len(measurements))
	for _, m := range measurements {
		trendData = append(trendData, models.TrendData{
			Year: m.MeasurementYear,
			Metals: map[string]float64{
				"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
				"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
			},
			PollutionIndex: repository.CalculatePollutionIndex(m.Pb, m.Zn, m.Cu, m.As, m.Hg, m.Cd),
		})
	}

	c.JSON(http.StatusOK, trendData)
}

func (h *Handler) CreateXRFMeasurement(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var m models.XRFMeasurement
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := repository.InsertXRFMeasurement(ctx, &m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	alerts, err := h.alertService.CheckAndCreateAlerts(ctx, &m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(alerts) > 0 {
		go h.alertService.SendAlerts(ctx, alerts)
	}

	c.JSON(http.StatusCreated, gin.H{
		"measurement": m,
		"alerts":      alerts,
	})
}

func (h *Handler) GetFingerprints(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	fingerprints, err := repository.GetAllPollutionFingerprints(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, fingerprints)
}

func (h *Handler) MatchFingerprint(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid site ID"})
		return
	}

	result, err := h.fingerprintService.MatchFingerprint(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) PerformPCA(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	sites, err := repository.GetAllSitesWithPollution(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	validSites := make([]models.SiteWithPollution, 0)
	for _, s := range sites {
		if s.Pb > 0 || s.Zn > 0 || s.Cu > 0 || s.As > 0 || s.Hg > 0 || s.Cd > 0 {
			validSites = append(validSites, s)
		}
	}

	result, err := h.fingerprintService.PerformPCA(ctx, validSites)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetTechnologies(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	techs, err := repository.GetAllRemediationTechnologies(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, techs)
}

func (h *Handler) AssessRemediation(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid site ID"})
		return
	}

	assessment, err := h.remediationService.AssessRemediation(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, assessment)
}

func (h *Handler) GetRiskStandards(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	standards, err := repository.GetRiskStandards(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, standards)
}

func (h *Handler) GetAlerts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	alerts, err := repository.GetAlerts(ctx, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, alerts)
}

func (h *Handler) GetDashboardStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sites, err := repository.GetAllSitesWithPollution(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalSites := len(sites)
	withData := 0
	lowPollution := 0
	mediumPollution := 0
	highPollution := 0
	severePollution := 0

	metalCounts := map[string]int{"铜": 0, "铁": 0, "银": 0, "铅": 0, "汞": 0}
	scaleCounts := map[string]int{"小型": 0, "中型": 0, "大型": 0, "超大型": 0}

	for _, s := range sites {
		metalCounts[s.MetalType]++
		scaleCounts[s.Scale]++
		if s.LatestYear > 0 {
			withData++
			switch {
			case s.PollutionIndex >= 3.0:
				severePollution++
			case s.PollutionIndex >= 2.0:
				highPollution++
			case s.PollutionIndex >= 1.0:
				mediumPollution++
			default:
				lowPollution++
			}
		}
	}

	alerts, _ := repository.GetAlerts(ctx, 1000)
	alertCounts := map[string]int{"低": 0, "中": 0, "高": 0, "严重": 0}
	for _, a := range alerts {
		alertCounts[a.Severity]++
	}

	c.JSON(http.StatusOK, gin.H{
		"total_sites":      totalSites,
		"sites_with_data":  withData,
		"pollution_levels": gin.H{
			"low":     lowPollution,
			"medium":  mediumPollution,
			"high":    highPollution,
			"severe":  severePollution,
		},
		"metal_distribution": metalCounts,
		"scale_distribution": scaleCounts,
		"alert_summary":      alertCounts,
		"total_alerts":       len(alerts),
	})
}
