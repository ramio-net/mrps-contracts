package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The two shape decisions that were argued over and settled: cameras travel as an
// array (a drawing side iterates series and wants the label beside the numbers), and
// the operator's actions travel as their own list rather than being inferred from
// kinks in the series.
func TestTimelineWireShape(t *testing.T) {
	tl := Timeline{
		BucketSec: 60,
		StartedAt: time.Date(2026, 8, 14, 11, 43, 10, 0, time.UTC),
		Cameras: []CameraTimeline{{
			DeviceKey: "25062RN2DY-7f7e87b7",
			SlotIndex: 1,
			Label:     "CAM-2",
			Buckets: []TimelineBucket{{
				OffsetSec:      60,
				StreamingSec:   60,
				HeldPct:        0.51,
				Cause:          map[string]float64{"none": 0.95, "late": 0.05},
				SyncErrorMsP95: 4.2,
				ApparentMaxMs:  801,
				ApparentP95Ms:  312,
				LossPct:        0.04,
				RetransmitPct:  0.31,
				PlayoutMs:      300,
			}},
		}},
		Events: []TimelineEvent{
			{OffsetSec: 74, Type: EventPlayoutChange, FromMs: 240, ToMs: 300},
			{OffsetSec: 91, Type: EventCalibration, DeviceKey: "25062RN2DY-7f7e87b7", RttMs: 291},
		},
	}
	raw, err := json.Marshal(SessionReport{ReportID: "r1", Cameras: []CameraReport{}, Timeline: &tl})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)

	// An array, not an object keyed by device_key. `"cameras":[` is the whole assertion:
	// a map would encode as `"cameras":{`.
	if !strings.Contains(encoded, `"cameras":[{"device_key":"25062RN2DY-7f7e87b7"`) {
		t.Fatalf("timeline cameras must be an array of objects: %s", encoded)
	}
	for _, field := range []string{
		`"bucket_sec":60`, `"started_at"`, `"events":[`,
		`"apparent_max_ms":801`, `"apparent_p95_ms":312`, `"retransmit_pct":0.31`,
	} {
		if !strings.Contains(encoded, field) {
			t.Fatalf("missing %s in %s", field, encoded)
		}
	}

	var back SessionReport
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Timeline == nil {
		t.Fatal("timeline lost in round trip")
	}
	if got := back.Timeline.Cameras[0].Buckets[0].Cause["late"]; got != 0.05 {
		t.Fatalf("bucket cause shares lost: %+v", back.Timeline.Cameras[0].Buckets[0].Cause)
	}
	if got := back.Timeline.Events[0].ToMs; got != 300 {
		t.Fatalf("playout_change to_ms = %d, want 300", got)
	}
}

// A minute the camera missed is sent as zeros rather than skipped, so the drawing
// side never has to reconstruct a gap — and a reconstructed gap is indistinguishable
// from a perfect minute. That only holds while the zeros are actually on the wire, so
// no numeric bucket field may carry omitempty.
func TestAbsentBucketKeepsItsZeros(t *testing.T) {
	raw, err := json.Marshal(TimelineBucket{OffsetSec: 120, PlayoutMs: 400})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"streaming_sec":0`, `"held_pct":0`, `"sync_error_ms_p95":0`,
		`"apparent_max_ms":0`, `"apparent_p95_ms":0`, `"loss_pct":0`, `"retransmit_pct":0`,
	} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("an absent minute must still say %s: %s", field, raw)
		}
	}
}

// The event list is open by agreement: Edge adds types, Cloud ignores what it cannot
// draw. Decoding must therefore keep an unknown type intact instead of failing, and a
// report from an Edge too old to record a timeline must stay a report.
func TestTimelineIsOptionalAndEventsAreOpen(t *testing.T) {
	raw, err := json.Marshal(SessionReport{ReportID: "r1", Cameras: []CameraReport{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "timeline") {
		t.Fatalf("a report without a timeline must not carry the field: %s", raw)
	}

	var back SessionReport
	if err := json.Unmarshal([]byte(
		`{"report_id":"r1","cameras":[],"timeline":{"bucket_sec":60,"cameras":[],`+
			`"events":[{"offset_sec":5,"type":"something_new_in_2027","text":"hi"}]}}`), &back); err != nil {
		t.Fatalf("unknown event type must decode, not fail: %v", err)
	}
	if back.Timeline.Events[0].Type != "something_new_in_2027" {
		t.Fatalf("unknown event type mangled: %+v", back.Timeline.Events[0])
	}
}
