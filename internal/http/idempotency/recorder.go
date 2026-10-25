package idempotency

import (
	"bytes"
	"net/http"
)

// recorder оборачивает http.ResponseWriter для записи статуса и тела ответа.
type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

// WriteHeader записывает HTTP-статус в ответ и сохраняет его в структуре recorder.
func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write записывает тело ответа и сохраняет его в структуре recorder.
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}
