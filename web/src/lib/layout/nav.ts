// Definisi navigasi panel — satu sumber untuk Sidebar, judul dokumen, dan router.
// Standar: halaman baru WAJIB didaftarkan di sini (id = segmen hash #/id).

export interface NavItem {
  id: string;
  label: string;
  icon: string;
  group: 'utama' | 'sistem';
}

export const NAV_ITEMS: NavItem[] = [
  { id: 'home', label: 'Dashboard', icon: 'ri-dashboard-3-line', group: 'utama' },
  { id: 'website', label: 'Website', icon: 'ri-global-line', group: 'utama' },
  { id: 'databases', label: 'Databases', icon: 'ri-database-2-line', group: 'utama' },
  { id: 'files', label: 'Files', icon: 'ri-folder-3-line', group: 'utama' },
  { id: 'services', label: 'Services', icon: 'ri-server-line', group: 'utama' },
  { id: 'terminal', label: 'Terminal', icon: 'ri-terminal-box-line', group: 'utama' },
  { id: 'monitor', label: 'Monitor', icon: 'ri-line-chart-line', group: 'sistem' },
  { id: 'security', label: 'Security', icon: 'ri-shield-keyhole-line', group: 'sistem' },
  { id: 'assistant', label: 'AI Assistant', icon: 'ri-sparkling-line', group: 'sistem' },
  { id: 'settings', label: 'Settings', icon: 'ri-settings-4-line', group: 'sistem' },
];

/** Ambil id rute dari hash URL; rute tak dikenal jatuh ke dashboard. */
export function routeFromHash(hash: string): string {
  const id = hash.replace(/^#\/?/, '').split(/[?/]/)[0];
  return NAV_ITEMS.some((n) => n.id === id) ? id : 'home';
}
