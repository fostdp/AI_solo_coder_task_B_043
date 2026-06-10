package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"

	"gopkg.in/gomail.v2"
)

type AlertService struct {
	aggregator *AlertAggregator
}

type AlertAggregator struct {
	mu          sync.Mutex
	pending     []models.Alert
	timer       *time.Timer
	flushPeriod time.Duration
}

func NewAlertService() *AlertService {
	agg := &AlertAggregator{
		pending:     make([]models.Alert, 0),
		flushPeriod: 30 * time.Minute,
	}
	return &AlertService{aggregator: agg}
}

func (a *AlertAggregator) AddAlerts(alerts []models.Alert) []models.Alert {
	a.mu.Lock()
	defer a.mu.Unlock()

	var immediateAlerts []models.Alert
	for _, alert := range alerts {
		if alert.Severity == "严重" {
			immediateAlerts = append(immediateAlerts, alert)
		} else {
			a.pending = append(a.pending, alert)
		}
	}

	if len(a.pending) > 0 && a.timer == nil {
		a.timer = time.AfterFunc(a.flushPeriod, func() {
			a.flush()
		})
	}

	return immediateAlerts
}

func (a *AlertAggregator) flush() []models.Alert {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.pending) == 0 {
		return nil
	}

	flushed := make([]models.Alert, len(a.pending))
	copy(flushed, a.pending)
	a.pending = a.pending[:0]

	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}

	return flushed
}

func (a *AlertAggregator) FlushAll() []models.Alert {
	return a.flush()
}

func (a *AlertAggregator) PendingCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

func (s *AlertService) CheckAndCreateAlerts(ctx context.Context, m *models.XRFMeasurement) ([]models.Alert, error) {
	standards, err := repository.GetRiskStandards(ctx)
	if err != nil {
		return nil, err
	}

	stdMap := make(map[string]models.RiskStandard)
	for _, st := range standards {
		stdMap[st.MetalType] = st
	}

	metals := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}

	var alerts []models.Alert
	site, _ := repository.GetSiteByID(ctx, m.SiteID)
	siteName := "未知遗址"
	if site != nil {
		siteName = site.Name
	}

	for metal, conc := range metals {
		std, ok := stdMap[metal]
		if !ok || conc <= 0 {
			continue
		}

		if conc >= std.InterventionValue {
			alert := models.Alert{
				SiteID:          m.SiteID,
				MeasurementID:   &m.ID,
				AlertType:       "重度污染",
				MetalType:       metal,
				Concentration:   conc,
				Threshold:       std.InterventionValue,
				Severity:        "严重",
				EmailRecipients: config.AppConfig.AlertRecipients,
				Message: fmt.Sprintf(
					"【严重告警】%s 在第 %d 年检测到 %s 浓度 %.2f mg/kg，超过管制值 %.2f mg/kg（超标 %.2f 倍），请立即启动应急修复程序！",
					siteName, m.MeasurementYear, metal, conc, std.InterventionValue, conc/std.InterventionValue,
				),
			}
			err := repository.InsertAlert(ctx, &alert)
			if err == nil {
				alerts = append(alerts, alert)
			}
		} else if conc >= std.ScreeningValue {
			exceedRatio := conc / std.ScreeningValue
			severity := "中"
			alertType := "超标预警"
			if exceedRatio >= 1.5 {
				severity = "高"
				alertType = "修复预警"
			}
			alert := models.Alert{
				SiteID:          m.SiteID,
				MeasurementID:   &m.ID,
				AlertType:       alertType,
				MetalType:       metal,
				Concentration:   conc,
				Threshold:       std.ScreeningValue,
				Severity:        severity,
				EmailRecipients: config.AppConfig.AlertRecipients,
				Message: fmt.Sprintf(
					"【%s】%s 第 %d 年 %s 浓度 %.2f mg/kg，超过筛选值 %.2f mg/kg（超标 %.2f 倍），建议开展详细调查并评估修复需求。",
					alertType, siteName, m.MeasurementYear, metal, conc, std.ScreeningValue, exceedRatio,
				),
			}
			err := repository.InsertAlert(ctx, &alert)
			if err == nil {
				alerts = append(alerts, alert)
			}
		}
	}

	pollutionIndex := repository.CalculatePollutionIndex(m.Pb, m.Zn, m.Cu, m.As, m.Hg, m.Cd)
	if pollutionIndex >= 2.0 {
		ecoRisk := s.estimateEcoRisk(metals)
		alert := models.Alert{
			SiteID:          m.SiteID,
			MeasurementID:   &m.ID,
			AlertType:       "生态风险",
			Severity:        "高",
			EmailRecipients: config.AppConfig.AlertRecipients,
			Message: fmt.Sprintf(
				"【生态风险告警】%s 第 %d 年综合污染指数为 %.4f（≥2.0为重污染），潜在生态风险指数约为 %.1f，建议开展生态风险评估并制定修复方案。",
				siteName, m.MeasurementYear, pollutionIndex, ecoRisk,
			),
		}
		err := repository.InsertAlert(ctx, &alert)
		if err == nil {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

func (s *AlertService) estimateEcoRisk(metals map[string]float64) float64 {
	toxFactors := map[string]float64{
		"Pb": 5, "Zn": 1, "Cu": 5, "As": 10, "Hg": 40, "Cd": 30,
	}
	refs := map[string]float64{
		"Pb": 35, "Zn": 80, "Cu": 35, "As": 15, "Hg": 0.25, "Cd": 0.5,
	}
	total := 0.0
	for m, c := range metals {
		if c > 0 && refs[m] > 0 {
			total += (c / refs[m]) * toxFactors[m]
		}
	}
	return math.Round(total*100) / 100
}

func (s *AlertService) SendAlerts(ctx context.Context, alerts []models.Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	if config.AppConfig.SMTPHost == "smtp.example.com" || config.AppConfig.SMTPUser == "" {
		log.Println("SMTP not configured, skipping email sending. Logging alerts instead:")
		for _, a := range alerts {
			log.Printf("ALERT [%s] %s: %s", a.Severity, a.AlertType, a.Message)
		}
		return nil
	}

	immediateAlerts := s.aggregator.AddAlerts(alerts)

	if len(immediateAlerts) > 0 {
		s.sendImmediateAlerts(ctx, immediateAlerts)
	}

	pendingCount := s.aggregator.PendingCount()
	if pendingCount > 0 {
		log.Printf("Aggregated %d non-severe alerts, will be sent in batch", pendingCount)
	}

	return nil
}

func (s *AlertService) sendImmediateAlerts(ctx context.Context, alerts []models.Alert) {
	d := gomail.NewDialer(
		config.AppConfig.SMTPHost,
		config.AppConfig.SMTPPort,
		config.AppConfig.SMTPUser,
		config.AppConfig.SMTPPassword,
	)

	for _, alert := range alerts {
		if alert.IsSent {
			continue
		}

		msg := gomail.NewMessage()
		msg.SetHeader("From", config.AppConfig.SMTPFrom)
		msg.SetHeader("To", alert.EmailRecipients...)
		subject := fmt.Sprintf("[紧急][%s] 古代冶炼遗址污染告警 - %s", alert.AlertType, alert.MetalType)
		if alert.MetalType == "" {
			subject = fmt.Sprintf("[紧急][%s] 古代冶炼遗址污染告警", alert.AlertType)
		}
		msg.SetHeader("Subject", subject)
		msg.SetBody("text/plain", alert.Message)

		htmlBody := fmt.Sprintf(`
		<html>
			<body style="font-family: Arial, sans-serif; padding: 20px;">
				<h2 style="color: #d32f2f;">🚨 严重告警 - 古代金属冶炼遗址污染</h2>
				<div style="background: #ffebee; padding: 15px; border-radius: 5px; margin: 15px 0;">
					<strong>⚠️ 此为严重级别告警，请立即处理！</strong>
				</div>
				<table style="border-collapse: collapse; margin-top: 20px;">
					<tr><td style="padding: 8px; border: 1px solid #ddd; font-weight: bold;">告警类型:</td><td style="padding: 8px; border: 1px solid #ddd;">%s</td></tr>
					<tr><td style="padding: 8px; border: 1px solid #ddd; font-weight: bold;">严重程度:</td><td style="padding: 8px; border: 1px solid #ddd; color: %s;">%s</td></tr>
					<tr><td style="padding: 8px; border: 1px solid #ddd; font-weight: bold;">遗址ID:</td><td style="padding: 8px; border: 1px solid #ddd;">%d</td></tr>
					%s
					%s
				</table>
				<p style="margin-top: 20px; padding: 15px; background-color: #fff3e0; border-radius: 5px;">%s</p>
				<p style="color: #757575; font-size: 12px; margin-top: 30px;">此邮件由古代金属冶炼遗址污染指纹识别与环境修复系统自动发送</p>
			</body>
		</html>`, alert.AlertType, getSeverityColor(alert.Severity), alert.Severity, alert.SiteID,
			formatMetalRow(alert.MetalType),
			formatConcRow(alert.Concentration, alert.Threshold),
			alert.Message)
		msg.AddAlternative("text/html", htmlBody)

		if err := d.DialAndSend(msg); err != nil {
			log.Printf("Failed to send immediate alert email for alert %d: %v", alert.ID, err)
		} else {
			_ = repository.UpdateAlertSent(ctx, alert.ID)
			log.Printf("Immediate alert email sent successfully for alert %d", alert.ID)
		}
	}
}

func (s *AlertService) SendAggregatedDigest(ctx context.Context) error {
	alerts := s.aggregator.FlushAll()
	if len(alerts) == 0 {
		return nil
	}

	if config.AppConfig.SMTPHost == "smtp.example.com" || config.AppConfig.SMTPUser == "" {
		log.Printf("SMTP not configured. Digest contains %d alerts:", len(alerts))
		for _, a := range alerts {
			log.Printf("  - [%s] %s", a.Severity, a.AlertType)
		}
		for _, a := range alerts {
			_ = repository.UpdateAlertSent(ctx, a.ID)
		}
		return nil
	}

	siteGroups := groupAlertsBySite(alerts)
	severityCounts := countBySeverity(alerts)

	d := gomail.NewDialer(
		config.AppConfig.SMTPHost,
		config.AppConfig.SMTPPort,
		config.AppConfig.SMTPUser,
		config.AppConfig.SMTPPassword,
	)

	recipients := getRecipientsFromAlerts(alerts)
	subject := fmt.Sprintf("[汇总] 古代冶炼遗址污染告警汇总 - %d条告警 · %d处遗址", len(alerts), len(siteGroups))

	msg := gomail.NewMessage()
	msg.SetHeader("From", config.AppConfig.SMTPFrom)
	msg.SetHeader("To", recipients...)
	msg.SetHeader("Subject", subject)

	textBody := buildDigestText(alerts, siteGroups, severityCounts)
	htmlBody := buildDigestHTML(alerts, siteGroups, severityCounts)

	msg.SetBody("text/plain", textBody)
	msg.AddAlternative("text/html", htmlBody)

	if err := d.DialAndSend(msg); err != nil {
		log.Printf("Failed to send aggregated digest email: %v", err)
		return err
	}

	for _, alert := range alerts {
		_ = repository.UpdateAlertSent(ctx, alert.ID)
	}

	log.Printf("Aggregated digest email sent successfully: %d alerts, %d sites", len(alerts), len(siteGroups))
	return nil
}

func groupAlertsBySite(alerts []models.Alert) map[int][]models.Alert {
	groups := make(map[int][]models.Alert)
	for _, a := range alerts {
		groups[a.SiteID] = append(groups[a.SiteID], a)
	}
	return groups
}

func countBySeverity(alerts []models.Alert) map[string]int {
	counts := make(map[string]int)
	for _, a := range alerts {
		counts[a.Severity]++
	}
	return counts
}

func getRecipientsFromAlerts(alerts []models.Alert) []string {
	if len(alerts) == 0 {
		return nil
	}
	return alerts[0].EmailRecipients
}

func buildDigestText(alerts []models.Alert, siteGroups map[int][]models.Alert, severityCounts map[string]int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("古代金属冶炼遗址污染告警汇总\n"))
	sb.WriteString(fmt.Sprintf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("告警总数: %d 条\n", len(alerts)))
	sb.WriteString(fmt.Sprintf("涉及遗址: %d 处\n\n", len(siteGroups)))

	sb.WriteString("一、严重程度统计:\n")
	severities := []string{"严重", "高", "中", "低"}
	for _, sev := range severities {
		if count, ok := severityCounts[sev]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %d 条\n", sev, count))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("二、各遗址告警详情:\n")
	siteIDs := make([]int, 0, len(siteGroups))
	for id := range siteGroups {
		siteIDs = append(siteIDs, id)
	}
	sort.Ints(siteIDs)

	for i, siteID := range siteIDs {
		siteAlerts := siteGroups[siteID]
		sb.WriteString(fmt.Sprintf("\n[%d] 遗址 #%d (%d 条告警):\n", i+1, siteID, len(siteAlerts)))

		for j, a := range siteAlerts {
			metalInfo := ""
			if a.MetalType != "" {
				metalInfo = fmt.Sprintf(" [%s]", a.MetalType)
			}
			sb.WriteString(fmt.Sprintf("  %d. [%s]%s %s\n", j+1, a.Severity, metalInfo, a.AlertType))
			if len(a.Message) > 100 {
				sb.WriteString(fmt.Sprintf("     %s...\n", a.Message[:100]))
			} else {
				sb.WriteString(fmt.Sprintf("     %s\n", a.Message))
			}
		}
	}

	sb.WriteString("\n---\n")
	sb.WriteString("此邮件由古代金属冶炼遗址污染指纹识别与环境修复系统自动发送\n")

	return sb.String()
}

func buildDigestHTML(alerts []models.Alert, siteGroups map[int][]models.Alert, severityCounts map[string]int) string {
	var sb strings.Builder

	sb.WriteString(`
	<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; padding: 20px; color: #333; }
			.header { background: linear-gradient(135deg, #1a2332, #0d1117); color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
			.header h2 { margin: 0 0 10px 0; }
			.summary-stats { display: flex; gap: 20px; margin: 15px 0; }
			.stat-card { flex: 1; background: #f5f5f5; padding: 15px; border-radius: 6px; text-align: center; }
			.stat-card .number { font-size: 28px; font-weight: bold; color: #1976d2; }
			.stat-card .label { font-size: 12px; color: #666; margin-top: 4px; }
			.severity-bar { display: flex; height: 24px; border-radius: 4px; overflow: hidden; margin: 10px 0; }
			.severity-item { display: flex; align-items: center; justify-content: center; color: white; font-size: 11px; font-weight: bold; }
			.site-section { margin-bottom: 20px; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden; }
			.site-header { background: #f5f5f5; padding: 12px 15px; font-weight: bold; cursor: pointer; }
			.site-alerts { padding: 10px 15px; }
			.alert-item { padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
			.alert-item:last-child { border-bottom: none; }
			.severity-badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: bold; margin-right: 8px; }
			.severe { background: #ffebee; color: #d32f2f; }
			.high { background: #fff3e0; color: #f57c00; }
			.medium { background: #fff8e1; color: #fbc02d; }
			.low { background: #e8f5e9; color: #388e3c; }
			.alert-message { color: #666; font-size: 13px; margin-top: 4px; line-height: 1.4; }
			.footer { margin-top: 30px; padding-top: 15px; border-top: 1px solid #eee; color: #999; font-size: 12px; }
			h3 { margin-top: 30px; color: #333; border-bottom: 2px solid #1976d2; padding-bottom: 5px; }
		</style>
	</head>
	<body>`)

	sb.WriteString(`
		<div class="header">
			<h2>📊 古代金属冶炼遗址污染告警汇总</h2>
			<div>生成时间: ` + time.Now().Format("2006-01-02 15:04:05") + `</div>
		</div>`)

	sb.WriteString(`
		<div class="summary-stats">
			<div class="stat-card">
				<div class="number">` + fmt.Sprintf("%d", len(alerts)) + `</div>
				<div class="label">告警总数</div>
			</div>
			<div class="stat-card">
				<div class="number">` + fmt.Sprintf("%d", len(siteGroups)) + `</div>
				<div class="label">涉及遗址</div>
			</div>
			<div class="stat-card">
				<div class="number" style="color: #d32f2f;">` + fmt.Sprintf("%d", severityCounts["严重"]) + `</div>
				<div class="label">严重告警</div>
			</div>
		</div>`)

	sb.WriteString(`<h3>一、严重程度分布</h3>`)
	sb.WriteString(`<div class="severity-bar">`)
	colors := map[string]string{
		"严重": "#d32f2f",
		"高":   "#f57c00",
		"中":   "#fbc02d",
		"低":   "#388e3c",
	}
	severities := []string{"严重", "高", "中", "低"}
	for _, sev := range severities {
		if count, ok := severityCounts[sev]; ok && count > 0 {
			percent := float64(count) / float64(len(alerts)) * 100
			sb.WriteString(fmt.Sprintf(`<div class="severity-item" style="width: %.1f%%; background: %s;">%d</div>`, percent, colors[sev], count))
		}
	}
	sb.WriteString(`</div>`)

	sb.WriteString(`<h3>二、各遗址告警详情</h3>`)

	siteIDs := make([]int, 0, len(siteGroups))
	for id := range siteGroups {
		siteIDs = append(siteIDs, id)
	}
	sort.Ints(siteIDs)

	for _, siteID := range siteIDs {
		siteAlerts := siteGroups[siteID]
		maxSev := getMaxSeverity(siteAlerts)

		sb.WriteString(fmt.Sprintf(`
		<div class="site-section">
			<div class="site-header">
				<span class="severity-badge %s">%s</span>
				遗址 #%d · %d 条告警
			</div>
			<div class="site-alerts">`, getSeverityClass(maxSev), maxSev, siteID, len(siteAlerts)))

		for _, a := range siteAlerts {
			metalInfo := ""
			if a.MetalType != "" {
				metalInfo = fmt.Sprintf(" · %s", a.MetalType)
			}
			sb.WriteString(fmt.Sprintf(`
				<div class="alert-item">
					<div>
						<span class="severity-badge %s">%s</span>
						<strong>%s</strong>%s
					</div>
					<div class="alert-message">%s</div>
				</div>`, getSeverityClass(a.Severity), a.Severity, a.AlertType, metalInfo, a.Message))
		}

		sb.WriteString(`
			</div>
		</div>`)
	}

	sb.WriteString(`
		<div class="footer">
			此邮件由古代金属冶炼遗址污染指纹识别与环境修复系统自动发送 · 请及时处理相关告警
		</div>
	</body>
	</html>`)

	return sb.String()
}

func getMaxSeverity(alerts []models.Alert) string {
	severityOrder := map[string]int{"严重": 4, "高": 3, "中": 2, "低": 1}
	maxSev := "低"
	maxLevel := 0
	for _, a := range alerts {
		if level, ok := severityOrder[a.Severity]; ok && level > maxLevel {
			maxLevel = level
			maxSev = a.Severity
		}
	}
	return maxSev
}

func getSeverityClass(severity string) string {
	switch severity {
	case "严重":
		return "severe"
	case "高":
		return "high"
	case "中":
		return "medium"
	default:
		return "low"
	}
}

func getSeverityColor(severity string) string {
	switch severity {
	case "严重":
		return "#d32f2f"
	case "高":
		return "#f57c00"
	case "中":
		return "#fbc02d"
	default:
		return "#388e3c"
	}
}

func formatMetalRow(metal string) string {
	if metal == "" {
		return ""
	}
	return fmt.Sprintf(`<tr><td style="padding: 8px; border: 1px solid #ddd; font-weight: bold;">重金属:</td><td style="padding: 8px; border: 1px solid #ddd;">%s</td></tr>`, metal)
}

func formatConcRow(conc, threshold float64) string {
	if conc == 0 {
		return ""
	}
	return fmt.Sprintf(`<tr><td style="padding: 8px; border: 1px solid #ddd; font-weight: bold;">浓度/阈值:</td><td style="padding: 8px; border: 1px solid #ddd;">%.2f / %.2f mg/kg</td></tr>`, conc, threshold)
}
