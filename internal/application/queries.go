package application

import (
	"context"
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

func (s *Service) ListProjects(ctxs ...context.Context) ([]*domain.TrialProject, error) {
	out := []*domain.TrialProject{}
	err := s.repo.ReadCtx(contextFrom(ctxs), func(st *store.State) error {
		for _, p := range st.Projects {
			out = append(out, p)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
		return nil
	})
	return out, err
}

func (s *Service) GetProject(id string, ctxs ...context.Context) (*ProjectDetail, error) {
	var out *ProjectDetail
	err := s.repo.ReadCtx(contextFrom(ctxs), func(st *store.State) error {
		p := st.Projects[id]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
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
		out = d
		return nil
	})
	return out, err
}

func (s *Service) Diagnostics() (store.Statistics, error) {
	return s.repo.Inspect()
}

func (s *Service) VerifyCard(cardID string, ctxs ...context.Context) (*domain.ProcessCard, bool, error) {
	var card *domain.ProcessCard
	valid := false
	err := s.repo.ReadCtx(contextFrom(ctxs), func(st *store.State) error {
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
