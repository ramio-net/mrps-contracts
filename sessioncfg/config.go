package sessioncfg

import (
	"fmt"

	"github.com/ramio-net/mrps-contracts/capability"
)

const SchemaVersion = 2

type Video struct {
	Preset      string `json:"preset"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	BitrateKbps int    `json:"bitrate_kbps"`
	Codec       string `json:"codec"`
	Profile     string `json:"profile"`
}

type Audio struct {
	Channels    int `json:"channels"`
	SampleRate  int `json:"sample_rate"`
	BitrateKbps int `json:"bitrate_kbps"`
}

type Cameras struct {
	MaxCameras int `json:"max_cameras"`
}

type SoftSync struct {
	Enabled  bool `json:"enabled"`
	WindowMs int  `json:"window_ms"`
	RateMs   int  `json:"rate_ms"`
}

type Adaptation struct {
	Slew        bool `json:"slew"`
	RequestIDR  bool `json:"request_idr"`
	PlayoutDrop bool `json:"playout_delay"`
}

type LiveBitrate struct {
	Enabled  bool `json:"enabled"`
	MinKbps  int  `json:"min_kbps"`
	MaxKbps  int  `json:"max_kbps"`
	StepKbps int  `json:"step_kbps"`
}

type Operational struct {
	PlayoutDelayMs int         `json:"playout_delay_ms"`
	FrameSync      bool        `json:"frame_sync"`
	SoftSync       SoftSync    `json:"soft_sync"`
	Adaptation     Adaptation  `json:"adaptation"`
	LiveBitrate    LiveBitrate `json:"live_bitrate"`
}

type Config struct {
	SchemaVersion    int               `json:"schema_version"`
	SessionID        string            `json:"session_id"`
	SessionName      string            `json:"session_name"`
	CapabilityPreset capability.Preset `json:"capability_preset"`
	CapabilitySource string            `json:"capability_source"`
	Video            Video             `json:"video"`
	Audio            Audio             `json:"audio"`
	Cameras          Cameras           `json:"cameras"`
	Transport        string            `json:"transport"`
	SRTLatencyMs     int               `json:"srt_latency_ms"`
	Operational      Operational       `json:"operational"`
}

const (
	TransportFile = "file"
	TransportLive = "live"
)

func DefaultForProfile(profile capability.Profile) Config {
	video := Video{
		Preset:      "standard_720p30",
		Width:       1280,
		Height:      720,
		FPS:         30,
		BitrateKbps: 3300,
		Codec:       "h264",
		Profile:     "main",
	}
	name := "MRPS Registered"
	if profile.Preset == capability.PresetDemo {
		name = "MRPS Demo"
	}
	return Config{
		SchemaVersion:    SchemaVersion,
		SessionID:        "local-" + string(profile.Preset),
		SessionName:      name,
		CapabilityPreset: profile.Preset,
		CapabilitySource: profile.Source,
		Video:            video,
		Audio: Audio{
			Channels:    2,
			SampleRate:  48000,
			BitrateKbps: 128,
		},
		Cameras: Cameras{MaxCameras: profile.Limits.MaxCameras},
		// LIVE, not FILE. Measured on one clean-network run, one camera, 120 ms
		// buffer: LIVE 203 ms latency and 7,6 ms arrival jitter against FILE 274 and
		// 14,6. The TSBPD window is not overhead, it is smoothing. Confirmed by a
		// field A/B on a bad network, where LIVE held at 360 ms and FILE barely held
		// at 400. FILE stays in the contract as the fallback path, not as the default.
		Transport:    TransportLive,
		SRTLatencyMs: 120,
		Operational:  DefaultOperational(video),
	}
}

func DefaultOperational(v Video) Operational {
	return Operational{
		PlayoutDelayMs: RecommendedPlayoutMs(v),
		FrameSync:      true,
		SoftSync: SoftSync{
			Enabled:  true,
			WindowMs: 3000,
			RateMs:   10,
		},
		Adaptation: Adaptation{
			Slew:        true,
			RequestIDR:  true,
			PlayoutDrop: true,
		},
		LiveBitrate: LiveBitrate{
			Enabled:  true,
			MinKbps:  1500,
			MaxKbps:  8500,
			StepKbps: 500,
		},
	}
}

// RecommendedPlayoutMs is the starting playout delay for a video config: more
// resolution and bitrate mean more airtime pressure, more arrival jitter, and more
// buffer needed to keep several cameras aligned.
//
// The floor is 280 ms because two real handsets said so. Measured 2026-07-29 on a
// two-camera run at 720p30: one model was clean from 280 ms, the other from 240.
// The buffer is one for the whole session and is set by the worst camera, so 280 is
// the value at which both are clean. The earlier floor of 180 came from a
// single-camera calibration and was below what multi-camera actually needs — a
// session started there spends its first minutes repeating frames while the operator
// hunts for the knob.
//
// This is only the sensible starting point. The operator knob and the history-based
// per-device recommendation on Edge sit above it.
func RecommendedPlayoutMs(v Video) int {
	base := 200
	if v.Height >= 1080 {
		base += 60
	}
	if v.BitrateKbps > 3300 {
		base += (v.BitrateKbps - 3300) * 15 / 1000
	}
	ms := ((base + 10) / 20) * 20
	if ms < 280 {
		return 280
	}
	if ms > 400 {
		return 400
	}
	return ms
}

func Validate(cfg Config, profile capability.Profile) error {
	if cfg.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %d", cfg.SchemaVersion)
	}
	if !validResolution(cfg.Video.Width, cfg.Video.Height) {
		return fmt.Errorf("unsupported resolution %dx%d", cfg.Video.Width, cfg.Video.Height)
	}
	if !oneOf(cfg.Video.FPS, 25, 30, 60) {
		return fmt.Errorf("unsupported fps %d", cfg.Video.FPS)
	}
	if cfg.Video.BitrateKbps < 500 || cfg.Video.BitrateKbps > 20000 {
		return fmt.Errorf("bitrate_kbps out of range [500,20000]")
	}
	if cfg.Video.Codec != "h264" {
		return fmt.Errorf("unsupported codec %q", cfg.Video.Codec)
	}
	if cfg.Video.Profile != "main" && cfg.Video.Profile != "baseline" {
		return fmt.Errorf("unsupported h264 profile %q", cfg.Video.Profile)
	}
	if cfg.Audio.Channels != 1 && cfg.Audio.Channels != 2 {
		return fmt.Errorf("audio channels must be 1 or 2")
	}
	if cfg.Audio.SampleRate != 48000 {
		return fmt.Errorf("audio sample_rate must be 48000")
	}
	if cfg.Audio.BitrateKbps < 64 || cfg.Audio.BitrateKbps > 192 {
		return fmt.Errorf("audio bitrate_kbps out of range [64,192]")
	}
	if cfg.Transport != TransportFile && cfg.Transport != TransportLive {
		return fmt.Errorf("unsupported transport %q", cfg.Transport)
	}
	if cfg.SRTLatencyMs < 20 || cfg.SRTLatencyMs > 400 {
		return fmt.Errorf("srt_latency_ms out of range [20,400]")
	}
	if cfg.Operational.PlayoutDelayMs < 120 || cfg.Operational.PlayoutDelayMs > 400 {
		return fmt.Errorf("operational.playout_delay_ms out of range [120,400]")
	}
	if !oneOf(cfg.Operational.SoftSync.WindowMs, 1000, 3000, 5000, 10000) {
		return fmt.Errorf("unsupported soft_sync.window_ms %d", cfg.Operational.SoftSync.WindowMs)
	}
	if !oneOf(cfg.Operational.SoftSync.RateMs, 5, 10, 20, 40) {
		return fmt.Errorf("unsupported soft_sync.rate_ms %d", cfg.Operational.SoftSync.RateMs)
	}
	if cfg.Cameras.MaxCameras < 1 || cfg.Cameras.MaxCameras > profile.Limits.MaxCameras {
		return fmt.Errorf("max_cameras exceeds profile limit")
	}
	return nil
}

func validResolution(w, h int) bool {
	return (w == 960 && h == 540) || (w == 1280 && h == 720) || (w == 1920 && h == 1080)
}

func oneOf(v int, xs ...int) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
