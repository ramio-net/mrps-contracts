package health

import "time"

type TransportSection struct {
	RTTMs      float64 `json:"rtt_ms"`
	PacketLoss float64 `json:"packet_loss_pct"`
	SeqGaps    int64   `json:"seq_gaps"`
}

type PipelineSection struct {
	IngestFPS       float64 `json:"ingest_fps"`
	BufferDepthMs   float64 `json:"buffer_depth_ms"`
	HeldFramesRate  float64 `json:"held_frames_rate"`
	DecoderQueueLen int     `json:"decoder_queue_len,omitempty"`
}

type SyncSection struct {
	SyncErrorMs float64 `json:"sync_error_ms"`
	Confidence  string  `json:"confidence"`
}

type OutputSection struct {
	OutputFPS float64 `json:"output_fps"`
	Dropped   int64   `json:"dropped"`
}

type CauseSection struct {
	Held  string `json:"held"`
	Lever string `json:"lever"`
}

type EncoderSection struct {
	BitrateKbps int    `json:"bitrate_kbps"`
	Profile     string `json:"profile"`
}

type SystemSection struct {
	CPUPercent float64 `json:"cpu_pct"`
	MemMB      int     `json:"mem_mb"`
}

type DeviceHealthSnapshot struct {
	DeviceID        string           `json:"device_id"`
	Ts              time.Time        `json:"ts"`
	SnapshotVersion int              `json:"snapshot_version"`
	WindowSec       int              `json:"window_sec"`
	StreamingSec    int              `json:"streaming_sec"`
	Transport       TransportSection `json:"transport"`
	Pipeline        PipelineSection  `json:"pipeline"`
	Sync            SyncSection      `json:"sync"`
	Output          OutputSection    `json:"output"`
	Cause           CauseSection     `json:"cause"`
	Encoder         *EncoderSection  `json:"encoder,omitempty"`
	System          *SystemSection   `json:"system,omitempty"`

	// State is the diagnosis for this same device at this same moment, carried
	// alongside the measurement rather than merged into it. Edge already computes
	// it; without this field a consumer had to guess, or join a measurement to a
	// verdict from some other second.
	State *DeviceHealthState `json:"state,omitempty"`
	// Latency and SRT are optional throughout: a value that was not measured is
	// absent, never zero. Zero is a legitimate reading for several of them.
	Latency *LatencySection `json:"latency,omitempty"`
	SRT     *SRTSection     `json:"srt,omitempty"`
}

// LatencySection splits the delay a camera actually shows into parts an operator
// can act on.
//
// The split matters because no other signal separates a slow handset from bad air:
// the fast metrics are all inter-frame, and a constant extra delay is invisible to
// every one of them.
type LatencySection struct {
	// HandsetMs is PhoneTs − SensorTs: capture and encode on the phone's own clock,
	// so it holds no network time at all.
	HandsetMs *float64 `json:"handset_ms,omitempty"`
	// NetworkMs is DERIVED, not measured: ApparentMs − HandsetMs. It is everything
	// else — send queue, air, SRT and our read — which means the negotiated SRT
	// delay sits INSIDE it. That is why a camera that settled on 280 ms instead of
	// the announced 120 read 295 here while its neighbours read 130, with zero
	// packet loss and an RTT of 5.5 ms. Do not recompute it from RTT: RTT is not
	// part of it.
	NetworkMs *float64 `json:"network_ms,omitempty"`
	// ApparentMs is wall_now − corrected_ts, measured by Edge.
	ApparentMs *float64 `json:"apparent_latency_ms,omitempty"`
	// PlayoutMs is a property of the whole session, not of one camera. Repeated per
	// snapshot so a reader never has to join it in from somewhere else.
	PlayoutMs *float64 `json:"playout_ms,omitempty"`
	// MarginMs is what is left before frames start being held. It can be negative,
	// which is exactly the interesting case, so absence and zero are different.
	MarginMs *float64 `json:"margin_ms,omitempty"`
}

// SRTSection reports what this connection asked for against what it got.
//
// SRT negotiates the MAXIMUM of the two ends at connect time, so an announced 120
// becomes 280 if the handset asks for more, and no transport counter says so.
// Comparing these two is an absolute check: it works with a single camera, where
// comparing against neighbours cannot.
type SRTSection struct {
	// ConnectionID identifies the connection these two values belong to. Latency
	// applies on the phone's next reconnect, so a knob changed mid-broadcast leaves
	// the live connection on the old value legitimately — that is "awaiting
	// reconnect", not a fault, and only a per-connection comparison tells them apart.
	ConnectionID string `json:"connection_id,omitempty"`
	// AssignedLatencyMs is what Edge announced for this connection.
	AssignedLatencyMs *int `json:"assigned_latency_ms,omitempty"`
	// NegotiatedLatencyMs is what SRT settled on (MsRcvTsbPdDelay). MsRcvBuf is not
	// a substitute: that is the undelivered timespan sitting in the buffer and it
	// moves on its own, while this is a settled property of the connection.
	NegotiatedLatencyMs *int `json:"negotiated_latency_ms,omitempty"`
}

type DeviceHealthState struct {
	DeviceID      string    `json:"device_id"`
	Ts            time.Time `json:"ts"`
	ScorerVersion int       `json:"scorer_version"`
	Lifecycle     string    `json:"lifecycle"`
	Quality       string    `json:"quality"`
	Score         int       `json:"score"`
	Confidence    string    `json:"confidence"`
	Trend         string    `json:"trend"`
	PrimaryReason string    `json:"primary_reason"`
	Reasons       []string  `json:"reasons"`
	OperatorHint  string    `json:"operator_hint"`
	SinceSec      int       `json:"since_sec"`
}

func FakeSnapshot(deviceID string, ts time.Time, seq int, cause CauseSection) DeviceHealthSnapshot {
	loss := 0.1
	held := 0.0
	if cause.Held == "starved" {
		loss = 4.5
		held = 0.12
	}
	if cause.Held == "late" {
		held = 0.08
	}
	return DeviceHealthSnapshot{
		DeviceID:        deviceID,
		Ts:              ts.UTC(),
		SnapshotVersion: 1,
		WindowSec:       5,
		StreamingSec:    seq,
		Transport: TransportSection{
			RTTMs:      7.0 + float64(seq%5),
			PacketLoss: loss,
			SeqGaps:    int64(seq / 30),
		},
		Pipeline: PipelineSection{
			IngestFPS:      30,
			BufferDepthMs:  200,
			HeldFramesRate: held,
		},
		Sync: SyncSection{
			SyncErrorMs: 4.0,
			Confidence:  "high",
		},
		Output: OutputSection{
			OutputFPS: 30,
		},
		Cause: cause,
		Encoder: &EncoderSection{
			BitrateKbps: 3300,
			Profile:     "main",
		},
		System: &SystemSection{
			CPUPercent: 35,
			MemMB:      512,
		},
	}
}
