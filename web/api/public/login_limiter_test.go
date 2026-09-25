package public

import (
	"testing"
	"time"
)

func TestLoginAttemptLimiterBoundsAccountAndRemoteIP(t *testing.T) {
	now := time.Now()
	l := &loginAttemptLimiter{}
	for i := 0; i < loginAccountLimit; i++ {
		if !l.Allow("192.0.2.1", "Admin", now) {
			t.Fatalf("attempt %d rejected too early", i)
		}
	}
	if l.Allow("192.0.2.2", "admin", now) {
		t.Fatal("account limit was bypassed with another IP or case")
	}
	if !l.Allow("192.0.2.2", "admin", now.Add(loginWindow)) {
		t.Fatal("account limit did not expire")
	}

	l = &loginAttemptLimiter{}
	for i := 0; i < loginIPLimit; i++ {
		if !l.Allow("192.0.2.3", string(rune(0x1000+i)), now) {
			t.Fatalf("IP attempt %d rejected too early", i)
		}
	}
	if l.Allow("192.0.2.3", "another-account", now) {
		t.Fatal("IP limit was bypassed with another account")
	}
}
