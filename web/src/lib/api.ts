// Helper pemanggilan REST API panel dengan proteksi CSRF otomatis
export async function apiFetch<T = any>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {});
  
  // Wajibkan header anti-CSRF untuk request mutasi (POST, PUT, DELETE, PATCH)
  if (options.method && options.method !== 'GET') {
    headers.set('X-Panel-Request', '1');
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
  }

  const res = await fetch(endpoint, {
    ...options,
    headers,
  });

  if (!res.ok) {
    let errMsg = `HTTP Error ${res.status}`;
    try {
      const data = await res.json();
      if (data && data.error) {
        errMsg = data.error;
      }
    } catch {
      // Abaikan jika bukan JSON
    }
    throw new Error(errMsg);
  }

  return res.json();
}

// Format bytes ke satuan yang mudah dibaca manusia (B, KB, MB, GB, TB)
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}