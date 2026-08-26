package verification

import (
	"testing"
	"time"

	"kilncurve-release/internal/domain"
)

func testLimits() domain.KilnLimits {
	return domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 120, MaxCycleMinutes: 500, TemperatureTolerance: 10}
}
func goodSegments() []domain.CurveSegment {
	return []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1200, DurationMinutes: 120}, {Order: 2, Kind: domain.SegmentHold, StartTemperature: 1200, EndTemperature: 1200, DurationMinutes: 30}, {Order: 3, Kind: domain.SegmentCool, StartTemperature: 1200, EndTemperature: 20, DurationMinutes: 180}}
}

func TestCurveValidatorLocatesContinuityAndRate(t *testing.T) {
	segments := goodSegments()
	segments[1].StartTemperature = 1190
	segments[0].DurationMinutes = 10
	checks := NewCurveValidator().Validate(segments, testLimits())
	codes := map[string]bool{}
	for _, c := range checks {
		codes[c.Code] = true
		if !c.Passed && c.Location == "" {
			t.Fatalf("失败检查 %s 没有定位", c.Code)
		}
	}
	if !codes["SEGMENT_CONTINUITY"] || !codes["HEATING_RATE"] {
		t.Fatalf("未返回预期检查代码: %#v", codes)
	}
}

func TestEvidenceEvaluatorAllQualityRules(t *testing.T) {
	now := time.Now().UTC()
	curve := domain.NewRevision("r", "p", 1, "", "工程师", goodSegments(), now)
	if err := curve.Freeze(now); err != nil {
		t.Fatal(err)
	}
	water, shrink, color, deform, defects := 0.8, 12.0, 2.0, 0.4, true
	run, _ := domain.NewTrialRun("run", "p", "r", 1, nil, "操作员", now)
	if err := run.Record([]domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 120, Temperature: 1200}, {Minute: 150, Temperature: 1200}, {Minute: 330, Temperature: 20}}, domain.QualityMeasurements{WaterAbsorption: &water, Shrinkage: &shrink, ColorDifference: &color, Deformation: &deform, SurfaceDefects: &defects}); err != nil {
		t.Fatal(err)
	}
	project := &domain.TrialProject{KilnLimits: testLimits(), QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 0.5}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1}}
	checks, err := NewEvidenceEvaluator().Evaluate(curve, run, project)
	if err != nil {
		t.Fatal(err)
	}
	failed := map[string]bool{}
	for _, c := range checks {
		if !c.Passed {
			failed[c.Code] = true
		}
	}
	for _, code := range []string{CheckWater, CheckColor, CheckSurface} {
		if !failed[code] {
			t.Errorf("检查 %s 应失败", code)
		}
	}
}

func TestRetestScopeRejectsPassedOrOpenItems(t *testing.T) {
	d := &domain.Deviation{CheckCode: CheckColor, Status: domain.DeviationActioned}
	if err := ValidateRetestScope([]string{CheckColor}, []*domain.Deviation{d}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetestScope([]string{CheckWater}, []*domain.Deviation{d}); err == nil {
		t.Fatal("越界复试范围未被拒绝")
	}
}
