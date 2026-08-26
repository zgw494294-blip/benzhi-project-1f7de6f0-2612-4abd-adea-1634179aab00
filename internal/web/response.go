package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorEnvelope struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Field          string `json:"field,omitempty"`
	CurrentVersion int    `json:"currentVersion,omitempty"`
	Details        any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	var be *domain.BusinessError
	if errors.As(err, &be) {
		status := http.StatusBadRequest
		if be.Code == domain.ErrNotFound {
			status = http.StatusNotFound
		} else if be.Code == domain.ErrConflict {
			status = http.StatusConflict
		} else if be.Code == domain.ErrForbidden {
			status = http.StatusForbidden
		} else if be.Code == domain.ErrState {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, errorEnvelope{apiError{Code: string(be.Code), Message: be.Message, Field: be.Field, CurrentVersion: be.CurrentVersion}})
		return
	}
	var ae *application.AppError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, errorEnvelope{apiError{Code: ae.Code, Message: ae.Message, Field: ae.Field, CurrentVersion: ae.CurrentVersion, Details: ae.Details}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorEnvelope{apiError{Code: "INTERNAL_ERROR", Message: "服务内部错误"}})
}

func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return domain.NewError(domain.ErrInvalid, "JSON 请求体无效："+err.Error(), "body")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return domain.NewError(domain.ErrInvalid, "请求体只能包含一个 JSON 对象", "body")
	}
	return nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
