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
		{Video{Height: 720, BitrateKbps: 3300}, 200},
		{Video{Height: 720, BitrateKbps: 6000}, 240},
		{Video{Height: 1080, BitrateKbps: 6000}, 300},
		{Video{Height: 1080, BitrateKbps: 8500}, 340},
	}
	for _, c := range cases {
		if got := RecommendedPlayoutMs(c.video); got != c.want {
			t.Fatalf("RecommendedPlayoutMs(%+v) = %d, want %d", c.video, got, c.want)
		}
	}
}
