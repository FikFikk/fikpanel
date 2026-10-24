// Linter standar UI/UX & keamanan frontend fikpanel.
// Dijalankan via `npm run lint:ui` dan otomatis sebelum `npm run build`.
// Aturan sesuai .agents/rules/ui-ux-standards.md:
//   UI001  warna mentah (#hex / rgb() / hsl()) di <style> atau atribut style → pakai var(--token)
//   SEC001 {@html …} dilarang (risiko XSS)
//   SEC002 target="_blank" wajib rel="noopener noreferrer"
//   SEC003 fetch() langsung dilarang → pakai apiFetch (CSRF, timeout, sesi)
//   SEC004 eval / new Function / innerHTML / outerHTML / document.write dilarang
//   UX001  window.confirm/alert/prompt dilarang → pakai confirmDialog / notify
//   A11Y001 tombol ikon tanpa teks wajib aria-label atau title
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const srcDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../src');
// File yang boleh memuat nilai mentah (sumber token & klien API itu sendiri)
const ALLOW = {
  UI001: [/styles[\\/]tokens\.css$/],
  SEC003: [/lib[\\/]api\.ts$/],
};

const files = [];
(function walk(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p);
    else if (/\.(svelte|ts|css)$/.test(p)) files.push(p);
  }
})(srcDir);

const problems = [];
const lineOf = (text, idx) => text.slice(0, idx).split('\n').length;
const allowed = (rule, file) => (ALLOW[rule] || []).some((re) => re.test(file));

function report(rule, file, text, idx, msg) {
  if (allowed(rule, file)) return;
  problems.push(`${path.relative(srcDir, file)}:${lineOf(text, idx)}  ${rule}  ${msg}`);
}

const RAW_COLOR = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(/g;

for (const file of files) {
  const text = fs.readFileSync(file, 'utf8');
  const isSvelte = file.endsWith('.svelte');
  const isCss = file.endsWith('.css');

  // UI001 — warna mentah di CSS
  const cssBlocks = [];
  if (isCss) cssBlocks.push({ start: 0, css: text });
  if (isSvelte) {
    for (const m of text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
      cssBlocks.push({ start: m.index + m[0].indexOf(m[1]), css: m[1] });
    }
    for (const m of text.matchAll(/\sstyle="([^"]*)"/g)) {
      cssBlocks.push({ start: m.index, css: m[1] });
    }
  }
  for (const { start, css } of cssBlocks) {
    // Abaikan komentar CSS agar dokumentasi boleh menyebut contoh nilai
    const stripped = css.replace(/\/\*[\s\S]*?\*\//g, (c) => ' '.repeat(c.length));
    for (const m of stripped.matchAll(RAW_COLOR)) {
      // rgb(0 0 0 / x) untuk bayangan & overlay hitam diizinkan
      const tail = stripped.slice(m.index, m.index + 24);
      if (/^rgba?\(\s*0[\s,]+0[\s,]+0\b/.test(tail)) continue;
      report('UI001', file, text, start + m.index, `warna mentah "${m[0]}" → gunakan var(--token) dari styles/tokens.css`);
    }
  }

  if (isSvelte || file.endsWith('.ts')) {
    const blank = (s) => s.replace(/[^\n]/g, ' ');
    // Kosongkan blok style, komentar, dan string literal kosong-panjang agar posisi baris tetap akurat
    const code = text
      .replace(/<style[^>]*>[\s\S]*?<\/style>/g, blank)
      .replace(/<!--[\s\S]*?-->/g, blank)
      .replace(/\/\*[\s\S]*?\*\//g, blank)
      .replace(/(^|[^:'"`\\])\/\/[^\n]*/g, (m, p) => p + blank(m.slice(p.length)));
    const rules = [
      ['SEC001', /\{@html\s/g, '{@html} dilarang — render teks biasa agar otomatis di-escape'],
      ['SEC003', /(?<![\w.])fetch\(/g, 'fetch() langsung dilarang — gunakan apiFetch dari lib/api'],
      ['SEC004', /\beval\(|new Function\(|\.innerHTML\s*=|\.outerHTML\s*=|document\.write\(/g, 'API rawan XSS dilarang'],
      ['UX001', /(?<![\w.])(?:window\.)?(?:confirm|alert|prompt)\(/g, 'dialog native dilarang — gunakan confirmDialog() / notify()'],
    ];
    for (const [rule, re, msg] of rules) {
      for (const m of code.matchAll(re)) report(rule, file, text, m.index, msg);
    }
  }

  if (isSvelte) {
    for (const m of text.matchAll(/<a\b[^>]*target="_blank"[^>]*>/g)) {
      if (!/rel="[^"]*noopener/.test(m[0])) report('SEC002', file, text, m.index, 'target="_blank" tanpa rel="noopener noreferrer"');
    }
    // Tombol yang isinya hanya ikon wajib punya label aksesibel
    for (const m of text.matchAll(/<button\b([^>]*)>\s*(<i [^>]*><\/i>)\s*<\/button>/g)) {
      if (!/aria-label=|title=/.test(m[1])) report('A11Y001', file, text, m.index, 'tombol ikon tanpa aria-label/title');
    }
  }
}

if (problems.length) {
  const byRule = problems.reduce((acc, p) => {
    const r = p.split(/\s+/)[1];
    acc[r] = (acc[r] || 0) + 1;
    return acc;
  }, {});
  console.error(problems.join('\n'));
  console.error(`\n✖ ${problems.length} pelanggaran standar UI/keamanan:`, byRule);
  process.exit(1);
}
console.log(`✓ lint:ui lolos (${files.length} file diperiksa)`);
