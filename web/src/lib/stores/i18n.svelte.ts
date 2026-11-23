// Store reaktif multi-bahasa (ID / EN) tanpa dependensi eksternal berbasis Svelte 5 runes.
import { dictID, type TranslationKey } from '../i18n/id';
import { dictEN } from '../i18n/en';

export type Locale = 'id' | 'en';

const LANG_STORAGE_KEY = 'fp:lang';
const DICTS: Record<Locale, Record<TranslationKey, string>> = {
  id: dictID,
  en: dictEN,
};

function readInitialLocale(): Locale {
  if (typeof window === 'undefined') return 'id';
  try {
    const saved = localStorage.getItem(LANG_STORAGE_KEY);
    if (saved === 'id' || saved === 'en') return saved;
  } catch {
    // Abaikan jika localStorage tidak tersedia
  }
  return 'id';
}

function applyHtmlLang(lang: Locale) {
  if (typeof document === 'undefined') return;
  document.documentElement.lang = lang;
}

export const i18nState = $state<{ lang: Locale }>({
  lang: readInitialLocale(),
});

applyHtmlLang(i18nState.lang);

export function setLocale(lang: Locale) {
  const next: Locale = lang === 'en' ? 'en' : 'id';
  i18nState.lang = next;
  applyHtmlLang(next);
  try {
    localStorage.setItem(LANG_STORAGE_KEY, next);
  } catch {
    // Abaikan kegagalan simpan localStorage
  }
}

export function toggleLocale(): Locale {
  const next: Locale = i18nState.lang === 'id' ? 'en' : 'id';
  setLocale(next);
  return next;
}

/** Menerjemahkan kunci i18n secara reaktif sesuai bahasa aktif dengan substitusi variabel {nama}. */
export function t(key: TranslationKey, vars?: Record<string, string | number>): string {
  const dict = DICTS[i18nState.lang] ?? dictID;
  let text = dict[key] ?? dictID[key] ?? key;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) {
      text = text.replaceAll(`{${k}}`, String(v));
    }
  }
  return text;
}

/** Kode locale BCP-47 untuk pemformatan tanggal/jam bawaan browser. */
export function dateLocale(): string {
  return i18nState.lang === 'en' ? 'en-US' : 'id-ID';
}
