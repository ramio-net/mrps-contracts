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
	InstallationID string                        `json:"installation_id"`
	EdgeID         string                        `json:"edge_id"`
	SessionID      string                        `json:"session_id"`
	Seq            int64                         `json:"seq"`
	Snapshots      []health.DeviceHealthSnapshot `json:"snapshots"`
}

type TelemetryUploadResponse struct {
	Accepted int `json:"accepted"`
}
