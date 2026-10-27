package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var b64 = base64.RawURLEncoding

// Claims is a minimal JWT payload used for access and ID tokens.
type Claims map[string]interface{}

func RandomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b64.EncodeToString(b), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func SignJWT(secret []byte, claims Claims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	h, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	p, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + b64.EncodeToString(mac.Sum(nil)), nil
}

func ParseJWT(secret []byte, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	expected := b64.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid token signature")
	}
	payload, err := b64.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	if exp, ok := claims["exp"].(float64); ok && time.Now().Unix() >= int64(exp) {
		return nil, errors.New("token expired")
	}
	if iss, ok := claims["iss"].(string); ok && iss != "gotravel" {
		return nil, errors.New("invalid issuer")
	}
	return claims, nil
}

func StringClaim(c Claims, key string) string {
	v, _ := c[key].(string)
	return v
}

func Int64Claim(c Claims, key string) int64 {
	switch v := c[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func NewJTI() string {
	t, err := RandomToken(16)
	if err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return t
}

func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return b64.EncodeToString(sum[:])
}

func VerifyPKCE(verifier, challenge string) bool {
	return challenge != "" && hmac.Equal([]byte(PKCEChallenge(verifier)), []byte(challenge))
}

func DecodeBase64URL(v string) ([]byte, error) {
	if b, err := b64.DecodeString(v); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(v)
}
