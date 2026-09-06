package config

import "testing"

func TestLoadReadsVerificationRelayEnvironment(t *testing.T) {
	t.Setenv("EMAIL_VERIFICATION_RELAY_URL", "https://relay.example.test/v1/verification-email")
	t.Setenv("EMAIL_VERIFICATION_RELAY_TOKEN", "runtime-token")

	cfg := Load()
	if cfg.Email.VerificationRelayURL != "https://relay.example.test/v1/verification-email" {
		t.Fatalf("verification relay URL was not loaded: %q", cfg.Email.VerificationRelayURL)
	}
	if cfg.Email.VerificationRelayToken != "runtime-token" {
		t.Fatal("verification relay token was not loaded")
	}
}
