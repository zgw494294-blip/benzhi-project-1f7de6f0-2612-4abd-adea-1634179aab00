package revisioncomparisoncache

import (
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestRevisionComparisonCacheTracksEditedContents(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	project, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "create-project",
		Role:           string(domain.RoleProcessEngineer),
		Code:           "CACHE-EDIT",
		Title:          "修订对比缓存失效复现",
		Owner:          "工程师",
		BodyMaterial:   "坯体",
		GlazeMaterial:  "釉料",
		LoadingMethod:  "平码",
		KilnLimits: domain.KilnLimits{
			MinTemperature:       20,
			MaxTemperature:       1300,
			MaxHeatingRate:       20,
			MaxCoolingRate:       20,
			MaxHoldMinutes:       120,
			MaxCycleMinutes:      600,
			TemperatureTolerance: 15,
		},
		QualityLimits: domain.QualityLimits{
			WaterAbsorption:    domain.Range{Min: 0, Max: 1},
			Shrinkage:          domain.Range{Min: 8, Max: 16},
			MaxColorDifference: 2,
			MaxDeformation:     1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	baseline, err := service.CreateRevision(project.ID, application.RevisionCommand{
		ExpectedVersion: 1,
		IdempotencyKey:  "baseline",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "工程师",
		Segments: []domain.CurveSegment{{
			Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1020, DurationMinutes: 100,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := service.CreateRevision(project.ID, application.RevisionCommand{
		ExpectedVersion: 2,
		IdempotencyKey:  "comparison",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "工程师",
		Segments: []domain.CurveSegment{{
			Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1020, DurationMinutes: 120,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	first, err := service.CompareRevisions(project.ID, baseline.ID, comparison.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.MetricsDelta.TotalMinutes != 20 {
		t.Fatalf("首次对比基线异常：delta=%d", first.MetricsDelta.TotalMinutes)
	}

	_, err = service.EditRevision(project.ID, comparison.ID, application.EditRevisionCommand{
		ExpectedVersion: 3,
		IdempotencyKey:  "edit-comparison",
		Role:            string(domain.RoleProcessEngineer),
		Actor:           "工程师",
		Segments: []domain.CurveSegment{{
			Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1020, DurationMinutes: 150,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := service.CompareRevisions(project.ID, baseline.ID, comparison.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.MetricsDelta.TotalMinutes != 50 {
		t.Fatalf("编辑后的修订仍返回旧缓存：want delta=50, got delta=%d", second.MetricsDelta.TotalMinutes)
	}
}
