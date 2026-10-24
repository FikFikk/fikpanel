<script lang="ts">
  import { onMount } from 'svelte';
  import { apiFetch } from '../api';
  import type { DomainReport, Site, SiteKind, SiteMeta } from '../types';

  interface Props {
    onClose: () => void;
    onCreated: (site: Site) => void;
    onOpenSSL: (site: Site) => void;
    onNotify: (msg: string) => void;
  }

  let { onClose, onCreated, onOpenSSL, onNotify }: Props = $props();

  const KINDS: { id: SiteKind; label: string; desc: string; icon: string }[] = [
    { id: 'static', label: 'Static', desc: 'HTML, SPA build, landing page', icon: 'ri-html5-line' },
    { id: 'php', label: 'PHP', desc: 'Laravel, WordPress, CMS PHP', icon: 'ri-code-s-slash-line' },
    { id: 'proxy', label: 'Proxy / App', desc: 'Node, Go, Python, Docker', icon: 'ri-share-forward-line' }
  ];
  const UPSTREAM_PRESETS = [
    { label: 'Node', url: 'http://127.0.0.1:3000' },
    { label: 'Go', url: 'http://127.0.0.1:8080' },
    { label: 'Python', url: 'http://127.0.0.1:8000' }
  ];

  // State form
  let domains = $state('');
  let addWWW = $state(true);
  let description = $state('');
  let kind = $state<SiteKind>('static');
  let upstream = $state('http://127.0.0.1:3000');
  let rootPath = $state('');
  let rootTouched = $state(false);
  let phpVersion = $state('');

  // State validasi & meta
  let meta = $state<SiteMeta>({ php_versions: [], server_ips: [] });
  let report = $state<DomainReport | null>(null);
  let checking = $state(false);
  let checkError = $state('');
  let submitting = $state(false);
  let submitError = $state('');
  let created = $state<Site | null>(null);

  let debounceTimer: ReturnType<typeof setTimeout> | undefined;
  let abortCtl: AbortController | undefined;

  // Root path mengikuti domain utama sampai pengguna mengubahnya sendiri
  let defaultRoot = $derived(report?.primary ? `/var/www/${report.primary}/public_html` : '');
  $effect(() => {
    if (!rootTouched) rootPath = defaultRoot;
  });

  let checks = $derived(report?.checks ?? []);
  let dnsMap = $derived(new Map((report?.dns ?? []).map((d) => [d.domain, d])));
  let canSubmit = $derived(
    !!report?.valid && !checking && !submitting && domains.trim() !== '' &&
      (kind !== 'proxy' || upstream.trim() !== '') &&
      (kind === 'proxy' || rootPath.trim() !== '')
  );

  onMount(() => {
    apiFetch<SiteMeta>('/api/sites/meta')
      .then((m) => {
        meta = m;
        if (!phpVersion && m.php_versions.length) phpVersion = m.php_versions[0];
      })
      .catch((err: Error) => onNotify(`Gagal memuat metadata situs: ${err.message}`));
    return () => {
      clearTimeout(debounceTimer);
      abortCtl?.abort();
    };
  });

  // Validasi realtime dengan debounce; request lama dibatalkan agar hasil tidak tertukar
  function scheduleCheck() {
    clearTimeout(debounceTimer);
    abortCtl?.abort();
    checkError = '';
    if (!domains.trim()) {
      report = null;
      checking = false;
      return;
    }
    checking = true;
    debounceTimer = setTimeout(runCheck, 450);
  }

  async function runCheck() {
    const ctl = new AbortController();
    abortCtl = ctl;
    try {
      report = await apiFetch<DomainReport>('/api/sites/check-domains', {
        method: 'POST',
        body: JSON.stringify({ domains, add_www: addWWW, dns: true }),
        signal: ctl.signal
      });
    } catch (err) {
      if ((err as Error).name === 'AbortError') return;
      checkError = (err as Error).message;
      report = null;
    } finally {
      if (abortCtl === ctl) checking = false;
    }
  }

  async function submit(e: Event) {
    e.preventDefault();
    if (!canSubmit) return;
    submitting = true;
    submitError = '';
    try {
      const site = await apiFetch<Site>('/api/sites', {
        method: 'POST',
        body: JSON.stringify({
          domains,
          add_www: addWWW,
          kind,
          upstream: kind === 'proxy' ? upstream.trim() : '',
          root_path: kind === 'proxy' ? '' : rootPath.trim(),
          php_version: kind === 'php' ? phpVersion : '',
          description: description.trim()
        })
      });
      created = site;
      onCreated(site);
      onNotify(`Situs ${site.domain} berhasil dibuat`);
    } catch (err) {
      submitError = (err as Error).message;
    } finally {
      submitting = false;
    }
  }

  let allPointHere = $derived((report?.dns ?? []).length > 0 && (report?.dns ?? []).every((d) => d.points_here));

  function onWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && !submitting) onClose();
  }
</script>

<svelte:window onkeydown={onWindowKey} />

<div class="as-backdrop" role="presentation" onclick={(e) => { if (e.target === e.currentTarget && !submitting) onClose(); }}>
  <div class="as-card" role="dialog" aria-modal="true" aria-labelledby="as-title">
    <header class="as-head">
      <div class="as-head-title">
        <span class="as-icon"><i class={created ? 'ri-checkbox-circle-fill' : 'ri-global-line'}></i></span>
        <div>
          <h3 id="as-title">{created ? 'Situs berhasil dibuat' : 'Tambah Situs'}</h3>
          <p>{created ? created.domain : 'Vhost Nginx dibuat & divalidasi otomatis'}</p>
        </div>
      </div>
      <button class="as-close" onclick={onClose} aria-label="Tutup" id="as-close-btn" disabled={submitting}>
        <i class="ri-close-line"></i>
      </button>
    </header>

    {#if created}
      <!-- Langkah 2: ringkasan + ajakan aktivasi SSL -->
      <div class="as-body as-done">
        <div class="done-hero">
          <div class="done-ring"><i class="ri-check-line"></i></div>
          <p>Vhost <b>{created.domain}</b> aktif di port 80 dan siap menerima trafik.</p>
        </div>
        <dl class="done-grid">
          <dt>Jenis</dt><dd>{KINDS.find((k) => k.id === created!.kind)?.label}</dd>
          {#if created.kind === 'proxy'}
            <dt>Upstream</dt><dd class="mono">{created.upstream}</dd>
          {:else}
            <dt>Direktori</dt><dd class="mono">{created.root_path}</dd>
          {/if}
          {#if created.php_version}<dt>PHP</dt><dd>{created.php_version}</dd>{/if}
          <dt>Domain</dt><dd class="mono">{[created.domain, ...JSON.parse(created.aliases || '[]')].join(', ')}</dd>
        </dl>
        <div class="ssl-cta" class:warn={!allPointHere}>
          <i class={allPointHere ? 'ri-shield-check-line' : 'ri-error-warning-line'}></i>
          <div>
            <b>Langkah berikutnya: aktifkan SSL</b>
            <span>
              {allPointHere
                ? 'Semua domain sudah mengarah ke server ini, Let\'s Encrypt siap diterbitkan.'
                : 'Sebagian domain belum mengarah ke server ini. Perbarui DNS A record terlebih dahulu agar validasi HTTP-01 berhasil.'}
            </span>
          </div>
        </div>
        <footer class="as-foot">
          <button class="btn-ghost" onclick={onClose} id="as-done-btn">Selesai</button>
          <button class="btn-primary" onclick={() => onOpenSSL(created!)} id="as-ssl-btn">
            <i class="ri-lock-line"></i> Aktifkan SSL sekarang
          </button>
        </footer>
      </div>
    {:else}
      <form class="as-body" onsubmit={submit}>
        <div class="as-scroll">
          <!-- Domain multi-baris ala aaPanel -->
          <label class="fld" for="as-domains">
            <span class="fld-label">Domain <em>satu per baris, baris pertama jadi domain utama</em></span>
            <textarea
              id="as-domains"
              rows="4"
              spellcheck="false"
              placeholder={'contoh.com\nblog.contoh.com\n*.app.contoh.com'}
              bind:value={domains}
              oninput={scheduleCheck}
            ></textarea>
          </label>
          <div class="row-between">
            <label class="switch" for="as-www">
              <input id="as-www" type="checkbox" bind:checked={addWWW} onchange={scheduleCheck} />
              <span class="track"></span>
              <span>Tambahkan <code>www.</code> otomatis untuk domain utama</span>
            </label>
            {#if checking}
              <span class="hint"><i class="ri-loader-4-line spin"></i> Memeriksa…</span>
            {:else if report}
              <span class="hint" class:ok={report.valid} class:bad={!report.valid}>
                <i class={report.valid ? 'ri-checkbox-circle-line' : 'ri-close-circle-line'}></i>
                {report.valid ? `${checks.filter((c) => !c.error).length} domain valid` : 'Periksa kembali domain'}
              </span>
            {/if}
          </div>

          {#if checkError}
            <p class="err-line"><i class="ri-error-warning-line"></i> {checkError}</p>
          {/if}

          {#if checks.length}
            <ul class="chips">
              {#each checks as c, i (c.input + i)}
                {@const dns = dnsMap.get(c.domain)}
                <li class="chip" class:bad={!!c.error} class:auto={c.auto} title={c.error || dns?.error || (dns?.ips ?? []).join(', ')}>
                  {#if c.error}
                    <i class="ri-close-circle-fill"></i>
                  {:else if dns?.points_here}
                    <i class="ri-checkbox-circle-fill ok"></i>
                  {:else if dns}
                    <i class="ri-error-warning-fill warn"></i>
                  {:else}
                    <i class="ri-checkbox-blank-circle-line"></i>
                  {/if}
                  <span class="mono">{c.domain || c.input}</span>
                  {#if report?.primary === c.domain}<b class="tag">utama</b>{/if}
                  {#if c.auto}<b class="tag">auto</b>{/if}
                  {#if c.wildcard}<b class="tag">wildcard</b>{/if}
                  {#if c.error}<small>{c.error}</small>{:else if dns && !dns.points_here}<small>{dns.error ? 'DNS belum ada' : `→ ${(dns.ips ?? []).join(', ')}`}</small>{/if}
                </li>
              {/each}
            </ul>
            {#if report?.server_ips.length}
              <p class="ip-note">
                <i class="ri-server-line"></i> IP server: <span class="mono">{report.server_ips.join(', ')}</span>
                — status DNS hanya informasi, situs tetap bisa dibuat.
              </p>
            {/if}
          {/if}

          <label class="fld" for="as-desc">
            <span class="fld-label">Deskripsi <em>opsional</em></span>
            <input id="as-desc" type="text" maxlength="200" placeholder="Website company profile" bind:value={description} />
          </label>

          <!-- Pilihan jenis situs -->
          <span class="fld-label">Jenis situs</span>
          <div class="kinds" role="radiogroup" aria-label="Jenis situs">
            {#each KINDS as k (k.id)}
              <button
                type="button"
                role="radio"
                aria-checked={kind === k.id}
                id="as-kind-{k.id}"
                class="kind"
                class:active={kind === k.id}
                onclick={() => (kind = k.id)}
              >
                <i class={k.icon}></i>
                <b>{k.label}</b>
                <small>{k.desc}</small>
              </button>
            {/each}
          </div>

          {#if kind === 'proxy'}
            <label class="fld" for="as-upstream">
              <span class="fld-label">Upstream URL</span>
              <input id="as-upstream" class="mono" type="text" placeholder="http://127.0.0.1:3000" bind:value={upstream} />
            </label>
            <div class="presets">
              {#each UPSTREAM_PRESETS as p (p.label)}
                <button type="button" class="preset" class:active={upstream === p.url} onclick={() => (upstream = p.url)}>
                  {p.label} <span class="mono">:{p.url.split(':').pop()}</span>
                </button>
              {/each}
            </div>
          {:else}
            <div class="grid-2" class:single={kind !== 'php'}>
              <label class="fld" for="as-root">
                <span class="fld-label">
                  Direktori situs
                  {#if rootTouched && defaultRoot}
                    <button type="button" class="link" onclick={() => { rootTouched = false; }}>reset</button>
                  {/if}
                </span>
                <input
                  id="as-root"
                  class="mono"
                  type="text"
                  placeholder="/var/www/contoh.com/public_html"
                  bind:value={rootPath}
                  oninput={() => (rootTouched = true)}
                />
              </label>
              {#if kind === 'php'}
                <label class="fld" for="as-php">
                  <span class="fld-label">Versi PHP</span>
                  <select id="as-php" bind:value={phpVersion}>
                    {#each meta.php_versions as v (v)}
                      <option value={v}>PHP {v}</option>
                    {:else}
                      <option value="">Default (php-fpm)</option>
                    {/each}
                  </select>
                </label>
              {/if}
            </div>
          {/if}

          <div class="info">
            <i class="ri-lock-line"></i>
            <span>SSL Let's Encrypt diaktifkan <b>setelah situs dibuat</b>, karena validasi HTTP-01 membutuhkan vhost yang sudah aktif di port 80.</span>
          </div>

          {#if submitError}
            <p class="err-line"><i class="ri-error-warning-line"></i> {submitError}</p>
          {/if}
        </div>

        <footer class="as-foot">
          <button type="button" class="btn-ghost" onclick={onClose} disabled={submitting}>Batal</button>
          <button type="submit" class="btn-primary" disabled={!canSubmit} id="as-submit-btn">
            {#if submitting}<i class="ri-loader-4-line spin"></i> Membuat…{:else}<i class="ri-add-line"></i> Buat Situs{/if}
          </button>
        </footer>
      </form>
    {/if}
  </div>
</div>

<style>
  .as-backdrop {
    position: fixed; inset: 0; z-index: 1000;
    display: grid; place-items: center; padding: 1.5rem;
    background: rgba(3, 6, 12, 0.72); backdrop-filter: blur(6px);
    animation: fade 0.18s ease-out;
  }
  .as-card {
    width: min(640px, 100%); max-height: calc(100vh - 3rem);
    display: flex; flex-direction: column;
    background: linear-gradient(180deg, #121624 0%, #0e111a 100%);
    border: 1px solid #222a3b; border-radius: 14px;
    box-shadow: 0 30px 80px -20px rgba(0, 0, 0, 0.7);
    overflow: hidden; animation: pop 0.22s cubic-bezier(0.2, 0.9, 0.3, 1.2);
  }
  .as-head { display: flex; align-items: center; justify-content: space-between; padding: 0.9rem 1.2rem; border-bottom: 1px solid #1c2230; }
  .as-head-title { display: flex; align-items: center; gap: 0.7rem; }
  .as-head h3 { margin: 0; font-size: 1rem; font-weight: 700; color: #f1f5f9; }
  .as-head p { margin: 0.1rem 0 0; font-size: 0.72rem; color: #64748b; }
  .as-icon {
    width: 34px; height: 34px; border-radius: 9px; display: grid; place-items: center;
    background: rgba(16, 185, 129, 0.12); color: #34d399; font-size: 1.1rem;
  }
  .as-close {
    width: 32px; height: 32px; border-radius: 8px; border: 1px solid #222a3b;
    background: #131722; color: #94a3b8; font-size: 1.1rem; cursor: pointer; transition: all 0.15s ease;
  }
  .as-close:hover:not(:disabled) { color: #f43f5e; border-color: rgba(244, 63, 94, 0.4); transform: rotate(90deg); }
  .as-body { display: flex; flex-direction: column; min-height: 0; flex: 1; }
  .as-scroll { overflow-y: auto; padding: 1.1rem 1.3rem; display: flex; flex-direction: column; gap: 0.8rem; }
  .as-foot { display: flex; justify-content: flex-end; gap: 0.6rem; padding: 0.85rem 1.3rem; border-top: 1px solid #1c2230; background: rgba(8, 10, 16, 0.4); }

  .fld { display: flex; flex-direction: column; gap: 0.35rem; }
  .fld-label { font-size: 0.75rem; font-weight: 600; color: #cbd5e1; display: flex; align-items: center; gap: 0.5rem; }
  .fld-label em { font-style: normal; font-weight: 400; color: #64748b; }
  textarea, input[type='text'], select {
    width: 100%; box-sizing: border-box; padding: 0.6rem 0.75rem;
    background: #0b0e16; border: 1px solid #222a3b; border-radius: 8px;
    color: #e2e8f0; font-size: 0.85rem; transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }
  textarea { resize: vertical; font-family: 'Fira Code', monospace; line-height: 1.55; min-height: 92px; }
  textarea:focus, input:focus, select:focus { outline: none; border-color: #10b981; box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.15); }
  .mono { font-family: 'Fira Code', monospace; }

  .row-between { display: flex; align-items: center; justify-content: space-between; gap: 0.8rem; flex-wrap: wrap; }
  .switch { display: inline-flex; align-items: center; gap: 0.55rem; font-size: 0.78rem; color: #94a3b8; cursor: pointer; }
  .switch input { position: absolute; opacity: 0; pointer-events: none; }
  .switch .track { width: 32px; height: 18px; border-radius: 99px; background: #1e2535; position: relative; transition: background 0.15s ease; flex-shrink: 0; }
  .switch .track::after { content: ''; position: absolute; top: 2px; left: 2px; width: 14px; height: 14px; border-radius: 50%; background: #94a3b8; transition: transform 0.18s ease, background 0.18s ease; }
  .switch input:checked + .track { background: rgba(16, 185, 129, 0.35); }
  .switch input:checked + .track::after { transform: translateX(14px); background: #34d399; }
  .switch input:focus-visible + .track { box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.25); }
  .switch code { color: #34d399; }

  .hint { font-size: 0.74rem; color: #64748b; display: inline-flex; align-items: center; gap: 0.3rem; }
  .hint.ok { color: #34d399; }
  .hint.bad { color: #fb7185; }

  .chips { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 0.4rem; }
  .chip {
    display: inline-flex; align-items: center; gap: 0.35rem; padding: 0.3rem 0.55rem;
    border-radius: 7px; background: #111624; border: 1px solid #1f2738; font-size: 0.74rem; color: #cbd5e1;
    animation: pop 0.18s ease-out;
  }
  .chip.auto { border-style: dashed; }
  .chip.bad { border-color: rgba(244, 63, 94, 0.4); background: rgba(244, 63, 94, 0.06); color: #fda4af; }
  .chip i { font-size: 0.9rem; color: #475569; }
  .chip.bad i { color: #f43f5e; }
  .chip i.ok { color: #10b981; }
  .chip i.warn { color: #f59e0b; }
  .chip small { color: #64748b; }
  .chip.bad small { color: #fb7185; }
  .tag { font-size: 0.62rem; font-weight: 600; padding: 0.05rem 0.35rem; border-radius: 4px; background: rgba(16, 185, 129, 0.12); color: #34d399; text-transform: uppercase; letter-spacing: 0.03em; }
  .ip-note { margin: 0; font-size: 0.72rem; color: #64748b; display: flex; align-items: center; gap: 0.35rem; flex-wrap: wrap; }

  .kinds { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.55rem; }
  .kind {
    display: flex; flex-direction: column; align-items: flex-start; gap: 0.2rem; text-align: left;
    padding: 0.75rem 0.8rem; border-radius: 10px; cursor: pointer;
    background: #0f131e; border: 1px solid #1f2738; color: #cbd5e1; transition: all 0.15s ease;
  }
  .kind i { font-size: 1.25rem; color: #64748b; transition: color 0.15s ease; }
  .kind b { font-size: 0.85rem; }
  .kind small { font-size: 0.7rem; color: #64748b; }
  .kind:hover { border-color: #2c3850; transform: translateY(-1px); }
  .kind.active { border-color: rgba(16, 185, 129, 0.55); background: linear-gradient(160deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.02)); box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.08); }
  .kind.active i { color: #34d399; }

  .presets { display: flex; gap: 0.4rem; flex-wrap: wrap; margin-top: -0.3rem; }
  .preset { padding: 0.25rem 0.55rem; font-size: 0.72rem; border-radius: 6px; background: #111624; border: 1px solid #1f2738; color: #94a3b8; cursor: pointer; transition: all 0.15s ease; }
  .preset:hover, .preset.active { color: #34d399; border-color: rgba(16, 185, 129, 0.45); }

  .grid-2 { display: grid; grid-template-columns: 1fr 160px; gap: 0.7rem; }
  .grid-2.single { grid-template-columns: 1fr; }
  .link { background: none; border: none; padding: 0; color: #34d399; font-size: 0.7rem; cursor: pointer; text-decoration: underline; }

  .info {
    display: flex; gap: 0.6rem; align-items: flex-start; padding: 0.7rem 0.8rem; border-radius: 9px;
    background: rgba(56, 189, 248, 0.06); border: 1px solid rgba(56, 189, 248, 0.2); color: #bae6fd; font-size: 0.76rem; line-height: 1.5;
  }
  .info i { color: #38bdf8; font-size: 1rem; margin-top: 0.05rem; }
  .err-line { margin: 0; font-size: 0.76rem; color: #fb7185; display: flex; gap: 0.35rem; align-items: center; }

  .btn-primary, .btn-ghost {
    display: inline-flex; align-items: center; gap: 0.4rem; padding: 0.55rem 1rem; border-radius: 8px;
    font-size: 0.82rem; font-weight: 600; cursor: pointer; transition: all 0.15s ease;
  }
  .btn-primary { border: none; color: #04130d; background: linear-gradient(135deg, #34d399, #10b981); box-shadow: 0 6px 18px -6px rgba(16, 185, 129, 0.6); }
  .btn-primary:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 10px 24px -8px rgba(16, 185, 129, 0.7); }
  .btn-primary:disabled { opacity: 0.45; cursor: not-allowed; box-shadow: none; }
  .btn-ghost { background: transparent; border: 1px solid #222a3b; color: #94a3b8; }
  .btn-ghost:hover:not(:disabled) { color: #e2e8f0; border-color: #334155; }

  .as-done { padding: 1.3rem 1.3rem 0; gap: 1rem; overflow-y: auto; }
  .as-done .as-foot { margin: 0 -1.3rem; }
  .done-hero { display: flex; flex-direction: column; align-items: center; gap: 0.6rem; text-align: center; color: #94a3b8; font-size: 0.85rem; }
  .done-hero p { margin: 0; }
  .done-hero b { color: #e2e8f0; }
  .done-ring {
    width: 56px; height: 56px; border-radius: 50%; display: grid; place-items: center;
    background: rgba(16, 185, 129, 0.12); color: #34d399; font-size: 1.8rem;
    box-shadow: 0 0 0 6px rgba(16, 185, 129, 0.06); animation: pop 0.35s cubic-bezier(0.2, 0.9, 0.3, 1.4);
  }
  .done-grid { display: grid; grid-template-columns: 110px 1fr; gap: 0.45rem 0.8rem; margin: 0; padding: 0.85rem 1rem; border-radius: 10px; background: #0f131e; border: 1px solid #1c2230; font-size: 0.8rem; }
  .done-grid dt { color: #64748b; }
  .done-grid dd { margin: 0; color: #e2e8f0; word-break: break-all; }
  .ssl-cta { display: flex; gap: 0.7rem; padding: 0.8rem 0.9rem; border-radius: 10px; background: rgba(16, 185, 129, 0.07); border: 1px solid rgba(16, 185, 129, 0.25); font-size: 0.78rem; }
  .ssl-cta i { font-size: 1.3rem; color: #34d399; }
  .ssl-cta div { display: flex; flex-direction: column; gap: 0.2rem; }
  .ssl-cta b { color: #e2e8f0; }
  .ssl-cta span { color: #94a3b8; line-height: 1.5; }
  .ssl-cta.warn { background: rgba(245, 158, 11, 0.07); border-color: rgba(245, 158, 11, 0.28); }
  .ssl-cta.warn i { color: #f59e0b; }

  .spin { display: inline-block; animation: spin 0.8s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @keyframes fade { from { opacity: 0; } }
  @keyframes pop { from { opacity: 0; transform: translateY(8px) scale(0.98); } }
  @media (max-width: 600px) {
    .kinds { grid-template-columns: 1fr; }
    .grid-2 { grid-template-columns: 1fr; }
  }
</style>
