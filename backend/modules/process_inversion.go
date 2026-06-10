package modules

import (
	"context"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/process_inverter"
)

type ProcessInversionModule struct {
	inner *process_inverter.ProcessInverter
}

type MorphologyFeature = process_inverter.MorphologyFeature

func NewProcessInversionModule() *ProcessInversionModule {
	return &ProcessInversionModule{inner: process_inverter.NewProcessInverter()}
}

func (m *ProcessInversionModule) InvertProcess(ctx context.Context, siteID int, measurements []models.XRFMeasurement, slag *models.SlagComposition) (*models.SmeltingProcessResult, error) {
	return m.inner.InvertProcess(ctx, siteID, measurements, slag)
}

func (m *ProcessInversionModule) GetTemperatureBinLabels() []string {
	return m.inner.GetTemperatureBinLabels()
}
