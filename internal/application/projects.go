package application

import (
	"context"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type CreateProjectCommand struct {
	IdempotencyKey string               `json:"idempotencyKey"`
	Role           string               `json:"role"`
	Code           string               `json:"code"`
	Title          string               `json:"title"`
	Owner          string               `json:"owner"`
	BodyMaterial   string               `json:"bodyMaterial"`
	GlazeMaterial  string               `json:"glazeMaterial"`
	LoadingMethod  string               `json:"loadingMethod"`
	KilnLimits     domain.KilnLimits    `json:"kilnLimits"`
	QualityLimits  domain.QualityLimits `json:"qualityLimits"`
}

func (s *Service) CreateProject(c CreateProjectCommand, ctxs ...context.Context) (*domain.TrialProject, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("create-project", c.IdempotencyKey)
	now := s.now()
	var result *domain.TrialProject
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Owner, Action: "PROJECT_CREATED"}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Projects[prior.ProjectID]
			return nil
		}
		for _, p := range st.Projects {
			if p.Code == c.Code {
				return appError("PROJECT_CODE_EXISTS", "课题编号已存在", 409)
			}
		}
		p, err := domain.CreateProject(s.id("project"), domain.NewProject{Code: c.Code, Title: c.Title, Owner: c.Owner, BodyMaterial: c.BodyMaterial, GlazeMaterial: c.GlazeMaterial, LoadingMethod: c.LoadingMethod, KilnLimits: c.KilnLimits, QualityLimits: c.QualityLimits}, now)
		if err != nil {
			return err
		}
		st.Projects[p.ID] = p
		st.Idempotency[key] = store.CommandResult{Command: "create-project", EntityID: p.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = p
		return nil
	})
	return result, err
}

type RevisionCommand struct {
	ExpectedVersion   int                   `json:"expectedVersion"`
	IdempotencyKey    string                `json:"idempotencyKey"`
	Role              string                `json:"role"`
	Actor             string                `json:"actor"`
	BasedOnRevisionID string                `json:"basedOnRevisionId,omitempty"`
	Segments          []domain.CurveSegment `json:"segments"`
}

func (s *Service) CreateRevision(projectID string, c RevisionCommand, ctxs ...context.Context) (*domain.FiringCurveRevision, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("create-revision", c.IdempotencyKey)
	now := s.now()
	var result *domain.FiringCurveRevision
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Actor, Action: "REVISION_CREATED", ProjectID: projectID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Revisions[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if err := p.RequireStatus(domain.ProjectDraft, domain.ProjectCorrection); err != nil {
			return err
		}
		if c.BasedOnRevisionID != "" {
			base := st.Revisions[c.BasedOnRevisionID]
			if base == nil || base.ProjectID != projectID || base.FreezeStatus != domain.CurveFrozen {
				return domain.NewError(domain.ErrInvalid, "来源曲线必须是本课题已冻结版本", "basedOnRevisionId")
			}
		}
		no := len(p.RevisionIDs) + 1
		r := domain.NewRevision(s.id("curve"), projectID, no, c.BasedOnRevisionID, c.Actor, c.Segments, now)
		st.Revisions[r.ID] = r
		p.RevisionIDs = append(p.RevisionIDs, r.ID)
		p.Touch(now)
		p.AddEvent(now, "REVISION_CREATED", c.Actor, "创建曲线修订版")
		st.Idempotency[key] = store.CommandResult{Command: "create-revision", EntityID: r.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = r
		return nil
	})
	return result, err
}

type EditRevisionCommand struct {
	ExpectedVersion int                   `json:"expectedVersion"`
	IdempotencyKey  string                `json:"idempotencyKey"`
	Role            string                `json:"role"`
	Actor           string                `json:"actor"`
	Segments        []domain.CurveSegment `json:"segments"`
}

func (s *Service) EditRevision(projectID, revisionID string, c EditRevisionCommand, ctxs ...context.Context) (*domain.FiringCurveRevision, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("edit-revision", c.IdempotencyKey)
	now := s.now()
	var result *domain.FiringCurveRevision
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Actor, Action: "REVISION_EDITED", ProjectID: projectID, EntityID: revisionID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Revisions[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if err := p.RequireStatus(domain.ProjectDraft, domain.ProjectCorrection); err != nil {
			return err
		}
		r := st.Revisions[revisionID]
		if r == nil || r.ProjectID != projectID {
			return domain.NewError(domain.ErrNotFound, "曲线修订不存在", "revisionId")
		}
		if err := r.ReplaceSegments(c.Segments); err != nil {
			return err
		}
		p.Touch(now)
		p.AddEvent(now, "REVISION_EDITED", c.Actor, "更新曲线分段")
		st.Idempotency[key] = store.CommandResult{Command: "edit-revision", EntityID: r.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = r
		return nil
	})
	return result, err
}

type FreezeCommand struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Role            string `json:"role"`
	Actor           string `json:"actor"`
}

func (s *Service) FreezeRevision(projectID, revisionID string, c FreezeCommand, ctxs ...context.Context) (*domain.FiringCurveRevision, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("freeze-revision", c.IdempotencyKey)
	now := s.now()
	var result *domain.FiringCurveRevision
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Actor, Action: "REVISION_FROZEN", ProjectID: projectID, EntityID: revisionID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.Revisions[prior.EntityID]
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if err := p.RequireStatus(domain.ProjectDraft, domain.ProjectCorrection); err != nil {
			return err
		}
		r := st.Revisions[revisionID]
		if r == nil || r.ProjectID != projectID {
			return domain.NewError(domain.ErrNotFound, "曲线修订不存在", "revisionId")
		}
		checks := s.curves.Validate(r.Segments, p.KilnLimits)
		if verification.HasFailures(checks) {
			return appErrorWithDetails("CURVE_VALIDATION_FAILED", "曲线未通过全部窑炉能力校验", "segments", 422, map[string]any{"checks": checks})
		}
		wasCorrection := p.Status == domain.ProjectCorrection
		if err := r.Freeze(now); err != nil {
			return err
		}
		if wasCorrection {
			p.Status = domain.ProjectCorrection
		} else {
			p.Status = domain.ProjectCurveFrozen
		}
		p.Touch(now)
		p.AddEvent(now, "REVISION_FROZEN", c.Actor, "冻结曲线修订版 "+r.ContentDigest[:12])
		st.Idempotency[key] = store.CommandResult{Command: "freeze-revision", EntityID: r.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = r
		return nil
	})
	return result, err
}

func (s *Service) ValidateCurve(projectID string, segments []domain.CurveSegment, ctxs ...context.Context) ([]domain.CheckResult, error) {
	var out []domain.CheckResult
	err := s.repo.ReadCtx(contextFrom(ctxs), func(st *store.State) error {
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		out = s.curves.Validate(segments, p.KilnLimits)
		return nil
	})
	return out, err
}
