// Store notifikasi global (Svelte 5 runes, modul .svelte.ts).
// Standar UX:
// - success: konfirmasi aksi selesai (hilang 3 detik)
// - info: status netral (3 detik)
// - warning: perlu perhatian (5 detik)
// - error: kegagalan — tampil 6 detik, bisa ditutup manual, dibacakan pembaca layar segera
// Jangan pakai toast untuk error validasi form; tampilkan di bawah field (.field__error).

export type ToastType = 'success' | 'info' | 'warning' | 'error';

export interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
}

const DURATION: Record<ToastType, number> = { success: 3000, info: 3000, warning: 5000, error: 6000 };
const MAX_VISIBLE = 4;
// Heuristik untuk pemanggil lama onNotify(msg) yang tidak menyebut tipe
const ERROR_HINT = /\b(gagal|error|tidak valid|ditolak|tidak dapat|timeout|kedaluwarsa)\b/i;

let nextId = 1;
const timers = new Map<number, ReturnType<typeof setTimeout>>();

export const toasts = $state<{ items: ToastItem[] }>({ items: [] });

export function dismissToast(id: number) {
  clearTimeout(timers.get(id));
  timers.delete(id);
  toasts.items = toasts.items.filter((t) => t.id !== id);
}

export function notify(message: string, type?: ToastType): number {
  const resolved: ToastType = type ?? (ERROR_HINT.test(message) ? 'error' : 'success');
  // Pesan identik yang masih tampil tidak diduplikasi, cukup perpanjang waktunya
  const dup = toasts.items.find((t) => t.message === message && t.type === resolved);
  const id = dup ? dup.id : nextId++;
  if (!dup) {
    toasts.items = [...toasts.items, { id, type: resolved, message }].slice(-MAX_VISIBLE);
  }
  clearTimeout(timers.get(id));
  timers.set(id, setTimeout(() => dismissToast(id), DURATION[resolved]));
  return id;
}

export const toast = {
  success: (m: string) => notify(m, 'success'),
  info: (m: string) => notify(m, 'info'),
  warning: (m: string) => notify(m, 'warning'),
  error: (m: string) => notify(m, 'error'),
};
