package middleware

import "net/http"

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// newStatusWriter создает новый экземпляр statusWriter, оборачивая переданный http.ResponseWriter.
func newStatusWriter(w http.ResponseWriter) *statusWriter {
	return &statusWriter{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader записывает заголовок ответа с указанным кодом статуса.
func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write записывает тело ответа и устанавливает код статуса по умолчанию, если заголовок еще не был записан.
func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
