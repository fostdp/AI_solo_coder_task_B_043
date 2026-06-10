package modules

import (
	"context"

	"archaeology-pollution-system/models"
	"archaeology-pollution-system/modules/global_comparator"
)

type TimelineCompareModule struct {
	inner *global_comparator.GlobalComparator
}

func NewTimelineCompareModule() *TimelineCompareModule {
	return &TimelineCompareModule{inner: global_comparator.NewGlobalComparator()}
}

func (t *TimelineCompareModule) CompareTimelines(ctx context.Context, siteIDs []int, allMeasurements map[int][]models.TrendData) (*models.TimelineComparisonResult, error) {
	return t.inner.CompareTimelines(ctx, siteIDs, allMeasurements)
}
