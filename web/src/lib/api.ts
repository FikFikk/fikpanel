// Klien REST API panel: proteksi CSRF otomatis, timeout, dan penanganan sesi kedaluwarsa terpusat.
// Standar: SEMUA request ke backend wajib lewat apiFetch — jangan memanggil fetch() langsung.
import { formatBytes } from './utils/format';

export { formatBytes };

/** Event global yang dipancarkan saat server membalas 401 (sesi habis). App.svelte mendengarkannya. */
export const UNAUTHORIZED_EVENT = 'fikpanel:unauthorized';

const DEFAULT_TIMEOUT_MS = 30_000;

// Simpan ID build server pertama yang diterima agar tab browser otomatis me-reload saat biner server diperbarui
let activeServerBuildId = '';

/** Error terstruktur agar UI bisa membedakan status HTTP (mis. 409 konflik vs 400 validasi). */
export class ApiError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export interface ApiOptions extends RequestInit {
  /** Batas waktu request dalam ms (default 30 detik, 0 = tanpa batas). */
  timeoutMs?: number;
}

export async function apiFetch<T = any>(endpoint: string, options: ApiOptions = {}): Promise<T> {
  // Hanya izinkan path relatif same-origin agar cookie sesi tidak pernah terkirim ke host lain
  if (!endpoint.startsWith('/')) {
    throw new ApiError('Endpoint API harus berupa path relatif', 0);
  }

  const { timeoutMs = DEFAULT_TIMEOUT_MS, signal: externalSignal, ...init } = options;
  const headers = new Headers(init.headers || {});
  const method = (init.method || 'GET').toUpperCase();

  // Wajibkan header anti-CSRF untuk request mutasi (POST, PUT, DELETE, PATCH)
  if (method !== 'GET' && method !== 'HEAD') {
    headers.set('X-Panel-Request', '1');
    if (!headers.has('Content-Type') && !(init.body instanceof FormData)) {
      headers.set('Content-Type', 'application/json');
    }
  }
  headers.set('Accept', 'application/json');

  // Gabungkan sinyal pembatalan dari pemanggil dengan timeout internal
  const ctl = new AbortController();
  const onExternalAbort = () => ctl.abort(externalSignal?.reason);
  if (externalSignal) {
    if (externalSignal.aborted) ctl.abort(externalSignal.reason);
    else externalSignal.addEventListener('abort', onExternalAbort, { once: true });
  }
  const timer = timeoutMs > 0 ? setTimeout(() => ctl.abort(new DOMException('Timeout', 'TimeoutError')), timeoutMs) : undefined;

  let res: Response;
  try {
    res = await fetch(endpoint, {
      ...init,
      method,
      headers,
      signal: ctl.signal,
      credentials: 'same-origin',
      cache: 'no-store',
    });
  } catch (err) {
    const reason = ctl.signal.reason as DOMException | undefined;
    if (reason?.name === 'TimeoutError') {
      throw new ApiError('Server tidak merespons (timeout), coba lagi', 0);
    }
    if ((err as Error).name === 'AbortError') throw err;
    throw new ApiError('Tidak dapat terhubung ke server panel', 0);
  } finally {
    clearTimeout(timer);
    externalSignal?.removeEventListener('abort', onExternalAbort);
  }

  // Deteksi pembaruan versi build server secara otomatis pada request GET agar UI selalu sinkron
  const buildHdr = res.headers.get('X-FikPanel-Build') || '';
  if (buildHdr) {
    if (!activeServerBuildId) {
      activeServerBuildId = buildHdr;
    } else if (activeServerBuildId !== buildHdr && method === 'GET' && !endpoint.includes('deploy-progress')) {
      activeServerBuildId = buildHdr;
      window.location.reload();
    }
  }

  if (!res.ok) {
    let errMsg = `Permintaan gagal (HTTP ${res.status})`;
    try {
      const data = await res.json();
      if (data && typeof data.error === 'string') errMsg = data.error;
    } catch {
      // Respons bukan JSON, gunakan pesan default
    }
    // Sesi habis: beri tahu App agar kembali ke layar login (kecuali saat proses login itu sendiri)
    if (res.status === 401 && !endpoint.startsWith('/api/auth/login')) {
      window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT, { detail: errMsg }));
    }
    throw new ApiError(errMsg, res.status);
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}

/** Mengambil data berbasis teks mentah (HTML/string) dengan proteksi timeout & penanganan error terpusat. */
export async function apiFetchText(endpoint: string, options: ApiOptions = {}): Promise<string> {
  if (!endpoint.startsWith('/')) {
    throw new ApiError('Endpoint API harus berupa path relatif', 0);
  }

  const { timeoutMs = DEFAULT_TIMEOUT_MS, signal: externalSignal, ...init } = options;
  const headers = new Headers(init.headers || {});
  headers.set('Accept', 'text/html, text/plain, */*');

  const ctl = new AbortController();
  const onExternalAbort = () => ctl.abort(externalSignal?.reason);
  if (externalSignal) {
    if (externalSignal.aborted) ctl.abort(externalSignal.reason);
    else externalSignal.addEventListener('abort', onExternalAbort, { once: true });
  }
  const timer = timeoutMs > 0 ? setTimeout(() => ctl.abort(new DOMException('Timeout', 'TimeoutError')), timeoutMs) : undefined;

  let res: Response;
  try {
    res = await fetch(endpoint, {
      ...init,
      headers,
      signal: ctl.signal,
      credentials: 'same-origin',
      cache: 'no-store',
    });
  } catch (err) {
    const reason = ctl.signal.reason as DOMException | undefined;
    if (reason?.name === 'TimeoutError') {
      throw new ApiError('Server tidak merespons (timeout), coba lagi', 0);
    }
    if ((err as Error).name === 'AbortError') throw err;
    throw new ApiError('Tidak dapat terhubung ke server panel', 0);
  } finally {
    clearTimeout(timer);
    externalSignal?.removeEventListener('abort', onExternalAbort);
  }

  // Deteksi pembaruan versi build server secara otomatis pada request GET agar UI selalu sinkron
  const buildHdr = res.headers.get('X-FikPanel-Build') || '';
  if (buildHdr) {
    if (!activeServerBuildId) {
      activeServerBuildId = buildHdr;
    } else if (activeServerBuildId !== buildHdr && method === 'GET' && !endpoint.includes('deploy-progress')) {
      activeServerBuildId = buildHdr;
      window.location.reload();
    }
  }

  if (!res.ok) {
    let errMsg = `Permintaan gagal (HTTP ${res.status})`;
    if (res.status === 401 && !endpoint.startsWith('/api/auth/login')) {
      window.dispatchEvent(new CustomEvent(UNAUTHORIZED_EVENT, { detail: errMsg }));
    }
    throw new ApiError(errMsg, res.status);
  }

  return res.text();
}