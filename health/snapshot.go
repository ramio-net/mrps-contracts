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
