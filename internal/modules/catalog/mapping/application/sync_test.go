package application

import (
	"testing"
	"time"

	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// fakeSettingsProvider 以固定同步间隔实现 SettingsProvider。
type fakeSettingsProvider struct {
	interval time.Duration
}

func TestSyncedSpecValuesKeepsCuratedLabelWhenUpstreamHasNoSpecs(t *testing.T) {
	current := jsonmap.JSON{"name": "Claude Max 5X 1个月"}
	if got := syncedSpecValues(current, jsonmap.JSON{}); got["name"] != current["name"] {
		t.Fatalf("curated spec label was overwritten: %#v", got)
	}
	if got := syncedSpecValues(current, jsonmap.JSON{"race": ""}); got["name"] != current["name"] {
		t.Fatalf("blank upstream spec was treated as a real value: %#v", got)
	}
	upstream := jsonmap.JSON{"name": "Claude Max 5X"}
	if got := syncedSpecValues(current, upstream); got["name"] != upstream["name"] {
		t.Fatalf("non-empty upstream spec should update local value: %#v", got)
	}
}

func (f fakeSettingsProvider) GetUpstreamSyncConfig(fallbackInterval string) (settingsintegration.UpstreamSyncConfig, error) {
	cfg := settingsintegration.DefaultUpstreamSyncConfig()
	cfg.IntervalMinutes = int(f.interval / time.Minute)
	return cfg, nil
}

func (f fakeSettingsProvider) GetUpstreamSyncInterval(fallbackInterval string) (time.Duration, error) {
	return f.interval, nil
}

func TestComputeFullSyncIntervalFloorsAt24h(t *testing.T) {
	svc := &Service{settings: fakeSettingsProvider{interval: 5 * time.Minute}}

	// 5m 同步间隔 × 3 = 15m < 24h，期望落到 24h floor
	got := svc.computeFullSyncInterval()
	if got != fullSyncIntervalFloor {
		t.Fatalf("expected floor=24h, got %v", got)
	}
}

func TestComputeFullSyncIntervalScalesWithLongInterval(t *testing.T) {
	svc := &Service{settings: fakeSettingsProvider{interval: 12 * time.Hour}}

	// 12h * 3 = 36h，应使用 scaled 值
	got := svc.computeFullSyncInterval()
	want := 36 * time.Hour
	if got != want {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestComputeFullSyncIntervalWithoutSettings(t *testing.T) {
	svc := &Service{}
	got := svc.computeFullSyncInterval()
	if got != fullSyncIntervalFloor {
		t.Fatalf("expected floor when settings=nil, got %v", got)
	}
}
