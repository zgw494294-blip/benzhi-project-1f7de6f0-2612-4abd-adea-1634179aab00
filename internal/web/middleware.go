package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request-id"

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), requestIDKey, id), 25*time.Second)
		defer cancel()
		started := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("请求 %s 发生 panic: %v\n%s", id, recovered, debug.Stack())
				if sw.status == 0 {
					writeJSON(sw, http.StatusInternalServerError, errorEnvelope{apiError{Code: "INTERNAL_ERROR", Message: "服务内部错误"}})
				}
			}
			log.Printf("请求 %s %s %s 状态=%d 字节=%d 耗时=%s", id, r.Method, r.URL.Path, sw.status, sw.bytes, time.Since(started).Round(time.Millisecond))
		}()
		next.ServeHTTP(sw, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(value[:])
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
