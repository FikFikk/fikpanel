<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { Site } from '../../lib/types';
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
    output?: string;
  }

  // Antarmuka properti untuk modal Git Pull & Auto-Deploy
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
    onExecuteGitPull: () => void;
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
    getProjectRootDir,
  }: Props = $props();
</script>

<Modal
  {open}
  title={gitPullTarget ? `Git Pull & Sinkronisasi: ${gitPullTarget.domain}` : 'Git Pull'}
  size="lg"
  onclose={onClose}
>
  {#if gitPullTarget}
    <div class="pg-sites-git-pull-modal">
      <span class="danger-sub-caption">Tarik commit terbaru dari repositori remote tanpa menimpa file .env lokal</span>

      <!-- Ringkasan Repositori & Branch Aktif -->
      <div class="git-summary-strip mono">
        <div class="git-sum-item">
          <span class="git-sum-lbl">Repositori</span>
          <strong><i class="ri-github-fill"></i> {gitPullTarget.git_repo || 'origin'}</strong>
        </div>
        <div class="git-sum-item">
          <span class="git-sum-lbl">Branch Aktif</span>
          <strong class="git-branch-highlight"><i class="ri-git-branch-line"></i> {gitPullTarget.git_branch || 'main'}</strong>
        </div>
        <div class="git-sum-item">
          <span class="git-sum-lbl">Commit Saat Ini</span>
          <strong>#{gitPullTarget.git_commit || 'HEAD'}</strong>
        </div>
        {#if gitPullTarget.has_cicd}
          <div class="git-sum-item">
            <span class="git-sum-lbl">Pipeline CI/CD</span>
            <strong class="git-branch-highlight">
              {#if gitPullTarget.cicd_provider === 'actions-runner'}
                <i class="ri-cpu-line"></i> Action Runner
              {:else if gitPullTarget.cicd_provider === 'github-actions'}
                <i class="ri-github-fill"></i> GitHub Actions
              {:else if gitPullTarget.cicd_provider === 'gitlab-ci'}
                <i class="ri-gitlab-line"></i> GitLab CI
              {:else}
                <i class="ri-rocket-2-line"></i> {gitPullTarget.cicd_provider || 'CI/CD'}
              {/if}
            </strong>
          </div>
        {/if}
      </div>

      <!-- Konfirmasi & Editor Perintah Dinamis Git Pull -->
      <div class="git-cmd-section">
        <div class="git-cmd-label-row">
          <span class="git-cmd-label">
            <i class="ri-terminal-box-line"></i>
            Perintah Eksekusi (Dapat Diedit Dinamis)
          </span>
          <button
            type="button"
            class="git-chip-reset"
            onclick={onResetGitPullCmd}
            disabled={gitPullRunning}
            title="Kembalikan ke perintah git pull murni"
          >
            Reset ke git pull
          </button>
        </div>

        <div class="git-cmd-input-wrap">
          <span class="git-cmd-prompt mono">$</span>
          <input
            type="text"
            class="git-cmd-input mono"
            bind:value={gitPullCustomCmd}
            disabled={gitPullRunning}
            placeholder="git pull origin {gitPullTarget.git_branch || 'main'} && php artisan optimize:clear"
          />
        </div>

        <!-- Tombol Chip Tambah/Hapus Perintah Cepat -->
        <div class="git-chips-bar">
          <span class="git-chips-hint">Klik untuk tambah/hapus:</span>
          {#each gitPullSnippets.filter((sn) => !sn.phpOnly || gitPullTarget?.kind === 'php') as sn}
            <button
              type="button"
              class="git-snippet-chip mono {hasGitSnippet(sn.cmd) ? 'is-active' : ''}"
              onclick={() => onToggleGitSnippet(sn.cmd)}
              disabled={gitPullRunning}
              title={sn.cmd}
            >
              <i class={hasGitSnippet(sn.cmd) ? 'ri-check-line' : 'ri-add-line'}></i>
              <span>{sn.label.replace(/^\+\s*/, '')}</span>
            </button>
          {/each}
        </div>

        <div class="git-confirm-note">
          <i class="ri-shield-check-line"></i>
          <span>
            Dijalankan di <code class="mono">{getProjectRootDir(gitPullTarget)}</code>. File <code class="mono">.env</code> lokal dicadangkan otomatis &amp; izin <code class="mono">www-data</code> tetap terjaga.
          </span>
        </div>
      </div>

      {#if gitPullError}
        <div class="git-pull-alert is-error">
          <i class="ri-error-warning-fill"></i>
          <div>
            <strong>Git Pull Gagal</strong>
            <p class="mono">{gitPullError}</p>
          </div>
        </div>
      {/if}

      {#if gitPullResult}
        <div class="git-pull-alert {gitPullResult.updated ? 'is-updated' : 'is-uptodate'}">
          <i class={gitPullResult.updated ? 'ri-checkbox-circle-fill' : 'ri-information-fill'}></i>
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
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn--ghost" onclick={onClose} disabled={gitPullRunning}>
      Tutup
    </button>
    <button
      type="button"
      class="btn btn--primary"
      disabled={gitPullRunning}
      onclick={onExecuteGitPull}
    >
      {#if gitPullRunning}
        <i class="ri-loader-4-line spin"></i>
        <span>Menarik Commit &amp; Menyinkronkan Aplikasi...</span>
      {:else}
        <i class="ri-git-pull-request-line"></i>
        <span>Tarik Perubahan Terbaru (Git Pull)</span>
      {/if}
    </button>
  {/snippet}
</Modal>

