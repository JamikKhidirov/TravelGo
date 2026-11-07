package middleware

import "testing"

func TestRateLimiterBurstAndRefill(t *testing.T) {
	l := NewRateLimiter(2) // 2 requests per minute, burst 2
	if l == nil {
		t.Fatal("limiter should not be nil")
	}
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") {
		t.Fatal("burst requests should be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("third immediate request should be limited")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("different key must have its own bucket")
	}
}

func TestRateLimiterDisabled(t *testing.T) {
	if NewRateLimiter(0) != nil {
		t.Fatal("zero rate should disable the limiter")
	}
}
