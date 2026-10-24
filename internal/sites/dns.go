package sites

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// DNSResult adalah hasil resolusi DNS satu domain terhadap IP server
type DNSResult struct {
	Domain     string   `json:"domain"`
	IPs        []string `json:"ips"`
	PointsHere bool     `json:"points_here"`
	Error      string   `json:"error,omitempty"`
}

// ipCache menyimpan IP publik hasil deteksi agar tidak memanggil layanan eksternal tiap request
type ipCache struct {
	mu      sync.Mutex
	ips     []string
	fetched time.Time
}

const publicIPTTL = time.Hour

// ServerIPs mengembalikan IP server: env FIKPANEL_PUBLIC_IP, IP interface, dan IP publik (cache 1 jam)
func (m *Manager) ServerIPs(ctx context.Context) []string {
	set := map[string]bool{}

	if env := os.Getenv("FIKPANEL_PUBLIC_IP"); env != "" {
		for _, ip := range strings.Split(env, ",") {
			if ip = strings.TrimSpace(ip); net.ParseIP(ip) != nil {
				set[ip] = true
			}
		}
	} else {
		for _, ip := range m.publicIPs(ctx) {
			set[ip] = true
		}
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			set[ipNet.IP.String()] = true
		}
	}

	out := make([]string, 0, len(set))
	for ip := range set {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out
}

// publicIPs mendeteksi IP publik (penting untuk server di balik NAT cloud) dengan cache
func (m *Manager) publicIPs(ctx context.Context) []string {
	m.ipc.mu.Lock()
	defer m.ipc.mu.Unlock()
	if time.Since(m.ipc.fetched) < publicIPTTL {
		return m.ipc.ips
	}

	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return m.ipc.ips
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Tandai waktu agar kegagalan jaringan tidak memicu percobaan beruntun
		m.ipc.fetched = time.Now().Add(-publicIPTTL + time.Minute)
		return m.ipc.ips
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err == nil && resp.StatusCode == http.StatusOK {
		if ip := strings.TrimSpace(string(body)); net.ParseIP(ip) != nil {
			m.ipc.ips = []string{ip}
		}
	}
	m.ipc.fetched = time.Now()
	return m.ipc.ips
}

// CheckDNS meresolusi A/AAAA setiap domain secara paralel dan membandingkan dengan IP server
func (m *Manager) CheckDNS(ctx context.Context, domains []string) ([]DNSResult, []string) {
	serverIPs := m.ServerIPs(ctx)
	own := map[string]bool{}
	for _, ip := range serverIPs {
		own[ip] = true
	}

	results := make([]DNSResult, len(domains))
	var wg sync.WaitGroup
	for i, d := range domains {
		wg.Add(1)
		go func(i int, d string) {
			defer wg.Done()
			res := DNSResult{Domain: d, IPs: []string{}}
			host := d
			// Wildcard diuji lewat label acak untuk melihat record *.domain
			if strings.HasPrefix(d, "*.") {
				host = "fikpanel-check." + d[2:]
			}
			lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			ips, err := net.DefaultResolver.LookupHost(lookupCtx, host)
			if err != nil {
				res.Error = "domain belum memiliki record A/AAAA"
			}
			for _, ip := range ips {
				res.IPs = append(res.IPs, ip)
				if own[ip] {
					res.PointsHere = true
				}
			}
			results[i] = res
		}(i, d)
	}
	wg.Wait()
	return results, serverIPs
}
