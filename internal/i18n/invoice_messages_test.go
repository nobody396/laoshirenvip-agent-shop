package i18n

import "testing"

func TestInvoiceErrorsAreLocalized(t *testing.T) {
	for _, locale := range []string{LocaleZH, LocaleTW, LocaleEN} {
		for _, key := range []string{
			"error.invoice_invalid",
			"error.invoice_order_ineligible",
			"error.invoice_already_requested",
			"error.invoice_create_failed",
			"error.invoice_not_found",
			"error.invoice_wallet_insufficient",
		} {
			if got := T(locale, key); got == key || got == "" {
				t.Fatalf("locale %s did not translate %s", locale, key)
			}
		}
	}
}
