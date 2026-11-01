// Store tema aplikasi (dark / light / auto) berbasis Svelte 5 runes.
// Pilihan disimpan di localStorage ('fp:theme') dan disinkronkan ke atribut data-theme pada <html>.

export type ThemeMode = 'dark' | 'light' | 'auto';

const STORAGE_KEY = 'fp:theme';

function readInitialTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark';
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark' || saved === 'auto') {
      return saved;
    }
  } catch {
    // Abaikan jika akses localStorage dibatasi browser
  }
  return 'dark';
}

function applyThemeAttribute(mode: ThemeMode) {
  if (typeof document === 'undefined') return;
  document.documentElement.setAttribute('data-theme', mode);
}

export const themeState = $state<{ mode: ThemeMode }>({
  mode: readInitialTheme(),
});

// Sinkronkan atribut awal saat modul dimuat
applyThemeAttribute(themeState.mode);

export function setTheme(mode: ThemeMode) {
  themeState.mode = mode;
  applyThemeAttribute(mode);
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Abaikan jika penyimpanan penuh atau diblokir
  }
}

export function cycleTheme() {
  const order: ThemeMode[] = ['dark', 'light', 'auto'];
  const next = order[(order.indexOf(themeState.mode) + 1) % order.length];
  setTheme(next);
}
