package upstream

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestSharedVariantsPreferFactoryPricesFromINI(t *testing.T) {
	raw, err := json.Marshal("[category]\n250点=80\n500点=160\n[category_factory]\n250点=75\n500点=145\n")
	if err != nil {
		t.Fatal(err)
	}
	got := sharedVariants(raw)
	if got["250点"] != "75" || got["500点"] != "145" || len(got) != 2 {
		t.Fatalf("unexpected variants: %#v", got)
	}
}

func TestSharedStockAdapterRequiresPersistentReferenceRegistry(t *testing.T) {
	_, err := NewAdapter(&siteconnectiondomain.Connection{Protocol: constants.ConnectionProtocolSharedStock}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "reference registry") {
		t.Fatalf("expected registry requirement, got %v", err)
	}
}

func TestUnknownAdapterProtocolIsRejected(t *testing.T) {
	_, err := NewAdapter(&siteconnectiondomain.Connection{Protocol: "unknown"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("expected unsupported protocol error, got %v", err)
	}
}
