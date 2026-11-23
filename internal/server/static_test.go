package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestServeStaticWithCompression(t *testing.T) {
	rawJS := strings.Repeat("console.log('fikpanel-speed');\n", 100)
	fakeBr := []byte("BROTLI_PRECOMPRESSED_BYTES")

	fsys := fstest.MapFS{
		"index.html":          {Data: []byte("<!doctype html><html><body>OK</body></html>")},
		"assets/app-123.js":   {Data: []byte(rawJS)},
		"assets/app-123.js.br": {Data: fakeBr},
	}

	// 1. Klien mendukung Brotli: wajib menyajikan berkas .br dengan header immutable + Vary
	reqBr := httptest.NewRequest(http.MethodGet, "/assets/app-123.js", nil)
	reqBr.Header.Set("Accept-Encoding", "gzip, deflate, br")
	wBr := httptest.NewRecorder()
	serveStaticWithCompression(wBr, reqBr, fsys)
	if wBr.Code != http.StatusOK {
		t.Fatalf("Expected 200 for br asset, got %d", wBr.Code)
	}
	if wBr.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("Expected Content-Encoding=br, got %q", wBr.Header().Get("Content-Encoding"))
	}
	if !strings.Contains(wBr.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("Expected immutable Cache-Control, got %q", wBr.Header().Get("Cache-Control"))
	}
	if !bytes.Equal(wBr.Body.Bytes(), fakeBr) {
		t.Fatalf("Body harus identik dengan berkas .br precompressed")
	}

	// 2. Klien hanya mendukung Gzip: wajib fallback ke kompresi gzip valid
	reqGz := httptest.NewRequest(http.MethodGet, "/assets/app-123.js", nil)
	reqGz.Header.Set("Accept-Encoding", "gzip")
	wGz := httptest.NewRecorder()
	serveStaticWithCompression(wGz, reqGz, fsys)
	if wGz.Code != http.StatusOK || wGz.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Expected 200 gzip fallback, got %d enc=%q", wGz.Code, wGz.Header().Get("Content-Encoding"))
	}
	gr, err := gzip.NewReader(bytes.NewReader(wGz.Body.Bytes()))
	if err != nil {
		t.Fatalf("Gagal membaca stream gzip: %v", err)
	}
	unzipped, _ := io.ReadAll(gr)
	_ = gr.Close()
	if string(unzipped) != rawJS {
		t.Fatalf("Isi hasil dekompresi gzip tidak cocok")
	}

	// 3. Chunk JS usang pasca-update: wajib membalas reload stub no-store
	reqStale := httptest.NewRequest(http.MethodGet, "/assets/Settings-oldhash.js", nil)
	wStale := httptest.NewRecorder()
	serveStaticWithCompression(wStale, reqStale, fsys)
	if wStale.Code != http.StatusOK || !strings.Contains(wStale.Body.String(), "window.location.reload()") {
		t.Fatalf("Chunk JS usang harus membalas reload stub, got %d %s", wStale.Code, wStale.Body.String())
	}

	// 4. Dokumen HTML utama wajib no-store agar hash bundle selalu terbaru
	reqIdx := httptest.NewRequest(http.MethodGet, "/", nil)
	wIdx := httptest.NewRecorder()
	serveStaticWithCompression(wIdx, reqIdx, fsys)
	if wIdx.Code != http.StatusOK || !strings.Contains(wIdx.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("index.html harus no-store, got %d cc=%q", wIdx.Code, wIdx.Header().Get("Cache-Control"))
	}
}
