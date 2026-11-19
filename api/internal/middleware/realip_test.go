package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func captureRemoteAddr(out *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*out = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	})
}

func runRealIP(trustedProxy bool, remoteAddr, xff string) string {
	var got string
	h := RealIP(trustedProxy)(captureRemoteAddr(&got))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return got
}

func TestRealIP_Disabled(t *testing.T) {
	got := runRealIP(false, "172.17.0.2:12345", "1.2.3.4")
	if got != "172.17.0.2:12345" {
		t.Fatalf("trustedProxy=false: RemoteAddr должен остаться, got %q", got)
	}
}

func TestRealIP_TrustedPrivatePeer(t *testing.T) {
	got := runRealIP(true, "172.17.0.2:12345", "1.2.3.4")
	if got != "1.2.3.4" {
		t.Fatalf("trustedProxy=true, private peer: got %q", got)
	}
}

func TestRealIP_TrustedLoopback(t *testing.T) {
	got := runRealIP(true, "127.0.0.1:12345", "1.2.3.4")
	if got != "1.2.3.4" {
		t.Fatalf("trustedProxy=true, loopback: got %q", got)
	}
}

func TestRealIP_TrustedPublicPeer_Ignored(t *testing.T) {
	got := runRealIP(true, "8.8.8.8:12345", "1.2.3.4")
	if got != "8.8.8.8:12345" {
		t.Fatalf("trustedProxy=true, public peer: XFF должен быть проигнорирован, got %q", got)
	}
}

func TestRealIP_NoXFF(t *testing.T) {
	got := runRealIP(true, "172.17.0.2:12345", "")
	if got != "172.17.0.2:12345" {
		t.Fatalf("no XFF: RemoteAddr должен остаться, got %q", got)
	}
}

func TestRealIP_XFFMultiple(t *testing.T) {
	// XFF "spoofed, real": берём ПРАВЫЙ — его дописал прокси.
	got := runRealIP(true, "172.17.0.2:12345", "6.6.6.6, 203.0.113.7")
	if got != "203.0.113.7" {
		t.Fatalf("XFF multi: должен взять правый IP, got %q", got)
	}
}

func TestRealIP_XRealIPInjection(t *testing.T) {
	var got string
	h := RealIP(true)(captureRemoteAddr(&got))
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.17.0.2:12345"
	req.Header.Set("X-Real-IP", "6.6.6.6")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got != "172.17.0.2:12345" {
		t.Fatalf("X-Real-IP инъекция: RemoteAddr не должен меняться, got %q", got)
	}
}

func TestRealIP_TrueClientIPInjection(t *testing.T) {
	var got string
	h := RealIP(true)(captureRemoteAddr(&got))
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.17.0.2:12345"
	req.Header.Set("True-Client-IP", "6.6.6.6")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got != "172.17.0.2:12345" {
		t.Fatalf("True-Client-IP инъекция: got %q", got)
	}
}

func TestRealIP_XFFGarbageRight(t *testing.T) {
	got := runRealIP(true, "172.17.0.2:12345", "203.0.113.7, garbage")
	if got != "203.0.113.7" {
		t.Fatalf("XFF с мусором: got %q", got)
	}
}

func TestRealIP_XFFIPv6(t *testing.T) {
	got := runRealIP(true, "172.17.0.2:12345", "2001:db8::1")
	if got != "2001:db8::1" {
		t.Fatalf("IPv6: got %q", got)
	}
}
