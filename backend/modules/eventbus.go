package modules

import (
	"context"

	"archaeology-pollution-system/modules/common"
)

type EventType = common.EventType

const (
	EventXRFReceived      = common.EventXRFReceived
	EventAlertsGenerated  = common.EventAlertsGenerated
	EventFingerprintReady = common.EventFingerprintReady
	EventRemediationReady = common.EventRemediationReady
	EventEmailSent        = common.EventEmailSent
)

type Event = common.Event

type XRFReceivedPayload = common.XRFReceivedPayload

type AlertsGeneratedPayload = common.AlertsGeneratedPayload

type FingerprintPayload = common.FingerprintPayload

type RemediationPayload = common.RemediationPayload

type EventBus = common.EventBus

func GetEventBus() *EventBus {
	return common.GetEventBus()
}

var _ = context.Background
