package protocol

// Codes of the warnings an Edge lists on its own panel and reports in EdgeRuntime.Warnings
// (contract v0.10.0). Each names one line of that list; the line's text travels next to it
// as RuntimeWarning.Message and carries the numbers.
//
// The list is closed only on the writer's side. Edge sends one of these or WarningOther;
// a reader must show an unknown code by its message rather than drop it.
const (
	// The venue.
	WarningDiscoveryNotPublished = "discovery_not_published" // phones cannot find Edge by name
	WarningEdgeNotLinked         = "edge_not_linked"         // phones refused until it is linked
	WarningEdgeRevoked           = "edge_revoked"            // blocked by the owner; recovery in Console

	// Terms. The four ways the paid set stops applying, told apart because the cures differ.
	WarningPaidSetEnded        = "paid_set_ended"         // right expired
	WarningPaidSetSuspended    = "paid_set_suspended"     // offline window ran out: bring the venue online
	WarningPaidSetWithdrawn    = "paid_set_withdrawn"     // Cloud stopped issuing it
	WarningPaidSetEndedOffline = "paid_set_ended_offline" // both at once
	WarningCameraLimitReached  = "camera_limit_reached"
	WarningSessionTimeLimit    = "session_time_limit"  // the session limit is reached
	WarningSessionEndingSoon   = "session_ending_soon" // minutes left on the session limit

	// Cloud.
	WarningNegotiationRefused   = "negotiation_refused"   // no agreed key or schema: no new terms arrive
	WarningCloudNeverAccepted   = "cloud_never_accepted"  // this installation was never accepted
	WarningCloudSyncStale       = "cloud_sync_stale"      // no sync for longer than the fresh cache lasts
	WarningCapabilityDegraded   = "capability_degraded"   // cache degraded: reduced limits until sync
	WarningCapabilityExpired    = "capability_expired"    // cache expired: fallback limits
	WarningCapabilityInvalid    = "capability_invalid"    // cache invalid: safe fallback limits
	WarningCapabilityUnreadable = "capability_unreadable" // a profile schema this build cannot read
	WarningCapabilityMissing    = "capability_missing"    // registered but no capability cache

	// Cameras and the air.
	WarningSRTLatencyMismatch = "srt_latency_mismatch" // a camera runs on a delay nobody assigned
	WarningStartSpikeCured    = "start_spike_cured"    // Edge reconnected a camera that started late
	WarningStartSpikeUncured  = "start_spike_uncured"  // …and could not do it again: operator acts
	WarningPacketLossPark     = "packet_loss_park"     // every camera losing: look outside the system
	WarningPacketLossCamera   = "packet_loss_camera"   // one camera losing: its bitrate

	// WarningOther is a line this contract has no code for yet. Show its message.
	WarningOther = "other"
)

// Keys of RuntimeWarning.Params. Values are strings in one fixed form each, so a reader in
// any language formats them itself:
//   - counts and milliseconds: a decimal integer ("8", "195");
//   - durations: whole seconds ("259200"), the reader picks hours or days;
//   - percentages: one decimal with a dot ("12.5");
//   - moments: RFC 3339 in UTC ("2026-09-29T06:20:16Z");
//   - camera: the name the Edge panel shows for it (slot label, nickname, or short id) —
//     free text, shown as is.
const (
	ParamCamera       = "camera"
	ParamCameras      = "cameras"        // the camera limit now in force
	ParamRunningMs    = "running_ms"     // SRT latency the camera actually runs on
	ParamAssignedMs   = "assigned_ms"    // SRT latency the operator assigned
	ParamAt           = "at"             // when Edge acted
	ParamEarlyRTTMs   = "early_rtt_ms"   // RTT while the connection opened
	ParamSettledRTTMs = "settled_rtt_ms" // RTT once it settled
	ParamLateMs       = "late_ms"        // how late the camera runs (or would have run)
	ParamLossPct      = "loss_pct"       // packet loss of the named camera
	ParamOthersMaxPct = "others_max_pct" // the worst loss among the other cameras
	ParamOfflineSec   = "offline_sec"    // time without Cloud
	ParamSinceSyncSec = "since_sync_sec" // time since the last successful sync
	ParamFreshSec     = "fresh_sec"      // how long a fresh terms cache lasts
	ParamLimitMin     = "limit_min"      // the session length limit
	ParamMinutesLeft  = "minutes_left"   // minutes left before that limit
	ParamLastError    = "last_error"     // Cloud's last answer, verbatim
)

// WarningParamSpec is what one code carries: Required keys are always sent with it,
// Optional ones only when they exist (the second camera to compare with, an error text).
type WarningParamSpec struct {
	Required []string
	Optional []string
}

var warningParams = map[string]WarningParamSpec{
	WarningPaidSetEnded:        {Required: []string{ParamCameras}},
	WarningPaidSetSuspended:    {Required: []string{ParamOfflineSec, ParamCameras}},
	WarningPaidSetWithdrawn:    {Required: []string{ParamCameras}},
	WarningPaidSetEndedOffline: {Required: []string{ParamCameras}},
	WarningCameraLimitReached:  {Required: []string{ParamCameras}},
	WarningSessionTimeLimit:    {Required: []string{ParamLimitMin}},
	WarningSessionEndingSoon:   {Required: []string{ParamMinutesLeft}},

	WarningNegotiationRefused:   {Required: []string{ParamCameras}},
	WarningCloudNeverAccepted:   {Required: []string{ParamCameras}, Optional: []string{ParamLastError}},
	WarningCloudSyncStale:       {Required: []string{ParamSinceSyncSec, ParamFreshSec, ParamCameras}},
	WarningCapabilityDegraded:   {Required: []string{ParamCameras}},
	WarningCapabilityExpired:    {Required: []string{ParamCameras}},
	WarningCapabilityInvalid:    {Required: []string{ParamCameras}},
	WarningCapabilityUnreadable: {Required: []string{ParamCameras}},

	WarningSRTLatencyMismatch: {Required: []string{ParamCamera, ParamRunningMs, ParamAssignedMs}},
	WarningStartSpikeCured:    {Required: []string{ParamCamera, ParamAt, ParamEarlyRTTMs, ParamSettledRTTMs, ParamLateMs}},
	WarningStartSpikeUncured:  {Required: []string{ParamCamera, ParamEarlyRTTMs, ParamSettledRTTMs, ParamLateMs}},
	// The park verdict names the worst camera.
	WarningPacketLossPark: {Required: []string{ParamCamera, ParamLossPct}},
	// others_max_pct is absent when there is no other camera to compare with.
	WarningPacketLossCamera: {Required: []string{ParamCamera, ParamLossPct}, Optional: []string{ParamOthersMaxPct}},
}

// WarningParamSpecFor returns the params a code carries. A code with no entry — one whose
// sentence has no values in it, WarningOther, or a code this contract does not know —
// carries none, and a reader shows its message.
func WarningParamSpecFor(code string) (WarningParamSpec, bool) {
	spec, ok := warningParams[code]
	if !ok {
		return WarningParamSpec{}, false
	}
	return WarningParamSpec{
		Required: append([]string(nil), spec.Required...),
		Optional: append([]string(nil), spec.Optional...),
	}, true
}
