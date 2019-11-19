package cmd

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/rcrowley/go-metrics"
	"github.com/sclasen/go-metrics-cloudwatch/config"
	"github.com/sclasen/go-metrics-cloudwatch/reporter"

	"github.com/SEEK-Jobs/orgbot/pkg/orgbot"
)

// MetricsReporterFunc is the type used to define a function that continually reports
// metrics to CloudWatch.
type MetricsReporterFunc func()

// metricsRegistry provides the metrics.Registry implementation that metrics are reported to.
var metricsRegistry = metrics.NewRegistry()

// noFilter provides an implementation of go-cloudwatch-metrics config.Filter as the
// the config.NoFilter provided by that package logs incessantly.
type noFilter struct{}

// ShouldReport implements config.Filter.
func (n *noFilter) ShouldReport(metric string, value float64) bool {
	return true
}

// Percentiles implements config.Filter.
func (n *noFilter) Percentiles(metric string) []float64 {
	return []float64{0.5, 0.75, 0.95, 0.99, 0.999, 1.0}
}

// metricsReporter returns a configured MetricsReporterFunc func.
func metricsReporter(c *orgbot.Config, sess *session.Session) MetricsReporterFunc {
	metricsConf := &config.Config{
		Client:            cloudwatch.New(sess),
		Namespace:         c.Name,
		Filter:            &noFilter{},
		ReportingInterval: c.MetricsInterval,
		StaticDimensions:  map[string]string{"version": c.Version},
	}

	return func() {
		reporter.Silence = true
		reporter.Cloudwatch(metricsRegistry, metricsConf)
	}
}
