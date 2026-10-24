// Helper format tampilan. Standar: semua angka/tanggal yang tampil ke user lewat fungsi di sini
// agar format konsisten (locale id-ID) di seluruh halaman.

const LOCALE = 'id-ID';

/** Format bytes ke satuan yang mudah dibaca manusia (B, KB, MB, GB, TB). */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1);
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

/** Format timestamp Unix (detik) ke tanggal + jam lokal; '—' bila kosong. */
export function formatDateTime(unixSeconds: number): string {
  if (!unixSeconds) return '—';
  return new Date(unixSeconds * 1000).toLocaleString(LOCALE, { dateStyle: 'medium', timeStyle: 'short' });
}

/** Format timestamp Unix (detik) ke tanggal saja. */
export function formatDate(unixSeconds: number): string {
  if (!unixSeconds) return '—';
  return new Date(unixSeconds * 1000).toLocaleDateString(LOCALE, { dateStyle: 'medium' });
}

/** Sisa hari hingga timestamp Unix (detik), minimal 0. */
export function daysUntil(unixSeconds: number): number {
  return Math.max(0, Math.floor((unixSeconds * 1000 - Date.now()) / 86_400_000));
}

const rtf = new Intl.RelativeTimeFormat(LOCALE, { numeric: 'auto' });

/** Waktu relatif ("5 menit yang lalu") dari timestamp Unix (detik). */
export function formatRelative(unixSeconds: number): string {
  if (!unixSeconds) return '—';
  const diff = unixSeconds - Date.now() / 1000;
  const abs = Math.abs(diff);
  if (abs < 60) return rtf.format(Math.round(diff), 'second');
  if (abs < 3600) return rtf.format(Math.round(diff / 60), 'minute');
  if (abs < 86_400) return rtf.format(Math.round(diff / 3600), 'hour');
  return rtf.format(Math.round(diff / 86_400), 'day');
}

/** Format angka dengan pemisah ribuan lokal. */
export function formatNumber(n: number): string {
  return new Intl.NumberFormat(LOCALE).format(n);
}
