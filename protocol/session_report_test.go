package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionReportWireShape(t *testing.T) {
	report := SessionReport{
		ReportID:       "report-7f3a",
		DurationSec:    1591,
		Outcome:        OutcomeCompleted,
		PlayoutDelayMs: 280,
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
	if back.ReportID != report.ReportID || back.PlayoutDelayMs != report.PlayoutDelayMs {
		t.Fatalf("round trip = %+v, want %+v", back, report)
	}
	if back.Cameras[0].CauseHistogram["late"] != 0.1 {
		t.Fatalf("cause histogram lost its shares: %+v", back.Cameras[0].CauseHistogram)
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
