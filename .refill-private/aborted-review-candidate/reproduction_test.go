package abortedreviewcandidate_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestAbortedReviewCandidateCannotLeakIntoLaterApproval(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	project, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "candidate-create",
		Role:           string(domain.RoleProcessEngineer),
		Code:           "KC-CANDIDATE",
		Title:          "失败复核候选隔离",
		Owner:          "工艺工程师",
		BodyMaterial:   "瓷坯",
		GlazeMaterial:  "透明釉",
		LoadingMethod:  "平码",
		KilnLimits: domain.KilnLimits{
			MinTemperature:       20,
			MaxTemperature:       1300,
			MaxHeatingRate:       10,
			MaxCoolingRate:       10,
			MaxHoldMinutes:       120,
			MaxCycleMinutes:      500,
			TemperatureTolerance: 20,
		},
		QualityLimits: domain.QualityLimits{
			WaterAbsorption:    domain.Range{Min: 0, Max: 0.5},
			Shrinkage:          domain.Range{Min: 10, Max: 15},
			MaxColorDifference: 1.5,
			MaxDeformation:     1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	baselineSegments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}
	baseline, err := service.CreateRevision(project.ID, application.RevisionCommand{ExpectedVersion: 1, IdempotencyKey: "candidate-r1", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师", Segments: baselineSegments})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.FreezeRevision(project.ID, baseline.ID, application.FreezeCommand{ExpectedVersion: 2, IdempotencyKey: "candidate-f1", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师"}); err != nil {
		t.Fatal(err)
	}

	water, shrinkage, color, deformation, defects := 0.8, 12.0, 0.8, 0.4, false
	quality := domain.QualityMeasurements{WaterAbsorption: &water, Shrinkage: &shrinkage, ColorDifference: &color, Deformation: &deformation, SurfaceDefects: &defects}
	baselineSamples := []domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 100, Temperature: 1000}}
	if _, err = service.RecordAndEvaluateRun(project.ID, application.TrialRunCommand{ExpectedVersion: 3, IdempotencyKey: "candidate-run1", Role: string(domain.RoleTrialOperator), CurveRevisionID: baseline.ID, TemperatureSamples: baselineSamples, QualityMeasurements: quality, Operator: "试烧员"}); err != nil {
		t.Fatal(err)
	}

	_, err = service.ReviewProject(project.ID, application.ReviewCommand{ExpectedVersion: 4, IdempotencyKey: "candidate-early-review", Role: string(domain.RoleQualityReviewer), Reviewer: "质量复核员", Decision: "APPROVE"})
	var businessErr *domain.BusinessError
	if !errors.As(err, &businessErr) || businessErr.Code != domain.ErrState {
		t.Fatalf("整改状态下的提前批准应失败，实际为 %v", err)
	}

	derived, err := service.DeriveRevision(project.ID, baseline.ID, application.DeriveRevisionCommand{ExpectedVersion: 4, IdempotencyKey: "candidate-derive", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师"})
	if err != nil {
		t.Fatal(err)
	}
	revisedSegments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 900, DurationMinutes: 100}}
	if _, err = service.EditRevision(project.ID, derived.ID, application.EditRevisionCommand{ExpectedVersion: 5, IdempotencyKey: "candidate-edit", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师", Segments: revisedSegments}); err != nil {
		t.Fatal(err)
	}
	derived, err = service.FreezeRevision(project.ID, derived.ID, application.FreezeCommand{ExpectedVersion: 6, IdempotencyKey: "candidate-f2", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师"})
	if err != nil {
		t.Fatal(err)
	}

	detail, err := service.GetProject(project.ID)
	if err != nil || len(detail.Deviations) != 1 {
		t.Fatalf("预期一个待整改偏差，详情=%#v，错误=%v", detail, err)
	}
	deviation := detail.Deviations[0]
	if _, err = service.CorrectDeviation(project.ID, deviation.ID, application.CorrectionCommand{ExpectedVersion: 7, IdempotencyKey: "candidate-correct", Role: string(domain.RoleProcessEngineer), Actor: "工艺工程师", Cause: "峰值制度不匹配", CorrectiveAction: "调整曲线并定向复试", RelatedRevisionID: derived.ID}); err != nil {
		t.Fatal(err)
	}

	water = 0.2
	retestSamples := []domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 100, Temperature: 900}}
	if _, err = service.RecordAndEvaluateRun(project.ID, application.TrialRunCommand{ExpectedVersion: 8, IdempotencyKey: "candidate-retest", Role: string(domain.RoleTrialOperator), CurveRevisionID: derived.ID, ScopeCheckCodes: []string{deviation.CheckCode}, TemperatureSamples: retestSamples, QualityMeasurements: quality, Operator: "试烧员"}); err != nil {
		t.Fatal(err)
	}

	card, err := service.ReviewProject(project.ID, application.ReviewCommand{ExpectedVersion: 9, IdempotencyKey: "candidate-final-review", Role: string(domain.RoleQualityReviewer), Reviewer: "质量复核员", Decision: "APPROVE"})
	if err != nil {
		t.Fatal(err)
	}
	wantSnapshot, err := derived.CanonicalSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(card.CurveSnapshot, wantSnapshot) {
		t.Fatalf("最终工艺卡复用了失败事务缓存的旧曲线：card=%s want=%s", card.CurveSnapshot, wantSnapshot)
	}
}
