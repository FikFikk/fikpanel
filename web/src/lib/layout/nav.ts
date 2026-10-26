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
  { id: 'waf', label: 'WAF', icon: 'ri-shield-flash-line', group: 'utama' },
  { id: 'appstore', label: 'App Store', icon: 'ri-apps-2-line', group: 'utama' },
  { id: 'php', label: 'PHP Manager', icon: 'ri-code-s-slash-line', group: 'utama' },
  { id: 'databases', label: 'Databases', icon: 'ri-database-2-line', group: 'utama' },
  { id: 'docker', label: 'Docker', icon: 'ri-box-3-line', group: 'utama' },
  { id: 'k8s', label: 'Kubernetes', icon: 'ri-bubble-chart-line', group: 'utama' },
  { id: 'node', label: 'Node / PM2', icon: 'ri-nodejs-line', group: 'utama' },
  { id: 'files', label: 'Files', icon: 'ri-folder-3-line', group: 'utama' },
  { id: 'services', label: 'Services', icon: 'ri-server-line', group: 'utama' },
  { id: 'cron', label: 'Cron Jobs', icon: 'ri-time-line', group: 'utama' },
  { id: 'terminal', label: 'Terminal', icon: 'ri-terminal-box-line', group: 'utama' },
  { id: 'monitor', label: 'Monitor', icon: 'ri-line-chart-line', group: 'sistem' },
  { id: 'system', label: 'Task & Cleaner', icon: 'ri-speed-up-line', group: 'sistem' },
  { id: 'security', label: 'Security', icon: 'ri-shield-check-line', group: 'sistem' },
  { id: 'assistant', label: 'AI Assistant', icon: 'ri-sparkling-line', group: 'sistem' },
  { id: 'settings', label: 'Settings', icon: 'ri-settings-4-line', group: 'sistem' },
];

/** Ambil id rute dari hash URL; rute tak dikenal jatuh ke dashboard. */
export function routeFromHash(hash: string): string {
  const id = hash.replace(/^#\/?/, '').split(/[?/]/)[0];
  if (id === 'backup') return 'databases';
  return NAV_ITEMS.some((n) => n.id === id) ? id : 'home';
}
