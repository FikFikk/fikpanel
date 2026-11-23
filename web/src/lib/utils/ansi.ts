// Parser ANSI SGR bebas XSS (tanpa {@html}) untuk merender warna output terminal di mode Console.

export interface AnsiSegment {
  text: string;
  className: string;
}

export interface HighlightChunk {
  text: string;
  match: boolean;
}

const FG_MAP: Record<number, string> = {
  30: 'ansi-fg-muted',
  31: 'ansi-fg-red',
  32: 'ansi-fg-green',
  33: 'ansi-fg-yellow',
  34: 'ansi-fg-blue',
  35: 'ansi-fg-magenta',
  36: 'ansi-fg-cyan',
  37: 'ansi-fg-bright',
  90: 'ansi-fg-muted',
  91: 'ansi-fg-red',
  92: 'ansi-fg-green',
  93: 'ansi-fg-yellow',
  94: 'ansi-fg-blue',
  95: 'ansi-fg-magenta',
  96: 'ansi-fg-cyan',
  97: 'ansi-fg-bright',
};

const BG_MAP: Record<number, string> = {
  41: 'ansi-bg-red',
  42: 'ansi-bg-green',
  43: 'ansi-bg-yellow',
  44: 'ansi-bg-blue',
  45: 'ansi-bg-magenta',
  46: 'ansi-bg-cyan',
  101: 'ansi-bg-red',
  102: 'ansi-bg-green',
  103: 'ansi-bg-yellow',
  104: 'ansi-bg-blue',
  105: 'ansi-bg-magenta',
  106: 'ansi-bg-cyan',
};

// Hapus urutan kontrol OSC dan CSI non-warna (mis. judul window, kursor)
const NON_SGR_RE = /\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[A-LN-Za-ln-z]/g;
const ALL_ANSI_RE = /\x1b(?:\][^\x07\x1b]*(?:\x07|\x1b\\)|\[[0-9;?]*[A-Za-z])/g;
const SGR_RE = /\x1b\[([0-9;]*)m/g;

/** Menghapus seluruh kode escape ANSI sehingga aman untuk pencarian, clipboard, dan export log. */
export function stripAnsi(raw: string): string {
  if (!raw) return '';
  return raw.replace(ALL_ANSI_RE, '');
}

/** Memecah teks output yang mengandung kode warna ANSI menjadi daftar segmen dengan kelas CSS aman. */
export function parseAnsiSegments(raw: string): AnsiSegment[] {
  if (!raw) return [];
  const cleaned = raw.replace(NON_SGR_RE, '');
  const segments: AnsiSegment[] = [];

  let fg = '';
  let bg = '';
  let bold = false;
  let dim = false;
  let italic = false;
  let underline = false;

  const buildClass = () => {
    const cls: string[] = [];
    if (fg) cls.push(fg);
    if (bg) cls.push(bg);
    if (bold) cls.push('ansi-bold');
    if (dim) cls.push('ansi-dim');
    if (italic) cls.push('ansi-italic');
    if (underline) cls.push('ansi-underline');
    return cls.join(' ');
  };

  let lastIdx = 0;
  SGR_RE.lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = SGR_RE.exec(cleaned)) !== null) {
    if (match.index > lastIdx) {
      segments.push({
        text: cleaned.slice(lastIdx, match.index),
        className: buildClass(),
      });
    }
    const paramStr = match[1];
    const codes = paramStr === '' ? [0] : paramStr.split(';').map((n) => parseInt(n, 10) || 0);
    for (let i = 0; i < codes.length; i++) {
      const code = codes[i];
      if (code === 0) {
        fg = '';
        bg = '';
        bold = false;
        dim = false;
        italic = false;
        underline = false;
      } else if (code === 1) {
        bold = true;
      } else if (code === 2) {
        dim = true;
      } else if (code === 3) {
        italic = true;
      } else if (code === 4) {
        underline = true;
      } else if (code === 22) {
        bold = false;
        dim = false;
      } else if (code === 23) {
        italic = false;
      } else if (code === 24) {
        underline = false;
      } else if (code === 39) {
        fg = '';
      } else if (code === 49) {
        bg = '';
      } else if (FG_MAP[code]) {
        fg = FG_MAP[code];
      } else if (BG_MAP[code]) {
        bg = BG_MAP[code];
      } else if (code === 38 && codes[i + 1] === 5 && codes[i + 2] !== undefined) {
        // Petakan 256-color dasar (0-15) ke warna semantik panel
        const c256 = codes[i + 2];
        if (c256 < 16) {
          fg = FG_MAP[30 + (c256 % 8)] || '';
        }
        i += 2;
      } else if (code === 38 && codes[i + 1] === 2) {
        i += 4;
      }
    }
    lastIdx = SGR_RE.lastIndex;
  }

  if (lastIdx < cleaned.length) {
    segments.push({
      text: cleaned.slice(lastIdx),
      className: buildClass(),
    });
  }

  return segments;
}

/** Membagi teks biasa menjadi potongan cocok/tidak cocok untuk highlight pencarian tanpa {@html}. */
export function splitHighlightMatches(text: string, query: string): HighlightChunk[] {
  const q = query.trim();
  if (!q || !text) return [{ text, match: false }];

  const lowerText = text.toLowerCase();
  const lowerQuery = q.toLowerCase();
  const chunks: HighlightChunk[] = [];
  let cursor = 0;

  while (cursor < text.length) {
    const idx = lowerText.indexOf(lowerQuery, cursor);
    if (idx === -1) {
      chunks.push({ text: text.slice(cursor), match: false });
      break;
    }
    if (idx > cursor) {
      chunks.push({ text: text.slice(cursor, idx), match: false });
    }
    chunks.push({ text: text.slice(idx, idx + q.length), match: true });
    cursor = idx + q.length;
  }

  return chunks;
}
