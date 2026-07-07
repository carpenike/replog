package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

// Minimal, dependency-free metrics. RequestLogger feeds recordRequest on every
// request; MetricsHandler renders a small Prometheus text exposition of the
// request counters plus a few Go runtime gauges. This deliberately avoids
// pulling in prometheus/client_golang — the counter set is tiny and hand-rolled
// so /metrics stays a stdlib-only endpoint.

var (
	reqTotal atomic.Int64
	req2xx   atomic.Int64
	req3xx   atomic.Int64
	req4xx   atomic.Int64
	req5xx   atomic.Int64

	processStart = time.Now()
)

// recordRequest tallies one completed request by status class.
func recordRequest(status int) {
	reqTotal.Add(1)
	switch {
	case status >= 500:
		req5xx.Add(1)
	case status >= 400:
		req4xx.Add(1)
	case status >= 300:
		req3xx.Add(1)
	default:
		req2xx.Add(1)
	}
}

// MetricsHandler serves the counters + runtime gauges in Prometheus text
// exposition format (v0.0.4). It is dependency-light and safe to scrape
// frequently. Expose it on an internal/gated route — it reveals request volume
// and process internals.
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP replog_http_requests_total Total HTTP requests handled, by status class.\n")
		fmt.Fprintf(w, "# TYPE replog_http_requests_total counter\n")
		fmt.Fprintf(w, "replog_http_requests_total %d\n", reqTotal.Load())
		fmt.Fprintf(w, "replog_http_requests_by_class_total{class=\"2xx\"} %d\n", req2xx.Load())
		fmt.Fprintf(w, "replog_http_requests_by_class_total{class=\"3xx\"} %d\n", req3xx.Load())
		fmt.Fprintf(w, "replog_http_requests_by_class_total{class=\"4xx\"} %d\n", req4xx.Load())
		fmt.Fprintf(w, "replog_http_requests_by_class_total{class=\"5xx\"} %d\n", req5xx.Load())

		fmt.Fprintf(w, "# HELP replog_process_uptime_seconds Seconds since process start.\n")
		fmt.Fprintf(w, "# TYPE replog_process_uptime_seconds gauge\n")
		fmt.Fprintf(w, "replog_process_uptime_seconds %.0f\n", time.Since(processStart).Seconds())

		fmt.Fprintf(w, "# HELP replog_go_goroutines Number of goroutines.\n")
		fmt.Fprintf(w, "# TYPE replog_go_goroutines gauge\n")
		fmt.Fprintf(w, "replog_go_goroutines %d\n", runtime.NumGoroutine())

		fmt.Fprintf(w, "# HELP replog_go_memstats_alloc_bytes Bytes of allocated heap objects.\n")
		fmt.Fprintf(w, "# TYPE replog_go_memstats_alloc_bytes gauge\n")
		fmt.Fprintf(w, "replog_go_memstats_alloc_bytes %d\n", mem.Alloc)

		fmt.Fprintf(w, "# HELP replog_go_memstats_sys_bytes Bytes of memory obtained from the OS.\n")
		fmt.Fprintf(w, "# TYPE replog_go_memstats_sys_bytes gauge\n")
		fmt.Fprintf(w, "replog_go_memstats_sys_bytes %d\n", mem.Sys)
	}
}
