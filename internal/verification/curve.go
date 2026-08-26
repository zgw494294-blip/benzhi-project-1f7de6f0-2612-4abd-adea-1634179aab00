package verification

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	"kilncurve-release/internal/domain"
)

type CurveValidator struct {
	mu            sync.RWMutex
	hasCached     bool
	cachedKey     [sha256.Size]byte
	cachedResults []domain.CheckResult
}

func NewCurveValidator() *CurveValidator { return &CurveValidator{} }

func (v *CurveValidator) Validate(segments []domain.CurveSegment, limits domain.KilnLimits) []domain.CheckResult {
	key, cacheable := curveCacheKey(segments)
	if cacheable {
		v.mu.RLock()
		if v.hasCached && v.cachedKey == key {
			results := append([]domain.CheckResult(nil), v.cachedResults...)
			v.mu.RUnlock()
			return results
		}
		v.mu.RUnlock()
	}

	results := validateCurve(segments, limits)
	if cacheable {
		v.mu.Lock()
		v.cachedKey = key
		v.cachedResults = append(v.cachedResults[:0], results...)
		v.hasCached = true
		v.mu.Unlock()
	}
	return results
}

func curveCacheKey(segments []domain.CurveSegment) ([sha256.Size]byte, bool) {
	payload, err := json.Marshal(segments)
	if err != nil {
		return [sha256.Size]byte{}, false
	}
	return sha256.Sum256(payload), true
}

func validateCurve(segments []domain.CurveSegment, limits domain.KilnLimits) []domain.CheckResult {
	results := make([]domain.CheckResult, 0)
	if len(segments) == 0 {
		return []domain.CheckResult{fail("CURVE_EMPTY", "曲线分段", "0", ">=1", "segments", "曲线至少需要一个分段")}
	}
	ordered := append([]domain.CurveSegment(nil), segments...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	total := 0
	for i, s := range ordered {
		loc := fmt.Sprintf("segments[%d]", i)
		if s.Order != i+1 {
			results = append(results, fail("SEGMENT_ORDER", "分段顺序", fmt.Sprint(s.Order), fmt.Sprint(i+1), loc+".order", "分段序号必须从 1 连续递增"))
		}
		if s.DurationMinutes <= 0 {
			results = append(results, fail("SEGMENT_DURATION", "分段时长", fmt.Sprint(s.DurationMinutes), ">0", loc+".durationMinutes", "分段时长必须为正数"))
			continue
		}
		total += s.DurationMinutes
		if math.IsNaN(s.StartTemperature) || math.IsInf(s.StartTemperature, 0) {
			results = append(results, fail("SEGMENT_TEMPERATURE", "分段起温", fmt.Sprint(s.StartTemperature), "有限数值", loc+".startTemperature", "分段起温必须为有限数值"))
			continue
		}
		if math.IsNaN(s.EndTemperature) || math.IsInf(s.EndTemperature, 0) {
			results = append(results, fail("SEGMENT_TEMPERATURE", "分段终温", fmt.Sprint(s.EndTemperature), "有限数值", loc+".endTemperature", "分段终温必须为有限数值"))
			continue
		}
		if i > 0 && math.Abs(ordered[i-1].EndTemperature-s.StartTemperature) > 0.001 {
			results = append(results, fail("SEGMENT_CONTINUITY", "温度连续性", format(s.StartTemperature), format(ordered[i-1].EndTemperature), loc+".startTemperature", "当前起温必须等于上一段终温"))
		}
		if s.StartTemperature < limits.MinTemperature || s.EndTemperature < limits.MinTemperature || s.StartTemperature > limits.MaxTemperature || s.EndTemperature > limits.MaxTemperature {
			results = append(results, fail("TEMPERATURE_CAPABILITY", "窑炉温度能力", format(max(s.StartTemperature, s.EndTemperature)), fmt.Sprintf("%.1f~%.1f℃", limits.MinTemperature, limits.MaxTemperature), loc, "分段温度超出窑炉能力"))
		}
		delta := s.EndTemperature - s.StartTemperature
		rate := math.Abs(delta) / float64(s.DurationMinutes)
		switch s.Kind {
		case domain.SegmentHeat:
			if delta <= 0 {
				results = append(results, fail("HEAT_DIRECTION", "升温方向", format(delta), ">0", loc, "升温段终温必须高于起温"))
			}
			if rate > limits.MaxHeatingRate {
				results = append(results, fail("HEATING_RATE", "升温斜率", format(rate), fmt.Sprintf("<=%.2f℃/min", limits.MaxHeatingRate), loc, "升温斜率超过窑炉能力"))
			}
		case domain.SegmentCool:
			if delta >= 0 {
				results = append(results, fail("COOL_DIRECTION", "降温方向", format(delta), "<0", loc, "降温段终温必须低于起温"))
			}
			if rate > limits.MaxCoolingRate {
				results = append(results, fail("COOLING_RATE", "降温斜率", format(rate), fmt.Sprintf("<=%.2f℃/min", limits.MaxCoolingRate), loc, "降温斜率超过窑炉能力"))
			}
		case domain.SegmentHold:
			if math.Abs(delta) > 0.001 {
				results = append(results, fail("HOLD_STABILITY", "保温稳定性", format(delta), "0℃", loc, "保温段起止温度必须相同"))
			}
			if s.DurationMinutes > limits.MaxHoldMinutes {
				results = append(results, fail("HOLD_DURATION", "保温时长", fmt.Sprint(s.DurationMinutes), fmt.Sprintf("<=%d min", limits.MaxHoldMinutes), loc, "保温时长超过设备能力"))
			}
		default:
			results = append(results, fail("SEGMENT_KIND", "分段类型", string(s.Kind), "HEAT/HOLD/COOL", loc+".kind", "未知分段类型"))
		}
	}
	if total > limits.MaxCycleMinutes {
		results = append(results, fail("TOTAL_CYCLE", "总周期", fmt.Sprint(total), fmt.Sprintf("<=%d min", limits.MaxCycleMinutes), "segments", "总周期超过窑炉能力"))
	}
	if len(results) == 0 {
		results = append(results, pass("CURVE_VALID", "曲线静态校验", "通过", "全部窑炉边界", "曲线合法"))
	}
	return results
}

func HasFailures(results []domain.CheckResult) bool {
	for _, r := range results {
		if !r.Passed {
			return true
		}
	}
	return false
}
func fail(code, name, observed, boundary, location, message string) domain.CheckResult {
	return domain.CheckResult{Code: code, Name: name, Passed: false, Observed: observed, Boundary: boundary, Location: location, Message: message}
}
func pass(code, name, observed, boundary, message string) domain.CheckResult {
	return domain.CheckResult{Code: code, Name: name, Passed: true, Observed: observed, Boundary: boundary, Message: message}
}
func format(v float64) string { return fmt.Sprintf("%.2f", v) }
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
