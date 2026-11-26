<script lang="ts">
  /**
   * Konsol Langsung & Key Browser Redis (SCAN aman, inspeksi nilai/TTL, hapus key, FLUSHDB).
   */
  import { onMount } from 'svelte';
  import { apiFetch } from '../../lib/api';
  import { notify } from '../../lib/stores/toast.svelte';
  import { confirmDialog } from '../../lib/stores/confirm.svelte';
  import type { DatabaseEngineInfo, DatabaseTelemetry, RedisKeySummary } from '../../lib/types';
  import '../../styles/pages/databases/redis-console.css';

  interface Props {
    currentEngine: DatabaseEngineInfo;
    telemetry: DatabaseTelemetry | null;
    onCopyText: (text: string, label: string) => void;
    onOpenEngineManager: () => void;
    onOpenRootPass: () => void;
    onRefreshTelemetry: () => void;
  }

  let {
    currentEngine,
    telemetry,
    onCopyText,
    onOpenEngineManager,
    onOpenRootPass,
    onRefreshTelemetry,
  }: Props = $props();

  let selectedDb = $state(0);
  let pattern = $state('*');
  let keys = $state<RedisKeySummary[]>([]);
  let loadingKeys = $state(false);
  let flushing = $state(false);

  let inspectedKey = $state<{ key: string; type: string; ttl: number; value: string } | null>(null);
  let loadingInspect = $state(false);

  // Pindai kunci Redis menggunakan SCAN (non-blocking)
  async function scanKeys() {
    loadingKeys = true;
    try {
      const res = await apiFetch<{ ok: boolean; keys: RedisKeySummary[] }>(
        `/api/databases/redis/keys?db=${selectedDb}&pattern=${encodeURIComponent(pattern.trim() || '*')}&count=100`
      );
      keys = res.keys || [];
    } catch (e: any) {
      keys = [];
      notify(e.message || 'Gagal memindai kunci Redis', 'error');
    } finally {
      loadingKeys = false;
    }
  }

  // Inspeksi detail nilai dan TTL satu kunci
  async function inspectKey(k: string) {
    loadingInspect = true;
    try {
      const res = await apiFetch<{ ok: boolean; key: string; type: string; ttl: number; value: string }>(
        `/api/databases/redis/key?db=${selectedDb}&key=${encodeURIComponent(k)}`
      );
      inspectedKey = res;
    } catch (e: any) {
      notify(e.message || 'Gagal membaca nilai kunci', 'error');
    } finally {
      loadingInspect = false;
    }
  }

  // Hapus satu kunci Redis
  async function deleteKey(k: string) {
    const ok = await confirmDialog({
      title: 'Hapus Kunci Redis',
      message: `Hapus kunci "${k}" dari Redis db${selectedDb}?`,
      confirmText: 'Hapus Kunci',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/databases/redis/key?db=${selectedDb}&key=${encodeURIComponent(k)}`, {
        method: 'DELETE',
      });
      notify(`Kunci "${k}" dihapus`, 'success');
      if (inspectedKey?.key === k) inspectedKey = null;
      await scanKeys();
      onRefreshTelemetry();
    } catch (e: any) {
      notify(e.message || 'Gagal menghapus kunci Redis', 'error');
    }
  }

  // Kosongkan seluruh isi database Redis aktif (FLUSHDB)
  async function flushCurrentDb() {
    const ok = await confirmDialog({
      title: `Kosongkan Redis db${selectedDb} (FLUSHDB)`,
      message: `Seluruh cache, sesi, dan antrean di dalam db${selectedDb} akan dihapus permanen. Lanjutkan?`,
      confirmText: `Kosongkan db${selectedDb}`,
      tone: 'danger',
    });
    if (!ok) return;

    flushing = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>('/api/databases/redis/flush', {
        method: 'POST',
        body: JSON.stringify({ db: selectedDb }),
      });
      notify(res.message || `Redis db${selectedDb} dikosongkan`, 'success');
      inspectedKey = null;
      await scanKeys();
      onRefreshTelemetry();
    } catch (e: any) {
      notify(e.message || 'Gagal menjalankan FLUSHDB', 'error');
    } finally {
      flushing = false;
    }
  }

  function formatTTL(ttl: number): string {
    if (ttl < 0) return 'Permanen';
    if (ttl < 60) return `${ttl}d`;
    if (ttl < 3600) return `${Math.floor(ttl / 60)}m`;
    return `${Math.floor(ttl / 3600)}j`;
  }

  function dbKeyCount(idx: number): number {
    const found = telemetry?.redis_databases?.find((d) => d.db === idx);
    return found ? found.keys : 0;
  }

  onMount(() => {
    scanKeys();
  });
</script>

<div class="pg-databases-redis-console">
  <!-- Toolbar Kontrol Redis -->
  <div class="redis-toolbar">
    <div class="redis-toolbar-left">
      <select
        class="redis-db-select"
        aria-label="Pilih indeks database Redis"
        bind:value={selectedDb}
        onchange={() => {
          inspectedKey = null;
          scanKeys();
        }}
      >
        {#each Array.from({ length: 16 }, (_, i) => i) as idx}
          <option value={idx}>
            db{idx} {dbKeyCount(idx) > 0 ? `(${dbKeyCount(idx).toLocaleString('id-ID')} kunci)` : ''}
          </option>
        {/each}
      </select>

      <input
        type="text"
        class="redis-pattern-input"
        placeholder="Pola kunci (mis. laravel_*, *)"
        aria-label="Filter pola kunci Redis"
        bind:value={pattern}
        onkeydown={(e) => {
          if (e.key === 'Enter') scanKeys();
        }}
      />

      <button type="button" class="btn btn-primary btn-sm" disabled={loadingKeys} onclick={scanKeys}>
        <i class={loadingKeys ? 'ri-loader-4-line spin' : 'ri-search-line'} aria-hidden="true"></i>
        <span>Pindai (SCAN)</span>
      </button>
    </div>

    <div class="redis-toolbar-right">
      <button type="button" class="btn btn-outline btn-sm" onclick={onOpenRootPass}>
        <i class="ri-key-2-line" aria-hidden="true"></i>
        <span>Password Auth</span>
      </button>
      <button type="button" class="btn btn-outline btn-sm" onclick={onOpenEngineManager}>
        <i class="ri-settings-3-line" aria-hidden="true"></i>
        <span>{currentEngine.display_name} {currentEngine.version || ''}</span>
      </button>
      <button
        type="button"
        class="btn btn-outline btn-sm"
        disabled={flushing}
        onclick={flushCurrentDb}
      >
        <i class={flushing ? 'ri-loader-4-line spin' : 'ri-delete-bin-7-line'} aria-hidden="true"></i>
        <span>Flush db{selectedDb}</span>
      </button>
    </div>
  </div>

  <!-- Area Browser Kunci & Inspektor Nilai -->
  <div class="redis-workspace">
    <!-- Daftar Kunci -->
    <div class="redis-card">
      <div class="redis-card-head">
        <span>Daftar Kunci — db{selectedDb} ({keys.length} ditampilkan)</span>
        <span class="redis-ttl-badge">SCAN maks. 100 kunci</span>
      </div>
      <div class="redis-keys-scroll">
        {#if loadingKeys}
          <div class="redis-empty">Memindai kunci Redis...</div>
        {:else if keys.length === 0}
          <div class="redis-empty">Tidak ada kunci yang cocok pada db{selectedDb}.</div>
        {:else}
          {#each keys as item (item.key)}
            <div class="redis-key-row" class:is-active={inspectedKey?.key === item.key}>
              <button
                type="button"
                class="redis-key-btn"
                onclick={() => inspectKey(item.key)}
                title={item.key}
              >
                <span class="redis-type-badge">{item.type}</span>
                <span class="redis-key-name">{item.key}</span>
              </button>
              <span class="redis-ttl-badge" title="Sisa masa berlaku kunci">TTL: {formatTTL(item.ttl)}</span>
              <button
                type="button"
                class="btn-icon"
                onclick={() => deleteKey(item.key)}
                title="Hapus kunci ini"
                aria-label={`Hapus kunci ${item.key}`}
              >
                <i class="ri-delete-bin-line" aria-hidden="true"></i>
              </button>
            </div>
          {/each}
        {/if}
      </div>
    </div>

    <!-- Inspektor Nilai -->
    <div class="redis-card">
      <div class="redis-card-head">
        <span>Inspektor Nilai Kunci</span>
        {#if inspectedKey}
          <button
            type="button"
            class="btn btn-outline btn-sm"
            onclick={() => onCopyText(inspectedKey?.value || '', 'Nilai Redis')}
          >
            <i class="ri-file-copy-line" aria-hidden="true"></i>
            <span>Salin</span>
          </button>
        {/if}
      </div>
      <div class="redis-inspector-body">
        {#if loadingInspect}
          <div class="redis-empty">Mengambil nilai kunci...</div>
        {:else if !inspectedKey}
          <div class="redis-empty">
            Klik salah satu kunci di sebelah kiri untuk melihat tipe, TTL, dan isi nilainya.
          </div>
        {:else}
          <div class="redis-meta-strip">
            <span class="redis-key-name">{inspectedKey.key}</span>
            <span>
              <span class="redis-type-badge">{inspectedKey.type}</span>
              <span class="redis-ttl-badge"> · TTL: {formatTTL(inspectedKey.ttl)}</span>
            </span>
          </div>
          <pre class="redis-value-pre">{inspectedKey.value}</pre>
        {/if}
      </div>
    </div>
  </div>
</div>
