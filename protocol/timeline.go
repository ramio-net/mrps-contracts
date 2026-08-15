package protocol

import "time"

// Timeline is the per-minute summary of one broadcast, carried inside the same
// SessionReport as the aggregates.
//
// It exists because the aggregates alone could not answer the question the field
// actually asks. Two of them, in fact:
//
//   - "in the cloud there are numbers, but is that an average or a spread, at what
//     time, and what affected it" — the customer, after a four-hour broadcast. A
//     single held percentage cannot say that a camera was clean for three hours and
//     fell apart during the last twenty minutes, and those are different diagnoses
//     with different fixes.
//   - the aggregates were also WRONG, and this is what makes the accumulator a fix
//     rather than a feature. Edge computed them from a rolling health ring holding
//     300 snapshots at 1 Hz, so held_frames_pct, cause_histogram and sync_error_ms_p95
//     described the last five minutes before the stop and claimed to describe the
//     session. On 14.08.2026 that was five minutes standing in for four hours and
//     fourteen. Summing as the session runs fixes the average and produces the series
//     in the same pass.
//
// Not telemetry: nothing is streamed, nothing is queried live. Edge folds one bucket
// a minute in memory and the whole series travels once, with the report. Measured
// size for a 254-minute broadcast with two cameras: a few tens of kilobytes.
type Timeline struct {
	// BucketSec is the width of one bucket. 60 today; sent so a reader never infers
	// the width from the offsets and gets it wrong on a session shorter than one
	// bucket.
	BucketSec int `json:"bucket_sec"`

	// StartedAt is the moment the production session began — the first camera joining,
	// not Edge starting. Every OffsetSec in this structure, in buckets and in events
	// alike, counts from here, so markers land on the series without either side
	// recomputing anything.
	StartedAt time.Time `json:"started_at"`

	// Cameras is an array rather than a map keyed by device: a drawing side iterates
	// series and needs the label and slot next to the numbers, and a map forces it to
	// carry the key separately from the value.
	Cameras []CameraTimeline `json:"cameras"`

	// Events are what the operator did and what happened to the cameras. They are a
	// separate list on purpose, and the reason is measured: the first draft assumed
	// operator actions would be visible as kinks in the series. The 14.08 broadcast
	// disproved it — the playout margin was moved fifteen times and the clocks aligned
	// twice; from the kinks alone maybe half of those are recoverable, and the REASON
	// for none of them.
	Events []TimelineEvent `json:"events,omitempty"`
}

// CameraTimeline is one camera's series. Buckets cover the whole session, including
// the time this camera was absent — see TimelineBucket.StreamingSec.
type CameraTimeline struct {
	DeviceKey string           `json:"device_key"`
	SlotIndex int              `json:"slot_index"`
	Label     string           `json:"label,omitempty"`
	Buckets   []TimelineBucket `json:"buckets"`
}

// TimelineBucket is one minute of one camera.
//
// Buckets are emitted for the whole session even when the camera was not there. The
// alternative — omitting them — has to be repaired at drawing time, and a gap is
// indistinguishable from a perfect minute. StreamingSec: 0 says it outright, and the
// extra weight is negligible.
type TimelineBucket struct {
	// OffsetSec is the START of the bucket, in seconds from Timeline.StartedAt.
	OffsetSec int `json:"offset_sec"`

	// StreamingSec is how many of this bucket's seconds the camera actually delivered
	// frames — 0..BucketSec. A boolean was the first draft and it was worse: a camera
	// that joined mid-minute neither "was" nor "wasn't" there, and HeldPct for such a
	// minute cannot be read without knowing how much of it is real.
	StreamingSec int `json:"streaming_sec"`

	// HeldPct is held frames as a percentage of frames output in this bucket. Not an
	// average of the health snapshots' rolling rates: those overlap, and averaging them
	// double-counts. This is the exact count over the bucket.
	HeldPct float64 `json:"held_pct"`

	// Cause is the SHARE of this bucket's streaming seconds attributed to each held
	// cause (none | late | starved | resync). Same units as CameraReport.CauseHistogram
	// and for the same reason — shares stay comparable between buckets of unequal
	// streaming time. Values sum to 1 within one bucket, or the map is empty when the
	// camera delivered nothing.
	//
	// It is a map rather than one dominant string because late and starved are cured by
	// opposite actions (raise the buffer / lower the bitrate), and a minute that was
	// half of each is a different situation from a minute that was all of one.
	Cause map[string]float64 `json:"cause,omitempty"`

	// SyncErrorMsP95 is the 95th percentile of inter-camera sync error over the bucket,
	// by nearest rank. Only measured with two or more cameras up; 0 in single-camera
	// stretches, where the quantity does not exist.
	SyncErrorMsP95 float64 `json:"sync_error_ms_p95"`

	// ApparentMaxMs and ApparentP95Ms are the worst and the 95th-percentile arrival
	// latency of this camera's frames in this bucket — wall clock now minus the frame's
	// corrected timestamp, so capture, encode, the send queue and the air are all in it.
	//
	// Both are needed and they answer different questions. Max explains WHY frames were
	// held: a frame is held when its latency crosses the playout deadline, so it is the
	// tail that breaks the picture, and this is the line that crosses PlayoutMs. P95
	// says whether that was the camera's normal state or a rare spike. On 14.08 one
	// handset reached 1178 ms with a p95 near 300: by the max alone it looked hopeless,
	// while the real fault was rare outliers of the device itself.
	ApparentMaxMs float64 `json:"apparent_max_ms"`
	ApparentP95Ms float64 `json:"apparent_p95_ms"`

	// LossPct and RetransmitPct are both shares of the SRT packets RECEIVED in this
	// bucket — one denominator deliberately, so the two can be drawn on one axis and
	// compared. Loss that was repaired by retransmission is the ordinary case on Wi-Fi
	// and looks identical to a clean link in the picture; the pair is what tells them
	// apart.
	LossPct       float64 `json:"loss_pct"`
	RetransmitPct float64 `json:"retransmit_pct"`

	// PlayoutMs is the playout margin in force at the END of this bucket. Held rates
	// are unreadable without it — 10% at 240 ms and 10% at 400 ms are two different
	// handsets and two different diagnoses — and the margin moves during a broadcast.
	//
	// End of bucket, not an average: a minute containing 13:06:00 → 240 and
	// 13:06:03 → 300 carries 300, and the change itself arrives as a playout_change
	// event with from/to. The line then never draws a value that was never set, and the
	// exact moment is still visible as a marker.
	PlayoutMs int `json:"playout_ms"`
}

// Event types Edge knows and emits. The list is open: new types will be added and old
// ones will not be renamed. A reader that cannot draw a type must ignore it, not fail.
const (
	EventCameraJoin    = "camera_join"
	EventCameraLeave   = "camera_leave"
	EventCalibration   = "calibration"
	EventPlayoutChange = "playout_change"
	EventClockAlign    = "clock_align"
	EventVideoChange   = "video_change"
	EventWarning       = "warning"
)

// TimelineEvent is one thing that happened during the session, at OffsetSec from
// Timeline.StartedAt. Fields not used by a given Type are omitted.
type TimelineEvent struct {
	OffsetSec int    `json:"offset_sec"`
	Type      string `json:"type"`

	// DeviceKey is set on the per-camera types: camera_join, camera_leave, calibration.
	DeviceKey string `json:"device_key,omitempty"`

	// RttMs carries the quality of a calibration, not just its occurrence. The floor of
	// any calibration is about 2 × srt_latency (measured on a loopback: 259 ms at 120 ms
	// latency over a zero-latency network), so a figure well above that means the phone
	// measured its clock offset through a disturbance and will carry that error until it
	// recalibrates.
	RttMs float64 `json:"rtt_ms,omitempty"`

	// FromMs/ToMs: playout_change.
	FromMs int `json:"from_ms,omitempty"`
	ToMs   int `json:"to_ms,omitempty"`

	// DivergenceBeforeMs: clock_align — how far the cameras' clocks had drifted apart
	// when the operator pressed it, which is the number that says whether pressing it
	// was warranted.
	DivergenceBeforeMs float64 `json:"divergence_before_ms,omitempty"`

	// video_change.
	Width       int `json:"width,omitempty"`
	Height      int `json:"height,omitempty"`
	FPS         int `json:"fps,omitempty"`
	BitrateKbps int `json:"bitrate_kbps,omitempty"`

	// Text: warning.
	Text string `json:"text,omitempty"`
}
