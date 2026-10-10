package protocol

import "strconv"

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

	// v0.10.1, from the broadcast of 07.10.2026.
	//
	// A camera's clock is measured once per connection; measured while the phone was busy
	// (just back from a call, the app restored) it carries that error for the whole
	// connection, and the camera runs early or late against the others with every counter
	// green. Edge re-measures by reconnecting the camera before it goes on air, once; the
	// second time it is the operator's Recalibrate.
	WarningCalibDisturbedCured   = "calib_disturbed_cured"   // Edge re-measured a camera's clock
	WarningCalibDisturbedUncured = "calib_disturbed_uncured" // …and could not do it again: operator acts
	// The phone discards frames before they reach the network: its send queue does not
	// drain. Invisible as packet loss, and the cure is the same as for a losing camera — its
	// own bitrate, or getting it closer to the router.
	WarningPhoneDropsCamera = "phone_drops_camera"

	// v0.10.2, owner's decision of 09.10.2026: on a venue linked to Cloud, the person in the
	// room may change a setting Cloud delivered, and the change stays until Cloud sends a
	// NEW version — Edge applies each version once, not on every sync. The version Edge
	// echoes as applied is still the one that took effect; this line says what has been
	// changed on the Edge since, so Console can say "running X · changed on Edge" instead of
	// "confirmed" over values the venue is no longer running.
	WarningConfigChangedOnSite = "config_changed_on_site"

	// v0.10.3, the Solo preset (one camera on the schedule, no lag bound): a slow phone does
	// not freeze on a short buffer, it runs steadily behind its place in the schedule. The
	// delay to vMix is then longer than the preset promises and outside audio drifts off the
	// lips, with every freeze counter at zero. Named with the buffer that would cure it.
	WarningLoneCameraLagging = "lone_camera_lagging"

	// v0.10.5, MRPS Camera's automatic bitrate (plan of 09.10.2026): the phone already cut its
	// own bitrate to the floor and its send queue still grows. Past this point lowering the
	// bitrate cures nothing more — the cure is the phone's place (closer to the router), and
	// "lower this camera's bitrate" (phone_drops_camera) would send the operator to a knob the
	// automatic has already turned all the way.
	WarningAutoBitrateAtFloor = "auto_bitrate_at_floor"

	// v0.10.6, live run of 10.10.2026: Cloud delivered a new video (720p → 1080p) in the middle
	// of a broadcast, Edge put it in force at once, and the next camera to reconnect was refused
	// as a mismatch — the phone still had the old video — and stayed off air until MRPS Camera
	// was restarted, after which the park ran 1080p and 720p side by side in vMix. Edge now keeps
	// the broadcast's video from its first camera to its end, wherever a new one comes from
	// (Cloud or the Edge's own panel); the new one takes effect with the next broadcast. This
	// line says so, so neither the Edge page nor Console shows the new video as running.
	WarningVideoChangeDeferred = "video_change_deferred"
	// A phone came with a video other than the one this Edge runs and was refused. Between
	// broadcasts it is a phone whose app kept yesterday's settings; MRPS Camera before 0.1.16
	// does not take the video Edge sends back with the refusal, and stays out until it is
	// restarted — which nothing else on any screen would say.
	WarningCameraVideoRefused = "camera_video_refused"

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
//     free text, shown as is;
//   - lists: comma-separated, no spaces ("operational.frame_sync,operational.live_bitrate.enabled");
//   - video: width x height @ fps / kbps, no spaces ("1920x1080@30/5000") — FormatVideoParam (v0.10.6).
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
	ParamCalibRTTMs   = "calib_rtt_ms"   // round trip of the camera's clock measurement (v0.10.1)
	ParamFloorMs      = "floor_ms"       // what an undisturbed measurement takes (v0.10.1)
	ParamErrorMs      = "error_ms"       // up to how far the camera's clock may be off (v0.10.1)
	ParamDropPct      = "drop_pct"       // share of frames the phone discarded before sending (v0.10.1)
	// The settings changed on the Edge since Cloud's version took effect, as a list of their
	// paths in sessioncfg.Config's JSON ("operational.playout_delay_ms") (v0.10.2).
	ParamSettings       = "settings"
	ParamConfigVersion  = "config_version"   // the Cloud config version they were changed from (v0.10.2)
	ParamPlayoutMs      = "playout_ms"       // the buffer the Edge runs on now (v0.10.2)
	ParamCloudPlayoutMs = "cloud_playout_ms" // the buffer that version put in force (v0.10.2)
	// lone_camera_lagging (v0.10.3); the buffer itself travels as playout_ms.
	ParamLagMs              = "lag_ms"               // how far behind its place in the schedule the camera runs
	ParamSuggestedPlayoutMs = "suggested_playout_ms" // the buffer that would cover the lag
	ParamToVMixMs           = "to_vmix_ms"           // the delay to vMix now, with the lag in it
	// auto_bitrate_at_floor (v0.10.5).
	ParamBitrateKbps = "bitrate_kbps" // the bitrate the phone holds now: the automatic's floor
	ParamBacklogMs   = "backlog_ms"   // the phone's send queue, still growing at the floor
	// video_change_deferred and camera_video_refused (v0.10.6).
	ParamVideo       = "video"        // the video this Edge runs and asks of every camera now
	ParamNextVideo   = "next_video"   // the video that takes effect when the broadcast ends
	ParamCameraVideo = "camera_video" // the video the refused phone came with
)

// FormatVideoParam writes a video in the one form the video keys carry ("1920x1080@30/5000").
func FormatVideoParam(width, height, fps, bitrateKbps int) string {
	return strconv.Itoa(width) + "x" + strconv.Itoa(height) + "@" + strconv.Itoa(fps) + "/" + strconv.Itoa(bitrateKbps)
}

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
	// The park verdict names the worst camera; drop_pct (v0.10.1) when the worst one's loss
	// is its phone discarding frames rather than packets lost on the way.
	WarningPacketLossPark: {Required: []string{ParamCamera, ParamLossPct}, Optional: []string{ParamDropPct}},
	// others_max_pct is absent when there is no other camera to compare with.
	WarningPacketLossCamera: {Required: []string{ParamCamera, ParamLossPct}, Optional: []string{ParamOthersMaxPct}},

	WarningCalibDisturbedCured:   {Required: []string{ParamCamera, ParamAt, ParamCalibRTTMs, ParamFloorMs, ParamErrorMs}},
	WarningCalibDisturbedUncured: {Required: []string{ParamCamera, ParamCalibRTTMs, ParamFloorMs, ParamErrorMs}},
	// others_max_pct: the worst loss of either kind among the other cameras, absent alone.
	WarningPhoneDropsCamera: {Required: []string{ParamCamera, ParamDropPct}, Optional: []string{ParamOthersMaxPct}},
	// playout_ms and cloud_playout_ms travel together, and only when the buffer is among
	// the settings changed.
	WarningConfigChangedOnSite: {Required: []string{ParamSettings, ParamConfigVersion},
		Optional: []string{ParamPlayoutMs, ParamCloudPlayoutMs}},
	// to_vmix_ms only where Edge knows the output SRT latency it adds.
	WarningLoneCameraLagging: {Required: []string{ParamCamera, ParamLagMs, ParamPlayoutMs, ParamSuggestedPlayoutMs},
		Optional: []string{ParamToVMixMs}},
	// drop_pct when the phone also discards frames at the floor.
	WarningAutoBitrateAtFloor: {Required: []string{ParamCamera, ParamBitrateKbps, ParamBacklogMs},
		Optional: []string{ParamDropPct}},
	// config_version when the new video came from Cloud; absent when it was set on the Edge.
	WarningVideoChangeDeferred: {Required: []string{ParamVideo, ParamNextVideo}, Optional: []string{ParamConfigVersion}},
	// camera: the phone's own name from its JOIN, or its model — a refused phone has no slot.
	WarningCameraVideoRefused: {Required: []string{ParamCamera, ParamCameraVideo, ParamVideo}},
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
