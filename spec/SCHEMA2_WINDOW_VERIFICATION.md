# Schema-2 verification with key constraints

Status: agreed by Edge/Cloud, implementation in progress (2026-09-20).
Scope: an additive verifier, not a wire change, key rollout or format retirement.

## Agreement

Schemas 2 and 3 use the same consumer-provisioned TrustedKeys, including public
material, AllowedSchemaVersions, SigningFrom and SigningUntil. Edge embeds that
set in its binary; this change introduces no remote key delivery or dev fallback.
The schema-2 adapter must not flatten the complete set to KeySet before checking
constraints. Old Verify/VerifyForSubject entry points remain source-compatible.

## API

```go
func VerifyV2ForSubject(p Profile, keys TrustedKeys, expected Subject) error
```

The new entry point requires schema_version=2, a signature, key_id and nonzero
issued_at. It selects only the named trusted key, applies TrustedKey.mayIssue(2,
issued_at), and verifies the original schema-2 signature and expected subject.
Wrong-length public material is an error, never an Ed25519 panic. An empty or
unknown key set authorizes nothing; another matching public key is not inferred.

Signing windows have inclusive endpoints and compare the SIGNED issued_at,
not the verifier's clock. A document issued inside the window remains verifiable
after retirement. ValidUntil/full expiry are separate consumer policy checks;
this function does not grant an entitlement, check current subscription status
or impose a new wall-clock deadline on registered documents.

On error the consumer must not apply ANY part of p or retry with bare KeySet.
The function returns only an error, not a partially verified document. Unsigned
demo handling stays outside this verifier in the already agreed Edge state flow.

## Compatibility and security limits

No change to Profile fields, Sign, CanonicalBytes, existing signatures or golden
fixtures. The historical v0.6.0 profile must pass through both old and new APIs
when trusted for schema 2 within its window. No legacy API removal or automatic
consumer migration. Consumers must adopt the new function after a reviewed tag.

Window checking is not compromise revocation: possession of the private key
allows backdating. A compromised key must be removed from active trust by an
updated Edge binary. Existing/offline binaries retain their previous trust.
Old uncompromised public keys can remain verify-only for historic documents.

## Acceptance

Executable Go vectors report stage=verify_profile, a named reason and accepted.
Each negative checks the actual returned reason and leaves inputs unchanged:

- Historical in-window document and exact start/end boundaries are accepted.
- Just before start / after end are refused, with old-API positive controls.
- Empty/wrong schema permission, unknown/missing key, empty trust, malformed
  public-key length, wrong key material, absent/invalid signature, zero issued_at
  and wrong schema version are refused without panic.
- Missing/mismatched installation, edge or organization subject are refused.
- Withdrawal rejects an old document; keeping the old set still verifies it,
  documenting the residual risk instead of claiming remote revocation.
- Re-signed vectors exercise constraints, not accidentally bad signatures;
  tampering with signed issued_at remains an invalid-signature failure.
- Existing schema-2 and schema-3 canonical/signing fixtures stay unchanged.

Run go vet ./..., go build ./..., go test ./... -race -count=1 and the golden
suite. Cloud should also rehearse its existing tests against the candidate with
an isolated modfile, without changing its released dependency pin. A contract
PR does not by itself prove Edge adoption, live H2 negotiation or production
key custody. No production private key belongs in tests, CI or this repository.
