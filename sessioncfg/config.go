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
	// Auto runs MRPS Camera's own bitrate automat: each phone lowers its bitrate from its send
	// queue and raises it back, within Edge's bounds (v0.10.7, owner's decision of 10.10.2026 after
	// phases A–C: on by default in every preset — "better to lower the quality a little and deliver
	// the picture than to lose it"). A pointer because absent means ON: a config written before
	// this field, or by a writer that does not know it, keeps the automat on. Only an explicit
	// false — the "Manual" preset — turns it off. Read it through AutoOn.
	Auto *bool `json:"auto,omitempty"`
}

// AutoOn reports whether the bitrate automat runs: on unless the config says false.
func (l LiveBitrate) AutoOn() bool { return l.Auto == nil || *l.Auto }

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
			// Written out, so a default config says what it does rather than relying on the
			// absent-means-on rule. A fresh pointer per call: a caller setting it to false must
			// not turn the automat off in every other default.
			Auto: BoolPtr(true),
		},
	}
}

// BoolPtr returns a pointer to a new copy of v, for optional fields such as LiveBitrate.Auto.
func BoolPtr(v bool) *bool { return &v }

// RecommendedPlayoutMs is the starting playout delay for a video config: more
// resolution and bitrate mean more airtime pressure, more arrival jitter, and more
// buffer needed to keep several cameras aligned.
//
// The floor is 400 ms since 2026-09-29, and with the 400 ms ceiling Validate enforces the
// load-aware part below no longer binds: every video config starts at 400.
//
// History. 280 came from two handsets on 2026-07-29 (720p30, two cameras: one model clean
// from 280, the other from 240), and it went on being the default after the field had
// moved: 400 was confirmed as the margin on 29.08 (0.1–0.7% repeats on three cameras, the
// rest at 280), carried the cleanest broadcast so far on 06.09 (six hours, 0.0006% /
// 0.0017%), and the dress rehearsal of 28.09 ran thirty minutes on three phones at 400
// without a single repeated frame. A venue linked afresh got 280 from Cloud nonetheless —
// found in the owner's Console review of 28.09 — and would have started a multi-camera
// broadcast below the margin it needs. The earlier floor of 180 was a single-camera
// calibration; single-camera work picks its own lower preset explicitly.
//
// This is only the starting point. The operator knob, the Cloud presets and the
// history-based per-device recommendation on Edge sit above it.
const recommendedPlayoutFloorMs = 400

func RecommendedPlayoutMs(v Video) int {
	base := 200
	if v.Height >= 1080 {
		base += 60
	}
	if v.BitrateKbps > 3300 {
		base += (v.BitrateKbps - 3300) * 15 / 1000
	}
	ms := ((base + 10) / 20) * 20
	if ms < recommendedPlayoutFloorMs {
		return recommendedPlayoutFloorMs
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
