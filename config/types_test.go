package config

import "testing"

func TestGitteConfig_TelemetryUnmarshal(t *testing.T) {
	yamlData := []byte(`
telemetry:
  endpoint: https://apm.example.com:8200
  headers:
    Authorization: "Bearer secret"
`)
	cfg, err := LoadGitteConfigFromYAML(yamlData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Telemetry.Endpoint != "https://apm.example.com:8200" {
		t.Errorf("endpoint = %q, want https://apm.example.com:8200", cfg.Telemetry.Endpoint)
	}
	if got := cfg.Telemetry.Headers["Authorization"]; got != "Bearer secret" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer secret")
	}
}
