package idempotency_cross_project_test

import (
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestRevisionIdempotencyCannotCrossProjectBoundary(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	create := func(key, code string) *domain.TrialProject {
		project, createErr := service.CreateProject(application.CreateProjectCommand{
			IdempotencyKey: key, Role: string(domain.RoleProcessEngineer), Code: code,
			Title: code, Owner: "工程师", BodyMaterial: "坯", GlazeMaterial: "釉", LoadingMethod: "平码",
			KilnLimits:    domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 120, MaxCycleMinutes: 500, TemperatureTolerance: 10},
			QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 1}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1},
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return project
	}
	firstProject := create("project-1", "IDEM-1")
	secondProject := create("project-2", "IDEM-2")
	segments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}
	command := application.RevisionCommand{ExpectedVersion: 1, IdempotencyKey: "same-client-key", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Segments: segments}
	if _, err = service.CreateRevision(firstProject.ID, command); err != nil {
		t.Fatal(err)
	}
	secondRevision, err := service.CreateRevision(secondProject.ID, command)
	if err != nil {
		t.Fatal(err)
	}
	if secondRevision == nil || secondRevision.ProjectID != secondProject.ID {
		t.Fatalf("第二个课题错误复用了第一个课题的幂等结果: %#v", secondRevision)
	}
}
