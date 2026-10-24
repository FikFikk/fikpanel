package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"fikpanel/internal/store"
)

type contextKey string

const userContextKey contextKey = "auth_user"

// RateLimiter mengontrol frekuensi percobaan login gagal per IP
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewRateLimiter membuat instance rate limiter in-memory
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
	}
}

// Allow memeriksa apakah IP masih memiliki kuota percobaan login (maks 5 gagal / 15 menit)
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-15 * time.Minute)

	var valid []time.Time
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	return len(valid) < 5
}

// RecordFailure mencatat percobaan autentikasi yang gagal
func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// ResetMembersihkan rekaman gagal setelah login berhasil
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// getIP mengekstrak alamat IP klien dari request
func getIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// AuthMiddleware memverifikasi cookie sesi login pengguna
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("panel_session")
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		user, err := s.store.GetSessionUser(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"sesi kedaluwarsa atau tidak valid"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRFMiddleware memvalidasi header proteksi mutasi untuk mencegah serangan CSRF
func (s *Server) CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			// Lewati verifikasi CSRF khusus endpoint login publik
			if !strings.HasSuffix(r.URL.Path, "/api/auth/login") && strings.HasPrefix(r.URL.Path, "/api/") {
				if r.Header.Get("X-Panel-Request") != "1" {
					http.Error(w, `{"error":"header CSRF X-Panel-Request tidak valid"}`, http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware menyematkan header keamanan HTTP standar
func (s *Server) SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext mengekstrak data user dari konteks HTTP
func GetUserFromContext(ctx context.Context) *store.User {
	if u, ok := ctx.Value(userContextKey).(*store.User); ok {
		return u
	}
	return nil
}
