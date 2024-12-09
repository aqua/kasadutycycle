package collector

import (
	"testing"
	"time"

	"github.com/fffonion/tplink-plug-exporter/kasa"
	"github.com/jonboulle/clockwork"
)

func mustParseTime(fmt, s string) time.Time {
	t, err := time.Parse(fmt, s)
	if err != nil {
		panic("error parsing test time")
	}
	return t
}

func TestSetupNoCheckpoint(t *testing.T) {
	c := New([]string{"127.0.0.1"}, "", clockwork.NewFakeClock())
	if c == nil {
		t.Errorf("error making collector")
		return
	}
	if len(c.Monitors) != 1 {
		t.Errorf("want 1 monitor, got %d", len(c.Monitors))
	}
	if !c.Monitors["127.0.0.1"].State.Timestamp.IsZero() {
		t.Errorf("want 1 monitor with no state, got a timestamp; state=%v",
			c.Monitors["127.0.0.1"].State)
	}
}

func TestSetupCurrentCheckpoint(t *testing.T) {
	now := mustParseTime(time.RFC3339, "2024-03-12T21:25:30.000000-00:00")
	c := New([]string{"127.0.0.1"},
		"testdata/current-checkpoint.json",
		clockwork.NewFakeClockAt(now))
	if c == nil {
		t.Errorf("error making collector")
		return
	}
	if len(c.Monitors) != 1 {
		t.Errorf("want 1 monitor, got %d", len(c.Monitors))
	}
	if c.Monitors["127.0.0.1"].State.Timestamp.IsZero() {
		t.Errorf("want 1 monitor with no state, got a timestamp, state=%v",
			c.Monitors["127.0.0.1"].State)
	}
}

func TestSetupOldCheckpoint(t *testing.T) {
	now := mustParseTime(time.RFC3339, "2024-03-12T21:25:30.000000-00:00")
	c := New([]string{"127.0.0.1"},
		"testdata/old-checkpoint.json",
		clockwork.NewFakeClockAt(now))
	if c == nil {
		t.Errorf("error making collector")
		return
	}
	if len(c.Monitors) != 1 {
		t.Errorf("want 1 monitor, got %d", len(c.Monitors))
	}
	if !c.Monitors["127.0.0.1"].State.Timestamp.IsZero() {
		t.Errorf("want 1 monitor with no state, got a timestamp")
	}
}

var (
	sysinfoResponse = &kasa.GetSysInfoResponse{
		MAC:             "aa:bb:cc:dd:ee",
		Model:           "model",
		Alias:           "alias",
		Feature:         "feature",
		RSSI:            42,
		LEDOff:          1,
		OnTime:          1234,
		DeviceID:        "FFFF",
		SoftwareVersion: "1.2.3",
		HardwareVersion: "2.3.4",
	}
	rtOff = &kasa.GetRealtimeResponse{
		// some fw/hw may use the following keys
		Current: 0.05,   // amps
		Voltage: 115,    // volts
		Power:   0.05,   // watts
		Total:   1234.5, // kWh
	}
	rtOn = &kasa.GetRealtimeResponse{
		// some fw/hw may use the following keys
		Current: 0.45,   // amps
		Voltage: 115,    // volts
		Power:   90,     // watts
		Total:   1234.5, // kWh
	}
)

func TestSampleIterations(t *testing.T) {
	cases := []struct {
		desc              string
		checkpoint        string
		checkpointedStart time.Time
		values            []*kasa.GetRealtimeResponse
		repeats           []int
		expectedStates    []bool
	}{
		{
			"start in off state, no checkpoint",
			"",
			time.Time{},
			[]*kasa.GetRealtimeResponse{rtOff, rtOn, rtOff, rtOn},
			[]int{10, 10, 10, 10},
			[]bool{false, true, false, true},
		},
		{
			"start in on state, no checkpoint",
			"",
			time.Time{},
			[]*kasa.GetRealtimeResponse{rtOn, rtOff, rtOn, rtOff},
			[]int{10, 10, 10, 10},
			[]bool{true, false, true, false},
		},
		/*
			{
				"start in on state, current-on checkpoint",
				"testdata/current-checkpoint-on.json",
				mustParseTime(time.RFC3339, "2024-03-12T19:50:00.000000-00:00"),
				[]*kasa.GetRealtimeResponse{rtOn, rtOff, rtOn, rtOff},
				[]int{10, 10, 10, 10},
				[]bool{true, false, true, false},
			},
		*/
		/*
			{
				"start in off state, current-off checkpoint",
				"testdata/current-checkpoint-off.json",
				mustParseTime(time.RFC3339, "2024-03-12T19:50:00.000000-00:00"),
				[]*kasa.GetRealtimeResponse{rtOff, rtOn, rtOff, rtOn},
				[]int{10, 10, 10, 10},
				[]bool{false, true, false, true},
			},
		*/
		/*
			{
				"start in off state, current-on checkpoint",
				"testdata/current-checkpoint-on.json",
				mustParseTime(time.RFC3339, "2024-03-12T19:50:00.000000-00:00"),
				[]*kasa.GetRealtimeResponse{rtOff, rtOn, rtOff, rtOn},
				[]int{10, 10, 10, 10},
				[]bool{false, true, false, true},
			},
		*/
		/*
			{
				"start in on state, current-off checkpoint",
				"testdata/current-checkpoint-off.json",
				mustParseTime(time.RFC3339, "2024-03-12T19:50:00.000000-00:00"),
				[]*kasa.GetRealtimeResponse{rtOn, rtOff, rtOn, rtOff},
				[]int{10, 10, 10, 10},
				[]bool{true, false, true, false},
			},
		*/
	}
	for _, td := range cases {
		now := mustParseTime(time.RFC3339, "2020-03-12T20:00:00.000000-00:00")
		c := New([]string{"127.0.0.1"}, td.checkpoint, clockwork.NewFakeClockAt(now))
		if c == nil {
			t.Errorf("%s: error making collector", td.desc)
			return
		}
		mon := c.Monitors["127.0.0.1"]
		for i, rt := range td.values {
			t.Logf("%s: at step %d start, mon.State=%v", td.desc, i, mon.State)
			stateStart := c.time.Now()
			for j := 0; j < td.repeats[i]; j++ {
				mon.sample(c.time.Now(), sysinfoResponse, rt)
				if mon.State.CycleState != td.expectedStates[i] {
					t.Errorf("%s: at step %d repeat %d, time %s, want state %v, got %v",
						td.desc, i, j, c.time.Now(),
						td.expectedStates[i], mon.State.CycleState)
				}
				if i == 0 && !td.checkpointedStart.IsZero() {
					if td.expectedStates[i] && mon.State.LastOn != td.checkpointedStart {
						t.Errorf("%s: at step %d repeat %d, time %s, want LastOn=%s, got %s",
							td.desc, i, j, c.time.Now(), td.checkpointedStart, mon.State.LastOn)
					}
					if !td.expectedStates[i] && mon.State.LastOff != td.checkpointedStart {
						t.Errorf("%s: at step %d repeat %d, time %s, want LastOff=%s, got %s",
							td.desc, i, j, c.time.Now(), td.checkpointedStart, mon.State.LastOff)
					}
				} else if i > 0 {
					if td.expectedStates[i] && mon.State.LastOn != stateStart {
						t.Errorf("%s: at step %d repeat %d, time %s, want LastOn=%s, got %s",
							td.desc, i, j, c.time.Now(), stateStart, mon.State.LastOn)
					}
					if !td.expectedStates[i] && mon.State.LastOff != stateStart {
						t.Errorf("%s: at step %d repeat %d, time %s, want LastOff=%s, got %s",
							td.desc, i, j, c.time.Now(), stateStart, mon.State.LastOff)
					}
				}
				c.time.(clockwork.FakeClock).Advance(*interval)
			}
		}
	}
}
