package common

import (
	"context"
	"log"
	"sync"
	"time"

	"archaeology-pollution-system/config"
	"archaeology-pollution-system/models"
)

type EventType string

const (
	EventXRFReceived      EventType = "xrf_received"
	EventAlertsGenerated  EventType = "alerts_generated"
	EventFingerprintReady EventType = "fingerprint_ready"
	EventRemediationReady EventType = "remediation_ready"
	EventEmailSent        EventType = "email_sent"
)

type Event struct {
	Type      EventType
	Timestamp time.Time
	Payload   interface{}
	Context   context.Context
}

type XRFReceivedPayload struct {
	Measurement models.XRFMeasurement
	Site        *models.Site
}

type AlertsGeneratedPayload struct {
	Measurement models.XRFMeasurement
	Site        *models.Site
	Alerts      []models.Alert
}

type FingerprintPayload struct {
	SiteID int
	Result *models.FingerprintMatchResult
	Err    error
}

type RemediationPayload struct {
	SiteID     int
	Assessment *models.RemediationAssessment
	Err        error
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
	bufferSize  int
}

var (
	eventBusInstance *EventBus
	eventBusOnce     sync.Once
)

func GetEventBus() *EventBus {
	eventBusOnce.Do(func() {
		eventBusInstance = &EventBus{
			subscribers: make(map[EventType][]chan Event),
			bufferSize:  config.EventBusChannelBufferSize,
		}
	})
	return eventBusInstance
}

func (eb *EventBus) Subscribe(eventType EventType) <-chan Event {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(chan Event, eb.bufferSize)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], ch)
	log.Printf("[EventBus] Subscribed to %s (total subscribers: %d)",
		eventType, len(eb.subscribers[eventType]))
	return ch
}

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
		default:
			log.Printf("[EventBus] Warning: channel full for %s, dropping event", event.Type)
		}
	}
}

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
