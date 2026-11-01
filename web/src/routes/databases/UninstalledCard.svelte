<script lang="ts">
  /**
   * Subkomponen Kartu Informasi Engine Database Belum Terpasang.
   */
  import '../../styles/pages/databases/uninstalled-card.css';

  interface Props {
    displayName: string;
    onInstall: () => void;
    onRemoteDb: () => void;
  }

  let { displayName, onInstall, onRemoteDb }: Props = $props();
</script>

<div class="uninstalled-card pg-databases-uninstalled-card">
  <div class="uninstalled-card__body">
    <div class="alert-icon-box">
      <i class="ri-error-warning-fill" aria-hidden="true"></i>
    </div>
    <div class="uninstalled-card__content">
      <h2 class="uninstalled-title">
        Server database {displayName} belum terkonfigurasi
      </h2>
      <p class="uninstalled-desc">
        Layanan {displayName} belum dipasang pada server ini. Anda dapat menginstal
        paket resmi secara otomatis dengan Quick Install atau menghubungkan database eksternal / remote.
      </p>
      <div class="uninstalled-actions">
        <button class="btn btn-primary" onclick={onInstall}>
          <i class="ri-download-cloud-line" aria-hidden="true"></i>
          <span>Pasang Sekarang (Quick Install)</span>
        </button>
        <button class="btn btn-outline" onclick={onRemoteDb}>
          <i class="ri-external-link-line" aria-hidden="true"></i>
          <span>Hubungkan Remote DB</span>
        </button>
      </div>
    </div>
  </div>
</div>

