package security

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestSignAndParseJWT(t *testing.T) {
	secret := []byte("test-secret")
	claims := Claims{
		"sub":  "user-1",
		"sid":  "session-1",
		"role": "guide",
		"iss":  "gotravel",
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	token, err := SignJWT(secret, claims)
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	parsed, err := ParseJWT(secret, token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if got := StringClaim(parsed, "sub"); got != "user-1" {
		t.Fatalf("sub = %q, want user-1", got)
	}
	if got := StringClaim(parsed, "role"); got != "guide" {
		t.Fatalf("role = %q, want guide", got)
	}
	if got := StringClaim(parsed, "sid"); got != "session-1" {
		t.Fatalf("sid = %q, want session-1", got)
	}
}

func TestParseJWTRejectsWrongSecret(t *testing.T) {
	token, err := SignJWT([]byte("secret-a"), Claims{"sub": "u", "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	if _, err := ParseJWT([]byte("secret-b"), token); err == nil {
		t.Fatal("expected signature error, got nil")
	}
}

func TestParseJWTRejectsTamperedPayload(t *testing.T) {
	secret := []byte("secret")
	token, err := SignJWT(secret, Claims{"sub": "u", "role": "user", "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %q", token)
	}
	evil, _ := SignJWT(secret, Claims{"sub": "u", "role": "admin", "exp": time.Now().Add(time.Hour).Unix()})
	evilParts := strings.Split(evil, ".")
	forged := parts[0] + "." + evilParts[1] + "." + parts[2]
	if _, err := ParseJWT(secret, forged); err == nil {
		t.Fatal("expected signature error for tampered payload")
	}
}

func TestParseJWTRejectsExpired(t *testing.T) {
	secret := []byte("secret")
	token, err := SignJWT(secret, Claims{"sub": "u", "exp": time.Now().Add(-time.Minute).Unix()})
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	if _, err := ParseJWT(secret, token); err == nil {
		t.Fatal("expected expiry error")
	}
}

func TestParseJWTRejectsWrongIssuer(t *testing.T) {
	secret := []byte("secret")
	token, err := SignJWT(secret, Claims{"sub": "u", "iss": "other", "exp": time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("SignJWT: %v", err)
	}
	if _, err := ParseJWT(secret, token); err == nil {
		t.Fatal("expected issuer error")
	}
}

func TestParseJWTRejectsMalformed(t *testing.T) {
	for _, token := range []string{"", "abc", "a.b", "a.b.c.d"} {
		if _, err := ParseJWT([]byte("secret"), token); err == nil {
			t.Fatalf("expected error for %q", token)
		}
	}
}

func TestPKCE(t *testing.T) {
	// RFC 7636 appendix B verifier and challenge.
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := PKCEChallenge(verifier); got != want {
		t.Fatalf("PKCEChallenge = %q, want %q", got, want)
	}
	if !VerifyPKCE(verifier, want) {
		t.Fatal("VerifyPKCE should accept matching verifier")
	}
	if VerifyPKCE("wrong", want) {
		t.Fatal("VerifyPKCE should reject mismatched verifier")
	}
	if VerifyPKCE(verifier, "") {
		t.Fatal("VerifyPKCE should reject empty challenge")
	}
}

func TestRandomTokenAndHash(t *testing.T) {
	token, err := RandomToken(32)
	if err != nil {
		t.Fatalf("RandomToken: %v", err)
	}
	if len(token) < 40 {
		t.Fatalf("token too short: %q", token)
	}
	if strings.ContainsAny(token, "+/=") {
		t.Fatalf("token is not base64url: %q", token)
	}
	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Fatal("HashToken must be deterministic")
	}
	if h1 == token {
		t.Fatal("HashToken must not return the raw token")
	}
	if HashToken(token+"x") == h1 {
		t.Fatal("HashToken collision for different input")
	}
}

func TestTOTPKnownVector(t *testing.T) {
	// RFC 6238 SHA1 secret "12345678901234567890".
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	code, err := TOTPCode(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if code != "287082" {
		t.Fatalf("TOTPCode = %q, want 287082", code)
	}
}

func TestValidateTOTPWindow(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	code, err := TOTPCode(secret, now)
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if !ValidateTOTP(secret, code, now) {
		t.Fatal("current code should be valid")
	}
	if !ValidateTOTP(secret, code, now.Add(29*time.Second)) {
		t.Fatal("code should be valid within the same step")
	}
	prev, _ := TOTPCode(secret, now.Add(-30*time.Second))
	if !ValidateTOTP(secret, prev, now) {
		t.Fatal("previous step code should be accepted (±1 window)")
	}
	if ValidateTOTP(secret, "000000", now.Add(10*time.Minute)) {
		t.Fatal("stale code should be rejected")
	}
	if ValidateTOTP(secret, "12345", now) {
		t.Fatal("short code should be rejected")
	}
}

func TestOTPAuthURL(t *testing.T) {
	got := OTPAuthURL("GoTravel", "user@example.com", "JBSWY3DPEHPK3PXP")
	for _, want := range []string{
		"otpauth://totp/",
		"secret=JBSWY3DPEHPK3PXP",
		"issuer=GoTravel",
		"algorithm=SHA1",
		"digits=6",
		"period=30",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("OTPAuthURL missing %q: %s", want, got)
		}
	}
}

func TestPasskeyChallengeHelpers(t *testing.T) {
	challenge, err := NewChallenge()
	if err != nil {
		t.Fatalf("NewChallenge: %v", err)
	}
	clientData := `{"type":"webauthn.get","challenge":"` + challenge + `","origin":"http://localhost"}`
	encoded := base64.RawURLEncoding.EncodeToString([]byte(clientData))
	if !ChallengeValid(encoded, challenge) {
		t.Fatal("ChallengeValid should accept matching challenge")
	}
	if ChallengeValid(encoded, "other") {
		t.Fatal("ChallengeValid should reject mismatched challenge")
	}
	if ChallengeValid("not-base64!!", challenge) {
		t.Fatal("ChallengeValid should reject invalid base64")
	}
	cd, err := ParseClientData(encoded)
	if err != nil {
		t.Fatalf("ParseClientData: %v", err)
	}
	if cd.Type != "webauthn.get" {
		t.Fatalf("type = %q, want webauthn.get", cd.Type)
	}
	if _, err := ParseClientData(base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get"}`))); err == nil {
		t.Fatal("ParseClientData should reject missing challenge")
	}
}

func TestExpired(t *testing.T) {
	if !Expired(time.Now().Add(-time.Second)) {
		t.Fatal("past time should be expired")
	}
	if Expired(time.Now().Add(time.Minute)) {
		t.Fatal("future time should not be expired")
	}
}
