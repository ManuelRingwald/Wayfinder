package store

import (
	"strings"
	"testing"
)

// TestFeedSecretsReconcileTriggerMigration guards #177: a feed-secret change must
// drive the change-driven reconcile, exactly like a change to feeds or
// subscriptions — otherwise a newly stored OpenSky credential is only picked up
// on the next interval sweep and looks like it had "no effect". The trigger lives
// in migration 00020; assert it is present and wired to the reconcile-notify
// function so it can't silently regress.
func TestFeedSecretsReconcileTriggerMigration(t *testing.T) {
	migs, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	var up string
	for _, m := range migs {
		if m.version == 20 {
			up = m.up
			break
		}
	}
	if up == "" {
		t.Fatal("migration 20 (feed_secrets reconcile notify) missing or empty")
	}
	for _, want := range []string{
		"feed_secrets_notify_reconcile",
		"ON feed_secrets",
		"wayfinder_notify_reconcile",
		"AFTER INSERT OR UPDATE OR DELETE",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("migration 20 up SQL missing %q:\n%s", want, up)
		}
	}
}
