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
