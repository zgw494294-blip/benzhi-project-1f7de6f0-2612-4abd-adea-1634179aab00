package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type Service struct {
	repo     *store.Repository
	curves   *verification.CurveValidator
	evidence *verification.EvidenceEvaluator
	now      func() time.Time
	id       func(string) string
}

func NewService(repo *store.Repository) *Service {
	return &Service{repo: repo, curves: verification.NewCurveValidator(), evidence: verification.NewEvidenceEvaluator(), now: func() time.Time { return time.Now().UTC() }, id: newID}
}

func newID(prefix string) string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := time.Now().UnixNano()
		return prefix + "-" + time.Unix(0, n).UTC().Format("20060102150405.000000000")
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

func requireKey(key string) error {
	if key == "" {
		return appError("IDEMPOTENCY_REQUIRED", "idempotencyKey 不能为空", 400)
	}
	return nil
}

type AppError struct {
	Code, Message  string
	Status         int
	CurrentVersion int
	Field          string
	Details        any
}

func (e *AppError) Error() string { return e.Message }
func appError(code, message string, status int) error {
	return &AppError{Code: code, Message: message, Status: status}
}

func appErrorWithDetails(code, message, field string, status int, details any) error {
	return &AppError{Code: code, Message: message, Field: field, Status: status, Details: details}
}

func idemKey(command, key string) string { return command + ":" + key }

// contextFrom 从可变参数中取出请求上下文。
// Service 的命令方法以 ctxs ...context.Context 形式接收上下文，
// 既能兼容现有不带 context 的调用方（测试、内部复用），又能让 HTTP
// 处理器把请求作用域传入，以便在等待写锁或正式提交前响应取消。
func contextFrom(ctxs []context.Context) context.Context {
	if len(ctxs) > 0 && ctxs[0] != nil {
		return ctxs[0]
	}
	return context.Background()
}
