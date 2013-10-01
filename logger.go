package main

import (
	"bufio"
	"github.com/joinmytalk/xlog"
	"net"
	"net/http"
	"time"
)

// LoggingHandler wraps a http.Handler and logs an access log.
type LoggingHandler struct {
	h http.Handler
}

// LogResponseWriter wraps a http.ResponseWriter for logging.
type LogResponseWriter struct {
	http.ResponseWriter
	RespCode int
	Size     int
}

// Header returns the http.Header object of the underlying http.ResponseWriter.
func (w *LogResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// Write wraps the Write function of the underlying http.ResponseWriter and
// logs the amount of written bytes.
func (w *LogResponseWriter) Write(data []byte) (s int, err error) {
	s, err = w.ResponseWriter.Write(data)
	w.Size += s
	return
}

// Hijack wraps the Hijack function of the underlying http.ResponseWriter.
func (w *LogResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		panic("w.ResponseWriter is not a http.Hijacker")
	}
	return hj.Hijack()
}

// WriteHeader wraps the WriteHeader function of the underlying http.ResponseWriter
// and records the response code.
func (w *LogResponseWriter) WriteHeader(r int) {
	w.ResponseWriter.WriteHeader(r)
	w.RespCode = r
}

// Logger creates a LoggingHandler that wraps a http.Handler and returns a new http.Handler.
func Logger(h http.Handler) http.Handler {
	return &LoggingHandler{h: h}
}

// ServeHTTP forwards the HTTP request to the wrapped http.Handler and logs the HTTP request and response.
func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lrw := &LogResponseWriter{ResponseWriter: w}
	t := time.Now()
	h.h.ServeHTTP(lrw, r)
	duration := time.Since(t).String()
	if lrw.RespCode == 0 {
		lrw.RespCode = 200
	}
	xlog.Requestf("%s \"%s %s %s\" %d %d (%s)", r.RemoteAddr, r.Method, r.RequestURI, r.Proto, lrw.RespCode, lrw.Size, duration)
}
