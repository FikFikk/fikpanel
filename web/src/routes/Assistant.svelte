<script lang="ts">
  import { apiFetch } from '../lib/api';

  interface ActionBtn {
    label: string;
    action: string;
    target: string;
  }

  interface ChatMessage {
    role: 'user' | 'assistant';
    text: string;
    actions?: ActionBtn[];
    time: string;
  }

  interface Props {
    onSvcAction: (unit: string, action: string) => void;
    onNotify: (msg: string) => void;
  }

  let { onSvcAction, onNotify }: Props = $props();

  let messageInput = $state('');
  let loading = $state(false);
  let messages = $state<ChatMessage[]>([
    {
      role: 'assistant',
      text: 'Halo! Saya AI Copilot diagnostik server (Read-Only Guardrail Aktif).\nSaya dapat menganalisis kesehatan vhost Nginx, pemakaian RAM/CPU, serta unit systemd yang bermasalah. Tanyakan apa saja mengenai kondisi server Anda.',
      time: '12:00',
    },
  ]);

  const promptSuggestions = [
    'cek status nginx dan web server',
    'analisis pemakaian memory dan CPU',
    'periksa kapasitas penyimpanan disk',
    'cek apakah ada error 502 Bad Gateway',
  ];

  async function sendMessage(text: string) {
    if (!text.trim() || loading) return;
    const now = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

    messages = [...messages, { role: 'user', text: text.trim(), time: now }];
    messageInput = '';
    loading = true;

    try {
      const data = await apiFetch<{ ok: boolean; response: string; actions?: ActionBtn[] }>(
        '/api/ai/chat',
        {
          method: 'POST',
          body: JSON.stringify({ message: text.trim() }),
        }
      );

      messages = [
        ...messages,
        {
          role: 'assistant',
          text: data.response,
          actions: data.actions,
          time: new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }),
        },
      ];
    } catch (err: any) {
      messages = [
        ...messages,
        {
          role: 'assistant',
          text: `Kesalahan diagnosa: ${err.message || 'Server AI tidak merespon'}`,
          time: new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }),
        },
      ];
    } finally {
      loading = false;
    }
  }

  function handleAction(act: ActionBtn) {
    if (act.action === 'service.restart') {
      onSvcAction(act.target, 'restart');
      onNotify(`Menjalankan aksi rekomendasi: restart ${act.target}`);
    }
  }
</script>

<div class="assistant-container">
  <div class="head-bar">
    <div>
      <h2 class="title">AI Copilot & Diagnostik Server</h2>
      <p class="subtitle">Asisten pemecahan masalah read-only sesuai Blueprint Bagian 5 &middot; Human-in-the-loop</p>
    </div>
    <div class="guard-pill">
      <span class="shield-dot"></span>
      <span>Read-Only Guardrails Enforced</span>
    </div>
  </div>

  <div class="suggestions">
    {#each promptSuggestions as ps}
      <button class="sug-chip" onclick={() => sendMessage(ps)}>{ps}</button>
    {/each}
  </div>

  <div class="chat-box">
    <div class="chat-messages">
      {#each messages as m}
        <div class="msg-wrap {m.role === 'user' ? 'msg-user' : 'msg-ai'}">
          <div class="msg-bubble">
            <div class="msg-author">{m.role === 'user' ? 'Anda' : 'AI Copilot'} &middot; {m.time}</div>
            <div class="msg-content">{m.text}</div>

            {#if m.actions && m.actions.length > 0}
              <div class="actions-list">
                <div class="action-title">Rekomendasi Aksi:</div>
                <div class="action-buttons">
                  {#each m.actions as act}
                    <button class="act-btn" onclick={() => handleAction(act)}>
                      âš¡ {act.label} ({act.target})
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        </div>
      {/each}

      {#if loading}
        <div class="msg-wrap msg-ai">
          <div class="msg-bubble loading-bubble">
            <span class="dot"></span><span class="dot"></span><span class="dot"></span>
            <span style="margin-left:0.5rem;font-size:0.78rem;">Menganalisis sistem...</span>
          </div>
        </div>
      {/if}
    </div>

    <form class="chat-input-bar" onsubmit={(e) => { e.preventDefault(); sendMessage(messageInput); }}>
      <input
        type="text"
        placeholder="Tanyakan masalah server (mis. kenapa nginx 502, cek ram)..."
        bind:value={messageInput}
        disabled={loading}
      />
      <button type="submit" class="btn-send" disabled={loading || !messageInput.trim()}>
        Kirim
      </button>
    </form>
  </div>
</div>

<style>
  .assistant-container {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--text);
  }
  .subtitle {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-top: 0.2rem;
  }
  .guard-pill {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    background: color-mix(in srgb, var(--violet) 12%, transparent);
    border: 1px solid color-mix(in srgb, var(--violet) 25%, transparent);
    color: var(--violet);
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.3rem 0.75rem;
    border-radius: 999px;
  }
  .shield-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--violet);
  }
  .suggestions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .sug-chip {
    background: color-mix(in srgb, var(--text) 5%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
    color: var(--text-secondary);
    font-size: 0.75rem;
    padding: 0.35rem 0.75rem;
    border-radius: 20px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .sug-chip:hover {
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    color: var(--accent);
    border-color: color-mix(in srgb, var(--accent) 25%, transparent);
  }
  .chat-box {
    background: color-mix(in srgb, var(--bg-hover) 75%, transparent);
    border: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    height: 520px;
    overflow: hidden;
  }
  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .msg-wrap {
    display: flex;
  }
  .msg-user {
    justify-content: flex-end;
  }
  .msg-ai {
    justify-content: flex-start;
  }
  .msg-bubble {
    max-width: 80%;
    padding: 0.85rem 1.15rem;
    border-radius: 12px;
    font-size: 0.85rem;
    line-height: 1.5;
  }
  .msg-user .msg-bubble {
    background: var(--accent);
    color: var(--text-on-accent);
    font-weight: 500;
  }
  .msg-ai .msg-bubble {
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
    color: var(--text);
  }
  .msg-author {
    font-size: 0.7rem;
    color: var(--text-muted);
    margin-bottom: 0.35rem;
  }
  .msg-user .msg-author {
    color: color-mix(in srgb, var(--text-on-accent) 70%, transparent);
  }
  .msg-content {
    white-space: pre-wrap;
  }
  .actions-list {
    margin-top: 0.85rem;
    padding-top: 0.75rem;
    border-top: 1px solid color-mix(in srgb, var(--text) 8%, transparent);
  }
  .action-title {
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--accent);
    margin-bottom: 0.4rem;
  }
  .action-buttons {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .act-btn {
    background: color-mix(in srgb, var(--accent) 15%, transparent);
    border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
    color: var(--accent);
    padding: 0.35rem 0.75rem;
    border-radius: 6px;
    font-size: 0.75rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }
  .act-btn:hover {
    background: var(--accent);
    color: var(--text-on-accent);
  }
  .chat-input-bar {
    display: flex;
    gap: 0.5rem;
    padding: 0.85rem 1.25rem;
    background: var(--bg-raised);
    border-top: 1px solid color-mix(in srgb, var(--text) 7%, transparent);
  }
  .chat-input-bar input {
    flex: 1;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid color-mix(in srgb, var(--text) 10%, transparent);
    border-radius: 8px;
    padding: 0.6rem 0.85rem;
    color: var(--text);
    font-size: 0.85rem;
    outline: none;
  }
  .chat-input-bar input:focus {
    border-color: var(--accent);
  }
  .btn-send {
    background: var(--accent);
    color: var(--text-on-accent);
    border: none;
    border-radius: 8px;
    padding: 0.6rem 1.25rem;
    font-weight: 600;
    font-size: 0.82rem;
    cursor: pointer;
  }
  .btn-send:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
  .loading-bubble {
    display: flex;
    align-items: center;
  }
  .dot {
    width: 6px;
    height: 6px;
    background: var(--violet);
    border-radius: 50%;
    margin-right: 4px;
    animation: bounce 1.4s infinite ease-in-out both;
  }
  @keyframes bounce {
    0%,
    80%,
    100% {
      transform: scale(0);
    }
    40% {
      transform: scale(1);
    }
  }
</style>