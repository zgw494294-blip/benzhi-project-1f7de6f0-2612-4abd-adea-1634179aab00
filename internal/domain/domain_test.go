package domain

import (
	"testing"
	"time"
)

func TestCurveFreezeIsDeterministicAndImmutable(t *testing.T) {
	now := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	segments := []CurveSegment{{Order: 1, Kind: SegmentHeat, StartTemperature: 20, EndTemperature: 1200, DurationMinutes: 120}, {Order: 2, Kind: SegmentHold, StartTemperature: 1200, EndTemperature: 1200, DurationMinutes: 30}}
	a := NewRevision("r1", "p1", 1, "", "工程师", segments, now)
	b := NewRevision("r2", "p1", 1, "", "工程师", segments, now)
	if err := a.Freeze(now); err != nil {
		t.Fatal(err)
	}
	if err := b.Freeze(now); err != nil {
		t.Fatal(err)
	}
	if a.ContentDigest != b.ContentDigest {
		t.Fatalf("相同规范化内容的摘要不同: %s != %s", a.ContentDigest, b.ContentDigest)
	}
	if err := a.ReplaceSegments(nil); err == nil {
		t.Fatal("冻结后仍能原位修改")
	}
}

func TestProcessCardDetectsTampering(t *testing.T) {
	now := time.Date(2026, 8, 26, 1, 2, 3, 0, time.UTC)
	p := &TrialProject{ID: "p1", Code: "KC-1", Owner: "工程师", BodyMaterial: "坯", GlazeMaterial: "釉", LoadingMethod: "平码", Status: ProjectReview, Version: 4, CreatedAt: now, UpdatedAt: now}
	r := NewRevision("r1", p.ID, 1, "", "工程师", []CurveSegment{{Order: 1, Kind: SegmentHeat, StartTemperature: 20, EndTemperature: 1000, DurationMinutes: 100}}, now)
	if err := r.Freeze(now); err != nil {
		t.Fatal(err)
	}
	card, err := IssueCard("c1", "CARD-1", p, r, []string{"trial-run:run1"}, "复核员", now)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := card.Verify()
	if err != nil || !ok {
		t.Fatalf("新工艺卡核验失败: %v", err)
	}
	card.Reviewer = "被篡改"
	ok, err = card.Verify()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("篡改后的工艺卡仍通过核验")
	}
}

func TestOptimisticVersionConflict(t *testing.T) {
	p := &TrialProject{Version: 7}
	err := p.RequireVersion(6)
	if err == nil {
		t.Fatal("过期版本未被拒绝")
	}
	be, ok := err.(*BusinessError)
	if !ok || be.Code != ErrConflict || be.CurrentVersion != 7 {
		t.Fatalf("冲突信息不完整: %#v", err)
	}
}
