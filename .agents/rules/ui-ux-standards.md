# Standar Desain UI/UX & Keamanan Frontend FikPanel (Versi Final)

Dokumen ini adalah acuan resmi arsitektur UI/UX, sistem desain layer, dan keamanan frontend FikPanel. Setiap AI assistant atau developer yang membuat, memodifikasi, atau merefaktor komponen antarmuka di repositori ini **WAJIB** membaca dan mematuhi aturan berikut.

---

## 1. Arsitektur File & Cascading Layer

Semua stylesheet aplikasi dikelola melalui CSS Cascade Layers (`@layer`) di `web/src/styles/index.css` dengan urutan:
`@layer vendor, tokens, base, layout, components, utilities, pages;`

| Layer / Lokasi | Tanggung Jawab | Aturan |
|---|---|---|
| `@layer vendor` | Font mandiri (`@fontsource`), ikon (`remixicon`), lib CSS (`leaflet`, `uplot`) | Font dan aset wajib self-hosted, dilarang CDN eksternal |
| `@layer tokens` (`styles/tokens.css`) | Sumber tunggal variabel desain (colors, typography, spacing, radius, z-index, motion, themes) | Dilarang keras menulis literal warna di luar file ini. Semua tema (`light` dan `dark`) didefinisikan di sini |
| `@layer base` (`styles/base.css`) | Reset CSS global, focus ring aksesibilitas, `@keyframes fp-*`, scrollbar, typography dasar | Berlaku global, animasi wajib berprefiks `fp-*` |
| `@layer layout` (`styles/layout.css`) | Shell aplikasi (`Sidebar.svelte`, `Topbar.svelte`, `CommandPalette.svelte`, `Login.svelte`) | Struktur kerangka utama admin panel |
| `@layer components` (`styles/components/`) | Primitif UI bersama (`btn`, `input`, `field`, `card`, `table`, `badge`, `modal`, `tabs`, `stat-card`, `page-header`, `toast`, `alert`, `progress`, `state`) | Seluruh komponen tombol, form, dan modal wajib menggunakan kelas standar BEM ini |
| `@layer utilities` (`styles/utilities.css`) | Layout helpers minimal (`.stack`, `.row`, `.row-between`, `.grow`, `.wrap`, `.mono`, `.tabular`, `.truncate`, `.break-all`, `.text-muted`) | Utility murni tanpa side-effect warna mentah |
| `@layer pages` (`styles/pages/`) | Modul rute halaman (`pg-<halaman>`) dan subkomponennya | Setiap modul halaman memiliki folder terpisah jika dipecah (`styles/pages/<modul>/<bagian>.css`). Wajib dibungkus `@layer pages` |

### Aturan Blok `<style>` Svelte
> [!IMPORTANT]
> **DILARANG KERAS** menggunakan blok `<style>` di dalam file `.svelte` mana pun (aturan linter `UI002`).
> Seluruh styling harus diekstrak ke file CSS eksternal ber-layer (`styles/pages/**` atau `styles/components/**`).

---

## 2. Arsitektur Penamaan & Subkomponen (BEM & Root Namespacing)

1. **Kelas Akar Unik Modul Halaman:**
   - Setiap rute halaman memiliki satu kelas akar berprefiks `pg-` (misalnya `.pg-sites`, `.pg-files`, `.pg-waf`, `.pg-docker`, `.pg-databases`, `.pg-phpmanager`).
   - Setiap subkomponen spesifik halaman menggunakan prefiks akar modul induk diikuti nama bagian, misalnya:
     - `.pg-sites-table`
     - `.pg-waf-attack-map`
     - `.pg-docker-container`
     - `.pg-databases-action-bar`
     - `.pg-files-file-table`
2. **Aturan Bersarang (Nesting):**
   - Seluruh aturan CSS halaman wajib berada di dalam selektor kelas akarnya untuk mencegah kebocoran gaya ke modul lain:
     ```css
     @layer pages {
       .pg-sites-table {
         /* aturan khusus subkomponen */
         .site-chip { ... }
       }
     }
     ```
3. **Pemeriksaan Bentrokan (Collision Checker):**
   - Setiap kelas akar `pg-*` / `cmp-*` dan setiap `@keyframes` harus unik di seluruh proyek, diverifikasi otomatis melalui `scripts/css-collision.mjs`.

---

## 3. Pemetaan Standar Komponen (Peta Rename Global)

| Kelas Usang / Lama | Kelas Baru Standar | Catatan |
|---|---|---|
| `.btn-primary`, `.btn-secondary`, `.btn-danger`, `.btn-ghost` | `.btn .btn--primary`, `.btn--secondary`, `.btn--danger`, `.btn--ghost` | Primitif BEM button |
| `.btn-icon`, `.btn-mini`, `.sub-btn` | `.btn .btn--icon` (tambah `.btn--sm` bila kecil) | Wajib menyertakan `aria-label` atau `title` |
| `.modal-backdrop`, `.modal-card`, `.modal-wrapper` | Komponen `Modal.svelte` (`.modal`, `.modal__panel`) | Menggunakan komponen terpusat |
| `.modal-head`, `.modal-header` | `.modal__head` | Header modal |
| `.modal-title`, `.modal-sub` | `.modal__title`, `.modal__sub` | Judul & subjudul modal |
| `.modal-body` | `.modal__body` | Isi dialog |
| `.modal-foot`, `.modal-footer`, `.modal-actions` | `.modal__foot` | Aksi bawah modal |
| `.form-group` | `.field` | Bungkus field formulir |
| `.form-label` | `.field__label` | Label input |
| `.form-input`, `.search-input` | `.input` (tambah `.input--search` bila berpencarian) | Input kontrol standar |
| `.tab-btn`, `.sub-tab`, `.seg-btn` | `.tabs` / `.tabs__tab` / `.is-active` | Tab bar navigasi |
| `.stat-card`, `.stat-label`, `.stat-value`, `.stat-icon` | `.stat-card`, `.stat-card__label`, `.stat-card__value`, `.stat-card__icon` | Ringkasan metrik |
| `.page-title`, `.page-sub`, `.head-bar` | Komponen `PageHeader.svelte` (`.page-header`, `.page-header__title`, `.page-header__sub`) | Header halaman standar |
| `.table-wrap`, `.table-container` | `.table-wrap` + `.table` | Kontainer tabel responsif |
| `.panel`, `.panel-card`, `.panel-head` | `.card`, `.card__head`, `.card__title` | Kartu konten terstruktur |
| `.badge-tag`, `.chip`, `.status-pill` | `.badge` + `.badge--success`, `.badge--warning`, `.badge--danger`, `.badge--info`, `.badge--neutral` | Lencana status |

---

## 4. Aturan Token Desain & Tema (Light & Dark)

1. **Warna Token Resmi:**
   - Dilarang menulis literal warna `#hex`, `rgb(...)`, `hsl(...)` di luar `tokens.css` (aturan `UI001`).
   - Warna teks: `var(--text)`, `var(--text-muted)`, `var(--text-subtle)`, `var(--text-inverse)`.
   - Latar belakang: `var(--bg-app)`, `var(--bg-surface)`, `var(--bg-raised)`, `var(--bg-inset)`, `var(--bg-overlay)`.
   - Garis batas: `var(--border)`, `var(--border-strong)`, `var(--border-subtle)`.
   - Aksen & Status: `var(--accent)`, `var(--success)`, `var(--warning)`, `var(--danger)`, `var(--info)`, `var(--violet)`.
   - Tint & Alpha: `var(--tint-accent)`, `var(--tint-success)`, `var(--tint-warning)`, `var(--tint-danger)`, `var(--tint-info)`.
2. **Dukungan Dua Tema (Dark / Light):**
   - Mode default adalah dark, didukung toggle instan tanpa flicker melalui atribut `[data-theme="light"]` di elemen `<html>`.
   - Hindari hardcoded kontras asumsi dark mode; selalu gunakan token agar otomatis beradaptasi saat tema light aktif.

---

## 5. Standar UX & Pola Interaksi

1. **Aksi Destruktif Wajib Konfirmasi:**
   - Dilarang memanggil `window.confirm()`, `window.alert()`, atau `window.prompt()`.
   - Gunakan `confirmDialog({ title, message, confirmText, cancelText, tone: 'danger' })` dari `../lib/stores/confirm.svelte`.
   - Tombol konfirmasi harus secara eksplisit menyebutkan aksi yang akan dijalankan (mis. *"Hapus Basis Data"*, bukan *"OK"*).
2. **Feedback Visual Terpusat (Toast):**
   - Gunakan `notify(msg)` atau `toast.success()`, `toast.error()`, `toast.warning()`, `toast.info()`.
   - Semua operasi asinkron mutasi data wajib memberikan umpan balik status yang jelas.
3. **Indikator Proses:**
   - Tombol asinkron wajib dinonaktifkan (`disabled`) dan menampilkan indikator loading selama request berjalan.
   - Tabel dan modul yang memuat data menampilkan state skeleton `.skeleton` atau status memuat informatif.
4. **State Kosong (Empty State):**
   - Saat koleksi data bernilai kosong, gunakan pola komponen `.empty-state` yang menyertakan ikon, pesan jelas, dan CTA untuk membuat entitas baru.

---

## 6. Keamanan Frontend

1. **Anti-CSRF Terotomatisasi:**
   - Seluruh mutasi HTTP (`POST`, `PUT`, `DELETE`, `PATCH`) wajib menyertakan header `X-Panel-Request: 1`. Helper `apiFetch` dan `apiFetchJson` menyematkannya otomatis.
2. **Perlindungan Terhadap XSS:**
   - Dilarang keras menggunakan `{@html ...}` tanpa sanitasi eksplisit yang aman.
   - Dilarang manipulasi DOM mentah (`innerHTML`, `eval`, `document.write`).
3. **Tautan Eksternal:**
   - Setiap tag `<a target="_blank">` wajib menyertakan `rel="noopener noreferrer"`.
   - Gunakan pembungkus `safeExternalUrl()` untuk URL yang berasal dari masukan pengguna atau database.
4. **Proteksi Sesi:**
   - Autentikasi sesi disimpan dalam cookie `HttpOnly` dengan kebijakan `SameSite=Strict`. Dilarang menyimpan access token atau kredensial sensitif di `localStorage`.

---

## 7. Pipeline Validasi & Verifikasi Otomatis

Sebelum setiap commit atau rilis produksi, 4 verifikasi wajib lolos tanpa kompromi:

1. **Type & Component Check:**
   ```bash
   npm --prefix web run check
   ```
   Wajib menghasilkan **0 errors** dan **0 warnings**.
2. **UI Rules Lint:**
   ```bash
   npm --prefix web run lint:ui
   ```
   Memvalidasi `UI001` (warna mentah), `UI002` (larangan `<style>`), `UI003` (`@layer pages`), `UI004` (inline style), dan `UI005` (kelas usang).
3. **CSS Collision Check:**
   ```bash
   npm --prefix web run check:css
   ```
   Memastikan seluruh kelas akar `pg-*`/`cmp-*` dan `@keyframes` unik.
4. **Go Binary Embed Build:**
   ```bash
   cd web && npm run build && cd .. && go build ./...
   ```
   Memastikan bundle produksi berhasil dikompilasi dan valid di-embed ke dalam binary Go server.
