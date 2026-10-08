# MRPS Contracts Worklog

## 2026-10-09: v0.10.2 — config_changed_on_site (additive)

- Branch feat/v0.10.2-config-changed-on-site from main 05b9a06. Only additions: one code,
  four keys, one value form (lists). A reader on v0.10.1 shows the new code by its message.
- Owner's decision of 09.10: on a venue linked to Cloud, the person in the room may change
  a setting Cloud delivered, and it stays until Cloud sends a NEW version — Edge now
  applies each version once instead of re-applying it on every sync, which turned every
  panel knob back within ten seconds. The applied echo stays truthful (that version did
  take effect); config_changed_on_site {settings, config_version, playout_ms?,
  cloud_playout_ms?} says what has been changed on the Edge since, so Console can stop
  saying "confirmed" over values the venue no longer runs.
- settings is a list of sessioncfg.Config JSON paths ("operational.playout_delay_ms").
  playout_ms and cloud_playout_ms travel together, only when the buffer is among them.

## 2026-10-07: v0.10.1 — three warning codes from the broadcast of 07.10 (additive)

- Branch feat/v0.10.1-field-lessons from main 70b6332. Only additions: new codes and keys,
  one Optional key on an existing code. A reader on v0.10.0 shows the new codes by their
  message, as the contract already requires for unknown codes.
- calib_disturbed_cured {camera, at, calib_rtt_ms, floor_ms, error_ms} / _uncured (no at) {camera, calib_rtt_ms, floor_ms,
  error_ms}: a camera's clock measured while the phone was busy. 05.10 (after a call):
  RTT 553 against a floor of 240, the camera ran ~145 ms early with 10-20% repeated
  frames; 07.10 on air: RTT 367, ~57 ms early for 36 minutes, nothing on the panel. Edge
  now re-measures by reconnecting the camera before it goes on air, once in 5 minutes;
  the second time the operator presses Recalibrate.
- phone_drops_camera {camera, drop_pct, others_max_pct?}: the phone discards frames from
  its send queue — invisible as packet loss. 07.10: the handheld dropped up to 1100
  frames a minute with zero SRT loss while the loss verdict blamed the wide shot alone.
- packet_loss_park gains Optional drop_pct: the worst camera of a park-wide fault may be
  losing frames on the phone rather than packets on the way.

## 2026-09-29: v0.10.0 proposal — what the Console review found missing (Б)

- From the owner's Console review of 28.09 and the dress rehearsal the same evening.
  Branch feat/edge-runtime-b from main 0bf1e88. Proposal only: no tag until Cloud has
  reviewed it, and Edge does not adopt it before the broadcast of 7.10 (the broadcast
  build stays on v0.9.0).
- EdgeRuntime gains Transport, MaxCamerasInForce (the ceiling a broadcast keeps when
  terms drop mid-show — Cloud cannot derive it), SessionElapsedSec (computed on Edge, no
  two clocks compared) and Warnings ([]RuntimeWarning{Code, Message}, NOT omitempty: []
  means "nothing to say", absent means "Edge too old to report"). Codes in
  protocol/warnings.go, one per line of the Edge panel's list; unknown codes are shown
  by their message, never dropped.
- ClaimRequest and SyncRequest gain Hostname and EdgeKind (software/hardware);
  SyncResponse gains EdgeName (the name the owner gave in Console, for the Edge panel).
- TrustState constants, with TrustStateReleased: the owner UNLINKED the Edge — free to be
  linked again by any account with no recovery grant — as opposed to revoked (BLOCKED,
  only its owner brings it back). Cloud keeps answering the old secret on sync until it
  has delivered the state, as it does for revoked.
- sessioncfg.RecommendedPlayoutMs: floor 280 → 400. Every config now starts at 400, the
  margin confirmed 29.08, on 06.09 and in the rehearsal of 28.09; a venue linked afresh
  was getting 280.
- All new fields are optional/additive; old requests keep their exact shape (tested).
  go vet, go test ./... -race pass.
- Cloud's review (PR #8 comment, 29.09) — all four answers taken as proposed:
  RuntimeWarning.Params (map[string]string, optional) with the keys of every code in
  protocol/warnings.go (WarningParamSpecFor, required/optional, one fixed value form per
  kind); Console localizes a known code with all required params and shows Message
  otherwise. SyncRequest.SupportedFeatures with FeatureReleasedV1 — support for "released"
  is declared on the signed sync, not inferred from EdgeKind. The old secret after
  unlinking serves the signed sync only and is retired when the NEXT link completes, not
  on the unauthenticated claim request. Hostname names an Edge only at confirm and never
  renames one the owner named. New tests: a wire vector for Console, pre-params decoding,
  the spec table (mutation-checked: an undocumented key, a duplicate key, a shared slice
  are each caught).

## 2026-09-20: schema-2 signing-window follow-up

- Edge and Cloud agreed to add a constrained schema-2 verifier, keeping signed
  bytes and old APIs unchanged. Cloud core PR #23 was accepted and merged as
  346b6bd; H2 HTTP activation is separate work.
- Branch feat/schema2-window-verification starts from main 470ecfd (v0.8.0).
  This is not a change to the existing schema-3 branch or a release/tag.
- Plan and acceptance are in spec/SCHEMA2_WINDOW_VERIFICATION.md. Implementation
  and executable evidence follow in this PR. Real public/private keys are not
  installed, embedded or tested here; only ephemeral/dedicated test material.
- Draft PR #6 opened in the day of branch creation:
  https://github.com/ramio-net/mrps-contracts/pull/6
- Implemented VerifyV2ForSubject using existing mayIssue and VerifyForSubject.
  No wire/codec/legacy API changes. TrustedKeys comments now also describe
  consumer-provisioned embedded trust instead of assuming manifest-only delivery.
- Thirty stage/reason vectors, input-immutability checks, honestly re-signed
  constraint cases and legacy positive controls pass. The frozen v0.6.0 profile
  still verifies and reproduces the same signature. Withdrawal and no automatic
  dev/demo trust have separate controls.
- Local go vet ./..., go build ./... and go test ./... -race -count=1 passed.
  TestVerifyV2 passed ten repetitions. Implementation f41c51a passed CI:
  https://github.com/ramio-net/mrps-contracts/actions/runs/35524996012
- Cloud main 346b6bd passed go test ./cmd/... ./internal/... ./migrations -count=1
  and vet against this candidate using an ignored, isolated modfile/replace.
  Database integration was disabled for this candidate rehearsal. Cloud's real
  go.mod/go.sum stayed unchanged on v0.8.0; no branch pin or module-cache patch.
- Cloud's independent JS verifier preserved all historical canonical/signing
  bytes: 9 positive and 23 negative vectors, including duplicate-key, exponent
  and surrogate controls. Existing schema2/schema3 fixtures and codecs have no diff.
- The first broad Cloud go test ./... included the customer's read-only reference
  go/ directory and failed on its Edge-only imports. It was not edited or supplied
  new dependencies; the successful rerun used exactly Cloud CI's product packages.
- No tag/merge, production keys, Cloud deployment or Edge installation was changed.
  Review and then a released tag are required before consumer adoption.
