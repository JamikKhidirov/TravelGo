package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSwaggerSpecIsValidYAML(t *testing.T) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(swaggerSpec), &doc); err != nil {
		t.Fatalf("swagger spec is not valid YAML: %v", err)
	}
	info, ok := doc["info"].(map[string]interface{})
	if !ok {
		t.Fatal("missing info section")
	}
	if got := info["version"]; got != "1.5.0" {
		t.Fatalf("version = %v, want 1.5.0", got)
	}
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("missing paths section")
	}
	for _, p := range []string{
		"/api/v1/auth/login-history",
		"/api/v1/auth/security/summary",
		"/api/v1/auth/devices",
		"/api/v1/auth/sessions/{id}/ips",
		"/api/v1/auth/logout-others",
		"/api/v1/admin/security/events",
		"/api/v1/admin/security/stats",
		"/api/v1/admin/users/{id}/block",
	} {
		if _, ok := paths[p]; !ok {
			t.Fatalf("missing path %s", p)
		}
	}
	if !strings.Contains(swaggerSpec, "UserDevice:") {
		t.Fatal("missing UserDevice schema")
	}
}

// TestSwaggerYAMLFileInSync keeps api/swagger/swagger.yaml identical to the
// embedded spec. Regenerate with: WRITE_SWAGGER_YAML=1 go test ./cmd/server
func TestSwaggerYAMLFileInSync(t *testing.T) {
	path := "../../api/swagger/swagger.yaml"
	if os.Getenv("WRITE_SWAGGER_YAML") == "1" {
		if err := os.WriteFile(path, []byte(swaggerSpec), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != swaggerSpec {
		t.Fatal("api/swagger/swagger.yaml is out of sync; run WRITE_SWAGGER_YAML=1 go test ./cmd/server")
	}
}
