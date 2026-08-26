package verification

import (
	"fmt"
	"math"
	"sort"

	"kilncurve-release/internal/domain"
)

type EvidenceEvaluator struct{}

func NewEvidenceEvaluator() *EvidenceEvaluator { return &EvidenceEvaluator{} }

const (
	CheckTemperature = "TEMP_TRACK"
	CheckMaturity    = "MATURITY"
	CheckWater       = "WATER_ABSORPTION"
	CheckShrinkage   = "SHRINKAGE"
	CheckColor       = "COLOR_DIFFERENCE"
	CheckDeformation = "DEFORMATION"
	CheckSurface     = "SURFACE_DEFECT"
)

var allChecks = []string{CheckTemperature, CheckMaturity, CheckWater, CheckShrinkage, CheckColor, CheckDeformation, CheckSurface}

func AllCheckCodes() []string { return append([]string(nil), allChecks...) }

func (e *EvidenceEvaluator) Evaluate(curve *domain.FiringCurveRevision, run *domain.TrialRun, project *domain.TrialProject) ([]domain.CheckResult, error) {
	if curve.FreezeStatus != domain.CurveFrozen {
		return nil, domain.NewError(domain.ErrState, "只能依据冻结曲线评估试烧", "curveRevisionId")
	}
	if len(run.TemperatureSamples) < 2 {
		return nil, domain.NewError(domain.ErrInvalid, "测温证据不完整", "temperatureSamples")
	}
	q := run.QualityMeasurements
	if q.WaterAbsorption == nil || q.Shrinkage == nil || q.ColorDifference == nil || q.Deformation == nil || q.SurfaceDefects == nil {
		return nil, domain.NewError(domain.ErrInvalid, "五项成品质检数据必须完整", "qualityMeasurements")
	}
	scope := map[string]bool{}
	if len(run.ScopeCheckCodes) == 0 {
		for _, c := range allChecks {
			scope[c] = true
		}
	} else {
		for _, c := range run.ScopeCheckCodes {
			scope[c] = true
		}
	}
	results := make([]domain.CheckResult, 0, len(scope))
	if scope[CheckTemperature] {
		results = append(results, evaluateTrajectory(curve.Segments, run.TemperatureSamples, project.KilnLimits.TemperatureTolerance))
	}
	if scope[CheckMaturity] {
		results = append(results, evaluateMaturity(curve.Segments, run.TemperatureSamples, project.KilnLimits.TemperatureTolerance))
	}
	if scope[CheckWater] {
		results = append(results, rangeCheck(CheckWater, "吸水率", *q.WaterAbsorption, project.QualityLimits.WaterAbsorption, "%"))
	}
	if scope[CheckShrinkage] {
		results = append(results, rangeCheck(CheckShrinkage, "收缩率", *q.Shrinkage, project.QualityLimits.Shrinkage, "%"))
	}
	if scope[CheckColor] {
		results = append(results, maxCheck(CheckColor, "色差", *q.ColorDifference, project.QualityLimits.MaxColorDifference, "ΔE"))
	}
	if scope[CheckDeformation] {
		results = append(results, maxCheck(CheckDeformation, "变形", *q.Deformation, project.QualityLimits.MaxDeformation, "mm"))
	}
	if scope[CheckSurface] {
		allowed := project.QualityLimits.AllowSurfaceDefects || !*q.SurfaceDefects
		results = append(results, domain.CheckResult{Code: CheckSurface, Name: "表面缺陷", Passed: allowed, Observed: fmt.Sprint(*q.SurfaceDefects), Boundary: fmt.Sprintf("允许缺陷=%t", project.QualityLimits.AllowSurfaceDefects), Message: choose(allowed, "表面缺陷符合边界", "发现不允许的表面缺陷")})
	}
	sort.SliceStable(results, func(i, j int) bool { return index(results[i].Code) < index(results[j].Code) })
	return results, nil
}

func evaluateTrajectory(segments []domain.CurveSegment, samples []domain.TemperatureSample, tolerance float64) domain.CheckResult {
	worst := 0.0
	worstMinute := 0
	for _, s := range samples {
		target, _ := domain.TargetTemperatureAt(segments, s.Minute)
		d := math.Abs(s.Temperature - target)
		if d > worst {
			worst = d
			worstMinute = s.Minute
		}
	}
	ok := worst <= tolerance
	return domain.CheckResult{Code: CheckTemperature, Name: "测温轨迹", Passed: ok, Observed: fmt.Sprintf("最大偏差 %.2f℃（%d min）", worst, worstMinute), Boundary: fmt.Sprintf("<=%.2f℃", tolerance), Location: fmt.Sprintf("temperatureSamples@%d", worstMinute), Message: choose(ok, "测温轨迹符合容差", "测温轨迹偏离目标曲线")}
}

func evaluateMaturity(segments []domain.CurveSegment, samples []domain.TemperatureSample, tolerance float64) domain.CheckResult {
	targetPeak := -math.MaxFloat64
	for _, s := range segments {
		targetPeak = max(targetPeak, max(s.StartTemperature, s.EndTemperature))
	}
	actualPeak := -math.MaxFloat64
	for _, s := range samples {
		actualPeak = max(actualPeak, s.Temperature)
	}
	d := math.Abs(actualPeak - targetPeak)
	ok := d <= tolerance
	return domain.CheckResult{Code: CheckMaturity, Name: "成熟温度", Passed: ok, Observed: fmt.Sprintf("峰值 %.2f℃", actualPeak), Boundary: fmt.Sprintf("目标 %.2f℃±%.2f℃", targetPeak, tolerance), Message: choose(ok, "成熟温度符合要求", "峰值温度未达到成熟度要求")}
}

func rangeCheck(code, name string, v float64, r domain.Range, unit string) domain.CheckResult {
	ok := v >= r.Min && v <= r.Max
	return domain.CheckResult{Code: code, Name: name, Passed: ok, Observed: fmt.Sprintf("%.3f%s", v, unit), Boundary: fmt.Sprintf("%.3f~%.3f%s", r.Min, r.Max, unit), Message: choose(ok, name+"符合边界", name+"超出边界")}
}
func maxCheck(code, name string, v, maxv float64, unit string) domain.CheckResult {
	ok := v <= maxv
	return domain.CheckResult{Code: code, Name: name, Passed: ok, Observed: fmt.Sprintf("%.3f%s", v, unit), Boundary: fmt.Sprintf("<=%.3f%s", maxv, unit), Message: choose(ok, name+"符合边界", name+"超出边界")}
}
func choose(ok bool, a, b string) string {
	if ok {
		return a
	}
	return b
}
func index(code string) int {
	for i, c := range allChecks {
		if c == code {
			return i
		}
	}
	return 100
}
