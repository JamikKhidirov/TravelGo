package security

import "testing"

func TestNormalizePlatform(t *testing.T) {
	cases := map[string]string{
		"web":      "web",
		" iOS ":    "ios",
		"ANDROID":  "android",
		"desktop":  "desktop",
		"api":      "api",
		"":         "web",
		"tv":       "web",
		"<script>": "web",
	}
	for input, want := range cases {
		if got := NormalizePlatform(input); got != want {
			t.Fatalf("NormalizePlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestDeviceFingerprintIsStableAndCaseInsensitive(t *testing.T) {
	a := DeviceFingerprint("Chrome on Windows", "web", "Mozilla/5.0")
	b := DeviceFingerprint(" chrome on windows ", "WEB", "mozilla/5.0")
	if a != b {
		t.Fatalf("fingerprint must be stable for the same device: %q != %q", a, b)
	}
	if len(a) != 32 {
		t.Fatalf("fingerprint length = %d, want 32 hex chars", len(a))
	}
	c := DeviceFingerprint("Chrome on Windows", "web", "Mozilla/6.0")
	if a == c {
		t.Fatal("different user agents must produce different fingerprints")
	}
}
