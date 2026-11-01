<script lang="ts">
  import type {
    Fail2BanStatus,
    HardeningStatus,
    FirewallExtendedStatus,
  } from '../../lib/types';
  import '../../styles/pages/security/anti-intrusion.css';

  interface Props {
    f2bStatus: Fail2BanStatus | null;
    hardening: HardeningStatus | null;
    fwStatus: FirewallExtendedStatus | null;
    onUnbanF2B: (jail: string, ip: string) => void;
    onToggleCompilerAccess: () => void;
    onToggleICMP: () => void;
  }

  let {
    f2bStatus,
    hardening,
    fwStatus,
    onUnbanF2B,
    onToggleCompilerAccess,
    onToggleICMP,
  }: Props = $props();
</script>

<div class="pg-security-anti-intrusion tab-pane">
  <!-- Fail2Ban Daemon Status & Jails -->
  <div class="card p-4 mb-6">
    <div class="card__head mb-3">
      <span class="card__title"><i class="ri-shield-flash-line"></i> Fail2Ban Intrusion Prevention Jails</span>
      <span class="badge {f2bStatus?.running ? 'badge--success' : 'badge--danger'}">
        {f2bStatus?.running ? 'Running' : 'Stopped'}
      </span>
    </div>

    {#if !f2bStatus || !f2bStatus.jails || f2bStatus.jails.length === 0}
      <p class="text-muted text-sm">Tidak ada jail Fail2Ban yang sedang aktif memantau.</p>
    {:else}
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>Jail Name</th>
              <th>Currently Failed</th>
              <th>Total Failed</th>
              <th>Banned IPs</th>
              <th class="text-right">Operate</th>
            </tr>
          </thead>
          <tbody>
            {#each f2bStatus.jails as j}
              <tr>
                <td class="mono font-bold text-accent">{j.name}</td>
                <td class="mono text-muted">{j.currently_failed}</td>
                <td class="mono text-muted">{j.total_failed}</td>
                <td>
                  {#if j.banned_ips.length === 0}
                    <span class="text-subtle">None</span>
                  {:else}
                    {#each j.banned_ips as ip}
                      <span class="badge badge--danger mono mr-1">{ip}</span>
                    {/each}
                  {/if}
                </td>
                <td class="text-right">
                  {#each j.banned_ips as ip}
                    <button class="btn btn--secondary btn--sm" onclick={() => onUnbanF2B(j.name, ip)}>
                      Unban {ip}
                    </button>
                  {/each}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <!-- Pengerasan Sistem Operasi & Akses Compiler -->
  <div class="card p-4">
    <h3 class="section-title mb-4">Kernel & System Hardening</h3>

    <!-- Compiler Access -->
    <div class="setting-item-row">
      <div class="setting-info">
        <span class="setting-name">Compiler Access (gcc, make)</span>
        <p class="setting-desc">Batasi izin eksekusi compiler (chmod 700) hanya untuk root agar malware tidak dapat melakukan kompilasi biner di server</p>
      </div>
      <label class="switch" aria-label="Toggle Compiler Access">
        <input
          type="checkbox"
          role="switch"
          checked={hardening ? hardening.compiler_blocked : false}
          onchange={onToggleCompilerAccess}
        />
        <span class="switch__track"></span>
      </label>
    </div>

    <!-- ICMP Ping Block -->
    <div class="setting-item-row">
      <div class="setting-info">
        <span class="setting-name">Block ICMP Ping Requests</span>
        <p class="setting-desc">Cegah server merespons ping ICMP dari jaringan publik (net.ipv4.icmp_echo_ignore_all = 1)</p>
      </div>
      <label class="switch" aria-label="Toggle ICMP Hardening">
        <input
          type="checkbox"
          role="switch"
          checked={fwStatus ? fwStatus.block_icmp : false}
          onchange={onToggleICMP}
        />
        <span class="switch__track"></span>
      </label>
    </div>

    <!-- SYN Flood Protection -->
    <div class="setting-item-row">
      <div class="setting-info">
        <span class="setting-name">TCP SYN Flood Cookies Protection</span>
        <p class="setting-desc">Aktifkan perlindungan mitigasi serangan DDoS paket SYN setengah terbuka (net.ipv4.tcp_syncookies = 1)</p>
      </div>
      <span class="badge badge--success">AKTIF</span>
    </div>

    <!-- IP Spoofing Prevention -->
    <div class="setting-item-row">
      <div class="setting-info">
        <span class="setting-name">IP Spoofing Reverse Path Filter</span>
        <p class="setting-desc">Verifikasi alamat sumber paket untuk mencegah spoofing (net.ipv4.conf.all.rp_filter = 1)</p>
      </div>
      <span class="badge badge--success">AKTIF</span>
    </div>
  </div>
</div>

