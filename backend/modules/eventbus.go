package modules

import (
	"context"
	"log"
	"sync"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
)

// ========================================
// EventBus - 模块间通信的消息总线
// 基于 Go Channel 的发布/订阅模式
// ========================================

type EventType string

const (
	EventXRFReceived      EventType = "xrf_received"       // XRF数据接收完成
	EventAlertsGenerated  EventType = "alerts_generated"   // 告警生成完成
	EventFingerprintReady EventType = "fingerprint_ready"  // 指纹分析完成
	EventRemediationReady EventType = "remediation_ready"  // 修复评估完成
	EventEmailSent        EventType = "email_sent"         // 邮件发送完成
)

// Event 总线消息
type Event struct {
	Type      EventType
	Timestamp time.Time
	Payload   interface{}
	Context   context.Context
}

// XRFReceivedPayload XRF接收事件载荷
type XRFReceivedPayload struct {
	Measurement models.XRFMeasurement
	Site        *models.Site
}

// AlertsGeneratedPayload 告警生成事件载荷
type AlertsGeneratedPayload struct {
	Measurement models.XRFMeasurement
	Site        *models.Site
	Alerts      []models.Alert
}

// FingerprintPayload 指纹分析事件载荷
type FingerprintPayload struct {
	SiteID int
	Result *models.FingerprintMatchResult
	Err    error
}

// RemediationPayload 修复评估事件载荷
type RemediationPayload struct {
	SiteID     int
	Assessment *models.RemediationAssessment
	Err        error
}

// EventBus 消息总线
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
	bufferSize  int
}

var (
	instance *EventBus
	once     sync.Once
)

// GetEventBus 获取全局单例
func GetEventBus() *EventBus {
	once.Do(func() {
		instance = &EventBus{
			subscribers: make(map[EventType][]chan Event),
			bufferSize:  config.EventBusChannelBufferSize,
		}
	})
	return instance
}

// Subscribe 订阅某个事件类型
// 返回接收 channel
func (eb *EventBus) Subscribe(eventType EventType) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan Event, eb.bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	log.Printf("[EventBus] Subscribed to %s (total subscribers: %d)",
		eventType, len(eb.subscribers[eventType]))
	return ch
}

// Publish 发布事件（异步不阻塞）
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	subs := eb.subscribers[event.Type]
	channels := make([]chan Event, len(subs))
	copy(channels, subs)
	eb.mu.RUnlock()

	if len(channels) == 0 {
		return
	}

	event.Timestamp = time.Now()

	for _, ch := range channels {
		select {
		case ch <- event:
			// 发送成功
		default:
			log.Printf("[EventBus] Warning: channel full for %s, dropping event", event.Type)
		}
	}
}

// PublishSync 同步发布（阻塞直到所有订阅者消费）
func (eb *EventBus) PublishSync(event Event) {
	eb.mu.RLock()
	subs := eb.subscribers[event.Type]
	channels := make([]chan Event, len(subs))
	copy(channels, subs)
	eb.mu.RUnlock()

	event.Timestamp = time.Now()

	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(c chan Event) {
			defer wg.Done()
			c <- event
		}(ch)
	}
	wg.Wait()
}

// Close 关闭所有订阅通道
func (eb *EventBus) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	for _, subs := range eb.subscribers {
		for _, ch := range subs {
			close(ch)
		}
	}
	eb.subscribers = make(map[EventType][]chan Event)
}
