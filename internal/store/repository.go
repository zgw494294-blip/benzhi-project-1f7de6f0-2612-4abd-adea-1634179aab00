package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

type Repository struct {
	mu    sync.RWMutex
	path  string
	state *State
}

func Open(path string) (*Repository, error) {
	if path == "" {
		return nil, errors.New("持久化路径不能为空")
	}
	r := &Repository{path: path, state: NewState()}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取快照: %w", err)
	}
	var s State
	if err = json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("解析快照: %w", err)
	}
	if err = ValidateState(&s); err != nil {
		return nil, fmt.Errorf("校验快照: %w", err)
	}
	r.state = &s
	return r, nil
}

func (r *Repository) Read(fn func(*State) error) error {
	return r.ReadCtx(context.Background(), fn)
}

// ReadCtx 与 Read 相同，但在取得读锁前检查请求上下文：若调用方已取消，
// 立即返回而不读取状态，避免对已放弃的请求继续执行只读工作。
func (r *Repository) ReadCtx(ctx context.Context, fn func(*State) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone, err := r.state.Clone()
	if err != nil {
		return err
	}
	return fn(clone)
}

type CommitMeta struct {
	At                                 time.Time
	Actor, Action, ProjectID, EntityID string
}

func (r *Repository) Update(meta CommitMeta, fn func(*State) error) error {
	return r.UpdateCtx(context.Background(), meta, fn)
}

// UpdateCtx 与 Update 相同，但尊重请求上下文的取消：
//   - 在等待写锁期间或取得写锁后被取消时，立即停止，不应用任何变更、
//     不追加审计事件、不写盘、不发布新的内存状态。
//   - 在持久化前再次检查取消，确保“正式提交前”的取消不会落盘。
//
// 这样客户端已放弃的请求不会产生课题、审计记录或快照副作用，同时不影响
// 未取消请求、正常并发串行化和现有超时机制。
func (r *Repository) UpdateCtx(ctx context.Context, meta CommitMeta, fn func(*State) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	next, err := r.state.Clone()
	if err != nil {
		return err
	}
	if err = fn(next); err != nil {
		return err
	}
	// 幂等重放只读取首次结果，不产生新的业务状态、审计事件或磁盘快照。
	if reflect.DeepEqual(next, r.state) {
		return nil
	}
	if meta.Action != "" {
		if err = appendAudit(next, AuditEvent{At: meta.At, Actor: meta.Actor, Action: meta.Action, ProjectID: meta.ProjectID, EntityID: meta.EntityID}); err != nil {
			return err
		}
	}
	if err = ValidateState(next); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = r.persist(next); err != nil {
		return err
	}
	r.state = next
	return nil
}

func (r *Repository) persist(s *State) error {
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("创建持久化目录: %w", err)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".kilncurve-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	cleanup := func() { _ = f.Close(); _ = os.Remove(name) }
	if _, err = f.Write(b); err != nil {
		cleanup()
		return err
	}
	if err = f.Sync(); err != nil {
		cleanup()
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err = os.Rename(name, r.path); err != nil {
		_ = os.Remove(name)
		return err
	}
	d, err := os.Open(dir)
	if err == nil {
		err = d.Sync()
		_ = d.Close()
	}
	return err
}

func (r *Repository) Snapshot() (*State, error) {
	var out *State
	err := r.Read(func(s *State) error { out = s; return nil })
	return out, err
}
