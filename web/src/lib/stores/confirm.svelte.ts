// Dialog konfirmasi berbasis Promise pengganti window.confirm().
// Standar UX: SETIAP aksi destruktif/irreversibel (hapus, stop service, reset SSL, timpa file)
// wajib memakai confirmDialog dengan tone 'danger' dan teks tombol yang menyebut aksinya.
//   if (!(await confirmDialog({ title: 'Hapus situs?', message: '…', confirmText: 'Hapus situs', tone: 'danger' }))) return;
// Untuk aksi sangat berbahaya, isi requireText agar user mengetik ulang nama target.

export interface ConfirmOptions {
  title: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  tone?: 'default' | 'danger';
  /** Bila diisi, tombol konfirmasi aktif hanya setelah user mengetik teks ini persis. */
  requireText?: string;
}

export interface PromptOptions extends Omit<ConfirmOptions, 'requireText'> {
  /** Label field input. */
  label: string;
  /** Nilai awal field. */
  value?: string;
  inputType?: 'text' | 'number';
  min?: number;
  max?: number;
  placeholder?: string;
}

interface PromptField {
  label: string;
  value: string;
  inputType: 'text' | 'number';
  min?: number;
  max?: number;
  placeholder?: string;
}

interface ConfirmState extends ConfirmOptions {
  open: boolean;
  /** Terisi hanya saat mode prompt. */
  prompt: PromptField | null;
}

export const confirmState = $state<ConfirmState>({ open: false, title: '', prompt: null });

let resolver: ((ok: boolean) => void) | null = null;

function openDialog(opts: ConfirmOptions, prompt: PromptField | null): Promise<boolean> {
  // Dialog sebelumnya yang belum dijawab dianggap batal
  resolver?.(false);
  Object.assign(confirmState, {
    message: '',
    confirmText: 'Lanjutkan',
    cancelText: 'Batal',
    tone: 'default',
    requireText: '',
    ...opts,
    prompt,
    open: true,
  });
  return new Promise((resolve) => {
    resolver = resolve;
  });
}

export function confirmDialog(opts: ConfirmOptions): Promise<boolean> {
  return openDialog(opts, null);
}

/** Pengganti window.prompt(): resolve string hasil input, atau null bila dibatalkan. */
export async function promptDialog(opts: PromptOptions): Promise<string | null> {
  const { label, value = '', inputType = 'text', min, max, placeholder, ...rest } = opts;
  const field: PromptField = { label, value, inputType, min, max, placeholder };
  const ok = await openDialog(rest, field);
  // Bandingkan dengan field milik pemanggil ini: dialog lain bisa saja sudah dibuka menggantikannya
  const own = confirmState.prompt === field || (ok && confirmState.prompt !== null);
  const current = own ? confirmState.prompt : null;
  const result = ok && current ? String(current.value ?? '').trim() : null;
  if (own) confirmState.prompt = null;
  return result;
}

export function resolveConfirm(ok: boolean) {
  confirmState.open = false;
  resolver?.(ok);
  resolver = null;
}
