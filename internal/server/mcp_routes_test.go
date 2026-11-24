package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fikpanel/internal/metrics"
	"fikpanel/internal/services"
	"fikpanel/internal/store"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bearerRoundTripper menyuntikkan header Authorization Bearer ke klien MCP pengujian
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if b.token != "" {
		clone.Header.Set("Authorization", "Bearer "+b.token)
	}
	return b.base.RoundTrip(clone)
}

func TestMCPServerStreamableHTTPAndScopes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fikpanel_mcp_test_*")
	if err != nil {
		t.Fatalf("gagal membuat tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	st, err := store.Open(filepath.Join(tempDir, "mcp.db"))
	if err != nil {
		t.Fatalf("gagal membuka store: %v", err)
	}
	defer st.Close()

	if _, err := st.EnsureDevUser("root", "Rahasia123!"); err != nil {
		t.Fatalf("gagal membuat user: %v", err)
	}

	srv, err := NewServer(st, metrics.NewHub(), services.NewManager(st.DB()), 0)
	if err != nil {
		t.Fatalf("gagal membuat server: %v", err)
	}
	defer srv.bgCancel()

	// Login sebagai root untuk memperoleh cookie sesi panel
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"root","password":"Rahasia123!"}`))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("login uji gagal: %d %s", wLogin.Code, wLogin.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range wLogin.Result().Cookies() {
		if c.Name == "panel_session" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatalf("cookie panel_session tidak ditemukan")
	}

	ts := httptest.NewServer(srv.server.Handler)
	defer ts.Close()

	// 1. Akses /mcp tanpa Bearer token wajib ditolak HTTP 401
	respNoAuth, err := http.Post(ts.URL+"/mcp", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatalf("request /mcp gagal: %v", err)
	}
	respNoAuth.Body.Close()
	if respNoAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("harapan 401 tanpa token, dapat %d", respNoAuth.StatusCode)
	}

	// 2. Buat token MCP read-only melalui REST API panel
	createBody := bytes.NewBufferString(`{"name":"Antigravity Test","scope":"readonly"}`)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/mcp/tokens", createBody)
	reqCreate.AddCookie(cookie)
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("X-Panel-Request", "1")
	wCreate := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("gagal membuat token MCP: %d %s", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		OK    bool   `json:"ok"`
		Token string `json:"token"`
		Item  struct {
			ID int64 `json:"id"`
		} `json:"item"`
	}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &created); err != nil || created.Token == "" {
		t.Fatalf("respons pembuatan token tidak valid: %s", wCreate.Body.String())
	}

	// 3. Hubungkan klien resmi Go MCP SDK (StreamableClientTransport) memakai token Read-Only
	ctx := context.Background()
	roClient := mcp.NewClient(&mcp.Implementation{Name: "antigravity-test", Version: "1.0.0"}, nil)
	roTransport := &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{token: created.Token, base: http.DefaultTransport},
		},
	}
	roSession, err := roClient.Connect(ctx, roTransport, nil)
	if err != nil {
		t.Fatalf("gagal connect MCP read-only: %v", err)
	}
	defer roSession.Close()

	toolsRes, err := roSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools read-only gagal: %v", err)
	}
	if len(toolsRes.Tools) != 12 {
		t.Fatalf("harapan 12 tools read-only, dapat %d", len(toolsRes.Tools))
	}
	for _, tl := range toolsRes.Tools {
		if tl.Name == "restart_service" || tl.Name == "test_and_reload_nginx" || tl.Name == "restart_docker_container" {
			t.Fatalf("tool operator %q tidak boleh muncul pada token readonly", tl.Name)
		}
	}

	// 4. Panggil tool server_overview dan pastikan mengembalikan metrik JSON + mencatat call_count
	callRes, err := roSession.CallTool(ctx, &mcp.CallToolParams{Name: "server_overview", Arguments: map[string]any{}})
	if err != nil || callRes.IsError || len(callRes.Content) == 0 {
		t.Fatalf("panggilan server_overview gagal: err=%v res=%+v", err, callRes)
	}

	// Pastikan call_count di daftar token bertambah menjadi 1
	reqList := httptest.NewRequest(http.MethodGet, "/api/mcp/tokens", nil)
	reqList.AddCookie(cookie)
	wList := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wList, reqList)
	if wList.Code != http.StatusOK || !bytes.Contains(wList.Body.Bytes(), []byte(`"call_count":1`)) {
		t.Fatalf("call_count tidak bertambah pasca pemanggilan MCP: %s", wList.Body.String())
	}

	// 5. Buat token Operator dan pastikan 14 tools (10 read-only + 4 operator) diekspos
	reqOp := httptest.NewRequest(http.MethodPost, "/api/mcp/tokens", bytes.NewBufferString(`{"name":"Antigravity Ops","scope":"operator"}`))
	reqOp.AddCookie(cookie)
	reqOp.Header.Set("Content-Type", "application/json")
	reqOp.Header.Set("X-Panel-Request", "1")
	wOp := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wOp, reqOp)
	var opCreated struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(wOp.Body.Bytes(), &opCreated)

	opClient := mcp.NewClient(&mcp.Implementation{Name: "antigravity-ops", Version: "1.0.0"}, nil)
	opSession, err := opClient.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: ts.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &bearerRoundTripper{token: opCreated.Token, base: http.DefaultTransport},
		},
	}, nil)
	if err != nil {
		t.Fatalf("gagal connect MCP operator: %v", err)
	}
	defer opSession.Close()

	opTools, err := opSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools operator gagal: %v", err)
	}
	if len(opTools.Tools) != 17 {
		t.Fatalf("harapan 17 tools operator, dapat %d", len(opTools.Tools))
	}

	// 6. Cabut token read-only dan verifikasi permintaan selanjutnya langsung ditolak 401
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/mcp/tokens/1", nil)
	reqDel.AddCookie(cookie)
	reqDel.Header.Set("X-Panel-Request", "1")
	wDel := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("gagal mencabut token: %d %s", wDel.Code, wDel.Body.String())
	}

	reqAfterRevoke, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewBufferString(`{}`))
	reqAfterRevoke.Header.Set("Authorization", "Bearer "+created.Token)
	respRevoked, err := http.DefaultClient.Do(reqAfterRevoke)
	if err != nil {
		t.Fatalf("request pasca pencabutan gagal: %v", err)
	}
	respRevoked.Body.Close()
	if respRevoked.StatusCode != http.StatusUnauthorized {
		t.Fatalf("harapan 401 setelah token dicabut, dapat %d", respRevoked.StatusCode)
	}
}
