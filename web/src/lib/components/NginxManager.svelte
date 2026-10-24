<script lang="ts">
  import { onDestroy } from 'svelte';
  import ManagerModal from './ManagerModal.svelte';
  import { apiFetch, formatBytes } from '../api';
  import { confirmDialog } from '../stores/confirm.svelte';
  import type {
    ManagerTab,
    NginxStatus,
    NginxLoad,
    NginxVersions,
    NginxOptimization,
    NginxErrorLog,
  } from '../types';

  interface Props {
    onClose: () => void;
    onNotify: (msg: string) => void;
    onStatusChange?: (st: NginxStatus) => void;
  }

  let { onClose, onNotify, onStatusChange }: Props = $props();

  const tabs: ManagerTab[] = [
    { id: 'service', label: 'Layanan', icon: 'ri-pulse-line' },
    { id: 'config', label: 'File Konfigurasi', icon: 'ri-file-code-line' },
    { id: 'version', label: 'Ganti Versi', icon: 'ri-git-branch-line' },
    { id: 'load', label: 'Status Beban', icon: 'ri-dashboard-3-line' },
    { id: 'optimize', label: 'Optimasi', icon: 'ri-equalizer-line' },
    { id: 'errorlog', label: 'Log Error', icon: 'ri-bug-line' },
  ];

  let activeTab = $state('service');

  // ═══ Layanan ═══
  let status = $state<NginxStatus | null>(null);
  let busyAction = $state('');
  let savingPrefs = $state(false);

  async function loadStatus() {
    try {
      status = await apiFetch<NginxStatus>('/api/software/nginx/status');
      onStatusChange?.(status);
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat status Nginx');
    }
  }

  async function serviceAction(action: string) {
    if (action === 'stop' && !(await confirmDialog({
      title: 'Hentikan Nginx?',
      message: 'Semua situs di server ini tidak akan dapat diakses hingga Nginx dijalankan kembali.',
      confirmText: 'Hentikan Layanan',
      tone: 'danger',
    }))) return;
    busyAction = action;
    try {
      const res = await apiFetch<{ status: NginxStatus }>('/api/software/nginx/service', {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      status = res.status;
      onStatusChange?.(res.status);
      onNotify(`Nginx berhasil di-${action}`);
    } catch (err: any) {
      onNotify(err.message || `Gagal ${action} Nginx`);
    } finally {
      busyAction = '';
    }
  }

  async function savePrefs(next: { daemon: boolean; alert: boolean }) {
    if (!status) return;
    const prev = { daemon: status.daemon, alert: status.alert };
    status = { ...status, ...next };
    savingPrefs = true;
    try {
      await apiFetch('/api/software/nginx/settings', { method: 'POST', body: JSON.stringify(next) });
      onNotify('Preferensi watchdog disimpan');
    } catch (err: any) {
      status = { ...status, ...prev };
      onNotify(err.message || 'Gagal menyimpan preferensi');
    } finally {
      savingPrefs = false;
    }
  }

  function formatUptime(startedAt: number): string {
    if (!startedAt) return '—';
    let s = Math.max(0, Math.floor(Date.now() / 1000) - startedAt);
    const d = Math.floor(s / 86400);
    s %= 86400;
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    if (d > 0) return `${d}h ${h}j ${m}m`;
    if (h > 0) return `${h}j ${m}m`;
    return `${m}m ${s % 60}d`;
  }

  // ═══ File Konfigurasi ═══
  let confContent = $state('');
  let confOriginal = $state('');
  let confPath = $state('');
  let confLoading = $state(false);
  let confSaving = $state(false);
  let confError = $state('');
  let gutterEl = $state<HTMLDivElement | null>(null);
  let confDirty = $derived(confContent !== confOriginal);
  let confLineCount = $derived(confContent.split('\n').length);

  async function loadConfig() {
    confLoading = true;
    confError = '';
    try {
      const res = await apiFetch<{ path: string; content: string }>('/api/software/nginx/config');
      confContent = res.content;
      confOriginal = res.content;
      confPath = res.path;
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat konfigurasi');
    } finally {
      confLoading = false;
    }
  }

  async function saveConfig() {
    if (!confDirty || confSaving) return;
    confSaving = true;
    confError = '';
    try {
      await apiFetch('/api/software/nginx/config', {
        method: 'POST',
        body: JSON.stringify({ content: confContent }),
      });
      confOriginal = confContent;
      onNotify('Konfigurasi lolos uji, disimpan, dan Nginx di-reload');
    } catch (err: any) {
      confError = err.message || 'Gagal menyimpan konfigurasi';
    } finally {
      confSaving = false;
    }
  }

  function onEditorKey(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      saveConfig();
      return;
    }
    // Tab menyisipkan 4 spasi alih-alih memindahkan fokus
    if (e.key === 'Tab') {
      e.preventDefault();
      const ta = e.currentTarget as HTMLTextAreaElement;
      const { selectionStart: s, selectionEnd: en } = ta;
      confContent = confContent.slice(0, s) + '    ' + confContent.slice(en);
      requestAnimationFrame(() => ta.setSelectionRange(s + 4, s + 4));
    }
  }

  function syncGutter(e: Event) {
    if (gutterEl) gutterEl.scrollTop = (e.currentTarget as HTMLTextAreaElement).scrollTop;
  }

  // ═══ Ganti Versi ═══
  let versions = $state<NginxVersions | null>(null);
  let selectedVersion = $state('');
  let switching = $state(false);
  let versionPoll: ReturnType<typeof setInterval> | null = null;

  async function loadVersions() {
    try {
      const v = await apiFetch<NginxVersions>('/api/software/nginx/versions');
      const wasRunning = versions?.job.running;
      versions = v;
      if (!selectedVersion) {
        selectedVersion = v.available.find((a) => a.startsWith(v.current)) || v.available[0] || '';
      }
      if (v.job.running) {
        startVersionPoll();
      } else {
        stopVersionPoll();
        if (wasRunning) {
          onNotify(v.job.message);
          loadStatus();
        }
      }
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat daftar versi');
    }
  }

  function startVersionPoll() {
    if (!versionPoll) versionPoll = setInterval(loadVersions, 2000);
  }

  function stopVersionPoll() {
    if (versionPoll) {
      clearInterval(versionPoll);
      versionPoll = null;
    }
  }

  async function switchVersion() {
    if (!selectedVersion || !versions) return;
    if (versions.current && selectedVersion.startsWith(versions.current)) {
      onNotify('Versi tersebut sudah aktif');
      return;
    }
    if (!(await confirmDialog({
      title: `Ganti Nginx ke versi ${selectedVersion}?`,
      message: 'Layanan web server akan mengalami restart singkat saat beralih versi.',
      confirmText: 'Ganti Versi',
      tone: 'danger',
    }))) return;
    switching = true;
    try {
      await apiFetch('/api/software/nginx/versions/switch', {
        method: 'POST',
        body: JSON.stringify({ version: selectedVersion }),
      });
      onNotify(`Job ganti versi ke ${selectedVersion} dimulai`);
      await loadVersions();
    } catch (err: any) {
      onNotify(err.message || 'Gagal memulai ganti versi');
    } finally {
      switching = false;
    }
  }

  // ═══ Status Beban ═══
  let load = $state<NginxLoad | null>(null);
  let activeHistory = $state<number[]>([]);
  let rpsHistory = $state<number[]>([]);
  let lastReq: { requests: number; ts: number } | null = null;
  let currentRps = $state(0);

  async function loadLoad() {
    try {
      const l = await apiFetch<NginxLoad>('/api/software/nginx/load');
      if (l.available && lastReq && l.timestamp > lastReq.ts) {
        currentRps = Math.max(0, (l.requests - lastReq.requests) / (l.timestamp - lastReq.ts));
        rpsHistory = [...rpsHistory, currentRps].slice(-40);
      }
      if (l.available) {
        lastReq = { requests: l.requests, ts: l.timestamp };
        activeHistory = [...activeHistory, l.active].slice(-40);
      }
      load = l;
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat status beban');
    }
  }

  // Membangun path SVG sparkline dari deret angka
  function sparkPath(values: number[], w = 300, h = 60): string {
    if (values.length < 2) return '';
    const max = Math.max(...values, 1);
    const step = w / (values.length - 1);
    return values
      .map((v, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(h - (v / max) * (h - 6) - 3).toFixed(1)}`)
      .join(' ');
  }

  // ═══ Optimasi ═══
  let opt = $state<NginxOptimization | null>(null);
  let optSaving = $state(false);
  let optError = $state('');

  const optFields: { key: keyof NginxOptimization; label: string; unit: string; hint: string }[] = [
    { key: 'worker_processes', label: 'worker_processes', unit: '', hint: "Jumlah proses worker, 'auto' = sesuai core CPU" },
    { key: 'worker_connections', label: 'worker_connections', unit: '', hint: 'Koneksi maksimal per worker' },
    { key: 'keepalive_timeout', label: 'keepalive_timeout', unit: 'detik', hint: 'Batas waktu koneksi keep-alive' },
    { key: 'gzip', label: 'gzip', unit: '', hint: 'Kompresi transfer respons' },
    { key: 'gzip_min_length', label: 'gzip_min_length', unit: 'KB', hint: 'Ukuran minimal file yang dikompresi' },
    { key: 'gzip_comp_level', label: 'gzip_comp_level', unit: '1-9', hint: 'Level kompresi (tinggi = CPU lebih berat)' },
    { key: 'client_max_body_size', label: 'client_max_body_size', unit: 'MB', hint: 'Ukuran upload maksimal' },
    { key: 'server_names_hash_bucket_size', label: 'server_names_hash_bucket_size', unit: '', hint: 'Ukuran hash nama server (pangkat dua)' },
    { key: 'client_header_buffer_size', label: 'client_header_buffer_size', unit: 'KB', hint: 'Buffer header request' },
    { key: 'client_body_buffer_size', label: 'client_body_buffer_size', unit: 'KB', hint: 'Buffer body request' },
  ];

  async function loadOptimize() {
    optError = '';
    try {
      opt = await apiFetch<NginxOptimization>('/api/software/nginx/optimize');
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat parameter optimasi');
    }
  }

  async function saveOptimize(e: Event) {
    e.preventDefault();
    if (!opt) return;
    optSaving = true;
    optError = '';
    try {
      const payload = {
        ...opt,
        worker_processes: String(opt.worker_processes),
        worker_connections: Number(opt.worker_connections),
        keepalive_timeout: Number(opt.keepalive_timeout),
        gzip_min_length: Number(opt.gzip_min_length),
        gzip_comp_level: Number(opt.gzip_comp_level),
        client_max_body_size: Number(opt.client_max_body_size),
        server_names_hash_bucket_size: Number(opt.server_names_hash_bucket_size),
        client_header_buffer_size: Number(opt.client_header_buffer_size),
        client_body_buffer_size: Number(opt.client_body_buffer_size),
      };
      await apiFetch('/api/software/nginx/optimize', { method: 'POST', body: JSON.stringify(payload) });
      onNotify('Optimasi diterapkan dan Nginx di-reload');
      // Konfigurasi berubah, paksa muat ulang editor saat tab dibuka
      confOriginal = '';
      confContent = '';
    } catch (err: any) {
      optError = err.message || 'Gagal menyimpan optimasi';
    } finally {
      optSaving = false;
    }
  }

  // ═══ Log Error ═══
  let errLog = $state<NginxErrorLog | null>(null);
  let logLines = $state(200);
  let logFilter = $state('');
  let logAuto = $state(false);
  let logClearing = $state(false);
  let logBox = $state<HTMLDivElement | null>(null);

  let logRows = $derived.by(() => {
    if (!errLog?.content) return [] as { text: string; level: string }[];
    const q = logFilter.trim().toLowerCase();
    return errLog.content
      .split('\n')
      .filter((l) => !q || l.toLowerCase().includes(q))
      .map((text) => {
        const m = text.match(/\[(emerg|alert|crit|error|warn|notice|info|debug)\]/);
        return { text, level: m ? m[1] : 'info' };
      });
  });

  async function loadErrorLog() {
    try {
      errLog = await apiFetch<NginxErrorLog>(`/api/software/nginx/error-log?lines=${logLines}`);
      requestAnimationFrame(() => {
        if (logBox) logBox.scrollTop = logBox.scrollHeight;
      });
    } catch (err: any) {
      onNotify(err.message || 'Gagal memuat log error');
    }
  }

  async function clearErrorLog() {
    if (!(await confirmDialog({
      title: 'Kosongkan Log Error Nginx?',
      message: 'Seluruh riwayat catatan error Nginx akan dihapus secara permanen.',
      confirmText: 'Kosongkan Log',
      tone: 'danger',
    }))) return;
    logClearing = true;
    try {
      await apiFetch('/api/software/nginx/error-log/clear', { method: 'POST' });
      onNotify('Log error dikosongkan');
      await loadErrorLog();
    } catch (err: any) {
      onNotify(err.message || 'Gagal mengosongkan log');
    } finally {
      logClearing = false;
    }
  }

  // ═══ Orkestrasi per tab: muat data + auto refresh selama tab aktif ═══
  $effect(() => {
    const tab = activeTab;
    let timer: ReturnType<typeof setInterval> | null = null;

    if (tab === 'service') {
      loadStatus();
      timer = setInterval(loadStatus, 5000);
    } else if (tab === 'config') {
      if (!confOriginal) loadConfig();
    } else if (tab === 'version') {
      loadVersions();
    } else if (tab === 'load') {
      loadLoad();
      timer = setInterval(loadLoad, 3000);
    } else if (tab === 'optimize') {
      loadOptimize();
    } else if (tab === 'errorlog') {
      loadErrorLog();
      if (logAuto) timer = setInterval(loadErrorLog, 5000);
    }

    return () => {
      if (timer) clearInterval(timer);
    };
  });

  onDestroy(stopVersionPoll);

  async function requestClose() {
    if (confDirty && !(await confirmDialog({
      title: 'Tutup tanpa menyimpan?',
      message: 'Perubahan pada konfigurasi Nginx belum disimpan dan akan hilang.',
      confirmText: 'Tutup Editor',
      tone: 'danger',
    }))) return;
    onClose();
  }
</script>

<ManagerModal
  title="Nginx"
  subtitle={status ? `v${status.version || '?'} · ${status.conf_path}` : 'Memuat...'}
  {tabs}
  bind:active={activeTab}
  onClose={requestClose}
>
  {#snippet logo()}
    <svg width="30" height="30" viewBox="0 0 32 32" aria-hidden="true">
      <path d="M16 2 28.1 9v14L16 30 3.9 23V9z" fill="#009639" />
      <path d="M11.5 22V10.5l9 11V10" stroke="#fff" stroke-width="2.6" fill="none" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  {/snippet}

  {#snippet badge()}
    {#if status}
      <span class="state-pill" class:on={status.running}>
        <span class="dot"></span>{status.running ? 'Berjalan' : 'Berhenti'}
      </span>
      {#if status.mock}<span class="mock-pill" title="Lingkungan non-Linux: aksi disimulasikan">SIMULASI</span>{/if}
    {/if}
  {/snippet}

  {#snippet children(tab: string)}
    <!-- ═════════ TAB: LAYANAN ═════════ -->
    {#if tab === 'service'}
      {#if !status}
        <div class="loading"><i class="ri-loader-4-line spin"></i> Memuat status...</div>
      {:else}
        <div class="stat-grid">
          <div class="stat">
            <span class="stat-label">Status</span>
            <span class="stat-value" class:ok={status.running} class:bad={!status.running}>
              {status.running ? 'Berjalan' : 'Berhenti'}
            </span>
          </div>
          <div class="stat">
            <span class="stat-label">PID Master</span>
            <span class="stat-value mono">{status.master_pid || '—'}</span>
          </div>
          <div class="stat">
            <span class="stat-label">Worker</span>
            <span class="stat-value mono">{status.workers || '—'}</span>
          </div>
          <div class="stat">
            <span class="stat-label">Memori</span>
            <span class="stat-value mono">{status.memory_bytes ? formatBytes(status.memory_bytes) : '—'}</span>
          </div>
          <div class="stat">
            <span class="stat-label">Uptime</span>
            <span class="stat-value mono">{status.running ? formatUptime(status.started_at) : '—'}</span>
          </div>
        </div>

        <div class="btn-row">
          {#if status.running}
            <button id="ngx-stop" class="btn btn-danger" disabled={!!busyAction} onclick={() => serviceAction('stop')}>
              <i class={busyAction === 'stop' ? 'ri-loader-4-line spin' : 'ri-stop-circle-line'}></i> Stop
            </button>
          {:else}
            <button id="ngx-start" class="btn btn-primary" disabled={!!busyAction} onclick={() => serviceAction('start')}>
              <i class={busyAction === 'start' ? 'ri-loader-4-line spin' : 'ri-play-circle-line'}></i> Start
            </button>
          {/if}
          <button id="ngx-restart" class="btn btn-ghost" disabled={!!busyAction} onclick={() => serviceAction('restart')}>
            <i class={busyAction === 'restart' ? 'ri-loader-4-line spin' : 'ri-restart-line'}></i> Restart
          </button>
          <button id="ngx-reload" class="btn btn-ghost" disabled={!!busyAction || !status.running} onclick={() => serviceAction('reload')}>
            <i class={busyAction === 'reload' ? 'ri-loader-4-line spin' : 'ri-refresh-line'}></i> Reload
          </button>
        </div>
        <p class="note"><i class="ri-shield-check-line"></i> Start, restart, dan reload selalu didahului uji konfigurasi (<code>nginx -t</code>). Bila gagal, aksi dibatalkan.</p>

        <div class="divider"></div>

        <div class="pref-row">
          <div>
            <div class="pref-title">Peringatan saat Nginx berhenti</div>
            <div class="pref-desc">Catat kejadian ke Audit Log setiap kali watchdog mendeteksi Nginx mati.</div>
          </div>
          <button
            id="ngx-alert-toggle"
            class="switch"
            class:on={status.alert}
            disabled={savingPrefs}
            aria-pressed={status.alert}
            aria-label="Peringatan saat berhenti"
            onclick={() => savePrefs({ daemon: status!.daemon, alert: !status!.alert })}
          ><span></span></button>
        </div>

        <div class="pref-row">
          <div>
            <div class="pref-title">Daemon (auto-start)</div>
            <ul class="pref-list">
              <li>Pemeriksaan otomatis setiap 1 menit.</li>
              <li>Jika Nginx berhenti, daemon akan menyalakannya kembali agar layanan selalu aktif.</li>
            </ul>
          </div>
          <button
            id="ngx-daemon-toggle"
            class="switch"
            class:on={status.daemon}
            disabled={savingPrefs}
            aria-pressed={status.daemon}
            aria-label="Daemon auto-start"
            onclick={() => savePrefs({ daemon: !status!.daemon, alert: status!.alert })}
          ><span></span></button>
        </div>
        {#if status.daemon}
          <p class="note warn"><i class="ri-error-warning-line"></i> Daemon aktif: menghentikan Nginx secara manual akan dinyalakan ulang dalam ±1 menit.</p>
        {/if}
      {/if}

    <!-- ═════════ TAB: FILE KONFIGURASI ═════════ -->
    {:else if tab === 'config'}
      <div class="conf-wrap">
        <div class="conf-top">
          <span class="tips">Tips: <kbd>Ctrl</kbd>+<kbd>S</kbd> simpan · <kbd>Ctrl</kbd>+<kbd>F</kbd> cari · <kbd>Tab</kbd> indentasi</span>
          <span class="mono path">{confPath}</span>
        </div>
        {#if confLoading}
          <div class="loading"><i class="ri-loader-4-line spin"></i> Memuat konfigurasi...</div>
        {:else}
          <div class="editor" class:dirty={confDirty}>
            <div class="gutter" bind:this={gutterEl} aria-hidden="true">
              {#each { length: confLineCount } as _, i}<div>{i + 1}</div>{/each}
            </div>
            <textarea
              id="ngx-conf-editor"
              spellcheck="false"
              bind:value={confContent}
              onkeydown={onEditorKey}
              onscroll={syncGutter}
            ></textarea>
          </div>
          {#if confError}
            <pre class="err-box"><i class="ri-close-circle-line"></i> {confError}</pre>
          {/if}
          <div class="conf-actions">
            <button id="ngx-conf-save" class="btn btn-primary" disabled={!confDirty || confSaving} onclick={saveConfig}>
              <i class={confSaving ? 'ri-loader-4-line spin' : 'ri-save-3-line'}></i> {confSaving ? 'Menguji & menyimpan...' : 'Simpan'}
            </button>
            <button class="btn btn-ghost" disabled={!confDirty || confSaving} onclick={() => { confContent = confOriginal; confError = ''; }}>
              <i class="ri-arrow-go-back-line"></i> Batalkan perubahan
            </button>
            {#if confDirty}<span class="dirty-tag">● Belum disimpan</span>{/if}
          </div>
          <ul class="pref-list">
            <li>Ini file konfigurasi utama Nginx. Jika tidak memahami aturannya, jangan diubah sembarangan.</li>
            <li>Setiap simpan: uji <code>nginx -t</code> → backup otomatis (10 terakhir) → ganti atomik → reload. Gagal uji = file lama tetap utuh.</li>
          </ul>
        {/if}
      </div>

    <!-- ═════════ TAB: GANTI VERSI ═════════ -->
    {:else if tab === 'version'}
      {#if !versions}
        <div class="loading"><i class="ri-loader-4-line spin"></i> Membaca repositori...</div>
      {:else}
        <div class="ver-row">
          <label for="ngx-ver-select">Ganti versi</label>
          <select id="ngx-ver-select" bind:value={selectedVersion} disabled={versions.job.running}>
            {#each versions.available as v}
              <option value={v}>nginx {v}{versions.current && v.startsWith(versions.current) ? '  ✓ aktif' : ''}</option>
            {/each}
          </select>
          <button id="ngx-ver-switch" class="btn btn-primary" disabled={switching || versions.job.running || !selectedVersion} onclick={switchVersion}>
            <i class={switching || versions.job.running ? 'ri-loader-4-line spin' : 'ri-swap-line'}></i> Ganti
          </button>
        </div>
        <p class="note">Versi aktif: <strong class="mono">{versions.current || 'tidak diketahui'}</strong> · {versions.available.length} versi tersedia di repositori.</p>

        {#if versions.job.target}
          <div class="job-card" class:running={versions.job.running} class:ok={!versions.job.running && versions.job.ok} class:fail={!versions.job.running && !versions.job.ok}>
            <div class="job-head">
              <i class={versions.job.running ? 'ri-loader-4-line spin' : versions.job.ok ? 'ri-checkbox-circle-fill' : 'ri-close-circle-fill'}></i>
              <strong>Job: nginx {versions.job.target}</strong>
              <span class="mono muted">{new Date(versions.job.started_at * 1000).toLocaleTimeString()}</span>
            </div>
            {#if versions.job.running}<div class="progress"><span></span></div>{/if}
            <pre class="job-msg">{versions.job.message}</pre>
          </div>
        {/if}
        <ul class="pref-list">
          <li>Instalasi berjalan di background; modal boleh ditutup dan dibuka lagi.</li>
          <li><code>nginx.conf</code> yang sudah dikustomisasi dipertahankan (dpkg confold).</li>
        </ul>
      {/if}

    <!-- ═════════ TAB: STATUS BEBAN ═════════ -->
    {:else if tab === 'load'}
      {#if !load}
        <div class="loading"><i class="ri-loader-4-line spin"></i> Mengambil metrik...</div>
      {:else}
        {#if !load.available}
          <div class="hint-box"><i class="ri-information-line"></i> <span>{load.hint}</span></div>
        {:else}
          <div class="load-grid">
            <div class="load-card big">
              <span class="stat-label">Koneksi aktif</span>
              <span class="load-num">{load.active}</span>
              <svg viewBox="0 0 300 60" preserveAspectRatio="none" class="spark">
                <path d={sparkPath(activeHistory)} />
              </svg>
            </div>
            <div class="load-card big">
              <span class="stat-label">Request / detik</span>
              <span class="load-num">{currentRps.toFixed(1)}</span>
              <svg viewBox="0 0 300 60" preserveAspectRatio="none" class="spark cyan">
                <path d={sparkPath(rpsHistory)} />
              </svg>
            </div>
          </div>
          <div class="stat-grid">
            <div class="stat"><span class="stat-label">Total accepts</span><span class="stat-value mono">{load.accepts.toLocaleString()}</span></div>
            <div class="stat"><span class="stat-label">Total handled</span><span class="stat-value mono">{load.handled.toLocaleString()}</span></div>
            <div class="stat"><span class="stat-label">Total requests</span><span class="stat-value mono">{load.requests.toLocaleString()}</span></div>
            <div class="stat"><span class="stat-label">Reading</span><span class="stat-value mono">{load.reading}</span></div>
            <div class="stat"><span class="stat-label">Writing</span><span class="stat-value mono">{load.writing}</span></div>
            <div class="stat"><span class="stat-label">Waiting</span><span class="stat-value mono">{load.waiting}</span></div>
          </div>
        {/if}
        <div class="stat-grid">
          <div class="stat"><span class="stat-label">Worker aktif</span><span class="stat-value mono">{load.workers || '—'}</span></div>
          <div class="stat"><span class="stat-label">Total RSS memori</span><span class="stat-value mono">{load.memory_bytes ? formatBytes(load.memory_bytes) : '—'}</span></div>
          <div class="stat"><span class="stat-label">Diperbarui</span><span class="stat-value mono">{new Date(load.timestamp * 1000).toLocaleTimeString()}</span></div>
        </div>
        <p class="note"><i class="ri-timer-line"></i> Auto-refresh tiap 3 detik selama tab ini terbuka.</p>
      {/if}

    <!-- ═════════ TAB: OPTIMASI ═════════ -->
    {:else if tab === 'optimize'}
      {#if !opt}
        <div class="loading"><i class="ri-loader-4-line spin"></i> Membaca directive...</div>
      {:else}
        <form class="opt-form" onsubmit={saveOptimize}>
          {#each optFields as f (f.key)}
            <label class="opt-row" for="opt-{f.key}">
              <span class="opt-name mono">{f.label}</span>
              {#if f.key === 'gzip'}
                <select id="opt-{f.key}" bind:value={opt.gzip}>
                  <option value={true}>Aktif (on)</option>
                  <option value={false}>Nonaktif (off)</option>
                </select>
              {:else if f.key === 'worker_processes'}
                <input id="opt-{f.key}" type="text" bind:value={opt.worker_processes} required />
              {:else}
                <input id="opt-{f.key}" type="number" min="0" bind:value={opt[f.key]} required />
              {/if}
              <span class="opt-hint">{#if f.unit}<b>{f.unit}</b> · {/if}{f.hint}</span>
            </label>
          {/each}
          {#if optError}<pre class="err-box"><i class="ri-close-circle-line"></i> {optError}</pre>{/if}
          <div class="conf-actions center">
            <button id="ngx-opt-save" type="submit" class="btn btn-primary" disabled={optSaving}>
              <i class={optSaving ? 'ri-loader-4-line spin' : 'ri-save-3-line'}></i> {optSaving ? 'Menerapkan...' : 'Simpan'}
            </button>
          </div>
        </form>
      {/if}

    <!-- ═════════ TAB: LOG ERROR ═════════ -->
    {:else if tab === 'errorlog'}
      <div class="log-wrap">
        <div class="log-toolbar">
          <select bind:value={logLines} onchange={loadErrorLog} aria-label="Jumlah baris">
            <option value={100}>100 baris</option>
            <option value={200}>200 baris</option>
            <option value={500}>500 baris</option>
            <option value={1000}>1000 baris</option>
          </select>
          <div class="log-search">
            <i class="ri-search-line"></i>
            <input id="ngx-log-filter" type="text" placeholder="Filter kata kunci..." bind:value={logFilter} />
          </div>
          <label class="auto-toggle">
            <input type="checkbox" bind:checked={logAuto} /> Auto-refresh 5d
          </label>
          <button class="btn btn-ghost sm" onclick={loadErrorLog}><i class="ri-refresh-line"></i> Refresh</button>
          <button id="ngx-log-clear" class="btn btn-danger sm" disabled={logClearing} onclick={clearErrorLog}>
            <i class="ri-delete-bin-6-line"></i> Kosongkan
          </button>
        </div>
        <div class="log-box" bind:this={logBox}>
          {#if !errLog}
            <div class="loading"><i class="ri-loader-4-line spin"></i> Memuat log...</div>
          {:else if logRows.length === 0}
            <div class="empty"><i class="ri-checkbox-circle-line"></i> {logFilter ? 'Tidak ada baris yang cocok' : 'Log error kosong'}</div>
          {:else}
            {#each logRows as row}
              <div class="log-line lvl-{row.level}">{row.text}</div>
            {/each}
          {/if}
        </div>
        {#if errLog}
          <div class="log-meta mono">{errLog.path} · {formatBytes(errLog.size)} · {logRows.length} baris ditampilkan</div>
        {/if}
      </div>
    {/if}
  {/snippet}
</ManagerModal>

<style>
  .mono { font-family: 'Fira Code', monospace; }
  .muted { color: var(--text-subtle); font-size: 0.72rem; }
  .spin { display: inline-block; animation: spin 0.9s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .loading, .empty {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--text-muted);
    font-size: 0.82rem;
    padding: 2rem 0;
    justify-content: center;
  }

  .state-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 0.2rem 0.55rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--danger-strong) 10%, transparent);
    color: var(--danger-strong);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
  }
  .state-pill .dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
  .state-pill.on { background: color-mix(in srgb, var(--accent) 10%, transparent); color: var(--accent); border-color: color-mix(in srgb, var(--accent) 25%, transparent); }
  .state-pill.on .dot { box-shadow: 0 0 0 0 currentColor; animation: pulse 1.8s infinite; }
  @keyframes pulse {
    0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--accent) 60%, transparent); }
    100% { box-shadow: 0 0 0 7px color-mix(in srgb, var(--accent) 0%, transparent); }
  }
  .mock-pill {
    font-size: 0.6rem;
    font-weight: 700;
    letter-spacing: 0.06em;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    color: var(--violet);
    background: color-mix(in srgb, var(--violet) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--violet) 25%, transparent);
  }

  .stat-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
    gap: 0.6rem;
    margin-bottom: 1rem;
  }
  .stat {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    padding: 0.75rem 0.9rem;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 10px;
    transition: border-color 0.15s ease, transform 0.15s ease;
  }
  .stat:hover { border-color: var(--border-strong); transform: translateY(-1px); }
  .stat-label { font-size: 0.66rem; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text-subtle); }
  .stat-value { font-size: 0.98rem; font-weight: 700; color: var(--text); }
  .stat-value.ok { color: var(--accent); }
  .stat-value.bad { color: var(--danger-strong); }

  .btn-row, .conf-actions { display: flex; gap: 0.55rem; flex-wrap: wrap; align-items: center; }
  .conf-actions { margin: 0.8rem 0; }
  .conf-actions.center { justify-content: center; }
  .btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.5rem 0.95rem;
    border-radius: 8px;
    font-size: 0.8rem;
    font-weight: 600;
    cursor: pointer;
    border: 1px solid transparent;
    transition: all 0.15s ease;
  }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn.sm { padding: 0.38rem 0.7rem; font-size: 0.74rem; }
  .btn-primary { background: linear-gradient(135deg, var(--accent), var(--accent-hover)); color: var(--text-on-accent); box-shadow: 0 4px 14px -4px color-mix(in srgb, var(--accent) 50%, transparent); }
  .btn-primary:not(:disabled):hover { transform: translateY(-1px); box-shadow: 0 6px 18px -4px color-mix(in srgb, var(--accent) 60%, transparent); }
  .btn-ghost { background: var(--bg-raised); color: var(--text-secondary); border-color: var(--border); }
  .btn-ghost:not(:disabled):hover { background: var(--bg-active); color: var(--text); }
  .btn-danger { background: color-mix(in srgb, var(--danger-strong) 10%, transparent); color: var(--danger-strong); border-color: color-mix(in srgb, var(--danger-strong) 28%, transparent); }
  .btn-danger:not(:disabled):hover { background: color-mix(in srgb, var(--danger-strong) 20%, transparent); }

  .note { font-size: 0.74rem; color: var(--text-muted); margin: 0.75rem 0 0; display: flex; gap: 0.35rem; align-items: center; }
  .note.warn { color: var(--warning); }
  code { font-family: 'Fira Code', monospace; font-size: 0.72rem; background: var(--bg-hover); padding: 0.05rem 0.3rem; border-radius: 4px; color: var(--info); }
  kbd { font-family: 'Fira Code', monospace; font-size: 0.66rem; background: var(--bg-active); border: 1px solid var(--border-strong); border-bottom-width: 2px; padding: 0 0.3rem; border-radius: 4px; color: var(--text-secondary); }
  .divider { height: 1px; background: var(--bg-active); margin: 1.3rem 0; }

  .pref-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 1.5rem; padding: 0.7rem 0; }
  .pref-title { font-size: 0.86rem; font-weight: 600; color: var(--text); }
  .pref-desc { font-size: 0.74rem; color: var(--text-muted); margin-top: 0.2rem; }
  .pref-list { margin: 0.35rem 0 0; padding-left: 1.1rem; font-size: 0.74rem; color: var(--text-muted); line-height: 1.7; }

  .switch {
    position: relative;
    flex-shrink: 0;
    width: 44px;
    height: 24px;
    border-radius: 999px;
    border: 1px solid var(--border-strong);
    background: var(--bg-active);
    cursor: pointer;
    transition: background 0.2s ease, border-color 0.2s ease;
  }
  .switch span {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--text-secondary);
    transition: transform 0.22s cubic-bezier(0.3, 1.4, 0.5, 1), background 0.2s ease;
  }
  .switch.on { background: var(--accent); border-color: var(--accent); }
  .switch.on span { transform: translateX(20px); background: var(--text); }
  .switch:disabled { opacity: 0.6; }

  /* Editor konfigurasi */
  .conf-wrap { display: flex; flex-direction: column; height: 100%; }
  .conf-top { display: flex; justify-content: space-between; align-items: center; gap: 1rem; margin-bottom: 0.6rem; flex-wrap: wrap; }
  .tips { font-size: 0.74rem; color: var(--text-muted); }
  .path { font-size: 0.7rem; color: var(--text-subtle); }
  .editor {
    flex: 1;
    min-height: 320px;
    display: flex;
    border: 1px solid var(--border);
    border-radius: 10px;
    background: var(--bg-inset);
    overflow: hidden;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  .editor:focus-within { border-color: color-mix(in srgb, var(--accent) 50%, transparent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 8%, transparent); }
  .editor.dirty { border-color: color-mix(in srgb, var(--warning) 45%, transparent); }
  .gutter {
    padding: 0.75rem 0.55rem;
    min-width: 3rem;
    text-align: right;
    font-family: 'Fira Code', monospace;
    font-size: 0.76rem;
    line-height: 1.55;
    color: var(--text-disabled);
    background: var(--bg-surface);
    border-right: 1px solid var(--border);
    overflow: hidden;
    user-select: none;
  }
  .editor textarea {
    flex: 1;
    resize: none;
    border: none;
    outline: none;
    padding: 0.75rem 0.9rem;
    background: transparent;
    color: var(--text-secondary);
    font-family: 'Fira Code', monospace;
    font-size: 0.76rem;
    line-height: 1.55;
    white-space: pre;
    overflow: auto;
    tab-size: 4;
  }
  .dirty-tag { font-size: 0.72rem; color: var(--warning); font-weight: 600; }
  .err-box {
    margin: 0.7rem 0 0;
    padding: 0.7rem 0.9rem;
    border-radius: 8px;
    background: color-mix(in srgb, var(--danger-strong) 7%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger-strong) 25%, transparent);
    color: var(--danger-strong);
    font-family: 'Fira Code', monospace;
    font-size: 0.72rem;
    white-space: pre-wrap;
    word-break: break-word;
  }

  /* Versi */
  .ver-row { display: flex; align-items: center; gap: 0.7rem; flex-wrap: wrap; }
  .ver-row label { font-size: 0.84rem; color: var(--text-secondary); font-weight: 600; }
  select, input[type='text'], input[type='number'] {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 8px;
    padding: 0.48rem 0.7rem;
    font-size: 0.8rem;
    outline: none;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  select:focus, input:focus { border-color: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent); }
  #ngx-ver-select { min-width: 240px; font-family: 'Fira Code', monospace; }
  .job-card { margin-top: 1rem; padding: 0.85rem 1rem; border-radius: 10px; border: 1px solid var(--border); background: var(--bg-raised); }
  .job-card.running { border-color: color-mix(in srgb, var(--info) 35%, transparent); }
  .job-card.ok { border-color: color-mix(in srgb, var(--accent) 35%, transparent); }
  .job-card.fail { border-color: color-mix(in srgb, var(--danger-strong) 35%, transparent); }
  .job-head { display: flex; align-items: center; gap: 0.5rem; font-size: 0.82rem; color: var(--text); }
  .job-card.running .job-head i { color: var(--info); }
  .job-card.ok .job-head i { color: var(--accent); }
  .job-card.fail .job-head i { color: var(--danger-strong); }
  .job-head .muted { margin-left: auto; }
  .job-msg { margin: 0.5rem 0 0; font-family: 'Fira Code', monospace; font-size: 0.72rem; color: var(--text-muted); white-space: pre-wrap; max-height: 180px; overflow: auto; }
  .progress { height: 3px; border-radius: 2px; background: var(--bg-active); overflow: hidden; margin-top: 0.6rem; }
  .progress span { display: block; height: 100%; width: 35%; background: linear-gradient(90deg, transparent, var(--info), transparent); animation: slide 1.2s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(300%); } }

  /* Beban */
  .hint-box {
    display: flex;
    gap: 0.6rem;
    padding: 0.85rem 1rem;
    border-radius: 10px;
    background: color-mix(in srgb, var(--info) 6%, transparent);
    border: 1px solid color-mix(in srgb, var(--info) 22%, transparent);
    color: var(--info);
    font-size: 0.78rem;
    line-height: 1.5;
    margin-bottom: 1rem;
  }
  .load-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.7rem; margin-bottom: 0.8rem; }
  .load-card {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding: 0.9rem 1rem 0;
    border-radius: 12px;
    border: 1px solid var(--border);
    background: radial-gradient(120% 100% at 0% 0%, color-mix(in srgb, var(--accent) 8%, transparent), transparent 60%), var(--bg-raised);
    overflow: hidden;
  }
  .load-num { font-size: 1.9rem; font-weight: 800; color: var(--text); font-family: 'Fira Code', monospace; letter-spacing: -0.02em; }
  .spark { width: calc(100% + 2rem); margin: 0.3rem -1rem 0; height: 60px; }
  .spark path { fill: none; stroke: var(--accent); stroke-width: 2; vector-effect: non-scaling-stroke; filter: drop-shadow(0 0 4px color-mix(in srgb, var(--accent) 50%, transparent)); }
  .spark.cyan path { stroke: var(--info); filter: drop-shadow(0 0 4px color-mix(in srgb, var(--info) 50%, transparent)); }

  /* Optimasi */
  .opt-form { display: flex; flex-direction: column; gap: 0.55rem; }
  .opt-row { display: grid; grid-template-columns: 230px 170px 1fr; align-items: center; gap: 0.9rem; }
  .opt-name { font-size: 0.78rem; color: var(--text); text-align: right; font-weight: 600; }
  .opt-hint { font-size: 0.72rem; color: var(--text-subtle); }
  .opt-hint b { color: var(--text-muted); }
  @media (max-width: 820px) {
    .opt-row { grid-template-columns: 1fr; gap: 0.3rem; }
    .opt-name { text-align: left; }
  }

  /* Log error */
  .log-wrap { display: flex; flex-direction: column; height: 100%; gap: 0.6rem; }
  .log-toolbar { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
  .log-search { position: relative; flex: 1; min-width: 180px; }
  .log-search i { position: absolute; left: 0.6rem; top: 50%; transform: translateY(-50%); color: var(--text-subtle); font-size: 0.85rem; }
  .log-search input { width: 100%; padding-left: 1.9rem; box-sizing: border-box; }
  .auto-toggle { display: flex; align-items: center; gap: 0.35rem; font-size: 0.74rem; color: var(--text-muted); cursor: pointer; }
  .auto-toggle input { accent-color: var(--accent); }
  .log-box {
    flex: 1;
    min-height: 300px;
    overflow: auto;
    padding: 0.7rem 0.9rem;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-app);
    font-family: 'Fira Code', monospace;
    font-size: 0.72rem;
    line-height: 1.6;
  }
  .log-line { white-space: pre-wrap; word-break: break-all; padding: 0.05rem 0.4rem; border-left: 2px solid transparent; color: var(--text-muted); }
  .log-line:hover { background: color-mix(in srgb, var(--text) 2%, transparent); }
  .lvl-emerg, .lvl-alert, .lvl-crit { color: var(--danger-strong); border-left-color: var(--danger-strong); background: color-mix(in srgb, var(--danger-strong) 5%, transparent); }
  .lvl-error { color: var(--danger-strong); border-left-color: var(--danger-strong); }
  .lvl-warn { color: var(--warning-strong); border-left-color: var(--warning); }
  .lvl-notice { color: var(--info); }
  .log-meta { font-size: 0.68rem; color: var(--text-disabled); }
</style>
