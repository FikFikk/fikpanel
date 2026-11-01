<script lang="ts">
  // Subkomponen modal pengaturan Edge & Keamanan Cloudflare
  import '../../styles/pages/waf/cf-edge-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { CloudflareZone } from '../../lib/types';

  interface Props {
    showCFSettingsModal: boolean;
    selectedZone: CloudflareZone | null;
    onUpdateCFZoneSetting: (zone: CloudflareZone, setting: string, value: string) => void;
    onToggleDevMode: (zone: CloudflareZone) => void;
    onPurgeCFCache: (zone: CloudflareZone) => void;
  }

  let {
    showCFSettingsModal = $bindable(),
    selectedZone,
    onUpdateCFZoneSetting,
    onToggleDevMode,
    onPurgeCFCache,
  }: Props = $props();
</script>

<div class="pg-waf-cf-edge-modal">
  <!-- Modal Pengaturan Edge & Keamanan Cloudflare -->
  {#if showCFSettingsModal && selectedZone}
    <Modal
      open={showCFSettingsModal && !!selectedZone}
      title={`Pengaturan Edge & Keamanan Cloudflare: ${selectedZone.name}`}
      subtitle="Konfigurasi SSL/TLS, DDoS Under Attack Mode, HTTPS Redirect, dan Optimasi Kecepatan"
      size="xl"
      onclose={() => (showCFSettingsModal = false)}
    >
      <div class="stack-gap">
        <div class="forensic-modal-grid">
          <!-- Kolom 1: SSL/TLS & HTTPS -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-lock-password-line text-accent"></i> Enkripsi SSL/TLS & HTTPS</div>
            <div class="stack-gap mt-2">
              <div class="form-group">
                <label for="cf-ssl-select">Mode Enkripsi SSL/TLS</label>
                <select
                  id="cf-ssl-select"
                  class="input-dark"
                  value={selectedZone.ssl_mode || 'full'}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'ssl', (e.currentTarget as HTMLSelectElement).value)}
                >
                  <option value="off">Off — Tidak mengenkripsi trafik</option>
                  <option value="flexible">Flexible — HTTPS ke Cloudflare, HTTP ke Origin Server</option>
                  <option value="full">Full — End-to-end HTTPS (Mendukung Self-Signed)</option>
                  <option value="strict">Full (Strict) — End-to-end HTTPS dengan Sertifikat Valid (Let's Encrypt)</option>
                </select>
              </div>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.always_use_https}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'always_use_https', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Always Use HTTPS</strong> — Alihkan semua permintaan <code>http://</code> ke <code>https://</code> di Edge</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.auto_https_rewrites}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'automatic_https_rewrites', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Automatic HTTPS Rewrites</strong> — Perbaiki tautan mixed-content HTTP secara otomatis</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.tls_1_3}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'tls_1_3', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>TLS 1.3</strong> — Aktifkan protokol enkripsi TLS 1.3 terbaru & tercepat</span>
              </label>
            </div>
          </div>

          <!-- Kolom 2: Proteksi DDoS & Cache CDN -->
          <div class="drawer-card">
            <div class="drawer-card-title"><i class="ri-radar-line text-danger"></i> Mitigasi DDoS & Performa Edge CDN</div>
            <div class="stack-gap mt-2">
              <div class="form-group">
                <label for="cf-sec-select">Security Level & Mode DDoS</label>
                <select
                  id="cf-sec-select"
                  class="input-dark"
                  value={selectedZone.security_level || 'medium'}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'security_level', (e.currentTarget as HTMLSelectElement).value)}
                >
                  <option value="under_attack">🔥 I'm Under Attack! — Tampilkan tantangan JS untuk semua pengunjung</option>
                  <option value="high">High — Tantang semua pengunjung berisiko dalam 14 hari terakhir</option>
                  <option value="medium">Medium — Tantang pengunjung berisiko sedang & tinggi</option>
                  <option value="low">Low — Hanya tantang pengunjung paling berbahaya</option>
                  <option value="essentially_off">Essentially Off — Hanya tantang bot paling parah</option>
                </select>
              </div>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.brotli}
                  disabled={selectedZone.read_only}
                  onchange={(e) => onUpdateCFZoneSetting(selectedZone!, 'brotli', (e.currentTarget as HTMLInputElement).checked ? 'on' : 'off')}
                />
                <span><strong>Brotli Compression</strong> — Percepat pemuatan halaman dengan kompresi Brotli</span>
              </label>

              <label class="custom-chk">
                <input
                  type="checkbox"
                  checked={selectedZone.development_mode === 1}
                  disabled={selectedZone.read_only}
                  onchange={() => onToggleDevMode(selectedZone!)}
                />
                <span><strong>Development Mode (Bypass Cache 3 Jam)</strong> — Lewati cache CDN saat menguji kode baru</span>
              </label>

              <div class="pt-2">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm w-full"
                  disabled={selectedZone.read_only}
                  onclick={() => onPurgeCFCache(selectedZone!)}
                >
                  <i class="ri-delete-bin-7-line text-warning"></i> Kosongkan Seluruh Cache CDN (Purge Everything)
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      {#snippet footer()}
        <button type="button" class="btn btn-ghost" onclick={() => (showCFSettingsModal = false)}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}
</div>

