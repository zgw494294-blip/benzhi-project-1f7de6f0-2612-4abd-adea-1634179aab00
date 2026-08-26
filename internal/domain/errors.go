package domain

import "fmt"

type ErrorCode string

const (
	ErrInvalid   ErrorCode = "INVALID_ARGUMENT"
	ErrConflict  ErrorCode = "VERSION_CONFLICT"
	ErrState     ErrorCode = "INVALID_STATE"
	ErrNotFound  ErrorCode = "NOT_FOUND"
	ErrForbidden ErrorCode = "FORBIDDEN"
)

type BusinessError struct {
	Code           ErrorCode `json:"code"`
	Message        string    `json:"message"`
	Field          string    `json:"field,omitempty"`
	CurrentVersion int       `json:"currentVersion,omitempty"`
}

func (e *BusinessError) Error() string { return e.Message }

func NewError(code ErrorCode, message, field string) error {
	return &BusinessError{Code: code, Message: message, Field: field}
}

func VersionConflict(current int) error {
	return &BusinessError{Code: ErrConflict, Message: fmt.Sprintf("课题版本已更新，当前版本为 %d", current), CurrentVersion: current}
}
