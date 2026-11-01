<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch } from '../lib/api';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import AIProviderModal from '../lib/components/AIProviderModal.svelte';
  import type { AIConversation, AIMessage, AIProvider } from '../lib/types';
  import '../styles/pages/assistant.css';

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
        content: `Kesalahan AI: ${err.message || 'Tidak ada respon dari server'}`,
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

<div class="pg-assistant assistant-layout">
  <!-- Baris Header Utama -->
  <div class="assistant-head">
    <div class="head-title-wrap">
      <div class="title-row">
        <button
          class="btn btn--sm btn--icon btn--ghost"
          onclick={() => (sidebarOpen = !sidebarOpen)}
          title={sidebarOpen ? 'Tutup Riwayat' : 'Buka Riwayat'}
          aria-label="Toggle riwayat chat"
        >
          <i class={sidebarOpen ? 'ri-side-bar-line' : 'ri-menu-unfold-line'}></i>
        </button>
        <h2 class="page-header__title">AI Copilot & Diagnostik Server</h2>
      </div>
      <p class="page-header__sub">Asisten pemecahan masalah read-only server &middot; Didukung multi-provider kustom</p>
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

      <div class="badge badge--violet badge--pill" title="Mencegah eksekusi aksi destruktif tanpa konfirmasi administrator">
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
          <div class="state state--loading">
            <span class="spinner"></span>
            <span class="state__desc">Memuat riwayat...</span>
          </div>
        {:else if conversations.length === 0}
          <div class="state state--empty">
            <i class="ri-chat-history-line"></i>
            <span class="state__desc">Belum ada riwayat percakapan</span>
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
                  class="btn btn--icon btn--sm btn--ghost conv-delete-btn"
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
          <div class="state state--loading">
            <span class="spinner"></span>
            <span class="state__desc">Memuat percakapan...</span>
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

                <div class="msg-content">
                  {#if m.content.startsWith('Kesalahan AI:')}
                    <i class="ri-error-warning-line text-danger"></i>
                  {/if}
                  {m.content}
                </div>

                <!-- Rekomendasi Aksi Cepat (jika disediakan oleh AI) -->
                {#if parseActions(m.tool_name).length > 0}
                  <div class="actions-list">
                    <div class="action-title"><i class="ri-flashlight-line"></i> Rekomendasi Aksi:</div>
                    <div class="action-buttons">
                      {#each parseActions(m.tool_name) as act}
                        <button class="btn btn--sm btn--secondary" onclick={() => handleAction(act)}>
                          <i class="ri-flashlight-line"></i> {act.label} ({act.target})
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
          class="input grow"
          placeholder="Tanyakan masalah server (mis. kenapa nginx 502, cek ram)..."
          bind:value={messageInput}
          onkeydown={handleInputKeydown}
          disabled={sending}
        />
        <button
          type="submit"
          class="btn btn--primary"
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
</div>