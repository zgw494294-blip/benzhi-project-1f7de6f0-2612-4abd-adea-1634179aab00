package project_detail_cache_alias_test

import (
	"path/filepath"
	"testing"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func TestProjectDetailCacheDoesNotShareCallerMutations(t *testing.T) {
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	service := application.NewService(repo)
	project, err := service.CreateProject(application.CreateProjectCommand{
		IdempotencyKey: "create-cache-alias-project",
		Role:           string(domain.RoleProcessEngineer),
		Code:           "CACHE-ALIAS-001",
		Title:          "缓存所有权课题",
		Owner:          "工艺工程师",
		BodyMaterial:   "高岭土坯体",
		GlazeMaterial:  "透明釉",
		LoadingMethod:  "棚板平码",
		KilnLimits: domain.KilnLimits{
			MinTemperature: 20, MaxTemperature: 1300,
			MaxHeatingRate: 10, MaxCoolingRate: 8,
			MaxHoldMinutes: 180, MaxCycleMinutes: 600,
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
		t.Fatalf("创建课题: %v", err)
	}

	first, err := service.GetProject(project.ID)
	if err != nil {
		t.Fatalf("首次查询: %v", err)
	}
	first.Project.Title = "调用方伪造标题"
	first.Project.Timeline[0].Summary = "调用方伪造时间线"
	first.Overview.StatusLabel = "调用方伪造状态"

	second, err := service.GetProject(project.ID)
	if err != nil {
		t.Fatalf("再次查询: %v", err)
	}
	if second.Project.Title != "缓存所有权课题" || second.Project.Timeline[0].Summary != "建立试烧课题并登记适用边界" || second.Overview.StatusLabel != "草拟" {
		t.Fatalf("后续查询复用了被调用方修改的缓存对象: title=%q timeline=%q status=%q", second.Project.Title, second.Project.Timeline[0].Summary, second.Overview.StatusLabel)
	}
}
