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

// A vector for Console: one complete warning as it travels. Console localizes from code +
// params and falls back to message when anything is missing.
func TestAWarningWithParamsOnTheWire(t *testing.T) {
	w := RuntimeWarning{
		Code: WarningStartSpikeCured,
		Params: map[string]string{
			ParamCamera: "CAM-2", ParamAt: "2026-09-29T06:20:16Z",
			ParamEarlyRTTMs: "201", ParamSettledRTTMs: "1", ParamLateMs: "195",
		},
		Message: "CAM-2 was reconnected by Edge at 06:20:16: its connection opened through a delay spike (RTT 201 → 1 ms) that would have kept it ~195 ms late until it reconnected",
	}
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"code":"start_spike_cured","params":{"at":"2026-09-29T06:20:16Z","camera":"CAM-2","early_rtt_ms":"201","late_ms":"195","settled_rtt_ms":"1"},"message":"CAM-2 was reconnected by Edge at 06:20:16: its connection opened through a delay spike (RTT 201 → 1 ms) that would have kept it ~195 ms late until it reconnected"}`
	if string(raw) != want {
		t.Fatalf("wire form changed:\n got %s\nwant %s", raw, want)
	}
	spec, ok := WarningParamSpecFor(w.Code)
	if !ok {
		t.Fatal("no param spec for start_spike_cured")
	}
	for _, k := range spec.Required {
		if _, ok := w.Params[k]; !ok {
			t.Errorf("the vector lacks required param %q", k)
		}
	}
}

// An Edge from before params — or a line with no values — sends no params key at all, and
// a warning without params still decodes.
func TestParamsAreOptional(t *testing.T) {
	raw, _ := json.Marshal(RuntimeWarning{Code: WarningEdgeNotLinked, Message: "Edge is not linked to Cloud"})
	if strings.Contains(string(raw), "params") {
		t.Fatalf("a warning with no values grew a params key: %s", raw)
	}
	var old RuntimeWarning
	if err := json.Unmarshal([]byte(`{"code":"camera_limit_reached","message":"camera limit reached (4)"}`), &old); err != nil || old.Params != nil {
		t.Fatalf("a pre-params warning did not decode cleanly: %v %v", err, old.Params)
	}
}

// The spec table is what Cloud writes templates against: every code in it must be a real
// code, every key a documented key, and no key may be listed twice.
func TestParamSpecsNameOnlyKnownCodesAndKeys(t *testing.T) {
	codes := map[string]bool{}
	for _, c := range []string{
		WarningDiscoveryNotPublished, WarningEdgeNotLinked, WarningEdgeRevoked,
		WarningPaidSetEnded, WarningPaidSetSuspended, WarningPaidSetWithdrawn, WarningPaidSetEndedOffline,
		WarningCameraLimitReached, WarningSessionTimeLimit, WarningSessionEndingSoon,
		WarningNegotiationRefused, WarningCloudNeverAccepted, WarningCloudSyncStale,
		WarningCapabilityDegraded, WarningCapabilityExpired, WarningCapabilityInvalid,
		WarningCapabilityUnreadable, WarningCapabilityMissing,
		WarningSRTLatencyMismatch, WarningStartSpikeCured, WarningStartSpikeUncured,
		WarningPacketLossPark, WarningPacketLossCamera, WarningOther,
		WarningCalibDisturbedCured, WarningCalibDisturbedUncured, WarningPhoneDropsCamera,
	} {
		codes[c] = true
	}
	keys := map[string]bool{}
	for _, k := range []string{
		ParamCamera, ParamCameras, ParamRunningMs, ParamAssignedMs, ParamAt, ParamEarlyRTTMs,
		ParamSettledRTTMs, ParamLateMs, ParamLossPct, ParamOthersMaxPct, ParamOfflineSec,
		ParamSinceSyncSec, ParamFreshSec, ParamLimitMin, ParamMinutesLeft, ParamLastError,
		ParamCalibRTTMs, ParamFloorMs, ParamErrorMs, ParamDropPct,
	} {
		keys[k] = true
	}
	for code, spec := range warningParams {
		if !codes[code] {
			t.Errorf("param spec for unknown code %q", code)
		}
		if len(spec.Required) == 0 {
			t.Errorf("%s: a spec with no required params should not exist", code)
		}
		seen := map[string]bool{}
		for _, k := range append(append([]string(nil), spec.Required...), spec.Optional...) {
			if !keys[k] {
				t.Errorf("%s: undocumented param key %q", code, k)
			}
			if seen[k] {
				t.Errorf("%s: key %q listed twice", code, k)
			}
			seen[k] = true
		}
	}
	if _, ok := WarningParamSpecFor(WarningOther); ok {
		t.Error("other must carry no params: it is shown by its message")
	}
	spec, _ := WarningParamSpecFor(WarningPacketLossCamera)
	spec.Required[0] = "mutated"
	if again, _ := WarningParamSpecFor(WarningPacketLossCamera); again.Required[0] == "mutated" {
		t.Error("WarningParamSpecFor hands out the table itself; a caller could rewrite the contract")
	}
}

// supported_features rides the signed sync; an Edge that declares nothing sends the request
// it always did.
func TestSupportedFeaturesAreDeclaredNotInferred(t *testing.T) {
	legacy, _ := json.Marshal(SyncRequest{Hostname: "STAGE-LAPTOP", EdgeKind: EdgeKindSoftware})
	if strings.Contains(string(legacy), "supported_features") {
		t.Fatalf("an Edge declaring nothing grew supported_features: %s", legacy)
	}
	raw, _ := json.Marshal(SyncRequest{SupportedFeatures: []string{FeatureReleasedV1}})
	if !strings.Contains(string(raw), `"supported_features":["released_v1"]`) {
		t.Fatalf("released_v1 missing from %s", raw)
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
