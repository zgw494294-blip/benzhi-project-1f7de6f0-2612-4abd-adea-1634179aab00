package application

import (
	"encoding/json"
	"fmt"
	"sort"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type ProjectDetail struct {
	Project          *domain.TrialProject          `json:"project"`
	Revisions        []*domain.FiringCurveRevision `json:"revisions"`
	Runs             []*domain.TrialRun            `json:"runs"`
	Deviations       []*domain.Deviation           `json:"deviations"`
	DeviationBatches []*domain.DeviationBatch      `json:"deviationBatches"`
	ProcessCard      *domain.ProcessCard           `json:"processCard,omitempty"`
	CardIntegrity    *bool                         `json:"cardIntegrity,omitempty"`
	EvidenceSummary  verification.EvidenceSummary  `json:"evidenceSummary"`
	Audits           []store.AuditEvent            `json:"audits"`
	Overview         ProjectOverview               `json:"overview"`
	RevisionViews    []RevisionView                `json:"revisionViews"`
	RunViews         []RunView                     `json:"runViews"`
}

func (s *Service) ListProjects() ([]*domain.TrialProject, error) {
	out := []*domain.TrialProject{}
	err := s.repo.Read(func(st *store.State) error {
		for _, p := range st.Projects {
			out = append(out, p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
		return nil
	})
	return out, err
}

func (s *Service) GetProject(id string) (*ProjectDetail, error) {
	var out *ProjectDetail
	err := s.repo.Read(func(st *store.State) error {
		p := st.Projects[id]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		cacheKey := fmt.Sprintf("%s:%d", id, p.Version)
		if cached, ok := s.cachedProjectDetail(cacheKey); ok {
			// 缓存中保存的是只读快照，调用方修改返回结果不能污染后续 application 调用和 HTTP 响应，
			// 因此始终返回深拷贝副本，避免共享 Project.Title、Timeline、Overview 等可写字段。
			cloned, err := cloneProjectDetail(cached)
			if err != nil {
				return err
			}
			out = cloned
			return nil
		}
		d := &ProjectDetail{Project: p}
		for _, rid := range p.RevisionIDs {
			d.Revisions = append(d.Revisions, st.Revisions[rid])
		}
		for _, runID := range p.RunIDs {
			d.Runs = append(d.Runs, st.Runs[runID])
		}
		for _, did := range p.DeviationIDs {
			d.Deviations = append(d.Deviations, st.Deviations[did])
		}
		for _, batchID := range p.DeviationBatchIDs {
			d.DeviationBatches = append(d.DeviationBatches, st.DeviationBatches[batchID])
		}
		d.EvidenceSummary = verification.SummarizeEvidence(p, st.Runs, st.Deviations)
		d.Overview = buildOverview(p, d.Revisions, d.Deviations)
		d.RevisionViews = buildRevisionViews(d.Revisions)
		d.RunViews = buildRunViews(d.Runs)
		for _, event := range st.Audits {
			if event.ProjectID == id || event.EntityID == id {
				d.Audits = append(d.Audits, event)
			}
		}
		if p.ProcessCardID != "" {
			d.ProcessCard = st.Cards[p.ProcessCardID]
			ok, err := d.ProcessCard.Verify()
			if err != nil {
				return err
			}
			d.CardIntegrity = &ok
		}
		// 缓存只读快照，供同版本后续查询复用；返回给调用方的是深拷贝副本，二者不共享可写指针。
		s.rememberProjectDetail(cacheKey, d)
		cloned, err := cloneProjectDetail(d)
		if err != nil {
			return err
		}
		out = cloned
		return nil
	})
	return out, err
}

func cloneProjectDetail(detail *ProjectDetail) (*ProjectDetail, error) {
	b, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var out ProjectDetail
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Service) Diagnostics() (store.Statistics, error) {
	return s.repo.Inspect()
}

func (s *Service) VerifyCard(cardID string) (*domain.ProcessCard, bool, error) {
	var card *domain.ProcessCard
	valid := false
	err := s.repo.Read(func(st *store.State) error {
		card = st.Cards[cardID]
		if card == nil {
			return domain.NewError(domain.ErrNotFound, "工艺卡不存在", "cardId")
		}
		var err error
		valid, err = card.Verify()
		return err
	})
	return card, valid, err
}
