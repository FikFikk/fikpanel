<script lang="ts">
  import '../../styles/pages/security/tabs.css';

  // Tipe identifikasi tab modul keamanan
  export type SecurityTabId =
    | 'overview'
    | 'firewall'
    | 'ssh'
    | 'server_security'
    | 'bruteforce'
    | 'anti_intrusion'
    | 'scan_detection';

  interface Props {
    activeTab: SecurityTabId;
    onSelect: (tab: SecurityTabId) => void;
  }

  let { activeTab, onSelect }: Props = $props();
</script>

<!-- Tab Navigasi Atas (7 Modul Keamanan Tingkat Enterprise) -->
<nav class="pg-security-tabs tabs" aria-label="Tab Navigasi Keamanan">
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'overview'}
    onclick={() => onSelect('overview')}
  >
    Overview
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'firewall'}
    onclick={() => onSelect('firewall')}
  >
    Firewall
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'ssh'}
    onclick={() => onSelect('ssh')}
  >
    SSH
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'server_security'}
    onclick={() => onSelect('server_security')}
  >
    Server security
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'bruteforce'}
    onclick={() => onSelect('bruteforce')}
  >
    Brute force protection
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'anti_intrusion'}
    onclick={() => onSelect('anti_intrusion')}
  >
    Anti Intrusion & Hardening
  </button>
  <button
    class="tabs__tab"
    class:is-active={activeTab === 'scan_detection'}
    onclick={() => onSelect('scan_detection')}
  >
    Scan Detection
  </button>
</nav>

