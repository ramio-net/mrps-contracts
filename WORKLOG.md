# MRPS Contracts Worklog

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
