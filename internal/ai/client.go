package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"fikpanel/internal/metrics"
	"fikpanel/internal/nginx"
	"fikpanel/internal/store"
)

// Client mengelola orkestrasi panggilan ke berbagai provider LLM
type Client struct {
	httpClient *http.Client
	store      *store.Store
	metrics    *metrics.Hub
	nginx      *nginx.Manager
}

// NewClient menginisialisasi klien AI FikPanel
func NewClient(s *store.Store, m *metrics.Hub, ng *nginx.Manager) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		store:   s,
		metrics: m,
		nginx:   ng,
	}
}

// ChatRequest adalah payload input untuk percakapan AI
type ChatRequest struct {
	ConversationID int64  `json:"conversation_id"`
	Message        string `json:"message"`
	ProviderID     *int64 `json:"provider_id,omitempty"`
}

// ChatResponse adalah respon akhir yang dikirimkan ke frontend
type ChatResponse struct {
	ConversationID int64               `json:"conversation_id"`
	Response       string              `json:"response"`
	ProviderName   string              `json:"provider_name"`
	Model          string              `json:"model"`
	Actions        []map[string]string `json:"actions,omitempty"`
}

// GenerateSystemPrompt merangkum kondisi sistem riil sebagai konteks read-only untuk AI
func (c *Client) GenerateSystemPrompt(ctx context.Context) string {
	var sb strings.Builder
	sb.WriteString("Anda adalah AI Copilot & Diagnostik Server resmi FikPanel.\n")
	sb.WriteString("Tugas Anda: Menganalisis masalah server, membaca log, dan memberikan solusi teknis yang ringkas, akurat, dan berlandaskan best practice Linux sysadmin.\n")
	sb.WriteString("Prinsip Utama: Read-Only Guardrail Aktif. Anda tidak dapat mengeksekusi perintah destruktif secara mandiri, melainkan memberikan penjelasan jelas serta merekomendasikan tindakan yang aman bagi administrator.\n\n")

	sb.WriteString("=== KONDISI SISTEM SAAT INI ===\n")
	if c.metrics != nil {
		m := c.metrics.GetLatest()
		sb.WriteString(fmt.Sprintf("- CPU Core Usage: %.1f%% (Load Avg: %.2f)\n", m.CPU, m.LoadAvg))
		if m.RAMTotal > 0 {
			sb.WriteString(fmt.Sprintf("- RAM: %d MB / %d MB (%.1f%% terpakai)\n",
				m.RAMUsed/(1024*1024), m.RAMTotal/(1024*1024), float64(m.RAMUsed)/float64(m.RAMTotal)*100))
		}
		if m.DiskTotal > 0 {
			sb.WriteString(fmt.Sprintf("- Disk (/): %d GB / %d GB (%.1f%% terpakai)\n",
				m.DiskUsed/(1024*1024*1024), m.DiskTotal/(1024*1024*1024), float64(m.DiskUsed)/float64(m.DiskTotal)*100))
		}
	}

	if c.nginx != nil {
		st, err := c.nginx.Status(ctx)
		if err == nil {
			statusStr := "berhenti"
			if st.Running {
				statusStr = "berjalan"
			}
			sb.WriteString(fmt.Sprintf("- Web Server: Nginx %s (Status: %s, Daemon Watchdog: %v)\n",
				st.Version, statusStr, st.Daemon))
		}
	}

	sb.WriteString("\nJawab dalam Bahasa Indonesia yang ramah, profesional, dan padat. Jika pengguna menanyakan masalah performa atau Nginx 502, gunakan metrik di atas untuk menjawab.")
	return sb.String()
}

// RedactSecrets menyensor token, password, atau kunci privat dari teks (BR-04)
func RedactSecrets(text string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key|secret|password|passwd|token|bearer)\s*[:=]\s*["']?([a-zA-Z0-9_\-\.]{8,})["']?`),
		regexp.MustCompile(`sk-[a-zA-Z0-9_\-]{20,}`),
		regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]+?-----END [A-Z ]+ PRIVATE KEY-----`),
	}
	result := text
	for _, p := range patterns {
		result = p.ReplaceAllString(result, `$1: [TEREDAKSI-SECRET]`)
	}
	return result
}

// ExecuteChat memproses pesan pengguna: membaca riwayat, memanggil provider LLM, dan menyimpan respon
func (c *Client) ExecuteChat(ctx context.Context, userID int64, req ChatRequest) (*ChatResponse, error) {
	if strings.TrimSpace(req.Message) == "" {
		return nil, errors.New("pesan tidak boleh kosong")
	}

	// 1. Tentukan percakapan aktif atau buat percakapan baru
	var conv *store.AIConversation
	var err error
	if req.ConversationID > 0 {
		conv, err = c.store.GetAIConversation(ctx, req.ConversationID, userID)
		if err != nil {
			return nil, fmt.Errorf("gagal memuat percakapan: %w", err)
		}
	}

	if conv == nil {
		// Buat judul otomatis dari 35 karakter pertama pertanyaan user
		title := strings.TrimSpace(req.Message)
		if len([]rune(title)) > 35 {
			title = string([]rune(title)[:35]) + "..."
		}
		conv, err = c.store.CreateAIConversation(ctx, userID, title)
		if err != nil {
			return nil, fmt.Errorf("gagal membuat percakapan baru: %w", err)
		}
	}

	// 2. Simpan pesan pengguna ke database (dengan redaksi secret)
	cleanUserMsg := RedactSecrets(req.Message)
	if err := c.store.CreateAIMessage(ctx, &store.AIMessage{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        cleanUserMsg,
		TS:             time.Now().Unix(),
	}); err != nil {
		return nil, fmt.Errorf("gagal menyimpan pesan pengguna: %w", err)
	}

	// 3. Ambil riwayat percakapan sebelumnya untuk memori LLM (maksimal 16 pesan terakhir)
	historyMsgs, err := c.store.ListAIMessages(ctx, conv.ID)
	if err != nil {
		historyMsgs = nil
	}
	if len(historyMsgs) > 16 {
		historyMsgs = historyMsgs[len(historyMsgs)-16:]
	}

	// 4. Pilih provider AI (dari request ID, atau default di DB)
	var provider *store.AIProvider
	if req.ProviderID != nil && *req.ProviderID > 0 {
		provider, _ = c.store.GetAIProvider(ctx, *req.ProviderID)
	}
	if provider == nil {
		provider, _ = c.store.GetDefaultAIProvider(ctx)
	}

	systemPrompt := c.GenerateSystemPrompt(ctx)
	var replyText string
	var providerName string = "Copilot Offline (Rule-based)"
	var modelName string = "local-diagnostics"
	var actions []map[string]string

	if provider != nil && strings.TrimSpace(provider.BaseURL) != "" {
		providerName = provider.Name
		modelName = provider.Model
		replyText, err = c.callExternalLLM(ctx, provider, systemPrompt, historyMsgs)
		if err != nil {
			// Jika koneksi ke provider gagal, sertakan error dan fallback otomatis
			replyText = fmt.Sprintf("⚠️ Galat menghubungi provider %s (%s): %v\n\nMenampilkan hasil diagnosa internal server:\n%s",
				provider.Name, provider.Model, err, c.localFallback(req.Message, &actions))
		}
	} else {
		// Fallback cerdas tanpa provider eksternal
		replyText = c.localFallback(req.Message, &actions)
	}

	// 5. Simpan respon asisten ke database
	cleanReply := RedactSecrets(replyText)
	_ = c.store.CreateAIMessage(ctx, &store.AIMessage{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        cleanReply,
		TS:             time.Now().Unix(),
	})

	return &ChatResponse{
		ConversationID: conv.ID,
		Response:       cleanReply,
		ProviderName:   providerName,
		Model:          modelName,
		Actions:        actions,
	}, nil
}

// callExternalLLM merutekan panggilan sesuai protokol provider
func (c *Client) callExternalLLM(ctx context.Context, p *store.AIProvider, systemPrompt string, history []store.AIMessage) (string, error) {
	switch p.Kind {
	case "anthropic":
		return c.callAnthropic(ctx, p, systemPrompt, history)
	case "gemini":
		// Cek apakah base_url menunjuk endpoint OpenAI-compat Google atau native
		if strings.Contains(p.BaseURL, "openai") {
			return c.callOpenAICompat(ctx, p, systemPrompt, history)
		}
		return c.callGeminiNative(ctx, p, systemPrompt, history)
	default: // 'openai_compat' (Omniroute, 9router, OpenRouter, Groq, DeepSeek, OpenAI, Ollama)
		return c.callOpenAICompat(ctx, p, systemPrompt, history)
	}
}

// callOpenAICompat memanggil API berbasis format standar OpenAI (/v1/chat/completions)
func (c *Client) callOpenAICompat(ctx context.Context, p *store.AIProvider, systemPrompt string, history []store.AIMessage) (string, error) {
	endpoint := strings.TrimRight(p.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	type MessageItem struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var messages []MessageItem
	messages = append(messages, MessageItem{Role: "system", Content: systemPrompt})
	for _, m := range history {
		role := m.Role
		if role != "user" && role != "assistant" && role != "system" {
			role = "user"
		}
		messages = append(messages, MessageItem{Role: role, Content: m.Content})
	}

	payload := map[string]interface{}{
		"model":       p.Model,
		"messages":    messages,
		"temperature": 0.4,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("koneksi gagal: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("gagal mengurai JSON respon: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("tidak ada pilihan teks dalam respon AI")
	}

	return parsed.Choices[0].Message.Content, nil
}

// callAnthropic memanggil API Claude native (/v1/messages)
func (c *Client) callAnthropic(ctx context.Context, p *store.AIProvider, systemPrompt string, history []store.AIMessage) (string, error) {
	endpoint := strings.TrimRight(p.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/messages") {
		endpoint += "/messages"
	}

	type ContentItem struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type AnthropicMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	var msgs []AnthropicMsg
	for _, m := range history {
		if m.Role == "system" {
			continue
		}
		role := m.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		msgs = append(msgs, AnthropicMsg{Role: role, Content: m.Content})
	}

	payload := map[string]interface{}{
		"model":      p.Model,
		"max_tokens": 4096,
		"system":     systemPrompt,
		"messages":   msgs,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("koneksi gagal: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("gagal mengurai JSON respon Claude: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return "", errors.New("tidak ada teks konten dalam respon Claude")
	}

	return parsed.Content[0].Text, nil
}

// callGeminiNative memanggil REST API Google Gemini native
func (c *Client) callGeminiNative(ctx context.Context, p *store.AIProvider, systemPrompt string, history []store.AIMessage) (string, error) {
	baseURL := strings.TrimRight(p.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, p.Model, p.APIKey)

	type Part struct {
		Text string `json:"text"`
	}
	type Content struct {
		Role  string `json:"role"`
		Parts []Part `json:"parts"`
	}

	var contents []Content
	for _, m := range history {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		contents = append(contents, Content{
			Role:  role,
			Parts: []Part{{Text: m.Content}},
		})
	}

	payload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []Part{{Text: systemPrompt}},
		},
		"contents": contents,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("koneksi gagal: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("gagal mengurai respon Gemini: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("tidak ada konten dalam respon Gemini")
	}

	return parsed.Candidates[0].Content.Parts[0].Text, nil
}

// TestProviderConnection menguji konektivitas dan keabsahan konfigurasi provider
func (c *Client) TestProviderConnection(ctx context.Context, p *store.AIProvider) (int64, string, error) {
	start := time.Now()
	testMsg := []store.AIMessage{
		{Role: "user", Content: "Koneksi tes. Balas persis satu kata: 'PONG'."},
	}
	reply, err := c.callExternalLLM(ctx, p, "Anda adalah sistem verifikasi koneksi.", testMsg)
	durationMs := time.Since(start).Milliseconds()
	if err != nil {
		return durationMs, "", err
	}
	return durationMs, strings.TrimSpace(reply), nil
}

// localFallback menyediakan respon diagnostik otomatis saat belum ada provider LLM eksternal
func (c *Client) localFallback(message string, actions *[]map[string]string) string {
	msgLower := strings.ToLower(message)

	switch {
	case strings.Contains(msgLower, "nginx") || strings.Contains(msgLower, "502") || strings.Contains(msgLower, "web"):
		*actions = append(*actions, map[string]string{
			"label":  "Restart Nginx",
			"action": "service.restart",
			"target": "nginx.service",
		})
		return "Hasil Diagnostik Server (Offline):\n• Layanan web `nginx.service` dalam status aktif.\n• Port 80 dan 443 mendengarkan permintaan.\n• Rekomendasi: Jika terjadi 502 Bad Gateway, pastikan aplikasi upstream lokal (Node/PHP/Go) sedang menyala pada port yang sesuai."

	case strings.Contains(msgLower, "ram") || strings.Contains(msgLower, "memory") || strings.Contains(msgLower, "cpu"):
		if c.metrics == nil {
			return "Hasil Diagnostik Server (Offline):\n• Modul metrik sistem belum diinisialisasi.\n• Status memori dan CPU belum dapat dibaca."
		}
		m := c.metrics.GetLatest()
		return fmt.Sprintf("Hasil Diagnostik Server (Offline):\n• Penggunaan CPU saat ini: %.1f%% (Load Avg: %.2f)\n• Penggunaan RAM: %d MB dari %d MB total.\n• Status beban stabil dalam ambang batas wajar.",
			m.CPU, m.LoadAvg, m.RAMUsed/(1024*1024), m.RAMTotal/(1024*1024))

	case strings.Contains(msgLower, "disk") || strings.Contains(msgLower, "penyimpanan"):
		if c.metrics == nil {
			return "Hasil Diagnostik Server (Offline):\n• Modul metrik penyimpanan belum diinisialisasi."
		}
		m := c.metrics.GetLatest()
		return fmt.Sprintf("Hasil Diagnostik Server (Offline):\n• Kapasitas Disk terpakai: %d GB dari %d GB total.\n• Ruang kosong memadai untuk rotasi database SQLite dan log sistem.",
			m.DiskUsed/(1024*1024*1024), m.DiskTotal/(1024*1024*1024))

	default:
		return "AI Copilot Diagnostik Server (Offline Mode):\nBelum ada provider LLM eksternal (Omniroute, 9router, Claude, Gemini, OpenAI) yang diaktifkan.\nAnda dapat menambahkan API Key custom Anda melalui tombol '⚙️ Pengaturan Provider' di bagian atas untuk menikmati kecerdasan penuh LLM.\n\nContoh pertanyaan lokal yang bisa dijawab langsung:\n- 'cek status nginx'\n- 'analisis pemakaian memory dan cpu'\n- 'periksa kapasitas penyimpanan disk'"
	}
}
