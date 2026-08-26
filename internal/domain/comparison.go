package domain

import "reflect"

type SegmentDifference struct {
	Order                 int           `json:"order"`
	ChangeType            string        `json:"changeType"`
	Baseline              *CurveSegment `json:"baseline,omitempty"`
	Comparison            *CurveSegment `json:"comparison,omitempty"`
	StartTemperatureDelta float64       `json:"startTemperatureDelta"`
	EndTemperatureDelta   float64       `json:"endTemperatureDelta"`
	DurationMinutesDelta  int           `json:"durationMinutesDelta"`
	RateDelta             float64       `json:"rateDelta"`
}

type CurveMetricsDelta struct {
	TotalMinutes       int     `json:"totalMinutes"`
	PeakTemperature    float64 `json:"peakTemperature"`
	AverageHeatingRate float64 `json:"averageHeatingRate"`
	AverageCoolingRate float64 `json:"averageCoolingRate"`
}

type CurveComparison struct {
	BaselineRevisionID     string              `json:"baselineRevisionId"`
	ComparisonRevisionID   string              `json:"comparisonRevisionId"`
	BaselineFreezeStatus   FreezeStatus        `json:"baselineFreezeStatus"`
	ComparisonFreezeStatus FreezeStatus        `json:"comparisonFreezeStatus"`
	BaselineBasedOn        string              `json:"baselineBasedOnRevisionId,omitempty"`
	ComparisonBasedOn      string              `json:"comparisonBasedOnRevisionId,omitempty"`
	BaselineDigest         string              `json:"baselineDigest,omitempty"`
	ComparisonDigest       string              `json:"comparisonDigest,omitempty"`
	BaselineMetrics        CurveMetrics        `json:"baselineMetrics"`
	ComparisonMetrics      CurveMetrics        `json:"comparisonMetrics"`
	MetricsDelta           CurveMetricsDelta   `json:"metricsDelta"`
	SegmentDifferences     []SegmentDifference `json:"segmentDifferences"`
}

func CompareRevisions(baseline, comparison *FiringCurveRevision) CurveComparison {
	bm := CalculateCurveMetrics(baseline.Segments)
	cm := CalculateCurveMetrics(comparison.Segments)
	result := CurveComparison{
		BaselineRevisionID: baseline.ID, ComparisonRevisionID: comparison.ID,
		BaselineFreezeStatus: baseline.FreezeStatus, ComparisonFreezeStatus: comparison.FreezeStatus,
		BaselineBasedOn: baseline.BasedOnRevisionID, ComparisonBasedOn: comparison.BasedOnRevisionID,
		BaselineDigest: baseline.ContentDigest, ComparisonDigest: comparison.ContentDigest,
		BaselineMetrics: bm, ComparisonMetrics: cm,
		MetricsDelta:       CurveMetricsDelta{TotalMinutes: cm.TotalMinutes - bm.TotalMinutes, PeakTemperature: cm.PeakTemperature - bm.PeakTemperature, AverageHeatingRate: cm.AverageHeatingRate - bm.AverageHeatingRate, AverageCoolingRate: cm.AverageCoolingRate - bm.AverageCoolingRate},
		SegmentDifferences: []SegmentDifference{},
	}
	baseByOrder := make(map[int]CurveSegment, len(baseline.Segments))
	comparisonByOrder := make(map[int]CurveSegment, len(comparison.Segments))
	maxOrder := 0
	for _, segment := range OrderedSegments(baseline.Segments) {
		baseByOrder[segment.Order] = segment
		if segment.Order > maxOrder {
			maxOrder = segment.Order
		}
	}
	for _, segment := range OrderedSegments(comparison.Segments) {
		comparisonByOrder[segment.Order] = segment
		if segment.Order > maxOrder {
			maxOrder = segment.Order
		}
	}
	for order := 1; order <= maxOrder; order++ {
		before, hadBefore := baseByOrder[order]
		after, hasAfter := comparisonByOrder[order]
		if hadBefore && hasAfter && reflect.DeepEqual(before, after) {
			continue
		}
		diff := SegmentDifference{Order: order}
		if hadBefore {
			copy := before
			diff.Baseline = &copy
		}
		if hasAfter {
			copy := after
			diff.Comparison = &copy
		}
		switch {
		case !hadBefore:
			diff.ChangeType = "ADDED"
		case !hasAfter:
			diff.ChangeType = "REMOVED"
		default:
			diff.ChangeType = "MODIFIED"
		}
		diff.StartTemperatureDelta = after.StartTemperature - before.StartTemperature
		diff.EndTemperatureDelta = after.EndTemperature - before.EndTemperature
		diff.DurationMinutesDelta = after.DurationMinutes - before.DurationMinutes
		diff.RateDelta = segmentRate(after) - segmentRate(before)
		result.SegmentDifferences = append(result.SegmentDifferences, diff)
	}
	return result
}

func segmentRate(segment CurveSegment) float64 {
	if segment.DurationMinutes <= 0 {
		return 0
	}
	return (segment.EndTemperature - segment.StartTemperature) / float64(segment.DurationMinutes)
}
