package collector

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/fffonion/tplink-plug-exporter/kasa"
	"github.com/jonboulle/clockwork"
)

var interval = flag.Duration("interval", 1*time.Minute, "sample interval")
var checkpointInterval = flag.Duration("checkpoint-interval", 1*time.Minute, "checkpoint interval")
var checkpointMaxAge = flag.Duration("checkpoint-max-age", 1*time.Hour, "ignore checkpoint samples older than this")
var thresholdWatts = flag.Float64("threshold-watts", 5, "Wattage above which the unit is considered running")

type MonitorState struct {
	MAC             string `json:"mac"`
	Model           string `json:"model"`
	Alias           string `json:"alias"`
	Feature         string `json:"feature"`
	RSSI            int    `json:"rssi"`
	DeviceID        string `json:"device_id"`
	SoftwareVersion string `json:"software_version"`
	HardwareVersion string `json:"hardware_version"`

	Timestamp       time.Time       `json:"timestamp"`
	CycleState      bool            `json:"state"`
	CycleCount      uint            `json:"cycle_count"`
	CycleDurations  []time.Duration `json:"cycle_durations"` // awaiting export
	LastOn          time.Time       `json:"last_on"`
	LastOnDuration  time.Duration   `json:"last_on_duration"`
	LastOff         time.Time       `json:"last_off"`
	LastOffDuration time.Duration   `json:"last_off_duration"`

	// most recent samples
	Power    float64 `json:"power"`
	Voltage  float64 `json:"voltage"`
	Current  float64 `json:"current"`
	TotalKwH float64 `json:"total_kwh"`
}

type Monitor struct {
	Addr           string
	interval       time.Duration
	client         *kasa.KasaClient
	ThresholdWatts float64

	lastSample *kasa.GetRealtimeResponse
	State      MonitorState

	time clockwork.Clock
}

func (m *Monitor) sample(now time.Time, sys *kasa.GetSysInfoResponse, rt *kasa.GetRealtimeResponse) {
	m.State.Timestamp = now
	log.Printf("sampling %s (%q), %fw at %fa@%fv",
		sys.Model, sys.Alias,
		rt.Power, rt.Current, rt.Voltage)
	m.State.MAC, m.State.Model, m.State.Alias, m.State.Feature = sys.MAC, sys.Model, sys.Alias, sys.Feature
	m.State.DeviceID, m.State.SoftwareVersion, m.State.HardwareVersion = sys.DeviceID, sys.SoftwareVersion, sys.HardwareVersion
	m.State.RSSI = sys.RSSI

	m.State.Power, m.State.Voltage, m.State.Current, m.State.TotalKwH = rt.Power, rt.Voltage, rt.Current, rt.Total

	if m.State.Timestamp.IsZero() && m.lastSample == nil {
		m.lastSample = rt
		if m.State.CycleState = rt.Power >= m.ThresholdWatts; m.State.CycleState {
			m.State.LastOn = now
			log.Printf("no prior state, starting at %v as of %s", m.State.CycleState, m.State.LastOn)
		} else {
			m.State.LastOff = now
			log.Printf("no prior state, starting at %v as of %s", m.State.CycleState, m.State.LastOff)
		}
	} else if rt.Power >= m.ThresholdWatts && !m.State.CycleState {
		m.State.CycleState = true
		m.State.LastOn = now
		if !m.State.LastOff.IsZero() {
			m.State.LastOffDuration = m.State.LastOn.Sub(m.State.LastOff)
		}
		log.Printf("low-to-high transition: %fw; was off %s", rt.Power, m.State.LastOffDuration)
	} else if rt.Power < m.ThresholdWatts && m.State.CycleState {
		m.State.CycleState = false
		m.State.LastOff = now
		if !m.State.LastOn.IsZero() {
			m.State.LastOnDuration = m.State.LastOff.Sub(m.State.LastOn)
			m.State.CycleDurations = append(m.State.CycleDurations, m.State.LastOnDuration)
			m.State.CycleCount++
		}
		log.Printf("high-to-low transition: %fw; was on %s", rt.Power, m.State.LastOnDuration)
	}
}

type Collector struct {
	shutdown       chan bool
	checkpointFile string
	time           clockwork.Clock

	Monitors map[string]*Monitor
}

func New(addrs []string, checkpointFile string, clock clockwork.Clock) *Collector {
	c := &Collector{
		checkpointFile: checkpointFile,
		Monitors:       map[string]*Monitor{},
		time:           clock,
	}
	cpStates := map[string]MonitorState{}
	if checkpointFile != "" {
		if s, err := c.loadStateCheckpoint(checkpointFile); err != nil {
			log.Printf("error loading checkpoint, starting fresh")
		} else {
			log.Printf("got checkpoint on %d device(s)", len(s))
			cpStates = s
		}
	}
	for _, a := range addrs {
		c.Monitors[a] = &Monitor{
			Addr:           a,
			interval:       *interval,
			ThresholdWatts: *thresholdWatts,
			client: kasa.New(&kasa.KasaClientConfig{
				Host: a,
			}),
			time: c.time,
		}
		if s, ok := cpStates[a]; ok {
			if c.time.Now().Sub(s.Timestamp) <= *checkpointMaxAge {
				c.Monitors[a].State = s
				log.Printf("restoring checkpointed state of %s", a)
			} else {
				log.Printf("checkpointed state of %s is too old (%s vs limit of %s), ignoring",
					a, c.time.Now().Sub(s.Timestamp), *checkpointMaxAge)
			}
		} else {
			log.Printf("no prior checkpointed state of %s", a)
		}
	}
	return c
}

func (c *Collector) loadStateCheckpoint(fn string) (map[string]MonitorState, error) {
	f, err := os.Open(fn)
	if err != nil {
		log.Printf("error opening checkpoint file %s: %v", fn, err)
		return nil, err
	}
	defer f.Close()
	states := map[string]MonitorState{}
	if err := json.NewDecoder(f).Decode(&states); err != nil {
		log.Printf("error decoding checkpoint file %s: %v", fn, err)
		return nil, err
	}
	return states, nil
}

func (c *Collector) saveStateCheckpoint(fn string) error {
	if c.checkpointFile == "" {
		return nil
	}
	states := map[string]MonitorState{}
	for name, m := range c.Monitors {
		states[name] = m.State
	}
	f, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("error opening checkpoint file %s: %v", fn, err)
		return err
	}
	defer f.Close()
	log.Printf("checkpointing to %s", fn)
	return json.NewEncoder(f).Encode(&states)
}

func (c *Collector) Shutdown() {
	c.shutdown <- true
}

type cpEvent bool

func (c *Collector) Run(shutdown chan bool) {
	cpTicker := time.NewTicker(*checkpointInterval)
	intervalTicker := time.NewTicker(*interval)
	for {
		select {
		case <-shutdown:
			log.Printf("kasadutycycle: shutdown")
			return
		case <-cpTicker.C:
			log.Printf("checkpoint tick")
			if c.checkpointFile != "" {
				c.saveStateCheckpoint(c.checkpointFile)
			}
		case <-intervalTicker.C:
			for _, m := range c.Monitors {
				sys := m.client.SystemService(nil)
				if sysinfo, err := sys.GetSysInfo(); err != nil {
					log.Println("error collecting", m.Addr, ":", err)
				} else {
					// log.Printf("sysinfo %v", sysinfo)
					emeter := m.client.EmeterService(nil)
					if rt, err := emeter.GetRealtime(); err != nil {
						log.Println("error collecting", m.Addr, ":", err)
					} else {
						m.sample(c.time.Now(), sysinfo, rt)
					}
				}
			}
		}
	}
}
