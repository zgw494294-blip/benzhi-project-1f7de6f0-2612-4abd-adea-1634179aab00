package domain

import (
	"math"
	"sort"
)

type CurveMetrics struct {
	TotalMinutes       int     `json:"totalMinutes"`
	HeatingMinutes     int     `json:"heatingMinutes"`
	HoldingMinutes     int     `json:"holdingMinutes"`
	CoolingMinutes     int     `json:"coolingMinutes"`
	MinimumTemperature float64 `json:"minimumTemperature"`
	PeakTemperature    float64 `json:"peakTemperature"`
	AverageHeatingRate float64 `json:"averageHeatingRate"`
	AverageCoolingRate float64 `json:"averageCoolingRate"`
}

type CurvePoint struct {
	Minute       int     `json:"minute"`
	Temperature  float64 `json:"temperature"`
	SegmentOrder int     `json:"segmentOrder"`
}

func OrderedSegments(segments []CurveSegment) []CurveSegment {
	ordered := append([]CurveSegment(nil), segments...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	return ordered
}

func CalculateCurveMetrics(segments []CurveSegment) CurveMetrics {
	metrics := CurveMetrics{MinimumTemperature: math.MaxFloat64, PeakTemperature: -math.MaxFloat64}
	heatingDelta := 0.0
	coolingDelta := 0.0
	for _, segment := range OrderedSegments(segments) {
		metrics.TotalMinutes += segment.DurationMinutes
		metrics.MinimumTemperature = math.Min(metrics.MinimumTemperature, math.Min(segment.StartTemperature, segment.EndTemperature))
		metrics.PeakTemperature = math.Max(metrics.PeakTemperature, math.Max(segment.StartTemperature, segment.EndTemperature))
		switch segment.Kind {
		case SegmentHeat:
			metrics.HeatingMinutes += segment.DurationMinutes
			heatingDelta += math.Max(0, segment.EndTemperature-segment.StartTemperature)
		case SegmentHold:
			metrics.HoldingMinutes += segment.DurationMinutes
		case SegmentCool:
			metrics.CoolingMinutes += segment.DurationMinutes
			coolingDelta += math.Max(0, segment.StartTemperature-segment.EndTemperature)
		}
	}
	if len(segments) == 0 {
		metrics.MinimumTemperature = 0
		metrics.PeakTemperature = 0
	}
	if metrics.HeatingMinutes > 0 {
		metrics.AverageHeatingRate = heatingDelta / float64(metrics.HeatingMinutes)
	}
	if metrics.CoolingMinutes > 0 {
		metrics.AverageCoolingRate = coolingDelta / float64(metrics.CoolingMinutes)
	}
	return metrics
}

func TargetTemperatureAt(segments []CurveSegment, minute int) (float64, bool) {
	if len(segments) == 0 || minute < 0 {
		return 0, false
	}
	ordered := OrderedSegments(segments)
	elapsed := 0
	for _, segment := range ordered {
		end := elapsed + segment.DurationMinutes
		if minute <= end {
			if segment.DurationMinutes <= 0 {
				return segment.StartTemperature, false
			}
			ratio := float64(minute-elapsed) / float64(segment.DurationMinutes)
			return segment.StartTemperature + (segment.EndTemperature-segment.StartTemperature)*ratio, true
		}
		elapsed = end
	}
	return ordered[len(ordered)-1].EndTemperature, true
}

func CurveBoundaryPoints(segments []CurveSegment) []CurvePoint {
	ordered := OrderedSegments(segments)
	if len(ordered) == 0 {
		return []CurvePoint{}
	}
	points := make([]CurvePoint, 0, len(ordered)+1)
	elapsed := 0
	points = append(points, CurvePoint{Minute: 0, Temperature: ordered[0].StartTemperature, SegmentOrder: ordered[0].Order})
	for _, segment := range ordered {
		elapsed += segment.DurationMinutes
		points = append(points, CurvePoint{Minute: elapsed, Temperature: segment.EndTemperature, SegmentOrder: segment.Order})
	}
	return points
}

func SampleTargetCurve(segments []CurveSegment, intervalMinutes int) []CurvePoint {
	if intervalMinutes <= 0 {
		intervalMinutes = 10
	}
	metrics := CalculateCurveMetrics(segments)
	if metrics.TotalMinutes == 0 {
		return CurveBoundaryPoints(segments)
	}
	points := make([]CurvePoint, 0, metrics.TotalMinutes/intervalMinutes+2)
	for minute := 0; minute < metrics.TotalMinutes; minute += intervalMinutes {
		temperature, ok := TargetTemperatureAt(segments, minute)
		if ok {
			points = append(points, CurvePoint{Minute: minute, Temperature: temperature, SegmentOrder: segmentOrderAt(segments, minute)})
		}
	}
	temperature, _ := TargetTemperatureAt(segments, metrics.TotalMinutes)
	points = append(points, CurvePoint{Minute: metrics.TotalMinutes, Temperature: temperature, SegmentOrder: segmentOrderAt(segments, metrics.TotalMinutes)})
	return points
}

func segmentOrderAt(segments []CurveSegment, minute int) int {
	elapsed := 0
	for _, segment := range OrderedSegments(segments) {
		elapsed += segment.DurationMinutes
		if minute <= elapsed {
			return segment.Order
		}
	}
	if len(segments) == 0 {
		return 0
	}
	return OrderedSegments(segments)[len(segments)-1].Order
}
