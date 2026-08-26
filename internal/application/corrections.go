package application

import (
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

type CorrectionCommand struct {
	ExpectedVersion   int    `json:"expectedVersion"`
	IdempotencyKey    string `json:"idempotencyKey"`
	Role              string `json:"role"`
	Actor             string `json:"actor"`
	Cause             string `json:"cause"`
	CorrectiveAction  string `json:"correctiveAction"`
	RelatedRevisionID string `json:"relatedRevisionId"`
}

func (s *Service) CorrectDeviation(projectID, deviationID string, c CorrectionCommand) (*domain.Deviation, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("correct-deviation", c.IdempotencyKey)
	now := s.now()
	var result *domain.Deviation
	err := s.repo.Update(store.CommitMeta{At: now, Actor: c.Actor, Action: "DEVIATION_CORRECTED", ProjectID: projectID, EntityID: deviationID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Deviations[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if err := p.RequireStatus(domain.ProjectCorrection); err != nil {
			return err
		}
		d := st.Deviations[deviationID]
		if d == nil || !contains(p.DeviationIDs, deviationID) {
			return domain.NewError(domain.ErrNotFound, "偏差不存在", "deviationId")
		}
		if c.RelatedRevisionID != "" {
			r := st.Revisions[c.RelatedRevisionID]
			if r == nil || r.ProjectID != projectID {
				return domain.NewError(domain.ErrInvalid, "关联曲线修订无效", "relatedRevisionId")
			}
		}
		if err := d.Correct(c.Cause, c.CorrectiveAction, c.RelatedRevisionID); err != nil {
			return err
		}
		p.Touch(now)
		p.AddEvent(now, "DEVIATION_CORRECTED", c.Actor, "登记偏差 "+d.CheckCode+" 原因与纠正动作")
		st.Idempotency[key] = store.CommandResult{Command: "correct-deviation", EntityID: d.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = d
		return nil
	})
	return result, err
}
