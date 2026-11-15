package idempotency

import (
	"bytes"
	"net/http"
)

// recorder оборачивает http.ResponseWriter для записи статуса и тела ответа.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newRecorder() *recorder {
	return &recorder{header: make(http.Header)}
}

// Header возвращает заголовки буферизованного ответа.
func (r *recorder) Header() http.Header {
	return r.header
}

// WriteHeader сохраняет HTTP-статус ответа.
func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
}

// Write буферизует тело ответа.
func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *recorder) flush(w http.ResponseWriter) {
	for key, values := range r.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if r.status == 0 {
		r.status = http.StatusOK
	}
	w.WriteHeader(r.status)
	_, _ = w.Write(r.body.Bytes())
}
