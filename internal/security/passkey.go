package security

import (
	"encoding/json"
	"errors"
	"time"
)

type ClientData struct {
	Type        string `json:"type"`
	Challenge   string `json:"challenge"`
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"crossOrigin"`
}

func ParseClientData(encoded string) (*ClientData, error) {
	raw, err := DecodeBase64URL(encoded)
	if err != nil {
		return nil, err
	}
	var cd ClientData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, err
	}
	if cd.Challenge == "" {
		return nil, errors.New("challenge missing")
	}
	return &cd, nil
}

func NewChallenge() (string, error) {
	return RandomToken(32)
}

func ChallengeValid(clientDataJSON, expected string) bool {
	cd, err := ParseClientData(clientDataJSON)
	return err == nil && cd.Challenge == expected
}

func Expired(t time.Time) bool {
	return time.Now().After(t)
}
