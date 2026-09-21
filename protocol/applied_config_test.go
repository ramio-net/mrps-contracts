package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The whole point of the pair is that Edge can name exactly what Cloud handed
// it. If either half drops off the wire the round trip cannot close, so both
// directions are checked here rather than trusted.
func TestAppliedConfigClosesTheRoundTrip(t *testing.T) {
	delivered := SyncResponse{
		SessionConfigVersion: 7,
		NextSyncAfterSec:     10,
		TrustState:           "registered",
	}
	rawResponse, err := json.Marshal(delivered)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawResponse), `"session_config_version":7`) {
		t.Fatalf("delivered config is anonymous on the wire: %s", rawResponse)
	}

	applied := time.Date(2026, 8, 3, 17, 30, 0, 0, time.UTC)
	echo := SyncRequest{
		InstallationID: "install-1",
		EdgeID:         "edge-1",
		AppliedConfig: &AppliedConfig{
			SessionID: "edge-1",
			Version:   7,
			AppliedAt: applied,
		},
	}
	rawRequest, err := json.Marshal(echo)
	if err != nil {
		t.Fatal(err)
	}

	var back SyncRequest
	if err := json.Unmarshal(rawRequest, &back); err != nil {
		t.Fatal(err)
	}
	if back.AppliedConfig == nil {
		t.Fatal("applied config did not survive the round trip")
	}
	if back.AppliedConfig.Version != delivered.SessionConfigVersion {
		t.Fatalf("echoed version %d, want the delivered %d", back.AppliedConfig.Version, delivered.SessionConfigVersion)
	}
	if !back.AppliedConfig.AppliedAt.Equal(applied) {
		t.Fatalf("applied_at = %v, want %v", back.AppliedConfig.AppliedAt, applied)
	}
}

// An Edge too old to report what it applied must stay distinguishable from one
// reporting version zero, because Cloud renders the two differently: "taken, not
// confirmed" against a venue that answered.
func TestAppliedConfigAbsentStaysAbsent(t *testing.T) {
	raw, err := json.Marshal(SyncRequest{InstallationID: "install-1", EdgeID: "edge-1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "applied_config") {
		t.Fatalf("silent Edge must not claim a config: %s", raw)
	}

	var back SyncRequest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.AppliedConfig != nil {
		t.Fatalf("absent applied_config decoded to %+v, want nil", back.AppliedConfig)
	}
}

// Zero must not leak into the delivered response either: a sync carrying no
// config should not name a version for it.
func TestSessionConfigVersionOmittedWithoutConfig(t *testing.T) {
	raw, err := json.Marshal(SyncResponse{TrustState: "registered"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "session_config_version") {
		t.Fatalf("version named without a config: %s", raw)
	}
	if !strings.Contains(string(raw), `"session_config":null`) {
		t.Fatalf("nil session config must still encode as explicit null: %s", raw)
	}
}

func TestSyncNegotiationFieldsRoundTrip(t *testing.T) {
	request := SyncRequest{
		InstallationID:          "install-1",
		EdgeID:                  "edge-1",
		KnownKeyIDs:             []string{"p0", "p1"},
		SupportedSchemaVersions: []int{2, 3},
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"supported_schema_versions":[2,3]`,
		`"known_key_ids":["p0","p1"]`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("sync negotiation vector missing %s: %s", want, raw)
		}
	}

	var back SyncRequest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(back.KnownKeyIDs, ","); got != "p0,p1" {
		t.Fatalf("known key ids = %q, want p0,p1", got)
	}
	if got := len(back.SupportedSchemaVersions); got != 2 ||
		back.SupportedSchemaVersions[0] != 2 ||
		back.SupportedSchemaVersions[1] != 3 {
		t.Fatalf("supported schema versions = %+v, want [2 3]", back.SupportedSchemaVersions)
	}
}

func TestSyncNegotiationFieldsAbsentVector(t *testing.T) {
	raw, err := json.Marshal(SyncRequest{InstallationID: "install-1", EdgeID: "edge-1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "known_key_ids") {
		t.Fatalf("absent known_key_ids must stay absent: %s", raw)
	}
	if strings.Contains(string(raw), "supported_schema_versions") {
		t.Fatalf("absent supported_schema_versions must stay absent: %s", raw)
	}

	var back SyncRequest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.KnownKeyIDs != nil {
		t.Fatalf("absent known_key_ids decoded to %+v, want nil", back.KnownKeyIDs)
	}
	if back.SupportedSchemaVersions != nil {
		t.Fatalf("absent supported_schema_versions decoded to %+v, want nil", back.SupportedSchemaVersions)
	}
}

func TestSyncNegotiationFieldsEmptyListVector(t *testing.T) {
	raw := []byte(`{"installation_id":"install-1","edge_id":"edge-1","known_key_ids":[],"supported_schema_versions":[]}`)

	var back SyncRequest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.KnownKeyIDs == nil || len(back.KnownKeyIDs) != 0 {
		t.Fatalf("empty known_key_ids decoded to %+v, want empty non-nil slice", back.KnownKeyIDs)
	}
	if back.SupportedSchemaVersions == nil || len(back.SupportedSchemaVersions) != 0 {
		t.Fatalf("empty supported_schema_versions decoded to %+v, want empty non-nil slice", back.SupportedSchemaVersions)
	}

	echo, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(echo), "known_key_ids") {
		t.Fatalf("empty known_key_ids must not authorize a wire upgrade: %s", echo)
	}
	if strings.Contains(string(echo), "supported_schema_versions") {
		t.Fatalf("empty supported_schema_versions must not authorize a wire upgrade: %s", echo)
	}
}
