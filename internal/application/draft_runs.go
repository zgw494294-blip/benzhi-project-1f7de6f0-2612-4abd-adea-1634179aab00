package application

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type StartTrialRunCommand struct {
	ExpectedVersion  int      `json:"expectedVersion"`
	IdempotencyKey   string   `json:"idempotencyKey"`
	Role             string   `json:"role"`
	CurveRevisionID  string   `json:"curveRevisionId"`
	ScopeCheckCodes  []string `json:"scopeCheckCodes,omitempty"`
	DeviationBatchID string   `json:"deviationBatchId,omitempty"`
	Operator         string   `json:"operator"`
}

func (s *Service) StartTrialRun(projectID string, c StartTrialRunCommand, ctxs ...context.Context) (*domain.TrialRun, error) {
	if err := domain.RequireRole(c.Role, domain.RoleTrialOperator); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("start-trial-run:"+projectID, c.IdempotencyKey)
	now := s.now()
	operator := strings.TrimSpace(c.Operator)
	runID := s.id("run")
	var result *domain.TrialRun
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Operator, Action: "TRIAL_RUN_STARTED", ProjectID: projectID, EntityID: runID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Runs[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if err := p.RequireStatus(domain.ProjectCurveFrozen, domain.ProjectCorrection); err != nil {
			return err
		}
		curve := st.Revisions[c.CurveRevisionID]
		if curve == nil || curve.ProjectID != projectID || curve.FreezeStatus != domain.CurveFrozen {
			return domain.NewError(domain.ErrInvalid, "试烧草稿必须锁定本课题冻结曲线", "curveRevisionId")
		}
		scope, err := resolveDraftScope(st, p, c.DeviationBatchID, c.ScopeCheckCodes)
		if err != nil {
			return err
		}
		for _, runID := range p.RunIDs {
			run := st.Runs[runID]
			if run != nil && run.Status == domain.RunRecording && run.Operator == operator && equalStrings(run.ScopeCheckCodes, scope) {
				return domain.NewError(domain.ErrState, "同一操作员已存在相同范围的活动试烧草稿", "scopeCheckCodes")
			}
		}
		run, err := domain.NewTrialRun(runID, projectID, curve.ID, len(p.RunIDs)+1, scope, operator, now)
		if err != nil {
			return err
		}
		run.DeviationBatchID = c.DeviationBatchID
		st.Runs[run.ID] = run
		p.RunIDs = append(p.RunIDs, run.ID)
		if p.Status == domain.ProjectCurveFrozen {
			p.Status = domain.ProjectTesting
		}
		p.Touch(now)
		p.AddEvent(now, "TRIAL_RUN_STARTED", c.Operator, fmt.Sprintf("开始第 %d 轮试烧证据草稿", run.RunNo))
		st.Idempotency[key] = store.CommandResult{Command: "start-trial-run", EntityID: run.ID, ProjectID: projectID, Version: p.Version, CompletedAt: now}
		result = run
		return nil
	})
	return result, err
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func resolveDraftScope(st *store.State, p *domain.TrialProject, batchID string, requested []string) ([]string, error) {
	if batchID != "" {
		batch := st.DeviationBatches[batchID]
		if batch == nil || batch.ProjectID != p.ID {
			return nil, domain.NewError(domain.ErrInvalid, "整改批次不属于当前课题或不存在", "deviationBatchId")
		}
		scope, err := verification.BuildBatchRetestScope(batch, deviationsForProject(st, p))
		if err != nil {
			return nil, err
		}
		if len(requested) > 0 && !reflect.DeepEqual(requested, scope) {
			return nil, domain.NewError(domain.ErrInvalid, "定向复试范围必须由整改批次自动生成", "scopeCheckCodes")
		}
		return scope, nil
	}
	if len(requested) == 0 {
		return []string{}, nil
	}
	if err := verification.ValidateRetestScope(requested, deviationsForProject(st, p)); err != nil {
		return nil, err
	}
	return append([]string(nil), requested...), nil
}

type SaveTrialEvidenceCommand struct {
	ExpectedVersion     int                        `json:"expectedVersion"`
	IdempotencyKey      string                     `json:"idempotencyKey"`
	Role                string                     `json:"role"`
	Operator            string                     `json:"operator"`
	TemperatureSamples  []domain.TemperatureSample `json:"temperatureSamples"`
	QualityMeasurements domain.QualityMeasurements `json:"qualityMeasurements"`
}

func (s *Service) SaveTrialEvidence(projectID, runID string, c SaveTrialEvidenceCommand, ctxs ...context.Context) (*domain.TrialRun, error) {
	if err := domain.RequireRole(c.Role, domain.RoleTrialOperator); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("save-trial-evidence:"+projectID, c.IdempotencyKey)
	now := s.now()
	var result *domain.TrialRun
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Operator, Action: "TRIAL_EVIDENCE_SAVED", ProjectID: projectID, EntityID: runID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Runs[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		run := st.Runs[runID]
		if run == nil || run.ProjectID != projectID {
			return domain.NewError(domain.ErrNotFound, "试烧草稿不存在", "runId")
		}
		if run.Operator != strings.TrimSpace(c.Operator) {
			return domain.NewError(domain.ErrForbidden, "只能由草稿锁定的操作员保存证据", "operator")
		}
		curve := st.Revisions[run.CurveRevisionID]
		totalMinutes := domain.CalculateCurveMetrics(curve.Segments).TotalMinutes
		if err := run.SaveEvidence(c.TemperatureSamples, c.QualityMeasurements, totalMinutes, now); err != nil {
			return err
		}
		p.Touch(now)
		p.AddEvent(now, "TRIAL_EVIDENCE_SAVED", c.Operator, fmt.Sprintf("保存第 %d 轮试烧证据草稿（完整度 %d%%）", run.RunNo, run.EvidenceProgress().Percent))
		st.Idempotency[key] = store.CommandResult{Command: "save-trial-evidence", EntityID: run.ID, ProjectID: projectID, Version: p.Version, CompletedAt: now}
		result = run
		return nil
	})
	return result, err
}

type CompleteTrialRunCommand struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Role            string `json:"role"`
	Operator        string `json:"operator"`
}

func (s *Service) CompleteTrialRun(projectID, runID string, c CompleteTrialRunCommand, ctxs ...context.Context) (*domain.TrialRun, error) {
	if err := domain.RequireRole(c.Role, domain.RoleTrialOperator); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("complete-trial-run:"+projectID, c.IdempotencyKey)
	now := s.now()
	var result *domain.TrialRun
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Operator, Action: "TRIAL_RUN_EVALUATED", ProjectID: projectID, EntityID: runID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Runs[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		run := st.Runs[runID]
		if run == nil || run.ProjectID != projectID {
			return domain.NewError(domain.ErrNotFound, "试烧草稿不存在", "runId")
		}
		if run.Operator != strings.TrimSpace(c.Operator) {
			return domain.NewError(domain.ErrForbidden, "只能由草稿锁定的操作员完成评估", "operator")
		}
		if err := run.RequireCompleteEvidence(); err != nil {
			return err
		}
		curve := st.Revisions[run.CurveRevisionID]
		checks, err := s.evidence.Evaluate(curve, run, p)
		if err != nil {
			return err
		}
		if err := run.Complete(checks, now); err != nil {
			return err
		}
		failed := failedChecks(checks)
		if len(run.ScopeCheckCodes) == 0 {
			for _, check := range checks {
				if !check.Passed {
					deviation := domain.NewDeviation(s.id("deviation"), run.ID, check)
					st.Deviations[deviation.ID] = deviation
					p.DeviationIDs = append(p.DeviationIDs, deviation.ID)
				}
			}
		} else if run.DeviationBatchID != "" {
			if err := applyBatchRetest(st, p, run, failed, now); err != nil {
				return err
			}
		} else {
			applyLegacyRetest(deviationsForProject(st, p), run, failed, now)
		}
		applyPostEvaluationStatus(st, p)
		p.Touch(now)
		p.AddEvent(now, "TRIAL_RUN_EVALUATED", c.Operator, fmt.Sprintf("完成第 %d 轮试烧评估：%d 项检查，%d 项失败", run.RunNo, len(checks), len(failed)))
		st.Idempotency[key] = store.CommandResult{Command: "complete-trial-run", EntityID: run.ID, ProjectID: projectID, Version: p.Version, CompletedAt: now}
		result = run
		return nil
	})
	return result, err
}

func failedChecks(checks []domain.CheckResult) map[string]domain.CheckResult {
	failed := map[string]domain.CheckResult{}
	for _, check := range checks {
		if !check.Passed {
			failed[check.Code] = check
		}
	}
	return failed
}

func applyLegacyRetest(deviations []*domain.Deviation, run *domain.TrialRun, failed map[string]domain.CheckResult, now time.Time) {
	for _, deviation := range deviations {
		if !contains(run.ScopeCheckCodes, deviation.CheckCode) || deviation.Status != domain.DeviationActioned {
			continue
		}
		deviation.RetestRunID = run.ID
		if check, ok := failed[deviation.CheckCode]; ok {
			deviation.Status = domain.DeviationOpen
			deviation.ResolvedAt = nil
			deviation.ObservedValue = check.Observed
			deviation.AllowedBoundary = check.Boundary
		} else {
			deviation.Resolve(now)
		}
	}
}

func applyBatchRetest(st *store.State, p *domain.TrialProject, run *domain.TrialRun, failed map[string]domain.CheckResult, now time.Time) error {
	batch := st.DeviationBatches[run.DeviationBatchID]
	if batch == nil || batch.ProjectID != p.ID {
		return domain.NewError(domain.ErrInvalid, "试烧关联的整改批次无效", "deviationBatchId")
	}
	resolved, open := map[string]bool{}, map[string]bool{}
	for _, deviationID := range batch.DeviationIDs {
		deviation := st.Deviations[deviationID]
		if deviation == nil || deviation.BatchID != batch.ID || deviation.Status != domain.DeviationActioned {
			continue
		}
		deviation.RetestRunID = run.ID
		if check, ok := failed[deviation.CheckCode]; ok {
			deviation.Status = domain.DeviationOpen
			deviation.ResolvedAt = nil
			deviation.ObservedValue, deviation.AllowedBoundary = check.Observed, check.Boundary
			open[deviation.CheckCode] = true
		} else {
			deviation.Resolve(now)
			resolved[deviation.CheckCode] = true
		}
	}
	batch.LatestRetestRunID = run.ID
	batch.ResolvedCodes = orderedCodes(resolved)
	batch.OpenCodes = orderedCodes(open)
	if len(batch.OpenCodes) == 0 {
		batch.Status = domain.DeviationBatchResolved
	} else {
		batch.Status = domain.DeviationBatchNeedsAction
	}
	return nil
}

func orderedCodes(values map[string]bool) []string {
	out := []string{}
	for _, code := range verification.AllCheckCodes() {
		if values[code] {
			out = append(out, code)
		}
	}
	unknown := []string{}
	for code := range values {
		if !contains(out, code) {
			unknown = append(unknown, code)
		}
	}
	sort.Strings(unknown)
	return append(out, unknown...)
}
