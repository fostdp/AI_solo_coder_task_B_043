package services

import (
	"context"
	"fmt"
	"log"
	"math"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"

	"gopkg.in/gomail.v2"
)

type AlertService struct{}

func NewAlertService() *AlertService {
	return &AlertService{}
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
	if config.AppConfig.SMTPHost == "smtp.example.com" || config.AppConfig.SMTPUser == "" {
		log.Println("SMTP not configured, skipping email sending. Logging alerts instead:")
		for _, a := range alerts {
			log.Printf("ALERT [%s] %s: %s", a.Severity, a.AlertType, a.Message)
		}
		return nil
	}

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
		subject := fmt.Sprintf("[%s][%s] 古代冶炼遗址污染告警 - %s", alert.Severity, alert.AlertType, alert.MetalType)
		if alert.MetalType == "" {
			subject = fmt.Sprintf("[%s][%s] 古代冶炼遗址污染告警", alert.Severity, alert.AlertType)
		}
		msg.SetHeader("Subject", subject)
		msg.SetBody("text/plain", alert.Message)

		htmlBody := fmt.Sprintf(`
		<html>
			<body style="font-family: Arial, sans-serif; padding: 20px;">
				<h2 style="color: #d32f2f;">⚠️ 古代金属冶炼遗址污染告警</h2>
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
			log.Printf("Failed to send alert email for alert %d: %v", alert.ID, err)
		} else {
			_ = repository.UpdateAlertSent(ctx, alert.ID)
			log.Printf("Alert email sent successfully for alert %d", alert.ID)
		}
	}

	return nil
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
