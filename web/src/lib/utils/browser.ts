// Utilitas keamanan & interaksi browser yang dipakai lintas halaman.

/**
 * Salin teks ke clipboard. Clipboard API hanya tersedia di secure context (HTTPS/localhost);
 * panel yang diakses via http://IP memakai fallback textarea tersembunyi.
 */
export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try {
    if (!document.execCommand('copy')) throw new Error('Browser menolak akses clipboard');
  } finally {
    document.body.removeChild(ta);
  }
}

/**
 * Validasi URL eksternal sebelum dipakai di href/window.open.
 * Hanya http/https yang diizinkan — memblokir javascript:, data:, vbscript:.
 */
export function safeExternalUrl(raw: string): string | null {
  try {
    const u = new URL(raw, window.location.origin);
    return u.protocol === 'http:' || u.protocol === 'https:' ? u.href : null;
  } catch {
    return null;
  }
}

/** Buka URL eksternal di tab baru tanpa memberi akses window.opener ke halaman tujuan. */
export function openExternal(raw: string): boolean {
  const url = safeExternalUrl(raw);
  if (!url) return false;
  window.open(url, '_blank', 'noopener,noreferrer');
  return true;
}

/** Baca nilai design token CSS (untuk kanvas chart yang tidak bisa memakai var()). */
export function readToken(name: string, fallback = ''): string {
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}
