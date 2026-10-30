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
	// Urutkan agar IPv4 publik (non-private) selalu berada di urutan paling pertama (indeks 0)
	sort.Slice(out, func(i, j int) bool {
		ipA := net.ParseIP(out[i])
		ipB := net.ParseIP(out[j])
		score := func(ip net.IP) int {
			if ip == nil {
				return 99
			}
			if ip.To4() != nil && !ip.IsPrivate() {
				return 0 // Prioritas utama: IPv4 Publik (misal 54.254.187.53)
			}
			if ip.To4() == nil && !ip.IsPrivate() {
				return 1 // Prioritas kedua: IPv6 Publik
			}
			if ip.To4() != nil && ip.IsPrivate() {
				return 2 // Prioritas ketiga: IPv4 Private VPC/LAN (misal 172.31.x.x)
			}
			return 3
		}
		sa, sb := score(ipA), score(ipB)
		if sa != sb {
			return sa < sb
		}
		return out[i] < out[j]
	})
	return out
}

// publicIPs mendeteksi IP publik (penting untuk server di balik NAT cloud) dengan cache & multi-provider fallback
func (m *Manager) publicIPs(ctx context.Context) []string {
	m.ipc.mu.Lock()
	defer m.ipc.mu.Unlock()
	if len(m.ipc.ips) > 0 && time.Since(m.ipc.fetched) < publicIPTTL {
		return m.ipc.ips
	}

	endpoints := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me/ip",
	}
	for _, ep := range endpoints {
		reqCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, ep, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		cancel()
		if err == nil && resp.StatusCode == http.StatusOK {
			if ip := strings.TrimSpace(string(body)); net.ParseIP(ip) != nil {
				m.ipc.ips = []string{ip}
				m.ipc.fetched = time.Now()
				return m.ipc.ips
			}
		}
	}
	m.ipc.fetched = time.Now().Add(-publicIPTTL + 30*time.Second)
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
