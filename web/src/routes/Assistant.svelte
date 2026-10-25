<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import AIProviderModal from '../lib/components/AIProviderModal.svelte';
  import type { AIConversation, AIMessage, AIProvider } from '../lib/types';

  interface ActionBtn {
    label: string;
    action: string;
    target: string;
  }

  interface Props {
    onSvcAction: (unit: string, action: string) => void;
    onNotify: (msg: string) => void;
  }

  let { onSvcAction, onNotify }: Props = $props();

  // State percakapan & riwayat
  let conversations = $state<AIConversation[]>([]);
  let activeConvId = $state<number | null>(null);
  let activeTitle = $state('Percakapan Baru');
  let messages = $state<AIMessage[]>([]);
  let convLoading = $state(false);
  let msgLoading = $state(false);

  // State pengiriman pesan
  let messageInput = $state('');
  let sending = $state(false);
  let messagesContainer = $state<HTMLDivElement | null>(null);

  // State provider aktif & modal pengaturan
  let providers = $state<AIProvider[]>([]);
  let defaultProvider = $state<AIProvider | null>(null);
  let showProviderModal = $state(false);
  let sidebarOpen = $state(true);

  const promptSuggestions = [
    'cek status nginx dan web server',
    'analisis pemakaian memory dan CPU',
    'periksa kapasitas penyimpanan disk',
    'cek apakah ada error 502 Bad Gateway',
  ];

  // Muat daftar provider untuk badge status aktif
  async function loadProviders() {
    try {
      const res = await apiFetch<{ ok: boolean; providers: AIProvider[] }>('/api/ai/providers');
      providers = res.providers || [];
      defaultProvider = providers.find((p) => p.is_default) || (providers.length > 0 ? providers[0] : null);
    } catch {
      // Abaikan jika offline, sistem akan menggunakan fallback lokal
      defaultProvider = null;
    }
  }

  // Muat seluruh riwayat percakapan milik user
  async function loadConversations(selectFirst = false) {
    convLoading = true;
    try {
      const res = await apiFetch<{ ok: boolean; conversations: AIConversation[] }>('/api/ai/conversations');
      conversations = res.conversations || [];

      if (selectFirst && conversations.length > 0 && !activeConvId) {
        await selectConversation(conversations[0].id);
      } else if (activeConvId) {
        const found = conversations.find((c) => c.id === activeConvId);
        if (found) activeTitle = found.title;
      }
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat riwayat percakapan');
    } finally {
      convLoading = false;
    }
  }

  // Pilih percakapan tertentu dan ambil riwayat pesannya
  async function selectConversation(id: number) {
    if (activeConvId === id && messages.length > 0) return;
    activeConvId = id;
    msgLoading = true;

    try {
      const res = await apiFetch<{ ok: boolean; conversation: AIConversation; messages: AIMessage[] }>(
        `/api/ai/conversations/${id}/messages`
      );
      activeTitle = res.conversation.title || 'Percakapan';
      messages = res.messages || [];
      await scrollToBottom();
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat pesan percakapan');
    } finally {
      msgLoading = false;
    }
  }

  // Memulai percakapan baru yang kosong
  function startNewConversation() {
    activeConvId = null;
    activeTitle = 'Percakapan Baru';
    messages = [];
    messageInput = '';
  }

  // Hapus satu percakapan dari riwayat
  async function deleteConversation(e: Event, conv: AIConversation) {
    e.stopPropagation();
    const ok = await confirmDialog({
      title: 'Hapus Percakapan?',
      message: `Riwayat pesan dalam "${conv.title}" akan dihapus permanen dari server.`,
      confirmText: 'Hapus Percakapan',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/ai/conversations/${conv.id}`, { method: 'DELETE' });
      onNotify('Percakapan berhasil dihapus');
      if (activeConvId === conv.id) {
        startNewConversation();
      }
      await loadConversations();
    } catch (err: any) {
      onNotify(err.message || 'Gagal menghapus percakapan');
    }
  }

  // Kosongkan seluruh riwayat obrolan
  async function clearAllConversations() {
    if (conversations.length === 0) return;
    const ok = await confirmDialog({
      title: 'Kosongkan Seluruh Riwayat Chat?',
      message: 'Semua utas dan pesan AI akan dihapus permanen dari database SQLite panel.',
      confirmText: 'Kosongkan Semua',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch('/api/ai/conversations', { method: 'DELETE' });
      onNotify('Seluruh riwayat chat dikosongkan');
      startNewConversation();
      await loadConversations();
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengosongkan riwayat');
    }
  }

  // Kirim pesan pengguna ke server AI
  async function sendMessage(text: string) {
    const cleanText = text.trim();
    if (!cleanText || sending) return;

    messageInput = '';
    sending = true;

    // Tambahkan pesan user sementara secara optimis
    const tempUserMsg: AIMessage = {
      id: Date.now(),
      conversationID: activeConvId || 0,
      role: 'user',
      content: cleanText,
      ts: Math.floor(Date.now() / 1000),
    };
    messages = [...messages, tempUserMsg];
    await scrollToBottom();

    try {
      const data = await apiFetch<{
        ok: boolean;
        conversation_id: number;
        response: string;
        provider_name: string;
        model: string;
        actions?: ActionBtn[];
      }>('/api/ai/chat', {
        method: 'POST',
        body: JSON.stringify({
          conversation_id: activeConvId || 0,
          message: cleanText,
        }),
      });

      // Update ID percakapan aktif jika ini pesan pertama
      if (!activeConvId || activeConvId !== data.conversation_id) {
        activeConvId = data.conversation_id;
        await loadConversations();
      }

      // Tambahkan respon asisten ke feed
      const assistantMsg: AIMessage = {
        id: Date.now() + 1,
        conversationID: data.conversation_id,
        role: 'assistant',
        content: data.response,
        tool_name: data.actions && data.actions.length > 0 ? JSON.stringify(data.actions) : undefined,
        ts: Math.floor(Date.now() / 1000),
      };
      messages = [...messages, assistantMsg];
      await scrollToBottom();
    } catch (err: any) {
      const errorMsg: AIMessage = {
        id: Date.now() + 1,
        conversationID: activeConvId || 0,
        role: 'assistant',
        content: `⚠️ Kesalahan AI: ${err.message || 'Tidak ada respon dari server'}`,
        ts: Math.floor(Date.now() / 1000),
      };
      messages = [...messages, errorMsg];
      await scrollToBottom();
    } finally {
      sending = false;
    }
  }

  function handleAction(act: ActionBtn) {
    if (act.action === 'service.restart') {
      onSvcAction(act.target, 'restart');
      onNotify(`Menjalankan aksi rekomendasi: restart ${act.target}`);
    }
  }

  function parseActions(toolName?: string): ActionBtn[] {
    if (!toolName) return [];
    try {
      return JSON.parse(toolName);
    } catch {
      return [];
    }
  }

  function formatTime(ts: number): string {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
  }

  function formatDate(ts: number): string {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
  }

  async function scrollToBottom() {
    await tick();
    if (messagesContainer) {
      messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
  }

  function handleInputKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage(messageInput);
    }
  }

  onMount(() => {
    loadProviders();
    loadConversations(true);
  });
</script>

<div class="assistant-layout">
  <!-- Baris Header Utama -->
  <div class="assistant-head">
    <div class="head-title-wrap">
      <div class="title-row">
        <button
          class="btn btn--sm btn--ghost sidebar-toggle"
          onclick={() => (sidebarOpen = !sidebarOpen)}
          title={sidebarOpen ? 'Tutup Riwayat' : 'Buka Riwayat'}
          aria-label="Toggle riwayat chat"
        >
          <i class={sidebarOpen ? 'ri-side-bar-line' : 'ri-menu-unfold-line'}></i>
        </button>
        <h2 class="title">AI Copilot & Diagnostik Server</h2>
      </div>
      <p class="subtitle">Asisten pemecahan masalah read-only server &middot; Didukung multi-provider kustom</p>
    </div>

    <div class="head-controls">
      <!-- Status Provider Aktif -->
      <button
        class="provider-badge"
        onclick={() => (showProviderModal = true)}
        title="Klik untuk ubah provider AI"
      >
        <i class="ri-cpu-line"></i>
        {#if defaultProvider}
          <span class="p-active-name">{defaultProvider.name}</span>
          <span class="p-active-model mono">({defaultProvider.model})</span>
        {:else}
          <span class="p-active-name">Offline Mode</span>
          <span class="p-active-model">(Rule-based)</span>
        {/if}
        <i class="ri-settings-4-line p-cog"></i>
      </button>

      <button
        class="btn btn--sm btn--secondary"
        onclick={() => (showProviderModal = true)}
      >
        <i class="ri-settings-3-line"></i> Provider AI
      </button>

      <div class="guard-pill" title="Mencegah eksekusi aksi destruktif tanpa konfirmasi administrator">
        <span class="shield-dot"></span>
        <span>Guardrail Read-Only</span>
      </div>
    </div>
  </div>

  <!-- Workspace Obrolan & Riwayat -->
  <div class="chat-workspace">
    <!-- Panel Kiri: Riwayat Percakapan -->
    {#if sidebarOpen}
      <aside class="chat-sidebar">
        <div class="sidebar-header">
          <span class="sidebar-title">Riwayat Chat</span>
          <button class="btn btn--sm btn--primary" onclick={startNewConversation}>
            <i class="ri-add-line"></i> Baru
          </button>
        </div>

        {#if convLoading}
          <div class="sidebar-loading">
            <span class="spinner"></span>
            <span>Memuat riwayat...</span>
          </div>
        {:else if conversations.length === 0}
          <div class="sidebar-empty">
            <i class="ri-chat-history-line"></i>
            <span>Belum ada riwayat percakapan</span>
          </div>
        {:else}
          <div class="conversation-list">
            {#each conversations as c}
              <div
                class="conv-item"
                class:conv-active={activeConvId === c.id}
              >
                <button
                  type="button"
                  class="conv-select-btn"
                  onclick={() => selectConversation(c.id)}
                >
                  <div class="conv-title truncate">{c.title}</div>
                  <div class="conv-meta">
                    <span>{formatDate(c.updated_at)}</span>
                    <span>&middot;</span>
                    <span>{c.message_count} pesan</span>
                  </div>
                </button>
                <button
                  type="button"
                  class="conv-delete-btn"
                  onclick={(e) => deleteConversation(e, c)}
                  title="Hapus percakapan"
                  aria-label="Hapus percakapan"
                >
                  <i class="ri-delete-bin-line"></i>
                </button>
              </div>
            {/each}
          </div>

          {#if conversations.length > 1}
            <div class="sidebar-footer">
              <button class="btn-clear-history" onclick={clearAllConversations}>
                <i class="ri-delete-bin-2-line"></i> Kosongkan Riwayat
              </button>
            </div>
          {/if}
        {/if}
      </aside>
    {/if}

    <!-- Panel Kanan: Jendela Percakapan Aktif -->
    <main class="chat-main">
      <div class="active-conv-bar">
        <div class="active-conv-info truncate">
          <i class="ri-chat-1-line"></i>
          <span class="conv-current-title truncate">{activeTitle}</span>
        </div>
        {#if messages.length > 0}
          <button class="btn btn--sm btn--ghost" onclick={startNewConversation}>
            <i class="ri-chat-new-line"></i> Sesi Baru
          </button>
        {/if}
      </div>

      <!-- Feed Pesan -->
      <div class="chat-feed" bind:this={messagesContainer}>
        {#if msgLoading}
          <div class="feed-loading">
            <span class="spinner"></span>
            <span>Memuat percakapan...</span>
          </div>
        {:else if messages.length === 0}
          <!-- Tampilan Percakapan Kosong / Awal -->
          <div class="welcome-box">
            <div class="welcome-icon">
              <i class="ri-robot-2-line"></i>
            </div>
            <h3 class="welcome-title">Halo! Saya AI Copilot Diagnostik Server</h3>
            <p class="welcome-desc">
              Tanyakan kondisi vhost Nginx, pemakaian RAM/CPU, disk penuh, atau error 502 Bad Gateway.
              {#if defaultProvider}
                Didukung oleh <strong>{defaultProvider.name}</strong> ({defaultProvider.model}).
              {:else}
                Sedang berjalan dalam mode <em>offline diagnostic</em>.
              {/if}
            </p>

            <div class="prompt-chips">
              {#each promptSuggestions as ps}
                <button class="prompt-chip" onclick={() => sendMessage(ps)}>
                  <i class="ri-sparkling-line"></i> {ps}
                </button>
              {/each}
            </div>
          </div>
        {:else}
          {#each messages as m}
            <div class="msg-wrap {m.role === 'user' ? 'msg-user' : 'msg-ai'}">
              <div class="msg-bubble">
                <div class="msg-author">
                  {#if m.role === 'user'}
                    <span><i class="ri-user-line"></i> Anda</span>
                  {:else}
                    <span><i class="ri-robot-line"></i> {defaultProvider ? defaultProvider.name : 'AI Copilot'}</span>
                  {/if}
                  <span>&middot;</span>
                  <span>{formatTime(m.ts)}</span>
                </div>

                <div class="msg-content">{m.content}</div>

                <!-- Rekomendasi Aksi Cepat (jika disediakan oleh AI) -->
                {#if parseActions(m.tool_name).length > 0}
                  <div class="actions-list">
                    <div class="action-title"><i class="ri-flashlight-line"></i> Rekomendasi Aksi:</div>
                    <div class="action-buttons">
                      {#each parseActions(m.tool_name) as act}
                        <button class="act-btn" onclick={() => handleAction(act)}>
                          ⚡ {act.label} ({act.target})
                        </button>
                      {/each}
                    </div>
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        {/if}

        {#if sending}
          <div class="msg-wrap msg-ai">
            <div class="msg-bubble loading-bubble">
              <span class="spinner"></span>
              <span class="loading-label">Menganalisis sistem & memproses jawaban...</span>
            </div>
          </div>
        {/if}
      </div>

      <!-- Bilah Input Pesan -->
      <form class="chat-input-bar" onsubmit={(e) => { e.preventDefault(); sendMessage(messageInput); }}>
        <input
          type="text"
          class="chat-input"
          placeholder="Tanyakan masalah server (mis. kenapa nginx 502, cek ram)..."
          bind:value={messageInput}
          onkeydown={handleInputKeydown}
          disabled={sending}
        />
        <button
          type="submit"
          class="btn btn--primary btn-send"
          disabled={sending || !messageInput.trim()}
        >
          {#if sending}
            <span class="spinner"></span>
          {:else}
            <i class="ri-send-plane-fill"></i> Kirim
          {/if}
        </button>
      </form>
    </main>
  </div>
</div>

<!-- Modal Manajemen Provider AI Kustom -->
{#if showProviderModal}
  <AIProviderModal
    onClose={() => (showProviderModal = false)}
    onNotify={onNotify}
    onProviderChange={() => {
      loadProviders();
    }}
  />
{/if}

<style>
  .assistant-layout {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    height: calc(100vh - var(--topbar-height) - var(--space-8));
    min-height: 560px;
  }

  .assistant-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-3);
  }

  .head-title-wrap {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .sidebar-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-1) var(--space-2);
  }

  .title {
    font-size: var(--text-xl);
    font-weight: var(--weight-bold);
    color: var(--text);
  }

  .subtitle {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  .head-controls {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .provider-badge {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-full);
    color: var(--text);
    font-size: var(--text-xs);
    cursor: pointer;
    transition: all var(--duration-fast);
  }

  .provider-badge:hover {
    border-color: var(--violet);
    background: var(--tint-violet);
  }

  .p-active-name {
    font-weight: var(--weight-semibold);
    color: var(--violet);
  }

  .p-active-model {
    color: var(--text-muted);
  }

  .p-cog {
    color: var(--text-subtle);
  }

  .guard-pill {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    background: var(--tint-violet);
    border: 1px solid color-mix(in srgb, var(--violet) 25%, transparent);
    color: var(--violet);
    font-size: var(--text-2xs);
    font-weight: var(--weight-semibold);
    padding: 0.35rem 0.75rem;
    border-radius: var(--radius-full);
  }

  .shield-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--violet);
  }

  /* Workspace chat */
  .chat-workspace {
    display: grid;
    grid-template-columns: 280px 1fr;
    flex: 1;
    min-height: 0;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    overflow: hidden;
    box-shadow: var(--shadow-sm);
  }

  :global(.assistant-layout:has(.chat-sidebar:not(:first-child))) .chat-workspace {
    grid-template-columns: 1fr;
  }

  /* Sidebar Riwayat */
  .chat-sidebar {
    display: flex;
    flex-direction: column;
    background: var(--bg-inset);
    border-right: 1px solid var(--border-subtle);
    overflow: hidden;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--border-subtle);
  }

  .sidebar-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .conversation-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-2);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .conv-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-1);
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    transition: all var(--duration-fast);
  }

  .conv-item:hover {
    background: var(--bg-hover);
  }

  .conv-item.conv-active {
    background: var(--bg-active);
    border-color: var(--border-strong);
  }

  .conv-select-btn {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-1) var(--space-2);
    background: none;
    border: none;
    text-align: left;
    cursor: pointer;
  }

  .conv-title {
    font-size: var(--text-sm);
    font-weight: var(--weight-medium);
    color: var(--text);
  }

  .conv-meta {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: var(--text-2xs);
    color: var(--text-subtle);
  }

  .conv-delete-btn {
    opacity: 0;
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 4px;
    border-radius: var(--radius-sm);
    font-size: 0.9rem;
    transition: opacity var(--duration-fast), color var(--duration-fast);
  }

  .conv-item:hover .conv-delete-btn {
    opacity: 1;
  }

  .conv-delete-btn:hover {
    color: var(--danger-strong);
    background: var(--tint-danger);
  }

  .sidebar-footer {
    padding: var(--space-2) var(--space-3);
    border-top: 1px solid var(--border-subtle);
  }

  .btn-clear-history {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-2);
    background: none;
    border: none;
    color: var(--text-subtle);
    font-size: var(--text-xs);
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: all var(--duration-fast);
  }

  .btn-clear-history:hover {
    color: var(--danger-strong);
    background: var(--tint-danger);
  }

  .sidebar-loading,
  .sidebar-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    padding: var(--space-6);
    color: var(--text-subtle);
    font-size: var(--text-xs);
    text-align: center;
  }

  /* Main Chat Area */
  .chat-main {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }

  .active-conv-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-5);
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-subtle);
  }

  .active-conv-info {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--text-secondary);
    font-size: var(--text-sm);
  }

  .conv-current-title {
    font-weight: var(--weight-semibold);
    color: var(--text);
  }

  .chat-feed {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .feed-loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    height: 100%;
    color: var(--text-muted);
    font-size: var(--text-sm);
  }

  /* Welcome Screen */
  .welcome-box {
    margin: auto;
    max-width: 540px;
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: var(--space-6) var(--space-4);
  }

  .welcome-icon {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
    border-radius: var(--radius-xl);
    background: var(--tint-violet);
    color: var(--violet);
    font-size: 2rem;
    margin-bottom: var(--space-3);
  }

  .welcome-title {
    font-size: var(--text-lg);
    font-weight: var(--weight-bold);
    color: var(--text);
    margin-bottom: var(--space-2);
  }

  .welcome-desc {
    font-size: var(--text-sm);
    color: var(--text-muted);
    line-height: var(--leading-normal);
    margin-bottom: var(--space-5);
  }

  .prompt-chips {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: var(--space-2);
  }

  .prompt-chip {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-2) var(--space-3);
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-full);
    font-size: var(--text-xs);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all var(--duration-fast);
  }

  .prompt-chip:hover {
    color: var(--text);
    border-color: var(--accent);
    background: var(--tint-accent);
  }

  /* Bubbles */
  .msg-wrap {
    display: flex;
    width: 100%;
  }

  .msg-user {
    justify-content: flex-end;
  }

  .msg-ai {
    justify-content: flex-start;
  }

  .msg-bubble {
    max-width: min(720px, 85%);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-lg);
    font-size: var(--text-sm);
    line-height: var(--leading-normal);
  }

  .msg-user .msg-bubble {
    background: var(--accent);
    color: var(--text-on-accent);
    border-bottom-right-radius: 2px;
  }

  .msg-ai .msg-bubble {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text);
    border-bottom-left-radius: 2px;
  }

  .msg-author {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--text-2xs);
    margin-bottom: var(--space-1);
  }

  .msg-user .msg-author {
    color: color-mix(in srgb, var(--text-on-accent) 75%, transparent);
    justify-content: flex-end;
  }

  .msg-ai .msg-author {
    color: var(--text-muted);
  }

  .msg-content {
    white-space: pre-wrap;
    word-break: break-word;
  }

  .actions-list {
    margin-top: var(--space-3);
    padding-top: var(--space-2);
    border-top: 1px solid var(--border-subtle);
  }

  .action-title {
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    color: var(--accent);
    margin-bottom: var(--space-2);
  }

  .action-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .act-btn {
    background: var(--tint-accent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
    color: var(--accent);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
    font-weight: var(--weight-semibold);
    cursor: pointer;
    transition: all var(--duration-fast);
  }

  .act-btn:hover {
    background: var(--accent);
    color: var(--text-on-accent);
  }

  .loading-bubble {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .loading-label {
    font-size: var(--text-xs);
    color: var(--text-muted);
  }

  /* Input bar */
  .chat-input-bar {
    display: flex;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-5);
    background: var(--bg-surface);
    border-top: 1px solid var(--border-subtle);
  }

  .chat-input {
    flex: 1;
    padding: var(--space-2) var(--space-4);
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
    color: var(--text);
    font-size: var(--text-sm);
    outline: none;
    transition: border-color var(--duration-fast);
  }

  .chat-input:focus {
    border-color: var(--accent);
  }

  .btn-send {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  @media (max-width: 900px) {
    .chat-workspace {
      grid-template-columns: 1fr;
    }
    .chat-sidebar {
      position: absolute;
      top: 0;
      left: 0;
      bottom: 0;
      width: 280px;
      z-index: var(--z-dropdown);
      box-shadow: var(--shadow-lg);
    }
  }
</style>