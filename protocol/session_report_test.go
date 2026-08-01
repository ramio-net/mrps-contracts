package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func intPtr(v int) *int { return &v }

func TestSessionReportWireShape(t *testing.T) {
	report := SessionReport{
		ReportID:       "report-7f3a",
		DurationSec:    1591,
		Outcome:        OutcomeCompleted,
		PlayoutDelayMs: intPtr(280),
		Cameras: []CameraReport{{
			DeviceKey:      "BP2A-1c9f4e07",
			SlotIndex:      1,
			StreamingSec:   1480,
			Reconnects:     2,
			HeldFramesPct:  10.45,
			SyncErrorMsP95: 5.7,
			CauseHistogram: map[string]float64{"none": 0.9, "late": 0.1},
		}},
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)

	// The session binding is the path, never the body: one report goes to
	// /edge/v1/sessions/{id}/report and another to /edge/v1/reports, and the two
	// bodies must be byte-identical.
	if strings.Contains(encoded, "session_id") {
		t.Fatalf("session_id must not be a body field: %s", encoded)
	}
	for _, field := range []string{`"report_id"`, `"duration_sec"`, `"outcome"`, `"cameras"`, `"playout_delay_ms"`} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}

	var back SessionReport
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.ReportID != report.ReportID || back.PlayoutDelayMs == nil || *back.PlayoutDelayMs != 280 {
		t.Fatalf("round trip = %+v, want %+v", back, report)
	}
	if back.Cameras[0].CauseHistogram["late"] != 0.1 {
		t.Fatalf("cause histogram lost its shares: %+v", back.Cameras[0].CauseHistogram)
	}
}

// An Edge too old to report the buffer must stay distinguishable from one that ran
// at zero, because Cloud writes the column NULL for the first and a number for the
// second. With a plain int the two collapse and every old report reads as a session
// with no buffer at all.
func TestSessionReportKeepsMissingPlayoutDelayDistinct(t *testing.T) {
	var back SessionReport
	if err := json.Unmarshal([]byte(`{"report_id":"r","duration_sec":10,"outcome":"completed","cameras":[]}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.PlayoutDelayMs != nil {
		t.Fatalf("absent playout_delay_ms = %v, want nil", *back.PlayoutDelayMs)
	}
}

// The snapshots are evidence, not data this module owns. Whatever Edge put in them
// has to survive a decode/encode round trip byte for byte, including fields added by
// an Edge newer than the reader — v0.2.0 typed these and silently rewrote both.
func TestSessionReportSnapshotsSurviveUnchanged(t *testing.T) {
	sent := `{"session_id":"local-registered","transport":"live","srt_latency_ms":120,"experimental_knob":42}`
	body := []byte(`{"report_id":"r","duration_sec":10,"outcome":"completed",` +
		`"config_snapshot":` + sent + `,"cameras":[]}`)

	var back SessionReport
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if string(back.ConfigSnapshot) != sent {
		t.Fatalf("snapshot decoded to %s, want the bytes Edge sent", back.ConfigSnapshot)
	}

	raw, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"experimental_knob":42`) {
		t.Fatalf("unknown field did not survive re-encoding: %s", raw)
	}
	for _, invented := range []string{"schema_version", "operational"} {
		if strings.Contains(string(raw), invented) {
			t.Fatalf("re-encoding invented %q: %s", invented, raw)
		}
	}
}

// A report from a session that never had a Cloud session carries no snapshots and
// no warnings, and those must stay out of the body rather than appear as nulls —
// Cloud stores the snapshot columns NOT NULL and defaults them to {}.
func TestSessionReportOmitsEmptyOptionals(t *testing.T) {
	raw, err := json.Marshal(SessionReport{
		ReportID:    "report-empty",
		DurationSec: 12,
		Outcome:     OutcomeAborted,
		Cameras:     []CameraReport{{DeviceKey: "TP1A-04ab21ff", SlotIndex: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"config_snapshot", "capability_snapshot", "warnings", "playout_delay_ms"} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("%s must be omitted when empty: %s", field, raw)
		}
	}
}

// The whole reason SessionConfig lost omitempty: Edge has to tell "Cloud assigned
// no session" apart from "Cloud did not answer that question".
func TestSyncResponseEncodesNullSessionConfig(t *testing.T) {
	raw, err := json.Marshal(SyncResponse{TrustState: "registered"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"session_config":null`) {
		t.Fatalf("nil session config must encode as explicit null: %s", raw)
	}
}
