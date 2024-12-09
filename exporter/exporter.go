package exporter

import (
	"log"
	"net/http"
	"time"

	"github.com/aqua/kasadutycycle/collector"
	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	collector *collector.Collector
	registry  *prometheus.Registry

	onlineMetric,
	dutyThresholdMetric,
	voltageMetric,
	currentMetric,
	powerMetric,
	totalPowerMetric,
	currentStateMetric,
	currentOnDurationMetric,
	currentOffDurationMetric,
	lastOnDurationMetric,
	lastOffDurationMetric,
	cycleCountMetric *prometheus.Desc

	cycleDurationMetric prometheus.Histogram

	/*
		dutyThresholdMetric *prometheus.GaugeVec
		voltageMetric       *prometheus.GaugeVec
		currentMetric       *prometheus.GaugeVec
		powerMetric         *prometheus.GaugeVec
		totalPowerMetric    *prometheus.CounterVec

		currentStateMetric       *prometheus.GaugeVec
		currentOnDurationMetric  *prometheus.GaugeVec
		currentOffDurationMetric *prometheus.GaugeVec
		lastOnDurationMetric     *prometheus.GaugeVec
		lastOffDurationMetric    *prometheus.GaugeVec
	*/
}

func (c *Exporter) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, nil)
}

func New(c *collector.Collector) *Exporter {
	e := &Exporter{
		collector: c,
		registry:  prometheus.NewPedanticRegistry(),
		// registry: prometheus.NewRegistry(),
		onlineMetric: prometheus.NewDesc(
			"online",
			"If plugs are online (constant 1 once sampled)",
			nil,
			nil),
		dutyThresholdMetric: prometheus.NewDesc(
			"duty_threshold",
			"Threshold power (in watts) above which the circuit is considered ON.",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		voltageMetric: prometheus.NewDesc(
			"voltage",
			"Instantaneous voltage on the circuit.",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		currentMetric: prometheus.NewDesc(
			"current",
			"Instantaneous current (amps) on the circuit.",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		powerMetric: prometheus.NewDesc(
			"power_watts",
			"Instantaneous power (watts) on the circuit.",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		totalPowerMetric: prometheus.NewDesc(
			"total_power_kwh",
			"Total power (in KwH) delivered on the circuit.",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		currentStateMetric: prometheus.NewDesc(
			"current_duty_state",
			"Current state of the duty cycle (1 for on, 0 for off).",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		currentOnDurationMetric: prometheus.NewDesc(
			"current_on_duration",
			"Duration of the circuit's current ON duty state (0 if not on)",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		currentOffDurationMetric: prometheus.NewDesc(
			"current_off_duration",
			"Duration (in seconcs) of the circuit's current OFF duty state (0 if not on)",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		lastOnDurationMetric: prometheus.NewDesc(
			"last_on_duration",
			"Duration (in seconds) of the circuit's most recent full ON duty state (0 if no ON state observed)",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		lastOffDurationMetric: prometheus.NewDesc(
			"last_off_duration",
			"Duration of the circuit's most recent full OFF duty state (0 if no OFF state observed)",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		cycleCountMetric: prometheus.NewDesc(
			"cycle_count",
			"Full duty cycles observed",
			[]string{"addr", "mac", "model", "alias", "device_id"},
			nil),
		cycleDurationMetric: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "cycle_durations",
			Help:    "Duration (in seconds) of observed duty cycles",
			Buckets: prometheus.LinearBuckets(0, 60, 60),
		}),
	}
	e.registry.MustRegister(e.cycleDurationMetric)
	return e
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.onlineMetric
	ch <- e.dutyThresholdMetric
	ch <- e.voltageMetric
	ch <- e.currentMetric
	ch <- e.powerMetric
	ch <- e.totalPowerMetric
	ch <- e.currentStateMetric
	ch <- e.currentOnDurationMetric
	ch <- e.currentOffDurationMetric
	ch <- e.lastOnDurationMetric
	ch <- e.lastOffDurationMetric
	ch <- e.cycleCountMetric
	/*
		e.registry.MustRegister(
			e.dutyThresholdMetric,
			e.voltageMetric,
			e.currentMetric,
			e.powerMetric,
			e.totalPowerMetric,
			e.currentStateMetric,
			e.currentOnDurationMetric,
			e.currentOffDurationMetric,
			e.lastOnDurationMetric,
			e.lastOffDurationMetric)
	*/
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	now := time.Now()

	onlinePlugs := 0

	for ID, m := range e.collector.Monitors {
		if m.State.Timestamp.IsZero() {
			continue
		}
		onlinePlugs++
		ch <- prometheus.MustNewConstMetric(
			e.dutyThresholdMetric, prometheus.GaugeValue,
			float64(m.ThresholdWatts),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.voltageMetric, prometheus.GaugeValue,
			float64(m.State.Voltage),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.currentMetric, prometheus.GaugeValue,
			float64(m.State.Current),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.powerMetric, prometheus.GaugeValue,
			float64(m.State.Power),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.totalPowerMetric, prometheus.GaugeValue,
			float64(m.State.TotalKwH),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		var currentState, onDuration, offDuration float64
		if m.State.CycleState {
			currentState, onDuration, offDuration = 1, float64(now.Sub(m.State.LastOn).Seconds()), 0
			log.Printf("onDuration = %v", now.Sub(m.State.LastOn).Seconds())
		} else {
			currentState, onDuration, offDuration = 0, 0, float64(now.Sub(m.State.LastOff).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(
			e.currentStateMetric, prometheus.GaugeValue,
			currentState,
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.currentOnDurationMetric, prometheus.GaugeValue,
			onDuration,
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.currentOffDurationMetric, prometheus.GaugeValue,
			offDuration,
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.lastOnDurationMetric, prometheus.GaugeValue,
			float64(m.State.LastOnDuration.Seconds()),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.lastOffDurationMetric, prometheus.GaugeValue,
			float64(m.State.LastOffDuration.Seconds()),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		ch <- prometheus.MustNewConstMetric(
			e.cycleCountMetric, prometheus.CounterValue,
			float64(m.State.CycleCount),
			ID, m.State.MAC, m.State.Model, m.State.Alias, m.State.DeviceID)
		for _, v := range m.State.CycleDurations {
			log.Printf("flushing histogram observation of cycle lasting %s", v)
			e.cycleDurationMetric.Observe(float64(v.Seconds()))
		}
		m.State.CycleDurations = nil
	}
	ch <- prometheus.MustNewConstMetric(
		e.onlineMetric, prometheus.GaugeValue,
		float64(onlinePlugs))
}
