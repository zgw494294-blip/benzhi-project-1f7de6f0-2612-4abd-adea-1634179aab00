package application

import (
	"fmt"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type ReviewCommand struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Role            string `json:"role"`
	Reviewer        string `json:"reviewer"`
	Decision        string `json:"decision"`
	Comment         string `json:"comment"`
}

func (s *Service) ReviewProject(projectID string, c ReviewCommand) (*domain.ProcessCard, error) {
	if err := domain.RequireRole(c.Role, domain.RoleQualityReviewer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("review-project", []string{projectID}, c.IdempotencyKey)
	now := s.now()
	var result *domain.ProcessCard
	err := s.repo.Update(store.CommitMeta{At: now, Actor: c.Reviewer, Action: "PROJECT_REVIEWED", ProjectID: projectID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			if prior.EntityID != "" {
				result = st.Cards[prior.EntityID]
			}
			return nil
		}
		p := st.Projects[projectID]
		if p == nil {
			return domain.NewError(domain.ErrNotFound, "课题不存在", "projectId")
		}
		if err := p.RequireVersion(c.ExpectedVersion); err != nil {
			return err
		}
		if c.Reviewer == "" {
			return domain.NewError(domain.ErrInvalid, "质量复核员不能为空", "reviewer")
		}
		if c.Decision == "RETURN" {
			if err := p.RequireStatus(domain.ProjectReview); err != nil {
				return err
			}
			if c.Comment == "" {
				return domain.NewError(domain.ErrInvalid, "退回意见不能为空", "comment")
			}
			p.Status = domain.ProjectCorrection
			p.Touch(now)
			p.AddEvent(now, "REVIEW_RETURNED", c.Reviewer, "质量复核退回："+c.Comment)
			st.Idempotency[key] = store.CommandResult{Command: "review-project", ProjectID: p.ID, Version: p.Version, CompletedAt: now}
			return nil
		}
		if c.Decision != "APPROVE" {
			return domain.NewError(domain.ErrInvalid, "复核决定必须为 APPROVE 或 RETURN", "decision")
		}
		if err := p.RequireStatus(domain.ProjectReview); err != nil {
			return err
		}
		if err := verification.ReviewReady(p, st.Runs, st.Deviations); err != nil {
			return err
		}
		curve := latestFrozen(st, p)
		if curve == nil {
			return domain.NewError(domain.ErrState, "没有可签发的冻结曲线", "revisions")
		}
		refs := make([]string, 0, len(p.RunIDs))
		for _, id := range p.RunIDs {
			refs = append(refs, "trial-run:"+id)
		}
		card, err := domain.IssueCard(s.id("card"), fmt.Sprintf("KC-%s-%03d", p.Code, len(st.Cards)+1), p, curve, refs, c.Reviewer, now)
		if err != nil {
			return err
		}
		st.Cards[card.ID] = card
		p.ProcessCardID = card.ID
		p.Status = domain.ProjectApproved
		p.Touch(now)
		p.AddEvent(now, "PROCESS_CARD_ISSUED", c.Reviewer, "批准并签发工艺卡 "+card.CardNumber)
		st.Idempotency[key] = store.CommandResult{Command: "review-project", EntityID: card.ID, ProjectID: p.ID, Version: p.Version, CompletedAt: now}
		result = card
		return nil
	})
	return result, err
}

func latestFrozen(st *store.State, p *domain.TrialProject) *domain.FiringCurveRevision {
	for i := len(p.RevisionIDs) - 1; i >= 0; i-- {
		if r := st.Revisions[p.RevisionIDs[i]]; r != nil && r.FreezeStatus == domain.CurveFrozen {
			return r
		}
	}
	return nil
}
