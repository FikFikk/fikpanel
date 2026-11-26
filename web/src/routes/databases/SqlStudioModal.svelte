<script lang="ts">
  /**
   * Modal SQL Studio & Penjelajah Tabel bawaan FikPanel untuk MySQL/MariaDB & PostgreSQL.
   */
  import { apiFetch, formatBytes } from '../../lib/api';
  import { notify } from '../../lib/stores/toast.svelte';
  import Modal from '../../lib/components/ui/Modal.svelte';
  import CodeMirrorEditor from '../../lib/components/ui/CodeMirrorEditor.svelte';
  import type { DatabaseItem, DatabaseTableInfo } from '../../lib/types';
  import '../../styles/pages/databases/sql-studio-modal.css';

  interface Props {
    open: boolean;
    engine: string;
    db: DatabaseItem | null;
    onClose: () => void;
  }

  let { open, engine, db, onClose }: Props = $props();

  let tables = $state<DatabaseTableInfo[]>([]);
  let loadingTables = $state(false);
  let tableSearch = $state('');
  let selectedTable = $state('');
  let optimizingTable = $state('');

  let queryText = $state('');
  let runningQuery = $state(false);
  let resultCols = $state<string[]>([]);
  let resultRows = $state<string[][]>([]);
  let resultTimeMs = $state(0);
  let resultTruncated = $state(false);
  let resultNote = $state('');
  let resultError = $state('');

  // Daftar tabel terfilter sesuai pencarian di sidebar
  let filteredTables = $derived(
    tables.filter((t) => t.name.toLowerCase().includes(tableSearch.trim().toLowerCase()))
  );

  // Muat daftar tabel setiap kali modal dibuka untuk database tertentu
  $effect(() => {
    if (open && db) {
      selectedTable = '';
      tableSearch = '';
      resultCols = [];
      resultRows = [];
      resultError = '';
      resultNote = '';
      queryText = engine === 'postgresql'
        ? 'SELECT table_name FROM information_schema.tables WHERE table_schema = \'public\' ORDER BY table_name LIMIT 50;'
        : 'SHOW TABLES;';
      loadTables();
    }
  });

  // Ambil daftar tabel nyata dari engine database target
  async function loadTables() {
    if (!db) return;
    loadingTables = true;
    try {
      const res = await apiFetch<{ ok: boolean; tables: DatabaseTableInfo[] }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/tables`
      );
      tables = res.tables || [];
    } catch (e: any) {
      tables = [];
      notify(e.message || 'Gagal memuat daftar tabel database', 'error');
    } finally {
      loadingTables = false;
    }
  }

  // Eksekusi kueri SQL pada database target
  async function runQuery(customSql?: string) {
    if (!db) return;
    const sql = (customSql ?? queryText).trim();
    if (!sql) {
      notify('Tulis kueri SQL terlebih dahulu', 'warning');
      return;
    }
    runningQuery = true;
    resultError = '';
    resultNote = '';
    try {
      const res = await apiFetch<{
        ok: boolean;
        columns: string[];
        rows: string[][];
        row_count: number;
        truncated: boolean;
        time_ms: number;
        note: string;
      }>(`/api/databases/${engine}/${encodeURIComponent(db.name)}/query`, {
        method: 'POST',
        body: JSON.stringify({ query: sql }),
      });
      resultCols = res.columns || [];
      resultRows = res.rows || [];
      resultTimeMs = res.time_ms || 0;
      resultTruncated = !!res.truncated;
      resultNote = res.note || '';
    } catch (e: any) {
      resultCols = [];
      resultRows = [];
      resultError = e.message || 'Eksekusi kueri SQL gagal';
    } finally {
      runningQuery = false;
    }
  }

  // Klik tabel di sidebar untuk langsung pratinjau 50 baris pertama
  function previewTable(tableName: string) {
    selectedTable = tableName;
    const quoted = engine === 'postgresql' ? `"${tableName}"` : `\`${tableName}\``;
    queryText = `SELECT * FROM ${quoted} LIMIT 50;`;
    runQuery(queryText);
  }

  // Jalankan OPTIMIZE TABLE / VACUUM ANALYZE untuk satu tabel atau seluruh database
  async function optimizeTables(tableName = '') {
    if (!db) return;
    optimizingTable = tableName || '__all__';
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/databases/${engine}/${encodeURIComponent(db.name)}/tables/optimize`,
        {
          method: 'POST',
          body: JSON.stringify({ table: tableName }),
        }
      );
      notify(res.message || 'Optimasi tabel selesai', 'success');
      await loadTables();
    } catch (e: any) {
      notify(e.message || 'Gagal mengoptimasi tabel', 'error');
    } finally {
      optimizingTable = '';
    }
  }
</script>

<Modal
  {open}
  title={db ? `SQL Studio — ${db.name} (${engine.toUpperCase()})` : 'SQL Studio'}
  size="xl"
  onclose={onClose}
>
  <div class="pg-databases-sql-studio">
    <!-- Sidebar Daftar Tabel -->
    <aside class="studio-sidebar">
      <div class="studio-sidebar-head">
        <h4 class="studio-sidebar-title">
          <i class="ri-table-line" aria-hidden="true"></i>
          Tabel ({tables.length})
        </h4>
        <div class="studio-toolbar-right">
          <button
            type="button"
            class="studio-opt-btn"
            onclick={() => optimizeTables('')}
            disabled={optimizingTable !== '' || tables.length === 0}
            title="Optimasi seluruh tabel (OPTIMIZE / VACUUM)"
            aria-label="Optimasi seluruh tabel"
          >
            <i class={optimizingTable === '__all__' ? 'ri-loader-4-line spin' : 'ri-magic-line'} aria-hidden="true"></i>
          </button>
          <button
            type="button"
            class="studio-opt-btn"
            onclick={loadTables}
            disabled={loadingTables}
            title="Muat ulang daftar tabel"
            aria-label="Muat ulang daftar tabel"
          >
            <i class={loadingTables ? 'ri-loader-4-line spin' : 'ri-refresh-line'} aria-hidden="true"></i>
          </button>
        </div>
      </div>

      <input
        type="text"
        class="studio-search"
        placeholder="Cari nama tabel..."
        aria-label="Cari nama tabel"
        bind:value={tableSearch}
      />

      <div class="studio-table-list">
        {#if loadingTables}
          <div class="studio-empty">
            <i class="ri-loader-4-line spin" aria-hidden="true"></i>
            <span>Memindai tabel...</span>
          </div>
        {:else if filteredTables.length === 0}
          <div class="studio-empty">
            <span>Belum ada tabel ditemukan.</span>
          </div>
        {:else}
          {#each filteredTables as tbl (tbl.name)}
            <div class="studio-table-row" class:is-selected={selectedTable === tbl.name}>
              <button
                type="button"
                class="studio-table-btn"
                onclick={() => previewTable(tbl.name)}
                title={`Klik untuk SELECT * FROM ${tbl.name} LIMIT 50`}
              >
                <span class="studio-table-name">{tbl.name}</span>
                <span class="studio-table-meta">
                  {tbl.rows.toLocaleString('id-ID')} baris · {formatBytes(tbl.size_bytes)}
                </span>
              </button>
              <button
                type="button"
                class="studio-opt-btn"
                onclick={() => optimizeTables(tbl.name)}
                disabled={optimizingTable !== ''}
                title={`Optimasi tabel ${tbl.name}`}
                aria-label={`Optimasi tabel ${tbl.name}`}
              >
                <i class={optimizingTable === tbl.name ? 'ri-loader-4-line spin' : 'ri-sparkling-line'} aria-hidden="true"></i>
              </button>
            </div>
          {/each}
        {/if}
      </div>
    </aside>

    <!-- Panel Utama Editor SQL & Grid Hasil -->
    <section class="studio-main">
      <div class="studio-editor-box">
        <div class="studio-toolbar">
          <div class="studio-toolbar-left">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              onclick={() => runQuery()}
              disabled={runningQuery}
            >
              <i class={runningQuery ? 'ri-loader-4-line spin' : 'ri-play-fill'} aria-hidden="true"></i>
              <span>{runningQuery ? 'Menjalankan...' : 'Jalankan SQL (Ctrl+S)'}</span>
            </button>
            <button
              type="button"
              class="btn btn-outline btn-sm"
              onclick={() => {
                queryText = engine === 'postgresql'
                  ? 'SELECT table_name, pg_size_pretty(pg_total_relation_size(quote_ident(table_name))) AS size FROM information_schema.tables WHERE table_schema = \'public\';'
                  : 'SHOW TABLE STATUS;';
              }}
            >
              Status Tabel
            </button>
            <button
              type="button"
              class="btn btn-outline btn-sm"
              onclick={() => (queryText = '')}
            >
              Bersihkan
            </button>
          </div>
          <div class="studio-toolbar-right">
            <span class="studio-table-meta">Batas aman 500 baris · Timeout 15 detik</span>
          </div>
        </div>

        <CodeMirrorEditor
          bind:value={queryText}
          lang="sql"
          minHeight="145px"
          maxHeight="220px"
          placeholder="Tulis kueri SQL Anda di sini (contoh: SELECT * FROM users LIMIT 50;)"
          onSave={() => runQuery()}
        />
      </div>

      <!-- Area Hasil Eksekusi -->
      <div class="studio-result-box">
        <div class="studio-result-bar">
          <span>
            {#if resultCols.length > 0}
              <span class="studio-stat-pill">
                <i class="ri-check-line" aria-hidden="true"></i>
                {resultRows.length} baris · {resultTimeMs.toFixed(1)} ms
              </span>
            {:else if resultNote}
              <span class="studio-stat-pill">
                <i class="ri-check-line" aria-hidden="true"></i>
                Selesai ({resultTimeMs.toFixed(1)} ms)
              </span>
            {:else}
              Hasil Eksekusi Kueri
            {/if}
          </span>
          {#if resultTruncated}
            <span class="studio-warn-pill">
              <i class="ri-alert-line" aria-hidden="true"></i>
              Ditampilkan 500 baris pertama
            </span>
          {/if}
        </div>

        <div class="studio-result-scroll">
          {#if resultError}
            <div class="studio-error">{resultError}</div>
          {:else if resultCols.length > 0}
            <table class="studio-grid-table">
              <thead>
                <tr>
                  {#each resultCols as col}
                    <th>{col}</th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#if resultRows.length === 0}
                  <tr>
                    <td colspan={resultCols.length}>Tidak ada baris data yang cocok.</td>
                  </tr>
                {:else}
                  {#each resultRows as row}
                    <tr>
                      {#each row as cell}
                        <td title={cell}>{cell}</td>
                      {/each}
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          {:else if resultNote}
            <div class="studio-empty">
              <i class="ri-checkbox-circle-line" aria-hidden="true"></i>
              <span>{resultNote}</span>
            </div>
          {:else}
            <div class="studio-empty">
              <i class="ri-terminal-box-line" aria-hidden="true"></i>
              <span>Pilih tabel di kiri untuk pratinjau cepat atau jalankan kueri SQL di atas.</span>
            </div>
          {/if}
        </div>
      </div>
    </section>
  </div>

  {#snippet footer()}
    <button type="button" class="btn btn-outline" onclick={onClose}>
      Tutup SQL Studio
    </button>
  {/snippet}
</Modal>
