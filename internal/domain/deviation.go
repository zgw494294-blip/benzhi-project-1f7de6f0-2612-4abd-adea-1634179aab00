package domain

import (
	"strings"
	"time"
)

type Deviation struct {
	ID                string          `json:"id"`
	TrialRunID        string          `json:"trialRunId"`
	CheckCode         string          `json:"checkCode"`
	Severity          string          `json:"severity"`
	ObservedValue     string          `json:"observedValue"`
	AllowedBoundary   string          `json:"allowedBoundary"`
	Cause             string          `json:"cause,omitempty"`
	CorrectiveAction  string          `json:"correctiveAction,omitempty"`
	RelatedRevisionID string          `json:"relatedRevisionId,omitempty"`
	BatchID           string          `json:"batchId,omitempty"`
	RetestRunID       string          `json:"retestRunId,omitempty"`
	Status            DeviationStatus `json:"status"`
	ResolvedAt        *time.Time      `json:"resolvedAt,omitempty"`
}

func NewDeviation(id, runID string, check CheckResult) *Deviation {
	return &Deviation{ID: id, TrialRunID: runID, CheckCode: check.Code, Severity: "MAJOR", ObservedValue: check.Observed, AllowedBoundary: check.Boundary, Status: DeviationOpen}
}

func (d *Deviation) Correct(cause, action, revisionID string) error {
	if d.Status != DeviationOpen {
		return NewError(ErrState, "偏差已处置或关闭", "status")
	}
	if cause == "" || action == "" {
		return NewError(ErrInvalid, "偏差原因和纠正动作不能为空", "correctiveAction")
	}
	d.Cause, d.CorrectiveAction, d.RelatedRevisionID, d.Status = cause, action, revisionID, DeviationActioned
	return nil
}

func (d *Deviation) CorrectInBatch(batchID, cause, action, revisionID string) error {
	if err := d.Correct(cause, action, revisionID); err != nil {
		return err
	}
	d.BatchID = batchID
	return nil
}

func (d *Deviation) LinkRetest(runID string) error {
	if d.Status != DeviationActioned {
		return NewError(ErrState, "偏差尚未完成原因与动作登记", "status")
	}
	d.RetestRunID = runID
	return nil
}

func (d *Deviation) Resolve(at time.Time) { d.Status = DeviationResolved; d.ResolvedAt = &at }

type DeviationBatchStatus string

const (
	DeviationBatchActioned    DeviationBatchStatus = "ACTIONED"
	DeviationBatchNeedsAction DeviationBatchStatus = "NEEDS_ACTION"
	DeviationBatchResolved    DeviationBatchStatus = "RESOLVED"
)

type DeviationBatch struct {
	ID                string               `json:"id"`
	ProjectID         string               `json:"projectId"`
	DeviationIDs      []string             `json:"deviationIds"`
	Cause             string               `json:"cause"`
	CorrectiveAction  string               `json:"correctiveAction"`
	RelatedRevisionID string               `json:"relatedRevisionId"`
	ScopeCheckCodes   []string             `json:"scopeCheckCodes"`
	Status            DeviationBatchStatus `json:"status"`
	CreatedBy         string               `json:"createdBy"`
	CreatedAt         time.Time            `json:"createdAt"`
	ResolvedCodes     []string             `json:"resolvedCodes,omitempty"`
	OpenCodes         []string             `json:"openCodes,omitempty"`
	LatestRetestRunID string               `json:"latestRetestRunId,omitempty"`
}

func NewDeviationBatch(id, projectID string, deviationIDs []string, cause, action, revisionID, actor string, scope []string, at time.Time) (*DeviationBatch, error) {
	if len(deviationIDs) == 0 {
		return nil, NewError(ErrInvalid, "整改批次至少选择一个偏差", "deviationIds")
	}
	if strings.TrimSpace(cause) == "" {
		return nil, NewError(ErrInvalid, "共同原因不能为空", "cause")
	}
	if strings.TrimSpace(action) == "" {
		return nil, NewError(ErrInvalid, "共同纠正动作不能为空", "correctiveAction")
	}
	if strings.TrimSpace(actor) == "" {
		return nil, NewError(ErrInvalid, "操作者不能为空", "actor")
	}
	return &DeviationBatch{ID: id, ProjectID: projectID, DeviationIDs: append([]string(nil), deviationIDs...), Cause: strings.TrimSpace(cause), CorrectiveAction: strings.TrimSpace(action), RelatedRevisionID: revisionID, ScopeCheckCodes: append([]string(nil), scope...), Status: DeviationBatchActioned, CreatedBy: strings.TrimSpace(actor), CreatedAt: at}, nil
}
