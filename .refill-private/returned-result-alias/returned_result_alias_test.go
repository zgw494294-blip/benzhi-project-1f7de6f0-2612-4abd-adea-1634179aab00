package returned_result_alias_test

import (
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestReturnedCommandResultCannotMutateRepository(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	created, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "create", Role: string(domain.RoleProcessEngineer),
		Code: "ALIAS-1", Title: "原始标题", Owner: "工程师",
		BodyMaterial: "坯", GlazeMaterial: "釉", LoadingMethod: "平码",
		KilnLimits:    domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 120, MaxCycleMinutes: 500, TemperatureTolerance: 10},
		QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 1}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	created.Title = "调用方篡改"
	detail, err := service.GetProject(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Project.Title != "原始标题" {
		t.Fatalf("返回对象泄漏了仓储内部别名，读取到标题 %q", detail.Project.Title)
	}
}
