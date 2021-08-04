package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	defaultMetricNamespace = "flux"
	defaultMetricSubsystem = "http"
	defaultMetricBuckets   = []float64{
		0.0005,
		0.001, // 1ms
		0.002,
		0.005,
		0.01, // 10ms
		0.02,
		0.05,
		0.1, // 100 ms
		0.2,
		0.5,
		1.0, // 1s
		2.0,
		5.0,
		10.0, // 10s
		15.0,
		20.0,
		30.0,
	}
)

type Metrics struct {
	// 各组件请求耗时次数统计
	routeDuration *prometheus.HistogramVec
}

func (m *Metrics) NewRouteVec(kind, typeId string) prometheus.Observer {
	return m.routeDuration.WithLabelValues(kind, typeId)
}

func (m *Metrics) NewRouteVecTimer(kind, typeId string) *prometheus.Timer {
	return prometheus.NewTimer(m.NewRouteVec(kind, typeId))
}

// NewMetricsWith 创建绑定ListenerId的统计指标。
// 注意：此统计指标由WebListener初始化时创建和绑定
func NewMetricsWith(listener string) *Metrics {
	// rer: https://prometheus.io/docs/concepts/data_model/
	// must match the regex [a-zA-Z_:][a-zA-Z0-9_:]*.
	const namespace = "fluxgo"
	return &Metrics{
		routeDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: listener,
			Name:      "duration",
			Help:      "Spend time by processing a endpoint",
			Buckets:   defaultMetricBuckets,
		}, []string{"ComponentKind", "TypeId"}),
	}
}
