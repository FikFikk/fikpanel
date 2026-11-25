<script lang="ts">
  import '../../styles/pages/security/tabs.css';

  // Identitas 5 modul utama Security Center
  export type SecurityTabId =
    | 'overview'
    | 'firewall'
    | 'ssh'
    | 'bruteforce'
    | 'hardening';

  interface Props {
    activeTab: SecurityTabId;
    issuesCount: number;
    rulesCount: number;
    blacklistCount: number;
    f2bBannedCount: number;
    onSelect: (tab: SecurityTabId) => void;
  }

  let {
    activeTab,
    issuesCount,
    rulesCount,
    blacklistCount,
    f2bBannedCount,
    onSelect,
  }: Props = $props();
</script>

<!-- Navigasi 5 tab terpadu Security Center -->
<div class="pg-security-tabs tabs" role="tablist" aria-label="Navigasi Modul Keamanan">
  <button
    type="button"
    role="tab"
    class="tabs__tab"
    class:is-active={activeTab === 'overview'}
    aria-selected={activeTab === 'overview'}
    onclick={() => onSelect('overview')}
  >
    <i class="ri-shield-check-line" aria-hidden="true"></i>
    <span>Overview &amp; Audit</span>
    {#if issuesCount > 0}
      <span class="tab-count tab-count--danger">{issuesCount}</span>
    {/if}
  </button>

  <button
    type="button"
    role="tab"
    class="tabs__tab"
    class:is-active={activeTab === 'firewall'}
    aria-selected={activeTab === 'firewall'}
    onclick={() => onSelect('firewall')}
  >
    <i class="ri-fire-line" aria-hidden="true"></i>
    <span>Firewall (UFW)</span>
    {#if rulesCount > 0}
      <span class="tab-count">{rulesCount}</span>
    {/if}
  </button>

  <button
    type="button"
    role="tab"
    class="tabs__tab"
    class:is-active={activeTab === 'ssh'}
    aria-selected={activeTab === 'ssh'}
    onclick={() => onSelect('ssh')}
  >
    <i class="ri-terminal-box-line" aria-hidden="true"></i>
    <span>SSH &amp; Akses</span>
  </button>

  <button
    type="button"
    role="tab"
    class="tabs__tab"
    class:is-active={activeTab === 'bruteforce'}
    aria-selected={activeTab === 'bruteforce'}
    onclick={() => onSelect('bruteforce')}
  >
    <i class="ri-lock-password-line" aria-hidden="true"></i>
    <span>Brute Force &amp; IP</span>
    {#if blacklistCount > 0}
      <span class="tab-count tab-count--danger">{blacklistCount}</span>
    {/if}
  </button>

  <button
    type="button"
    role="tab"
    class="tabs__tab"
    class:is-active={activeTab === 'hardening'}
    aria-selected={activeTab === 'hardening'}
    onclick={() => onSelect('hardening')}
  >
    <i class="ri-radar-line" aria-hidden="true"></i>
    <span>Hardening &amp; Scanner</span>
    {#if f2bBannedCount > 0}
      <span class="tab-count">{f2bBannedCount}</span>
    {/if}
  </button>
</div>
