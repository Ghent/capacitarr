package main

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

// sampleHTML mirrors the structure of a real Nuxt-generated index.html.
// It contains all the patterns that rewriteHTML must handle, including
// inline scripts that require CSP nonce injection.
const sampleHTML = `<!DOCTYPE html><html><head>` +
	`<script type="text/javascript">(function(){var t=localStorage.getItem('capacitarr:theme')})()</script>` +
	`<link rel="stylesheet" href="/_assets/entry.DeJAGcQG.css" crossorigin>` +
	`<link rel="modulepreload" as="script" crossorigin href="/_assets/BZLeI64h.js">` +
	`<script type="module" src="/_assets/BZLeI64h.js" crossorigin></script>` +
	`</head><body>` +
	`<div id="__nuxt"></div>` +
	`<script>window.__NUXT__={};window.__NUXT__.config={public:{apiBaseUrl:""},app:{baseURL:"/",buildId:"test-build",buildAssetsDir:"/_assets/",cdnURL:""}}</script>` +
	`<script type="application/json" data-nuxt-data="nuxt-app" id="__NUXT_DATA__">[{}]</script>` +
	`</body></html>`

func TestRewriteHTML_SubdirectoryPath(t *testing.T) {
	result := string(rewriteHTML([]byte(sampleHTML), "/capacitarr/"))

	// Asset paths should be rewritten
	assertContains(t, result, `href="/capacitarr/_assets/entry.DeJAGcQG.css"`)
	assertContains(t, result, `href="/capacitarr/_assets/BZLeI64h.js"`)
	assertContains(t, result, `src="/capacitarr/_assets/BZLeI64h.js"`)

	// Nuxt config should be rewritten (keys are unquoted in minified Nuxt output)
	assertContains(t, result, `baseURL:"/capacitarr/"`)
	assertContains(t, result, `apiBaseUrl:"/capacitarr"`)

	// buildAssetsDir should NOT be rewritten — Nuxt treats it as relative to baseURL
	assertContains(t, result, `buildAssetsDir:"/_assets/"`)

	// Original root paths should NOT be present
	assertNotContains(t, result, `"/_assets/entry.DeJAGcQG.css"`)
	assertNotContains(t, result, `baseURL:"/"`)
}

func TestRewriteHTML_NestedSubdirectory(t *testing.T) {
	result := string(rewriteHTML([]byte(sampleHTML), "/apps/media/capacitarr/"))

	assertContains(t, result, `href="/apps/media/capacitarr/_assets/entry.DeJAGcQG.css"`)
	assertContains(t, result, `baseURL:"/apps/media/capacitarr/"`)
	assertContains(t, result, `apiBaseUrl:"/apps/media/capacitarr"`)

	// buildAssetsDir should NOT be rewritten
	assertContains(t, result, `buildAssetsDir:"/_assets/"`)
}

func TestRewriteHTML_RootPath_NoOp(t *testing.T) {
	// When baseURL is "/", rewriteHTML should still work (no-op for most replacements)
	// but the apiBaseUrl should be set to ""
	result := string(rewriteHTML([]byte(sampleHTML), "/"))

	// Asset paths should remain unchanged (replacing "/_assets/" with "/_assets/" is identity)
	assertContains(t, result, `href="/_assets/entry.DeJAGcQG.css"`)

	// baseURL should remain "/"
	assertContains(t, result, `baseURL:"/"`)

	// buildAssetsDir should remain "/_assets/"
	assertContains(t, result, `buildAssetsDir:"/_assets/"`)

	// apiBaseUrl should be empty string (no prefix needed)
	assertContains(t, result, `apiBaseUrl:""`)
}

func TestRewriteHTML_NormalizesBaseURL(t *testing.T) {
	// Missing leading slash
	result := string(rewriteHTML([]byte(sampleHTML), "capacitarr/"))
	assertContains(t, result, `baseURL:"/capacitarr/"`)

	// Missing trailing slash
	result = string(rewriteHTML([]byte(sampleHTML), "/capacitarr"))
	assertContains(t, result, `baseURL:"/capacitarr/"`)

	// Missing both
	result = string(rewriteHTML([]byte(sampleHTML), "capacitarr"))
	assertContains(t, result, `baseURL:"/capacitarr/"`)
}

func TestRewriteHTML_WithExistingApiBaseUrl(t *testing.T) {
	// When apiBaseUrl has a non-empty value (e.g. from dev build), it should be replaced
	htmlWithAPIBase := `<script>window.__NUXT__={};window.__NUXT__.config={public:{apiBaseUrl:"http://localhost:8080"},app:{baseURL:"/",buildAssetsDir:"/_assets/"}}</script>`

	result := string(rewriteHTML([]byte(htmlWithAPIBase), "/capacitarr/"))
	assertContains(t, result, `apiBaseUrl:"/capacitarr"`)
	assertNotContains(t, result, `apiBaseUrl:"http://localhost:8080"`)
}

func TestBuildHTMLTemplates_RootPath(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(sampleHTML)},
	}

	tmpl := buildHTMLTemplates(fsys, "/")
	if tmpl == nil {
		t.Fatal("expected non-nil templates even for root baseURL (nonce placeholders)")
	}

	// Template should have nonce placeholders but no base URL rewriting
	assertContains(t, string(tmpl.index), `href="/_assets/entry.DeJAGcQG.css"`)
	assertContains(t, string(tmpl.index), cspNoncePlaceholder)
}

func TestBuildHTMLTemplates_SubdirectoryPath(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(sampleHTML)},
	}

	tmpl := buildHTMLTemplates(fsys, "/capacitarr/")
	if tmpl == nil {
		t.Fatal("expected non-nil templates for subdirectory baseURL")
	}
	if tmpl.index == nil {
		t.Error("expected non-nil index in templates")
	}
	if tmpl.spa != nil {
		t.Error("expected nil spa in templates (no 200.html in test FS)")
	}

	// Verify the template HTML is rewritten AND has nonce placeholders
	assertContains(t, string(tmpl.index), `baseURL:"/capacitarr/"`)
	assertContains(t, string(tmpl.index), cspNoncePlaceholder)
}

func TestBuildHTMLTemplates_WithSPAFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(sampleHTML)},
		"200.html":   &fstest.MapFile{Data: []byte(sampleHTML)},
	}

	tmpl := buildHTMLTemplates(fsys, "/capacitarr/")
	if tmpl == nil {
		t.Fatal("expected non-nil templates")
	}
	if tmpl.index == nil {
		t.Error("expected non-nil index in templates")
	}
	if tmpl.spa == nil {
		t.Error("expected non-nil spa in templates")
	}

	// Both should be rewritten and have nonce placeholders
	assertContains(t, string(tmpl.index), `baseURL:"/capacitarr/"`)
	assertContains(t, string(tmpl.index), cspNoncePlaceholder)
	assertContains(t, string(tmpl.spa), `baseURL:"/capacitarr/"`)
	assertContains(t, string(tmpl.spa), cspNoncePlaceholder)
}

func TestBuildHTMLTemplates_MissingIndexHTML(t *testing.T) {
	fsys := fstest.MapFS{} // empty filesystem

	tmpl := buildHTMLTemplates(fsys, "/capacitarr/")
	if tmpl != nil {
		t.Error("expected nil templates when index.html is missing")
	}
}

func TestBuildHTMLTemplates_NoncePlaceholdersSurviveRewrite(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(sampleHTML)},
	}

	tmpl := buildHTMLTemplates(fsys, "/capacitarr/")
	if tmpl == nil {
		t.Fatal("expected non-nil templates")
	}

	// Verify nonce placeholders are present (injected after rewrite)
	result := string(tmpl.index)
	count := strings.Count(result, cspNoncePlaceholder)
	if count != 2 {
		t.Errorf("expected 2 nonce placeholders (theme + __NUXT__), got %d", count)
	}

	// Verify the theme/splash script has a nonce placeholder
	assertContains(t, result, `<script type="text/javascript" nonce="__CSP_NONCE__">`)

	// Verify the __NUXT__ config script has a nonce placeholder
	assertContains(t, result, `<script nonce="__CSP_NONCE__">window.__NUXT__`)
}

func TestReadFSFile(t *testing.T) {
	content := []byte("hello world")
	fsys := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: content},
	}

	data, err := readFSFile(fsys, "test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestReadFSFile_NotFound(t *testing.T) {
	fsys := fstest.MapFS{}

	_, err := readFSFile(fsys, "missing.txt")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// assertContains checks that s contains substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("expected string to contain %q, but it did not.\nFull string:\n%s", substr, s)
	}
}

// assertNotContains checks that s does NOT contain substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if contains(s, substr) {
		t.Errorf("expected string to NOT contain %q, but it did.\nFull string:\n%s", substr, s)
	}
}

func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && containsString(s, substr)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure the test file uses the fs package (for interface compliance).
var _ fs.FS = fstest.MapFS{}
