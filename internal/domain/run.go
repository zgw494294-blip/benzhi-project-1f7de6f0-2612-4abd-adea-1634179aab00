package domain

import (
	"fmt"
	"strings"
	"time"
)

type TemperatureSample struct {
	Minute      int     `json:"minute"`
	Temperature float64 `json:"temperature"`
}
type QualityMeasurements struct {
	WaterAbsorption *float64 `json:"waterAbsorption"`
	Shrinkage       *float64 `json:"shrinkage"`
	ColorDifference *float64 `json:"colorDifference"`
	Deformation     *float64 `json:"deformation"`
	SurfaceDefects  *bool    `json:"surfaceDefects"`
}

type TrialRun struct {
	ID                  string              `json:"id"`
	ProjectID           string              `json:"projectId"`
	CurveRevisionID     string              `json:"curveRevisionId"`
	RunNo               int                 `json:"runNo"`
	ScopeCheckCodes     []string            `json:"scopeCheckCodes,omitempty"`
	DeviationBatchID    string              `json:"deviationBatchId,omitempty"`
	TemperatureSamples  []TemperatureSample `json:"temperatureSamples"`
	QualityMeasurements QualityMeasurements `json:"qualityMeasurements"`
	Operator            string              `json:"operator"`
	StartedAt           time.Time           `json:"startedAt"`
	CompletedAt         *time.Time          `json:"completedAt,omitempty"`
	Status              RunStatus           `json:"status"`
	Version             int                 `json:"version"`
	UpdatedAt           time.Time           `json:"updatedAt"`
	Checks              []CheckResult       `json:"checks,omitempty"`
}

func NewTrialRun(id, projectID, revisionID string, no int, scope []string, operator string, at time.Time) (*TrialRun, error) {
	if strings.TrimSpace(operator) == "" {
		return nil, NewError(ErrInvalid, "试烧操作员不能为空", "operator")
	}
	return &TrialRun{ID: id, ProjectID: projectID, CurveRevisionID: revisionID, RunNo: no, ScopeCheckCodes: append([]string(nil), scope...), Operator: strings.TrimSpace(operator), StartedAt: at, UpdatedAt: at, Status: RunRecording, Version: 1}, nil
}

func (r *TrialRun) Record(samples []TemperatureSample, quality QualityMeasurements) error {
	if r.Status != RunRecording {
		return NewError(ErrState, "已评估轮次不能修改证据", "status")
	}
	if len(samples) < 2 {
		return NewError(ErrInvalid, "至少需要两个测温点", "temperatureSamples")
	}
	if err := validateEvidenceValues(samples, quality, 0); err != nil {
		return err
	}
	r.TemperatureSamples = append([]TemperatureSample(nil), samples...)
	r.QualityMeasurements = quality
	return nil
}

func (r *TrialRun) SaveEvidence(samples []TemperatureSample, quality QualityMeasurements, totalMinutes int, at time.Time) error {
	if r.Status != RunRecording {
		return NewError(ErrState, "已评估轮次不能修改证据", "status")
	}
	if err := validateEvidenceValues(samples, quality, totalMinutes); err != nil {
		return err
	}
	r.TemperatureSamples = append([]TemperatureSample(nil), samples...)
	r.QualityMeasurements = quality
	r.Version++
	r.UpdatedAt = at
	return nil
}

func validateEvidenceValues(samples []TemperatureSample, quality QualityMeasurements, totalMinutes int) error {
	last := -1
	for i, sample := range samples {
		if sample.Minute < 0 {
			return NewError(ErrInvalid, "测温分钟不能为负数", fmt.Sprintf("temperatureSamples[%d].minute", i))
		}
		if i > 0 && sample.Minute <= last {
			return NewError(ErrInvalid, "测温分钟必须严格递增且不能重复", fmt.Sprintf("temperatureSamples[%d].minute", i))
		}
		if totalMinutes > 0 && sample.Minute > totalMinutes {
			return NewError(ErrInvalid, "测温点不能越过曲线总周期", fmt.Sprintf("temperatureSamples[%d].minute", i))
		}
		if !finite(sample.Temperature) {
			return NewError(ErrInvalid, "测温值必须为有限数值", fmt.Sprintf("temperatureSamples[%d].temperature", i))
		}
		last = sample.Minute
	}
	qualityFields := []struct {
		name  string
		value *float64
	}{{"waterAbsorption", quality.WaterAbsorption}, {"shrinkage", quality.Shrinkage}, {"colorDifference", quality.ColorDifference}, {"deformation", quality.Deformation}}
	for _, field := range qualityFields {
		if field.value != nil && !finite(*field.value) {
			return NewError(ErrInvalid, "质检值必须为有限数值", "qualityMeasurements."+field.name)
		}
	}
	return nil
}

type EvidenceProgress struct {
	CompletedItems []string `json:"completedItems"`
	MissingItems   []string `json:"missingItems"`
	Percent        int      `json:"percent"`
	CanComplete    bool     `json:"canComplete"`
}

func (r *TrialRun) EvidenceProgress() EvidenceProgress {
	items := []struct {
		name     string
		complete bool
	}{
		{"temperatureSamples", len(r.TemperatureSamples) >= 2},
		{"qualityMeasurements.waterAbsorption", r.QualityMeasurements.WaterAbsorption != nil},
		{"qualityMeasurements.shrinkage", r.QualityMeasurements.Shrinkage != nil},
		{"qualityMeasurements.colorDifference", r.QualityMeasurements.ColorDifference != nil},
		{"qualityMeasurements.deformation", r.QualityMeasurements.Deformation != nil},
		{"qualityMeasurements.surfaceDefects", r.QualityMeasurements.SurfaceDefects != nil},
	}
	progress := EvidenceProgress{CompletedItems: []string{}, MissingItems: []string{}}
	for _, item := range items {
		if item.complete {
			progress.CompletedItems = append(progress.CompletedItems, item.name)
		} else {
			progress.MissingItems = append(progress.MissingItems, item.name)
		}
	}
	progress.Percent = len(progress.CompletedItems) * 100 / len(items)
	progress.CanComplete = len(progress.MissingItems) == 0
	return progress
}

func (r *TrialRun) RequireCompleteEvidence() error {
	progress := r.EvidenceProgress()
	if !progress.CanComplete {
		return NewError(ErrInvalid, "试烧证据不完整，缺少："+strings.Join(progress.MissingItems, "、"), "missingItems")
	}
	return nil
}

func (r *TrialRun) Complete(checks []CheckResult, at time.Time) error {
	if r.Status != RunRecording {
		return NewError(ErrState, "轮次已经评估", "status")
	}
	r.Checks = append([]CheckResult(nil), checks...)
	r.Status = RunEvaluated
	r.CompletedAt = &at
	r.UpdatedAt = at
	r.Version++
	return nil
}
