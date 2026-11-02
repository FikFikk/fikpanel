// Pemeriksa keunikan kelas akar pg-*/cmp-* dan @keyframes global.
// Dijalankan otomatis sebelum build produksi (npm run build).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const stylesDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../src/styles');

const roots = new Map();
const keyframes = new Map();

function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(fullPath);
    } else if (entry.name.endsWith('.css')) {
      checkFile(fullPath);
    }
  }
}

function checkFile(filePath) {
  const relPath = path.relative(stylesDir, filePath);
  const content = fs.readFileSync(filePath, 'utf8');
  // Hapus komentar CSS agar tidak mencocokkan contoh di komentar
  const clean = content.replace(/\/\*[\s\S]*?\*\//g, '');

  // Cari kelas akar pg-* dan cmp-*
  const rootMatches = clean.matchAll(/\.(pg-[a-z0-9-]+|cmp-[a-z0-9-]+)\b/g);
  for (const m of rootMatches) {
    const cls = m[1];
    if (!roots.has(cls)) roots.set(cls, []);
    const files = roots.get(cls);
    if (!files.includes(relPath)) files.push(relPath);
  }

  // Cari @keyframes <name>
  const kfMatches = clean.matchAll(/@keyframes\s+([a-zA-Z0-9_-]+)/g);
  for (const m of kfMatches) {
    const name = m[1];
    if (!keyframes.has(name)) keyframes.set(name, []);
    const files = keyframes.get(name);
    if (!files.includes(relPath)) files.push(relPath);
  }
}

walk(stylesDir);

const collisions = [];

for (const [cls, files] of roots.entries()) {
  if (files.length > 1) {
    collisions.push(`Kelas akar .${cls} duplikat di: ${files.join(', ')}`);
  }
}

for (const [name, files] of keyframes.entries()) {
  if (files.length > 1) {
    collisions.push(`@keyframes ${name} duplikat di: ${files.join(', ')}`);
  }
}

if (collisions.length > 0) {
  console.error(`\n✖ Terdeteksi ${collisions.length} bentrokan (collision) CSS:`);
  for (const c of collisions) {
    console.error(`  - ${c}`);
  }
  process.exit(1);
}

console.log(`✓ css-collision lolos (${roots.size} kelas akar unik, ${keyframes.size} @keyframes unik)`);
