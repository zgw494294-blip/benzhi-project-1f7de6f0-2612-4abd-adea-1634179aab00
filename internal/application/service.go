package application

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"kilncurve-release/internal/domain"
	"kilncurve-release/internal/store"
	"kilncurve-release/internal/verification"
)

type Service struct {
	repo     *store.Repository
	curves   *verification.CurveValidator
	evidence *verification.EvidenceEvaluator
	now      func() time.Time
	id       func(string) string

	comparisonMu    sync.RWMutex
	comparisonCache map[string]domain.CurveComparison
}

func NewService(repo *store.Repository) *Service {
	return &Service{
		repo:            repo,
		curves:          verification.NewCurveValidator(),
		evidence:        verification.NewEvidenceEvaluator(),
		now:             func() time.Time { return time.Now().UTC() },
		id:              newID,
		comparisonCache: make(map[string]domain.CurveComparison),
	}
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
