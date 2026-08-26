package application

import (
	"errors"
	"path/filepath"
	"testing"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func testService(t *testing.T) *Service {
	t.Helper()
	repo, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(repo)
}
func projectCommand() CreateProjectCommand {
	return CreateProjectCommand{IdempotencyKey: "create-1", Role: string(domain.RoleProcessEngineer), Code: "KC-1", Title: "测试课题", Owner: "工程师", BodyMaterial: "坯", GlazeMaterial: "釉", LoadingMethod: "平码", KilnLimits: domain.KilnLimits{MinTemperature: 20, MaxTemperature: 1300, MaxHeatingRate: 10, MaxCoolingRate: 8, MaxHoldMinutes: 100, MaxCycleMinutes: 500, TemperatureTolerance: 15}, QualityLimits: domain.QualityLimits{WaterAbsorption: domain.Range{Min: 0, Max: 0.5}, Shrinkage: domain.Range{Min: 10, Max: 15}, MaxColorDifference: 1.5, MaxDeformation: 1}}
}

func TestCreateProjectIsIdempotent(t *testing.T) {
	s := testService(t)
	first, err := s.CreateProject(projectCommand())
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateProject(projectCommand())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.Version != 1 {
		t.Fatalf("重复命令产生不同结果: %#v %#v", first, second)
	}
	list, _ := s.ListProjects()
	if len(list) != 1 {
		t.Fatalf("幂等重试产生了 %d 个课题", len(list))
	}
}

func TestRevisionRejectsStaleVersionAndWrongRole(t *testing.T) {
	s := testService(t)
	p, err := s.CreateProject(projectCommand())
	if err != nil {
		t.Fatal(err)
	}
	segments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}
	_, err = s.CreateRevision(p.ID, RevisionCommand{ExpectedVersion: 0, IdempotencyKey: "r-stale", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Segments: segments})
	var be *domain.BusinessError
	if !errors.As(err, &be) || be.Code != domain.ErrConflict {
		t.Fatalf("未返回乐观锁冲突: %v", err)
	}
	_, err = s.CreateRevision(p.ID, RevisionCommand{ExpectedVersion: p.Version, IdempotencyKey: "r-role", Role: string(domain.RoleTrialOperator), Actor: "操作员", Segments: segments})
	if !errors.As(err, &be) || be.Code != domain.ErrForbidden {
		t.Fatalf("错误角色未被拒绝: %v", err)
	}
}

func TestBoundaryRevisionRecordsOrderedChangesAndIdempotentAudit(t *testing.T) {
	s := testService(t)
	p, err := s.CreateProject(projectCommand())
	if err != nil {
		t.Fatal(err)
	}
	cmd := projectCommand()
	cmd.KilnLimits.MaxTemperature = 1350
	cmd.QualityLimits.WaterAbsorption.Max = 0.8
	update := ReviseBoundariesCommand{ExpectedVersion: p.Version, IdempotencyKey: "boundary-1", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Reason: "校正设备铭牌和质检标准", Owner: cmd.Owner, BodyMaterial: cmd.BodyMaterial, GlazeMaterial: cmd.GlazeMaterial, LoadingMethod: cmd.LoadingMethod, KilnLimits: cmd.KilnLimits, QualityLimits: cmd.QualityLimits}
	first, err := s.ReviseBoundaries(p.ID, update)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.ReviseBoundaries(p.ID, update)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 2 || second.Version != 2 || len(second.BoundaryHistory) != 1 {
		t.Fatalf("边界幂等结果异常: %#v", second)
	}
	changes := second.BoundaryHistory[0].Changes
	if len(changes) != 2 || changes[0].Field != "kilnLimits.maxTemperature" || changes[1].Field != "qualityLimits.waterAbsorption.max" {
		t.Fatalf("字段差异顺序异常: %#v", changes)
	}
	state, _ := s.repo.Snapshot()
	if len(state.Audits) != 2 {
		t.Fatalf("幂等重放产生了重复审计: %d", len(state.Audits))
	}
}

func TestDraftRunPersistsProgressAndEvaluatesOnce(t *testing.T) {
	s := testService(t)
	p, _ := s.CreateProject(projectCommand())
	segments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}
	revision, err := s.CreateRevision(p.ID, RevisionCommand{ExpectedVersion: 1, IdempotencyKey: "draft-r", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Segments: segments})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FreezeRevision(p.ID, revision.ID, FreezeCommand{ExpectedVersion: 2, IdempotencyKey: "draft-f", Role: string(domain.RoleProcessEngineer), Actor: "工程师"}); err != nil {
		t.Fatal(err)
	}
	run, err := s.StartTrialRun(p.ID, StartTrialRunCommand{ExpectedVersion: 3, IdempotencyKey: "draft-start", Role: string(domain.RoleTrialOperator), CurveRevisionID: revision.ID, Operator: "操作员"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartTrialRun(p.ID, StartTrialRunCommand{ExpectedVersion: 4, IdempotencyKey: "draft-duplicate", Role: string(domain.RoleTrialOperator), CurveRevisionID: revision.ID, Operator: "操作员"}); err == nil {
		t.Fatal("同一操作员重复创建相同范围的活动草稿未被拒绝")
	}
	samples := []domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 100, Temperature: 1000}}
	if _, err = s.SaveTrialEvidence(p.ID, run.ID, SaveTrialEvidenceCommand{ExpectedVersion: 4, IdempotencyKey: "draft-save-1", Role: string(domain.RoleTrialOperator), Operator: "操作员", TemperatureSamples: samples}); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.GetProject(p.ID)
	if detail.RunViews[0].EvidenceProgress.Percent != 16 || len(detail.Deviations) != 0 {
		t.Fatalf("部分草稿完整度或副作用异常: %#v", detail.RunViews[0])
	}
	_, err = s.CompleteTrialRun(p.ID, run.ID, CompleteTrialRunCommand{ExpectedVersion: 5, IdempotencyKey: "draft-complete", Role: string(domain.RoleTrialOperator), Operator: "操作员"})
	var be *domain.BusinessError
	if !errors.As(err, &be) || be.Field != "missingItems" {
		t.Fatalf("不完整草稿未返回缺失项: %v", err)
	}
	water, shrink, color, deformation, defects := 0.2, 12.0, 0.8, 0.4, false
	quality := domain.QualityMeasurements{WaterAbsorption: &water, Shrinkage: &shrink, ColorDifference: &color, Deformation: &deformation, SurfaceDefects: &defects}
	if _, err = s.SaveTrialEvidence(p.ID, run.ID, SaveTrialEvidenceCommand{ExpectedVersion: 5, IdempotencyKey: "draft-save-2", Role: string(domain.RoleTrialOperator), Operator: "操作员", TemperatureSamples: samples, QualityMeasurements: quality}); err != nil {
		t.Fatal(err)
	}
	completed, err := s.CompleteTrialRun(p.ID, run.ID, CompleteTrialRunCommand{ExpectedVersion: 6, IdempotencyKey: "draft-complete", Role: string(domain.RoleTrialOperator), Operator: "操作员"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != domain.RunEvaluated {
		t.Fatalf("草稿没有完成评估: %#v", completed)
	}
	state, _ := s.repo.Snapshot()
	audits := len(state.Audits)
	if _, err = s.CompleteTrialRun(p.ID, run.ID, CompleteTrialRunCommand{ExpectedVersion: 6, IdempotencyKey: "draft-complete", Role: string(domain.RoleTrialOperator), Operator: "操作员"}); err != nil {
		t.Fatal(err)
	}
	state, _ = s.repo.Snapshot()
	if len(state.Audits) != audits || len(state.Runs) != 1 {
		t.Fatal("完成动作幂等重试重复记账")
	}
}

func TestCorrectionBatchBuildsStableScopeAndResolvesTogether(t *testing.T) {
	s := testService(t)
	p, _ := s.CreateProject(projectCommand())
	segments := []domain.CurveSegment{{Order: 1, Kind: domain.SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}
	baseline, _ := s.CreateRevision(p.ID, RevisionCommand{ExpectedVersion: 1, IdempotencyKey: "batch-r1", Role: string(domain.RoleProcessEngineer), Actor: "工程师", Segments: segments})
	_, _ = s.FreezeRevision(p.ID, baseline.ID, FreezeCommand{ExpectedVersion: 2, IdempotencyKey: "batch-f1", Role: string(domain.RoleProcessEngineer), Actor: "工程师"})
	water, shrink, color, deformation, defects := 0.8, 12.0, 2.0, 0.4, false
	quality := domain.QualityMeasurements{WaterAbsorption: &water, Shrinkage: &shrink, ColorDifference: &color, Deformation: &deformation, SurfaceDefects: &defects}
	samples := []domain.TemperatureSample{{Minute: 0, Temperature: 20}, {Minute: 100, Temperature: 1000}}
	if _, err := s.RecordAndEvaluateRun(p.ID, TrialRunCommand{ExpectedVersion: 3, IdempotencyKey: "batch-run", Role: string(domain.RoleTrialOperator), CurveRevisionID: baseline.ID, TemperatureSamples: samples, QualityMeasurements: quality, Operator: "操作员"}); err != nil {
		t.Fatal(err)
	}
	detail, _ := s.GetProject(p.ID)
	if len(detail.Deviations) != 2 {
		t.Fatalf("预期两个开放偏差，得到 %d", len(detail.Deviations))
	}
	derived, err := s.DeriveRevision(p.ID, baseline.ID, DeriveRevisionCommand{ExpectedVersion: 4, IdempotencyKey: "batch-derive", Role: string(domain.RoleProcessEngineer), Actor: "工程师"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.FreezeRevision(p.ID, derived.ID, FreezeCommand{ExpectedVersion: 5, IdempotencyKey: "batch-f2", Role: string(domain.RoleProcessEngineer), Actor: "工程师"}); err != nil {
		t.Fatal(err)
	}
	ids := []string{detail.Deviations[1].ID, detail.Deviations[0].ID}
	batch, err := s.CreateCorrectionBatch(p.ID, CorrectionBatchCommand{ExpectedVersion: 6, IdempotencyKey: "batch-correct", Role: string(domain.RoleProcessEngineer), Actor: "工程师", DeviationIDs: ids, Cause: "共同工艺波动", CorrectiveAction: "调整后复试", RelatedRevisionID: derived.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.ScopeCheckCodes) != 2 || batch.ScopeCheckCodes[0] != "WATER_ABSORPTION" || batch.ScopeCheckCodes[1] != "COLOR_DIFFERENCE" {
		t.Fatalf("复试清单未稳定排序: %#v", batch.ScopeCheckCodes)
	}
	retest, err := s.StartTrialRun(p.ID, StartTrialRunCommand{ExpectedVersion: 7, IdempotencyKey: "batch-retest", Role: string(domain.RoleTrialOperator), CurveRevisionID: derived.ID, DeviationBatchID: batch.ID, Operator: "操作员"})
	if err != nil {
		t.Fatal(err)
	}
	water, color = 0.2, 0.8
	if _, err = s.SaveTrialEvidence(p.ID, retest.ID, SaveTrialEvidenceCommand{ExpectedVersion: 8, IdempotencyKey: "batch-evidence", Role: string(domain.RoleTrialOperator), Operator: "操作员", TemperatureSamples: samples, QualityMeasurements: quality}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteTrialRun(p.ID, retest.ID, CompleteTrialRunCommand{ExpectedVersion: 9, IdempotencyKey: "batch-finish", Role: string(domain.RoleTrialOperator), Operator: "操作员"}); err != nil {
		t.Fatal(err)
	}
	detail, _ = s.GetProject(p.ID)
	if detail.Project.Status != domain.ProjectReview || detail.DeviationBatches[0].Status != domain.DeviationBatchResolved {
		t.Fatalf("批次复试未整体关闭: %#v", detail.DeviationBatches[0])
	}
}
