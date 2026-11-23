package server

import (
	"compress/gzip"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// isCompressibleExt menentukan apakah ekstensi berkas layak dikompresi Gzip on-the-fly bila klien tidak mendukung Brotli
func isCompressibleExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".js", ".mjs", ".css", ".html", ".svg", ".json", ".txt", ".xml":
		return true
	default:
		return false
	}
}

// acceptEncoding memeriksa apakah header Accept-Encoding klien mencantumkan algoritma tertentu (mis. "br" atau "gzip")
func acceptEncoding(r *http.Request, enc string) bool {
	ae := strings.ToLower(r.Header.Get("Accept-Encoding"))
	for _, part := range strings.Split(ae, ",") {
		token := strings.TrimSpace(strings.Split(part, ";")[0])
		if token == enc {
			return true
		}
	}
	return false
}

// serveStaticWithCompression menyajikan aset statis dari fs.FS dengan dukungan:
// 1. Pre-compressed Brotli (.br) zero-CPU bila tersedia dan didukung klien
// 2. Fallback Gzip on-the-fly untuk berkas teks/JS/CSS
// 3. Cache-Control immutable 1 tahun untuk berkas hash Vite di bawah /assets/
// 4. Auto-reload modul ES apabila chunk JS lawas sudah diganti setelah pembaruan biner
func serveStaticWithCompression(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	cleanPath := path.Clean("/" + r.URL.Path)
	relPath := strings.TrimPrefix(cleanPath, "/")
	if relPath == "" {
		relPath = "index.html"
	}

	// Proteksi chunk JS lawas pasca-update biner agar tab browser langsung memuat ulang bundle terbaru
	if strings.HasPrefix(cleanPath, "/assets/") && strings.HasSuffix(cleanPath, ".js") {
		if _, err := fs.Stat(fsys, relPath); err != nil {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("window.location.reload();\nexport default {};\n"))
			return
		}
	}

	// Fallback SPA untuk rute tak ditemukan selain /assets/
	stat, err := fs.Stat(fsys, relPath)
	if err != nil || stat.IsDir() {
		if strings.HasPrefix(cleanPath, "/assets/") {
			http.NotFound(w, r)
			return
		}
		relPath = "index.html"
		stat, err = fs.Stat(fsys, relPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	ext := filepath.Ext(relPath)
	if ctype := mime.TypeByExtension(ext); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	w.Header().Add("Vary", "Accept-Encoding")

	// Aset ber-hash di /assets/ bersifat immutable sehingga aman di-cache panjang di browser
	if strings.HasPrefix(cleanPath, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}

	// Prioritas 1: Sajikan berkas pre-compressed Brotli (.br) langsung dari FS tanpa beban kompresi CPU
	if acceptEncoding(r, "br") {
		brPath := relPath + ".br"
		if brFile, err := fsys.Open(brPath); err == nil {
			defer brFile.Close()
			if brStat, err := brFile.Stat(); err == nil && !brStat.IsDir() {
				w.Header().Set("Content-Encoding", "br")
				if rs, ok := brFile.(io.ReadSeeker); ok {
					http.ServeContent(w, r, filepath.Base(relPath), brStat.ModTime(), rs)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = io.Copy(w, brFile)
				return
			}
		}
	}

	origFile, err := fsys.Open(relPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer origFile.Close()

	// Prioritas 2: Fallback Gzip streaming untuk klien HTTP tanpa dukungan Brotli pada berkas teks/JS/CSS
	if acceptEncoding(r, "gzip") && isCompressibleExt(ext) && stat.Size() >= 1024 && r.Method != http.MethodHead {
		w.Header().Set("Content-Encoding", "gzip")
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err == nil {
			defer gz.Close()
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(gz, origFile)
			return
		}
	}

	// Prioritas 3: Sajikan berkas mentah standar
	if rs, ok := origFile.(io.ReadSeeker); ok {
		http.ServeContent(w, r, filepath.Base(relPath), stat.ModTime(), rs)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, origFile)
}
