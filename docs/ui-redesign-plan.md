# Rencana Perombakan UI/UX fikpanel

Dokumen ini adalah rencana implementasi. Tidak ada kode yang diubah pada tahap audit/perencanaan.
Setiap fase ditulis agar bisa dikerjakan model murah secara mandiri: file yang disentuh, perubahan persis, kriteria selesai, dan perintah verifikasi.

---

## 1. Hasil Audit

### 1.1 Status build dan lint (diukur 2026-11-01)

| Pemeriksaan | Hasil |
|---|---|
| `npm run lint:ui` (`web/scripts/lint-ui.mjs`) | Lolos (47 file) |
| `vite build` (diuji ke folder temp, `dist/` tidak disentuh) | Lolos, 10 detik. `index-*.css` 192 kB, chunk `Sites-*.css` 67 kB |
| `svelte-check --threshold warning` | **148 error + 1 warning**, 34 file bermasalah (editor menampilkan 178; selisihnya kemungkinan hint a11y dari language server, belum bisa diverifikasi tanpa editor) |

Build lolos karena `vite build` tidak melakukan type-check. Semua error berasal dari `svelte-check`/editor.

### 1.2 Sumber masalah di editor (root cause)

| Penyebab | Jumlah | Keterangan |
|---|---|---|
| Tidak ada `svelte.config.js` dan `tsconfig.json` di `web/` | 34 | Pesan `No Svelte configuration found in vite config` di baris 1:1 tiap file. Language server tidak punya preprocessor TypeScript |
| Tipe TypeScript di `<script lang="ts">` | ~114 | Docker 51, WAF 26, PHPManager 11, Security 6, AppStore 5, Assistant 3, SystemManager 2, Monitor 2, sisanya 1 file satu. Pola: `Argument of type ...` (43), `Object literal ... does not exist in type` (37, contoh `confirmLabel` tidak ada di `ConfirmOptions` pada `lib/stores/confirm.svelte.ts`), `Property ... does not exist on type` (24), `possibly undefined/null` (5) |
| Properti CSS tanpa standar | 1 | `AppStore.svelte:1106` `-webkit-line-clamp` tanpa `line-clamp` |

Sebagian besar error TypeScript kemungkinan ikut hilang atau berubah setelah config ada, jadi hitung ulang setelah Fase 0.

### 1.3 Ukuran file dan blok `<style>`

Total blok `<style>` sekitar 17.000 baris yang tersebar di 32 file `.svelte`. File global: `tokens.css` 149, `base.css` 148, `components.css` 505, `utilities.css` 73, `index.css` 17.

| File | Total baris | Baris `<style>` | Inline `style=` |
|---|---|---|---|
| routes/WAF.svelte | 5585 | 2109 | 4 |
| routes/Files.svelte | 3913 | 1549 | 6 |
| routes/Docker.svelte | 3893 | 639 | 19 |
| routes/Databases.svelte | 3833 | 1522 | 2 |
| routes/Security.svelte | 3388 | 958 | 19 |
| lib/components/AddSiteModal.svelte | 3228 | 1269 | 1 |
| routes/PHPManager.svelte | 3021 | 1083 | 0 |
| routes/Sites.svelte | 2792 | 1519 | 0 |
| routes/Terminal.svelte | 2347 | 772 | 0 |
| routes/Cron.svelte | 2149 | 805 | 0 |
| routes/Services.svelte | 1588 | 817 | 9 |
| lib/components/SiteSettings.svelte | 1571 | 142 | 1 |
| routes/Kubernetes.svelte | 1366 | 591 | 0 |
| routes/AppStore.svelte | 1314 | 497 | 7 |
| routes/Home.svelte | 1218 | 611 | 6 |
| routes/Assistant.svelte | 1033 | 527 | 0 |
| lib/components/NginxManager.svelte | 998 | 285 | 0 |
| routes/NodeManager.svelte | 931 | 462 | 0 |
| lib/components/AIProviderModal.svelte | 752 | 292 | 0 |
| routes/Backup.svelte | 629 | 248 | 0 |
| lib/components/Chart.svelte | 611 | 238 | 5 |
| routes/DNSManager.svelte | 610 | 290 | 0 |
| routes/Monitor.svelte | 522 | 238 | 13 |
| routes/SystemManager.svelte | 513 | 193 | 0 |
| routes/Settings.svelte | 400 | 179 | 0 |
| lib/components/Gauge.svelte | 383 | 230 | 0 |
| App.svelte | 315 | 47 | 3 |
| lib/components/HistoryChart.svelte | 292 | 46 | 9 |
| lib/layout/Topbar.svelte | 289 | 150 | 0 |
| lib/components/ManagerModal.svelte | 226 | 154 | 0 |
| lib/layout/Sidebar.svelte | 204 | 142 | 0 |
| lib/components/ui/ConfirmHost.svelte | 186 | 70 | 0 |
| lib/layout/Login.svelte | 184 | 73 | 0 |
| lib/components/ui/ToastHost.svelte | 90 | 65 | 0 |

Total inline `style=`: 104 (Security 19, Docker 19, Monitor 13, Services 9, HistoryChart 9, AppStore 7, Files 6, Home 6, Chart 5, WAF 4, App 3, Databases 2, AddSiteModal 1, SiteSettings 1).

### 1.4 Duplikasi CSS

| Kelompok | Fakta |
|---|---|
| Modal | 17 file mendefinisikan ulang `.modal-backdrop`, 13 `.modal-card`. Ada 5 skema nama (`modal-head/actions`, `modal-header/footer/close-btn`, `modal-title/foot/close`, `.modal` polos, `ssh-modal-*`). Tombol tutup bernama `btn-close`, `close-btn`, `modal-close`, `btn-close-modal` |
| Utility yang sudah ada global tetapi diulang lokal | `.mono` 19 file, `.spin` 15 file, `.text-muted` 11, `.text-subtle` 10, `.truncate` 2, `.tabular` 2 |
| Utility yang dipakai lokal tetapi belum ada global | `.font-bold`, `.text-accent`, `.items-center`, `.mt-2`, `.mt-4`, `.w-full` |
| Tombol | `.btn` lokal di 6 file (padding `.45rem .85rem`, radius `6px` hardcoded). `.btn-primary`/`.btn-ghost`/`.btn-mini`/`.sub-btn` di AddSiteModal, SiteSettings, dan lainnya. `.btn-icon` (8 file) vs global `.btn--icon`. Kelas sekali pakai: `tool-btn`, `studio-btn`, `btn-icon-xs`, `btn-link`, `link-btn`, `act-btn`, `filter-btn`, `chip-btn`, `tz-btn` |
| Form | `.form-group`, `.form-label`, `.form-input` paralel dengan global `.field`, `.field__label`, `.input`. `.field` juga didefinisikan ulang di Files dan Settings |
| Tabs | 7+ varian (`sec-nav-bar`, `.sub-tab`, `.tab-btn` x3 berbeda, `.seg-btn`, strip Docker, sub-tab WAF, sidebar tab PHP). Tidak ada `.tabs` global |
| Komponen lain | `.badge`, `.badge-tag` (3 varian), `.empty-state` (Services memakai `!important`), `.stat-card/label/value`, `.panel*`, `.page-title/.page-sub`, `.search-icon` (9 file), `.table-wrap/.table-container`, `.status-dot/pill/indicator`, `.log-*` |
| Keyframes | `spin` 16 salinan (padahal `fp-spin` ada di `base.css`), `pulse` 2, `fp-spin` 2, plus belasan animasi fade/pop/slide per file yang menduplikasi `fp-fade/fp-pop/fp-slide-*` di `base.css` |
| Breakpoint | 8 nilai `max-width` berbeda (640, 720, 768, 820, 860, 900, 960, 1024) padahal standar proyek hanya 768 dan 1024. `@media` hanya ada di App, Assistant, Security, WAF, Sidebar, Topbar dan beberapa lainnya. Mayoritas halaman tanpa breakpoint |

### 1.5 Inkonsistensi UI

| Area | Temuan |
|---|---|
| Modal | Tidak ada komponen Modal bersama. Esc dan `aria-modal` tidak konsisten: Docker (10 modal), Databases (8), Security (3), PHP, Services, NodeManager tanpa handler Esc. Docker memakai `svelte-ignore a11y_click_events_have_key_events` |
| Konfirmasi | Sites membuat modal hapus sendiri padahal `confirmDialog` mendukung `requireText`. Home dan Services memanggil aksi tanpa konfirmasi (perlu dicek apakah destruktif, **jangan diubah di fase UI**, catat saja) |
| Toast | Terminal memanggil `toast.*` langsung, halaman lain lewat prop `onNotify` |
| State loading/kosong/error | `.skeleton` hanya dipakai di loader rute `App.svelte`. `.empty-state` dipakai 8 halaman, sisanya markup bebas atau tidak ada. Retry hanya di Docker, Files, PHP, Services |
| Tabel | `.table` global dipakai 5 halaman, sisanya tabel buatan sendiri |
| Ikon | Emoji tersisa di Cron (~21), WAF, Assistant, NginxManager. Sisanya Remix Icon |
| Warna | 72 literal warna mentah di luar `tokens.css` (Files 32, Chart 10, Sites 6, WAF 5, HistoryChart 4, Gauge 3). Linter `UI001` lolos hanya karena melewatkan sebagian pola |
| **Variabel CSS tidak terdefinisi** | Dipakai tetapi tidak ada di `tokens.css`: `--border-color` x36, `--text-primary` x28, `--border-soft` x23, `--bg-canvas` x17, `--bg-surface-soft` x14, `--primary` x14, `--status-online` x10, `--bg-card` x10, `--ring-warning` x8, `--shadow-modal` x6, `--status-offline` x6, `--ring-info` x5, `--bg-base` x3, `--leading-relaxed` x2, `--border-hover` x1. Ini bug visual senyap (nilai jatuh ke `initial`) |
| Shell | Sidebar tanpa pencarian, tanpa grup yang bisa dilipat. Topbar tanpa pencarian global, tanpa toggle tema, logout hanya di footer sidebar. Dark saja (`color-scheme: dark`). `Backup.svelte` tidak ada di navigasi (alias ke `databases`), kemungkinan kode warisan |
| Dokumen basi | `.agents/rules/ui-ux-standards.md` menyatakan `<style>` route boleh dipakai untuk layout halaman, bertentangan dengan target ini. Komentar `Security.svelte:690` menyebut "10 modul" padahal hanya 7 tab |

---

## 2. Keputusan Desain dan Arsitektur

### 2.1 Arah desain

Referensi dari pengguna dipakai sebagai dasar, dengan perubahan berikut:

| Referensi pengguna | Keputusan | Alasan |
|---|---|---|
| Sidebar dikelompokkan + pencarian cepat | **Dipakai, ditambah Command Palette** (`Ctrl+K`) yang mencari rute dan aksi. Sidebar tetap dikelompokkan dan grup bisa dilipat (status disimpan di `localStorage`). Pencarian di sidebar diganti pemicu palette | 18 menu terlalu panjang untuk filter inline di sidebar sempit. Palette juga bekerja di mode ikon (tablet) dan mobile |
| Dashboard ringkas dengan kartu status | **Dipakai**: kartu status (CPU, RAM, Disk, Uptime, layanan, ancaman) memakai komponen `.stat-card` bersama | Satu komponen dipakai Home, Docker, Services, WAF, PHP |
| Design token terpusat | **Dipakai dan diperluas**. `tokens.css` sudah ada (90 variabel), dilengkapi variabel yang hilang dan token breakpoint | Sudah sesuai standar proyek |
| Dark mode | **Dark + Light + Auto** lewat atribut `data-theme` di `<html>` dan toggle di Topbar. Default mengikuti `prefers-color-scheme`, pilihan disimpan di `localStorage` | Saat ini dark saja. Semua warna sudah variabel, jadi light mode murah |
| Tabel/form konsisten | **Dipakai**: satu `.table`, satu `.field`, satu `.input`, satu `.tabs`, satu `.modal` | Menghilangkan 7 varian tab dan 5 skema modal |
| Loading/kosong/error, responsif | **Dipakai**: komponen state bersama (`.state--loading/--empty/--error` + `.skeleton`) dan dua breakpoint tetap | Hanya 2 dari ~20 halaman punya pola ini |

Prinsip visual: densitas tinggi ala panel hosting (tabel padat, kartu datar), aksen satu warna (`--accent`), status memakai warna semantik (`--success/--warning/--danger/--info`), radius dan bayangan dari token, ikon hanya Remix Icon (emoji dihapus).

### 2.2 Arsitektur CSS

Memakai `@layer` agar urutan prioritas deterministik dan tidak bergantung urutan impor. Karena CSS tidak lagi scoped, setiap file halaman dibungkus di bawah satu kelas akar.

```
web/src/styles/
  index.css            Titik masuk. Deklarasi urutan layer + @import semua file global
  tokens.css           @layer tokens      :root + [data-theme="light"] + [data-theme="dark"]
  base.css             @layer base        reset, tipografi, fokus, keyframes bersama (fp-*)
  layout.css           @layer layout      app shell, sidebar, topbar, login, palette, page container
  components/          @layer components  satu file per komponen, impor via components/index.css
    button.css  form.css  card.css  stat-card.css  table.css  badge.css  alert.css
    tabs.css  modal.css  toolbar.css  page-header.css  state.css  toast.css  confirm.css
    switch.css  progress.css  chart.css
  utilities.css        @layer utilities   .stack .row .grow .mono .truncate .sr-only .text-* .mt-* ...
  pages/               @layer pages       satu file per halaman/komponen besar
    home.css  sites.css  ...  security/overview.css, security/firewall.css, ...
```

Deklarasi di baris pertama `index.css`:

```css
@layer vendor, tokens, base, layout, components, utilities, pages;
```

Impor CSS vendor (`remixicon`, `leaflet`, `uplot`, font) memakai `@import url(...) layer(vendor);` atau `import` dengan layer, agar tidak mengalahkan layer kita. **Pitfall:** CSS di luar layer selalu menang atas CSS ber-layer, jadi tidak boleh ada CSS tanpa layer di proyek (kecuali vendor yang sengaja di-layer).

**Strategi penamaan (anti-bentrok):**

| Lapisan | Pola | Contoh |
|---|---|---|
| Komponen bersama | BEM tanpa prefix halaman: `.blok`, `.blok__elemen`, `.blok--modifier` | `.btn`, `.btn--primary`, `.modal__head`, `.field__label`, `.tabs__tab` |
| Halaman | Kelas akar `pg-<rute>` pada elemen akar halaman. **Semua** aturan file halaman bersarang di bawah akar tersebut. Nama kelas dalam halaman boleh tetap seperti semula | `.pg-sites { .toolbar { ... } }` |
| Komponen besar di `lib/components` | Akar `cmp-<nama>` | `.cmp-add-site`, `.cmp-nginx-manager` |
| Subkomponen hasil pecahan | Akar `pg-<rute>-<bagian>` | `.pg-security-firewall` |
| Utility | Nama pendek tanpa prefix, hanya di layer `utilities`, tanpa deklarasi spesifik halaman | `.mt-2`, `.text-muted` |
| State/JS hook | `is-*`/`has-*` hanya di bawah blok pemiliknya | `.tabs__tab.is-active` |
| Keyframes | Hanya yang ada di `base.css` (`fp-spin`, `fp-fade`, `fp-pop`, `fp-slide-in`, `fp-slide-down`, `fp-pulse`, `fp-shimmer`). Keyframes khusus halaman diberi prefiks `<akar>-` (mis. `pg-waf-laser`) | Tidak boleh ada nama generik `spin`, `fadeIn` |

Alasan memilih akar bersarang dan bukan mengganti nama setiap kelas: spesifisitas aturan tetap `(0,2,0)` seperti hasil scoped Svelte (kelas + hash), sehingga tampilan tidak bergeser, dan model murah tidak perlu menulis ulang ribuan nama kelas.

**Cara halaman memuat CSS:** tiap halaman mengimpor file CSS-nya di `<script>`: `import '../styles/pages/sites.css';`. Vite memecahnya per chunk rute, jadi lazy-loading yang sekarang tetap terjaga. Itu bukan blok `<style>`, jadi aturan "tidak ada `<style>`" tetap terpenuhi.

**Kompatibilitas nesting:** `vite.config.mjs` ditambah `build.cssTarget: ['chrome108','safari15.4','firefox102']` sehingga esbuild menurunkan nesting menjadi `:is()` untuk browser lama.

**Tidak ada blok `<style>` tersisa.** Tidak ada pengecualian. Kasus dinamis (lebar bar persen, warna per item) memakai `style="--w: {x}%"` (custom property saja) dan CSS membacanya lewat `var(--w)`.

### 2.3 Token baru dan alias sementara

| Tindakan | Detail |
|---|---|
| Alias sementara (hapus di Fase 9) | `--border-color`→`--border`, `--text-primary`→`--text`, `--border-soft`→`--border-subtle`, `--bg-canvas`→`--bg-app`, `--bg-surface-soft`→`--bg-raised`, `--primary`→`--accent`, `--bg-card`→`--bg-surface`, `--bg-base`→`--bg-app`, `--border-hover`→`--border-strong`, `--status-online`→`--success`, `--status-offline`→`--danger`, `--leading-relaxed`→`1.65` |
| Token baru | `--ring-info`, `--ring-warning`, `--shadow-modal`, `--space-7`, `--space-9`, `--bp-tablet: 1024px`, `--bp-mobile: 768px` (hanya dokumentasi, `@media` tetap literal), `--bg-scrim`, `--chart-1..6`, `--z-palette` |
| Tema | `[data-theme="light"]` meniru semua token warna. `color-scheme` mengikuti tema |

### 2.4 Pemetaan rename (dipakai semua fase halaman)

| Lama (lokal) | Baru (global) |
|---|---|
| `.btn-primary`, `.btn-ghost`, `.btn-danger`, `.btn-secondary` | `.btn .btn--primary` / `--ghost` / `--danger` / `--secondary` |
| `.btn-icon`, `.btn-icon-xs`, `.btn-mini`, `.sub-btn` | `.btn .btn--icon` (+ `.btn--sm` untuk ukuran kecil). Wajib `aria-label` |
| `.btn-close`, `.close-btn`, `.modal-close`, `.btn-close-modal`, `.modal-close-btn` | `.modal__close` |
| `.modal-backdrop`, `.modal-card`, `.modal`, `.ssh-modal-*` | komponen `Modal.svelte` (`.modal`, `.modal__panel`) |
| `.modal-head`, `.modal-header` | `.modal__head` |
| `.modal-title`, `.modal-sub` | `.modal__title`, `.modal__sub` |
| `.modal-body` | `.modal__body` |
| `.modal-foot`, `.modal-footer`, `.modal-actions` | `.modal__foot` |
| `.modal-lg`, `.modal-sm` | `.modal--lg`, `.modal--sm` |
| `.form-group` | `.field` |
| `.form-label` | `.field__label` |
| `.form-input`, `.search-input` | `.input` (+ `.input--search` bila perlu ikon) |
| `.tab-btn`, `.sub-tab`, `.seg-btn`, `.sec-nav-bar` | `.tabs` / `.tabs__tab` / `.is-active` |
| `.stat-card`, `.stat-label`, `.stat-value`, `.stat-val`, `.stat-icon` | `.stat-card`, `.stat-card__label`, `.stat-card__value`, `.stat-card__icon` |
| `.page-title`, `.page-sub`, `.subtitle`, `.head-bar` | `.page-header__title`, `.page-header__sub`, `.page-header` |
| `.table-wrap`, `.table-container` | `.table-wrap` + `.table` |
| `.panel`, `.panel-card`, `.panel-head`, `.panel-title` | `.card`, `.card__head`, `.card__title` |
| `.badge-tag`, `.badge-type`, `.chip`, `.status-pill` | `.badge` + `--success/--warning/--danger/--info/--neutral` |
| `.status-dot`, `.status-indicator` | `.dot .dot--success` dst. |
| `.empty-state`, `.empty-title` | `.state .state--empty`, `.state__title` |
| `.search-icon` | `.input__icon` |
| `.spin`, `.spinner` | `.spinner` (global), animasi `fp-spin` |
| `.mono`, `.truncate`, `.tabular`, `.text-muted`, `.text-subtle`, `.text-accent`, `.font-bold`, `.items-center`, `.mt-*`, `.w-full` | utility global (ditambahkan di Fase 2) |

---

## 3. Aturan Umum untuk Semua Fase

1. Fungsionalitas, rute, dan logika `<script>` tidak boleh berubah. Perubahan di `<script>` hanya: tambah `import '...css'`, perbaikan tipe TS murni (Fase 0), dan nama kelas di string yang dipakai sebagai kelas CSS.
2. Jangan menjalankan `migrate:fresh`, `db:wipe`, atau operasi DB apa pun. Tidak relevan di rencana ini.
3. Semua komentar kode dalam Bahasa Indonesia.
4. Tiga perintah verifikasi standar (jalankan dari `c:\Project\Fikfikk\fikpanel\web`):

   | Nama | Perintah |
   |---|---|
   | `V-LINT` | `npm run lint:ui` |
   | `V-CHECK` | `npm run check` (dibuat di Fase 0) |
   | `V-BUILD` | `npx vite build --outDir "$env:TEMP\fp-dist" --emptyOutDir` (tidak mengotori `web/dist`, yang di-embed oleh `embed.go`) |

5. Pencarian blok style tersisa: `Select-String -Path .\src\*.svelte,.\src\*\*.svelte,.\src\*\*\*.svelte,.\src\*\*\*\*.svelte -Pattern '<style'`
6. Satu fase = satu commit conventional (`refactor(web): ...`). Jangan gabungkan fase.
7. Verifikasi visual tidak bisa otomatis. Setiap fase halaman diakhiri **checklist visual manual** (lihat 3.1). Model pelaksana wajib menuliskan hasilnya di deskripsi commit, atau minta pengguna memeriksa (`/browser` bisa dipakai bila dev server dan backend berjalan).

### 3.1 Checklist visual manual (per halaman)

Buka halaman pada lebar 1440, 900, dan 390 px, dalam tema dark dan light. Pastikan: tidak ada overflow horizontal, tabel bisa digulir di dalam `.table-wrap`, modal terbuka/tutup dengan klik backdrop dan Esc, tombol punya state hover/disabled/loading, state kosong tampil, tidak ada teks hitam di atas hitam (tanda variabel hilang).

### 3.2 Prosedur baku "Migrasi Halaman" (disebut SOP-HALAMAN di fase-fase bawah)

Untuk setiap file `.svelte` X dengan akar `pg-x` (atau `cmp-x`):

1. Buat `web/src/styles/pages/x.css` (komponen: `styles/pages/components/x.css`) dengan isi:
   ```css
   @layer pages {
     .pg-x {
       /* isi blok <style> lama, apa adanya */
     }
   }
   ```
2. Salin isi `<style>` lama persis. Perlakuan khusus:
   - Buka `:global(sel)`: hasilnya `sel` biasa (di dalam akar). Jika `:global(body|html|:root)`, pindahkan ke `base.css`/`layout.css`.
   - Selektor yang menyasar elemen akar itu sendiri ditulis `&.kelas` atau `&`.
   - `@keyframes` lokal: hapus bila duplikat `fp-*` dan ganti referensi `animation:` ke nama `fp-*`; bila unik, pindahkan ke luar akar dan beri prefiks `pg-x-`.
   - `@media`: tetap di dalam akar; samakan nilai ke 1024px atau 768px terdekat.
3. Beri kelas akar. Jika halaman sudah punya elemen akar tunggal, tambahkan kelas `pg-x`. Jika tidak, bungkus dengan `<div class="pg-x">` dan tambahkan `display: contents;` pada `.pg-x` di CSS.
4. Hapus blok `<style>…</style>` dari `.svelte`. Tambahkan `import '../styles/pages/x.css';` di `<script>` (path disesuaikan).
5. **Hapus duplikat**: untuk setiap aturan yang cocok dengan tabel 2.4, hapus aturan lokal, ganti nama kelas di markup ke versi global. Jika aturan lokal punya deklarasi unik yang memang diperlukan halaman, simpan hanya selisihnya sebagai penyesuaian di bawah akar.
6. Ganti warna mentah dengan `var(--token)` (atau `color-mix(in srgb, var(--token) N%, transparent)`). Ganti variabel tidak terdefinisi dengan token yang benar sesuai alias di 2.3.
7. Ganti `style="..."` statis dengan kelas utility. Yang dinamis diubah menjadi custom property (`style="--w: {pct}%"`).
8. Ganti modal buatan sendiri dengan `<Modal>` (setelah Fase 4), tab dengan `.tabs`, state kosong dengan `.state`, tanpa mengubah handler, `bind:`, atau kondisi `{#if}`.
9. Verifikasi: `V-LINT`, `V-CHECK` (tidak boleh menambah error), `V-BUILD`, pencarian `<style` pada file tersebut kosong, checklist visual 3.1.

Kriteria selesai SOP-HALAMAN: tidak ada `<style>` di file; file CSS ada dan semua aturannya bersarang di akar; tidak ada kelas di tabel 2.4 yang masih didefinisikan lokal; tiga verifikasi lolos; ukuran CSS halaman berkurang (catat sebelum/sesudah).

---

## 4. Fase Implementasi

Tier model yang disarankan: `cheap` untuk mekanis, `standard` untuk perubahan sedang, `capable` untuk fase yang memerlukan pertimbangan desain. Catatan: saat audit ini, worker pitroom gagal (lihat bagian 6), jadi eksekusi dapat memakai subagent bawaan atau pelaksana lain sampai pitroom diperbaiki.

### Fase 0: Fondasi tooling, aturan, dan nol error editor

Tier: standard. Tidak ada perubahan tampilan.

| Item | Detail |
|---|---|
| File dibuat | `web/svelte.config.js` (`vitePreprocess()` dari `@sveltejs/vite-plugin-svelte`), `web/tsconfig.json` (extends `@tsconfig/svelte` atau konfigurasi `moduleResolution: bundler`, `strict`, `verbatimModuleSyntax`, `include: ["src/**/*"]`), `web/scripts/extract-style.mjs` |
| File diubah | `web/package.json` (devDependency `svelte-check`, `@tsconfig/svelte`; script `"check": "svelte-check --tsconfig ./tsconfig.json --threshold warning"`), `web/vite.config.mjs` (tambah `build.cssTarget`), `web/scripts/lint-ui.mjs`, `.agents/rules/ui-ux-standards.md` |
| `extract-style.mjs` | CLI `node scripts/extract-style.mjs <file.svelte> <root-class> <css-output>`: mengambil semua blok `<style>`, membuka `:global(...)`, membungkus dengan `@layer pages { .<root> { ... } }`, menulis ke file output, menghapus blok dari `.svelte`, menambah `import` di `<script>`. Tidak mengganti nama kelas dan tidak dedup (itu manual di SOP-HALAMAN langkah 5-8) |
| Aturan lint baru | `UI002`: blok `<style>` di `.svelte` dilarang. `UI003`: tiap file di `styles/pages/**` harus berupa satu `@layer pages { .<nama-file-atau-akar> { … } }` (selain `@keyframes` berprefiks). `UI004`: `style="..."` hanya boleh berisi custom property (`--x:`) atau ekspresi `{}` murni. `UI005`: kelas `modal-*`, `form-*`, `btn-*` (selain `btn--`) dilarang di markup. Aktifkan `UI002-UI005` sebagai **peringatan** dulu (tidak menggagalkan build) dan ubah menjadi error di Fase 9 |
| Dokumen aturan | Perbarui `ui-ux-standards.md`: hapus izin `<style>` di route, tambahkan arsitektur layer, penamaan, SOP-HALAMAN, peta rename |
| Perbaikan tipe TS | Hanya anotasi/tipe, tanpa mengubah perilaku. Urutan sub-langkah (satu commit masing-masing): 0a config + `npm run check` berjalan, catat jumlah error baru; 0b `lib/stores/confirm.svelte.ts` (`ConfirmOptions` menerima `confirmLabel` bila dipakai pemanggil, periksa dulu apakah itu typo dari `confirmText`; ikuti yang dipakai store), AppStore, Assistant, SystemManager, Monitor; 0c Docker.svelte; 0d WAF.svelte; 0e PHPManager.svelte, Security.svelte, sisanya; 0f `AppStore.svelte:1106` tambah `line-clamp` (hilang sendiri di Fase 6) |

Kriteria selesai: `npm run check` menghasilkan 0 error dan 0 warning; `V-LINT` dan `V-BUILD` lolos; diff `<script>` hanya tipe.
Verifikasi: `npm install; npm run check; npm run lint:ui; V-BUILD`.

### Fase 1: Design tokens dan arsitektur layer

Tier: capable.

| Item | Detail |
|---|---|
| File | `styles/index.css`, `styles/tokens.css`, `styles/base.css`, `web/src/main.ts` (hanya jika urutan impor perlu), `lib/layout/Topbar.svelte` TIDAK disentuh di fase ini |
| Perubahan | 1) `index.css` baris pertama `@layer vendor, tokens, base, layout, components, utilities, pages;` lalu `@import` tiap file ke layer-nya (`@import './tokens.css' layer(tokens);` dst). Impor font/remixicon/leaflet/uplot ke `layer(vendor)`. 2) `tokens.css`: bungkus ke `@layer tokens`, tambahkan token baru dan alias sementara (2.3), tambahkan blok `[data-theme="light"]` dan `@media (prefers-color-scheme: light) { :root:not([data-theme]) {...} }`; `color-scheme` per tema. Nilai light mengikuti kontras WCAG AA (teks 4.5:1) 3) `base.css`: bungkus ke `@layer base`, pastikan semua keyframes `fp-*` ada (`fp-spin`, `fp-fade`, `fp-pop`, `fp-slide-in`, `fp-slide-down`, `fp-pulse`, `fp-shimmer`), tambahkan skrip kecil di `index.html` (inline `<script>` sebelum bundle) yang menerapkan `data-theme` dari `localStorage` agar tidak berkedip |
| Pitfall | Karena semua CSS kini ber-layer, CSS `<style>` Svelte yang belum dimigrasi (tanpa layer) akan MENANG atas semuanya sampai fase halaman selesai. Itu aman dan memang diharapkan selama transisi |

Kriteria selesai: tampilan aplikasi identik dengan sebelumnya (dark), tidak ada `var(--…)` tak terdefinisi: jalankan `Select-String -Path .\src -Recurse -Pattern 'var\(--(border-color|text-primary|border-soft|bg-canvas|bg-surface-soft|primary|status-online|status-offline|bg-card|bg-base|ring-info|ring-warning|shadow-modal|leading-relaxed|border-hover)\)'` dan pastikan tiap nama kini ada di `tokens.css` sebagai alias.
Verifikasi: `V-LINT`, `V-CHECK`, `V-BUILD`; buka satu halaman dan bandingkan visual.

### Fase 2: CSS global: base, utilities, keyframes

Tier: cheap.

| Item | Detail |
|---|---|
| File | `styles/utilities.css`, `styles/base.css` |
| Perubahan | Bungkus `utilities.css` ke `@layer utilities`. Tambahkan utility yang dipakai lokal tetapi belum ada: `.font-bold`, `.font-medium`, `.text-accent`, `.text-success`, `.text-warning`, `.text-danger`, `.items-center`, `.justify-between`, `.gap-1..4`, `.mt-1..4`, `.mb-1..4`, `.w-full`, `.hidden`, `.flex`, `.flex-wrap`. Nilai dari token `--space-*` |
| Catatan | Hanya menambah. Belum menghapus apa pun dari halaman |

Kriteria selesai: tiap utility di atas ada; `V-BUILD` lolos.
Verifikasi: `V-LINT`, `V-BUILD`.

### Fase 3: Layout shell (sidebar, topbar, login, App)

Tier: capable. Fase ini satu-satunya yang mengubah struktur navigasi.

| Item | Detail |
|---|---|
| File | `App.svelte`, `lib/layout/Sidebar.svelte`, `lib/layout/Topbar.svelte`, `lib/layout/Login.svelte`, `lib/layout/nav.ts`, baru: `lib/layout/CommandPalette.svelte`, `lib/stores/theme.svelte.ts`, `styles/layout.css` |
| CSS | Pindahkan blok `<style>` keempat file ke `styles/layout.css` di `@layer layout` memakai akar `.app-shell`, `.sidebar`, `.topbar`, `.login`, `.palette` (BEM, bukan `pg-`). Hapus 3 inline `style=` di App |
| Sidebar | Grup yang bisa dilipat (status di `localStorage` kunci `fp:nav-collapsed`), lencana opsional, tombol pemicu Command Palette di atas daftar. Mode ikon 769-1024 px, drawer di bawah 768 px (perilaku sekarang dipertahankan) |
| Pengelompokan menu (`nav.ts`, **id rute tidak berubah**, hanya properti grup dan urutan) | Ringkasan: home. Website: website, php, appstore. Data: databases, files. Infrastruktur: docker, k8s, node, services, cron, terminal. Keamanan: waf, security. Sistem: monitor, system, assistant, settings. Pengaturan `routeFromHash` dan alias `backup→databases` tetap |
| Topbar | Tambah toggle tema (dark/light/auto, memakai `theme.svelte.ts`), tombol Command Palette (`Ctrl+K`), menu pengguna berisi Logout (memanggil handler logout yang sama dengan sidebar) |
| CommandPalette | Dialog `role="dialog" aria-modal`, input fokus otomatis, daftar rute dari `nav.ts` terfilter fuzzy sederhana, panah atas/bawah + Enter, Esc menutup, navigasi lewat `location.hash` seperti sekarang |
| Login | Hanya CSS dipindah dan dirapikan dengan komponen `.field`/`.btn`; tidak mengubah alur autentikasi |

Kriteria selesai: tidak ada `<style>` di keempat file; semua 18 rute tetap bisa dibuka dari sidebar dan palette; toggle tema bekerja dan tersimpan; `Ctrl+K` membuka palette; fokus keyboard terlihat; tanpa overflow di 390 px.
Verifikasi: `V-LINT`, `V-CHECK`, `V-BUILD`; checklist 3.1 pada Home.

### Fase 4: Komponen bersama (CSS + komponen Svelte kecil)

Tier: capable untuk 4a, standard untuk sisanya. Dikerjakan berurutan; satu commit tiap sub-fase.

| Sub | File | Isi |
|---|---|---|
| 4a | `styles/components/button.css`, `form.css`, `card.css`, `stat-card.css`, `index.css` komponen; hapus `styles/components.css` setelah isinya dipindah | Pecah `components.css` (505 baris) per komponen tanpa mengubah nilai. Tambahkan varian yang dibutuhkan halaman: `.btn--sm`, `.btn--xs`, `.btn--link`, `.btn--loading`, `.input--search`, `.input__icon`, `.field__hint`, `.field__error`, `.card__head/__title/__body`, `.stat-card*` |
| 4b | `styles/components/table.css`, `badge.css`, `alert.css`, `state.css`, `progress.css` | `.table` (sticky head, hover baris, `.table--dense`), `.table-wrap` (overflow-x auto, border, radius), `.badge--*`, `.dot--*`, `.alert--*`, `.state` (+`--loading/--empty/--error`, ikon, judul, deskripsi, aksi), `.skeleton`, `.progress` |
| 4c | `styles/components/tabs.css` | `.tabs` (varian `--pill`, `--underline`, `--vertical` untuk sidebar tab PHP/Manager), `.tabs__tab`, `.is-active`, scroll horizontal di mobile |
| 4d | `styles/components/modal.css`, `lib/components/ui/Modal.svelte` | Komponen Svelte 5 dengan props `open`, `title`, `size` (`sm/md/lg/xl`), `onclose`, snippet `children`, `footer`. Fitur: backdrop, klik backdrop menutup, Esc, `role="dialog" aria-modal="true" aria-labelledby`, fokus pertama masuk dan kembali ke pemicu, kunci scroll body, animasi `fp-pop`. z-index `--z-modal`. Tanpa `<style>` |
| 4e | `styles/components/toolbar.css`, `page-header.css`, `lib/components/ui/PageHeader.svelte` | `.toolbar` (baris aksi + pencarian + filter, wrap di mobile), `.page-header` (judul, sub, aksi) |
| 4f | `styles/components/toast.css`, `confirm.css`; `ToastHost.svelte`, `ConfirmHost.svelte` | Pindahkan CSS keduanya (SOP-HALAMAN dengan akar `.toast-host`, `.confirm`). Logika store tidak diubah |

Kriteria selesai 4a-4f: file CSS ada dan dimuat; `components.css` lama terhapus; tidak ada perubahan visual di halaman yang belum dimigrasi (CSS lokal halaman masih menang); `Modal.svelte` dicoba pada satu halaman uji sementara lalu dihapus uji-nya.
Verifikasi: `V-LINT`, `V-CHECK`, `V-BUILD`.

### Fase 5: Komponen visual kecil

Tier: cheap. Terapkan SOP-HALAMAN (akar `cmp-<nama>`) pada: `Gauge.svelte`, `Chart.svelte` (ganti 10 warna mentah dengan `--chart-1..6`/token, 5 inline `style=`), `HistoryChart.svelte` (9 inline `style=`), `ManagerModal.svelte` (ubah memakai `Modal.svelte` bila memungkinkan tanpa mengubah API props/snippet). Satu commit per file.

Kriteria selesai: tiap file tanpa `<style>`; grafik tetap tampil benar di dark dan light.
Verifikasi: `V-LINT`, `V-CHECK`, `V-BUILD`, checklist 3.1 pada Home dan Monitor.

### Fase 6: Halaman, dari kecil ke besar

Setiap baris = satu fase mandiri memakai SOP-HALAMAN (3.2). Akar = `pg-<nama-lowercase>`. Dikerjakan **berurutan dari atas ke bawah**. Kolom "Catatan khusus" berisi hal tambahan di luar SOP.

| Fase | File | Baris CSS | Tier | Catatan khusus |
|---|---|---|---|---|
| 6.1 | `routes/Settings.svelte` | 179 | cheap | `.field` lokal dihapus, pakai global. `.form-input` → `.input` |
| 6.2 | `routes/SystemManager.svelte` | 193 | cheap | 3 `tab-btn` → `.tabs`. Perbaikan tipe sudah di Fase 0 |
| 6.3 | `routes/Monitor.svelte` | 238 | cheap | 13 inline `style=` → utility/custom property. `.sub-tab`/`.filter-btn` → `.tabs` |
| 6.4 | `routes/Backup.svelte` | 248 | cheap | Tidak ada di navigasi (alias ke `databases`). **Jangan hapus file**; hanya migrasi CSS. Catat di commit bahwa kemungkinan kode warisan untuk keputusan pengguna |
| 6.5 | `routes/DNSManager.svelte` | 290 | cheap | Pakai `.modal`, `.table`, `.state` |
| 6.6 | `routes/NodeManager.svelte` | 462 | cheap | `.form-group/.form-label/.form-input` → `.field` dst. |
| 6.7 | `routes/Home.svelte` | 611 | capable | **Redesain dashboard**: baris kartu status (`.stat-card`: CPU, RAM, Disk, Uptime, Layanan, Ancaman), di bawahnya grid `Gauge`/`HistoryChart`, tabel layanan memakai `.table`, state kosong memakai `.state`. Data dan fetch tidak berubah, hanya penataan markup dan kelas |
| 6.8 | `routes/Assistant.svelte` | 527 | standard | Emoji (l.208, l.449) → Remix Icon. Breakpoint 768 |
| 6.9 | `routes/AppStore.svelte` | 497 | standard | `.search-icon` → `.input__icon`. Toggle grid/tabel → `.tabs--pill` atau `.segmented`. Hapus `-webkit-line-clamp` ganda dengan `line-clamp` |
| 6.10 | `routes/Kubernetes.svelte` | 591 | standard | `.card` lokal memakai `--bg-card` yang tak terdefinisi, hapus dan pakai `.card` global. `.tab-btn` → `.tabs` |
| 6.11 | `routes/Services.svelte` | 817 | standard | `.empty-state` dengan `!important` dihapus. 9 inline `style=`. Aksi layanan tanpa konfirmasi: jangan ubah, catat |
| 6.12 | `routes/Cron.svelte` | 805 | standard | Emoji di string deskripsi jadwal (l.195-322) dan `<option>` (l.1143-1148): hapus emoji tanpa mengubah nilai `value` atau logika string. `.tab-btn` → `.tabs` |
| 6.13 | `routes/Terminal.svelte` | 772 | standard | Panggilan `toast.*` langsung dibiarkan (logika). Modal → `Modal` |
| 6.14 | `routes/Sites.svelte` | 1519 | capable | Pecah CSS jadi `pages/sites.css` saja (pemecahan komponen di Fase 8). Modal hapus buatan sendiri tetap logikanya, hanya tampilan ke `.modal`. `.badge-tag` → `.badge` |
| 6.15 | `routes/PHPManager.svelte` | 1083 | capable | `ssh-modal-*` → `Modal size="xl"` dengan `.tabs--vertical` di dalam. 10 tab panel tidak diubah logikanya |
| 6.16 | `routes/Security.svelte` | 958 | capable | Dikerjakan di belakang dan **dipecah** (lihat Fase 8). Di fase ini hanya CSS: `pages/security.css`. 19 inline `style=`. `sec-nav-bar` → `.tabs`. Perbaiki komentar basi "10 modul" menjadi "7 modul" |
| 6.17 | `routes/Databases.svelte` | 1522 | capable | 8 modal → `Modal`. Engine manager boleh memakai `ManagerModal` |
| 6.18 | `routes/Docker.svelte` | 639 | capable | 10 modal → `Modal` (menyelesaikan masalah Esc dan `aria-modal`, hapus `svelte-ignore a11y_click_events_have_key_events`). 19 inline `style=`. 10 tab → `.tabs` dengan scroll |
| 6.19 | `routes/Files.svelte` | 1549 | capable | 32 warna mentah → token. ~11 modal → `Modal`. Context menu dan code studio tetap di halaman, CSS di bawah akar. Toolbar → `.toolbar` |
| 6.20 | `routes/WAF.svelte` | 2109 | capable | Keyframes unik (`pulseScale`, `dashLaser`, `pulseGlow`, `slideInRight`) diberi prefiks `pg-waf-`. 5 modal → `Modal`. Pecah CSS per bagian di Fase 8 |

Kriteria selesai tiap fase 6.x: kriteria SOP-HALAMAN + `Select-String '<style'` pada file kosong + CSS halaman lebih kecil dari semula.
Verifikasi tiap fase: `V-LINT`, `V-CHECK`, `V-BUILD`, checklist 3.1.

### Fase 7: Komponen modal besar di `lib/components`

Tier: capable. SOP-HALAMAN dengan akar `cmp-<nama>`, satu commit per file.

| Fase | File | Baris CSS | Catatan |
|---|---|---|---|
| 7.1 | `AIProviderModal.svelte` | 292 | `modal-*` lokal → `Modal` |
| 7.2 | `NginxManager.svelte` | 285 | Duplikat `.switch`, `spin`, `slide` dihapus |
| 7.3 | `SiteSettings.svelte` | 142 | `.btn-primary/.sub-btn/.btn-mini` → `.btn` varian |
| 7.4 | `AddSiteModal.svelte` | 1269 | Terbesar. Ganti `.btn-primary/.btn-ghost`, duplikat `.switch`, `fade/pop/spin`. Wizard memakai `Modal size="xl"` dan indikator langkah memakai `.tabs--pill` atau `.stepper` bersama (tambahkan ke `components/tabs.css`) |

Kriteria selesai: sama seperti Fase 6. Verifikasi: sama; uji juga alur "Tambah situs" sampai langkah terakhir secara visual.

### Fase 8: Memecah halaman raksasa menjadi subkomponen

Tier: capable. Hanya setelah Fase 6 untuk halaman tersebut selesai. Aturan: **state dan fungsi tetap di halaman induk**; subkomponen menerima `props` (dan `$bindable` hanya untuk `bind:` yang sudah ada). Tidak boleh memindahkan `fetch` atau mengubah urutan efek. Setiap subkomponen mendapat file CSS sendiri dari bagian CSS induk (`styles/pages/<halaman>/<bagian>.css`, akar `pg-<halaman>-<bagian>`); aturan yang dipakai lintas bagian tetap di `pages/<halaman>/shared.css`. Satu subkomponen per commit.

| Fase | Halaman | Subkomponen (folder `routes/<halaman>/`) dan baris sumber saat audit |
|---|---|---|
| 8.1 | Security (3388) | `SecurityTabs` 690-744, `OverviewTab` 746-1076, `FirewallTab` 1078-1536, `SshTab` 1538-1766, `ServerSecurityTab` 1768-1841, `BruteForceTab` 1843-2085, `AntiIntrusionTab` 2087-2200, `ScanDetectionTab` 2202-2264, `AddPortModal` 2266-2341, `AddWhitelistModal` 2343-2381, `AddBlacklistModal` 2383-2430 |
| 8.2 | WAF (5585) | `OverviewTab` 1116-1479, `LiveAccessTab` 1481-1738, `SiteProtectionTab` 1740-1858, `AttackMapTab` 1860-2167, `ListsTab` 2169-2378, `CloudflareHub` 2380-2785, modal: `CfConnectModal` 2787-2865, `CfDnsModal` 2866-3111, `CfEdgeModal` 3112-3250, `SiteOptionsModal` 3251-3322, `ForensicsModal` 3323-3475 |
| 8.3 | Docker (3893) | satu subkomponen per tab: Overview 1067-1199, Container 1201-1418, Template 1420-1498, CloudImage 1500-1586, LocalImage 1588-1708, Compose 1710-1831, Network 1833-1928, Volume 1930-2025, Repository 2027-2119, Settings 2121-2469; modal 2471-3253 menjadi 8 file modal |
| 8.4 | Files (3913) | `DirTabs` 1032-1059, `PathBar` 1060-1112, `FileToolbar` 1113-1362, `FileTable` 1381-1488, `FileGrid` 1489-1524, `StatusBar` 1525-1556, `ContextMenu` 1557-1681, `CodeEditor` 2076-2328, modal kecil (remote download, symlink, search, favorites, cloud, properties, zip, new file, rename, chmod, upload) |
| 8.5 | Databases (3833) | `EngineTabs` 875-965, `ActionBar` 1002-1071, `DbTable` 1072-1173, modal-modal 1175-2226, `TasksDrawer` 2228-2310 |
| 8.6 | PHPManager (3021) | `VersionCards` 751-785, `SitesList` 786-871, tiap tab panel (Service 1008-1057 ... phpinfo 1793-1850) menjadi file sendiri, `InstallPanel` 1851-1937 |
| 8.7 | Sites (2792) | `SitesTable` 577-900, `LogsModal` 924-976, `GitPullModal` 977-1126, `DeleteModal` 1127-1262 |

Nomor baris adalah dari audit dan bergeser setelah fase sebelumnya; cari bagian berdasarkan komentar penanda atau struktur `{#if tab === ...}`.

Kriteria selesai tiap subkomponen: halaman induk berkurang ukurannya, perilaku identik, `V-CHECK` tanpa error baru (tipe `props` eksplisit dengan `$props()`), tanpa `<style>`.
Verifikasi: `V-LINT`, `V-CHECK`, `V-BUILD`, checklist 3.1 pada tab/modal yang dipindah.

### Fase 9: Pembersihan dan penutupan

Tier: standard.

| Item | Detail |
|---|---|
| Hapus alias sementara | Hapus alias di `tokens.css` (2.3). Cari ulang pemakaian nama lama, harus nol |
| Aturan lint jadi error | `UI002-UI005` dari peringatan menjadi error. `UI001` diperketat (deteksi `rgb(0 0 0 / .5)` bentuk spasi) |
| Cek bentrok | Skrip kecil `scripts/css-collision.mjs` yang memastikan tiap kelas akar `pg-*`/`cmp-*` unik dan tiap `@keyframes` unik. Tambahkan ke `npm run build` |
| CSS mati | Daftar kelas di `styles/pages/**` yang tidak muncul di `.svelte` mana pun (laporan saja, hapus yang jelas mati dan jangan kelas dari `class:` dinamis atau string template) |
| Build produksi | `npm run build` sungguhan (menulis `web/dist`), lalu `go build ./...` dari root untuk memastikan embed tetap valid |
| Audit akhir | `npm run check` 0 error/warning; `Select-String '<style'` kosong; QA visual 20 halaman x 3 lebar x 2 tema |
| Dokumentasi | Perbarui `ui-ux-standards.md` ke versi final |

Kriteria selesai: semua perintah di atas lolos, ukuran total CSS tercatat dibanding baseline (`index-*.css` 192 kB + chunk halaman).
Verifikasi: `npm run check; npm run lint:ui; npm run build; cd ..; go build ./...`

---

## 5. Ringkasan Edge Case dan Pitfall

| Risiko | Mitigasi |
|---|---|
| Scoped CSS Svelte menjadi global: aturan halaman bisa bocor ke komponen anak | Semua aturan bersarang di akar `pg-*`/`cmp-*`; anak yang punya kelas sama tetap diuji di checklist visual; `css-collision.mjs` di Fase 9 |
| Svelte membuang selektor tak terpakai, global tidak | CSS bisa membengkak sementara; dibersihkan di Fase 9 |
| CSS tanpa layer mengalahkan CSS ber-layer | Dilarang ada CSS tanpa layer; vendor dibungkus `layer(vendor)`. Selama transisi, blok `<style>` yang belum dimigrasi memang menang |
| Animasi `@keyframes` global | Hanya `fp-*` atau berprefiks akar |
| `display: contents` pada wrapper | Hanya jika halaman tak punya akar tunggal; `position: fixed` modal tidak terpengaruh |
| Light mode memunculkan warna mentah tersembunyi | Fase 5 dan 6 mengganti semua literal warna; cek tampilan light di checklist |
| Build lolos tapi UI rusak | `vite build` tidak mendeteksi regresi visual; wajib checklist 3.1 |
| `web/dist` di-embed ke binary Go | Verifikasi antar fase memakai `--outDir` temp; build produksi hanya di Fase 9 |
| Perubahan aksi tanpa konfirmasi (Home/Services) | Di luar cakupan; dicatat untuk keputusan terpisah |

---

## 6. Catatan Proses Audit

| Hal | Status |
|---|---|
| Pitroom (`pitroom doctor`) | Setup sehat, tetapi worker gagal saat dipakai: Codex (`gpt-5.6-luna`, read-only) menolak membuat proses (`CreateProcess ... blocked by policy`) sehingga tidak bisa membaca file, dan Gemini CLI gagal dengan `Unknown arguments: output-format, approval-mode, policy` (versi CLI tidak cocok dengan argumen pitroom). Sandbox tidak dilonggarkan. Saran: jalankan `pitroom install --mcp --no-skills` lalu periksa versi Gemini CLI/kebijakan sandbox Codex |
| Pengganti | Pemetaan codebase didelegasikan ke dua subagent read-only bawaan (audit duplikasi CSS dan audit layout/UX). Metrik ukuran file, jumlah `<style>`, build, `lint:ui`, dan `svelte-check` dijalankan langsung tanpa mengubah file |
| Batasan data | Angka dari subagent berasal dari regex/grep (±beberapa). Catatan per halaman bersifat grep-derived, bukan pembacaan penuh. Jumlah 178 masalah editor belum direproduksi persis (terukur 149 lewat `svelte-check`) |

