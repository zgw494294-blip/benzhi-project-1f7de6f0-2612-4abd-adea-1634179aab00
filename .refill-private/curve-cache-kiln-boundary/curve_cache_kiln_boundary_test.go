package curvecachekilnboundary_test

import (
	"errors"
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestCurveValidationCacheCannotCrossKilnLimits(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)

	loose := projectCommand("create-loose", "KC-CACHE-LOOSE", 10)
	looseProject, err := service.CreateProject(loose)
	if err != nil {
		t.Fatal(err)
	}
	strict := projectCommand("create-strict", "KC-CACHE-STRICT", 5)
	strictProject, err := service.CreateProject(strict)
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
	looseRevision, err := service.CreateRevision(looseProject.ID, application.RevisionCommand{
		ExpectedVersion: 1,
		IdempotencyKey:  "revision-loose",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "宽松边界工程师",
		Segments:        segments,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.FreezeRevision(looseProject.ID, looseRevision.ID, application.FreezeCommand{
		ExpectedVersion: 2,
		IdempotencyKey:  "freeze-loose",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "宽松边界工程师",
	}); err != nil {
		t.Fatalf("宽松边界曲线应能冻结: %v", err)
	}

	strictRevision, err := service.CreateRevision(strictProject.ID, application.RevisionCommand{
		ExpectedVersion: 1,
		IdempotencyKey:  "revision-strict",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "严格边界工程师",
		Segments:        segments,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.FreezeRevision(strictProject.ID, strictRevision.ID, application.FreezeCommand{
		ExpectedVersion: 2,
		IdempotencyKey:  "freeze-strict",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "严格边界工程师",
	})
	if err == nil {
		detail, readErr := service.GetProject(strictProject.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		t.Fatalf("严格边界下 9.8℃/min 的曲线被错误冻结并持久化，课题状态=%s", detail.Project.Status)
	}
	var appErr *application.AppError
	if !errors.As(err, &appErr) || appErr.Code != "CURVE_VALIDATION_FAILED" {
		t.Fatalf("严格边界应返回 CURVE_VALIDATION_FAILED，实际为 %v", err)
	}
	detail, err := service.GetProject(strictProject.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Project.Status != domain.ProjectDraft || detail.Revisions[0].FreezeStatus != domain.CurveEditable {
		t.Fatalf("校验失败后不应提交状态: project=%s revision=%s", detail.Project.Status, detail.Revisions[0].FreezeStatus)
	}
}

func projectCommand(key, code string, maxHeatingRate float64) application.CreateProjectCommand {
	return application.CreateProjectCommand{
		IdempotencyKey: key,
		Role:           string(domain.RoleProcessEngineer),
		Code:           code,
		Title:          "曲线缓存边界隔离复现",
		Owner:          "工艺工程师",
		BodyMaterial:   "测试坯体",
		GlazeMaterial:  "测试釉料",
		LoadingMethod:  "平码装窑",
		KilnLimits: domain.KilnLimits{
			MinTemperature:       20,
			MaxTemperature:       1300,
			MaxHeatingRate:       maxHeatingRate,
			MaxCoolingRate:       8,
			MaxHoldMinutes:       100,
			MaxCycleMinutes:      500,
			TemperatureTolerance: 15,
		},
		QualityLimits: domain.QualityLimits{
			WaterAbsorption:    domain.Range{Min: 0, Max: 0.5},
			Shrinkage:          domain.Range{Min: 10, Max: 15},
			MaxColorDifference: 1.5,
			MaxDeformation:     1,
		},
	}
}
