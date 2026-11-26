<script lang="ts">
  import type { WAFRule, WAFAutoBanConfig, WAFPayloadTestResult } from '../../lib/types';
  import '../../styles/pages/waf/lists.css';

  // Props untuk tab Aturan Daftar Hitam/Putih, Auto-Ban, dan Simulator Payload WAF
  interface Props {
    activeRuleType: 'ip_whitelist' | 'ip_blacklist' | 'ua_whitelist' | 'ua_blacklist' | 'url_whitelist' | 'url_blacklist';
    ruleType: 'IPv4' | 'IPv6';
    ruleStartIP: string;
    ruleEndIP: string;
    rulePattern: string;
    ruleRemarks: string;
    submittingRule: boolean;
    loadingRules: boolean;
    rules: WAFRule[];
    autoban: WAFAutoBanConfig;
    savingAutoBan: boolean;
    simMethod: string;
    simPath: string;
    simUA: string;
    simIP: string;
    simHeader: string;
    simTesting: boolean;
    simResult: WAFPayloadTestResult | null;
    onLoadRules: () => void;
    onAddRule: () => void;
    onDeleteRule: (rule: WAFRule) => void;
    onSaveAutoBan: () => void;
    onRunPayloadTest: () => void;
  }

  let {
    activeRuleType = $bindable(),
    ruleType = $bindable(),
    ruleStartIP = $bindable(),
    ruleEndIP = $bindable(),
    rulePattern = $bindable(),
    ruleRemarks = $bindable(),
    submittingRule,
    loadingRules,
    rules,
    autoban = $bindable(),
    savingAutoBan,
    simMethod = $bindable(),
    simPath = $bindable(),
    simUA = $bindable(),
    simIP = $bindable(),
    simHeader = $bindable(),
    simTesting,
    simResult,
    onLoadRules,
    onAddRule,
    onDeleteRule,
    onSaveAutoBan,
    onRunPayloadTest,
  }: Props = $props();

  // Preset simulasi serangan umum dan eksploitasi CVE kritis
  const payloadPresets = [
    { label: 'PHP2Shell (CVE-2024-4577)', method: 'POST', path: '/index.php?%ADd+allow_url_include%3d1+%ADd+auto_prepend_file%3dphp://input', ua: 'curl/8.5', header: '' },
    { label: 'React2Shell (CVE-2025-55182)', method: 'POST', path: '/?_rsc=1', ua: 'Mozilla/5.0', header: 'Next-Action: a1b2c3d4' },
    { label: 'Next.js Bypass (CVE-2025-29927)', method: 'GET', path: '/admin', ua: 'Mozilla/5.0', header: 'x-middleware-subrequest: middleware:middleware' },
    { label: 'Laravel Ignition RCE', method: 'POST', path: '/_ignition/execute-solution', ua: 'Mozilla/5.0', header: '' },
    { label: 'SQL Injection', method: 'GET', path: "/login?id=1' UNION SELECT username,password FROM users--", ua: 'sqlmap/1.8', header: '' },
    { label: 'XSS Script', method: 'GET', path: '/search?q=<script>document.cookie<' + '/script>', ua: 'Mozilla/5.0', header: '' },
    { label: 'Traversal /etc/passwd', method: 'GET', path: '/download?file=../../../../etc/passwd', ua: 'Mozilla/5.0', header: '' },
    { label: 'Trafik Normal', method: 'GET', path: '/', ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)', header: '' },
  ];

  // Terapkan preset payload dan langsung uji
  function applyPreset(p: typeof payloadPresets[number]) {
    simMethod = p.method;
    simPath = p.path;
    simUA = p.ua;
    simHeader = p.header;
    onRunPayloadTest();
  }

  // Label Indonesia yang rapi untuk tipe aturan
  function ruleLabel(t: string): string {
    const map: Record<string, string> = {
      ip_whitelist: 'IP Whitelist',
      ip_blacklist: 'IP Blacklist',
      ua_whitelist: 'User-Agent Whitelist',
      ua_blacklist: 'User-Agent Blacklist',
      url_whitelist: 'URL Whitelist',
      url_blacklist: 'URL Blacklist',
    };
    return map[t] || t;
  }
</script>

<div class="pg-waf-lists">
  <!-- Bagian Atas: Auto-Ban Penyerang Berulang + Simulator Pengujian Payload WAF -->
  <div class="tools-two-col">
    <!-- Auto-Ban Engine -->
    <section class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-forbid-2-line text-danger"></i>
          <div>
            <h3>Auto-Ban Penyerang Berulang</h3>
            <p class="text-xs text-subtle">Masukkan IP ke Blacklist Nginx secara otomatis saat melewati ambang serangan</p>
          </div>
        </div>
        <label class="switch" aria-label="Aktifkan Auto-Ban WAF">
          <input type="checkbox" bind:checked={autoban.enabled} />
          <span class="slider"></span>
        </label>
      </div>

      <div class="autoban-grid">
        <div class="form-field">
          <label for="ab-threshold" class="field-label">Ambang Serangan</label>
          <input
            id="ab-threshold"
            type="number"
            min="2"
            max="500"
            bind:value={autoban.threshold}
            class="input-dark mono"
          />
        </div>
        <div class="form-field">
          <label for="ab-window" class="field-label">Jendela (Menit)</label>
          <input
            id="ab-window"
            type="number"
            min="1"
            max="1440"
            bind:value={autoban.window_minutes}
            class="input-dark mono"
          />
        </div>
        <div class="form-field">
          <label for="ab-duration" class="field-label">Durasi Catatan (Menit)</label>
          <input
            id="ab-duration"
            type="number"
            min="5"
            max="43200"
            bind:value={autoban.ban_minutes}
            class="input-dark mono"
          />
        </div>
      </div>

      <div class="autoban-checks">
        <label class="custom-chk">
          <input type="checkbox" bind:checked={autoban.exclude_private} />
          <span>Kecualikan IP lokal/privat (127.0.0.1, 10.x, 192.168.x) agar admin tidak terkunci</span>
        </label>
        <label class="custom-chk">
          <input type="checkbox" bind:checked={autoban.sync_ufw} />
          <span>Sinkronkan blokir langsung ke kernel firewall UFW (Linux)</span>
        </label>
      </div>

      <div class="autoban-footer">
        <span class="text-xs text-muted">
          IP dalam Daftar Putih (Whitelist) selalu dikecualikan dari Auto-Ban.
        </span>
        <button
          type="button"
          class="btn btn--primary btn--sm"
          onclick={onSaveAutoBan}
          disabled={savingAutoBan}
        >
          <i class="ri-save-line"></i> {savingAutoBan ? 'Menyimpan...' : 'Simpan Auto-Ban'}
        </button>
      </div>
    </section>

    <!-- Simulator Pengujian Payload WAF -->
    <section class="bento-panel">
      <div class="panel-header">
        <div class="panel-title-wrap">
          <i class="ri-flask-line text-accent"></i>
          <div>
            <h3>Simulator &amp; Penguji Payload WAF</h3>
            <p class="text-xs text-subtle">Verifikasi langsung apakah payload eksploitasi / CVE tertangkap oleh aturan &amp; Virtual Patch aktif</p>
          </div>
        </div>
      </div>

      <div class="preset-chips">
        {#each payloadPresets as preset}
          <button type="button" class="preset-chip" onclick={() => applyPreset(preset)}>
            {preset.label}
          </button>
        {/each}
      </div>

      <div class="sim-grid">
        <div class="form-field">
          <label for="sim-method" class="field-label">Method</label>
          <select id="sim-method" bind:value={simMethod} class="input-dark mono">
            <option value="GET">GET</option>
            <option value="POST">POST</option>
            <option value="PUT">PUT</option>
            <option value="DELETE">DELETE</option>
            <option value="CONNECT">CONNECT</option>
          </select>
        </div>
        <div class="form-field">
          <label for="sim-path" class="field-label">URL Path &amp; Query String</label>
          <input
            id="sim-path"
            type="text"
            bind:value={simPath}
            placeholder="/index.php?%ADd+allow_url_include=1"
            class="input-dark mono"
          />
        </div>
      </div>

      <div class="sim-grid-2">
        <div class="form-field">
          <label for="sim-header" class="field-label">Header Kustom (Opsional: Nama: Nilai)</label>
          <input
            id="sim-header"
            type="text"
            bind:value={simHeader}
            placeholder="x-middleware-subrequest: middleware"
            class="input-dark mono"
          />
        </div>
        <div class="form-field">
          <label for="sim-ua" class="field-label">User-Agent / IP Klien</label>
          <input
            id="sim-ua"
            type="text"
            bind:value={simUA}
            placeholder="Mozilla/5.0 atau sqlmap/1.8"
            class="input-dark mono"
          />
        </div>
      </div>

      <div class="sim-actions">
        <div class="form-field">
          <label for="sim-ip" class="field-label">IP Pengirim Simulasi</label>
          <input id="sim-ip" type="text" bind:value={simIP} class="input-dark mono" />
        </div>
        <button
          type="button"
          class="btn btn--primary btn--sm"
          onclick={onRunPayloadTest}
          disabled={simTesting}
        >
          <i class="ri-play-circle-line {simTesting ? 'spin' : ''}"></i>
          {simTesting ? 'Menguji...' : 'Uji Payload Sekarang'}
        </button>
      </div>

      {#if simResult}
        <div class="sim-result-box {simResult.blocked ? 'is-blocked' : 'is-allowed'}">
          <div class="sim-result-head">
            <span class="{simResult.blocked ? 'text-danger' : 'text-accent'}">
              <i class="{simResult.blocked ? 'ri-shield-cross-fill' : 'ri-shield-check-fill'}"></i>
              {simResult.blocked ? 'DIBLOKIR / DICEGAT WAF' : 'DIIZINKAN (PASS-THROUGH)'}
            </span>
            <span class="mono text-xs">{simResult.attack_type}</span>
          </div>
          <div class="text-xs text-secondary">
            Aturan pemicu: <strong>{simResult.rule_name}</strong>
            {#if simResult.matched_patch_id}
              · Patch: <code class="mono">{simResult.matched_patch_id}</code>
            {/if}
          </div>
        </div>
      {/if}
    </section>
  </div>

  <!-- Bagian Bawah: Daftar Hitam / Putih IP, User-Agent, dan URL -->
  <section class="bento-panel">
    <div class="rule-subtabs" role="tablist">
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'ip_blacklist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'ip_blacklist'; onLoadRules(); }}
      >
        IP Blacklist
      </button>
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'ip_whitelist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'ip_whitelist'; onLoadRules(); }}
      >
        IP Whitelist
      </button>
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'ua_blacklist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'ua_blacklist'; onLoadRules(); }}
      >
        User-Agent Blacklist
      </button>
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'ua_whitelist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'ua_whitelist'; onLoadRules(); }}
      >
        User-Agent Whitelist
      </button>
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'url_blacklist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'url_blacklist'; onLoadRules(); }}
      >
        URL Blacklist
      </button>
      <button
        type="button"
        class="rule-subtab-btn {activeRuleType === 'url_whitelist' ? 'active' : ''}"
        onclick={() => { activeRuleType = 'url_whitelist'; onLoadRules(); }}
      >
        URL Whitelist
      </button>
    </div>

    <div class="add-rule-box">
      <h4 class="form-box-title">
        Tambahkan Entri Baru ke {ruleLabel(activeRuleType)}
      </h4>

      {#if activeRuleType.startsWith('ip_')}
        <div class="form-row-grid">
          <div class="form-field">
            <span class="field-label">Versi IP</span>
            <div class="pill-group">
              <button
                type="button"
                class="pill-opt {ruleType === 'IPv4' ? 'active' : ''}"
                onclick={() => (ruleType = 'IPv4')}
              >
                IPv4
              </button>
              <button
                type="button"
                class="pill-opt {ruleType === 'IPv6' ? 'active' : ''}"
                onclick={() => (ruleType = 'IPv6')}
              >
                IPv6
              </button>
            </div>
          </div>

          <div class="form-field">
            <label for="start-ip" class="field-label">IP Tunggal / CIDR / Awal</label>
            <input
              id="start-ip"
              type="text"
              bind:value={ruleStartIP}
              placeholder="Contoh: 203.0.113.5 atau 198.51.100.0/24"
              class="input-dark mono"
            />
          </div>

          <div class="form-field">
            <label for="end-ip" class="field-label">IP Akhir (Opsional)</label>
            <input
              id="end-ip"
              type="text"
              bind:value={ruleEndIP}
              placeholder="Contoh: 203.0.113.254"
              class="input-dark mono"
            />
          </div>

          <div class="form-field">
            <label for="remarks" class="field-label">Keterangan</label>
            <input
              id="remarks"
              type="text"
              bind:value={ruleRemarks}
              placeholder="Alasan pemblokiran / pengecualian"
              class="input-dark"
            />
          </div>

          <div class="form-field">
            <button
              type="button"
              class="btn btn--primary btn--sm"
              onclick={onAddRule}
              disabled={submittingRule}
            >
              <i class="ri-add-line"></i> {submittingRule ? 'Menyimpan...' : 'Tambah'}
            </button>
          </div>
        </div>
      {:else}
        <div class="form-row-grid">
          <div class="form-field">
            <label for="pattern" class="field-label">
              Pola Pencocokan ({activeRuleType.includes('ua') ? 'User-Agent' : 'Path URL'})
            </label>
            <input
              id="pattern"
              type="text"
              bind:value={rulePattern}
              placeholder={activeRuleType.includes('ua') ? 'Contoh: sqlmap, python-requests, curl' : 'Contoh: /wp-login.php, /secret-admin'}
              class="input-dark mono"
            />
          </div>
          <div class="form-field">
            <label for="remarks-text" class="field-label">Keterangan</label>
            <input
              id="remarks-text"
              type="text"
              bind:value={ruleRemarks}
              placeholder="Catatan singkat"
              class="input-dark"
            />
          </div>
          <div class="form-field">
            <button
              type="button"
              class="btn btn--primary btn--sm"
              onclick={onAddRule}
              disabled={submittingRule}
            >
              <i class="ri-add-line"></i> {submittingRule ? 'Menyimpan...' : 'Tambah'}
            </button>
          </div>
        </div>
      {/if}
    </div>

    <div class="table-responsive rules-table-wrap">
      <table class="waf-main-table">
        <thead>
          <tr>
            {#if activeRuleType.startsWith('ip_')}
              <th>Alamat IP / CIDR / Rentang</th>
              <th>Versi</th>
            {:else}
              <th>Pola String</th>
              <th>Kategori</th>
            {/if}
            <th>Keterangan</th>
            <th class="text-right">Aksi</th>
          </tr>
        </thead>
        <tbody>
          {#if loadingRules}
            <tr>
              <td colspan="4" class="empty-cell">Memuat daftar aturan...</td>
            </tr>
          {:else if rules.length === 0}
            <tr>
              <td colspan="4" class="empty-cell">
                Belum ada entri pada {ruleLabel(activeRuleType)}.
              </td>
            </tr>
          {:else}
            {#each rules as r}
              <tr>
                {#if activeRuleType.startsWith('ip_')}
                  <td class="mono font-semibold">
                    {r.start_ip}{r.end_ip ? ` – ${r.end_ip}` : ''}
                  </td>
                  <td>
                    <span class="badge-soft">{r.ip_version}</span>
                  </td>
                {:else}
                  <td class="mono font-semibold text-accent">{r.pattern}</td>
                  <td><span class="badge-soft">{ruleLabel(r.rule_type)}</span></td>
                {/if}
                <td class="text-subtle">{r.remarks || '-'}</td>
                <td class="text-right">
                  <button type="button" class="btn-link text-danger" onclick={() => onDeleteRule(r)}>
                    Hapus
                  </button>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <div class="info-footer-note">
      <i class="ri-shield-check-line"></i>
      <span>Hierarki Keamanan: IP Whitelist selalu diprioritaskan di atas IP Blacklist, Virtual Patch, dan filter URL/UA. Semua perubahan langsung disinkronkan ke <code class="mono">waf-rules.conf</code> Nginx.</span>
    </div>
  </section>
</div>
