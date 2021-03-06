package ginprometheus

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var defaultMetricPath = "/metrics"

// Prometheus contains the metrics gathered by the instance and its path
type Prometheus struct {
	reqCnt               *prometheus.CounterVec
	reqDur, reqSz, resSz prometheus.Summary

	//RouteAliases map[string]string
	MetricsPath string
}

// NewPrometheus generates a new set of metrics with a certain subsystem name
func NewPrometheus(subsystem string) *Prometheus {
	p := &Prometheus{
		MetricsPath: defaultMetricPath,
	}

	p.registerMetrics(subsystem)

	return p
}

func (p *Prometheus) registerMetrics(subsystem string) {

	p.reqCnt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "How many HTTP requests processed, partitioned by status code, HTTP method and path.",
		},
		[]string{"code", "method", "path"},
	)
	prometheus.MustRegister(p.reqCnt)

	p.reqDur = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "request_duration_microseconds",
			Help:      "The HTTP request latencies in microseconds.",
		},
	)
	prometheus.MustRegister(p.reqDur)

	p.reqSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "request_size_bytes",
			Help:      "The HTTP request sizes in bytes.",
		},
	)
	prometheus.MustRegister(p.reqSz)

	p.resSz = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Subsystem: subsystem,
			Name:      "response_size_bytes",
			Help:      "The HTTP response sizes in bytes.",
		},
	)
	prometheus.MustRegister(p.resSz)

}

// Use adds the middleware to a gin engine.
func (p *Prometheus) Use(e *gin.Engine) {
	//p.RouteAliases = make(map[string]string)
	//for _, route := range e.Routes() {
	//	p.RouteAliases[route.Handler] = route.Path
	//}

	e.Use(p.HandlerFunc())
	e.GET(p.MetricsPath, PrometheusHandler())
}

// UseWithAuth adds the middleware to a gin engine with BasicAuth.
func (p *Prometheus) UseWithAuth(e *gin.Engine, accounts gin.Accounts) {
	e.Use(p.HandlerFunc())
	e.GET(p.MetricsPath, gin.BasicAuth(accounts), PrometheusHandler())
}

func (p *Prometheus) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := c.Request.URL.Path
		if url == p.MetricsPath {
			c.Next()
			return
		}

		start := time.Now()

		reqSz := make(chan int)
		go computeApproximateRequestSize(c.Request, reqSz)

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		elapsed := float64(time.Since(start)) / float64(time.Microsecond)
		resSz := float64(c.Writer.Size())

		p.reqDur.Observe(elapsed)
		p.reqCnt.WithLabelValues(status, c.Request.Method, url).Inc()
		p.reqSz.Observe(float64(<-reqSz))
		p.resSz.Observe(resSz)
	}
}

func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// From https://github.com/DanielHeckrath/gin-prometheus/blob/master/gin_prometheus.go
func computeApproximateRequestSize(r *http.Request, out chan int) {
	s := 0
	if r.URL != nil {
		s = len(r.URL.String())
	}

	s += len(r.Method)
	s += len(r.Proto)
	for name, values := range r.Header {
		s += len(name)
		for _, value := range values {
			s += len(value)
		}
	}
	s += len(r.Host)

	// N.B. r.Form and r.MultipartForm are assumed to be included in r.URL.

	if r.ContentLength != -1 {
		s += int(r.ContentLength)
	}
	out <- s
}
