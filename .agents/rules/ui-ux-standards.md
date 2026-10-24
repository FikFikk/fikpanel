# Standar Desain UI/UX & Keamanan Frontend FikPanel

Dokumen ini adalah acuan resmi arsitektur UI/UX dan keamanan frontend FikPanel. Setiap AI assistant atau developer yang membuat, memodifikasi, atau merefaktor komponen antarmuka di repositori ini **WAJIB** membaca dan mematuhi aturan berikut.

---

## 1. Arsitektur File & Pemisahan Tanggung Jawab

| Lokasi | Tanggung Jawab | Aturan |
|---|---|---|
| `web/src/styles/tokens.css` | Sumber tunggal variabel desain (colors, typography, spacing, radius, z-index, motion) | Dilarang keras menulis token di luar file ini |
| `web/src/styles/base.css` | Reset CSS global, accessibility focus ring, prefers-reduced-motion, animasi standar | Berlaku global untuk aplikasi |
| `web/src/styles/components.css` | Primitive reusable UI (.btn, .input, .card, .table, .badge, .switch, .skeleton, .empty-state, .spinner) | Seluruh tombol dan input wajib memakai class ini |
| `web/src/styles/utilities.css` | Layout helpers (.stack, .row, .grow, .mono, .sr-only, .truncate) | Utility class tanpa side-effect |
| `web/src/lib/api.ts` | Klien REST terpusat (CSRF header, timeout, error normalization, unauthorized event) | Jangan pernah memanggil `fetch()` mentah |
| `web/src/lib/stores/` | State reaktif Svelte 5 runes (`toast.svelte.ts`, `confirm.svelte.ts`) | UI hooks & feedback terpusat |
| `web/src/lib/utils/` | Helper fungsi pure (`format.ts`, `browser.ts`) | Format byte, tanggal, sanitasi URL eksternal |
| `web/src/lib/layout/` | Shell aplikasi (`Sidebar.svelte`, `Topbar.svelte`, `Login.svelte`) | Komponen navigasi dan kerangka utama |
| `web/src/lib/components/ui/` | Host modal & dialog (`ToastHost.svelte`, `ConfirmHost.svelte`) | Dipasang sekali di `App.svelte` |
| `web/src/routes/` | Tampilan rute halaman (`Sites.svelte`, `Files.svelte`, dll.) | Gaya lokal di `<style>` hanya untuk layout halaman, wajib memakai `var(--token)` |

---

## 2. Aturan Token & Styling

1. **Dilarang Warna Mentah:**
   - Dilarang menulis `#hex`, `rgb(...)`, `hsl(...)` di file `.svelte` atau file `.css` apa pun (kecuali di `tokens.css`).
   - Gunakan `var(--accent)`, `var(--text)`, `var(--bg-surface)`, `var(--border)`, `var(--danger)`, `var(--warning)`, `var(--info)`, dsb.
   - Untuk transparansi / tinted background: gunakan `var(--tint-accent)` atau `color-mix(in srgb, var(--accent) X%, transparent)`.
2. **Skala Spasi & Radius:**
   - Spasi hanya dari `--space-1` (4px) s/d `--space-10` (40px).
   - Radius hanya dari `--radius-sm` (6px), `--radius-md` (8px), `--radius-lg` (10px), `--radius-xl` (14px), atau `--radius-full` (999px).
3. **Tipografi & Font:**
   - Font sans: `'Plus Jakarta Sans Variable'` (self-hosted).
   - Font mono: `'Fira Code'` (self-hosted).
   - Ukuran font: `--text-2xs` (11px) hingga `--text-2xl` (26px).
4. **Ikon:**
   - Gunakan ikon self-hosted RemixIcon (`ri-*`). Tidak ada ketergantungan CDN eksternal.

---

## 3. Standar UX & Pola Interaksi

1. **Aksi Destruktif Wajib Konfirmasi:**
   - Dilarang memanggil `window.confirm()`, `window.alert()`, atau `window.prompt()`.
   - Gunakan `confirmDialog({ title, message, confirmText, cancelText, tone: 'danger' })` dari `../stores/confirm.svelte`.
   - Gunakan `promptDialog({ title, label, value, inputType })` untuk dialog input terisolasi.
   - Tombol konfirmasi harus jelas menyebutkan aksi (mis. *"Hapus Situs"*, bukan sekadar *"Ya"*).
2. **Notifikasi & Feedback (Toast):**
   - Gunakan `notify(msg)` atau `toast.success()`, `toast.error()`, `toast.warning()`, `toast.info()`.
   - Jangan biarkan proses async selesai tanpa feedback visual yang jelas bagi user.
3. **Indikator Loading:**
   - Tombol async wajib di-disable dan menampilkan spinner `.spin` atau teks status selama proses berlangsung.
   - Konten yang memuat data awal wajib menampilkan skeleton `.skeleton` atau status memuat informatif.
4. **State Kosong (Empty State):**
   - Saat tabel atau daftar tidak memiliki data, gunakan pola `.empty-state` yang menyertakan ikon, judul, deskripsi, dan tombol aksi (CTA) untuk membuat data pertama.
5. **Form & Validasi:**
   - Label form berada di atas kontrol `.field__label`.
   - Pesan error field tampil di bawah input dengan class `.field__error` dan ubah `aria-invalid="true"`.

---

## 4. Keamanan Frontend (Security Standard)

1. **Anti-CSRF:**
   - Seluruh mutasi HTTP (`POST`, `PUT`, `DELETE`, `PATCH`) wajib menyertakan header `X-Panel-Request: 1`. Helper `apiFetch` menyematkannya secara otomatis.
2. **Penanganan XSS:**
   - Dilarang memakai `{@html ...}` tanpa sanitasi bersertifikat.
   - Dilarang manipulasi DOM mentah seperti `element.innerHTML = ...` atau `eval()`.
3. **Tautan Eksternal:**
   - Setiap tag `<a target="_blank">` wajib menyertakan atribut `rel="noopener noreferrer"`.
   - Untuk URL dinamis dari database, lewatkan ke helper `safeExternalUrl()` di `lib/utils/browser.ts`.
4. **Sesi & Token:**
   - Sesi menggunakan cookie `HttpOnly` dengan flag `SameSite=Strict`. Dilarang menyimpan credential atau token sensitif di `localStorage`.
5. **Content-Security-Policy (CSP):**
   - Backend Go menyematkan CSP ketat, `Permissions-Policy`, `X-Frame-Options: DENY`, `Cross-Origin-Opener-Policy: same-origin`, dan `Cross-Origin-Resource-Policy: same-origin`.

---

## 5. Aksesibilitas (A11y) & Responsif

1. **Target Sentuh & Kontrol:**
   - Tombol ikon saja (`.btn--icon`) wajib memiliki atribut `aria-label` atau `title`.
   - Fokus keyboard terlihat jelas dengan outline `--ring-accent` (dikontrol di `base.css`).
2. **Breakpoint Responsif:**
   - Desktop: `> 1024px` (Sidebar penuh `220px`).
   - Tablet: `768px - 1024px` (Sidebar icon-only `60px`).
   - Mobile: `< 768px` (Sidebar drawer flyout dengan backdrop scrim).
3. **Motion:**
   - Mendukung media query `@media (prefers-reduced-motion: reduce)` di mana animasi transisi diringkas agar nyaman bagi pengguna sensitif.

---

## 6. Otomatisasi Linter UI & Build

Sebelum setiap commit atau build produksi, linter UI otomatis dieksekusi:

```bash
npm --prefix web run lint:ui
npm --prefix web run build
```

Kode linter berada di `web/scripts/lint-ui.mjs` dan akan melempar exit code 1 jika mendeteksi pelanggaran UI001, UX001, SEC001, SEC002, atau SEC003.
