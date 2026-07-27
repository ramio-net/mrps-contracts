package protocol

import (
	"encoding/json"
	"time"
)

const ContractVersion = "0.1.0"

type Envelope struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Ts        time.Time       `json:"ts"`
	Payload   json.RawMessage `json:"payload"`
	ReplyToID string          `json:"reply_to_id,omitempty"`
}

type Hello struct {
	EdgeVersion       string   `json:"edge_version"`
	ContractVersion   string   `json:"contract_version"`
	FeaturesSupported []string `json:"features_supported"`
	InstallationID    string   `json:"installation_id"`
}

type HelloAck struct {
	Status               string    `json:"status"`
	ServerTime           time.Time `json:"server_time"`
	ContractVersion      string    `json:"contract_version"`
	HeartbeatIntervalSec int       `json:"heartbeat_interval_sec"`
	TelemetryEnabled     bool      `json:"telemetry_enabled"`
	CommandsEnabled      bool      `json:"commands_enabled"`
}

type Heartbeat struct {
	InstallationID string `json:"installation_id"`
	EdgeID         string `json:"edge_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	State          string `json:"state"`
}

type TelemetryBatch struct {
	SessionID string          `json:"session_id"`
	Seq       int64           `json:"seq"`
	Snapshots json.RawMessage `json:"snapshots"`
}

type Command struct {
	CommandID string          `json:"command_id"`
	Type      string          `json:"type"`
	Params    json.RawMessage `json:"params"`
}

type CommandResult struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}
