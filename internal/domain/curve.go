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

// VerifyFrozenIntegrity 校验冻结曲线的内容摘要与冻结元数据是否完整且自洽。
// 冻结修订必须有非空摘要和冻结时间，且按当前分段重算的摘要必须与存储摘要一致；
// 未冻结修订不得残留冻结摘要或冻结时间。任何不一致都说明快照被篡改或损坏。
func (r *FiringCurveRevision) VerifyFrozenIntegrity() error {
	if r.FreezeStatus == CurveFrozen {
		if r.ContentDigest == "" {
			return NewError(ErrState, "冻结曲线缺少内容摘要", "contentDigest")
		}
		if r.FrozenAt == nil {
			return NewError(ErrState, "冻结曲线缺少冻结时间", "frozenAt")
		}
		b, err := r.CanonicalSnapshot()
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		if hex.EncodeToString(h[:]) != r.ContentDigest {
			return NewError(ErrState, "冻结曲线内容摘要与分段内容不匹配", "contentDigest")
		}
		return nil
	}
	if r.ContentDigest != "" || r.FrozenAt != nil {
		return NewError(ErrState, "未冻结曲线不应携带冻结摘要或冻结时间", "freezeStatus")
	}
	return nil
}
