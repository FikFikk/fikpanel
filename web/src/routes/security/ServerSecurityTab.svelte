<script lang="ts">
  import type { ServerSecurityOverview } from '../../lib/types';
  import '../../styles/pages/security/server-security.css';

  interface Props {
    serverSec: ServerSecurityOverview | null;
    onApplySecurityFix: (actionKey: string) => void;
  }

  let { serverSec, onApplySecurityFix }: Props = $props();
</script>

<div class="pg-security-server-security tab-pane">
  <!-- Grid Atas: Rekap Login Terakhir SSH & Panel -->
  <div class="login-info-grid">
    <div class="card ssh-login-card">
      <div class="card__head">
        <span class="card__title">SSH login</span>
        <span class="badge badge--success">Success</span>
      </div>
      <div class="card__body">
        <p class="mono text-sm">IP: <strong class="text-accent">{serverSec?.last_ssh_login.ip || '127.0.0.1'}</strong></p>
        <p class="mono text-sm mt-1">Time: {serverSec?.last_ssh_login.time || '--'}</p>
        <p class="mono text-sm mt-1 text-subtle">Port: {serverSec?.last_ssh_login.port || 22}</p>
      </div>
    </div>

    <div class="card panel-login-card">
      <div class="card__head">
        <span class="card__title">Panel login</span>
        <span class="badge badge--success">Success</span>
      </div>
      <div class="card__body">
        <p class="mono text-sm">IP: <strong class="text-accent">{serverSec?.last_panel_login.ip || '127.0.0.1'}</strong></p>
        <p class="mono text-sm mt-1">Time: {serverSec?.last_panel_login.time || '--'}</p>
        <p class="mono text-sm mt-1 text-subtle">Port: {serverSec?.last_panel_login.port || 24441}</p>
      </div>
    </div>
  </div>

  <!-- Banner Skor Keselamatan (Safety rating: 40/100 Insecure / 85/100 Secure) -->
  <div class="safety-score-bar card">
    <div class="score-meta">
      <span class="safety-label">Safety rating:</span>
      <span class="score-display {serverSec && serverSec.score >= 80 ? 'text-accent' : 'text-danger'}">
        {serverSec?.score || 40} / 100
      </span>
      <span class="badge {serverSec && serverSec.score >= 80 ? 'badge--success' : 'badge--danger'}">
        {serverSec?.rating || 'Insecure'}
      </span>
    </div>
    <div>
      <button class="btn btn--primary btn--sm" onclick={() => onApplySecurityFix('change_ssh_port')}>
        <i class="ri-shield-check-line"></i> Security configuration
      </button>
    </div>
  </div>

  <!-- Grid 10 Poin Evaluasi Keamanan -->
  <div class="checklist-grid">
    {#each serverSec?.checklist || [] as item}
      <div class="card checklist-card" class:card-danger={item.status === 'danger'}>
        <div class="checklist-head">
          <div class="checklist-title-wrap">
            <i class="{item.status === 'safe' ? 'ri-checkbox-circle-fill text-accent' : 'ri-close-circle-fill text-danger'} check-icon"></i>
            <span class="check-title">{item.title}</span>
          </div>
          <span class="badge {item.status === 'safe' ? 'badge--success' : 'badge--danger'}">
            {item.status.toUpperCase()}
          </span>
        </div>
        <p class="check-desc">{item.description}</p>
        {#if item.fixable && item.status !== 'safe'}
          <div class="mt-3">
            <button class="btn btn--secondary btn--sm" onclick={() => onApplySecurityFix(item.action_key)}>
              Perbaiki Sekarang
            </button>
          </div>
        {/if}
      </div>
    {/each}
  </div>
</div>
