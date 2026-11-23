// Utilitas riwayat perintah persisten (localStorage), parser template variabel {{var:default}}, dan export log .txt.

const HISTORY_STORAGE_KEY = 'fikpanel_terminal_history_v1';
const MAX_HISTORY_PER_SCOPE = 200;

export interface CommandTemplateVar {
  name: string;
  defaultValue: string;
}

const TEMPLATE_VAR_RE = /\{\{\s*([a-zA-Z0-9_.-]+)(?::([^}]*))?\s*\}\}/g;

/** Membaca daftar perintah persisten dari localStorage untuk scope server tertentu (hanya teks perintah, bukan kredensial). */
export function loadTerminalHistory(scopeKey: string): string[] {
  try {
    const raw = window.localStorage.getItem(HISTORY_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as Record<string, string[]>;
    const list = parsed?.[scopeKey];
    return Array.isArray(list) ? list.filter((s) => typeof s === 'string' && s.trim() !== '') : [];
  } catch {
    return [];
  }
}

/** Menyimpan perintah terbaru ke riwayat persisten tanpa duplikasi berurutan dan membatasi ukuran daftar. */
export function recordTerminalHistory(scopeKey: string, command: string): string[] {
  const clean = command.trim();
  if (!clean) return loadTerminalHistory(scopeKey);
  try {
    const raw = window.localStorage.getItem(HISTORY_STORAGE_KEY);
    const parsed: Record<string, string[]> = raw ? JSON.parse(raw) : {};
    const prev = Array.isArray(parsed[scopeKey]) ? parsed[scopeKey] : [];
    // Pindahkan perintah yang sama ke posisi paling baru agar mudah dicari lewat Ctrl+R
    const deduped = prev.filter((item) => item !== clean);
    deduped.push(clean);
    const trimmed = deduped.slice(-MAX_HISTORY_PER_SCOPE);
    parsed[scopeKey] = trimmed;
    window.localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(parsed));
    return trimmed;
  } catch {
    return [clean];
  }
}

/** Menghapus seluruh riwayat tersimpan untuk scope tertentu. */
export function clearTerminalHistoryStore(scopeKey: string): void {
  try {
    const raw = window.localStorage.getItem(HISTORY_STORAGE_KEY);
    if (!raw) return;
    const parsed = JSON.parse(raw) as Record<string, string[]>;
    delete parsed[scopeKey];
    window.localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(parsed));
  } catch {
    // Abaikan kegagalan akses storage
  }
}

/** Mengekstrak seluruh placeholder variabel {{nama}} atau {{nama:default}} dari konten shortcut command. */
export function extractCommandVariables(content: string): CommandTemplateVar[] {
  const seen = new Map<string, CommandTemplateVar>();
  TEMPLATE_VAR_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = TEMPLATE_VAR_RE.exec(content)) !== null) {
    const name = m[1];
    const defaultValue = m[2] !== undefined ? m[2] : '';
    if (!seen.has(name)) {
      seen.set(name, { name, defaultValue });
    }
  }
  return Array.from(seen.values());
}

/** Memeriksa cepat apakah shortcut command memiliki placeholder variabel dinamis. */
export function hasCommandVariables(content: string): boolean {
  TEMPLATE_VAR_RE.lastIndex = 0;
  return TEMPLATE_VAR_RE.test(content);
}

/** Mengisi seluruh placeholder {{nama}} / {{nama:default}} dengan nilai yang dimasukkan pengguna. */
export function interpolateCommand(content: string, values: Record<string, string>): string {
  TEMPLATE_VAR_RE.lastIndex = 0;
  return content.replace(TEMPLATE_VAR_RE, (_full, name: string, fallback?: string) => {
    const v = values[name];
    if (v !== undefined && v !== '') return v;
    return fallback ?? '';
  });
}

/** Mengunduh konten teks sebagai berkas .txt secara aman di browser. */
export function downloadTextFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
