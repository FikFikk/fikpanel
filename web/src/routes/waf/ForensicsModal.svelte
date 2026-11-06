<script lang="ts">
  // Subkomponen modal inspeksi forensik paket HTTP & akses mendalam
  import '../../styles/pages/waf/forensics-modal.css';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import type { WAFAccessLog } from '../../lib/types';

  interface Props {
    selectedAccessLog: WAFAccessLog | null;
    formatDateTime: (ts: number) => string;
    formatBytes: (bytes: number) => string;
    onQuickWhitelistIP: (ip: string, reason?: string) => void;
    onQuickBlockPath: (urlPath: string, reason?: string) => void;
    onQuickBlockIP: (ip: string, reason?: string) => void;
  }

  let {
    selectedAccessLog = $bindable(),
    formatDateTime,
    formatBytes,
    onQuickWhitelistIP,
    onQuickBlockPath,
    onQuickBlockIP,
  }: Props = $props();
</script>

<div class="pg-waf-forensics-modal">
  <!-- Modal Forensik Paket HTTP & Akses Mendalam -->
  {#if selectedAccessLog}
    <Modal
      open={!!selectedAccessLog}
      title={`Inspeksi Forensik Paket HTTP #${selectedAccessLog.id}`}
      subtitle={`${formatDateTime(selectedAccessLog.timestamp || selectedAccessLog.created_at)} · ${selectedAccessLog.is_threat ? `Ancaman Terdeteksi: ${selectedAccessLog.attack_type}` : 'Lalu Lintas Pengunjung Normal'}`}
      size="xl"
      onclose={() => (selectedAccessLog = null)}
    >
      <div class="forensic-modal-grid">
        <!-- Kolom 1: Informasi Request & Target -->
        <div class="drawer-card">
          <div class="drawer-card-title"><i class="ri-links-line text-accent"></i> Detail Permintaan HTTP & Endpoint</div>
          <div class="drawer-info-grid">
            <div class="info-row">
              <span class="info-lbl">Domain Vhost:</span>
              <span class="info-val mono font-bold text-accent">{selectedAccessLog.site_name}</span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Metode & Protokol:</span>
              <span class="info-val mono">
                <span class="method-mini {selectedAccessLog.method.toLowerCase()}">{selectedAccessLog.method}</span>
                {selectedAccessLog.protocol}
              </span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Kode Status HTTP:</span>
              <span class="info-val">
                <span class="http-status-badge s{Math.floor(selectedAccessLog.status_code / 100)}xx">{selectedAccessLog.status_code}</span>
              </span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Ukuran Payload Dikirim:</span>
              <span class="info-val mono">{formatBytes(selectedAccessLog.bytes_sent)} ({selectedAccessLog.bytes_sent.toLocaleString()} bytes)</span>
            </div>
            <div class="info-col">
              <span class="info-lbl">URL Path / Payload Lengkap:</span>
              <div class="payload-box mono text-xs">{selectedAccessLog.url_path}</div>
            </div>
            {#if selectedAccessLog.referer}
              <div class="info-col">
                <span class="info-lbl">HTTP Referer:</span>
                <div class="ua-box mono text-2xs">{selectedAccessLog.referer}</div>
              </div>
            {/if}
          </div>
        </div>

        <!-- Kolom 2: Identitas Klien, Geolokasi & Perangkat -->
        <div class="drawer-card">
          <div class="drawer-card-title"><i class="ri-map-pin-user-line text-info"></i> Atribusi IP, Geolokasi & Perangkat</div>
          <div class="drawer-info-grid">
            <div class="info-row">
              <span class="info-lbl">Alamat IP Klien:</span>
              <span class="info-val mono font-bold">{selectedAccessLog.client_ip}</span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Negara & Kota:</span>
              <span class="info-val">
                <span class="flag-code">{selectedAccessLog.country_code}</span>
                {selectedAccessLog.city ? `${selectedAccessLog.city}, ` : ''}{selectedAccessLog.region ? `${selectedAccessLog.region}, ` : ''}{selectedAccessLog.country}
              </span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Koordinat GPS:</span>
              <span class="info-val mono text-xs">{selectedAccessLog.latitude.toFixed(4)}, {selectedAccessLog.longitude.toFixed(4)}</span>
            </div>
            <div class="info-row">
              <span class="info-lbl">ISP / Jaringan ASN:</span>
              <span class="info-val text-xs">{selectedAccessLog.isp || 'Autonomous System'}</span>
            </div>
            <div class="info-row">
              <span class="info-lbl">Perangkat & OS:</span>
              <span class="info-val">{selectedAccessLog.device} · {selectedAccessLog.os} · {selectedAccessLog.browser}</span>
            </div>
            <div class="info-col">
              <span class="info-lbl">Raw User-Agent Header:</span>
              <div class="ua-box mono text-2xs">{selectedAccessLog.user_agent || '(Kosong / Raw Socket)'}</div>
            </div>
            <div class="info-col">
              <span class="info-lbl">Keputusan Mesin WAF:</span>
              <div class="rule-box text-xs">
                {#if selectedAccessLog.is_threat}
                  <strong class="text-danger">{selectedAccessLog.attack_type}</strong> — {selectedAccessLog.rule_name || 'Signature Inspection Engine'} ({selectedAccessLog.action.toUpperCase()})
                {:else}
                  <strong class="text-success">Normal HTTP Request</strong> — Tidak ditemukan pola SQLi, XSS, LFI, atau scanner berbahaya.
                {/if}
              </div>
            </div>
          </div>
        </div>
      </div>

      {#snippet footer()}
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              onQuickWhitelistIP(selectedAccessLog.client_ip, 'Whitelisted via Packet Forensic Modal');
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-shield-check-line text-success"></i> Whitelist IP
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              onQuickBlockPath(selectedAccessLog.url_path, `Blocked endpoint from ${selectedAccessLog.client_ip}`);
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-forbid-2-line text-warning"></i> Blokir URL Path Ini
        </button>
        <button
          type="button"
          class="btn btn-danger btn-sm"
          onclick={() => {
            if (selectedAccessLog) {
              onQuickBlockIP(selectedAccessLog.client_ip, selectedAccessLog.attack_type || `Forensic block (${selectedAccessLog.url_path})`);
              selectedAccessLog = null;
            }
          }}
        >
          <i class="ri-shield-cross-line"></i> Blokir IP ke Blacklist
        </button>
        <button type="button" class="btn btn-ghost btn-sm" onclick={() => (selectedAccessLog = null)}>Tutup</button>
      {/snippet}
    </Modal>
  {/if}
</div>

