package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ramio-net/mrps-contracts/health"
)

// A field broadcast has no Cloud session, and an Edge between broadcasts has no
// cameras. Both were rejected by the intake, and both are the normal case rather
// than a degraded one — so the wire has to be able to say them at all.
func TestTelemetrySaysFieldBroadcastAndIdle(t *testing.T) {
	idle := TelemetryUploadRequest{
		InstallationID: "install-1",
		EdgeID:         "edge-1",
		StreamID:       "run-1",
		Seq:            42,
		Snapshots:      []health.DeviceHealthSnapshot{},
		Runtime: &EdgeRuntime{
			ObservedAt:   time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC),
			UptimeSec:    3600,
			SessionState: SessionStateIdle,
			Cameras:      []RuntimeCamera{},
		},
	}
	raw, err := json.Marshal(idle)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if strings.Contains(got, `"session_id"`) {
		t.Fatalf("a broadcast without a Cloud session must omit session_id entirely, "+
			"not send an empty one the receiver will try to parse as a UUID: %s", got)
	}
	if !strings.Contains(got, `"snapshots":[]`) {
		t.Fatalf("an empty batch must survive as an empty list: it is what separates "+
			"'Edge is here, no cameras' from silence: %s", got)
	}
	if !strings.Contains(got, `"session_state":"idle"`) {
		t.Fatalf("runtime state missing from the wire: %s", got)
	}
}

// The backlog problem: snapshots may be minutes old after a venue comes back
// online, while runtime is always about now. If they travelled as one blob a
// replayed backlog would read as the current picture.
func TestRuntimeIsSeparateFromSnapshots(t *testing.T) {
	raw, err := json.Marshal(TelemetryUploadRequest{
		InstallationID: "install-1",
		EdgeID:         "edge-1",
		StreamID:       "run-1",
		Seq:            7,
		Snapshots: []health.DeviceHealthSnapshot{
			{DeviceID: "cam-1", Ts: time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC)},
		},
		Runtime: &EdgeRuntime{
			ObservedAt:   time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC),
			SessionState: SessionStateActive,
			Cameras:      []RuntimeCamera{{DeviceID: "cam-1", SlotIndex: 0, Status: "streaming"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{`"stream_id":"run-1"`, `"runtime":`, `"observed_at":"2026-09-07T12:00:00Z"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s on the wire: %s", want, got)
		}
	}
}

// The indicator this exists for compares assigned against negotiated on the SAME
// connection. Losing either name, or the connection they belong to, turns a
// checkable fact back into a guess.
func TestSRTSectionCarriesBothLatencies(t *testing.T) {
	assigned, negotiated := 120, 280
	raw, err := json.Marshal(health.DeviceHealthSnapshot{
		DeviceID: "cam-1",
		SRT: &health.SRTSection{
			ConnectionID:        "conn-9",
			AssignedLatencyMs:   &assigned,
			NegotiatedLatencyMs: &negotiated,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		`"connection_id":"conn-9"`,
		`"assigned_latency_ms":120`,
		`"negotiated_latency_ms":280`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s: %s", want, got)
		}
	}
}

// An unmeasured value must be absent, not zero. Zero is a legitimate reading for
// margin and for loss, so a consumer that cannot tell them apart will draw a
// healthy camera as starving, or the reverse.
func TestUnmeasuredLatencyIsAbsentNotZero(t *testing.T) {
	raw, err := json.Marshal(health.DeviceHealthSnapshot{
		DeviceID: "cam-1",
		Latency:  &health.LatencySection{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"margin_ms"`) {
		t.Fatalf("an unmeasured margin must not appear as 0: %s", raw)
	}

	zero := 0.0
	raw, err = json.Marshal(health.DeviceHealthSnapshot{
		DeviceID: "cam-1",
		Latency:  &health.LatencySection{MarginMs: &zero},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"margin_ms":0`) {
		t.Fatalf("a measured zero margin must survive: %s", raw)
	}
}

// "This session is running on the full set, the next one will be free" is a
// correct state after a trial ends mid-broadcast, not a contradiction. Next is
// absent while Edge cannot know it, and absent must stay distinguishable from a
// copy of current.
func TestCapabilityStateCarriesCurrentWithoutNext(t *testing.T) {
	raw, err := json.Marshal(EdgeRuntime{
		SessionState: SessionStateActive,
		Capability: &CapabilityState{
			Current: &CapabilitySet{Preset: "full", ProfileID: "p-1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, `"current":`) {
		t.Fatalf("current set missing: %s", got)
	}
	if strings.Contains(got, `"next":`) {
		t.Fatalf("unknown next set must be absent, not guessed: %s", got)
	}
}

// Old Edge, new Cloud. Every addition is optional, so a sender that knows none of
// them still produces a request the new receiver accepts.
func TestLegacySenderStillProducesAValidRequest(t *testing.T) {
	raw, err := json.Marshal(TelemetryUploadRequest{
		InstallationID: "install-1",
		EdgeID:         "edge-1",
		SessionID:      "5f2b1c7e-0000-4000-8000-000000000001",
		Seq:            1,
		Snapshots:      []health.DeviceHealthSnapshot{{DeviceID: "cam-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var back TelemetryUploadRequest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Runtime != nil || back.StreamID != "" {
		t.Fatalf("a legacy request must not gain fields it never sent: %s", raw)
	}
	if back.SessionID == "" {
		t.Fatalf("a real session id must still survive the round trip: %s", raw)
	}
}
