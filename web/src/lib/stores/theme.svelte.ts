// Store tampilan, palet warna, kerapatan UI, sudut, & identitas branding FikPanel berbasis Svelte 5 runes.
// Disimpan di localStorage ('fp:appearance') untuk render anti-flicker dan disinkronkan dengan SQLite via API.

export type ThemeMode = 'dark' | 'light' | 'auto';
export type AccentPreset = 'lava' | 'ocean' | 'emerald' | 'violet' | 'crimson' | 'amber' | 'slate' | 'custom';
export type SurfaceStyle = 'obsidian' | 'midnight' | 'carbon';
export type DensityStyle = 'comfortable' | 'compact';
export type RadiusStyle = 'sharp' | 'default' | 'rounded';
export type EnvBadge = '' | 'production' | 'staging' | 'development';

export interface PanelAppearance {
  theme_mode: ThemeMode;
  accent_preset: AccentPreset;
  accent_custom: string;
  surface_style: SurfaceStyle;
  density: DensityStyle;
  radius_style: RadiusStyle;
  brand_name: string;
  env_badge: EnvBadge;
}

export const DEFAULT_APPEARANCE: PanelAppearance = {
  theme_mode: 'dark',
  accent_preset: 'lava',
  accent_custom: '#ff5a2b',
  surface_style: 'obsidian',
  density: 'comfortable',
  radius_style: 'default',
  brand_name: 'fikpanel',
  env_badge: '',
};

const STORAGE_KEY = 'fp:appearance';
const LEGACY_THEME_KEY = 'fp:theme';
const HEX_RE = /^#[0-9a-fA-F]{6}$/;

function readInitialAppearance(): PanelAppearance {
  if (typeof window === 'undefined') return { ...DEFAULT_APPEARANCE };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<PanelAppearance>;
      return {
        ...DEFAULT_APPEARANCE,
        ...parsed,
        accent_custom: HEX_RE.test(parsed.accent_custom || '') ? (parsed.accent_custom as string) : DEFAULT_APPEARANCE.accent_custom,
        brand_name: (parsed.brand_name || DEFAULT_APPEARANCE.brand_name).slice(0, 40),
      };
    }
    const legacy = localStorage.getItem(LEGACY_THEME_KEY);
    if (legacy === 'dark' || legacy === 'light' || legacy === 'auto') {
      return { ...DEFAULT_APPEARANCE, theme_mode: legacy };
    }
  } catch {
    // Abaikan bila localStorage dibatasi browser
  }
  return { ...DEFAULT_APPEARANCE };
}

// Hitung kontras teks (#ffffff atau #111111) terhadap warna aksen kustom sesuai luminance relatif WCAG
function computeOnAccentColor(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16) / 255;
  const g = parseInt(hex.slice(3, 5), 16) / 255;
  const b = parseInt(hex.slice(5, 7), 16) / 255;
  const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  const lum = 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
  return lum > 0.36 ? '#121110' : '#ffffff';
}

// Gelapkan warna hex sedikit untuk state hover tombol aksen kustom
function darkenHex(hex: string, factor = 0.85): string {
  const ch = (idx: number) => Math.max(0, Math.min(255, Math.round(parseInt(hex.slice(idx, idx + 2), 16) * factor)));
  const toHex = (n: number) => n.toString(16).padStart(2, '0');
  return `#${toHex(ch(1))}${toHex(ch(3))}${toHex(ch(5))}`;
}

function clearCustomAccentVars(root: HTMLElement) {
  root.style.removeProperty('--accent');
  root.style.removeProperty('--accent-strong');
  root.style.removeProperty('--accent-hover');
  root.style.removeProperty('--text-on-accent');
  root.style.removeProperty('--chart-1');
}

function applyAppearanceToDOM(a: PanelAppearance) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.setAttribute('data-theme', a.theme_mode);
  root.setAttribute('data-surface', a.surface_style);
  root.setAttribute('data-density', a.density);
  root.setAttribute('data-radius', a.radius_style);

  if (a.accent_preset === 'custom' && HEX_RE.test(a.accent_custom)) {
    root.setAttribute('data-accent', 'custom');
    root.style.setProperty('--accent', a.accent_custom);
    root.style.setProperty('--accent-strong', a.accent_custom);
    root.style.setProperty('--accent-hover', darkenHex(a.accent_custom));
    root.style.setProperty('--text-on-accent', computeOnAccentColor(a.accent_custom));
    root.style.setProperty('--chart-1', a.accent_custom);
  } else {
    clearCustomAccentVars(root);
    if (a.accent_preset && a.accent_preset !== 'lava') {
      root.setAttribute('data-accent', a.accent_preset);
    } else {
      root.removeAttribute('data-accent');
    }
  }
}

const initial = readInitialAppearance();

export const appearanceState = $state<PanelAppearance>({ ...initial });

// Kompatibilitas mundur dengan komponen yang membaca themeState.mode
export const themeState = {
  get mode(): ThemeMode {
    return appearanceState.theme_mode;
  },
  set mode(val: ThemeMode) {
    setTheme(val);
  },
};

applyAppearanceToDOM(appearanceState);

function persistAppearance() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...appearanceState }));
    localStorage.setItem(LEGACY_THEME_KEY, appearanceState.theme_mode);
  } catch {
    // Abaikan kegagalan simpan localStorage
  }
}

export function applyAppearance(patch: Partial<PanelAppearance>) {
  Object.assign(appearanceState, patch);
  if (!HEX_RE.test(appearanceState.accent_custom)) {
    appearanceState.accent_custom = DEFAULT_APPEARANCE.accent_custom;
  }
  if (!appearanceState.brand_name?.trim()) {
    appearanceState.brand_name = DEFAULT_APPEARANCE.brand_name;
  }
  applyAppearanceToDOM(appearanceState);
  persistAppearance();
}

export function resetAppearance() {
  applyAppearance({ ...DEFAULT_APPEARANCE });
}

export function setTheme(mode: ThemeMode) {
  applyAppearance({ theme_mode: mode });
}

export function cycleTheme() {
  const order: ThemeMode[] = ['dark', 'light', 'auto'];
  const next = order[(order.indexOf(appearanceState.theme_mode) + 1) % order.length];
  setTheme(next);
}
