<script lang="ts">
  import { copyText } from '../../lib/utils/browser';
  import { notify } from '../../lib/stores/toast.svelte';
  import type { FirewallExtendedStatus, PortRule } from '../../lib/types';
  import '../../styles/pages/security/firewall.css';

  export type FirewallSubTab = 'inbound' | 'outbound' | 'settings';

  export interface RuleTypePreset {
    type: string;
    proto: string;
    port: string;
    desc: string;
  }

  interface Props {
    fwStatus: FirewallExtendedStatus | null;
    fwSubTab: FirewallSubTab;
    fwPortSearch: string;
    isEditingInbound: boolean;
    showActionsDropdown: boolean;
    filteredPortRules: PortRule[];
    draftRules: PortRule[];
    ruleTypePresets: RuleTypePreset[];
    detectTypeFromPort: (port: string, proto: string) => string;
    isPresetFixed: (type?: string) => boolean;
    isPresetFixedPort: (type?: string) => boolean;
    onSelectSubTab: (tab: FirewallSubTab) => void;
    onToggleActionsDropdown: () => void;
    onCloseActionsDropdown: () => void;
    onStartEditInbound: () => void;
    onCancelEditInbound: () => void;
    onToggleFirewall: () => void;
    onToggleICMP: () => void;
    onClearSiteLogs: () => void;
    onLoadFirewall: () => void;
    onDeletePortRule: (rule: PortRule) => void;
    onAddDraftRule: () => void;
    onRemoveDraftRule: (index: number) => void;
    onRuleTypeChange: (index: number, newType: string) => void;
    onSourceTypeChange: (index: number, newType: string) => void;
    onSaveDraftRules: () => void;
  }

  let {
    fwStatus,
    fwSubTab,
    fwPortSearch = $bindable(),
    isEditingInbound,
    showActionsDropdown,
    filteredPortRules,
    draftRules = $bindable(),
    ruleTypePresets,
    detectTypeFromPort,
    isPresetFixed,
    isPresetFixedPort,
    onSelectSubTab,
    onToggleActionsDropdown,
    onCloseActionsDropdown,
    onStartEditInbound,
    onCancelEditInbound,
    onToggleFirewall,
    onToggleICMP,
    onClearSiteLogs,
    onLoadFirewall,
    onDeletePortRule,
    onAddDraftRule,
    onRemoveDraftRule,
    onRuleTypeChange,
    onSourceTypeChange,
    onSaveDraftRules,
  }: Props = $props();
</script>

<div class="pg-security-firewall tab-pane">
  {#if !isEditingInbound}
    <!-- AWS EC2 Header Breadcrumb & Actions Bar -->
    <div class="aws-header-bar">
      <div class="aws-title-wrap">
        <h2 class="aws-main-title">sg-0645a2456451e6a9f - launch-wizard-fikpanel</h2>
      </div>
      <div class="aws-header-actions">
        <div class="dropdown-wrap">
          <button class="btn btn--secondary btn--sm aws-actions-btn" onclick={onToggleActionsDropdown}>
            Actions <i class="ri-arrow-down-s-line"></i>
          </button>
          {#if showActionsDropdown}
            <div class="dropdown-menu aws-dropdown-menu">
              <button class="dropdown-item" onclick={() => { onCloseActionsDropdown(); onStartEditInbound(); }}>
                <i class="ri-edit-line"></i> Edit inbound rules
              </button>
              <button class="dropdown-item" onclick={() => { onCloseActionsDropdown(); onToggleFirewall(); }}>
                <i class="ri-shield-flash-line"></i> {fwStatus?.enabled ? 'Disable UFW' : 'Enable UFW'}
              </button>
              <button class="dropdown-item" onclick={() => { onCloseActionsDropdown(); onToggleICMP(); }}>
                <i class="ri-radar-line"></i> {fwStatus?.block_icmp ? 'Allow ICMP Ping' : 'Block ICMP Ping'}
              </button>
              <button class="dropdown-item" onclick={() => { onCloseActionsDropdown(); onClearSiteLogs(); }}>
                <i class="ri-delete-bin-line"></i> Clear web logs
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>

    <!-- AWS EC2 Details Card -->
    <div class="card aws-details-card">
      <div class="aws-card-title">Details</div>
      <div class="aws-details-grid">
        <div class="aws-detail-item">
          <span class="aws-label">Security group name</span>
          <div class="aws-value-copy">
            <span class="aws-val-text font-bold">launch-wizard-fikpanel</span>
            <button class="copy-icon-btn" title="Copy name" onclick={async () => { await copyText('launch-wizard-fikpanel'); notify('Security group name copied', 'info'); }}>
              <i class="ri-file-copy-line"></i>
            </button>
          </div>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">Security group ID</span>
          <div class="aws-value-copy">
            <span class="aws-val-text mono font-semibold">sg-0645a2456451e6a9f</span>
            <button class="copy-icon-btn" title="Copy ID" onclick={async () => { await copyText('sg-0645a2456451e6a9f'); notify('Security group ID copied', 'info'); }}>
              <i class="ri-file-copy-line"></i>
            </button>
          </div>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">Description</span>
          <span class="aws-val-text text-muted">Linux Host Firewall (UFW) managed by FikPanel</span>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">VPC ID / Network Host</span>
          <span class="aws-val-text text-accent mono">vpc-0630356aa20ed371f (Host IP)</span>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">Owner</span>
          <div class="aws-value-copy">
            <span class="aws-val-text mono">691687039084</span>
            <button class="copy-icon-btn" title="Copy Owner" onclick={async () => { await copyText('691687039084'); notify('Owner ID copied', 'info'); }}>
              <i class="ri-file-copy-line"></i>
            </button>
          </div>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">Inbound rules count</span>
          <span class="aws-val-text font-bold text-accent">{fwStatus?.port_rules_count || 0} Permission entries</span>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">Outbound rules count</span>
          <span class="aws-val-text">1 Permission entry (All traffic allowed)</span>
        </div>

        <div class="aws-detail-item">
          <span class="aws-label">UFW Daemon Status</span>
          <span class="badge {fwStatus?.enabled ? 'badge--success' : 'badge--ghost'}">
            <span class="dot {fwStatus?.enabled ? 'dot--success' : ''}"></span>
            {fwStatus?.enabled ? 'Active (Running)' : 'Disabled'}
          </span>
        </div>
      </div>
    </div>

    <!-- AWS EC2 Sub-Navigation Tabs Bar -->
    <div class="tabs tabs--underline mb-4">
      <button
        class="tabs__tab"
        class:is-active={fwSubTab === 'inbound'}
        onclick={() => onSelectSubTab('inbound')}
      >
        Inbound rules
      </button>
      <button
        class="tabs__tab"
        class:is-active={fwSubTab === 'outbound'}
        onclick={() => onSelectSubTab('outbound')}
      >
        Outbound rules
      </button>
      <button
        class="tabs__tab"
        class:is-active={fwSubTab === 'settings'}
        onclick={() => onSelectSubTab('settings')}
      >
        Firewall Settings & Logs
      </button>
    </div>

    <!-- SUBTAB 1: Inbound rules Table -->
    {#if fwSubTab === 'inbound'}
      <div class="aws-table-container">
        <div class="aws-table-header-row">
          <div class="aws-table-title">
            Inbound rules ({filteredPortRules.length})
          </div>
          <div class="aws-table-actions">
            <div class="search-box">
              <input
                type="text"
                class="input input--sm aws-search-input"
                placeholder="Search rules, port, type..."
                bind:value={fwPortSearch}
              />
            </div>
            <button class="btn btn--ghost btn--icon" title="Refresh" onclick={onLoadFirewall}>
              <i class="ri-refresh-line"></i>
            </button>
            <button class="btn btn--primary btn--sm aws-edit-rules-btn" onclick={onStartEditInbound}>
              Edit inbound rules
            </button>
          </div>
        </div>

        <div class="table-wrap aws-table-wrap">
          <table class="table aws-table">
            <thead>
              <tr>
                <th class="col-check"><input type="checkbox" aria-label="Select all" /></th>
                <th>Name</th>
                <th>Security group rule ID</th>
                <th>IP version</th>
                <th>Type</th>
                <th>Protocol</th>
                <th>Port range</th>
                <th>Source</th>
                <th>Description</th>
                <th>Status</th>
                <th class="text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {#if filteredPortRules.length === 0}
                <tr>
                  <td colspan="11" class="text-center text-muted p-4">
                    Tidak ada aturan inbound yang sesuai dengan kriteria pencarian.
                  </td>
                </tr>
              {:else}
                {#each filteredPortRules as r (r.number)}
                  <tr>
                    <td><input type="checkbox" aria-label="Select row" /></td>
                    <td class="text-subtle">-</td>
                    <td class="mono font-semibold text-accent text-xs">
                      {r.rule_id || `sgr-${r.port}00000000`}
                    </td>
                    <td>IPv4</td>
                    <td>
                      <span class="badge badge--pill badge--ghost font-semibold">
                        {r.rule_type || detectTypeFromPort(r.port, r.protocol)}
                      </span>
                    </td>
                    <td class="mono uppercase font-semibold">{r.protocol}</td>
                    <td class="mono font-bold text-white">{r.port}</td>
                    <td class="mono text-muted">{r.source_ip || '0.0.0.0/0'}</td>
                    <td class="text-subtle text-truncate col-remarks" title={r.remarks}>
                      {r.remarks || '-'}
                    </td>
                    <td>
                      <span class="badge {r.status.includes('Listening') ? 'badge--success' : 'badge--ghost'}">
                        <span class="dot {r.status.includes('Listening') ? 'dot--success' : ''}"></span>
                        {r.status}
                      </span>
                    </td>
                    <td class="text-right">
                      <button class="btn btn--ghost btn--sm text-danger" onclick={() => onDeletePortRule(r)}>
                        Delete
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- SUBTAB 2: Outbound rules Table -->
    {#if fwSubTab === 'outbound'}
      <div class="aws-table-container">
        <div class="aws-table-header-row">
          <div class="aws-table-title">
            Outbound rules (1)
          </div>
        </div>

        <div class="table-wrap aws-table-wrap">
          <table class="table aws-table">
            <thead>
              <tr>
                <th>Security group rule ID</th>
                <th>IP version</th>
                <th>Type</th>
                <th>Protocol</th>
                <th>Port range</th>
                <th>Destination</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              <tr>
                <td class="mono font-semibold text-accent text-xs">sgr-0938bfe43810c9e71</td>
                <td>IPv4</td>
                <td><span class="badge badge--pill badge--ghost">All traffic</span></td>
                <td class="mono uppercase">All</td>
                <td class="mono">All</td>
                <td class="mono">0.0.0.0/0</td>
                <td class="text-subtle">Allow all outbound IPv4 traffic</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <!-- SUBTAB 3: Firewall Settings & Logs -->
    {#if fwSubTab === 'settings'}
      <div class="card p-4">
        <h4 class="font-bold text-white mb-3">Pengaturan Tingkat Lanjut UFW & Logging</h4>
        <div class="fw-controls-row">
          <div class="fw-toggle-group">
            <span class="fw-control-label">Status Firewall UFW</span>
            <label class="switch" aria-label="Toggle Firewall UFW">
              <input
                type="checkbox"
                role="switch"
                checked={fwStatus ? fwStatus.enabled : false}
                onchange={onToggleFirewall}
              />
              <span class="switch__track"></span>
            </label>
          </div>

          <div class="fw-divider"></div>

          <div class="fw-toggle-group">
            <span class="fw-control-label">Blokir Ping ICMP Echo</span>
            <label class="switch" aria-label="Toggle Block ICMP Ping">
              <input
                type="checkbox"
                role="switch"
                checked={fwStatus ? fwStatus.block_icmp : false}
                onchange={onToggleICMP}
              />
              <span class="switch__track"></span>
            </label>
          </div>

          <div class="fw-divider"></div>

          <div class="fw-logs-group">
            <span class="fw-control-label">Berkas Log Web:</span>
            <span class="logs-path mono">{fwStatus?.site_logs_path || '/var/log/nginx'}</span>
            <span class="logs-size badge badge--info">{fwStatus?.site_logs_size || '0 KB'}</span>
            <button class="btn btn--secondary btn--sm" onclick={onClearSiteLogs}>Bersihkan Log</button>
          </div>
        </div>
      </div>
    {/if}

  {:else}
    <!-- AWS EC2: EDIT INBOUND RULES FULL-VIEW MULTI-ROW BUILDER -->
    <div class="aws-edit-view">
      <div class="aws-edit-header">
        <h2 class="aws-edit-title">Edit inbound rules <span class="aws-info-link">Info</span></h2>
        <p class="aws-edit-sub">
          Inbound rules control the incoming traffic that's allowed to reach the instance.
        </p>
      </div>

      <div class="card aws-edit-card">
        <div class="aws-card-top-head">
          <span class="aws-card-heading">Inbound rules</span>
          <span class="aws-info-link">Info</span>
        </div>

        <div class="table-wrap aws-edit-table-wrap">
          <table class="aws-edit-table">
            <thead>
              <tr>
                <th class="col-w-170">Security group rule ID</th>
                <th class="col-w-180">Type <span class="aws-info-link">Info</span></th>
                <th class="col-w-100">Protocol <span class="aws-info-link">Info</span></th>
                <th class="col-w-110">Port range <span class="aws-info-link">Info</span></th>
                <th class="col-w-130">Source type <span class="aws-info-link">Info</span></th>
                <th class="col-w-170">Source <span class="aws-info-link">Info</span></th>
                <th class="col-w-200">Description - optional <span class="aws-info-link">Info</span></th>
                <th class="col-act-90"></th>
              </tr>
            </thead>
            <tbody>
              {#each draftRules as rule, i}
                <tr class="aws-edit-row">
                  <!-- Rule ID -->
                  <td>
                    <span class="mono text-muted text-xs">{rule.rule_id || '-'}</span>
                  </td>

                  <!-- Type Preset Dropdown -->
                  <td>
                    <select
                      class="input input--sm aws-input-select"
                      value={rule.rule_type || detectTypeFromPort(rule.port, rule.protocol)}
                      onchange={(e) => onRuleTypeChange(i, (e.target as HTMLSelectElement).value)}
                    >
                      {#each ruleTypePresets as p}
                        <option value={p.type}>{p.type}</option>
                      {/each}
                    </select>
                  </td>

                  <!-- Protocol -->
                  <td>
                    <input
                      type="text"
                      class="input input--sm mono aws-input-disabled"
                      value={rule.protocol.toUpperCase()}
                      disabled={isPresetFixed(rule.rule_type)}
                      readonly={isPresetFixed(rule.rule_type)}
                    />
                  </td>

                  <!-- Port range -->
                  <td>
                    <input
                      type="text"
                      class="input input--sm mono"
                      class:aws-input-disabled={isPresetFixedPort(rule.rule_type)}
                      bind:value={rule.port}
                      placeholder="22, 80..."
                      disabled={isPresetFixedPort(rule.rule_type)}
                      readonly={isPresetFixedPort(rule.rule_type)}
                    />
                  </td>

                  <!-- Source type -->
                  <td>
                    <select
                      class="input input--sm aws-input-select"
                      onchange={(e) => onSourceTypeChange(i, (e.target as HTMLSelectElement).value)}
                    >
                      <option value="Anywhere-IPv4" selected={rule.source_ip === '0.0.0.0/0' || rule.source_ip === 'All'}>Anywhere-IPv4</option>
                      <option value="Custom" selected={rule.source_ip !== '0.0.0.0/0' && rule.source_ip !== 'All'}>Custom</option>
                      <option value="My IP">My IP</option>
                    </select>
                  </td>

                  <!-- Source Input with Badge Chip -->
                  <td>
                    <div class="aws-source-input-wrap">
                      <div class="source-chip-box">
                        <span class="source-chip">
                          {rule.source_ip || '0.0.0.0/0'}
                          <button
                            type="button"
                            class="chip-remove"
                            onclick={() => (rule.source_ip = '0.0.0.0/0')}
                            title="Reset"
                          >
                            &times;
                          </button>
                        </span>
                      </div>
                    </div>
                  </td>

                  <!-- Description -->
                  <td>
                    <input
                      type="text"
                      class="input input--sm"
                      bind:value={rule.remarks}
                      placeholder="Keterangan aturan port..."
                    />
                  </td>

                  <!-- Delete Row Button -->
                  <td class="text-right">
                    <button
                      type="button"
                      class="btn btn--sm aws-btn-delete"
                      onclick={() => onRemoveDraftRule(i)}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <!-- Add Rule Button -->
        <div class="aws-add-rule-row">
          <button type="button" class="btn btn--sm aws-btn-add-rule" onclick={onAddDraftRule}>
            <i class="ri-add-line"></i> Add rule
          </button>
        </div>
      </div>

      <!-- Alert Callout -->
      <div class="aws-warning-banner">
        <i class="ri-alert-line warning-icon"></i>
        <span class="warning-text">
          <strong>Rules with source of 0.0.0.0/0 or ::/0 allow all IP addresses to access your instance.</strong>
          We recommend setting security group rules to allow access from known IP addresses only.
        </span>
      </div>

      <!-- Sticky Bottom Actions -->
      <div class="aws-edit-footer-actions">
        <button type="button" class="btn btn--secondary" onclick={onCancelEditInbound}>
          Cancel
        </button>
        <button type="button" class="btn btn--primary aws-save-rules-btn" onclick={onSaveDraftRules}>
          Save rules
        </button>
      </div>
    </div>
  {/if}
</div>

