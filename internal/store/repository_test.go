package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kilncurve-release/internal/domain"
)

func TestRepositoryPersistsAndReloadsAuditChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	repo, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC)
	p := &domain.TrialProject{ID: "p", Code: "KC", Owner: "工程师", Status: domain.ProjectDraft, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err = repo.Update(CommitMeta{At: now, Actor: "工程师", Action: "PROJECT_CREATED", ProjectID: p.ID, EntityID: p.ID}, func(s *State) error { s.Projects[p.ID] = p; return nil }); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := reloaded.Inspect()
	if err != nil {
		t.Fatal(err)
	}
	if stats.Projects != 1 || stats.AuditEvents != 1 || stats.AuditHead == "" {
		t.Fatalf("恢复统计异常: %#v", stats)
	}
}

func TestRepositoryRejectsFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("未来 schemaVersion 未被拒绝")
	}
}

func TestUpdateDoesNotPublishFailedMutation(t *testing.T) {
	repo, err := Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := domain.NewError(domain.ErrInvalid, "失败", "field")
	err = repo.Update(CommitMeta{}, func(s *State) error { s.Projects["bad"] = &domain.TrialProject{ID: "bad"}; return sentinel })
	if err == nil {
		t.Fatal("预期事务失败")
	}
	state, _ := repo.Snapshot()
	if len(state.Projects) != 0 {
		t.Fatal("失败事务污染了内存快照")
	}
}
