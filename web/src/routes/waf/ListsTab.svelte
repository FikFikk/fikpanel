<script lang="ts">
  import type { WAFRule } from '../../lib/types';
  import '../../styles/pages/waf/lists.css';

  // Props untuk tab Black/White List WAF
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
    onLoadRules: () => void;
    onAddRule: () => void;
    onDeleteRule: (rule: WAFRule) => void;
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
    onLoadRules,
    onAddRule,
    onDeleteRule,
  }: Props = $props();
</script>

<div class="pg-waf-lists bento-panel">
  <!-- Sub-tabs Daftar Hitam/Putih -->
  <div class="rule-subtabs">
    <button
      class="rule-subtab-btn {activeRuleType === 'ip_whitelist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'ip_whitelist'; onLoadRules(); }}
    >
      IP whitelist
    </button>
    <button
      class="rule-subtab-btn {activeRuleType === 'ip_blacklist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'ip_blacklist'; onLoadRules(); }}
    >
      IP blacklist
    </button>
    <button
      class="rule-subtab-btn {activeRuleType === 'ua_whitelist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'ua_whitelist'; onLoadRules(); }}
    >
      UA whitelist
    </button>
    <button
      class="rule-subtab-btn {activeRuleType === 'ua_blacklist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'ua_blacklist'; onLoadRules(); }}
    >
      UA blacklist
    </button>
    <button
      class="rule-subtab-btn {activeRuleType === 'url_whitelist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'url_whitelist'; onLoadRules(); }}
    >
      URL whitelist
    </button>
    <button
      class="rule-subtab-btn {activeRuleType === 'url_blacklist' ? 'active' : ''}"
      onclick={() => { activeRuleType = 'url_blacklist'; onLoadRules(); }}
    >
      URL blacklist
    </button>
  </div>

  <!-- Form Tambah Aturan (Screenshot 5 Form Box) -->
  <div class="add-rule-box">
    <h4 class="form-box-title">
      Tambahkan entri baru ke {activeRuleType.replace('_', ' ').toUpperCase()}
    </h4>

    {#if activeRuleType.startsWith('ip_')}
      <div class="form-row-grid">
        <div class="form-field">
          <span class="field-label">Type</span>
          <div class="pill-group">
            <button
              type="button"
              class="pill-opt {ruleType === 'IPv4' ? 'active' : ''}"
              onclick={() => ruleType = 'IPv4'}
            >
              IPv4
            </button>
            <button
              type="button"
              class="pill-opt {ruleType === 'IPv6' ? 'active' : ''}"
              onclick={() => ruleType = 'IPv6'}
            >
              IPv6
            </button>
          </div>
        </div>

        <div class="form-field">
          <label for="start-ip" class="field-label">Start IP / Single IP</label>
          <input
            id="start-ip"
            type="text"
            bind:value={ruleStartIP}
            placeholder="Contoh: 192.168.1.1 atau 203.0.113.5"
            class="input-dark mono"
          />
        </div>

        <div class="form-field">
          <label for="end-ip" class="field-label">End IP (Opsional untuk rentang)</label>
          <input
            id="end-ip"
            type="text"
            bind:value={ruleEndIP}
            placeholder="Contoh: 192.168.1.254"
            class="input-dark mono"
          />
        </div>

        <div class="form-field">
          <label for="remarks" class="field-label">Remarks / Keterangan</label>
          <input
            id="remarks"
            type="text"
            bind:value={ruleRemarks}
            placeholder="Catatan alasan aturan"
            class="input-dark"
          />
        </div>

        <div class="form-field btn-cell">
          <button
            class="btn btn-primary"
            onclick={onAddRule}
            disabled={submittingRule}
          >
            {submittingRule ? 'Menyimpan...' : 'Add'}
          </button>
        </div>
      </div>
    {:else}
      <!-- Form untuk User-Agent atau URL -->
      <div class="form-row-grid">
        <div class="form-field col-span-2">
          <label for="pattern" class="field-label">Pola / String Pencocokan ({activeRuleType.includes('ua') ? 'User-Agent' : 'URL Path'})</label>
          <input
            id="pattern"
            type="text"
            bind:value={rulePattern}
            placeholder={activeRuleType.includes('ua') ? 'Contoh: Googlebot, curl, python-requests' : 'Contoh: /wp-admin, /api/internal'}
            class="input-dark mono"
          />
        </div>
        <div class="form-field">
          <label for="remarks-text" class="field-label">Remarks</label>
          <input
            id="remarks-text"
            type="text"
            bind:value={ruleRemarks}
            placeholder="Catatan"
            class="input-dark"
          />
        </div>
        <div class="form-field btn-cell">
          <button
            class="btn btn-primary"
            onclick={onAddRule}
            disabled={submittingRule}
          >
            {submittingRule ? 'Menyimpan...' : 'Add'}
          </button>
        </div>
      </div>
    {/if}
  </div>

  <!-- Tabel Daftar Aturan Aktif -->
  <div class="table-responsive mt-6">
    <table class="waf-main-table">
      <thead>
        <tr>
          {#if activeRuleType.startsWith('ip_')}
            <th>Start IP - End IP / IPv6 address</th>
            <th>Type</th>
          {:else}
            <th>Pattern String</th>
            <th>Target</th>
          {/if}
          <th>Remarks</th>
          <th class="text-right">Operate</th>
        </tr>
      </thead>
      <tbody>
        {#if loadingRules}
          <tr>
            <td colspan="4" class="text-center p-4 text-muted">Memuat daftar aturan...</td>
          </tr>
        {:else if rules.length === 0}
          <tr>
            <td colspan="4" class="text-center p-6 text-muted">
              Belum ada entri aturan untuk kategori {activeRuleType.replace('_', ' ').toUpperCase()}.
            </td>
          </tr>
        {:else}
          {#each rules as r}
            <tr>
              {#if activeRuleType.startsWith('ip_')}
                <td class="mono font-semibold">
                  {r.start_ip} {r.end_ip ? `- ${r.end_ip}` : ''}
                </td>
                <td>
                  <span class="badge-soft">{r.ip_version}</span>
                </td>
              {:else}
                <td class="mono font-semibold text-accent">{r.pattern}</td>
                <td><span class="badge-soft">{r.rule_type.toUpperCase()}</span></td>
              {/if}
              <td class="text-subtle">{r.remarks || '-'}</td>
              <td class="text-right">
                <button class="btn-link text-danger" onclick={() => onDeleteRule(r)}>
                  Delete
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Guide Rules Note -->
  <div class="info-footer-note mt-6">
    <i class="ri-shield-check-line"></i>
    <span>Hierarki Keamanan: IP Whitelist memiliki prioritas absolut tertinggi di atas Blacklist dan filter URL. Masukkan IP terpercaya pengelola ke whitelist untuk menghindari penguncian tak sengaja.</span>
  </div>
</div>
