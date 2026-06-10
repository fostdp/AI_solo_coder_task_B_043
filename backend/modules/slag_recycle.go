package modules

import (
	"context"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/slag_reuse_advisor"
)

type SlagRecycleModule struct {
	inner *slag_reuse_advisor.SlagReuseAdvisor
}

func NewSlagRecycleModule() *SlagRecycleModule {
	return &SlagRecycleModule{inner: slag_reuse_advisor.NewSlagReuseAdvisor()}
}

func (m *SlagRecycleModule) AssessRecycle(ctx context.Context, siteID int, slag *models.SlagComposition) (*models.SlagRecycleResult, error) {
	return m.inner.AssessRecycle(ctx, siteID, slag)
}
