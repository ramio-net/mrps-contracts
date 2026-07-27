package capability

import "time"

type Preset string

const (
	PresetDemo       Preset = "demo"
	PresetRegistered Preset = "registered"
	PresetFull       Preset = "full"
)

const (
	SourceDemoHardcoded        = "demo_hardcoded"
	SourceRegisteredPlatform   = "registered_platform_template"
	SourceEntitlementSub       = "entitlement_subscription"
	SourceEntitlementStream    = "entitlement_stream_pass"
	SourceManualGrant          = "manual_grant"
	SourceRegisteredOffline    = "registered_offline_limited"
	SourceCapabilityCacheStale = "capability_cache_degraded"
)

type Limits struct {
	MaxCameras                int  `json:"max_cameras"`
	MaxSessionDurationMinutes *int `json:"max_session_duration_minutes,omitempty"`
	ConfigEditable            bool `json:"config_editable"`
}

type Features struct {
	MultiStream         bool `json:"multi_stream"`
	StudioNodeTransport bool `json:"studio_node_transport"`
	EdgeLocalConfigUI   bool `json:"edge_local_config_ui"`
	MRPSConsoleSync     bool `json:"mrps_console_sync"`
	RemoteControl       bool `json:"remote_control"`
	TelemetryUpload     bool `json:"telemetry_upload"`
	OperationalProfile  bool `json:"operational_profile"`
}

// Subject binds a signed profile to one concrete Edge installation. Demo profiles
// are unsigned and may omit it; Cloud-issued profiles must include it.
type Subject struct {
	InstallationID string `json:"installation_id"`
	EdgeID         string `json:"edge_id"`
	OrgID          string `json:"org_id"`
}

type Profile struct {
	SchemaVersion int        `json:"schema_version"`
	ProfileID     string     `json:"profile_id"`
	Preset        Preset     `json:"preset"`
	Source        string     `json:"source"`
	Subject       *Subject   `json:"subject,omitempty"`
	Limits        Limits     `json:"limits"`
	Features      Features   `json:"features"`
	IssuedAt      time.Time  `json:"issued_at"`
	ValidUntil    *time.Time `json:"valid_until,omitempty"`
	KeyID         string     `json:"key_id,omitempty"`
	Signature     string     `json:"signature,omitempty"`
}

func IntPtr(v int) *int { return &v }

func DemoProfile(now time.Time) Profile {
	valid := now.UTC().Add(20 * time.Minute)
	return Profile{
		SchemaVersion: 2,
		Preset:        PresetDemo,
		Source:        SourceDemoHardcoded,
		IssuedAt:      now.UTC(),
		ValidUntil:    &valid,
		Limits: Limits{
			MaxCameras:                3,
			MaxSessionDurationMinutes: IntPtr(20),
			ConfigEditable:            false,
		},
	}
}

func RegisteredPlatformTemplate(now time.Time, subject Subject) Profile {
	valid := now.UTC().Add(30 * 24 * time.Hour)
	return Profile{
		SchemaVersion: 2,
		ProfileID:     "registered-platform-v1",
		Preset:        PresetRegistered,
		Source:        SourceRegisteredPlatform,
		Subject:       &subject,
		IssuedAt:      now.UTC(),
		ValidUntil:    &valid,
		Limits: Limits{
			MaxCameras:                4,
			MaxSessionDurationMinutes: IntPtr(60),
			ConfigEditable:            true,
		},
		Features: Features{
			EdgeLocalConfigUI:  true,
			MRPSConsoleSync:    true,
			TelemetryUpload:    true,
			OperationalProfile: true,
		},
	}
}

func FullProfile(now time.Time, subject Subject, validUntil time.Time) Profile {
	return Profile{
		SchemaVersion: 2,
		ProfileID:     "full-entitlement-v1",
		Preset:        PresetFull,
		Source:        SourceEntitlementSub,
		Subject:       &subject,
		IssuedAt:      now.UTC(),
		ValidUntil:    timePtr(validUntil.UTC()),
		Limits: Limits{
			MaxCameras:                8,
			MaxSessionDurationMinutes: nil,
			ConfigEditable:            true,
		},
		Features: Features{
			MultiStream:         true,
			StudioNodeTransport: true,
			EdgeLocalConfigUI:   true,
			MRPSConsoleSync:     true,
			RemoteControl:       true,
			TelemetryUpload:     true,
			OperationalProfile:  true,
		},
	}
}

func timePtr(v time.Time) *time.Time { return &v }
