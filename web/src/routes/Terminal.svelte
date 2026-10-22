<script lang="ts">
  import { apiFetch } from '../lib/api';

  interface HistoryItem {
    command: string;
    output: string;
    time: string;
  }

  let commandInput = $state('');
  let executing = $state(false);
  let history = $state<HistoryItem[]>([
    {
      command: 'uname -a',
      output: 'Linux fikpanel-node 6.8.0-45-generic #45-Ubuntu SMP PREEMPT_DYNAMIC x86_64 GNU/Linux',
      time: '12:00:00',
    },
    {
      command: 'uptime',
      output: ' 15:30:00 up 14 days,  3 users,  load average: 0.12, 0.08, 0.05',
      time: '12:00:01',
    },
  ]);

  const quickCommands = [
    'uptime',
    'free -m',
    'df -h',
    'uname -a',
    'whoami',
    'systemctl status nginx',
  ];

  async function runCommand(cmd: string) {
    if (!cmd.trim() || executing) return;
    executing = true;

    try {
      const data = await apiFetch<{ ok: boolean; command: string; output: string }>(
        '/api/terminal/exec',
        {
          method: 'POST',
          body: JSON.stringify({ command: cmd.trim() }),
        }
      );

      const now = new Date().toLocaleTimeString();
      history = [...history, { command: cmd.trim(), output: data.output, time: now }];
      commandInput = '';
    } catch (err: any) {
      const now = new Date().toLocaleTimeString();
      history = [
        ...history,
        {
          command: cmd.trim(),
          output: `Error: ${err.message || 'Gagal mengeksekusi perintah'}`,
          time: now,
        },
      ];
    } finally {
      executing = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      runCommand(commandInput);
    }
  }
</script>

<div class="terminal-container">
  <div class="head-bar">
    <div>
      <h2 class="title">Web Terminal (Console)</h2>
      <p class="subtitle">Sesi konsol terisolasi aman &middot; Bridge PTY privileged agent</p>
    </div>
    <div class="quick-chips">
      {#each quickCommands as qc}
        <button class="chip" onclick={() => runCommand(qc)}>{qc}</button>
      {/each}
    </div>
  </div>

  <div class="terminal-box">
    <div class="term-head">
      <div class="dots">
        <span class="d-red"></span>
        <span class="d-yellow"></span>
        <span class="d-green"></span>
      </div>
      <div class="term-title">root@fikpanel &mdash; bash</div>
    </div>

    <div class="term-body">
      {#each history as item}
        <div class="term-entry">
          <div class="prompt-line">
            <span class="prompt-user">root@fikpanel</span>:<span class="prompt-path">~</span>#
            <span class="prompt-cmd">{item.command}</span>
          </div>
          <pre class="term-output">{item.output}</pre>
        </div>
      {/each}

      <div class="input-line">
        <span class="prompt-user">root@fikpanel</span>:<span class="prompt-path">~</span>#
        <input
          type="text"
          bind:value={commandInput}
          onkeydown={handleKeydown}
          placeholder="Ketik perintah (mis. uptime, free -m, df -h)..."
          disabled={executing}
          autofocus
        />
      </div>
    </div>
  </div>
</div>

<style>
  .terminal-container {
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
    color: #f3f4f6;
  }
  .subtitle {
    font-size: 0.8rem;
    color: #9ca3af;
    margin-top: 0.2rem;
  }
  .quick-chips {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .chip {
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #9ca3af;
    font-family: 'Fira Code', monospace;
    font-size: 0.72rem;
    padding: 0.25rem 0.55rem;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.15s;
  }
  .chip:hover {
    background: rgba(34, 197, 94, 0.12);
    color: #22c55e;
    border-color: rgba(34, 197, 94, 0.25);
  }
  .terminal-box {
    background: #0d0f14;
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 12px;
    overflow: hidden;
    font-family: 'Fira Code', monospace;
  }
  .term-head {
    display: flex;
    align-items: center;
    background: #16181f;
    padding: 0.6rem 1rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
    position: relative;
  }
  .dots {
    display: flex;
    gap: 0.4rem;
  }
  .dots span {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }
  .d-red {
    background: #ef4444;
  }
  .d-yellow {
    background: #f59e0b;
  }
  .d-green {
    background: #22c55e;
  }
  .term-title {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    font-size: 0.75rem;
    color: #9ca3af;
  }
  .term-body {
    padding: 1.25rem;
    min-height: 400px;
    max-height: 600px;
    overflow-y: auto;
    font-size: 0.85rem;
  }
  .term-entry {
    margin-bottom: 1rem;
  }
  .prompt-line {
    color: #f3f4f6;
  }
  .prompt-user {
    color: #22c55e;
    font-weight: 600;
  }
  .prompt-path {
    color: #818cf8;
  }
  .prompt-cmd {
    color: #f3f4f6;
    margin-left: 0.35rem;
  }
  .term-output {
    color: #9ca3af;
    white-space: pre-wrap;
    margin-top: 0.35rem;
    padding-left: 0.5rem;
    line-height: 1.45;
  }
  .input-line {
    display: flex;
    align-items: center;
  }
  .input-line input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: #f3f4f6;
    font-family: 'Fira Code', monospace;
    font-size: 0.85rem;
    margin-left: 0.5rem;
  }
</style>