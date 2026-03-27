package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateCSPNonce_Length(t *testing.T) {
	nonce := generateCSPNonce()
	if nonce == "" {
		t.Fatal("nonce must not be empty")
	}
	// 16 bytes → 22 chars in base64url (no padding)
	if len(nonce) != 22 {
		t.Errorf("nonce length = %d, want 22 (base64url of 16 bytes)", len(nonce))
	}
}

func TestGenerateCSPNonce_ValidBase64URL(t *testing.T) {
	nonce := generateCSPNonce()
	_, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil {
		t.Errorf("nonce is not valid base64url: %v", err)
	}
}

func TestGenerateCSPNonce_Unique(t *testing.T) {
	a := generateCSPNonce()
	b := generateCSPNonce()
	if a == b {
		t.Error("two consecutive nonces must be different")
	}
}

// sampleNuxtHTML mirrors the inline script structure of a real Nuxt build.
const sampleNuxtHTML = `<!DOCTYPE html><html><head>` +
	`<script type="text/javascript">(function(){var t=localStorage.getItem('capacitarr:theme')})()</script>` +
	`<script type="module" src="/_assets/DmIJM9dq.js" crossorigin></script>` +
	`<script>window.__NUXT__={};window.__NUXT__.config={public:{appVersion:"1.5.0"}}</script>` +
	`<script type="application/json" data-nuxt-data="nuxt-app" id="__NUXT_DATA__">[{}]</script>` +
	`</head><body></body></html>`

func TestInjectNoncePlaceholders(t *testing.T) {
	resultBytes, _ := injectNoncePlaceholders([]byte(sampleNuxtHTML))
	result := string(resultBytes)

	// The theme/splash script must get a nonce placeholder
	if !strings.Contains(result, `<script type="text/javascript" nonce="__CSP_NONCE__">`) {
		t.Error("theme/splash script missing nonce placeholder")
	}

	// The __NUXT__ config script must get a nonce placeholder
	if !strings.Contains(result, `<script nonce="__CSP_NONCE__">window.__NUXT__`) {
		t.Error("__NUXT__ config script missing nonce placeholder")
	}

	// External module script must NOT get a nonce (covered by 'self')
	if strings.Contains(result, `<script type="module" src="/_assets/DmIJM9dq.js" nonce=`) {
		t.Error("external module script should not have a nonce")
	}

	// JSON data script must NOT get a nonce (not executable)
	if strings.Contains(result, `<script type="application/json" data-nuxt-data="nuxt-app" nonce=`) {
		t.Error("application/json script should not have a nonce")
	}
}

func TestInjectNoncePlaceholders_NoDoubleInjection(t *testing.T) {
	first, _ := injectNoncePlaceholders([]byte(sampleNuxtHTML))
	secondBytes, _ := injectNoncePlaceholders(first)
	second := string(secondBytes)

	// Count placeholder occurrences — should be exactly 2 (one per inline script)
	count := strings.Count(second, cspNoncePlaceholder)
	if count != 2 {
		t.Errorf("placeholder count after double injection = %d, want 2", count)
	}
}

func TestApplyNonce(t *testing.T) {
	template, _ := injectNoncePlaceholders([]byte(sampleNuxtHTML))
	nonce := "abc123_test-nonce"
	result := string(applyNonce(template, nonce))

	if strings.Contains(result, cspNoncePlaceholder) {
		t.Error("placeholder was not replaced")
	}
	if !strings.Contains(result, `nonce="abc123_test-nonce"`) {
		t.Error("nonce value not found in result")
	}
	// Should appear exactly twice (theme + __NUXT__ scripts)
	count := strings.Count(result, `nonce="abc123_test-nonce"`)
	if count != 2 {
		t.Errorf("nonce attribute count = %d, want 2", count)
	}
}

func TestApplyNonce_PreservesOtherContent(t *testing.T) {
	template, _ := injectNoncePlaceholders([]byte(sampleNuxtHTML))
	nonce := "test-nonce"
	result := string(applyNonce(template, nonce))

	// External script tag preserved as-is
	if !strings.Contains(result, `<script type="module" src="/_assets/DmIJM9dq.js" crossorigin></script>`) {
		t.Error("external script tag was modified")
	}
	// JSON data script preserved as-is
	if !strings.Contains(result, `<script type="application/json" data-nuxt-data="nuxt-app"`) {
		t.Error("JSON data script tag was modified")
	}
	// Page structure preserved
	if !strings.Contains(result, `<!DOCTYPE html>`) {
		t.Error("DOCTYPE missing from result")
	}
}
