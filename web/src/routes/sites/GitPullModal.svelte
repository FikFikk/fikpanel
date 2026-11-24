<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import { apiFetch } from '../../lib/api';
  import { copyText } from '../../lib/utils/browser';
  import { confirmDialog } from '../../lib/stores/confirm.svelte';
  import type { Site, SiteDeployInfo, SiteDeploymentItem } from '../../lib/types';
  import '../../styles/pages/sites/git-pull-modal.css';

  export interface GitPullSnippet {
    label: string;
    cmd: string;
    phpOnly?: boolean;
  }

  export interface GitPullResultData {
    ok?: boolean;
    domain?: string;
    branch?: string;
    repo?: string;
    old_commit?: string;
    new_commit: string;
    commit_message?: string;
    updated: boolean;
    duration_ms?: number;
    output?: string;
  }

  // Antarmuka properti untuk modal Git Pull, Webhook Auto-Deploy, & Rollback
  interface Props {
    open: boolean;
    gitPullTarget: Site | null;
    gitPullCustomCmd: string;
    gitPullRunning: boolean;
    gitPullError: string;
    gitPullResult: GitPullResultData | null;
    gitPullSnippets: GitPullSnippet[];
    hasGitSnippet: (cmd: string) => boolean;
    onClose: () => void;
    onResetGitPullCmd: () => void;
    onToggleGitSnippet: (cmd: string) => void;
    onExecuteGitPull: (opts?: { auto_optimize?: boolean; maintenance_mode?: boolean; rebuild_assets?: boolean }) => void;
    onNotify: (msg: string) => void;
    onSiteUpdated: () => void;
    getProjectRootDir: (s: Site) => string;
  }

  let {
    open,
    gitPullTarget,
    gitPullCustomCmd = $bindable(),
    gitPullRunning,
    gitPullError,
    gitPullResult,
    gitPullSnippets,
    hasGitSnippet,
    onClose,
    onResetGitPullCmd,
    onToggleGitSnippet,
    onExecuteGitPull,
    onNotify,
    onSiteUpdated,
    getProjectRootDir,
  }: Props = $props();

  let subTab = $state<'pull' | 'webhook' | 'history'>('pull');
  let deployInfo = $state<SiteDeployInfo | null>(null);
  let loadingInfo = $state(false);
  let savingCfg = $state(false);
  let rollingBackCommit = $state('');
  let expandedLogId = $state<number | null>(null);

  let optAutoOptimize = $state(true);
  let optMaintenance = $state(true);
  let optRebuildAssets = $state(true);
  let webhookEnabled = $state(false);

  // Muat konfigurasi webhook & riwayat deployment setiap kali modal dibuka untuk situs tertentu
  async function loadDeployInfo(siteId: number) {
    loadingInfo = true;
    try {
      const res = await apiFetch<SiteDeployInfo>(`/api/sites/${siteId}/deploy`);
      deployInfo = res;
      if (res?.config) {
        optAutoOptimize = Boolean(res.config.auto_optimize);
        optMaintenance = Boolean(res.config.maintenance_mode);
        optRebuildAssets = Boolean(res.config.rebuild_assets);
        webhookEnabled = Boolean(res.config.webhook_enabled);
      }
    } catch {
      // Abaikan jika gagal memuat metadata deploy tambahan
    } finally {
      loadingInfo = false;
    }
  }

  $effect(() => {
    if (open && gitPullTarget?.id) {
      subTab = 'pull';
      expandedLogId = null;
      loadDeployInfo(gitPullTarget.id);
    }
  });

  // Segarkan riwayat deploy otomatis setelah pull selesai
  $effect(() => {
    if (gitPullResult && gitPullTarget?.id) {
      loadDeployInfo(gitPullTarget.id);
    }
  });

  async function saveDeployConfig(rotateSecret = false) {
    if (!gitPullTarget || savingCfg) return;
    if (rotateSecret) {
      const ok = await confirmDialog({
        title: 'Rotasi Secret Webhook?',
        message: 'URL dan secret lama akan langsung tidak berlaku. Anda perlu memperbarui konfigurasi Webhook di GitHub/GitLab.',
        confirmText: 'Rotasi Secret',
        tone: 'danger',
      });
      if (!ok) return;
    }
    savingCfg = true;
    try {
      const res = await apiFetch<{ ok: boolean; webhook_url: string; config: SiteDeployInfo['config']; message: string }>(
        `/api/sites/${gitPullTarget.id}/deploy/config`,
        {
          method: 'PUT',
          body: JSON.stringify({
            webhook_enabled: webhookEnabled,
            auto_optimize: optAutoOptimize,
            maintenance_mode: optMaintenance,
            rebuild_assets: optRebuildAssets,
            rotate_secret: rotateSecret,
          }),
        }
      );
      if (deployInfo) {
        deployInfo = { ...deployInfo, config: res.config, webhook_url: res.webhook_url };
      }
      onNotify(rotateSecret ? 'Secret Webhook baru dibuat' : (res.message || 'Konfigurasi deploy disimpan'));
    } catch (err: any) {
      onNotify(`Gagal menyimpan konfigurasi: ${err?.message || 'error'}`);
    } finally {
      savingCfg = false;
    }
  }

  async function handleRollback(item: SiteDeploymentItem) {
    if (!gitPullTarget || rollingBackCommit || !item.new_commit) return;
    const ok = await confirmDialog({
      title: `Rollback ke commit #${item.new_commit}?`,
      message: `Kode ${gitPullTarget.domain} akan dikembalikan ke commit #${item.new_commit} dengan .env lokal tetap aman, lalu cache Laravel/Filament dioptimasi ulang.`,
      confirmText: 'Rollback Sekarang',
      tone: 'danger',
    });
    if (!ok) return;
    rollingBackCommit = item.new_commit;
    try {
      const res = await apiFetch<{ ok: boolean; message?: string }>(`/api/sites/${gitPullTarget.id}/deploy/rollback`, {
        method: 'POST',
        body: JSON.stringify({ commit: item.new_commit, run_composer: true }),
        timeoutMs: 300_000,
      });
      onNotify(res.message || `Rollback ke #${item.new_commit} berhasil`);
      await loadDeployInfo(gitPullTarget.id);
      onSiteUpdated();
    } catch (err: any) {
      onNotify(`Rollback gagal: ${err?.message || 'kesalahan jaringan'}`);
    } finally {
      rollingBackCommit = '';
    }
  }

  async function copyQuick(val: string, label: string) {
    if (!val) return;
    try {
      await copyText(val);
      onNotify(`${label} disalin ke clipboard`);
    } catch {
      onNotify(`Gagal menyalin ${label}`);
    }
  }

  function fmtDeployTime(unixSec: number): string {
    if (!unixSec) return '-';
    return new Date(unixSec * 1000).toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    });
  }
</script>

<Modal
  {open}
  title={gitPullTarget ? `Git Deploy, Webhook & Rollback: ${gitPullTarget.domain}` : 'Git Deploy'}
  size="lg"
  onclose={onClose}
>
  {#if gitPullTarget}
    <div class="pg-sites-git-pull-modal">
      <!-- Ringkasan Repositori & Branch Aktif -->
      <div class="git-summary-strip mono">
        <div class="git-sum-item">
          <span class="git-sum-lbl">Repositori</span>
          <strong><i class="ri-github-fill" aria-hidden="true"></i> {gitPullTarget.git_repo || 'origin'}</strong>
        </div>
        <div class="git-sum-item">
          <span class="git-sum-lbl">Branch / PHP CLI</span>
          <strong class="git-branch-highlight">
            <i class="ri-git-branch-line" aria-hidden="true"></i> {gitPullTarget.git_branch || 'main'}
            {#if deployInfo?.php_binary && gitPullTarget.kind === 'php'}
              · {deployInfo.php_binary}
            {/if}
          </strong>
        </div>
        <div class="git-sum-item">
          <span class="git-sum-lbl">Commit Aktif</span>
          <strong>#{deployInfo?.current_commit || gitPullTarget.git_commit || 'HEAD'}</strong>
        </div>
      </div>

      <!-- Navigasi Sub-Tab -->
      <div class="git-modal-tabs">
        <button
          type="button"
          class="git-modal-tab"
          class:is-active={subTab === 'pull'}
          onclick={() => (subTab = 'pull')}
        >
          <i class="ri-git-pull-request-line" aria-hidden="true"></i>
          <span>Pull &amp; Deploy</span>
        </button>
        <button
          type="button"
          class="git-modal-tab"
          class:is-active={subTab === 'webhook'}
          onclick={() => (subTab = 'webhook')}
        >
          <i class="ri-webhook-line" aria-hidden="true"></i>
          <span>Webhook Push-to-Deploy</span>
          {#if webhookEnabled}
            <span class="badge badge--success badge--sm">ON</span>
          {/if}
        </button>
        <button
          type="button"
          class="git-modal-tab"
          class:is-active={subTab === 'history'}
          onclick={() => (subTab = 'history')}
        >
          <i class="ri-history-line" aria-hidden="true"></i>
          <span>Riwayat &amp; Rollback ({deployInfo?.deployments?.length ?? 0})</span>
        </button>
      </div>

      <!-- ═══ SUB-TAB 1: PULL & DEPLOY ═══ -->
      {#if subTab === 'pull'}
        <div class="git-cmd-section">
          <div class="git-opt-row">
            <label class="git-opt-check" for="gp-opt-maint">
              <input id="gp-opt-maint" type="checkbox" bind:checked={optMaintenance} disabled={gitPullRunning} />
              <span>Maintenance Mode (<code class="mono">artisan down/up</code>)</span>
            </label>
            <label class="git-opt-check" for="gp-opt-opt">
              <input id="gp-opt-opt" type="checkbox" bind:checked={optAutoOptimize} disabled={gitPullRunning} />
              <span>Optimize Produksi (<code class="mono">optimize</code> + <code class="mono">filament:optimize</code>)</span>
            </label>
            <label class="git-opt-check" for="gp-opt-assets">
              <input id="gp-opt-assets" type="checkbox" bind:checked={optRebuildAssets} disabled={gitPullRunning} />
              <span>Rebuild Vite Otomatis (Anti-OOM 768 MB)</span>
            </label>
          </div>

          <div class="git-cmd-label-row">
            <span class="git-cmd-label">
              <i class="ri-terminal-box-line" aria-hidden="true"></i>
              Perintah Kustom Opsional (Kosongkan untuk Pipeline Otomatis Aman)
            </span>
            {#if gitPullCustomCmd.trim()}
              <button
                type="button"
                class="git-chip-reset"
                onclick={onResetGitPullCmd}
                disabled={gitPullRunning}
                title="Kosongkan untuk memakai pipeline otomatis"
              >
                Gunakan Pipeline Otomatis
              </button>
            {/if}
          </div>

          <div class="git-cmd-input-wrap">
            <span class="git-cmd-prompt mono">$</span>
            <input
              type="text"
              class="git-cmd-input mono"
              bind:value={gitPullCustomCmd}
              disabled={gitPullRunning}
              placeholder="Otomatis: down → git pull → composer → build Vite → migrate → optimize → up"
            />
          </div>

          <!-- Tombol Chip Tambah/Hapus Perintah Cepat -->
          <div class="git-chips-bar">
            <span class="git-chips-hint">Atau susun perintah kustom:</span>
            {#each gitPullSnippets.filter((sn) => !sn.phpOnly || gitPullTarget?.kind === 'php') as sn}
              <button
                type="button"
                class="git-snippet-chip mono {hasGitSnippet(sn.cmd) ? 'is-active' : ''}"
                onclick={() => onToggleGitSnippet(sn.cmd)}
                disabled={gitPullRunning}
                title={sn.cmd}
              >
                <i class={hasGitSnippet(sn.cmd) ? 'ri-check-line' : 'ri-add-line'} aria-hidden="true"></i>
                <span>{sn.label.replace(/^\+\s*/, '')}</span>
              </button>
            {/each}
          </div>

          <div class="git-confirm-note">
            <i class="ri-shield-check-line" aria-hidden="true"></i>
            <span>
              Dijalankan di <code class="mono">{getProjectRootDir(gitPullTarget)}</code>. File <code class="mono">.env</code> (izin <code class="mono">0640</code>) dicadangkan otomatis &amp; PHP-FPM di-reload setelah selesai.
            </span>
          </div>
        </div>

        {#if gitPullError}
          <div class="git-pull-alert is-error">
            <i class="ri-error-warning-fill" aria-hidden="true"></i>
            <div>
              <strong>Git Deploy Gagal</strong>
              <p class="mono">{gitPullError}</p>
            </div>
          </div>
        {/if}

        {#if gitPullResult}
          <div class="git-pull-alert {gitPullResult.updated ? 'is-updated' : 'is-uptodate'}">
            <i class={gitPullResult.updated ? 'ri-checkbox-circle-fill' : 'ri-information-fill'} aria-hidden="true"></i>
            <div>
              {#if gitPullResult.updated}
                <strong>Berhasil Diperbarui ({gitPullResult.old_commit} &rarr; {gitPullResult.new_commit})</strong>
              {:else}
                <strong>Sudah Versi Terbaru (#{gitPullResult.new_commit})</strong>
              {/if}
              {#if gitPullResult.commit_message}
                <p>Commit terakhir: <span class="mono">"{gitPullResult.commit_message}"</span></p>
              {/if}
            </div>
          </div>

          {#if gitPullResult.output}
            <div class="git-output-box mono">
              <div class="git-output-head">Log Eksekusi Git Pull &amp; Post-Deploy</div>
              <pre class="log-pre-raw">{gitPullResult.output}</pre>
            </div>
          {/if}
        {/if}

      <!-- ═══ SUB-TAB 2: WEBHOOK PUSH-TO-DEPLOY ═══ -->
      {:else if subTab === 'webhook'}
        <div class="git-webhook-card">
          <div class="git-webhook-head">
            <div>
              <strong>Webhook Push-to-Deploy (GitHub / GitLab)</strong>
              <p class="git-history-msg">Setiap <code class="mono">git push</code> ke branch <code class="mono">{gitPullTarget.git_branch || 'main'}</code> otomatis memicu deploy aman di latar belakang.</p>
            </div>
            <label class="git-opt-check" for="gp-wh-enabled">
              <input id="gp-wh-enabled" type="checkbox" bind:checked={webhookEnabled} onchange={() => saveDeployConfig(false)} disabled={savingCfg} />
              <span>{webhookEnabled ? 'Webhook Aktif' : 'Nonaktif'}</span>
            </label>
          </div>

          {#if loadingInfo && !deployInfo}
            <p class="git-history-msg"><i class="ri-loader-4-line spin" aria-hidden="true"></i> Memuat konfigurasi webhook…</p>
          {:else if deployInfo}
            <div class="field">
              <label for="gp-wh-url" class="field__label">Payload URL (Tempel di GitHub Repo → Settings → Webhooks)</label>
              <div class="git-copy-row">
                <input id="gp-wh-url" type="text" class="input mono" readonly value={deployInfo.webhook_url} />
                <button
                  type="button"
                  class="btn btn--secondary btn--sm"
                  onclick={() => copyQuick(deployInfo?.webhook_url || '', 'Payload URL')}
                >
                  <i class="ri-file-copy-line" aria-hidden="true"></i> Salin
                </button>
              </div>
            </div>

            <div class="field">
              <label for="gp-wh-secret" class="field__label">Webhook Secret (HMAC-SHA256 / GitLab Token)</label>
              <div class="git-copy-row">
                <input id="gp-wh-secret" type="text" class="input mono" readonly value={deployInfo.config.webhook_secret} />
                <button
                  type="button"
                  class="btn btn--secondary btn--sm"
                  onclick={() => copyQuick(deployInfo?.config.webhook_secret || '', 'Webhook Secret')}
                >
                  <i class="ri-file-copy-line" aria-hidden="true"></i> Salin
                </button>
                <button
                  type="button"
                  class="btn btn--ghost btn--sm"
                  onclick={() => saveDeployConfig(true)}
                  disabled={savingCfg}
                >
                  <i class="ri-refresh-line" aria-hidden="true"></i> Rotasi
                </button>
              </div>
            </div>

            <div class="git-opt-row">
              <label class="git-opt-check" for="gp-wh-maint">
                <input id="gp-wh-maint" type="checkbox" bind:checked={optMaintenance} onchange={() => saveDeployConfig(false)} disabled={savingCfg} />
                <span>Maintenance Mode saat auto-deploy</span>
              </label>
              <label class="git-opt-check" for="gp-wh-opt">
                <input id="gp-wh-opt" type="checkbox" bind:checked={optAutoOptimize} onchange={() => saveDeployConfig(false)} disabled={savingCfg} />
                <span>Auto Optimize (Artisan + Filament)</span>
              </label>
              <label class="git-opt-check" for="gp-wh-assets">
                <input id="gp-wh-assets" type="checkbox" bind:checked={optRebuildAssets} onchange={() => saveDeployConfig(false)} disabled={savingCfg} />
                <span>Auto Rebuild Vite bila aset berubah</span>
              </label>
            </div>

            <div class="git-confirm-note">
              <i class="ri-information-line" aria-hidden="true"></i>
              <span>Di GitHub Webhooks, pilih <b>Content type:</b> <code class="mono">application/json</code>, isi <b>Secret</b> di atas, dan pilih <b>Just the push event</b>.</span>
            </div>
          {/if}
        </div>

      <!-- ═══ SUB-TAB 3: RIWAYAT DEPLOY & 1-KLIK ROLLBACK ═══ -->
      {:else}
        {#if loadingInfo && !deployInfo}
          <p class="git-history-msg"><i class="ri-loader-4-line spin" aria-hidden="true"></i> Memuat riwayat deployment…</p>
        {:else if !deployInfo?.deployments?.length}
          <div class="git-pull-alert is-uptodate">
            <i class="ri-history-line" aria-hidden="true"></i>
            <div>
              <strong>Belum Ada Riwayat Deployment</strong>
              <p>Riwayat deploy akan tercatat otomatis setiap kali Anda menjalankan Git Pull, Webhook Push-to-Deploy, atau Rollback.</p>
            </div>
          </div>
        {:else}
          <div class="git-history-list">
            {#each deployInfo.deployments as item (item.id)}
              {@const isCurrent = item.new_commit && item.new_commit === deployInfo.current_commit}
              <div class="git-history-item">
                <div class="git-history-top">
                  <div class="git-history-meta">
                    <span
                      class="badge badge--sm"
                      class:badge--success={item.status === 'success'}
                      class:badge--warning={item.status === 'warning'}
                      class:badge--danger={item.status === 'failed'}
                    >
                      {item.status}
                    </span>
                    <span class="badge badge--sm badge--info">{item.trigger}</span>
                    <strong class="mono">
                      {#if item.old_commit && item.old_commit !== item.new_commit}
                        #{item.old_commit} &rarr; #{item.new_commit || '-'}
                      {:else}
                        #{item.new_commit || item.old_commit || '-'}
                      {/if}
                    </strong>
                    {#if isCurrent}
                      <span class="badge badge--sm badge--success">Aktif</span>
                    {/if}
                    <small class="mono">{fmtDeployTime(item.created_at)} · {(item.duration_ms / 1000).toFixed(1)}s</small>
                  </div>
                  <div class="git-history-actions">
                    {#if item.output}
                      <button
                        type="button"
                        class="btn btn--ghost btn--sm"
                        onclick={() => (expandedLogId = expandedLogId === item.id ? null : item.id)}
                      >
                        {expandedLogId === item.id ? 'Tutup Log' : 'Log'}
                      </button>
                    {/if}
                    {#if item.new_commit && !isCurrent}
                      <button
                        type="button"
                        class="btn btn--secondary btn--sm"
                        onclick={() => handleRollback(item)}
                        disabled={Boolean(rollingBackCommit) || gitPullRunning}
                      >
                        {#if rollingBackCommit === item.new_commit}
                          <i class="ri-loader-4-line spin" aria-hidden="true"></i> Rollback…
                        {:else}
                          <i class="ri-arrow-go-back-line" aria-hidden="true"></i> Rollback
                        {/if}
                      </button>
                    {/if}
                  </div>
                </div>
                {#if item.commit_message}
                  <p class="git-history-msg mono">{item.commit_message}</p>
                {/if}
                {#if expandedLogId === item.id && item.output}
                  <div class="git-output-box mono">
                    <pre class="log-pre-raw">{item.output}</pre>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn--ghost" onclick={onClose} disabled={gitPullRunning || Boolean(rollingBackCommit)}>
      Tutup
    </button>
    {#if subTab === 'pull'}
      <button
        type="button"
        class="btn btn--primary"
        disabled={gitPullRunning || Boolean(rollingBackCommit)}
        onclick={() =>
          onExecuteGitPull({
            auto_optimize: optAutoOptimize,
            maintenance_mode: optMaintenance,
            rebuild_assets: optRebuildAssets,
          })}
      >
        {#if gitPullRunning}
          <i class="ri-loader-4-line spin" aria-hidden="true"></i>
          <span>Menarik Commit &amp; Menyinkronkan Aplikasi…</span>
        {:else}
          <i class="ri-git-pull-request-line" aria-hidden="true"></i>
          <span>Deploy Perubahan Terbaru</span>
        {/if}
      </button>
    {/if}
  {/snippet}
</Modal>
