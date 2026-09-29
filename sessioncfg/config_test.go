package sessioncfg

import (
	"testing"
	"time"

	"github.com/ramio-net/mrps-contracts/capability"
)

func TestDefaultConfigValidates(t *testing.T) {
	p := capability.DemoProfile(time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC))
	cfg := DefaultForProfile(p)
	if err := Validate(cfg, p); err != nil {
		t.Fatal(err)
	}
}

// LIVE is the default transport, and it is a measured decision rather than a
// preference: same clean-network run, LIVE 203 ms latency / 7,6 ms arrival jitter
// against FILE 274 / 14,6. FILE remains the fallback path.
func TestDefaultTransportIsLive(t *testing.T) {
	p := capability.DemoProfile(time.Now())
	cfg := DefaultForProfile(p)
	if cfg.Transport != TransportLive {
		t.Fatalf("default transport = %q, want %q", cfg.Transport, TransportLive)
	}
	if err := Validate(cfg, p); err != nil {
		t.Fatal(err)
	}
}

func TestRejects24FPS(t *testing.T) {
	p := capability.DemoProfile(time.Now())
	cfg := DefaultForProfile(p)
	cfg.Video.FPS = 24
	if err := Validate(cfg, p); err == nil {
		t.Fatal("24 fps must be rejected")
	}
}

func TestRecommendedPlayoutMs(t *testing.T) {
	cases := []struct {
		video Video
		want  int
	}{
		// Every config starts on the margin the field confirmed (29.08, 06.09, 28.09):
		// 400 is both the floor and the ceiling now.
		{Video{Height: 720, BitrateKbps: 3300}, 400},
		{Video{Height: 720, BitrateKbps: 6000}, 400},
		{Video{Height: 1080, BitrateKbps: 6000}, 400},
		{Video{Height: 1080, BitrateKbps: 8500}, 400},
	}
	for _, c := range cases {
		if got := RecommendedPlayoutMs(c.video); got != c.want {
			t.Fatalf("RecommendedPlayoutMs(%+v) = %d, want %d", c.video, got, c.want)
		}
	}
}
