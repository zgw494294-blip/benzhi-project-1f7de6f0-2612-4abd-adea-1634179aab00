package curve_validation_result_alias_test

import (
	"errors"
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestCurveValidationResultCannotAuthorizeFreeze(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	project, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "create-alias-project",
		Role:           string(domain.RoleProcessEngineer),
		Code:           "ALIAS-001",
		Title:          "校验结果所有权边界复现",
		Owner:          "工程师",
		BodyMaterial:   "瓷坯",
		GlazeMaterial:  "透明釉",
		LoadingMethod:  "棚板平码",
		KilnLimits: domain.KilnLimits{
			MinTemperature:       20,
			MaxTemperature:       1300,
			MaxHeatingRate:       5,
			MaxCoolingRate:       8,
			MaxHoldMinutes:       120,
			MaxCycleMinutes:      500,
			TemperatureTolerance: 15,
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
	segments := []domain.CurveSegment{{
		Order:            1,
		Kind:             domain.SegmentHeat,
		StartTemperature: 20,
		EndTemperature:   1000,
		DurationMinutes:  100,
	}}
	revision, err := service.CreateRevision(project.ID, application.RevisionCommand{
		ExpectedVersion: project.Version,
		IdempotencyKey:  "create-alias-revision",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "工程师",
		Segments:        segments,
	})
	if err != nil {
		t.Fatal(err)
	}
	checks, err := service.ValidateCurve(project.ID, segments)
	if err != nil {
		t.Fatal(err)
	}
	foundFailure := false
	for index := range checks {
		if checks[index].Code == "HEATING_RATE" && !checks[index].Passed {
			foundFailure = true
			checks[index].Passed = true
		}
	}
	if !foundFailure {
		t.Fatalf("复现前置条件错误，未得到 HEATING_RATE 失败: %#v", checks)
	}
	_, err = service.FreezeRevision(project.ID, revision.ID, application.FreezeCommand{
		ExpectedVersion: project.Version + 1,
		IdempotencyKey:  "freeze-alias-revision",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "工程师",
	})
	var appErr *application.AppError
	if err == nil || !errors.As(err, &appErr) || appErr.Code != "CURVE_VALIDATION_FAILED" {
		snapshot, snapshotErr := repo.Snapshot()
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		persisted := snapshot.Revisions[revision.ID]
		t.Fatalf("调用方污染校验结果后，非法曲线未被拒绝且进入状态: err=%v freezeStatus=%s digest=%s", err, persisted.FreezeStatus, persisted.ContentDigest)
	}
}
