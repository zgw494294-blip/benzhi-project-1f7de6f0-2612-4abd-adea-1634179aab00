package frozen_digest_reload_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestReloadRejectsTamperedFrozenCurveDigest(t *testing.T) {
	dir := t.TempDir()
	repo, err := store.Open(filepath.Join(dir, "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	project, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "create", Role: string(domain.RoleProcessEngineer), Code: "DIGEST-1", Title: "摘要测试", Owner: "工程师", BodyMaterial: "坯", GlazeMaterial: "釉", LoadingMethod: "平码",
		KilnLimits:    domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 120, MaxCycleMinutes: 500, TemperatureTolerance: 10},
		QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 1}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	revision, err := service.CreateRevision(project.ID, application.RevisionCommand{ExpectedVersion: 1, IdempotencyKey: "revision", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Segments: []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.FreezeRevision(project.ID, revision.ID, application.FreezeCommand{ExpectedVersion: 2, IdempotencyKey: "freeze", Role: string(domain.RoleProcessEngineer), Actor: "工程师"}); err != nil {
		t.Fatal(err)
	}
	state, err := repo.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	state.Revisions[revision.ID].Segments[0].EndTemperature = 900
	tampered, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	tamperedPath := filepath.Join(dir, "tampered.json")
	if err = os.WriteFile(tamperedPath, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Open(tamperedPath); err == nil {
		t.Fatal("篡改冻结曲线内容且保留旧摘要的快照被成功加载")
	}
}
