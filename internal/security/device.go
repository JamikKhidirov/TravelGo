package security

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// KnownPlatforms is the allowlist for X-Device-Platform. Unknown values are
// mapped to "web" so a client cannot inject arbitrary labels into the audit log.
var KnownPlatforms = map[string]struct{}{
	"web":     {},
	"ios":     {},
	"android": {},
	"desktop": {},
	"api":     {},
}

func NormalizePlatform(platform string) string {
	p := strings.ToLower(strings.TrimSpace(platform))
	if _, ok := KnownPlatforms[p]; ok {
		return p
	}
	return "web"
}

// DeviceFingerprint is a stable, non-reversible identifier for a device. It is
// built from the human-readable name, platform and User-Agent, so re-logins from
// the same browser/phone are grouped into one device record.
func DeviceFingerprint(name, platform, userAgent string) string {
	raw := strings.ToLower(strings.TrimSpace(name)) + "\x00" +
		strings.ToLower(strings.TrimSpace(platform)) + "\x00" +
		strings.ToLower(strings.TrimSpace(userAgent))
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}
