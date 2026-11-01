// Definisi navigasi panel — satu sumber untuk Sidebar, Command Palette, judul dokumen, dan router.
// Standar: halaman baru WAJIB didaftarkan di sini (id = segmen hash #/id).

export type NavGroupId = 'ringkasan' | 'website' | 'data' | 'infrastruktur' | 'keamanan' | 'sistem';

export interface NavGroup {
  id: NavGroupId;
  label: string;
}

export interface NavItem {
  id: string;
  label: string;
  icon: string;
  group: NavGroupId;
  badge?: string;
}

export const NAV_GROUPS: readonly NavGroup[] = [
  { id: 'ringkasan', label: 'Ringkasan' },
  { id: 'website', label: 'Website' },
  { id: 'data', label: 'Data' },
  { id: 'infrastruktur', label: 'Infrastruktur' },
  { id: 'keamanan', label: 'Keamanan' },
  { id: 'sistem', label: 'Sistem' },
];

export const NAV_ITEMS: NavItem[] = [
  // Ringkasan
  { id: 'home', label: 'Dashboard', icon: 'ri-dashboard-3-line', group: 'ringkasan' },
  // Website
  { id: 'website', label: 'Website', icon: 'ri-global-line', group: 'website' },
  { id: 'php', label: 'PHP Manager', icon: 'ri-code-s-slash-line', group: 'website' },
  { id: 'appstore', label: 'App Store', icon: 'ri-apps-2-line', group: 'website' },
  // Data
  { id: 'databases', label: 'Databases', icon: 'ri-database-2-line', group: 'data' },
  { id: 'files', label: 'Files', icon: 'ri-folder-3-line', group: 'data' },
  // Infrastruktur
  { id: 'docker', label: 'Docker', icon: 'ri-box-3-line', group: 'infrastruktur' },
  { id: 'k8s', label: 'Kubernetes', icon: 'ri-bubble-chart-line', group: 'infrastruktur' },
  { id: 'node', label: 'Node / PM2', icon: 'ri-nodejs-line', group: 'infrastruktur' },
  { id: 'services', label: 'Services', icon: 'ri-server-line', group: 'infrastruktur' },
  { id: 'cron', label: 'Cron Jobs', icon: 'ri-time-line', group: 'infrastruktur' },
  { id: 'terminal', label: 'Terminal', icon: 'ri-terminal-box-line', group: 'infrastruktur' },
  // Keamanan
  { id: 'waf', label: 'WAF', icon: 'ri-shield-flash-line', group: 'keamanan' },
  { id: 'security', label: 'Security', icon: 'ri-shield-check-line', group: 'keamanan' },
  // Sistem
  { id: 'monitor', label: 'Monitor', icon: 'ri-line-chart-line', group: 'sistem' },
  { id: 'system', label: 'Task & Cleaner', icon: 'ri-speed-up-line', group: 'sistem' },
  { id: 'assistant', label: 'AI Assistant', icon: 'ri-sparkling-line', group: 'sistem' },
  { id: 'settings', label: 'Settings', icon: 'ri-settings-4-line', group: 'sistem' },
];

/** Ambil id rute dari hash URL; rute tak dikenal jatuh ke dashboard. */
export function routeFromHash(hash: string): string {
  const id = hash.replace(/^#\/?/, '').split(/[?/]/)[0];
  if (id === 'backup') return 'databases';
  return NAV_ITEMS.some((n) => n.id === id) ? id : 'home';
}
