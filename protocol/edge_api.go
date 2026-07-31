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
}

type SyncResponse struct {
	CapabilityProfile capability.Profile `json:"capability_profile"`
	// Nil means Cloud has no production session assigned to this Edge, and that is
	// encoded as an explicit null rather than dropped: "no session" is an answer Edge
	// acts on — it keeps its local config — not a missing value. With omitempty the
	// two were indistinguishable on the wire, which forced Cloud to mirror this
	// struct locally just to say null.
	SessionConfig    *sessioncfg.Config `json:"session_config"`
	ServerTime       time.Time          `json:"server_time"`
	NextSyncAfterSec int                `json:"next_sync_after_sec"`
	TrustState       string             `json:"trust_state,omitempty"`
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
