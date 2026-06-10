package modules

import (
	"context"
	"log"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
	"archaeology-pollution-system/repository"
)

// ========================================
// XRFReceiver - XRF数据接收与入库模块
// 职责：XRF数据校验、入库、污染指数计算、事件发布
// 发布事件：EventXRFReceived
// ========================================

type XRFReceiver struct {
	bus     *EventBus
	running bool
}

func NewXRFReceiver() *XRFReceiver {
	r := &XRFReceiver{
		bus: GetEventBus(),
	}
	go r.start()
	return r
}

func (r *XRFReceiver) start() {
	r.running = true
	log.Println("[XRFReceiver] Module started")
}

// Receive 接收并处理XRF数据
// 返回：入库后的measurement + 生成的告警
func (r *XRFReceiver) Receive(ctx context.Context, m *models.XRFMeasurement) (
	*models.XRFMeasurement, []models.Alert, error) {

	if err := validateXRF(m); err != nil {
		return nil, nil, err
	}

	if err := repository.UpsertXRFMeasurement(ctx, m); err != nil {
		return nil, nil, err
	}

	site, err := repository.GetSite(ctx, m.SiteID)
	if err != nil {
		log.Printf("[XRFReceiver] Warning: cannot fetch site %d: %v", m.SiteID, err)
		site = nil
	}

	m.PollutionIndex = calculatePollutionIndex(m)

	r.bus.Publish(Event{
		Type:    EventXRFReceived,
		Payload: XRFReceivedPayload{Measurement: *m, Site: site},
		Context: ctx,
	})

	log.Printf("[XRFReceiver] Received measurement: site=%d year=%d PI=%.2f",
		m.SiteID, m.MeasurementYear, m.PollutionIndex)

	return m, nil, nil
}

func validateXRF(m *models.XRFMeasurement) error {
	if m.SiteID <= 0 {
		return errInvalidSiteID
	}
	if m.MeasurementYear < 1900 || m.MeasurementYear > 2100 {
		return errInvalidYear
	}
	if m.Pb < 0 || m.Zn < 0 || m.Cu < 0 || m.As < 0 || m.Hg < 0 || m.Cd < 0 {
		return errNegativeMetal
	}
	return nil
}

// calculatePollutionIndex 内梅罗综合污染指数
// 使用 config.PollutionStandards 中的标准值
func calculatePollutionIndex(m *models.XRFMeasurement) float64 {
	if m == nil {
		return 0
	}
	metals := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}
	var sumPi float64
	var maxPi float64
	var count int
	for metal, concentration := range metals {
		std, ok := config.PollutionStandards[metal]
		if !ok || std <= 0 {
			continue
		}
		pi := concentration / std
		sumPi += pi
		if pi > maxPi {
			maxPi = pi
		}
		count++
	}
	if count == 0 {
		return 0
	}
	avgPi := sumPi / float64(count)
	return sqrt((maxPi*maxPi + avgPi*avgPi) / 2.0)
}

// GetTrend 获取遗址趋势数据
func (r *XRFReceiver) GetTrend(ctx context.Context, siteID int, limit int) ([]models.TrendData, error) {
	measurements, err := repository.GetXRFMeasurements(ctx, siteID, limit)
	if err != nil {
		return nil, err
	}
	trend := make([]models.TrendData, len(measurements))
	for i, m := range measurements {
		pi := calculatePollutionIndex(&m)
		trend[i] = models.TrendData{
			Year:            m.MeasurementYear,
			Pb:              m.Pb,
			Zn:              m.Zn,
			Cu:              m.Cu,
			As:              m.As,
			Hg:              m.Hg,
			Cd:              m.Cd,
			PollutionIndex:  pi,
			PH:              m.PH,
			OrganicMatter:   m.OrganicMatter,
			CEC:             m.CEC,
			SoilMoisture:    m.SoilMoisture,
			MeasurementDate: m.MeasurementDate,
		}
	}
	return trend, nil
}

// GetDetectedMetals 提取超过检测阈值的金属
func (r *XRFReceiver) GetDetectedMetals(m *models.XRFMeasurement) map[string]float64 {
	detected := make(map[string]float64)
	metals := map[string]float64{
		"Pb": m.Pb, "Zn": m.Zn, "Cu": m.Cu,
		"As": m.As, "Hg": m.Hg, "Cd": m.Cd,
	}
	ratio := config.MetalDetectionThresholdRatio
	for metal, conc := range metals {
		if std, ok := config.PollutionStandards[metal]; ok {
			if conc > std*ratio {
				detected[metal] = conc
			}
		}
	}
	return detected
}

// ============== 内部辅助 ==============

type moduleError string

func (e moduleError) Error() string { return string(e) }

var (
	errInvalidSiteID = moduleError("invalid site ID")
	errInvalidYear   = moduleError("invalid measurement year (1900-2100)")
	errNegativeMetal = moduleError("metal concentration cannot be negative")
)

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 50; i++ {
		prev := z
		z = (z + x/z) / 2
		if prev-z < 1e-12 && z-prev < 1e-12 {
			break
		}
	}
	return z
}

// ============== 接口兼容（供旧代码调用） ==============
func CalculatePollutionIndexPublic(m *models.XRFMeasurement) float64 {
	return calculatePollutionIndex(m)
}

var _ = time.Now
