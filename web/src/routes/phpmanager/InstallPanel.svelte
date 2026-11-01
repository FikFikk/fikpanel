<script lang="ts">
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { PHPExtensionInfo } from '../../lib/types';
  import '../../styles/pages/phpmanager/install-panel.css';

  // Antarmuka properti untuk modal panduan terminal SSH dan perintah instalasi
  interface Props {
    open: boolean;
    activeVersion: string;
    sshModalExt: PHPExtensionInfo | null;
    onClose: () => void;
    onCopyCommand: (cmd: string, label: string) => void;
    onInstallExtension: (ext: PHPExtensionInfo) => void;
    onUninstallExtension: (ext: PHPExtensionInfo) => void;
  }

  let {
    open,
    activeVersion,
    sshModalExt,
    onClose,
    onCopyCommand,
    onInstallExtension,
    onUninstallExtension,
  }: Props = $props();
</script>

<Modal
  {open}
  size="xl"
  onclose={onClose}
>
  {#snippet header()}
    {#if sshModalExt}
      <div class="flex items-center gap-3">
        <div class="header-icon">
          <i class="ri-terminal-box-line"></i>
        </div>
        <div class="modal__head-text">
          <h3 class="modal__title">Perintah Terminal / SSH: {sshModalExt.name}</h3>
          <p class="modal__sub">
            Ekstensi PHP {activeVersion} &bull; Paket APT: <span class="mono text-accent">{sshModalExt.package_name || `php${activeVersion}-${sshModalExt.name}`}</span>
          </p>
        </div>
      </div>
    {/if}
  {/snippet}

  {#if sshModalExt}
    <div class="flex flex-col gap-3 pg-phpmanager-install-panel">
      <div class="ssh-notice">
        <i class="ri-information-line"></i>
        <span>
          Jika Anda ingin menginstal atau mengelola ekstensi ini langsung melalui sesi terminal SSH server, gunakan perintah siap-pakai berikut:
        </span>
      </div>

      <!-- Perintah Install -->
      <div class="terminal-cmd-box">
        <div class="terminal-cmd-header flex justify-between items-center">
          <span class="cmd-type-label"><i class="ri-download-2-line"></i> Perintah Instalasi (Install)</span>
          <button
            type="button"
            class="btn btn-xs btn-ghost"
            onclick={() =>
              onCopyCommand(
                sshModalExt?.install_command ||
                  `sudo DEBIAN_FRONTEND=noninteractive apt-get update && sudo apt-get install -y php${activeVersion}-${sshModalExt?.name} && sudo systemctl restart php${activeVersion}-fpm`,
                'Perintah Install'
              )}
          >
            <i class="ri-file-copy-line"></i> Salin Perintah
          </button>
        </div>
        <pre class="terminal-snippet mono"><code>{sshModalExt.install_command || `sudo DEBIAN_FRONTEND=noninteractive apt-get update && sudo apt-get install -y php${activeVersion}-${sshModalExt.name} && sudo systemctl restart php${activeVersion}-fpm`}</code></pre>
      </div>

      <!-- Perintah Uninstall (Jika bukan core) -->
      {#if !sshModalExt.is_core}
        <div class="terminal-cmd-box mt-3">
          <div class="terminal-cmd-header flex justify-between items-center">
            <span class="cmd-type-label text-warning"><i class="ri-delete-bin-line"></i> Perintah Copot (Uninstall)</span>
            <button
              type="button"
              class="btn btn-xs btn-ghost"
              onclick={() =>
                onCopyCommand(
                  sshModalExt?.uninstall_command ||
                    `sudo DEBIAN_FRONTEND=noninteractive apt-get remove -y php${activeVersion}-${sshModalExt?.name} && sudo systemctl restart php${activeVersion}-fpm`,
                  'Perintah Uninstall'
                )}
            >
              <i class="ri-file-copy-line"></i> Salin Perintah
            </button>
          </div>
          <pre class="terminal-snippet mono"><code>{sshModalExt.uninstall_command || `sudo DEBIAN_FRONTEND=noninteractive apt-get remove -y php${activeVersion}-${sshModalExt.name} && sudo systemctl restart php${activeVersion}-fpm`}</code></pre>
        </div>
      {/if}

      <!-- Pilihan Instalasi Otomatis 1-Klik -->
      <div class="one-click-promo mt-4">
        <div class="flex justify-between items-center flex-wrap gap-2">
          <div>
            <span class="font-semibold text-text text-sm">Tidak ingin repot buka terminal SSH?</span>
            <p class="text-xs text-subtle">FikPanel dapat mengeksekusi instalasi paket ini secara otomatis di latar belakang.</p>
          </div>
          {#if sshModalExt.installed}
            {#if !sshModalExt.is_core}
              <button
                type="button"
                class="btn btn-sm btn-danger"
                onclick={() => {
                  const ext = sshModalExt;
                  onClose();
                  if (ext) onUninstallExtension(ext);
                }}
              >
                <i class="ri-delete-bin-line"></i> Copot via Panel
              </button>
            {/if}
          {:else}
            <button
              type="button"
              class="btn btn-sm btn-primary"
              onclick={() => {
                const ext = sshModalExt;
                onClose();
                if (ext) onInstallExtension(ext);
              }}
            >
              <i class="ri-download-2-line"></i> Pasang Otomatis Sekarang
            </button>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn-ghost" onclick={onClose}>Tutup</button>
  {/snippet}
</Modal>

