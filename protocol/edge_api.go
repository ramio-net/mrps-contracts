package protocol

import (
	"encoding/json"
	"time"

	"github.com/ramio-net/mrps-contracts/capability"
	"github.com/ramio-net/mrps-contracts/health"
	"github.com/ramio-net/mrps-contracts/sessioncfg"
)

type ClaimRequest struct {
	InstallationID string `json:"installation_id"`
	EdgeVersion    string `json:"edge_version"`
	OS             string `json:"os"`
	// Hostname is the name of the machine Edge runs on. Cloud uses it as the Edge's name
	// when the owner confirms the code without naming it: a list of cards all called
	// "MRPS Edge" and told apart by an id was a finding of the owner's Console review of
	// 28.09.2026. Cloud cleans and bounds it, uses it only when the owner leaves the name at
	// its default, and falls back to a neutral name when it is empty. Optional; absent from
	// Edges older than contract v0.10.0.
	Hostname string `json:"hostname,omitempty"`
	// EdgeKind is what this Edge IS — see the EdgeKind constants. Optional; absent means
	// software, the only kind that exists today.
	EdgeKind string `json:"edge_kind,omitempty"`
}

// Kinds of Edge. The difference matters beyond the card's icon: blocking a lost laptop and
// blocking a stolen box are different decisions for an owner, and only the box has an
// identity that survives a reinstall.
const (
	EdgeKindSoftware = "software"
	EdgeKindHardware = "hardware"
)

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

	// SupportedSchemaVersions is what this build can READ, declared so Cloud never
	// issues a document the venue cannot use.
	//
	// This is a hard precondition, not a courtesy. A capability document of a newer
	// schema carries its limits somewhere an older decoder does not look, so issuing
	// one blindly does not degrade a venue gracefully — measured on this build, it
	// lands on the emergency floor of two cameras and thirty minutes, and reports the
	// fault as a bad signature. Absent means schema 2 only: an Edge too old to
	// declare anything is too old to receive anything new.
	SupportedSchemaVersions []int `json:"supported_schema_versions,omitempty"`

	// KnownKeyIDs is which signing keys this venue already trusts.
	//
	// Rotation follows acknowledgement, never a calendar. Cloud may start signing
	// with a new key only for nodes that have said they hold it; the rest keep
	// getting the previous key until they catch up. Switching the fleet because N
	// days have passed is how a rotation turns into an outage.
	KnownKeyIDs []string `json:"known_key_ids,omitempty"`

	// Hostname and EdgeKind as in ClaimRequest, repeated on every sync so a venue linked
	// before contract v0.10.0 — or moved to a renamed machine — is described without being
	// linked again. They update what Cloud knows about the machine; they never rename an Edge
	// the owner has named. Optional.
	Hostname string `json:"hostname,omitempty"`
	EdgeKind string `json:"edge_kind,omitempty"`

	// SupportedFeatures names the behaviours this Edge build implements that Cloud must not
	// assume — see the Feature constants. Absent from Edges older than contract v0.10.0.
	// It rides the SIGNED sync on purpose: ClaimRequest is not authenticated, and a feature
	// Cloud acts on must be vouched for by the Edge's own secret. Cloud ignores names it does
	// not know. Proposed by Cloud (29.09.2026) in place of inferring support from EdgeKind:
	// the kind of machine and the version of the protocol are different axes.
	SupportedFeatures []string `json:"supported_features,omitempty"`
}

// Features an Edge declares in SyncRequest.SupportedFeatures.
const (
	// FeatureReleasedV1: this Edge understands TrustStateReleased — on receiving it, it drops
	// its credentials and shows a new link code by itself. Console offers "Unlink" only for an
	// Edge whose latest signed sync carried it; to an Edge without it, unlinking would look
	// like a revoke it does not understand.
	FeatureReleasedV1 = "released_v1"
)

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
	// CapabilityDocument carries a schema 3 document as the EXACT BYTES that were
	// signed, and it is raw on purpose.
	//
	// A typed field cannot do this job. Unmarshalling into a struct and marshalling
	// again drops every field this build does not know, so the canonical form would
	// differ from what the issuer signed and verification would fail — on precisely
	// the venues furthest behind, which are the ones least able to be fixed remotely.
	// Schema 3 verification reads the received bytes; this is where they arrive.
	//
	// Absent when Cloud is answering a node that has not declared schema 3, in which
	// case CapabilityProfile above carries schema 2 exactly as before.
	CapabilityDocument json.RawMessage `json:"capability_document,omitempty"`
	// KeyManifest is the trusted key list, signed by the OFFLINE ROOT and carried as
	// the exact bytes that were signed.
	//
	// Raised by Cloud in review and they were right: a plain list of keys inside an
	// authenticated sync response is only as trustworthy as the service sending it.
	// Whoever takes the running service could then install a key of their choosing and
	// mint any entitlement — the capability signature would verify perfectly, against
	// a key the attacker put there. The manifest is signed by a root that does not
	// live in the Cloud runtime at all, so the sync channel carries it without being
	// able to forge it.
	//
	// Absent leaves the venue's current set untouched.
	KeyManifest json.RawMessage `json:"key_manifest,omitempty"`
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
	// TrustState is one of the TrustState constants. A reader keeps its credentials on a
	// value it does not know: guessing "released" or "revoked" from an unknown word would
	// throw away a link that may be perfectly good.
	TrustState string `json:"trust_state,omitempty"`
	// EdgeName is what the owner called this Edge in Console. Edge shows it on its own panel
	// ("linked as …"), so the person at the venue and the owner at Console name the same
	// machine the same way. Optional.
	EdgeName string `json:"edge_name,omitempty"`
}

// Trust states Cloud reports in SyncResponse.TrustState.
const (
	TrustStateRegistered = "registered"
	// TrustStateRevoked: the owner BLOCKED this Edge ("lost / stolen"). Only that owner can
	// bring it back, by allowing recovery in Console first.
	TrustStateRevoked = "revoked"
	// TrustStateReleased: the owner UNLINKED this Edge — the ordinary way to part with one.
	// A released Edge is free: it drops its credentials, returns to "not linked" and shows a
	// new code by itself, and any account may confirm that code with no recovery grant. The
	// same account gets its card back with the history; another account gets a new card;
	// paid rights stay with the account that holds them (they are the organization's, not
	// the Edge's).
	//
	// The old secret after unlinking (agreed with Cloud 29.09.2026): it authenticates the
	// signed sync ONLY, so the Edge can learn it was released — no telemetry, reports or
	// terms are served on it. Every such sync may answer "released" again, which survives a
	// lost response. Cloud retires the secret atomically when the NEXT link completes (a new
	// secret is issued), not on the unauthenticated claim request: otherwise anyone who knows
	// the public installation_id could kill a working secret with one request.
	//
	// Console offers unlinking only for an Edge that declared FeatureReleasedV1. An Edge
	// without it keeps its credentials on this value (see TrustState).
	TrustStateReleased = "released"
)

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

	// Everything below was added in contract v0.10.0, from the owner's Console review of
	// 28.09.2026, where the Edge screen had fields for these and nothing to fill them with.

	// Transport is the phone→Edge SRT mode this Edge runs: "file" or "live".
	Transport string `json:"transport,omitempty"`
	// MaxCamerasInForce is the camera ceiling Edge enforces right now. It may be ABOVE the
	// limit in Capability.Current: a broadcast keeps the ceiling it started under when the
	// terms drop mid-show, and only Edge knows that — Cloud cannot derive it. This is the
	// number to put after "cameras 3 of …".
	MaxCamerasInForce *int `json:"max_cameras_in_force,omitempty"`
	// SessionElapsedSec is how long the current broadcast has been running, computed on
	// Edge from Edge's own clock, so no two clocks are ever compared. Absent when idle.
	SessionElapsedSec *int `json:"session_elapsed_sec,omitempty"`
	// Warnings are what the Edge's own panel lists right now. NOT omitempty on purpose: an
	// Edge that reports them sends [] when there is nothing to say, and an absent field
	// means an Edge too old to report — which Console must show as "not reported", never as
	// "no warnings". Until v0.10.0 the Edge screen said "Warnings: not reported" forever,
	// and not one warning an operator saw on site ever reached the owner.
	Warnings []RuntimeWarning `json:"warnings"`
}

// RuntimeWarning is one line of the Edge panel's warning list.
//
// Code names it so Console can say it in its own language and pick its own cure text;
// Params carry the values that line is about (which camera, how many milliseconds, since
// when) under the stable keys WarningParamSpecFor lists for the code; Message is the panel's
// line verbatim.
//
// How a reader shows one: a known code with every required param present → its own
// template with the params filled in. Anything else — an unknown code, a missing required
// param, an Edge older than the params — → Message, whole. Message is therefore always
// sent and never dropped: a warning that vanishes because a reader was older than the
// writer is the same silent failure this field exists to end. Unknown param keys are
// ignored. Params proposed by Cloud (29.09.2026): a code picks the sentence, but the
// numbers inside it cannot be localized from the English line.
type RuntimeWarning struct {
	Code    string            `json:"code"`
	Params  map[string]string `json:"params,omitempty"`
	Message string            `json:"message"`
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
