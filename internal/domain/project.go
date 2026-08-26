package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type BoundaryFieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

type BoundaryRevision struct {
	Version int                   `json:"version"`
	Reason  string                `json:"reason"`
	Actor   string                `json:"actor"`
	At      time.Time             `json:"at"`
	Changes []BoundaryFieldChange `json:"changes"`
}

type TrialProject struct {
	ID                string             `json:"id"`
	Code              string             `json:"code"`
	Title             string             `json:"title"`
	Owner             string             `json:"owner"`
	BodyMaterial      string             `json:"bodyMaterial"`
	GlazeMaterial     string             `json:"glazeMaterial"`
	LoadingMethod     string             `json:"loadingMethod"`
	KilnLimits        KilnLimits         `json:"kilnLimits"`
	QualityLimits     QualityLimits      `json:"qualityLimits"`
	Status            ProjectStatus      `json:"status"`
	Version           int                `json:"version"`
	CreatedAt         time.Time          `json:"createdAt"`
	UpdatedAt         time.Time          `json:"updatedAt"`
	RevisionIDs       []string           `json:"revisionIds"`
	RunIDs            []string           `json:"runIds"`
	DeviationIDs      []string           `json:"deviationIds"`
	ProcessCardID     string             `json:"processCardId,omitempty"`
	Timeline          []TimelineEvent    `json:"timeline"`
	BoundaryHistory   []BoundaryRevision `json:"boundaryHistory,omitempty"`
	DeviationBatchIDs []string           `json:"deviationBatchIds,omitempty"`
}

type NewProject struct {
	Code, Title, Owner, BodyMaterial, GlazeMaterial, LoadingMethod string
	KilnLimits                                                     KilnLimits
	QualityLimits                                                  QualityLimits
}

func CreateProject(id string, in NewProject, now time.Time) (*TrialProject, error) {
	if strings.TrimSpace(in.Code) == "" {
		return nil, NewError(ErrInvalid, "课题编号不能为空", "code")
	}
	if strings.TrimSpace(in.Title) == "" {
		return nil, NewError(ErrInvalid, "课题名称不能为空", "title")
	}
	if strings.TrimSpace(in.Owner) == "" {
		return nil, NewError(ErrInvalid, "负责人不能为空", "owner")
	}
	if strings.TrimSpace(in.BodyMaterial) == "" || strings.TrimSpace(in.GlazeMaterial) == "" {
		return nil, NewError(ErrInvalid, "坯体和釉料必须登记", "materials")
	}
	if strings.TrimSpace(in.LoadingMethod) == "" {
		return nil, NewError(ErrInvalid, "装窑方式不能为空", "loadingMethod")
	}
	if err := validateLimits(in.KilnLimits, in.QualityLimits); err != nil {
		return nil, err
	}
	p := &TrialProject{ID: id, Code: strings.TrimSpace(in.Code), Title: strings.TrimSpace(in.Title), Owner: strings.TrimSpace(in.Owner), BodyMaterial: strings.TrimSpace(in.BodyMaterial), GlazeMaterial: strings.TrimSpace(in.GlazeMaterial), LoadingMethod: strings.TrimSpace(in.LoadingMethod), KilnLimits: in.KilnLimits, QualityLimits: in.QualityLimits, Status: ProjectDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
	p.AddEvent(now, "PROJECT_CREATED", in.Owner, "建立试烧课题并登记适用边界")
	return p, nil
}

func validateLimits(k KilnLimits, q QualityLimits) error {
	if !finite(k.MinTemperature) || k.MinTemperature < -273.15 {
		return NewError(ErrInvalid, "窑炉最低温度必须为不低于绝对零度的有限数值", "kilnLimits.minTemperature")
	}
	if !finite(k.MaxTemperature) || k.MaxTemperature > 2500 {
		return NewError(ErrInvalid, "窑炉最高温度必须为不超过 2500℃ 的有限数值", "kilnLimits.maxTemperature")
	}
	if k.MaxTemperature <= k.MinTemperature {
		return NewError(ErrInvalid, "窑炉最高温度必须大于最低温度", "kilnLimits.maxTemperature")
	}
	if !finite(k.MaxHeatingRate) || k.MaxHeatingRate <= 0 {
		return NewError(ErrInvalid, "最大升温斜率必须为正有限数值", "kilnLimits.maxHeatingRate")
	}
	if !finite(k.MaxCoolingRate) || k.MaxCoolingRate <= 0 {
		return NewError(ErrInvalid, "最大降温斜率必须为正有限数值", "kilnLimits.maxCoolingRate")
	}
	if k.MaxHoldMinutes <= 0 {
		return NewError(ErrInvalid, "最大保温时长必须为正数", "kilnLimits.maxHoldMinutes")
	}
	if k.MaxCycleMinutes <= 0 {
		return NewError(ErrInvalid, "最大周期必须为正数", "kilnLimits.maxCycleMinutes")
	}
	if !finite(k.TemperatureTolerance) || k.TemperatureTolerance <= 0 {
		return NewError(ErrInvalid, "测温容差必须为正有限数值", "kilnLimits.temperatureTolerance")
	}
	if !finite(q.WaterAbsorption.Min) {
		return NewError(ErrInvalid, "吸水率下限必须为有限数值", "qualityLimits.waterAbsorption.min")
	}
	if !finite(q.WaterAbsorption.Max) || q.WaterAbsorption.Max < q.WaterAbsorption.Min {
		return NewError(ErrInvalid, "吸水率上限必须为有限数值且大于或等于下限", "qualityLimits.waterAbsorption.max")
	}
	if !finite(q.Shrinkage.Min) {
		return NewError(ErrInvalid, "收缩率下限必须为有限数值", "qualityLimits.shrinkage.min")
	}
	if !finite(q.Shrinkage.Max) || q.Shrinkage.Max < q.Shrinkage.Min {
		return NewError(ErrInvalid, "收缩率上限必须为有限数值且大于或等于下限", "qualityLimits.shrinkage.max")
	}
	if !finite(q.MaxColorDifference) || q.MaxColorDifference < 0 {
		return NewError(ErrInvalid, "最大色差必须为非负有限数值", "qualityLimits.maxColorDifference")
	}
	if !finite(q.MaxDeformation) || q.MaxDeformation < 0 {
		return NewError(ErrInvalid, "最大变形必须为非负有限数值", "qualityLimits.maxDeformation")
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

type BoundaryUpdate struct {
	Owner         string        `json:"owner"`
	BodyMaterial  string        `json:"bodyMaterial"`
	GlazeMaterial string        `json:"glazeMaterial"`
	LoadingMethod string        `json:"loadingMethod"`
	KilnLimits    KilnLimits    `json:"kilnLimits"`
	QualityLimits QualityLimits `json:"qualityLimits"`
}

func (p *TrialProject) ReviseBoundaries(in BoundaryUpdate, reason, actor string, now time.Time) ([]BoundaryFieldChange, error) {
	if p.Status != ProjectDraft {
		return nil, NewError(ErrState, "仅草拟状态课题允许修订适用边界", "status")
	}
	if strings.TrimSpace(reason) == "" {
		return nil, NewError(ErrInvalid, "变更原因不能为空", "reason")
	}
	if strings.TrimSpace(actor) == "" {
		return nil, NewError(ErrInvalid, "操作者不能为空", "actor")
	}
	if strings.TrimSpace(in.Owner) == "" {
		return nil, NewError(ErrInvalid, "负责人不能为空", "owner")
	}
	if strings.TrimSpace(in.BodyMaterial) == "" {
		return nil, NewError(ErrInvalid, "坯体不能为空", "bodyMaterial")
	}
	if strings.TrimSpace(in.GlazeMaterial) == "" {
		return nil, NewError(ErrInvalid, "釉料不能为空", "glazeMaterial")
	}
	if strings.TrimSpace(in.LoadingMethod) == "" {
		return nil, NewError(ErrInvalid, "装窑方式不能为空", "loadingMethod")
	}
	if err := validateLimits(in.KilnLimits, in.QualityLimits); err != nil {
		return nil, err
	}
	in.Owner = strings.TrimSpace(in.Owner)
	in.BodyMaterial = strings.TrimSpace(in.BodyMaterial)
	in.GlazeMaterial = strings.TrimSpace(in.GlazeMaterial)
	in.LoadingMethod = strings.TrimSpace(in.LoadingMethod)
	changes := boundaryDiff(p, in)
	if len(changes) == 0 {
		return nil, NewError(ErrInvalid, "适用边界没有实际变化", "boundaries")
	}
	p.Owner, p.BodyMaterial, p.GlazeMaterial, p.LoadingMethod = in.Owner, in.BodyMaterial, in.GlazeMaterial, in.LoadingMethod
	p.KilnLimits, p.QualityLimits = in.KilnLimits, in.QualityLimits
	p.Touch(now)
	p.BoundaryHistory = append(p.BoundaryHistory, BoundaryRevision{Version: p.Version, Reason: strings.TrimSpace(reason), Actor: strings.TrimSpace(actor), At: now, Changes: changes})
	p.AddEvent(now, "BOUNDARIES_REVISED", actor, fmt.Sprintf("修订适用边界（%d 个字段）：%s", len(changes), strings.TrimSpace(reason)))
	return changes, nil
}

func boundaryDiff(p *TrialProject, in BoundaryUpdate) []BoundaryFieldChange {
	type field struct {
		name      string
		old, next any
	}
	fields := []field{
		{"owner", p.Owner, in.Owner}, {"bodyMaterial", p.BodyMaterial, in.BodyMaterial}, {"glazeMaterial", p.GlazeMaterial, in.GlazeMaterial}, {"loadingMethod", p.LoadingMethod, in.LoadingMethod},
		{"kilnLimits.minTemperature", p.KilnLimits.MinTemperature, in.KilnLimits.MinTemperature}, {"kilnLimits.maxTemperature", p.KilnLimits.MaxTemperature, in.KilnLimits.MaxTemperature},
		{"kilnLimits.maxHeatingRate", p.KilnLimits.MaxHeatingRate, in.KilnLimits.MaxHeatingRate}, {"kilnLimits.maxCoolingRate", p.KilnLimits.MaxCoolingRate, in.KilnLimits.MaxCoolingRate},
		{"kilnLimits.maxHoldMinutes", p.KilnLimits.MaxHoldMinutes, in.KilnLimits.MaxHoldMinutes}, {"kilnLimits.maxCycleMinutes", p.KilnLimits.MaxCycleMinutes, in.KilnLimits.MaxCycleMinutes},
		{"kilnLimits.temperatureTolerance", p.KilnLimits.TemperatureTolerance, in.KilnLimits.TemperatureTolerance},
		{"qualityLimits.waterAbsorption.min", p.QualityLimits.WaterAbsorption.Min, in.QualityLimits.WaterAbsorption.Min}, {"qualityLimits.waterAbsorption.max", p.QualityLimits.WaterAbsorption.Max, in.QualityLimits.WaterAbsorption.Max},
		{"qualityLimits.shrinkage.min", p.QualityLimits.Shrinkage.Min, in.QualityLimits.Shrinkage.Min}, {"qualityLimits.shrinkage.max", p.QualityLimits.Shrinkage.Max, in.QualityLimits.Shrinkage.Max},
		{"qualityLimits.maxColorDifference", p.QualityLimits.MaxColorDifference, in.QualityLimits.MaxColorDifference}, {"qualityLimits.maxDeformation", p.QualityLimits.MaxDeformation, in.QualityLimits.MaxDeformation},
		{"qualityLimits.allowSurfaceDefects", p.QualityLimits.AllowSurfaceDefects, in.QualityLimits.AllowSurfaceDefects},
	}
	out := make([]BoundaryFieldChange, 0)
	for _, f := range fields {
		if reflect.DeepEqual(f.old, f.next) {
			continue
		}
		out = append(out, BoundaryFieldChange{Field: f.name, OldValue: jsonValue(f.old), NewValue: jsonValue(f.next)})
	}
	return out
}

func jsonValue(v any) string { b, _ := json.Marshal(v); return string(b) }

func (p *TrialProject) RequireVersion(expected int) error {
	if expected != p.Version {
		return VersionConflict(p.Version)
	}
	return nil
}
func (p *TrialProject) Touch(now time.Time) { p.Version++; p.UpdatedAt = now }
func (p *TrialProject) AddEvent(at time.Time, kind, actor, summary string) {
	p.Timeline = append(p.Timeline, TimelineEvent{At: at, Type: kind, Actor: actor, Summary: summary})
}
func (p *TrialProject) RequireStatus(allowed ...ProjectStatus) error {
	for _, s := range allowed {
		if p.Status == s {
			return nil
		}
	}
	return NewError(ErrState, fmt.Sprintf("课题状态 %s 不允许此操作", p.Status), "status")
}
