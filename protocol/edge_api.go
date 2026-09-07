package protocol

import (
	"time"

	"github.com/ramio-net/mrps-contracts/capability"
	"github.com/ramio-net/mrps-contracts/health"
	"github.com/ramio-net/mrps-contracts/sessioncfg"
)

type ClaimRequest struct {
	InstallationID string `json:"installation_id"`
	EdgeVersion    string `json:"edge_version"`
	OS             string `json:"os"`
}

type ClaimRequestResponse struct {
	Code      string    `json:"code"`
	PollToken string    `json:"poll_token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ClaimPollRequest struct {
	PollToken string `json:"poll_token"`
}

type ClaimPollResponse struct {
	Status     string `json:"status"`
	EdgeID     string `json:"edge_id,omitempty"`
	OrgID      string `json:"org_id,omitempty"`
	EdgeSecret string `json:"edge_secret,omitempty"`
}

type ClaimConfirmRequest struct {
	Code     string `json:"code"`
	EdgeName string `json:"edge_name,omitempty"`
}

type ClaimConfirmResponse struct {
	Status         string `json:"status"`
	InstallationID string `json:"installation_id"`
	EdgeID         string `json:"edge_id"`
	OrgID          string `json:"org_id"`
}

type SyncRequest struct {
	InstallationID  string    `json:"installation_id"`
	EdgeID          string    `json:"edge_id"`
	EdgeVersion     string    `json:"edge_version"`
	ContractVersion string    `json:"contract_version"`
	LocalTime       time.Time `json:"local_time"`

	// AppliedConfig is the config Edge is actually running as it asks. It is the
	// only way Cloud can honestly say a venue applied something: delivery alone
	// proves the bytes were fetched, not that the venue acted on them, and an
	// operator told "applied" about a venue that never started is exactly the kind
	// of confident lie this contract keeps removing.
	//
	// Absent from an Edge too old to report it, which Cloud must render as "taken,
	// not confirmed" rather than assuming either answer.
	AppliedConfig *AppliedConfig `json:"applied_config,omitempty"`
}

// AppliedConfig names one delivered configuration exactly.
//
// The pair matters. A venue's config comes from two independent sources — the
// base config Cloud keeps for the venue, and the config of a production session
// while one is live — and their version counters are unrelated. A bare number
// would be ambiguous at precisely the moments that matter: the sync where a
// broadcast starts, and the one where it ends.
type AppliedConfig struct {
	// SessionID is what the delivered config carried: a production session id
	// during a broadcast, the edge id for the venue's base config.
	SessionID string `json:"session_id"`
	Version   int    `json:"version"`
	// AppliedAt is Edge's own clock. Cloud stores it as reported and does not
	// compare it with server time: the two are not synchronised, and this project
	// has already paid for treating one clock's reading as another's.
	AppliedAt time.Time `json:"applied_at"`
}

type SyncResponse struct {
	CapabilityProfile capability.Profile `json:"capability_profile"`
	// Nil means Cloud has no production session assigned to this Edge, and that is
	// encoded as an explicit null rather than dropped: "no session" is an answer Edge
	// acts on — it keeps its local config — not a missing value. With omitempty the
	// two were indistinguishable on the wire, which forced Cloud to mirror this
	// struct locally just to say null.
	SessionConfig *sessioncfg.Config `json:"session_config"`
	// SessionConfigVersion names the config above so Edge can report back which one
	// it is running. Without it the delivered config is anonymous and the round
	// trip cannot close: Edge would have nothing to echo, and Cloud nothing to
	// match. Zero when SessionConfig is nil.
	SessionConfigVersion int       `json:"session_config_version,omitempty"`
	ServerTime           time.Time `json:"server_time"`
	NextSyncAfterSec     int       `json:"next_sync_after_sec"`
	TrustState           string    `json:"trust_state,omitempty"`
}

type TelemetryUploadRequest struct {
	InstallationID string `json:"installation_id"`
	EdgeID         string `json:"edge_id"`
	// SessionID is OPTIONAL. A field broadcast runs without a Cloud session at all —
	// that is the normal case, not a degraded one — and when it is absent the
	// installation and organization come from the signed credentials. When present
	// it must be a real production_sessions UUID and nothing else: there is no
	// foreign key on telemetry_snapshots, so putting an edge id here would be
	// accepted in silence and file the readings under a session that never existed.
	SessionID string `json:"session_id,omitempty"`
	// StreamID identifies one run of the sender, so a late packet from a previous
	// run cannot overwrite a newer one. Restarting Edge starts a new StreamID.
	StreamID string `json:"stream_id,omitempty"`
	// Seq is monotonic within one StreamID. A retry repeats the same pair with the
	// same payload; the same pair with different content is a conflict, not an
	// update.
	Seq int64 `json:"seq"`
	// Snapshots may be EMPTY. An empty batch means "Edge is here, no cameras" and
	// is the difference between silence and idleness — a distinction the console
	// cannot make from an absent request.
	Snapshots []health.DeviceHealthSnapshot `json:"snapshots"`
	// Runtime is what is true right now, as opposed to the snapshots, which may be
	// a backlog delivered after a venue came back online. Every upload carries a
	// fresh Runtime; a replayed backlog must never be read as the current picture.
	Runtime *EdgeRuntime `json:"runtime,omitempty"`
}

// Session states an Edge reports. Grace is a session whose cameras have all left
// but which has not been declared over yet.
const (
	SessionStateIdle   = "idle"
	SessionStateActive = "active"
	SessionStateGrace  = "grace"
)

// EdgeRuntime is the venue's current state, separate from accumulated readings.
//
// The camera list is complete rather than incremental on purpose: a full list can
// show a camera has gone, a stream of additions cannot.
type EdgeRuntime struct {
	// ObservedAt is the Edge's own clock. Liveness is judged by the server's receipt
	// time instead — a venue's clock is not something to trust, we have measured it.
	ObservedAt time.Time `json:"observed_at"`
	UptimeSec  int       `json:"uptime_sec"`
	// LocalSessionID is opaque and local. It is NOT a Cloud session id and must not
	// be stored as one.
	LocalSessionID string `json:"local_session_id,omitempty"`
	// SessionState is one of the constants above. A reader must tolerate values it
	// does not know rather than fall back to idle: an unknown state is unknown, and
	// claiming "no broadcast" while one is running is the worse error.
	SessionState string `json:"session_state"`
	// PlayoutMs is the deadline the held rates are measured against. Without it a
	// stored reading cannot be compared with any other.
	PlayoutMs *int            `json:"playout_ms,omitempty"`
	Cameras   []RuntimeCamera `json:"cameras"`
	// AUX sources are a separate list, never cameras with an invented slot index.
	// An AUX source at 5 fps holds about 0.8 of its frames by construction, and in
	// camera statistics that reads as the worst camera in the park forever.
	AUX []RuntimeAUX `json:"aux,omitempty"`
	// Capability is what Edge observes about its own permissions. It grants nothing
	// and replaces no signature — Cloud decides rights, Edge reports what it applied.
	Capability *CapabilityState `json:"capability,omitempty"`
}

type RuntimeCamera struct {
	DeviceID     string `json:"device_id"`
	SlotIndex    int    `json:"slot_index"`
	ConnectionID string `json:"connection_id,omitempty"`
	Status       string `json:"status"`
}

type RuntimeAUX struct {
	SourceID string  `json:"source_id"`
	Label    string  `json:"label,omitempty"`
	Status   string  `json:"status"`
	FPS      float64 `json:"fps,omitempty"`
}

// Reasons the next session's capability set differs from the current one.
const (
	CapabilityNextFullAvailable        = "full_available"
	CapabilityNextRightExpired         = "right_expired"
	CapabilityNextOfflineWindowExpired = "offline_window_expired"
	CapabilityNextBothExpired          = "both_expired"
	CapabilityNextInvalidProfile       = "invalid_profile"
	CapabilityNextRevoked              = "revoked"
)

// CapabilityState carries two observations, not two tariff modes.
//
// A broadcast already running keeps the set it started with, so "this session is
// running on the full set while the next one will be free" is a correct state, not
// a contradiction. Without both halves the console would show the free tier while
// eight cameras are legitimately on air.
type CapabilityState struct {
	Current *CapabilitySet `json:"current,omitempty"`
	// Next is absent while Edge cannot know it. Until the signed profile carries the
	// set that applies after expiry, there is nowhere to read it from — and absent
	// must be rendered as unknown, never as a copy of Current.
	Next *CapabilitySet `json:"next,omitempty"`
}

type CapabilitySet struct {
	Preset    string              `json:"preset"`
	ProfileID string              `json:"profile_id,omitempty"`
	Limits    capability.Limits   `json:"limits"`
	Features  capability.Features `json:"features"`
	// Reason is set on Next only, from the constants above.
	Reason string `json:"reason,omitempty"`
}

type TelemetryUploadResponse struct {
	Accepted int `json:"accepted"`
}
