package application

import (
	"context"
	"strconv"
	"strings"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type CorrectionBatchCommand struct {
	ExpectedVersion   int      `json:"expectedVersion"`
	IdempotencyKey    string   `json:"idempotencyKey"`
	Role              string   `json:"role"`
	Actor             string   `json:"actor"`
	DeviationIDs      []string `json:"deviationIds"`
	Cause             string   `json:"cause"`
	CorrectiveAction  string   `json:"correctiveAction"`
	RelatedRevisionID string   `json:"relatedRevisionId"`
}

func (s *Service) CreateCorrectionBatch(projectID string, c CorrectionBatchCommand, ctxs ...context.Context) (*domain.DeviationBatch, error) {
	if err := domain.RequireRole(c.Role, domain.RoleProcessEngineer); err != nil {
		return nil, err
	}
	if err := requireKey(c.IdempotencyKey); err != nil {
		return nil, err
	}
	key := idemKey("create-correction-batch:"+projectID, c.IdempotencyKey)
	now := s.now()
	batchID := s.id("batch")
	var result *domain.DeviationBatch
	err := s.repo.UpdateCtx(contextFrom(ctxs), store.CommitMeta{At: now, Actor: c.Actor, Action: "DEVIATION_BATCH_CREATED", ProjectID: projectID, EntityID: batchID}, func(st *store.State) error {
		if prior, ok := st.Idempotency[key]; ok {
			result = st.DeviationBatches[prior.EntityID]
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
		if len(c.DeviationIDs) == 0 {
			return domain.NewError(domain.ErrInvalid, "整改批次至少选择一个偏差", "deviationIds")
		}
		revision := st.Revisions[c.RelatedRevisionID]
		if revision == nil || revision.ProjectID != projectID || revision.FreezeStatus != domain.CurveFrozen || revision.BasedOnRevisionID == "" {
			return domain.NewError(domain.ErrInvalid, "关联修订必须是本课题已冻结的派生版本", "relatedRevisionId")
		}
		seenIDs := map[string]bool{}
		selected := make([]*domain.Deviation, 0, len(c.DeviationIDs))
		wantedCodes := map[string]bool{}
		for index, deviationID := range c.DeviationIDs {
			if seenIDs[deviationID] {
				return domain.NewError(domain.ErrInvalid, "整改批次不能重复选择偏差", "deviationIds")
			}
			seenIDs[deviationID] = true
			deviation := st.Deviations[deviationID]
			if deviation == nil || !contains(p.DeviationIDs, deviationID) {
				return domain.NewError(domain.ErrInvalid, "整改批次包含不属于当前课题的偏差", "deviationIds")
			}
			if deviation.Status != domain.DeviationOpen {
				return domain.NewError(domain.ErrState, "整改批次第 "+strconv.Itoa(index+1)+" 项偏差不是 OPEN 状态", "deviationIds")
			}
			run := st.Runs[deviation.TrialRunID]
			if run == nil || run.CurveRevisionID != revision.BasedOnRevisionID {
				return domain.NewError(domain.ErrInvalid, "关联修订必须从所选偏差的失败冻结版本派生", "relatedRevisionId")
			}
			selected = append(selected, deviation)
			wantedCodes[deviation.CheckCode] = true
		}
		scope := make([]string, 0, len(wantedCodes))
		for _, code := range verification.AllCheckCodes() {
			if wantedCodes[code] {
				scope = append(scope, code)
			}
		}
		batch, err := domain.NewDeviationBatch(batchID, projectID, c.DeviationIDs, strings.TrimSpace(c.Cause), strings.TrimSpace(c.CorrectiveAction), revision.ID, strings.TrimSpace(c.Actor), scope, now)
		if err != nil {
			return err
		}
		for _, deviation := range selected {
			if err := deviation.CorrectInBatch(batch.ID, batch.Cause, batch.CorrectiveAction, revision.ID); err != nil {
				return err
			}
		}
		st.DeviationBatches[batch.ID] = batch
		p.DeviationBatchIDs = append(p.DeviationBatchIDs, batch.ID)
		p.Touch(now)
		p.AddEvent(now, "DEVIATION_BATCH_CREATED", c.Actor, "建立整改批次，纳入 "+strconv.Itoa(len(selected))+" 项偏差")
		st.Idempotency[key] = store.CommandResult{Command: "create-correction-batch", EntityID: batch.ID, ProjectID: projectID, Version: p.Version, CompletedAt: now}
		result = batch
		return nil
	})
	return result, err
}
