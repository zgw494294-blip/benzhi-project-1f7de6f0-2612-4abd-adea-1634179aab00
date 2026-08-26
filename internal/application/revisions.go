package application

import (
	"strings"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
)

func (s *Service) CompareRevisions(projectID, baselineRevisionID, comparisonRevisionID string) (*domain.CurveComparison, error) {
	if baselineRevisionID == "" {
		return nil, domain.NewError(domain.ErrInvalid, "基线修订不能为空", "baselineRevisionId")
	}
	if comparisonRevisionID == "" {
		return nil, domain.NewError(domain.ErrInvalid, "对比修订不能为空", "comparisonRevisionId")
	}
	if baselineRevisionID == comparisonRevisionID {
		return nil, domain.NewError(domain.ErrInvalid, "不能选择同一修订进行比较", "comparisonRevisionId")
	}
	cacheKey := strings.Join([]string{projectID, baselineRevisionID, comparisonRevisionID}, "\x00")
	if cached, ok := s.cachedRevisionComparison(cacheKey); ok {
		return cached, nil
	}
	var result *domain.CurveComparison
	err := s.repo.Read(func(st *store.State) error {
		if st.Projects[projectID] == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		baseline := st.Revisions[baselineRevisionID]
		if baseline == nil || baseline.ProjectID != projectID {
			return domain.NewError(domain.ErrInvalid, "基线修订不属于当前课题或不存在", "baselineRevisionId")
		}
		comparison := st.Revisions[comparisonRevisionID]
		if comparison == nil || comparison.ProjectID != projectID {
			return domain.NewError(domain.ErrInvalid, "对比修订不属于当前课题或不存在", "comparisonRevisionId")
		}
		value := domain.CompareRevisions(baseline, comparison)
		result = &value
		return nil
	})
	if err == nil {
		s.rememberRevisionComparison(cacheKey, result)
	}
	return result, err
}

func (s *Service) cachedRevisionComparison(key string) (*domain.CurveComparison, bool) {
	s.comparisonMu.RLock()
	value, ok := s.comparisonCache[key]
	s.comparisonMu.RUnlock()
	if !ok {
		return nil, false
	}
	cloned := cloneRevisionComparison(value)
	return &cloned, true
}

func (s *Service) rememberRevisionComparison(key string, value *domain.CurveComparison) {
	if value == nil {
		return
	}
	s.comparisonMu.Lock()
	defer s.comparisonMu.Unlock()
	if len(s.comparisonCache) >= 64 {
		clear(s.comparisonCache)
	}
	s.comparisonCache[key] = cloneRevisionComparison(*value)
}

func cloneRevisionComparison(value domain.CurveComparison) domain.CurveComparison {
	cloned := value
	cloned.SegmentDifferences = make([]domain.SegmentDifference, len(value.SegmentDifferences))
	copy(cloned.SegmentDifferences, value.SegmentDifferences)
	for i := range cloned.SegmentDifferences {
		if value.SegmentDifferences[i].Baseline != nil {
			segment := *value.SegmentDifferences[i].Baseline
			cloned.SegmentDifferences[i].Baseline = &segment
		}
		if value.SegmentDifferences[i].Comparison != nil {
			segment := *value.SegmentDifferences[i].Comparison
			cloned.SegmentDifferences[i].Comparison = &segment
		}
	}
	return cloned
}

type DeriveRevisionCommand struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Role            string `json:"role"`
	Actor           string `json:"actor"`
}

func (s *Service) DeriveRevision(projectID, baselineRevisionID string, c DeriveRevisionCommand) (*domain.FiringCurveRevision, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("derive-revision:"+projectID, c.IdempotencyKey)
	now := s.now()
	var result *domain.FiringCurveRevision
	err := s.repo.Update(store.CommitMeta{At: now, Actor: c.Actor, Action: "REVISION_DERIVED", ProjectID: projectID, EntityID: baselineRevisionID}, func(st *store.State) error {
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
		baseline := st.Revisions[baselineRevisionID]
		if baseline == nil || baseline.ProjectID != projectID || baseline.FreezeStatus != domain.CurveFrozen {
			return domain.NewError(domain.ErrInvalid, "只能从本课题冻结修订派生", "baselineRevisionId")
		}
		r := domain.NewRevision(s.id("curve"), projectID, len(p.RevisionIDs)+1, baseline.ID, c.Actor, baseline.Segments, now)
		st.Revisions[r.ID] = r
		p.RevisionIDs = append(p.RevisionIDs, r.ID)
		p.Touch(now)
		p.AddEvent(now, "REVISION_DERIVED", c.Actor, "从冻结修订版派生可编辑副本")
		st.Idempotency[key] = store.CommandResult{Command: "derive-revision", EntityID: r.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = r
		return nil
	})
	return result, err
}
