package application

import (
	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

type ReviseBoundariesCommand struct {
	ExpectedVersion int                  `json:"expectedVersion"`
	IdempotencyKey  string               `json:"idempotencyKey"`
	Role            string               `json:"role"`
	Actor           string               `json:"actor"`
	Reason          string               `json:"reason"`
	Owner           string               `json:"owner"`
	BodyMaterial    string               `json:"bodyMaterial"`
	GlazeMaterial   string               `json:"glazeMaterial"`
	LoadingMethod   string               `json:"loadingMethod"`
	KilnLimits      domain.KilnLimits    `json:"kilnLimits"`
	QualityLimits   domain.QualityLimits `json:"qualityLimits"`
}

func (s *Service) ReviseBoundaries(projectID string, c ReviseBoundariesCommand) (*domain.TrialProject, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("revise-boundaries:"+projectID, c.IdempotencyKey)
	now := s.now()
	var result *domain.TrialProject
	err := s.repo.Update(store.CommitMeta{At: now, Actor: c.Actor, Action: "BOUNDARIES_REVISED", ProjectID: projectID, EntityID: projectID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Projects[prior.ProjectID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		for _, revisionID := range p.RevisionIDs {
			if revision := st.Revisions[revisionID]; revision != nil && revision.FreezeStatus == domain.CurveFrozen {
				return domain.NewError(domain.ErrState, "已有冻结曲线，不能追溯修改适用边界", "status")
			}
		}
		_, err := p.ReviseBoundaries(domain.BoundaryUpdate{Owner: c.Owner, BodyMaterial: c.BodyMaterial, GlazeMaterial: c.GlazeMaterial, LoadingMethod: c.LoadingMethod, KilnLimits: c.KilnLimits, QualityLimits: c.QualityLimits}, c.Reason, c.Actor, now)
		if err != nil {
			return err
		}
		st.Idempotency[key] = store.CommandResult{Command: "revise-boundaries", EntityID: p.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = p
		return nil
	})
	return result, err
}
