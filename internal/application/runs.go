package application

import (
	"context"
	"fmt"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type TrialRunCommand struct {
	ExpectedVersion     int                        `json:"expectedVersion"`
	IdempotencyKey      string                     `json:"idempotencyKey"`
	Role                string                     `json:"role"`
	CurveRevisionID     string                     `json:"curveRevisionId"`
	ScopeCheckCodes     []string                   `json:"scopeCheckCodes,omitempty"`
	TemperatureSamples  []domain.TemperatureSample `json:"temperatureSamples"`
	QualityMeasurements domain.QualityMeasurements `json:"qualityMeasurements"`
	Operator            string                     `json:"operator"`
}

func (s *Service) RecordAndEvaluateRun(projectID string, c TrialRunCommand, ctxs ...context.Context) (*domain.TrialRun, error) {
	if err := domain.RequireRole(c.Role, domain.RoleTrialOperator); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("evaluate-run", c.IdempotencyKey)
	now := s.now()
	var result *domain.TrialRun
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Operator, Action: "TRIAL_RUN_EVALUATED", ProjectID: projectID}, func(st *store.State) error {
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
			return domain.NewError(domain.ErrInvalid, "试烧必须关联本课题冻结曲线", "curveRevisionId")
		}
		projectDeviations := deviationsForProject(st, p)
		if len(c.ScopeCheckCodes) > 0 {
			if err := verification.ValidateRetestScope(c.ScopeCheckCodes, projectDeviations); err != nil {
				return err
			}
		}
		r, err := domain.NewTrialRun(s.id("run"), projectID, curve.ID, len(p.RunIDs)+1, c.ScopeCheckCodes, c.Operator, now)
		if err != nil {
			return err
		}
		if err = r.Record(c.TemperatureSamples, c.QualityMeasurements); err != nil {
			return err
		}
		checks, err := s.evidence.Evaluate(curve, r, p)
		if err != nil {
			return err
		}
		if err = r.Complete(checks, now); err != nil {
			return err
		}
		st.Runs[r.ID] = r
		p.RunIDs = append(p.RunIDs, r.ID)
		failed := map[string]domain.CheckResult{}
		for _, check := range checks {
			if !check.Passed {
				failed[check.Code] = check
			}
		}
		if len(c.ScopeCheckCodes) == 0 {
			for _, check := range checks {
				if !check.Passed {
					d := domain.NewDeviation(s.id("deviation"), r.ID, check)
					st.Deviations[d.ID] = d
					p.DeviationIDs = append(p.DeviationIDs, d.ID)
				}
			}
		} else {
			for _, d := range projectDeviations {
				if contains(c.ScopeCheckCodes, d.CheckCode) && d.Status == domain.DeviationActioned {
					d.RetestRunID = r.ID
					if check, ok := failed[d.CheckCode]; ok {
						d.Status = domain.DeviationOpen
						d.ObservedValue = check.Observed
						d.AllowedBoundary = check.Boundary
					} else {
						d.Resolve(now)
					}
				}
			}
		}
		applyPostEvaluationStatus(st, p)
		p.Touch(now)
		summary := fmt.Sprintf("完成第 %d 轮试烧评估：%d 项检查，%d 项失败", r.RunNo, len(checks), len(failed))
		p.AddEvent(now, "TRIAL_RUN_EVALUATED", c.Operator, summary)
		st.Idempotency[key] = store.CommandResult{Command: "evaluate-run", EntityID: r.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = r
		return nil
	})
	return result, err
}

func deviationsForProject(st *store.State, p *domain.TrialProject) []*domain.Deviation {
	out := make([]*domain.Deviation, 0, len(p.DeviationIDs))
	for _, id := range p.DeviationIDs {
		if d := st.Deviations[id]; d != nil {
			out = append(out, d)
		}
	}
	return out
}
func allResolved(st *store.State, p *domain.TrialProject) bool {
	for _, id := range p.DeviationIDs {
		if d := st.Deviations[id]; d == nil || d.Status != domain.DeviationResolved {
			return false
		}
	}
	return true
}
func applyPostEvaluationStatus(st *store.State, p *domain.TrialProject) {
	if !allResolved(st, p) {
		p.Status = domain.ProjectCorrection
		return
	}
	for _, runID := range p.RunIDs {
		if run := st.Runs[runID]; run != nil && run.Status == domain.RunRecording {
			p.Status = domain.ProjectTesting
			return
		}
	}
	p.Status = domain.ProjectReview
}
func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
