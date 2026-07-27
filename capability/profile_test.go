package capability

import (
	"strings"
	"testing"
	"time"
)

func testSubject() Subject {
	return Subject{
		InstallationID: "018f60f2-5df5-7a0b-b4d0-000000000001",
		EdgeID:         "018f60f2-5df5-7a0b-b4d0-000000000002",
		OrgID:          "018f60f2-5df5-7a0b-b4d0-000000000003",
	}
}

func TestCanonicalBytesSortsKeysAndClearsSignature(t *testing.T) {
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	p := RegisteredPlatformTemplate(now, testSubject())
	p.ProfileID = "кириллица"
	p.Signature = "must-not-be-signed"
	got, err := CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, "must-not-be-signed") {
		t.Fatalf("signature leaked into canonical payload: %s", s)
	}
	if !strings.HasPrefix(s, `{"features":`) {
		t.Fatalf("canonical object keys are not sorted: %s", s)
	}
	if !strings.Contains(s, `"profile_id":"кириллица"`) {
		t.Fatalf("unicode string was not preserved: %s", s)
	}
}

func TestSignVerifyAndTamper(t *testing.T) {
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	p := RegisteredPlatformTemplate(now, testSubject())
	if err := Sign(&p, DevKeyID, DevPrivateKey()); err != nil {
		t.Fatal(err)
	}
	if err := Verify(p, DevKeySet()); err != nil {
		t.Fatalf("verify signed profile: %v", err)
	}
	p.Limits.MaxCameras = 99
	if err := Verify(p, DevKeySet()); err == nil {
		t.Fatal("tampered profile verified successfully")
	}
}

func TestVerifyForSubject(t *testing.T) {
	now := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	subject := testSubject()
	p := RegisteredPlatformTemplate(now, subject)
	if err := Sign(&p, DevKeyID, DevPrivateKey()); err != nil {
		t.Fatal(err)
	}
	if err := VerifyForSubject(p, DevKeySet(), subject); err != nil {
		t.Fatalf("verify subject: %v", err)
	}

	other := subject
	other.InstallationID = "018f60f2-5df5-7a0b-b4d0-999999999999"
	if err := VerifyForSubject(p, DevKeySet(), other); err == nil {
		t.Fatal("profile verified for a different installation")
	}
}
