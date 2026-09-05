package upstream

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

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
