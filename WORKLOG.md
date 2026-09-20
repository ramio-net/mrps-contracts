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
