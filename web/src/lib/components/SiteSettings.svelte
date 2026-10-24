<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { apiFetch } from '../api';
  import ManagerModal from './ManagerModal.svelte';
  import type { DomainReport, ManagerTab, SiteDetail, SiteKind, SiteMeta } from '../types';

  interface Props {
    siteId: number;
    initialTab?: 'domain' | 'site' | 'ssl' | 'conf';
    onClose: () => void;
    onChanged: () => void;
    onNotify: (msg: string) => void;
  }

  let { siteId, initialTab = 'ssl', onClose, onChanged, onNotify }: Props = $props();

  const TABS: ManagerTab[] = [
    { id: 'domain', label: 'Domain & Alias', icon: 'ri-global-line' },
    { id: 'site', label: 'Situs', icon: 'ri-settings-3-line' },
    { id: 'ssl', label: 'SSL', icon: 'ri-lock-line' },
    { id: 'conf', label: 'Konfigurasi', icon: 'ri-file-code-line' }
  ];
  const STEPS = ['Memulai', 'Mendaftarkan akun ACME', 'Validasi HTTP-01 domain', 'Menerbitkan sertifikat', 'Menerapkan HTTPS ke Nginx', 'Selesai'];

  // Tab awal hanya dibaca sekali saat modal dibuka
  let active = $state<string>(untrack(() => initialTab));
  let detail = $state<SiteDetail | null>(null);
  let meta = $state<SiteMeta>({ php_versions: [], server_ips: [] });
  let loadError = $state('');

  // Tab Domain
  let domainText = $state('');
  let domainReport = $state<DomainReport | null>(null);
  let domainChecking = $state(false);
  let domainSaving = $state(false);

  // Tab Situs
  let fKind = $state<SiteKind>('static');
  let fRoot = $state('');
  let fUpstream = $state('');
  let fPHP = $state('');
  let fDesc = $state('');
  let siteSaving = $state(false);

  // Tab SSL
  let sslEnabled = $state(false);
  let sslForce = $state(true);
  let sslHSTS = $state(false);
  let skipDNS = $state(false);
  let sslSaving = $state(false);
  let dnsReport = $state<DomainReport | null>(null);
  let dnsLoading = $state(false);

  // Tab Konfigurasi
  let vhost = $state('');
  let vhostLoading = $state(false);

  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  let domainTimer: ReturnType<typeof setTimeout> | undefined;
  let destroyed = false;

  let site = $derived(detail?.site);
  let cert = $derived(detail?.certificate);
  let job = $derived(detail?.ssl_job);
  let jobRunning = $derived(!!job?.running);
  let certIssued = $derived(cert?.status === 'issued');
  let certSANs = $derived.by<string[]>(() => {
    try {
      return JSON.parse(cert?.domains || '[]');
    } catch {
      return [];
    }
  });
  let daysLeft = $derived(cert?.not_after ? Math.floor((cert.not_after * 1000 - Date.now()) / 86400000) : 0);
  let dnsAllReady = $derived(!!dnsReport && dnsReport.dns.length > 0 && dnsReport.dns.every((d) => d.points_here));
  let hasWildcard = $derived((detail?.domains ?? []).some((d) => d.startsWith('*.')));

  // Sinkronkan form dari data server setiap kali detail dimuat ulang
  function hydrate(d: SiteDetail) {
    detail = d;
    domainText = d.domains.join('\n');
    fKind = d.site.kind;
    fRoot = d.site.root_path;
    fUpstream = d.site.upstream || 'http://127.0.0.1:3000';
    fPHP = d.site.php_version || meta.php_versions[0] || '';
    fDesc = d.site.description;
    sslEnabled = d.site.ssl_mode !== 'none';
    sslForce = d.site.force_https === 1;
    sslHSTS = d.site.hsts === 1;
  }

  async function load() {
    try {
      const d = await apiFetch<SiteDetail>(`/api/sites/${siteId}`);
      if (destroyed) return;
      loadError = '';
      hydrate(d);
      if (d.ssl_job?.running) schedulePoll();
    } catch (err) {
      loadError = (err as Error).message;
    }
  }

  // Polling status penerbitan SSL selama job berjalan
  function schedulePoll() {
    clearTimeout(pollTimer);
    pollTimer = setTimeout(async () => {
      try {
        const d = await apiFetch<SiteDetail>(`/api/sites/${siteId}`);
        if (destroyed) return;
        detail = d;
        if (d.ssl_job?.running) {
          schedulePoll();
          return;
        }
        hydrate(d);
        onChanged();
        onNotify(d.ssl_job?.ok ? `SSL ${d.site.domain} aktif` : `Penerbitan SSL gagal: ${d.ssl_job?.message || d.certificate?.last_error}`);
      } catch (err) {
        if (!destroyed) {
          onNotify(`Gagal memantau status SSL: ${(err as Error).message}`);
          schedulePoll();
        }
      }
    }, 1500);
  }

  async function checkDNS() {
    if (!detail) return;
    dnsLoading = true;
    try {
      dnsReport = await apiFetch<DomainReport>('/api/sites/check-domains', {
        method: 'POST',
        body: JSON.stringify({ domains: detail.domains.join('\n'), add_www: false, exclude_id: siteId, dns: true })
      });
    } catch (err) {
      onNotify(`Cek DNS gagal: ${(err as Error).message}`);
    } finally {
      dnsLoading = false;
    }
  }

  async function loadVhost() {
    vhostLoading = true;
    try {
      const r = await apiFetch<{ content: string }>(`/api/sites/${siteId}/vhost`);
      vhost = r.content;
    } catch (err) {
      vhost = `# Gagal memuat konfigurasi: ${(err as Error).message}`;
    } finally {
      vhostLoading = false;
    }
  }

  // Muat data turunan hanya saat tab atau detail berganti (state loading tidak dilacak agar tidak loop saat gagal)
  $effect(() => {
    const tab = active;
    if (!detail) return;
    untrack(() => {
      if (tab === 'ssl' && !dnsReport && !dnsLoading) checkDNS();
      if (tab === 'conf') loadVhost();
    });
  });

  onMount(() => {
    apiFetch<SiteMeta>('/api/sites/meta')
      .then((m) => {
        meta = m;
        if (!fPHP && m.php_versions.length) fPHP = m.php_versions[0];
      })
      .catch((err: Error) => onNotify(`Gagal memuat metadata: ${err.message}`));
    load();
    return () => {
      destroyed = true;
      clearTimeout(pollTimer);
      clearTimeout(domainTimer);
    };
  });

  // Payload PUT lengkap; field yang tidak diubah tab aktif diambil dari data tersimpan
  function sitePayload(overrides: Record<string, unknown> = {}) {
    const s = detail!.site;
    return JSON.stringify({
      domains: detail!.domains.join('\n'),
      add_www: false,
      kind: s.kind,
      upstream: s.upstream,
      root_path: s.root_path,
      php_version: s.php_version,
      description: s.description,
      ...overrides
    });
  }

  function onDomainInput() {
    clearTimeout(domainTimer);
    domainChecking = true;
    domainTimer = setTimeout(async () => {
      try {
        domainReport = await apiFetch<DomainReport>('/api/sites/check-domains', {
          method: 'POST',
          body: JSON.stringify({ domains: domainText, add_www: false, exclude_id: siteId, dns: false })
        });
      } catch (err) {
        onNotify(`Validasi domain gagal: ${(err as Error).message}`);
      } finally {
        domainChecking = false;
      }
    }, 400);
  }

  async function saveDomains() {
    if (!detail) return;
    if (certIssued && !confirm('Mengubah domain akan menonaktifkan HTTPS sampai sertifikat diterbitkan ulang. Lanjutkan?')) return;
    domainSaving = true;
    try {
      const r = await apiFetch<{ ssl_reset: boolean }>(`/api/sites/${siteId}`, { method: 'PUT', body: sitePayload({ domains: domainText }) });
      onNotify(r.ssl_reset ? 'Domain diperbarui — terbitkan ulang SSL di tab SSL' : 'Domain diperbarui');
      domainReport = null;
      dnsReport = null;
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      domainSaving = false;
    }
  }

  async function saveSite() {
    if (!detail) return;
    siteSaving = true;
    try {
      await apiFetch(`/api/sites/${siteId}`, {
        method: 'PUT',
        body: sitePayload({
          kind: fKind,
          upstream: fKind === 'proxy' ? fUpstream.trim() : '',
          root_path: fKind === 'proxy' ? '' : fRoot.trim(),
          php_version: fKind === 'php' ? fPHP : '',
          description: fDesc.trim()
        })
      });
      onNotify('Pengaturan situs disimpan & Nginx di-reload');
      await load();
      onChanged();
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      siteSaving = false;
    }
  }

  async function saveSSL(issue: boolean) {
    sslSaving = true;
    try {
      const r = await apiFetch<{ issuing: boolean }>(`/api/sites/${siteId}/ssl`, {
        method: 'POST',
        body: JSON.stringify({ enabled: sslEnabled, force_https: sslForce, hsts: sslHSTS && sslForce, issue, skip_dns: skipDNS })
      });
      await load();
      onChanged();
      if (r.issuing) {
        onNotify('Penerbitan sertifikat Let\'s Encrypt dimulai');
        schedulePoll();
      } else {
        onNotify(sslEnabled ? 'Pengaturan SSL disimpan' : 'SSL dinonaktifkan');
      }
    } catch (err) {
      onNotify((err as Error).message);
    } finally {
      sslSaving = false;
    }
  }

  async function copyVhost() {
    try {
      await navigator.clipboard.writeText(vhost);
      onNotify('Konfigurasi disalin');
    } catch (err) {
      onNotify(`Gagal menyalin: ${(err as Error).message}`);
    }
  }

  function fmtDate(ts: number) {
    return ts ? new Date(ts * 1000).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
  }
</script>

<ManagerModal
  title={site ? site.domain : 'Memuat…'}
  subtitle={site ? `${site.kind.toUpperCase()} · ${site.kind === 'proxy' ? site.upstream : site.root_path}` : ''}
  tabs={TABS}
  bind:active
  {onClose}
>
  {#snippet logo()}
    <span class="ss-logo"><i class="ri-global-line"></i></span>
  {/snippet}
  {#snippet badge()}
    {#if site}
      <span class="ss-badge" class:on={certIssued} class:pending={cert?.status === 'pending'}>
        <i class={certIssued ? 'ri-lock-fill' : 'ri-lock-unlock-line'}></i>
        {certIssued ? 'HTTPS' : cert?.status === 'pending' ? 'Menerbitkan…' : 'HTTP'}
      </span>
    {/if}
  {/snippet}

  {#snippet children(tab: string)}
    {#if loadError}
      <div class="ss-empty"><i class="ri-error-warning-line"></i><p>{loadError}</p><button class="btn-ghost" onclick={load}>Coba lagi</button></div>
    {:else if !detail}
      <div class="ss-empty"><i class="ri-loader-4-line spin"></i><p>Memuat data situs…</p></div>
    {:else if tab === 'domain'}
      <section class="ss-sec">
        <h4>Domain & Alias</h4>
        <p class="ss-desc">Satu domain per baris. Baris pertama (non-wildcard) menjadi domain utama dan nama file vhost.</p>
        <textarea id="ss-domains" rows="6" spellcheck="false" bind:value={domainText} oninput={onDomainInput}></textarea>
        {#if domainReport?.checks?.length}
          <ul class="chips">
            {#each domainReport.checks as c, i (c.input + i)}
              <li class="chip" class:bad={!!c.error}>
                <i class={c.error ? 'ri-close-circle-fill' : 'ri-checkbox-circle-fill ok'}></i>
                <span class="mono">{c.domain || c.input}</span>
                {#if domainReport.primary === c.domain}<b class="tag">utama</b>{/if}
                {#if c.error}<small>{c.error}</small>{/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if certIssued}
          <div class="note warn"><i class="ri-error-warning-line"></i><span>Sertifikat saat ini hanya mencakup domain lama. Perubahan domain akan me-reset SSL dan HTTPS nonaktif sampai diterbitkan ulang.</span></div>
        {/if}
        <div class="ss-actions">
          <button class="btn-primary" id="ss-save-domains" onclick={saveDomains}
            disabled={domainSaving || domainChecking || (domainReport !== null && !domainReport.valid) || domainText.trim() === detail.domains.join('\n')}>
            {#if domainSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan domain
          </button>
        </div>
      </section>
    {:else if tab === 'site'}
      <section class="ss-sec">
        <h4>Pengaturan Situs</h4>
        <div class="seg" role="radiogroup" aria-label="Jenis situs">
          {#each [['static', 'Static'], ['php', 'PHP'], ['proxy', 'Proxy / App']] as [id, label] (id)}
            <button type="button" role="radio" aria-checked={fKind === id} class:active={fKind === id} onclick={() => (fKind = id as SiteKind)}>{label}</button>
          {/each}
        </div>
        {#if fKind === 'proxy'}
          <label class="fld" for="ss-upstream"><span>Upstream URL</span>
            <input id="ss-upstream" class="mono" type="text" bind:value={fUpstream} placeholder="http://127.0.0.1:3000" />
          </label>
        {:else}
          <label class="fld" for="ss-root"><span>Direktori situs</span>
            <input id="ss-root" class="mono" type="text" bind:value={fRoot} placeholder={`/var/www/${detail.site.domain}/public_html`} />
          </label>
          {#if fKind === 'php'}
            <label class="fld" for="ss-php"><span>Versi PHP</span>
              <select id="ss-php" bind:value={fPHP}>
                {#each meta.php_versions as v (v)}<option value={v}>PHP {v}</option>{:else}<option value="">Default (php-fpm)</option>{/each}
              </select>
            </label>
          {/if}
        {/if}
        <label class="fld" for="ss-desc"><span>Deskripsi</span>
          <input id="ss-desc" type="text" maxlength="200" bind:value={fDesc} />
        </label>
        <div class="ss-actions">
          <button class="btn-primary" id="ss-save-site" onclick={saveSite} disabled={siteSaving}>
            {#if siteSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-save-line"></i>{/if} Simpan & reload
          </button>
        </div>
      </section>
    {:else if tab === 'ssl'}
      <section class="ss-sec">
        <h4>SSL / TLS</h4>
        <label class="toggle-row" for="ss-ssl-on">
          <div><b>Aktifkan SSL untuk domain ini</b><small>Sertifikat gratis Let's Encrypt, diperpanjang otomatis 30 hari sebelum kedaluwarsa</small></div>
          <input id="ss-ssl-on" type="checkbox" bind:checked={sslEnabled} disabled={jobRunning} /><span class="track"></span>
        </label>

        {#if sslEnabled}
          <div class="sub">
            <label class="toggle-row" for="ss-force">
              <div><b>Paksa HTTPS</b><small>Redirect 301 seluruh trafik HTTP ke HTTPS</small></div>
              <input id="ss-force" type="checkbox" bind:checked={sslForce} disabled={jobRunning} /><span class="track"></span>
            </label>
            <label class="toggle-row" for="ss-hsts" class:dim={!sslForce}>
              <div><b>HSTS</b><small>Browser mengingat HTTPS selama 1 tahun (butuh Paksa HTTPS)</small></div>
              <input id="ss-hsts" type="checkbox" bind:checked={sslHSTS} disabled={!sslForce || jobRunning} /><span class="track"></span>
            </label>
          </div>

          {#if hasWildcard}
            <div class="note warn"><i class="ri-error-warning-line"></i><span>Domain wildcard butuh validasi DNS-01 dan belum didukung. Hapus alias wildcard untuk menerbitkan via HTTP-01.</span></div>
          {/if}

          <!-- Kesiapan DNS sebelum validasi HTTP-01 -->
          <div class="dns-box">
            <div class="dns-head">
              <span><i class="ri-route-line"></i> Kesiapan DNS</span>
              <button class="btn-mini" onclick={checkDNS} disabled={dnsLoading}><i class="ri-refresh-line" class:spin={dnsLoading}></i> Cek ulang</button>
            </div>
            {#if dnsReport}
              <ul class="dns-list">
                {#each dnsReport.dns as d (d.domain)}
                  <li>
                    <i class={d.points_here ? 'ri-checkbox-circle-fill ok' : 'ri-error-warning-fill warn'}></i>
                    <span class="mono">{d.domain}</span>
                    <small class="mono">{d.error ? 'tidak ter-resolve' : (d.ips ?? []).join(', ')}</small>
                  </li>
                {/each}
              </ul>
              <p class="ip-note">IP server: <span class="mono">{dnsReport.server_ips.join(', ') || '—'}</span></p>
              {#if !dnsAllReady}
                <label class="check" for="ss-skip-dns">
                  <input id="ss-skip-dns" type="checkbox" bind:checked={skipDNS} />
                  Lewati cek DNS (server di balik NAT/CDN, atau DNS baru berubah)
                </label>
              {/if}
            {:else}
              <p class="ip-note"><i class="ri-loader-4-line spin"></i> Memeriksa DNS…</p>
            {/if}
          </div>
        {/if}

        {#if jobRunning && job}
          <div class="job">
            {#each STEPS as st, i (st)}
              {@const cur = STEPS.indexOf(job.step)}
              <div class="step" class:done={i < cur} class:now={i === cur}>
                <i class={i < cur ? 'ri-check-line' : i === cur ? 'ri-loader-4-line spin' : 'ri-checkbox-blank-circle-line'}></i>{st}
              </div>
            {/each}
          </div>
        {:else if job && !job.ok && job.message}
          <div class="note bad"><i class="ri-close-circle-line"></i><span>{job.message}</span></div>
        {/if}

        <div class="ss-actions">
          {#if certIssued && sslEnabled}
            <button class="btn-ghost" id="ss-renew" onclick={() => saveSSL(true)} disabled={sslSaving || jobRunning}><i class="ri-refresh-line"></i> Terbitkan ulang</button>
          {/if}
          <button class="btn-primary" id="ss-save-ssl" onclick={() => saveSSL(false)} disabled={sslSaving || jobRunning || (sslEnabled && hasWildcard && !certIssued)}>
            {#if sslSaving}<i class="ri-loader-4-line spin"></i>{:else}<i class="ri-shield-check-line"></i>{/if}
            {sslEnabled && !certIssued ? 'Simpan & terbitkan SSL' : 'Simpan'}
          </button>
        </div>

        {#if cert && cert.status !== 'none'}
          <h4 class="mt">Detail Sertifikat</h4>
          <table class="cert">
            <tbody>
              <tr><th>Status</th><td><span class="pill {cert.status}">{cert.status}</span>{#if certIssued}<small class:warn={daysLeft < 30}> · sisa {daysLeft} hari</small>{/if}</td></tr>
              <tr><th>Issued To</th><td class="mono">{certSANs.join(', ') || detail.domains.join(', ')}</td></tr>
              <tr><th>Not Before</th><td>{fmtDate(cert.not_before)}</td></tr>
              <tr><th>Not After</th><td>{fmtDate(cert.not_after)}</td></tr>
              <tr><th>Signature</th><td>{cert.signature || '—'}</td></tr>
              <tr><th>Key</th><td>{cert.key_type || '—'}</td></tr>
              <tr><th>Issuer</th><td>{cert.issuer || '—'}</td></tr>
              {#if cert.last_error}<tr><th>Error terakhir</th><td class="err">{cert.last_error}</td></tr>{/if}
            </tbody>
          </table>
        {/if}
      </section>
    {:else if tab === 'conf'}
      <section class="ss-sec fill">
        <div class="conf-head">
          <div><h4>Vhost Nginx</h4><p class="ss-desc">Dihasilkan otomatis dari pengaturan situs & SSL (hanya-baca).</p></div>
          <button class="btn-mini" onclick={copyVhost} disabled={!vhost}><i class="ri-file-copy-line"></i> Salin</button>
        </div>
        <pre class="code">{vhostLoading ? 'Memuat…' : vhost}</pre>
      </section>
    {/if}
  {/snippet}
</ManagerModal>

<style>
  .ss-logo { width: 34px; height: 34px; border-radius: 9px; display: grid; place-items: center; background: rgba(16, 185, 129, 0.12); color: #34d399; font-size: 1.1rem; }
  .ss-badge { display: inline-flex; align-items: center; gap: 0.3rem; padding: 0.2rem 0.55rem; border-radius: 99px; font-size: 0.7rem; font-weight: 600; background: #1a2030; color: #94a3b8; }
  .ss-badge.on { background: rgba(16, 185, 129, 0.14); color: #34d399; }
  .ss-badge.pending { background: rgba(245, 158, 11, 0.14); color: #fbbf24; }
  .ss-empty { height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 0.6rem; color: #64748b; }
  .ss-empty i { font-size: 1.8rem; }
  .ss-sec { display: flex; flex-direction: column; gap: 0.8rem; max-width: 680px; }
  .ss-sec.fill { max-width: none; height: 100%; }
  .ss-sec h4 { margin: 0; font-size: 0.92rem; color: #f1f5f9; }
  .ss-sec h4.mt { margin-top: 0.6rem; }
  .ss-desc { margin: -0.4rem 0 0; font-size: 0.76rem; color: #64748b; }
  .ss-actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
  .mono { font-family: 'Fira Code', monospace; }

  textarea, input[type='text'], select {
    width: 100%; box-sizing: border-box; padding: 0.6rem 0.75rem; background: #0b0e16; border: 1px solid #222a3b;
    border-radius: 8px; color: #e2e8f0; font-size: 0.85rem; transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  textarea { font-family: 'Fira Code', monospace; resize: vertical; line-height: 1.55; }
  textarea:focus, input[type='text']:focus, select:focus { outline: none; border-color: #10b981; box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15); }
  .fld { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.75rem; font-weight: 600; color: #cbd5e1; }

  .seg { display: inline-flex; padding: 3px; border-radius: 9px; background: #0b0e16; border: 1px solid #1f2738; align-self: flex-start; }
  .seg button { padding: 0.4rem 0.9rem; border: none; border-radius: 7px; background: transparent; color: #94a3b8; font-size: 0.8rem; cursor: pointer; transition: all 0.15s ease; }
  .seg button.active { background: rgba(16, 185, 129, 0.15); color: #34d399; }

  .chips { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .chip { display: inline-flex; align-items: center; gap: 0.35rem; padding: 0.3rem 0.55rem; border-radius: 7px; background: #111624; border: 1px solid #1f2738; font-size: 0.74rem; color: #cbd5e1; }
  .chip.bad { border-color: rgba(244, 63, 94, 0.4); color: #fda4af; }
  .chip.bad i { color: #f43f5e; }
  .chip small { color: #fb7185; }
  .tag { font-size: 0.62rem; padding: 0.05rem 0.35rem; border-radius: 4px; background: rgba(16, 185, 129, 0.12); color: #34d399; text-transform: uppercase; }
  i.ok { color: #10b981; }
  i.warn { color: #f59e0b; }

  .toggle-row { position: relative; display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.75rem 0.9rem; border-radius: 10px; background: #0f131e; border: 1px solid #1c2230; cursor: pointer; }
  .toggle-row div { display: flex; flex-direction: column; gap: 0.15rem; }
  .toggle-row b { font-size: 0.83rem; color: #e2e8f0; }
  .toggle-row small { font-size: 0.72rem; color: #64748b; }
  .toggle-row.dim { opacity: 0.5; }
  .toggle-row input { position: absolute; opacity: 0; pointer-events: none; }
  .track { width: 36px; height: 20px; border-radius: 99px; background: #1e2535; position: relative; flex-shrink: 0; transition: background 0.15s ease; }
  .track::after { content: ''; position: absolute; top: 3px; left: 3px; width: 14px; height: 14px; border-radius: 50%; background: #94a3b8; transition: transform 0.18s ease, background 0.18s ease; }
  .toggle-row input:checked + .track { background: rgba(16, 185, 129, 0.35); }
  .toggle-row input:checked + .track::after { transform: translateX(16px); background: #34d399; }
  .toggle-row input:focus-visible + .track { box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.25); }
  .sub { display: flex; flex-direction: column; gap: 0.5rem; padding-left: 1rem; border-left: 2px solid #1c2230; animation: slide 0.18s ease-out; }

  .dns-box { padding: 0.8rem 0.9rem; border-radius: 10px; border: 1px dashed #263048; display: flex; flex-direction: column; gap: 0.5rem; }
  .dns-head { display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem; font-weight: 600; color: #cbd5e1; }
  .dns-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.3rem; }
  .dns-list li { display: flex; align-items: center; gap: 0.45rem; font-size: 0.78rem; color: #e2e8f0; }
  .dns-list small { color: #64748b; margin-left: auto; }
  .ip-note { margin: 0; font-size: 0.72rem; color: #64748b; }
  .check { display: flex; align-items: center; gap: 0.45rem; font-size: 0.75rem; color: #fbbf24; cursor: pointer; }
  .check input { accent-color: #f59e0b; }

  .job { display: flex; flex-direction: column; gap: 0.35rem; padding: 0.8rem 0.9rem; border-radius: 10px; background: rgba(16, 185, 129, 0.05); border: 1px solid rgba(16, 185, 129, 0.2); }
  .step { display: flex; align-items: center; gap: 0.5rem; font-size: 0.78rem; color: #475569; transition: color 0.2s ease; }
  .step.done { color: #34d399; }
  .step.now { color: #e2e8f0; font-weight: 600; }

  .note { display: flex; gap: 0.55rem; padding: 0.65rem 0.8rem; border-radius: 9px; font-size: 0.76rem; line-height: 1.5; }
  .note.warn { background: rgba(245, 158, 11, 0.07); border: 1px solid rgba(245, 158, 11, 0.25); color: #fde68a; }
  .note.bad { background: rgba(244, 63, 94, 0.07); border: 1px solid rgba(244, 63, 94, 0.25); color: #fda4af; }

  .cert { width: 100%; border-collapse: collapse; font-size: 0.78rem; border: 1px solid #1c2230; border-radius: 10px; overflow: hidden; }
  .cert th { width: 140px; text-align: left; font-weight: 500; color: #64748b; background: #0f131e; }
  .cert th, .cert td { padding: 0.5rem 0.8rem; border-bottom: 1px solid #1c2230; }
  .cert td { color: #e2e8f0; word-break: break-all; }
  .cert td.err { color: #fb7185; }
  .cert small { color: #64748b; }
  .cert small.warn { color: #f59e0b; }
  .pill { padding: 0.1rem 0.45rem; border-radius: 5px; font-size: 0.68rem; font-weight: 600; text-transform: uppercase; background: #1a2030; color: #94a3b8; }
  .pill.issued { background: rgba(16, 185, 129, 0.14); color: #34d399; }
  .pill.pending { background: rgba(245, 158, 11, 0.14); color: #fbbf24; }
  .pill.failed { background: rgba(244, 63, 94, 0.14); color: #fb7185; }

  .conf-head { display: flex; justify-content: space-between; align-items: flex-start; }
  .conf-head p { margin: 0.2rem 0 0; }
  .code { flex: 1; margin: 0; padding: 0.9rem 1rem; overflow: auto; border-radius: 10px; background: #080a10; border: 1px solid #1c2230; color: #a7f3d0; font: 0.76rem/1.6 'Fira Code', monospace; white-space: pre; }

  .btn-primary, .btn-ghost, .btn-mini { display: inline-flex; align-items: center; gap: 0.4rem; cursor: pointer; font-weight: 600; transition: all 0.15s ease; }
  .btn-primary { padding: 0.55rem 1rem; border: none; border-radius: 8px; font-size: 0.82rem; color: #04130d; background: linear-gradient(135deg, #34d399, #10b981); box-shadow: 0 6px 18px -6px rgba(16, 185, 129, 0.6); }
  .btn-primary:hover:not(:disabled) { transform: translateY(-1px); }
  .btn-primary:disabled, .btn-ghost:disabled, .btn-mini:disabled { opacity: 0.45; cursor: not-allowed; box-shadow: none; }
  .btn-ghost { padding: 0.55rem 1rem; border-radius: 8px; font-size: 0.82rem; background: transparent; border: 1px solid #222a3b; color: #94a3b8; }
  .btn-ghost:hover:not(:disabled) { color: #e2e8f0; border-color: #334155; }
  .btn-mini { padding: 0.3rem 0.6rem; border-radius: 6px; font-size: 0.72rem; background: #131722; border: 1px solid #222a3b; color: #94a3b8; }
  .btn-mini:hover:not(:disabled) { color: #34d399; border-color: rgba(16, 185, 129, 0.4); }

  .spin { display: inline-block; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes slide { from { opacity: 0; transform: translateY(-4px); } }
</style>
