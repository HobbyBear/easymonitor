package infra

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/http/httputil"

	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Status() int {
	return rw.statusCode
}

type Middleware func(http.Handler) http.Handler

func MetricMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w}
		// 在处理请求之前执行的逻辑
		// 可以在这里进行请求验证、日志记录等操作
		reqBody, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.WithError(err).Errorf("dump request fail")
		}
		now := time.Now()
		// 调用下一个处理程序
		MetricMonitor.RecordServerCount(TypeHTTP, r.Method, r.URL.Path)
		next.ServeHTTP(rw, r)
		MetricMonitor.RecordServerHandlerSeconds(TypeHTTP, r.Method, rw.Status(), r.URL.Path, time.Now().Sub(now).Seconds())
		if rw.Status() != http.StatusOK {
			log.WithFields(log.Fields{
				"request": string(reqBody),
				"code":    rw.statusCode,
			}).Warn("request fail ")
		}
	})
}
