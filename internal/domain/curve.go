package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type SegmentKind string

const (
	SegmentHeat SegmentKind = "HEAT"
	SegmentHold SegmentKind = "HOLD"
	SegmentCool SegmentKind = "COOL"
)

type CurveSegment struct {
	Order            int         `json:"order"`
	Kind             SegmentKind `json:"kind"`
	StartTemperature float64     `json:"startTemperature"`
	EndTemperature   float64     `json:"endTemperature"`
	DurationMinutes  int         `json:"durationMinutes"`
}

type FiringCurveRevision struct {
	ID                string         `json:"id"`
	ProjectID         string         `json:"projectId"`
	RevisionNo        int            `json:"revisionNo"`
	BasedOnRevisionID string         `json:"basedOnRevisionId,omitempty"`
	Segments          []CurveSegment `json:"segments"`
	FreezeStatus      FreezeStatus   `json:"freezeStatus"`
	ContentDigest     string         `json:"contentDigest,omitempty"`
	CreatedBy         string         `json:"createdBy"`
	CreatedAt         time.Time      `json:"createdAt"`
	FrozenAt          *time.Time     `json:"frozenAt,omitempty"`
}

func NewRevision(id, projectID string, no int, basedOn, actor string, segments []CurveSegment, now time.Time) *FiringCurveRevision {
	return &FiringCurveRevision{ID: id, ProjectID: projectID, RevisionNo: no, BasedOnRevisionID: basedOn, Segments: append([]CurveSegment(nil), segments...), FreezeStatus: CurveEditable, CreatedBy: actor, CreatedAt: now}
}

func (r *FiringCurveRevision) ReplaceSegments(segments []CurveSegment) error {
	if r.FreezeStatus == CurveFrozen {
		return NewError(ErrState, "已冻结曲线不可原位修改", "freezeStatus")
	}
	r.Segments = append([]CurveSegment(nil), segments...)
	return nil
}

func (r *FiringCurveRevision) Freeze(now time.Time) error {
	if r.FreezeStatus == CurveFrozen {
		return NewError(ErrState, "曲线已经冻结", "freezeStatus")
	}
	if len(r.Segments) == 0 {
		return NewError(ErrInvalid, "曲线至少需要一个分段", "segments")
	}
	b, err := r.CanonicalSnapshot()
	if err != nil {
		return err
	}
	h := sha256.Sum256(b)
	r.ContentDigest = hex.EncodeToString(h[:])
	r.FreezeStatus = CurveFrozen
	r.FrozenAt = &now
	return nil
}

func (r *FiringCurveRevision) CanonicalSnapshot() ([]byte, error) {
	type snapshot struct {
		ProjectID  string         `json:"projectId"`
		RevisionNo int            `json:"revisionNo"`
		BasedOn    string         `json:"basedOn"`
		Segments   []CurveSegment `json:"segments"`
	}
	return json.Marshal(snapshot{ProjectID: r.ProjectID, RevisionNo: r.RevisionNo, BasedOn: r.BasedOnRevisionID, Segments: r.Segments})
}
