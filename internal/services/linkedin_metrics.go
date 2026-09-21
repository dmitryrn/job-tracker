package services

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	linkedInJobsAdded   = "added"
	linkedInJobsErrors  = "errors"
	linkedInJobsInvalid = "invalid"
	linkedInJobsSkipped = "skipped"
)

type LinkedInMetrics struct {
	registry        *prometheus.Registry
	jobs            *prometheus.GaugeVec
	lastRunStarted  prometheus.Gauge
	lastRunFinished prometheus.Gauge
	lastSuccess     prometheus.Gauge
}

func NewLinkedInMetrics() *LinkedInMetrics {
	metrics := &LinkedInMetrics{
		registry: prometheus.NewRegistry(),
		jobs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "linkedin_sync_jobs",
			Help: "Job outcomes from the most recently completed LinkedIn sync.",
		}, []string{"outcome"}),
		lastRunStarted: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "linkedin_sync_last_run_started_timestamp_seconds",
			Help: "Unix timestamp when the most recent LinkedIn sync started.",
		}),
		lastRunFinished: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "linkedin_sync_last_run_finished_timestamp_seconds",
			Help: "Unix timestamp when the most recent LinkedIn sync finished.",
		}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "linkedin_sync_last_success_timestamp_seconds",
			Help: "Unix timestamp when the most recent LinkedIn sync completed without an error.",
		}),
	}
	metrics.registry.MustRegister(metrics.jobs, metrics.lastRunStarted, metrics.lastRunFinished, metrics.lastSuccess)
	return metrics
}

func (metrics *LinkedInMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{})
}

func (metrics *LinkedInMetrics) Start(at time.Time) {
	metrics.lastRunStarted.Set(float64(at.Unix()))
	for _, outcome := range []string{linkedInJobsAdded, linkedInJobsErrors, linkedInJobsInvalid, linkedInJobsSkipped} {
		metrics.jobs.WithLabelValues(outcome).Set(0)
	}
}

func (metrics *LinkedInMetrics) Complete(at time.Time, fetch LinkedInFetchResult, succeeded bool) {
	metrics.jobs.WithLabelValues(linkedInJobsAdded).Set(float64(fetch.SavedJobs))
	metrics.jobs.WithLabelValues(linkedInJobsSkipped).Set(float64(fetch.SkippedJobs))
	metrics.jobs.WithLabelValues(linkedInJobsInvalid).Set(float64(fetch.InvalidJobs))
	if succeeded {
		metrics.jobs.WithLabelValues(linkedInJobsErrors).Set(0)
		metrics.lastSuccess.Set(float64(at.Unix()))
	} else {
		metrics.jobs.WithLabelValues(linkedInJobsErrors).Set(1)
	}

	metrics.lastRunFinished.Set(float64(at.Unix()))
}
