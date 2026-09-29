package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Contract v0.10.0: what the owner's Console review of 28.09.2026 found missing.

// "Not reported" and "nothing to report" must stay two different things on the wire: until
// this field existed the Edge screen said "Warnings: not reported" forever.
func TestWarningsTellNotReportedFromNone(t *testing.T) {
	var old EdgeRuntime
	if err := json.Unmarshal([]byte(`{"observed_at":"2026-09-29T08:00:00Z","uptime_sec":1,"session_state":"idle","cameras":[]}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Warnings != nil {
		t.Fatal("an Edge that sends no warnings field decoded as having none, not as not reporting")
	}

	raw, _ := json.Marshal(EdgeRuntime{ObservedAt: time.Now(), SessionState: SessionStateIdle, Warnings: []RuntimeWarning{}})
	if !strings.Contains(string(raw), `"warnings":[]`) {
		t.Fatalf("a reporting Edge with nothing to say must send [], got %s", raw)
	}
	var none EdgeRuntime
	if err := json.Unmarshal(raw, &none); err != nil || none.Warnings == nil {
		t.Fatalf("[] decoded as not reported (%v)", err)
	}
}

func TestAWarningCarriesItsCodeAndTheNumbers(t *testing.T) {
	raw, _ := json.Marshal(EdgeRuntime{Warnings: []RuntimeWarning{{
		Code:    WarningStartSpikeCured,
		Message: "CAM-2 was reconnected by Edge at 06:20:16: its connection opened through a delay spike (RTT 201 → 1 ms)",
	}}})
	for _, want := range []string{`"code":"start_spike_cured"`, `RTT 201 → 1 ms`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s missing from %s", want, raw)
		}
	}
}

// The three readings Cloud cannot derive: absent when not known, present when they are.
func TestRuntimeReadingsAreAbsentUntilKnown(t *testing.T) {
	raw, _ := json.Marshal(EdgeRuntime{Warnings: []RuntimeWarning{}})
	for _, key := range []string{`"transport"`, `"max_cameras_in_force"`, `"session_elapsed_sec"`} {
		if strings.Contains(string(raw), key) {
			t.Errorf("%s sent while unknown: %s", key, raw)
		}
	}
	eight, elapsed := 8, 4260
	raw, _ = json.Marshal(EdgeRuntime{Transport: "live", MaxCamerasInForce: &eight, SessionElapsedSec: &elapsed, Warnings: []RuntimeWarning{}})
	for _, want := range []string{`"transport":"live"`, `"max_cameras_in_force":8`, `"session_elapsed_sec":4260`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s missing from %s", want, raw)
		}
	}
}

// Hostname and kind ride the claim and every sync; an older Edge sends neither and must
// still produce exactly the request it always did.
func TestIdentityFieldsAreOptional(t *testing.T) {
	legacy, _ := json.Marshal(ClaimRequest{InstallationID: "i", EdgeVersion: "v", OS: "windows"})
	if strings.Contains(string(legacy), "hostname") || strings.Contains(string(legacy), "edge_kind") {
		t.Fatalf("a legacy claim request changed shape: %s", legacy)
	}
	raw, _ := json.Marshal(SyncRequest{Hostname: "STAGE-LAPTOP", EdgeKind: EdgeKindSoftware})
	for _, want := range []string{`"hostname":"STAGE-LAPTOP"`, `"edge_kind":"software"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s missing from %s", want, raw)
		}
	}
	resp, _ := json.Marshal(SyncResponse{TrustState: TrustStateReleased, EdgeName: "Ноутбук — стенд"})
	for _, want := range []string{`"trust_state":"released"`, `"edge_name":"Ноутбук — стенд"`} {
		if !strings.Contains(string(resp), want) {
			t.Errorf("%s missing from %s", want, resp)
		}
	}
}
