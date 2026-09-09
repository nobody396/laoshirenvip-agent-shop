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

func TestLoadReadsFeishuNotificationSecretEnvironment(t *testing.T) {
	t.Setenv("NOTIFICATION_FEISHU_APP_SECRET", "runtime-feishu-secret")

	cfg := Load()
	if cfg.Notification.FeishuAppSecret != "runtime-feishu-secret" {
		t.Fatal("Feishu notification secret was not loaded")
	}
}

func TestLoadReadsPartnerInvoiceEmailEnvironment(t *testing.T) {
	t.Setenv("INVOICE_PARTNER_EMAIL_ENABLED", "true")
	t.Setenv("INVOICE_PARTNER_EMAIL_PASSWORD", "runtime-partner-password")

	cfg := Load()
	if !cfg.Invoice.PartnerEmail.Enabled || cfg.Invoice.PartnerEmail.Password != "runtime-partner-password" {
		t.Fatal("partner invoice email config was not loaded")
	}
	if cfg.Invoice.PartnerEmail.Host != "mail.spacemail.com" || cfg.Invoice.PartnerEmail.Port != 465 || cfg.Invoice.PartnerEmail.From != "invoice@lsrai.shop" || cfg.Invoice.PartnerEmail.FromName != "开票通知" || !cfg.Invoice.PartnerEmail.UseSSL || cfg.Invoice.PartnerEmail.UseTLS {
		t.Fatalf("unexpected partner invoice email defaults: %+v", cfg.Invoice.PartnerEmail)
	}
}
