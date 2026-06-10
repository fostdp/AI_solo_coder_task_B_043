package modules

import (
	"context"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/soil_safety_evaluator"
)

type FarmSafetyModule struct {
	inner *soil_safety_evaluator.SoilSafetyEvaluator
}

type KrigingParams = soil_safety_evaluator.KrigingParams

func NewFarmSafetyModule() *FarmSafetyModule {
	return &FarmSafetyModule{inner: soil_safety_evaluator.NewSoilSafetyEvaluator()}
}

func (fsm *FarmSafetyModule) AssessFarmSafety(ctx context.Context, siteID int, farmlands []models.FarmlandSoil) (*models.FarmSafetyAssessmentResult, error) {
	return fsm.inner.AssessFarmSafety(ctx, siteID, farmlands)
}
