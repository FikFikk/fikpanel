// Utilitas klien WebAuthn / Passkey (FIDO2) tanpa dependensi eksternal berbasis Web Crypto browser
import { apiFetch } from '../api';

export interface PasskeyItem {
  id: number;
  user_id: number;
  name: string;
  credential_id: string;
  transports: string;
  created_at: number;
  last_used_at: number;
}

// Konversi Base64URL string ke ArrayBuffer untuk API navigator.credentials
function b64urlToBuf(str: string): ArrayBuffer {
  const padded = str.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (str.length % 4)) % 4);
  const bin = atob(padded);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) {
    bytes[i] = bin.charCodeAt(i);
  }
  return bytes.buffer;
}

// Konversi ArrayBuffer ke Base64URL string tanpa padding
function bufToB64url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let bin = '';
  for (let i = 0; i < bytes.length; i++) {
    bin += String.fromCharCode(bytes[i]);
  }
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

// Periksa apakah hostname berupa alamat IP mentah (IPv4 atau IPv6)
export function isRawIPHost(hostname = window.location.hostname): boolean {
  const h = hostname.trim().replace(/^\[|\]$/g, '');
  if (!h || h === 'localhost') return false;
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(h)) return true;
  if (h.includes(':')) return true;
  return false;
}

// Periksa dukungan browser dan kelayakan origin HTTPS/localhost untuk WebAuthn/Passkey
export function getWebAuthnEnvironment(): {
  supported: boolean;
  validOrigin: boolean;
  reason: 'ok' | 'unsupported' | 'insecure' | 'raw_ip';
} {
  if (typeof window === 'undefined' || typeof window.PublicKeyCredential === 'undefined' || !navigator.credentials) {
    return { supported: false, validOrigin: false, reason: 'unsupported' };
  }
  if (!window.isSecureContext) {
    return { supported: true, validOrigin: false, reason: 'insecure' };
  }
  if (isRawIPHost(window.location.hostname)) {
    return { supported: true, validOrigin: false, reason: 'raw_ip' };
  }
  return { supported: true, validOrigin: true, reason: 'ok' };
}

// Usulkan nama perangkat default dari user-agent browser saat mendaftar Passkey
export function suggestPasskeyDeviceName(): string {
  const ua = navigator.userAgent || '';
  let os = 'Perangkat';
  if (ua.includes('Windows')) os = 'Windows';
  else if (ua.includes('Mac OS X') && !ua.includes('iPhone')) os = 'macOS';
  else if (ua.includes('Android')) os = 'Android';
  else if (ua.includes('iPhone') || ua.includes('iPad')) os = 'iPhone / iPad';
  else if (ua.includes('Linux')) os = 'Linux';

  let br = 'Browser';
  if (ua.includes('Edg/')) br = 'Edge';
  else if (ua.includes('Chrome/')) br = 'Chrome';
  else if (ua.includes('Firefox/')) br = 'Firefox';
  else if (ua.includes('Safari/')) br = 'Safari';
  return `${os} · ${br}`;
}

function mapWebAuthnError(err: unknown): Error {
  if (err instanceof DOMException) {
    if (err.name === 'NotAllowedError') {
      return new Error('Verifikasi Passkey dibatalkan atau waktu habis');
    }
    if (err.name === 'InvalidStateError') {
      return new Error('Perangkat ini sudah terdaftar sebagai Passkey pada akun ini');
    }
    if (err.name === 'SecurityError') {
      return new Error('Origin domain tidak valid untuk Passkey (wajib domain HTTPS atau localhost)');
    }
  }
  if (err instanceof Error) return err;
  return new Error('Operasi Passkey gagal');
}

// Alur pendaftaran Passkey baru (mendukung Windows Hello PIN, Touch ID, QR HP, maupun USB Security Key)
export async function registerPasskeyFlow(deviceName: string): Promise<{ ok: boolean; passkey: PasskeyItem; message: string }> {
  const begin = await apiFetch<{ ok: boolean; session_id: string; publicKey: any }>('/api/auth/passkey/register/begin', {
    method: 'POST',
  });

  const pk = begin.publicKey;
  const creationOptions: PublicKeyCredentialCreationOptions = {
    ...pk,
    challenge: b64urlToBuf(pk.challenge),
    user: {
      ...pk.user,
      id: b64urlToBuf(pk.user.id),
    },
    excludeCredentials: (pk.excludeCredentials || []).map((c: any) => ({
      ...c,
      id: b64urlToBuf(c.id),
    })),
  };

  let cred: PublicKeyCredential | null = null;
  try {
    cred = (await navigator.credentials.create({ publicKey: creationOptions })) as PublicKeyCredential | null;
  } catch (err) {
    throw mapWebAuthnError(err);
  }
  if (!cred) {
    throw new Error('Tidak ada kredensial Passkey yang dikembalikan browser');
  }

  const resp = cred.response as AuthenticatorAttestationResponse;
  const transports = typeof resp.getTransports === 'function' ? resp.getTransports() : [];

  const serialized = {
    id: cred.id,
    rawId: bufToB64url(cred.rawId),
    type: cred.type,
    authenticatorAttachment: (cred as any).authenticatorAttachment || undefined,
    clientExtensionResults: cred.getClientExtensionResults?.() || {},
    response: {
      attestationObject: bufToB64url(resp.attestationObject),
      clientDataJSON: bufToB64url(resp.clientDataJSON),
      transports,
    },
  };

  return apiFetch<{ ok: boolean; passkey: PasskeyItem; message: string }>('/api/auth/passkey/register/finish', {
    method: 'POST',
    body: JSON.stringify({
      session_id: begin.session_id,
      name: deviceName.trim() || suggestPasskeyDeviceName(),
      credential: serialized,
    }),
  });
}

// Alur login tanpa kata sandi dengan Passkey (discoverable: PIN Windows Hello / QR HP / Biometrik)
export async function loginWithPasskeyFlow(): Promise<{ ok: boolean; user?: { username: string } }> {
  const begin = await apiFetch<{ ok: boolean; session_id: string; publicKey: any }>('/api/auth/passkey/login/begin', {
    method: 'POST',
  });

  const pk = begin.publicKey;
  const requestOptions: PublicKeyCredentialRequestOptions = {
    ...pk,
    challenge: b64urlToBuf(pk.challenge),
    allowCredentials: (pk.allowCredentials || []).map((c: any) => ({
      ...c,
      id: b64urlToBuf(c.id),
    })),
  };

  let cred: PublicKeyCredential | null = null;
  try {
    cred = (await navigator.credentials.get({ publicKey: requestOptions })) as PublicKeyCredential | null;
  } catch (err) {
    throw mapWebAuthnError(err);
  }
  if (!cred) {
    throw new Error('Login Passkey dibatalkan');
  }

  const resp = cred.response as AuthenticatorAssertionResponse;
  const serialized = {
    id: cred.id,
    rawId: bufToB64url(cred.rawId),
    type: cred.type,
    authenticatorAttachment: (cred as any).authenticatorAttachment || undefined,
    clientExtensionResults: cred.getClientExtensionResults?.() || {},
    response: {
      authenticatorData: bufToB64url(resp.authenticatorData),
      clientDataJSON: bufToB64url(resp.clientDataJSON),
      signature: bufToB64url(resp.signature),
      userHandle: resp.userHandle ? bufToB64url(resp.userHandle) : undefined,
    },
  };

  return apiFetch<{ ok: boolean; user?: { username: string } }>('/api/auth/passkey/login/finish', {
    method: 'POST',
    body: JSON.stringify({
      session_id: begin.session_id,
      credential: serialized,
    }),
  });
}
