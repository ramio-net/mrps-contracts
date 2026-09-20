# MRPS Contracts Worklog

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
  CI and Cloud candidate rehearsal follow before requesting final review.
