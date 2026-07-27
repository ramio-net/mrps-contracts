package capability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenVectors(t *testing.T) {
	for _, name := range []string{"registered", "full"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var p Profile
			if err := json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			got, err := CanonicalBytes(p)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", name+".canonical"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
				t.Fatalf("canonical mismatch\n got: %s\nwant: %s", got, want)
			}
			if err := Sign(&p, DevKeyID, DevPrivateKey()); err != nil {
				t.Fatal(err)
			}
			if err := Verify(p, DevKeySet()); err != nil {
				t.Fatal(err)
			}
		})
	}
}
