package modules

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"

	"gopkg.in/gomail.v2"
)

// ========================================
// AlarmMailer - 告警检测 + 邮件聚合推送
// 职责：超标检测、告警分级、聚合发送、批量邮件
// 订阅：EventXRFReceived
// 发布：EventAlertsGenerated, EventEmailSent
// ========================================

type AlarmMailer struct {
	bus        *EventBus
	alertCfg   config.AlertConfig
	ecoCfg     config.EcoRiskConfig
	aggregator *AlertAggregatorModule
	running    bool
}

type AlertAggregatorModule struct {
	mu          sync.Mutex
	pending     []models.Alert
	timer       *time.Timer
	flushPeriod time.Duration
	flushFn     func([]models.Alert)
}

func NewAlarmMailer() *AlarmMailer {
	am := &AlarmMailer{
		bus:      GetEventBus(),
		alertCfg: config.DefaultAlertConfig,
		ecoCfg:   config.DefaultEcoRiskConfig,
	}
	am.aggregator = &AlertAggregatorModule{
		pending:     make([]models.Alert, 0),
		flushPeriod: time.Duration(am.alertCfg.AggregateFlushPeriodMinutes) * time.Minute,
	}
	am.aggregator.flushFn = func(alerts []models.Alert) {
		am.doSendAggregated(context.Background(), alerts)
	}
	go am.start()
	return am
}

func (am *AlarmMailer) start() {
	am.running = true
	log.Println("[AlarmMailer] Module started")

	ch := am.bus.Subscribe(EventXRFReceived)
	for event := range ch {
		if !am.running {
			return
		}
		payload, ok := event.Payload.(XRFReceivedPayload)
		if !ok {
			continue
		}
		go am.handleXRFReceived(event.Context, payload)
	}
}

func (am *AlarmMailer) handleXRFReceived(ctx context.Context, payload XRFReceivedPayload) {
	alerts, err := am.CheckAndCreate(ctx, payload.Measurement, payload.Site)
	if err != nil {
		log.Printf("[AlarmMailer] Error checking alerts: %v", err)
		return
	}
	if len(alerts) > 0 {
		am.bus.Publish(Event{
			Type:    EventAlertsGenerated,
			Payload: AlertsGeneratedPayload{
				Measurement: payload.Measurement,
				Site:        payload.Site,
				Alerts:      alerts,
			},
			Context: ctx,
		})
		am.Send(ctx, alerts)
	}
}

// ============== 告警检测 ==============

func (am *AlarmMailer) CheckAndCreate(ctx context.Context,
	m models.XRFMeasurement, site *models.Site) ([]models.Alert, error) {

	standards, err := repository.GetAllRiskStandards(ctx)
	if err != nil {
		return nil, err
	}
	stdMap := make(map[string]models.RiskStandard)
	for _, s := range standards {
		stdMap[s.MetalType] = s
	}

	alerts := make([]models.Alert, 0)
	metals := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}

	pollutionIndex := CalculatePollutionIndexPublic(&m)
	ecoRisk := am.calcEcoRisk(metals)
	recipients := config.AppConfig.AlertRecipients

	for metal, conc := range metals {
		std, ok := stdMap[metal]
		if !ok {
			continue
		}
		ratio := 0.0
		if std.InterventionValue > 0 {
			ratio = conc / std.InterventionValue
		}

		var severity, alertType string
		var threshold float64
		var message string

		switch {
		case std.InterventionValue > 0 && conc >= std.InterventionValue:
			severity = "严重"
			alertType = "重度污染告警"
			threshold = std.InterventionValue
			message = fmt.Sprintf(
				"遗址#%d [%s] %s浓度 %.2f mg/kg 超出管制值 %.2f mg/kg (%.1f倍)。污染指数PI=%.2f，需立即启动应急预案。",
				m.SiteID, siteNameOrDefault(site), metal, conc, threshold, ratio, pollutionIndex)

		case std.ScreenValue > 0 && conc >= std.ScreenValue*am.alertCfg.ExceedRatioForHighLevel:
			severity = "高"
			alertType = "修复预警"
			threshold = std.ScreenValue
			message = fmt.Sprintf(
				"遗址#%d [%s] %s浓度 %.2f mg/kg 超过筛选值%.1f倍 (阈值 %.2f)。污染指数PI=%.2f，建议进行详细调查和修复方案设计。",
				m.SiteID, siteNameOrDefault(site), metal, conc, am.alertCfg.ExceedRatioForHighLevel,
				threshold, pollutionIndex)

		case std.ScreenValue > 0 && conc >= std.ScreenValue:
			severity = "中"
			alertType = "超标预警"
			threshold = std.ScreenValue
			message = fmt.Sprintf(
				"遗址#%d [%s] %s浓度 %.2f mg/kg 刚超过筛选值 %.2f mg/kg。建议加强监测频次并开展初步风险评估。",
				m.SiteID, siteNameOrDefault(site), metal, conc, threshold)

		default:
			continue
		}

		alert := models.Alert{
			SiteID:          m.SiteID,
			AlertType:       alertType,
			MetalType:       metal,
			Severity:        severity,
			Concentration:   conc,
			Threshold:       threshold,
			ExceedRatio:     ratio,
			PollutionIndex:  pollutionIndex,
			EcoRiskIndex:    ecoRisk,
			Message:         message,
			EmailRecipients: recipients,
			MeasurementYear: m.MeasurementYear,
		}
		alerts = append(alerts, alert)
	}

	if pollutionIndex >= am.alertCfg.MediumPollutionThreshold {
		severity := "高"
		if ecoRisk >= am.alertCfg.HighEcoRiskThreshold {
			severity = "严重"
		}
		alert := models.Alert{
			SiteID:          m.SiteID,
			AlertType:       "生态风险告警",
			Severity:        severity,
			PollutionIndex:  pollutionIndex,
			EcoRiskIndex:    ecoRisk,
			Message:         fmt.Sprintf("遗址#%d [%s] 综合污染指数PI=%.2f，潜在生态风险指数RI=%.1f。建议立即开展详细风险评估，制定修复方案。", m.SiteID, siteNameOrDefault(site), pollutionIndex, ecoRisk),
			EmailRecipients: recipients,
			MeasurementYear: m.MeasurementYear,
		}
		alerts = append(alerts, alert)
	}

	for i := range alerts {
		created, err := repository.CreateAlert(ctx, &alerts[i])
		if err != nil {
			log.Printf("[AlarmMailer] Warning: failed to persist alert: %v", err)
			continue
		}
		if created != nil {
			alerts[i] = *created
		}
	}

	return alerts, nil
}

// ============== 邮件发送（聚合 + 单条） ==============

func (am *AlarmMailer) Send(ctx context.Context, alerts []models.Alert) error {
	if len(alerts) == 0 {
		return nil
	}
	immediate := am.aggregator.Add(alerts)
	if len(immediate) > 0 {
		am.doSendImmediate(ctx, immediate)
	}
	if am.aggregator.PendingCount() > 0 {
		log.Printf("[AlarmMailer] Aggregated %d non-severe alerts pending batch send",
			am.aggregator.PendingCount())
	}
	return nil
}

func (am *AlarmMailer) doSendImmediate(ctx context.Context, alerts []models.Alert) {
	d := gomail.NewDialer(
		config.AppConfig.SMTPHost,
		config.AppConfig.SMTPPort,
		config.AppConfig.SMTPUser,
		config.AppConfig.SMTPPassword,
	)
	for _, alert := range alerts {
		if alert.IsSent || config.AppConfig.SMTPHost == "smtp.example.com" {
			continue
		}
		msg := gomail.NewMessage()
		msg.SetHeader("From", config.AppConfig.SMTPFrom)
		msg.SetHeader("To", alert.EmailRecipients...)
		subject := fmt.Sprintf("[紧急][%s] 遗址污染告警 - %s", alert.AlertType, alert.MetalType)
		if alert.MetalType == "" {
			subject = fmt.Sprintf("[紧急][%s] 遗址污染告警", alert.AlertType)
		}
		msg.SetHeader("Subject", subject)
		htmlBody := fmt.Sprintf(`
		<html><body style="font-family:Arial;padding:20px;">
			<h2 style="color:#d32f2f;">🚨 严重告警 - 古代金属冶炼遗址污染</h2>
			<div style="background:#ffebee;padding:15px;border-radius:5px;margin:15px 0;">
				<strong>⚠️ 此为严重级别告警，请立即处理！</strong>
			</div>
			<table style="border-collapse:collapse;">
				<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold;">告警类型:</td><td style="padding:8px;">%s</td></tr>
				<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold;">严重程度:</td><td style="padding:8px;color:%s;">%s</td></tr>
				<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold;">遗址ID:</td><td style="padding:8px;">%d</td></tr>
				%s %s
			</table>
			<p style="margin-top:20px;padding:15px;background:#fff3e0;border-radius:5px;">%s</p>
			<p style="color:#757575;font-size:12px;margin-top:30px;">此邮件由古代金属冶炼遗址污染指纹识别与环境修复系统自动发送</p>
		</body></html>`, alert.AlertType, getSeverityColor(alert.Severity),
			alert.Severity, alert.SiteID,
			formatRowIf("金属类型", alert.MetalType),
			formatConcRowIf(alert.Concentration, alert.Threshold),
			alert.Message)
		msg.SetBody("text/plain", alert.Message)
		msg.AddAlternative("text/html", htmlBody)

		if err := d.DialAndSend(msg); err != nil {
			log.Printf("[AlarmMailer] Failed to send immediate alert %d: %v", alert.ID, err)
		} else {
			_ = repository.UpdateAlertSent(ctx, alert.ID)
			am.bus.Publish(Event{Type: EventEmailSent, Context: ctx})
			log.Printf("[AlarmMailer] Immediate alert %d sent", alert.ID)
		}
	}
}

func (am *AlarmMailer) doSendAggregated(ctx context.Context, alerts []models.Alert) {
	if len(alerts) == 0 {
		return
	}

	if config.AppConfig.SMTPHost == "smtp.example.com" {
		log.Printf("[AlarmMailer] SMTP not configured. Digest: %d alerts across %d sites",
			len(alerts), countSites(alerts))
		for _, a := range alerts {
			_ = repository.UpdateAlertSent(ctx, a.ID)
		}
		return
	}

	siteGroups := groupAlertsBySite(alerts)
	sevCounts := countBySev(alerts)

	d := gomail.NewDialer(
		config.AppConfig.SMTPHost,
		config.AppConfig.SMTPPort,
		config.AppConfig.SMTPUser,
		config.AppConfig.SMTPPassword,
	)
	recipients := alerts[0].EmailRecipients
	subject := fmt.Sprintf("[汇总] 污染告警汇总 - %d条 · %d处遗址", len(alerts), len(siteGroups))

	msg := gomail.NewMessage()
	msg.SetHeader("From", config.AppConfig.SMTPFrom)
	msg.SetHeader("To", recipients...)
	msg.SetHeader("Subject", subject)

	textBody := am.buildDigestText(alerts, siteGroups, sevCounts)
	htmlBody := am.buildDigestHTML(alerts, siteGroups, sevCounts)
	msg.SetBody("text/plain", textBody)
	msg.AddAlternative("text/html", htmlBody)

	if err := d.DialAndSend(msg); err != nil {
		log.Printf("[AlarmMailer] Failed to send aggregated digest: %v", err)
		return
	}
	for _, a := range alerts {
		_ = repository.UpdateAlertSent(ctx, a.ID)
	}
	am.bus.Publish(Event{Type: EventEmailSent, Context: ctx})
	log.Printf("[AlarmMailer] Aggregated digest sent: %d alerts, %d sites", len(alerts), len(siteGroups))
}

// FlushPending 手动刷新待发送告警（用于测试或每日汇总）
func (am *AlarmMailer) FlushPending(ctx context.Context) error {
	alerts := am.aggregator.Flush()
	if len(alerts) == 0 {
		return nil
	}
	am.doSendAggregated(ctx, alerts)
	return nil
}

// ============== 聚合器 ==============

func (a *AlertAggregatorModule) Add(alerts []models.Alert) []models.Alert {
	a.mu.Lock()
	defer a.mu.Unlock()

	var immediate []models.Alert
	for _, a2 := range alerts {
		if a2.Severity == "严重" {
			immediate = append(immediate, a2)
		} else {
			a.pending = append(a.pending, a2)
		}
	}
	if len(a.pending) > 0 && a.timer == nil {
		a.timer = time.AfterFunc(a.flushPeriod, func() {
			flushed := a.Flush()
			if len(flushed) > 0 && a.flushFn != nil {
				a.flushFn(flushed)
			}
		})
	}
	return immediate
}

func (a *AlertAggregatorModule) Flush() []models.Alert {
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

func (a *AlertAggregatorModule) PendingCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

// ============== 辅助函数 ==============

func (am *AlarmMailer) calcEcoRisk(metals map[string]float64) float64 {
	var sum float64
	for metal, conc := range metals {
		tf, ok1 := am.ecoCfg.ToxicFactors[metal]
		ref, ok2 := am.ecoCfg.RefValues[metal]
		if !ok1 || !ok2 || ref <= 0 {
			continue
		}
		sum += tf * conc / ref
	}
	return sum
}

func siteNameOrDefault(site *models.Site) string {
	if site != nil && site.Name != "" {
		return site.Name
	}
	return "未知遗址"
}

func getSeverityColor(sev string) string {
	switch sev {
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

func formatRowIf(label, val string) string {
	if val == "" {
		return ""
	}
	return fmt.Sprintf(`<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold;">%s:</td><td style="padding:8px;">%s</td></tr>`, label, val)
}

func formatConcRowIf(conc, threshold float64) string {
	if conc == 0 && threshold == 0 {
		return ""
	}
	return fmt.Sprintf(`<tr><td style="padding:8px;border:1px solid #ddd;font-weight:bold;">浓度/阈值:</td><td style="padding:8px;">%.2f / %.2f mg/kg</td></tr>`, conc, threshold)
}

func countSites(alerts []models.Alert) int {
	m := map[int]bool{}
	for _, a := range alerts {
		m[a.SiteID] = true
	}
	return len(m)
}

func groupAlertsBySite(alerts []models.Alert) map[int][]models.Alert {
	g := make(map[int][]models.Alert)
	for _, a := range alerts {
		g[a.SiteID] = append(g[a.SiteID], a)
	}
	return g
}

func countBySev(alerts []models.Alert) map[string]int {
	c := make(map[string]int)
	for _, a := range alerts {
		c[a.Severity]++
	}
	return c
}

func (am *AlarmMailer) buildDigestText(alerts []models.Alert,
	groups map[int][]models.Alert, sevCounts map[string]int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("古代冶炼遗址污染告警汇总 - %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("告警总数: %d 条, 涉及遗址: %d 处\n\n", len(alerts), len(groups)))
	sb.WriteString("一、严重程度统计:\n")
	for _, s := range []string{"严重", "高", "中", "低"} {
		if c, ok := sevCounts[s]; ok {
			sb.WriteString(fmt.Sprintf("  %s: %d 条\n", s, c))
		}
	}
	sb.WriteString("\n二、各遗址详情:\n")
	ids := make([]int, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for idx, id := range ids {
		as := groups[id]
		sb.WriteString(fmt.Sprintf("\n[%d] 遗址 #%d (%d 条):\n", idx+1, id, len(as)))
		for j, a := range as {
			info := ""
			if a.MetalType != "" {
				info = " [" + a.MetalType + "]"
			}
			sb.WriteString(fmt.Sprintf("  %d. [%s]%s %s\n", j+1, a.Severity, info, a.AlertType))
			if len(a.Message) > 120 {
				sb.WriteString(fmt.Sprintf("     %s...\n", a.Message[:120]))
			} else {
				sb.WriteString(fmt.Sprintf("     %s\n", a.Message))
			}
		}
	}
	return sb.String()
}

func (am *AlarmMailer) buildDigestHTML(alerts []models.Alert,
	groups map[int][]models.Alert, sevCounts map[string]int) string {
	var sb strings.Builder
	sb.WriteString(`<html><head><style>
		body{font-family:Arial,sans-serif;padding:20px;color:#333;}
		.header{background:linear-gradient(135deg,#1a2332,#0d1117);color:white;padding:20px;border-radius:8px;margin-bottom:20px;}
		.summary-stats{display:flex;gap:20px;margin:15px 0;}
		.stat-card{flex:1;background:#f5f5f5;padding:15px;border-radius:6px;text-align:center;}
		.stat-card .number{font-size:28px;font-weight:bold;color:#1976d2;}
		.site-section{margin-bottom:20px;border:1px solid #e0e0e0;border-radius:8px;overflow:hidden;}
		.site-header{background:#f5f5f5;padding:12px 15px;font-weight:bold;}
		.site-alerts{padding:10px 15px;}
		.alert-item{padding:8px 0;border-bottom:1px solid #f0f0f0;}
		.severity-badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:11px;font-weight:bold;margin-right:8px;}
		.severe{background:#ffebee;color:#d32f2f;}.high{background:#fff3e0;color:#f57c00;}
		.medium{background:#fff8e1;color:#fbc02d;}.low{background:#e8f5e9;color:#388e3c;}
		.alert-message{color:#666;font-size:13px;margin-top:4px;line-height:1.4;}
		.footer{margin-top:30px;padding-top:15px;border-top:1px solid #eee;color:#999;font-size:12px;}
		h3{margin-top:30px;color:#333;border-bottom:2px solid #1976d2;padding-bottom:5px;}
		.severity-bar{display:flex;height:24px;border-radius:4px;overflow:hidden;margin:10px 0;}
		.severity-item{display:flex;align-items:center;justify-content:center;color:white;font-size:11px;font-weight:bold;}
	</style></head><body>`)
	sb.WriteString(fmt.Sprintf(`<div class="header">
		<h2>📊 古代冶炼遗址污染告警汇总</h2>
		<div>生成时间: %s</div></div>`, time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf(`
		<div class="summary-stats">
			<div class="stat-card"><div class="number">%d</div><div class="label">告警总数</div></div>
			<div class="stat-card"><div class="number">%d</div><div class="label">涉及遗址</div></div>
			<div class="stat-card"><div class="number" style="color:#d32f2f;">%d</div><div class="label">严重告警</div></div>
		</div>`, len(alerts), len(groups), sevCounts["严重"]))

	sb.WriteString(`<h3>一、严重程度分布</h3><div class="severity-bar">`)
	colors := map[string]string{"严重": "#d32f2f", "高": "#f57c00", "中": "#fbc02d", "低": "#388e3c"}
	for _, s := range []string{"严重", "高", "中", "低"} {
		if c, ok := sevCounts[s]; ok && c > 0 {
			pct := float64(c) / float64(len(alerts)) * 100
			sb.WriteString(fmt.Sprintf(`<div class="severity-item" style="width:%.1f%%;background:%s;">%d</div>`,
				pct, colors[s], c))
		}
	}
	sb.WriteString(`</div><h3>二、各遗址详情</h3>`)

	ids := make([]int, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		as := groups[id]
		maxSev := getMaxSev(as)
		cls := getSevClass(maxSev)
		sb.WriteString(fmt.Sprintf(`<div class="site-section">
			<div class="site-header"><span class="severity-badge %s">%s</span>遗址 #%d · %d 条告警</div>
			<div class="site-alerts">`, cls, maxSev, id, len(as)))
		for _, a := range as {
			info := ""
			if a.MetalType != "" {
				info = " · " + a.MetalType
			}
			sb.WriteString(fmt.Sprintf(`
				<div class="alert-item">
					<div><span class="severity-badge %s">%s</span><strong>%s</strong>%s</div>
					<div class="alert-message">%s</div>
				</div>`, getSevClass(a.Severity), a.Severity, a.AlertType, info, a.Message))
		}
		sb.WriteString(`</div></div>`)
	}
	sb.WriteString(`<div class="footer">此邮件由古代金属冶炼遗址污染指纹识别与环境修复系统自动发送</div>
		</body></html>`)
	return sb.String()
}

func getMaxSev(alerts []models.Alert) string {
	order := map[string]int{"严重": 4, "高": 3, "中": 2, "低": 1}
	maxLvl := 0
	maxName := "低"
	for _, a := range alerts {
		if l, ok := order[a.Severity]; ok && l > maxLvl {
			maxLvl = l
			maxName = a.Severity
		}
	}
	return maxName
}

func getSevClass(s string) string {
	switch s {
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
