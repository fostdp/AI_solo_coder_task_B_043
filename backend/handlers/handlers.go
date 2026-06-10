package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules"
	"archaeology-pollution-system/repository"

	"github.com/gin-gonic/gin"
)

// Handler 聚合所有业务模块
// 各模块之间通过 EventBus 以 Channel 方式解耦通信
type Handler struct {
	xrf        *modules.XRFReceiver
	fingerprint *modules.FingerprintAnalyzer
	remediation *modules.RemediationAdvisor
	alarm      *modules.AlarmMailer
}

// NewHandler 初始化所有业务模块
// 各模块启动时自动订阅 EventBus 中的事件
func NewHandler() *Handler {
	h := &Handler{
		xrf:         modules.NewXRFReceiver(),
		fingerprint: modules.NewFingerprintAnalyzer(),
		remediation: modules.NewRemediationAdvisor(),
		alarm:       modules.NewAlarmMailer(),
	}
	log.Println("[Handlers] All modules initialized via EventBus channel communication")
	return h
}

// ====================================
// 遗址管理
// ====================================

// GetSites 获取所有遗址（含污染指数）
// GET /api/sites
func (h *Handler) GetSites(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sites, err := repository.GetSitesWithPollution(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sites})
}

// GetSite 获取单个遗址详情
// GET /api/sites/:id
func (h *Handler) GetSite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	site, err := repository.GetSite(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": site})
}

// ====================================
// XRF数据接收
// ====================================

// GetSiteTrend 获取遗址XRF趋势数据（近10年）
// GET /api/sites/:id/trend
func (h *Handler) GetSiteTrend(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	trend, err := h.xrf.GetTrend(ctx, id, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"site_id": id,
		"limit":   10,
		"order":   "ASC",
		"data":    trend,
	})
}

// CreateXRFMeasurement 接收XRF数据（核心入口）
// 数据流：入库 → EventBus[XRFReceived] →
//         FingerprintAnalyzer 订阅 → 指纹分析
//         RemediationAdvisor 订阅 → 修复评估
//         AlarmMailer 订阅 → 告警检测+邮件
// POST /api/sites/:id/xrf
func (h *Handler) CreateXRFMeasurement(c *gin.Context) {
	siteID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}

	var m models.XRFMeasurement
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m.SiteID = siteID

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	savedM, _, err := h.xrf.Receive(ctx, &m)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	site, _ := repository.GetSite(ctx, siteID)

	// Channel 模式：显式触发告警检测（模块也通过EventBus订阅了）
	alerts, alertErr := h.alarm.CheckAndCreate(ctx, *savedM, site)

	// 异步发送邮件（不阻塞HTTP响应）
	if len(alerts) > 0 {
		go func() {
			sendCtx := context.Background()
			if err := h.alarm.Send(sendCtx, alerts); err != nil {
				log.Printf("[Handlers] Async alert send warning: %v", err)
			}
		}()
	}

	response := gin.H{
		"data":  savedM,
		"alerts": gin.H{"count": len(alerts), "items": alerts},
	}
	if alertErr != nil {
		response["alert_error"] = alertErr.Error()
	}
	c.JSON(http.StatusCreated, response)
}

// ====================================
// 指纹识别（FingerprintAnalyzer模块）
// ====================================

// PerformPCA 执行PCA+聚类分析
// GET /api/fingerprint/pca
func (h *Handler) PerformPCA(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	siteData, err := repository.GetAllXRFMeasurements(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := h.fingerprint.PerformPCAWithQuality(siteData)
	if result == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient data for PCA (need >= 2 samples)"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// MatchFingerprint 匹配遗址污染指纹
// GET /api/sites/:id/fingerprint
func (h *Handler) MatchFingerprint(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	siteData, err := repository.GetXRFMeasurements(ctx, id, 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	allSiteData, _ := repository.GetAllXRFMeasurements(ctx)

	var latest *models.XRFMeasurement
	if len(siteData) > 0 {
		latest = &siteData[len(siteData)-1]
	}

	result, err := h.fingerprint.MatchFingerprint(ctx, id, allSiteData, latest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"site_id": id, "data": result})
}

// ====================================
// 修复评估（RemediationAdvisor模块）
// ====================================

// AssessRemediation 修复技术多属性决策推荐
// GET /api/sites/:id/remediation
func (h *Handler) AssessRemediation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	measurements, err := repository.GetXRFMeasurements(ctx, id, 1)
	if err != nil || len(measurements) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no XRF data found for site"})
		return
	}
	m := measurements[0]
	detectedMetals := h.xrf.GetDetectedMetals(&m)
	site, _ := repository.GetSite(ctx, id)

	assessment, err := h.remediation.Assess(ctx, id, detectedMetals, &m, site)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	assessmentID, err := repository.SaveRemediationAssessment(ctx, assessment)
	if err == nil {
		assessment.ID = assessmentID
	}

	c.JSON(http.StatusOK, gin.H{"data": assessment})
}

// GetRemediationTechnologies 获取修复技术库
// GET /api/remediation/technologies
func (h *Handler) GetRemediationTechnologies(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	techs, err := repository.GetAllRemediationTechnologies(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": techs})
}

// ====================================
// 告警系统（AlarmMailer模块）
// ====================================

// CheckAndCreateAlerts 主动检测告警（外部触发）
// POST /api/sites/:id/check-alerts
func (h *Handler) CheckAndCreateAlerts(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid site ID"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	measurements, err := repository.GetXRFMeasurements(ctx, id, 1)
	if err != nil || len(measurements) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no XRF data"})
		return
	}
	m := measurements[0]
	site, _ := repository.GetSite(ctx, id)

	alerts, err := h.alarm.CheckAndCreate(ctx, m, site)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 邮件发送不阻塞
	if len(alerts) > 0 {
		go h.alarm.Send(context.Background(), alerts)
	}

	c.JSON(http.StatusOK, gin.H{"alerts_created": len(alerts), "data": alerts})
}

// SendPendingAlerts 手动刷新聚合告警（发送待发送队列）
// POST /api/alerts/flush
func (h *Handler) SendPendingAlerts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	if err := h.alarm.FlushPending(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "flushed"})
}

// GetAlerts 获取告警列表
// GET /api/alerts
func (h *Handler) GetAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	siteID := 0
	if s := c.Query("site_id"); s != "" {
		siteID, _ = strconv.Atoi(s)
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	alerts, err := repository.GetAlerts(ctx, siteID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": alerts})
}

// ====================================
// 系统统计
// ====================================

// GetStats 获取系统统计概览
// GET /api/stats
func (h *Handler) GetStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	sites, _ := repository.GetSitesWithPollution(ctx)
	measurements, _ := repository.GetAllXRFMeasurements(ctx)
	alerts, _ := repository.GetAlerts(ctx, 0, 1000)

	count := map[string]int{
		"sites_total":      len(sites),
		"measurements":     len(measurements),
		"alerts_total":     len(alerts),
		"alerts_unhandled": 0,
	}
	for _, a := range alerts {
		if !a.IsResolved {
			count["alerts_unhandled"]++
		}
	}
	severityBreakdown := map[string]int{"严重": 0, "高": 0, "中": 0, "低": 0}
	for _, a := range alerts {
		severityBreakdown[a.Severity]++
	}

	techs, _ := repository.GetAllRemediationTechnologies(ctx)
	fps, _ := repository.GetAllFingerprints(ctx)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"counts":              count,
			"severity_breakdown":  severityBreakdown,
			"technologies_count":  len(techs),
			"fingerprints_count":  len(fps),
			"pending_alerts":      h.alarm.PendingCount(),
		},
	})
}
