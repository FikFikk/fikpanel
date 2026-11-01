<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { PHPVersionInfo, PHPVersionDetail, PHPExtensionInfo } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import VersionCards from './phpmanager/VersionCards.svelte';
  import ServiceTab from './phpmanager/ServiceTab.svelte';
  import '../styles/pages/phpmanager/shared.css';
  import '../styles/pages/phpmanager.css';

  // State daftar versi PHP
  let versions = $state<PHPVersionInfo[]>([]);
  let loading = $state(true);
  let installingVer = $state<string | null>(null);

  // State versi yang sedang dikelola di modal/workspace
  let activeVersion = $state<string | null>(null);
  let activeDetail = $state<PHPVersionDetail | null>(null);
  let loadingDetail = $state(false);
  let showManageModal = $state(false);

  // Sub-tab aktif di modal manajemen (aaPanel style)
  type ManageTab = 'service' | 'sites' | 'config' | 'raw_ini' | 'fpm_pool' | 'raw_fpm' | 'disabled_func' | 'extensions' | 'opcache' | 'logs' | 'phpinfo';
  let activeTab = $state<ManageTab>('config');
  let switchingSiteId = $state<number | null>(null);

  // State Form Visual Parameter (Screenshot 1 aaPanel)
  let formMaxExec = $state('300');
  let formMaxInput = $state('300');
  let formMemoryLimit = $state('256M');
  let formUploadMax = $state('64M');
  let formPostMax = $state('64M');
  let formDisplayErrors = $state('Off');
  let formErrorReporting = $state('E_ALL & ~E_DEPRECATED & ~E_STRICT');
  let formTimezone = $state('UTC');
  let savingConfig = $state(false);

  // State Editor php.ini Langsung (Screenshot 4 & 5 aaPanel)
  let rawIniContent = $state('');
  let rawIniOriginal = $state('');
  let rawIniPath = $state('');
  let loadingRawIni = $state(false);
  let savingRawIni = $state(false);
  let rawIniSearch = $state('');
  let rawIniMatchCount = $state(0);
  let rawIniCurrentMatchIndex = $state(0);
  let rawIniTextarea = $state<HTMLTextAreaElement | null>(null);
  let rawIniLineNumbers = $state<number[]>([]);
  let rawIniScrollTop = $state(0);

  // State Editor Pool FPM www.conf
  let rawFpmContent = $state('');
  let rawFpmOriginal = $state('');
  let rawFpmPath = $state('');
  let loadingRawFpm = $state(false);
  let savingRawFpm = $state(false);

  // State Form Pool FPM visual
  let fpmPm = $state('dynamic');
  let fpmMaxChildren = $state('50');
  let fpmStartServers = $state('5');
  let fpmMinSpare = $state('5');
  let fpmMaxSpare = $state('35');
  let fpmMaxRequests = $state('500');
  let savingFpmPool = $state(false);

  // State Disabled Functions
  let disabledFuncs = $state<string[]>([]);
  let newFuncInput = $state('');
  let savingDisabledFuncs = $state(false);

  // State Extensions
  let extSearch = $state('');
  let extList = $state<PHPExtensionInfo[]>([]);
  let actionExtName = $state<string | null>(null);
  let sshModalExt = $state<PHPExtensionInfo | null>(null);
  let showSshModal = $state(false);

  // State OPcache
  let opcacheEnable = $state('1');
  let opcacheMem = $state('128');
  let opcacheMaxFiles = $state('10000');
  let savingOpcache = $state(false);

  // State Logs
  let logType = $state<'fpm' | 'slow'>('fpm');
  let logContent = $state('');
  let loadingLogs = $state(false);

  // State phpinfo
  let phpInfoContent = $state('');
  let loadingPhpInfo = $state(false);

  // Periksa apakah berkas php.ini di editor sedang dalam keadaan dirty (telah diubah)
  let isRawIniDirty = $derived(rawIniContent !== rawIniOriginal);
  let isRawFpmDirty = $derived(rawFpmContent !== rawFpmOriginal);

  // Filter ekstensi berdasarkan kata kunci pencarian
  let filteredExtensions = $derived(
    extList.filter(
      (e) =>
        e.name.toLowerCase().includes(extSearch.toLowerCase()) ||
        e.description.toLowerCase().includes(extSearch.toLowerCase()) ||
        e.type.toLowerCase().includes(extSearch.toLowerCase())
    )
  );

  // Hitung jumlah baris editor php.ini
  $effect(() => {
    const lines = rawIniContent.split('\n').length;
    rawIniLineNumbers = Array.from({ length: Math.max(lines, 1) }, (_, i) => i + 1);
  });

  onMount(async () => {
    await loadPHPVersions();
  });

  // Memuat daftar seluruh versi PHP yang terdeteksi
  async function loadPHPVersions() {
    loading = true;
    try {
      const res = await apiFetch<{ ok: boolean; versions: PHPVersionInfo[] }>('/api/php/versions');
      versions = res?.versions || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat status PHP', 'error');
    } finally {
      loading = false;
    }
  }

  // Buka modal manajemen detail untuk versi PHP terpilih
  async function openManageModal(ver: string, tab: ManageTab = 'config') {
    activeVersion = ver;
    activeTab = tab;
    showManageModal = true;
    await fetchVersionDetail(ver);

    if (tab === 'raw_ini') {
      await fetchRawIni(ver);
    } else if (tab === 'raw_fpm') {
      await fetchRawFpm(ver);
    } else if (tab === 'logs') {
      await fetchLogs(ver, logType);
    } else if (tab === 'phpinfo') {
      await fetchPhpInfo(ver);
    }
  }

  // Ambil data detail versi PHP dari backend
  async function fetchVersionDetail(ver: string) {
    loadingDetail = true;
    try {
      const res = await apiFetch<{ ok: boolean; detail: PHPVersionDetail }>(`/api/php/versions/${ver}/detail`);
      if (res?.detail) {
        activeDetail = res.detail;
        // Salin ke form visual
        formMaxExec = res.detail.settings['max_execution_time'] || '300';
        formMaxInput = res.detail.settings['max_input_time'] || '300';
        formMemoryLimit = res.detail.settings['memory_limit'] || '256M';
        formUploadMax = res.detail.settings['upload_max_filesize'] || '64M';
        formPostMax = res.detail.settings['post_max_size'] || '64M';
        formDisplayErrors = res.detail.settings['display_errors'] || 'Off';
        formErrorReporting = res.detail.settings['error_reporting'] || 'E_ALL & ~E_DEPRECATED & ~E_STRICT';
        formTimezone = res.detail.settings['date.timezone'] || 'UTC';

        // Salin ke pool FPM
        fpmPm = res.detail.fpm_settings['pm'] || 'dynamic';
        fpmMaxChildren = res.detail.fpm_settings['pm.max_children'] || '50';
        fpmStartServers = res.detail.fpm_settings['pm.start_servers'] || '5';
        fpmMinSpare = res.detail.fpm_settings['pm.min_spare_servers'] || '5';
        fpmMaxSpare = res.detail.fpm_settings['pm.max_spare_servers'] || '35';
        fpmMaxRequests = res.detail.fpm_settings['pm.max_requests'] || '500';

        // Disabled functions & ekstensi
        disabledFuncs = [...(res.detail.disabled_functions || [])];
        extList = [...(res.detail.all_extensions || [])];

        // OPcache
        opcacheEnable = res.detail.opcache_settings['opcache.enable'] || '1';
        opcacheMem = res.detail.opcache_settings['opcache.memory_consumption'] || '128';
        opcacheMaxFiles = res.detail.opcache_settings['opcache.max_accelerated_files'] || '10000';
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat rincian konfigurasi PHP', 'error');
    } finally {
      loadingDetail = false;
    }
  }

  // Kontrol Layanan PHP-FPM (Start, Stop, Restart, Reload)
  async function controlService(action: 'start' | 'stop' | 'restart' | 'reload') {
    if (!activeVersion) return;
    const actionLabels: Record<string, string> = {
      start: 'Menyalakan',
      stop: 'Menghentikan',
      restart: 'Memulai ulang (Restart)',
      reload: 'Memuat ulang konfigurasi (Reload)',
    };
    if (action === 'stop') {
      const ok = await confirmDialog({
        title: `Hentikan PHP ${activeVersion} FPM?`,
        message: `Layanan php${activeVersion}-fpm akan dihentikan. Semua situs yang mengandalkan versi ini tidak akan dapat memproses request skrip PHP sampai dinyalakan kembali.`,
        confirmText: 'Hentikan Layanan',
        tone: 'danger',
      });
      if (!ok) return;
    }

    try {
      notify(`${actionLabels[action]} layanan php${activeVersion}-fpm...`, 'info');
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/service`, {
        method: 'POST',
        body: JSON.stringify({ action }),
      });
      notify(res.message || 'Aksi layanan berhasil dijalankan', 'success');
      await fetchVersionDetail(activeVersion);
      await loadPHPVersions();
    } catch (err: any) {
      notify(err.message || `Gagal melakukan ${action} layanan`, 'error');
    }
  }

  // Simpan Form Visual Parameter (Screenshot 1 aaPanel)
  async function submitVisualConfig() {
    if (!activeVersion) return;
    savingConfig = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/save-config`, {
        method: 'POST',
        body: JSON.stringify({
          settings: {
            max_execution_time: formMaxExec.trim(),
            max_input_time: formMaxInput.trim(),
            memory_limit: formMemoryLimit.trim(),
            upload_max_filesize: formUploadMax.trim(),
            post_max_size: formPostMax.trim(),
            display_errors: formDisplayErrors.trim(),
            error_reporting: formErrorReporting.trim(),
            'date.timezone': formTimezone.trim(),
          },
        }),
      });
      notify(res.message || 'Konfigurasi visual berhasil disimpan & FPM di-reload', 'success');
      await fetchVersionDetail(activeVersion);
      await loadPHPVersions();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi visual', 'error');
    } finally {
      savingConfig = false;
    }
  }

  // Ambil Teks Mentah php.ini (Screenshot 4 & 5 aaPanel)
  async function fetchRawIni(ver: string) {
    loadingRawIni = true;
    try {
      const res = await apiFetch<{ ok: boolean; path: string; content: string }>(`/api/php/versions/${ver}/raw-ini`);
      rawIniContent = res.content || '';
      rawIniOriginal = res.content || '';
      rawIniPath = res.path || '';
    } catch (err: any) {
      notify(err.message || 'Gagal mengambil berkas php.ini', 'error');
    } finally {
      loadingRawIni = false;
    }
  }

  // Simpan Teks Mentah php.ini
  async function submitRawIni() {
    if (!activeVersion) return;
    savingRawIni = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/raw-ini`, {
        method: 'POST',
        body: JSON.stringify({ content: rawIniContent }),
      });
      notify(res.message || 'Berkas php.ini berhasil disimpan & FPM di-reload', 'success');
      rawIniOriginal = rawIniContent;
      await fetchVersionDetail(activeVersion);
      await loadPHPVersions();
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan berkas php.ini', 'error');
    } finally {
      savingRawIni = false;
    }
  }

  // Ambil Teks Mentah Pool www.conf
  async function fetchRawFpm(ver: string) {
    loadingRawFpm = true;
    try {
      const res = await apiFetch<{ ok: boolean; path: string; content: string }>(`/api/php/versions/${ver}/raw-fpm`);
      rawFpmContent = res.content || '';
      rawFpmOriginal = res.content || '';
      rawFpmPath = res.path || '';
    } catch (err: any) {
      notify(err.message || 'Gagal mengambil berkas pool www.conf', 'error');
    } finally {
      loadingRawFpm = false;
    }
  }

  // Simpan Teks Mentah Pool www.conf
  async function submitRawFpm() {
    if (!activeVersion) return;
    savingRawFpm = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/raw-fpm`, {
        method: 'POST',
        body: JSON.stringify({ content: rawFpmContent }),
      });
      notify(res.message || 'Berkas pool www.conf berhasil disimpan & FPM di-reload', 'success');
      rawFpmOriginal = rawFpmContent;
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan berkas pool www.conf', 'error');
    } finally {
      savingRawFpm = false;
    }
  }

  // Simpan Form Visual FPM Pool
  async function submitFpmPool() {
    if (!activeVersion) return;
    savingFpmPool = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/save-fpm`, {
        method: 'POST',
        body: JSON.stringify({
          settings: {
            pm: fpmPm,
            'pm.max_children': fpmMaxChildren.trim(),
            'pm.start_servers': fpmStartServers.trim(),
            'pm.min_spare_servers': fpmMinSpare.trim(),
            'pm.max_spare_servers': fpmMaxSpare.trim(),
            'pm.max_requests': fpmMaxRequests.trim(),
          },
        }),
      });
      notify(res.message || 'Konfigurasi pool FPM berhasil disimpan & di-reload', 'success');
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan konfigurasi pool FPM', 'error');
    } finally {
      savingFpmPool = false;
    }
  }

  // Terapkan Preset Optimasi Pool FPM berdasarkan RAM
  function applyFpmRamPreset(ramGb: number) {
    if (ramGb <= 1) {
      fpmPm = 'dynamic';
      fpmMaxChildren = '25';
      fpmStartServers = '5';
      fpmMinSpare = '5';
      fpmMaxSpare = '15';
      fpmMaxRequests = '500';
    } else if (ramGb <= 2) {
      fpmPm = 'dynamic';
      fpmMaxChildren = '50';
      fpmStartServers = '10';
      fpmMinSpare = '10';
      fpmMaxSpare = '30';
      fpmMaxRequests = '1000';
    } else if (ramGb <= 4) {
      fpmPm = 'dynamic';
      fpmMaxChildren = '100';
      fpmStartServers = '20';
      fpmMinSpare = '15';
      fpmMaxSpare = '50';
      fpmMaxRequests = '2000';
    } else {
      fpmPm = 'dynamic';
      fpmMaxChildren = '200';
      fpmStartServers = '40';
      fpmMinSpare = '25';
      fpmMaxSpare = '100';
      fpmMaxRequests = '4000';
    }
    notify(`Preset FPM untuk server RAM ${ramGb}GB diterapkan ke formulir`, 'info');
  }

  // Tambah fungsi ke daftar disabled_functions
  function addDisabledFunction() {
    const fn = newFuncInput.trim().toLowerCase();
    if (!fn) return;
    if (disabledFuncs.includes(fn)) {
      notify(`Fungsi "${fn}" sudah ada di daftar blokir`, 'warning');
      return;
    }
    disabledFuncs = [...disabledFuncs, fn];
    newFuncInput = '';
  }

  // Hapus fungsi dari daftar disabled_functions
  function removeDisabledFunction(fn: string) {
    disabledFuncs = disabledFuncs.filter((f) => f !== fn);
  }

  // Simpan Disabled Functions
  async function submitDisabledFunctions() {
    if (!activeVersion) return;
    savingDisabledFuncs = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/disabled-functions`, {
        method: 'POST',
        body: JSON.stringify({ functions: disabledFuncs }),
      });
      notify(res.message || 'Daftar disable_functions berhasil disimpan & FPM di-reload', 'success');
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan disable_functions', 'error');
    } finally {
      savingDisabledFuncs = false;
    }
  }

  // Terapkan Preset Disabled Functions
  function applyDisabledPreset(type: 'strict' | 'compat') {
    if (type === 'strict') {
      disabledFuncs = [
        'passthru', 'exec', 'system', 'chroot', 'chgrp', 'chown', 'shell_exec', 'proc_open',
        'proc_get_status', 'popen', 'ini_alter', 'ini_restore', 'dl', 'openlog', 'syslog',
        'readlink', 'symlink', 'popepassthru', 'stream_socket_server',
      ];
      notify('Preset keamanan ketat (Strict Enterprise) diterapkan ke formulir', 'info');
    } else {
      // Izinkan proc_open, popen, exec untuk Laravel / Composer
      disabledFuncs = ['passthru', 'system', 'chroot', 'chgrp', 'chown', 'dl', 'openlog', 'syslog'];
      notify('Preset kompatibel framework (Laravel/WP) diterapkan', 'info');
    }
  }

  // Simpan Pengaturan OPcache
  async function submitOpcache() {
    if (!activeVersion) return;
    savingOpcache = true;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/versions/${activeVersion}/save-config`, {
        method: 'POST',
        body: JSON.stringify({
          settings: {
            'opcache.enable': opcacheEnable,
            'opcache.memory_consumption': opcacheMem.trim(),
            'opcache.max_accelerated_files': opcacheMaxFiles.trim(),
          },
        }),
      });
      notify(res.message || 'Pengaturan OPcache berhasil disimpan & FPM di-reload', 'success');
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || 'Gagal menyimpan pengaturan OPcache', 'error');
    } finally {
      savingOpcache = false;
    }
  }

  // Pasang ekstensi PHP secara otomatis (1-Klik via Panel)
  async function installExtension(ext: PHPExtensionInfo) {
    if (!activeVersion) return;
    const pkg = ext.package_name || `php${activeVersion}-${ext.name}`;
    const ok = await confirmDialog({
      title: `Pasang Ekstensi ${ext.name}?`,
      message: `Pasang paket ${pkg} untuk runtime PHP ${activeVersion}? Paket akan diinstal otomatis via APT dan layanan php${activeVersion}-fpm akan di-reload seketika.`,
      confirmText: 'Mulai Pasang',
      tone: 'default',
    });
    if (!ok) return;

    actionExtName = ext.name;
    try {
      notify(`Memasang ekstensi ${ext.name} (PHP ${activeVersion})...`, 'info');
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/php/versions/${activeVersion}/extensions/install`,
        {
          method: 'POST',
          body: JSON.stringify({ extension: ext.name }),
        }
      );
      notify(res.message || `Ekstensi ${ext.name} berhasil dipasang!`, 'success');
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || `Gagal memasang ekstensi ${ext.name}`, 'error');
    } finally {
      actionExtName = null;
    }
  }

  // Copot ekstensi PHP dan reload FPM
  async function uninstallExtension(ext: PHPExtensionInfo) {
    if (!activeVersion) return;
    const pkg = ext.package_name || `php${activeVersion}-${ext.name}`;
    const ok = await confirmDialog({
      title: `Copot Ekstensi ${ext.name}?`,
      message: `Apakah Anda yakin ingin mencopot paket ${pkg} dari PHP ${activeVersion}? Aplikasi web yang membutuhkan ekstensi ini mungkin akan mengalami error.`,
      confirmText: 'Copot Sekarang',
      tone: 'danger',
    });
    if (!ok) return;

    actionExtName = ext.name;
    try {
      notify(`Mencopot ekstensi ${ext.name}...`, 'info');
      const res = await apiFetch<{ ok: boolean; message: string }>(
        `/api/php/versions/${activeVersion}/extensions/uninstall`,
        {
          method: 'POST',
          body: JSON.stringify({ extension: ext.name }),
        }
      );
      notify(res.message || `Ekstensi ${ext.name} berhasil dicopot!`, 'success');
      await fetchVersionDetail(activeVersion);
    } catch (err: any) {
      notify(err.message || `Gagal mencopot ekstensi ${ext.name}`, 'error');
    } finally {
      actionExtName = null;
    }
  }

  // Buka modal opsi perintah terminal SSH
  function openSshModal(ext: PHPExtensionInfo) {
    sshModalExt = ext;
    showSshModal = true;
  }

  // Salin perintah SSH ke clipboard dengan fallback aman universal
  function copyToClipboard(text: string, label = 'Perintah SSH') {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(() => {
        notify(`${label} berhasil disalin ke papan klip!`, 'success');
      }).catch(() => {
        fallbackCopy(text, label);
      });
    } else {
      fallbackCopy(text, label);
    }
  }

  function fallbackCopy(text: string, label: string) {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.focus();
    ta.select();
    try {
      document.execCommand('copy');
      notify(`${label} berhasil disalin ke papan klip!`, 'success');
    } catch {
      notify(`Gagal menyalin ${label}`, 'error');
    }
    document.body.removeChild(ta);
  }


  // Ambil Log PHP-FPM
  async function fetchLogs(ver: string, type: 'fpm' | 'slow') {
    loadingLogs = true;
    try {
      const res = await apiFetch<{ ok: boolean; logs: string }>(`/api/php/versions/${ver}/logs?type=${type}`);
      logContent = res.logs || 'Belum ada catatan aktivitas log.';
    } catch (err: any) {
      notify(err.message || 'Gagal mengambil log PHP', 'error');
    } finally {
      loadingLogs = false;
    }
  }

  // Bersihkan Log PHP-FPM
  async function clearLogs() {
    if (!activeVersion) return;
    const ok = await confirmDialog({
      title: 'Kosongkan Berkas Log?',
      message: `Apakah Anda yakin ingin mengosongkan berkas ${logType === 'fpm' ? 'Error Log' : 'Slow Log'} PHP ${activeVersion}? Data riwayat akan dihapus permanen.`,
      confirmText: 'Kosongkan Log',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/php/versions/${activeVersion}/logs/clear?type=${logType}`, { method: 'POST' });
      notify('Berkas log berhasil dikosongkan', 'success');
      await fetchLogs(activeVersion, logType);
    } catch (err: any) {
      notify(err.message || 'Gagal mengosongkan log', 'error');
    }
  }

  // Ambil output phpinfo live
  async function fetchPhpInfo(ver: string) {
    loadingPhpInfo = true;
    try {
      const res = await apiFetch<{ ok: boolean; info: string }>(`/api/php/versions/${ver}/phpinfo`);
      phpInfoContent = res.info || 'Informasi phpinfo tidak tersedia.';
    } catch (err: any) {
      notify(err.message || 'Gagal mengambil phpinfo', 'error');
    } finally {
      loadingPhpInfo = false;
    }
  }

  // Pasang PHP versi baru via APT
  async function installPHP(v: PHPVersionInfo) {
    const ok = await confirmDialog({
      title: `Pasang PHP ${v.version}`,
      message: `Pasang paket runtime PHP ${v.version} dan daemon php${v.version}-fpm melalui manajer paket APT? Proses berjalan di background.`,
      confirmText: 'Mulai Pasang',
      tone: 'default',
    });
    if (!ok) return;

    try {
      installingVer = v.version;
      notify(`Memulai instalasi PHP ${v.version}...`, 'info');
      await apiFetch(`/api/appstore/install/php${v.version}`, { method: 'POST' });
      notify(`Instalasi PHP ${v.version} dimulai di background. Periksa menu App Store untuk melihat status.`, 'success');
      setTimeout(() => {
        loadPHPVersions();
      }, 5000);
    } catch (err: any) {
      notify(err.message || 'Gagal memicu instalasi PHP', 'error');
    } finally {
      installingVer = null;
    }
  }

  // Alihkan versi PHP untuk suatu situs secara instan
  async function switchSitePHP(siteId: number, domain: string, newVersion: string) {
    if (!newVersion || newVersion === activeVersion) return;
    const ok = await confirmDialog({
      title: `Alihkan Versi PHP ${domain}`,
      message: `Apakah Anda yakin ingin mengalihkan runtime PHP untuk situs "${domain}" ke PHP ${newVersion}? Konfigurasi virtual host Nginx akan dirender ulang secara instan tanpa mematikan server.`,
      confirmText: `Alihkan ke PHP ${newVersion}`,
      tone: 'default',
    });
    if (!ok) return;

    switchingSiteId = siteId;
    try {
      const res = await apiFetch<{ ok: boolean; message: string }>(`/api/php/sites/${siteId}/switch`, {
        method: 'POST',
        body: JSON.stringify({ php_version: newVersion }),
      });
      notify(res.message || `PHP untuk ${domain} berhasil dialihkan`, 'success');
      await loadPHPVersions();
      if (activeVersion) {
        await fetchVersionDetail(activeVersion);
      }
    } catch (err: any) {
      notify(err.message || 'Gagal mengalihkan versi PHP situs', 'error');
    } finally {
      switchingSiteId = null;
    }
  }

  // Penanganan tombol pintas keyboard di Editor php.ini (Ctrl+S, Ctrl+F, Tab)
  function handleEditorKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
      e.preventDefault();
      if (activeTab === 'raw_ini') {
        submitRawIni();
      } else if (activeTab === 'raw_fpm') {
        submitRawFpm();
      }
    } else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
      e.preventDefault();
      const sInput = document.getElementById('editor-search-input');
      if (sInput) sInput.focus();
    } else if (e.key === 'Tab') {
      e.preventDefault();
      const ta = e.target as HTMLTextAreaElement;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      ta.value = ta.value.substring(0, start) + '    ' + ta.value.substring(end);
      ta.selectionStart = ta.selectionEnd = start + 4;
      if (activeTab === 'raw_ini') {
        rawIniContent = ta.value;
      } else if (activeTab === 'raw_fpm') {
        rawFpmContent = ta.value;
      }
    }
  }

  // Sinkronisasi scroll gutter nomor baris dengan textarea
  function onEditorScroll(e: Event) {
    const ta = e.target as HTMLTextAreaElement;
    rawIniScrollTop = ta.scrollTop;
    const gutter = document.getElementById('editor-gutter');
    if (gutter) {
      gutter.scrollTop = ta.scrollTop;
    }
  }

  // Navigasi pencarian teks di editor php.ini
  function performEditorSearch(forward = true) {
    if (!rawIniSearch || !rawIniTextarea) return;
    const text = rawIniTextarea.value.toLowerCase();
    const query = rawIniSearch.toLowerCase();
    const currentPos = forward ? rawIniTextarea.selectionEnd : rawIniTextarea.selectionStart - 1;

    let idx = forward ? text.indexOf(query, currentPos) : text.lastIndexOf(query, currentPos);
    if (idx === -1) {
      // Loop kembali dari awal atau akhir
      idx = forward ? text.indexOf(query, 0) : text.lastIndexOf(query);
    }

    if (idx !== -1) {
      rawIniTextarea.focus();
      rawIniTextarea.setSelectionRange(idx, idx + query.length);
      // Scroll textarea ke baris pencarian
      const linesBefore = text.substring(0, idx).split('\n').length;
      const approxScrollTop = (linesBefore - 5) * 20;
      rawIniTextarea.scrollTop = Math.max(0, approxScrollTop);
    } else {
      notify(`Pencarian "${rawIniSearch}" tidak ditemukan`, 'warning');
    }
  }

  // Ganti Tab di dalam Modal
  async function switchTab(tab: ManageTab) {
    activeTab = tab;
    if (!activeVersion) return;
    if (tab === 'raw_ini' && !rawIniContent) {
      await fetchRawIni(activeVersion);
    } else if (tab === 'raw_fpm' && !rawFpmContent) {
      await fetchRawFpm(activeVersion);
    } else if (tab === 'logs') {
      await fetchLogs(activeVersion, logType);
    } else if (tab === 'phpinfo') {
      await fetchPhpInfo(activeVersion);
    }
  }
</script>

<div class="pg-phpmanager php-view">
  <!-- Header Halaman -->
  <header class="page-header">
    <div class="page-header__titles">
      <h1 class="page-header__title">
        <i class="ri-code-s-slash-line" aria-hidden="true"></i> PHP Multi-Version & FPM Tuning
      </h1>
      <p class="page-header__subtitle">
        Deteksi otomatis versi PHP aktif di server, kelola konfigurasi php.ini visual & raw editor, tuning worker FPM pool, dan monitoring modul.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn-ghost" onclick={loadPHPVersions} disabled={loading} title="Periksa ulang biner dan socket PHP">
        <i class="ri-refresh-line {loading ? 'spin' : ''}" aria-hidden="true"></i> Refresh Status
      </button>
    </div>
  </header>

  <!-- Grid Kartu Versi PHP Server (Screenshot aaPanel Style) -->
  <VersionCards
    {versions}
    {installingVer}
    onOpenManageModal={openManageModal}
    onInstallPHP={installPHP}
  />

<!-- ===================================================================== -->
<!-- Modal Komprehensif PHP-X.XManage (Gabungan Gambar 1, 2, 3, 4, 5 aaPanel) -->
<!-- ===================================================================== -->
<Modal
  open={Boolean(showManageModal && activeVersion)}
  size="xl"
  onclose={() => (showManageModal = false)}
>
  {#snippet header()}
    <div class="flex items-center gap-3">
      <div class="header-icon">
        <i class="ri-code-s-slash-line"></i>
      </div>
      <div class="modal__head-text">
        <h3 class="modal__title">php-{activeVersion}Manage</h3>
        <span class="modal__sub">Panel Konfigurasi Dinamis PHP & Worker Pool FastCGI</span>
      </div>
    </div>
    <div class="flex items-center gap-2">
      {#if activeDetail?.running}
        <span class="badge badge--success"><i class="ri-record-circle-line"></i> RUNNING (PID: {activeDetail.serviceStatus?.pid || '-'})</span>
      {:else if activeDetail?.installed}
        <span class="badge badge--danger"><i class="ri-stop-circle-line"></i> STOPPED</span>
      {/if}
    </div>
  {/snippet}

  {#if activeVersion}
    <!-- Modal Body: 2 Kolom (Sidebar Navigasi + Area Konten) -->
    <div class="manage-modal-body">
      <!-- Sidebar Menu Kiri (aaPanel Screenshot 3 Style) -->
      <aside class="manage-sidebar">
        <nav class="tabs tabs--vertical">
          <button
            class="tabs__tab {activeTab === 'service' ? 'is-active' : ''}"
            onclick={() => switchTab('service')}
          >
            <i class="ri-pulse-line"></i> Service
          </button>

          <button
            class="tabs__tab {activeTab === 'sites' ? 'is-active' : ''}"
            onclick={() => switchTab('sites')}
          >
            <i class="ri-global-line"></i> Situs Terhubung
            {#if activeDetail?.sites_count}
              <span class="nav-badge">{activeDetail.sites_count}</span>
            {/if}
          </button>

          <button
            class="tabs__tab {activeTab === 'config' ? 'is-active' : ''}"
            onclick={() => switchTab('config')}
          >
            <i class="ri-sliders-line"></i> Configuration
          </button>

          <button
            class="tabs__tab {activeTab === 'raw_ini' ? 'is-active' : ''}"
            onclick={() => switchTab('raw_ini')}
          >
            <i class="ri-file-code-line"></i> Configuration file
            {#if isRawIniDirty}
              <span class="dirty-dot" title="Perubahan belum disimpan"></span>
            {/if}
          </button>

          <button
            class="tabs__tab {activeTab === 'fpm_pool' ? 'is-active' : ''}"
            onclick={() => switchTab('fpm_pool')}
          >
            <i class="ri-cpu-line"></i> FPM profile
          </button>

          <button
            class="tabs__tab {activeTab === 'raw_fpm' ? 'is-active' : ''}"
            onclick={() => switchTab('raw_fpm')}
          >
            <i class="ri-file-settings-line"></i> FPM www.conf
          </button>

          <button
            class="tabs__tab {activeTab === 'disabled_func' ? 'is-active' : ''}"
            onclick={() => switchTab('disabled_func')}
          >
            <i class="ri-forbid-2-line"></i> Disabled functions
          </button>

          <button
            class="tabs__tab {activeTab === 'extensions' ? 'is-active' : ''}"
            onclick={() => switchTab('extensions')}
          >
            <i class="ri-puzzle-line"></i> Install extensions
          </button>

          <button
            class="tabs__tab {activeTab === 'opcache' ? 'is-active' : ''}"
            onclick={() => switchTab('opcache')}
          >
            <i class="ri-flashlight-line"></i> Optimization
          </button>

          <button
            class="tabs__tab {activeTab === 'logs' ? 'is-active' : ''}"
            onclick={() => switchTab('logs')}
          >
            <i class="ri-file-list-3-line"></i> Logs
          </button>

          <button
            class="tabs__tab {activeTab === 'phpinfo' ? 'is-active' : ''}"
            onclick={() => switchTab('phpinfo')}
          >
            <i class="ri-information-line"></i> phpinfo
          </button>
        </nav>
      </aside>

      <!-- Area Konten Utama Kanan -->
      <main class="manage-content">
        {#if loadingDetail}
          <div class="state state--loading">
            <i class="ri-loader-4-line spin"></i>
            <span>Memuat konfigurasi PHP {activeVersion}...</span>
          </div>

        <!-- ================= TAB 1: SERVICE ================= -->
        {:else if activeTab === 'service'}
          <ServiceTab
            activeVersion={activeVersion!}
            {activeDetail}
            onControlService={controlService}
          />

        <!-- ================= TAB: SITUS TERHUBUNG (HESTIACP / AAPANEL STYLE) ================= -->
        {:else if activeTab === 'sites'}
          <div class="tab-pane">
            <div class="section-title-wrap flex justify-between items-center">
              <div>
                <h4 class="section-title">Situs Terhubung: PHP {activeVersion}</h4>
                <p class="section-desc">
                  Daftar seluruh virtual host web di server yang mengeksekusi skrip PHP melalui socket php{activeVersion}-fpm.
                </p>
              </div>
              <a href="#/sites" class="btn btn-ghost btn-sm">
                <i class="ri-external-link-line"></i> Buka Menu Websites
              </a>
            </div>

            {#if activeDetail?.sites && activeDetail.sites.length > 0}
              <div class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>Domain Situs</th>
                      <th>Pengguna Linux</th>
                      <th>Direktori Root</th>
                      <th>Status</th>
                      <th>Ganti Versi PHP</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each activeDetail.sites as site (site.id)}
                      <tr>
                        <td>
                          <div class="flex items-center gap-2">
                            <span class="dot {site.enabled ? 'dot--success' : ''}"></span>
                            <span class="domain-title font-semibold">{site.domain}</span>
                            <a href="http://{site.domain}" target="_blank" rel="noopener noreferrer" class="link-external" title="Kunjungi website">
                              <i class="ri-external-link-line"></i>
                            </a>
                          </div>
                        </td>
                        <td>
                          <span class="user-pill"><i class="ri-user-line"></i> {site.user}</span>
                        </td>
                        <td>
                          <span class="mono text-xs text-subtle truncate max-w-xs" title={site.root_path}>{site.root_path}</span>
                        </td>
                        <td>
                          {#if site.enabled}
                            <span class="badge badge--success">Aktif</span>
                          {:else}
                            <span class="badge badge--neutral">Nonaktif</span>
                          {/if}
                        </td>
                        <td>
                          <div class="inline-flex items-center gap-2">
                            <select
                              class="form-select form-select-sm"
                              value={activeVersion}
                              onchange={(e) => switchSitePHP(site.id, site.domain, e.currentTarget.value)}
                              disabled={switchingSiteId === site.id}
                            >
                              {#each versions.filter((ver) => ver.installed) as pv (pv.version)}
                                <option value={pv.version}>PHP {pv.version} {pv.running ? '(FPM Aktif)' : ''}</option>
                              {/each}
                            </select>
                            {#if switchingSiteId === site.id}
                              <i class="ri-loader-4-line spin text-accent"></i>
                            {/if}
                          </div>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {:else}
              <div class="state">
                <i class="ri-global-line state__icon"></i>
                <h5 class="state__title">Belum Ada Situs Terhubung</h5>
                <p class="state__desc">
                  Saat ini belum ada virtual host yang dikonfigurasi untuk mengeksekusi skrip melalui PHP {activeVersion}. Anda dapat memilih versi ini saat menambah situs baru atau mengalihkan situs yang ada di menu Websites.
                </p>
                <div class="state__actions">
                  <a href="#/sites" class="btn btn-primary btn-sm">
                    <i class="ri-arrow-right-line"></i> Buka Menu Websites
                  </a>
                </div>
              </div>
            {/if}
          </div>

          <!-- ================= TAB 2: CONFIGURATION (VISUAL ALA GAMBAR 1) ================= -->
          {:else if activeTab === 'config'}
            <div class="tab-pane">
              <div class="section-title-wrap flex justify-between items-center">
                <div>
                  <h4 class="section-title">Configure Server: PHP {activeVersion}</h4>
                  <p class="section-desc">Atur parameter dasar php.ini secara visual dengan penyesuaian nilai interaktif.</p>
                </div>
                <button class="btn btn-ghost btn-sm" onclick={() => switchTab('raw_ini')}>
                  <i class="ri-file-code-line"></i> Buka Editor php.ini
                </button>
              </div>

              <div class="visual-form">
                <!-- max_execution_time -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-max-exec" class="f-label">max_execution_time</label>
                    <span class="f-hint">Batas waktu eksekusi skrip dalam detik sebelum dipaksa berhenti.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-max-exec" type="text" class="form-input mono" bind:value={formMaxExec} placeholder="300" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formMaxExec = '60')}>60s</button>
                      <button type="button" class="chip" onclick={() => (formMaxExec = '300')}>300s</button>
                      <button type="button" class="chip" onclick={() => (formMaxExec = '600')}>600s</button>
                      <button type="button" class="chip" onclick={() => (formMaxExec = '1200')}>1200s</button>
                    </div>
                  </div>
                </div>

                <!-- max_input_time -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-max-input" class="f-label">max_input_time</label>
                    <span class="f-hint">Waktu maksimum skrip mem-parsing data POST / GET dalam detik.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-max-input" type="text" class="form-input mono" bind:value={formMaxInput} placeholder="300" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formMaxInput = '60')}>60s</button>
                      <button type="button" class="chip" onclick={() => (formMaxInput = '300')}>300s</button>
                      <button type="button" class="chip" onclick={() => (formMaxInput = '600')}>600s</button>
                    </div>
                  </div>
                </div>

                <!-- memory_limit -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-mem-limit" class="f-label">memory_limit</label>
                    <span class="f-hint">Batas memori RAM maksimum yang diizinkan untuk setiap skrip PHP.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-mem-limit" type="text" class="form-input mono" bind:value={formMemoryLimit} placeholder="256M" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formMemoryLimit = '128M')}>128M</button>
                      <button type="button" class="chip" onclick={() => (formMemoryLimit = '256M')}>256M</button>
                      <button type="button" class="chip" onclick={() => (formMemoryLimit = '512M')}>512M</button>
                      <button type="button" class="chip" onclick={() => (formMemoryLimit = '1024M')}>1024M</button>
                      <button type="button" class="chip" onclick={() => (formMemoryLimit = '2048M')}>2048M</button>
                    </div>
                  </div>
                </div>

                <!-- upload_max_filesize -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-upload-max" class="f-label">upload_max_filesize</label>
                    <span class="f-hint">Ukuran maksimum sebuah berkas yang diunggah ke server.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-upload-max" type="text" class="form-input mono" bind:value={formUploadMax} placeholder="64M" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formUploadMax = '32M')}>32M</button>
                      <button type="button" class="chip" onclick={() => (formUploadMax = '64M')}>64M</button>
                      <button type="button" class="chip" onclick={() => (formUploadMax = '100M')}>100M</button>
                      <button type="button" class="chip" onclick={() => (formUploadMax = '256M')}>256M</button>
                      <button type="button" class="chip" onclick={() => (formUploadMax = '512M')}>512M</button>
                    </div>
                  </div>
                </div>

                <!-- post_max_size -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-post-max" class="f-label">post_max_size</label>
                    <span class="f-hint">Ukuran total muatan data POST HTTP (harus >= upload_max_filesize).</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-post-max" type="text" class="form-input mono" bind:value={formPostMax} placeholder="64M" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formPostMax = '64M')}>64M</button>
                      <button type="button" class="chip" onclick={() => (formPostMax = '100M')}>100M</button>
                      <button type="button" class="chip" onclick={() => (formPostMax = '256M')}>256M</button>
                      <button type="button" class="chip" onclick={() => (formPostMax = '512M')}>512M</button>
                    </div>
                  </div>
                </div>

                <!-- display_errors -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-display-errors" class="f-label">display_errors</label>
                    <span class="f-hint">Cetak galat langsung ke output browser (Gunakan Off di Production).</span>
                  </div>
                  <div class="form-input-wrap">
                    <select id="f-display-errors" class="form-select" bind:value={formDisplayErrors}>
                      <option value="Off">Off (Rekomendasi Aman)</option>
                      <option value="On">On (Mode Debugging / Dev)</option>
                    </select>
                  </div>
                </div>

                <!-- error_reporting -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-error-rep" class="f-label">error_reporting</label>
                    <span class="f-hint">Tingkat keparahan galat yang dicatat oleh runtime PHP.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="f-error-rep" type="text" class="form-input mono" bind:value={formErrorReporting} />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (formErrorReporting = 'E_ALL & ~E_DEPRECATED & ~E_STRICT')}>Standard Prod</button>
                      <button type="button" class="chip" onclick={() => (formErrorReporting = 'E_ALL')}>Semua (E_ALL)</button>
                      <button type="button" class="chip" onclick={() => (formErrorReporting = '0')}>Mute (0)</button>
                    </div>
                  </div>
                </div>

                <!-- date.timezone -->
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="f-tz" class="f-label">date.timezone</label>
                    <span class="f-hint">Zona waktu standar untuk seluruh fungsi tanggal PHP.</span>
                  </div>
                  <div class="form-input-wrap">
                    <div class="flex gap-2">
                      <input id="f-tz" type="text" class="form-input mono flex-1" bind:value={formTimezone} placeholder="UTC" />
                      <button type="button" class="btn btn-ghost btn-sm" onclick={() => (formTimezone = 'Asia/Jakarta')}>Asia/Jakarta</button>
                      <button type="button" class="btn btn-ghost btn-sm" onclick={() => (formTimezone = 'UTC')}>UTC</button>
                    </div>
                  </div>
                </div>
              </div>

              <div class="form-footer flex justify-between items-center">
                <span class="text-xs text-subtle">
                  Perubahan akan disimpan langsung ke <code class="mono">{activeDetail?.iniPath}</code> dan layanan FPM akan dimuat ulang.
                </span>
                <button class="btn btn-primary" onclick={submitVisualConfig} disabled={savingConfig}>
                  {savingConfig ? 'Menyimpan...' : 'Simpan & Reload PHP-FPM'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 3: CONFIGURATION FILE (EDITOR PHP.INI ALA GAMBAR 4 & 5) ================= -->
          {:else if activeTab === 'raw_ini'}
            <div class="tab-pane full-height-pane">
              <!-- Tips Bar ala aaPanel Screenshot 4 -->
              <div class="editor-tips-bar flex justify-between items-center">
                <div class="tips-text">
                  <span class="font-semibold text-accent"><i class="ri-lightbulb-line"></i> Tips:</span>
                  <span><kbd>Ctrl+F</kbd> Cari kata kunci, <kbd>Ctrl+S</kbd> Simpan berkas, <kbd>Tab</kbd> Indentasi 4 spasi</span>
                </div>
                <div class="flex items-center gap-2">
                  <span class="file-path-badge mono" title={rawIniPath}>
                    <i class="ri-file-text-line"></i> {rawIniPath || 'php.ini'}
                  </span>
                  {#if isRawIniDirty}
                    <span class="badge badge--warning">Belum Disimpan</span>
                  {/if}
                </div>
              </div>

              <!-- Search Bar Interaktif -->
              <div class="editor-search-bar flex items-center gap-2">
                <div class="search-input-wrap flex-1">
                  <i class="ri-search-line search-icon"></i>
                  <input
                    id="editor-search-input"
                    type="text"
                    class="editor-search-input"
                    placeholder="Ketik kata kunci untuk mencari di php.ini (misal: memory_limit, upload_max, disable_functions)..."
                    bind:value={rawIniSearch}
                    onkeydown={(e) => {
                      if (e.key === 'Enter') performEditorSearch(!e.shiftKey);
                    }}
                  />
                </div>
                <button class="btn btn-ghost btn-sm" onclick={() => performEditorSearch(false)} title="Cari sebelumnya (Shift+Enter)">
                  <i class="ri-arrow-up-s-line"></i>
                </button>
                <button class="btn btn-ghost btn-sm" onclick={() => performEditorSearch(true)} title="Cari selanjutnya (Enter)">
                  <i class="ri-arrow-down-s-line"></i>
                </button>
                <button class="btn btn-ghost btn-sm" onclick={() => fetchRawIni(activeVersion!)} disabled={loadingRawIni} title="Muat ulang dari disk (Batalkan perubahan)">
                  <i class="ri-restart-line"></i> Reset
                </button>
              </div>

              <!-- Textarea Code Editor dengan Nomor Baris Gutter (Screenshot 4 & 5) -->
              <div class="code-editor-wrapper flex-1">
                {#if loadingRawIni}
                  <div class="editor-loading">
                    <i class="ri-loader-4-line spin text-2xl"></i>
                    <span>Membaca berkas php.ini...</span>
                  </div>
                {:else}
                  <div class="editor-gutter mono" id="editor-gutter">
                    {#each rawIniLineNumbers as num}
                      <div class="gutter-num">{num}</div>
                    {/each}
                  </div>
                  <textarea
                    bind:this={rawIniTextarea}
                    class="editor-textarea mono"
                    bind:value={rawIniContent}
                    onkeydown={handleEditorKeydown}
                    onscroll={onEditorScroll}
                    spellcheck="false"
                    wrap="off"
                    placeholder="[PHP] ..."
                  ></textarea>
                {/if}
              </div>

              <!-- Footer Editor ala aaPanel Screenshot 4 -->
              <div class="editor-footer flex justify-between items-center">
                <p class="editor-note text-xs text-subtle">
                  • Berkas konfigurasi utama PHP {activeVersion}. Jangan mengedit sembarangan jika belum memahami format sintaks direktif PHP INI.
                </p>
                <button class="btn btn-primary" onclick={submitRawIni} disabled={savingRawIni || loadingRawIni}>
                  <i class="ri-save-line" aria-hidden="true"></i> {savingRawIni ? 'Menyimpan...' : 'Save'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 4: FPM PROFILE ================= -->
          {:else if activeTab === 'fpm_pool'}
            <div class="tab-pane">
              <div class="section-title-wrap flex justify-between items-center">
                <div>
                  <h4 class="section-title">FPM Profile (Pool: www.conf)</h4>
                  <p class="section-desc">Konfigurasi alokasi worker process FastCGI untuk performa maksimal dan efisiensi RAM.</p>
                </div>
                <button class="btn btn-ghost btn-sm" onclick={() => switchTab('raw_fpm')}>
                  <i class="ri-file-settings-line"></i> Buka www.conf Mentah
                </button>
              </div>

              <!-- Preset RAM 1-Klik -->
              <div class="preset-box">
                <span class="preset-title">Rekomendasi Preset Cepat Berdasarkan Kapasitas RAM:</span>
                <div class="flex flex-wrap gap-2 mt-2">
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyFpmRamPreset(1)}>RAM 1 GB</button>
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyFpmRamPreset(2)}>RAM 2 GB</button>
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyFpmRamPreset(4)}>RAM 4 GB</button>
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyFpmRamPreset(8)}>RAM 8 GB+</button>
                </div>
              </div>

              <div class="visual-form mt-4">
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-pm" class="f-label">pm (Process Manager)</label>
                    <span class="f-hint">Metode pembuatan proses worker (dynamic sangat disarankan untuk fleksibilitas).</span>
                  </div>
                  <div class="form-input-wrap">
                    <select id="fpm-pm" class="form-select" bind:value={fpmPm}>
                      <option value="dynamic">dynamic (Menyesuaikan beban trafik secara dinamis)</option>
                      <option value="static">static (Jumlah worker konstan di memori)</option>
                      <option value="ondemand">ondemand (Worker hanya dibuat saat request datang)</option>
                    </select>
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-mc" class="f-label">pm.max_children</label>
                    <span class="f-hint">Jumlah maksimum proses worker bersamaan yang diizinkan aktif.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="fpm-mc" type="text" class="form-input mono" bind:value={fpmMaxChildren} />
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-ss" class="f-label">pm.start_servers</label>
                    <span class="f-hint">Jumlah worker yang langsung dinyalakan saat layanan PHP-FPM start.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="fpm-ss" type="text" class="form-input mono" bind:value={fpmStartServers} />
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-min-s" class="f-label">pm.min_spare_servers</label>
                    <span class="f-hint">Batas minimum worker idle yang selalu siap melayani request baru.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="fpm-min-s" type="text" class="form-input mono" bind:value={fpmMinSpare} />
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-max-s" class="f-label">pm.max_spare_servers</label>
                    <span class="f-hint">Batas maksimum worker idle sebelum dipangkas oleh master process.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="fpm-max-s" type="text" class="form-input mono" bind:value={fpmMaxSpare} />
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="fpm-mr" class="f-label">pm.max_requests</label>
                    <span class="f-hint">Daur ulang worker setelah melayani X request untuk mencegah memory leak.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="fpm-mr" type="text" class="form-input mono" bind:value={fpmMaxRequests} />
                  </div>
                </div>
              </div>

              <div class="form-footer flex justify-end">
                <button class="btn btn-primary" onclick={submitFpmPool} disabled={savingFpmPool}>
                  {savingFpmPool ? 'Menyimpan...' : 'Simpan & Reload FPM'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 5: RAW FPM WWW.CONF ================= -->
          {:else if activeTab === 'raw_fpm'}
            <div class="tab-pane full-height-pane">
              <div class="editor-tips-bar flex justify-between items-center">
                <div class="tips-text">
                  <span class="font-semibold text-accent"><i class="ri-settings-line"></i> Pool Config:</span>
                  <span><kbd>Ctrl+S</kbd> Simpan berkas pool www.conf</span>
                </div>
                <span class="file-path-badge mono">{rawFpmPath || 'pool.d/www.conf'}</span>
              </div>

              <div class="code-editor-wrapper flex-1">
                {#if loadingRawFpm}
                  <div class="editor-loading">
                    <i class="ri-loader-4-line spin text-2xl"></i>
                    <span>Membaca berkas pool www.conf...</span>
                  </div>
                {:else}
                  <textarea
                    class="editor-textarea mono p-4"
                    bind:value={rawFpmContent}
                    onkeydown={handleEditorKeydown}
                    spellcheck="false"
                    wrap="off"
                  ></textarea>
                {/if}
              </div>

              <div class="editor-footer flex justify-between items-center">
                <p class="editor-note text-xs text-subtle">
                  • Berkas pool FPM www.conf asli. Mengubah konfigurasi socket atau user worker di sini akan langsung di-reload ke daemon.
                </p>
                <button class="btn btn-primary" onclick={submitRawFpm} disabled={savingRawFpm || loadingRawFpm}>
                  <i class="ri-save-line"></i> {savingRawFpm ? 'Menyimpan...' : 'Save'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 6: DISABLED FUNCTIONS ================= -->
          {:else if activeTab === 'disabled_func'}
            <div class="tab-pane">
              <div class="section-title-wrap">
                <h4 class="section-title">Disabled Functions (Fungsi Dinonaktifkan)</h4>
                <p class="section-desc">
                  Blokir eksekusi perintah shell dan fungsi berbahaya di tingkat PHP runtime untuk melindungi server dari serangan eksekusi kode berbahaya.
                </p>
              </div>

              <div class="flex justify-between items-center mb-3">
                <div class="flex gap-2">
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyDisabledPreset('strict')}>
                    <i class="ri-shield-check-line"></i> Preset Ketat (Enterprise)
                  </button>
                  <button type="button" class="btn btn-ghost btn-sm" onclick={() => applyDisabledPreset('compat')}>
                    <i class="ri-apps-line"></i> Preset Kompatibel (Laravel/WP)
                  </button>
                  <button type="button" class="btn btn-ghost btn-sm text-danger" onclick={() => (disabledFuncs = [])}>
                    <i class="ri-delete-bin-line"></i> Kosongkan Semua
                  </button>
                </div>
              </div>

              <!-- Input Tambah Fungsi -->
              <div class="add-func-wrap flex gap-2 mb-4">
                <input
                  type="text"
                  class="form-input mono flex-1"
                  placeholder="Ketik nama fungsi PHP (misal: exec, shell_exec, system, proc_open)..."
                  bind:value={newFuncInput}
                  onkeydown={(e) => { if (e.key === 'Enter') addDisabledFunction(); }}
                />
                <button type="button" class="btn btn-primary" onclick={addDisabledFunction}>
                  <i class="ri-add-line"></i> Tambahkan
                </button>
              </div>

              <!-- Visual Chips Container -->
              <div class="chips-container">
                {#if disabledFuncs.length === 0}
                  <p class="text-subtle text-sm p-4 text-center">
                    Tidak ada fungsi yang dinonaktifkan. Semua fungsi PHP dapat dieksekusi secara bebas.
                  </p>
                {:else}
                  {#each disabledFuncs as fn (fn)}
                    <span class="func-chip">
                      <span class="mono">{fn}</span>
                      <button
                        type="button"
                        class="chip-remove"
                        onclick={() => removeDisabledFunction(fn)}
                        title="Hapus {fn} dari daftar blokir"
                        aria-label="Hapus fungsi {fn}"
                      >
                        <i class="ri-close-line"></i>
                      </button>
                    </span>
                  {/each}
                {/if}
              </div>

              <div class="form-footer flex justify-between items-center mt-4">
                <span class="text-xs text-subtle">
                  Total: {disabledFuncs.length} fungsi diblokir pada <code class="mono">disable_functions</code>.
                </span>
                <button class="btn btn-primary" onclick={submitDisabledFunctions} disabled={savingDisabledFuncs}>
                  {savingDisabledFuncs ? 'Menyimpan...' : 'Simpan & Terapkan ke php.ini'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 7: EXTENSIONS ================= -->
          {:else if activeTab === 'extensions'}
            <div class="tab-pane">
              <div class="section-title-wrap flex justify-between items-center">
                <div>
                  <h4 class="section-title">Install Extensions (Ekstensi PHP {activeVersion})</h4>
                  <p class="section-desc">Status modul dan pustaka ekstensi PHP populer di server.</p>
                </div>
                <div class="search-mini-wrap">
                  <i class="ri-search-line"></i>
                  <input
                    type="text"
                    class="search-mini-input"
                    placeholder="Cari ekstensi (redis, opcache, imagick)..."
                    bind:value={extSearch}
                  />
                </div>
              </div>

              <div class="table-wrap extensions-table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>Nama Ekstensi</th>
                      <th>Kategori</th>
                      <th>Deskripsi</th>
                      <th>Status</th>
                      <th>Paket APT</th>
                      <th>Tindakan & Opsi SSH</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each filteredExtensions as ext (ext.name)}
                      <tr>
                        <td class="font-bold mono text-text">{ext.name}</td>
                        <td>
                          <span class="cat-pill {ext.type}">{ext.type}</span>
                        </td>
                        <td class="text-xs text-muted">{ext.description}</td>
                        <td>
                          {#if ext.installed}
                            <span class="badge badge--success"><i class="ri-check-line"></i> Terpasang</span>
                          {:else}
                            <span class="badge badge--neutral">Belum ada</span>
                          {/if}
                        </td>
                        <td>
                          <code class="apt-hint mono">{ext.package_name || 'php' + activeVersion + '-' + ext.name}</code>
                        </td>
                        <td>
                          <div class="ext-action-group flex items-center gap-2">
                            {#if ext.installed}
                              <span class="badge-active-runtime">
                                <i class="ri-checkbox-circle-fill"></i> Aktif
                              </span>
                              {#if !ext.is_core}
                                <button
                                  type="button"
                                  class="btn btn-xs btn-danger"
                                  onclick={() => uninstallExtension(ext)}
                                  disabled={actionExtName === ext.name}
                                  title="Copot paket ekstensi {ext.name}"
                                >
                                  {#if actionExtName === ext.name}
                                    <i class="ri-loader-4-line spin"></i>
                                  {:else}
                                    <i class="ri-delete-bin-line"></i> Copot
                                  {/if}
                                </button>
                              {/if}
                            {:else}
                              <button
                                type="button"
                                class="btn btn-xs btn-primary"
                                onclick={() => installExtension(ext)}
                                disabled={actionExtName === ext.name}
                                title="Pasang ekstensi {ext.name} secara otomatis (1-Klik via Panel)"
                              >
                                {#if actionExtName === ext.name}
                                  <i class="ri-loader-4-line spin"></i> Memasang...
                                {:else}
                                  <i class="ri-download-2-line"></i> Install
                                {/if}
                              </button>
                            {/if}

                            <button
                              type="button"
                              class="btn btn-xs btn-ghost"
                              onclick={() => openSshModal(ext)}
                              title="Lihat & Salin Perintah Instalasi via Terminal / SSH"
                            >
                              <i class="ri-terminal-box-line"></i> Perintah SSH
                            </button>
                          </div>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>

          <!-- ================= TAB 8: OPTIMIZATION (OPCACHE) ================= -->
          {:else if activeTab === 'opcache'}
            <div class="tab-pane">
              <div class="section-title-wrap">
                <h4 class="section-title">Optimization (Zend OPcache)</h4>
                <p class="section-desc">Optimasi bytecode compiler caching untuk mempercepat load time aplikasi berbasis PHP hingga 3x lipat.</p>
              </div>

              <div class="visual-form">
                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="opc-en" class="f-label">opcache.enable</label>
                    <span class="f-hint">Aktifkan Zend OPcache bytecode compiler cache.</span>
                  </div>
                  <div class="form-input-wrap">
                    <select id="opc-en" class="form-select" bind:value={opcacheEnable}>
                      <option value="1">1 (Aktif - Sangat Disarankan)</option>
                      <option value="0">0 (Nonaktif)</option>
                    </select>
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="opc-mem" class="f-label">opcache.memory_consumption</label>
                    <span class="f-hint">Alokasi memori RAM terdedikasi untuk penyimpanan bytecode cache (MB).</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="opc-mem" type="text" class="form-input mono" bind:value={opcacheMem} placeholder="128" />
                    <div class="quick-chips">
                      <button type="button" class="chip" onclick={() => (opcacheMem = '64')}>64M</button>
                      <button type="button" class="chip" onclick={() => (opcacheMem = '128')}>128M</button>
                      <button type="button" class="chip" onclick={() => (opcacheMem = '256')}>256M</button>
                      <button type="button" class="chip" onclick={() => (opcacheMem = '512')}>512M</button>
                    </div>
                  </div>
                </div>

                <div class="form-row">
                  <div class="form-label-wrap">
                    <label for="opc-files" class="f-label">opcache.max_accelerated_files</label>
                    <span class="f-hint">Jumlah maksimum file skrip PHP yang dapat disimpan di dalam tabel hash cache.</span>
                  </div>
                  <div class="form-input-wrap">
                    <input id="opc-files" type="text" class="form-input mono" bind:value={opcacheMaxFiles} placeholder="10000" />
                  </div>
                </div>
              </div>

              <div class="form-footer flex justify-end">
                <button class="btn btn-primary" onclick={submitOpcache} disabled={savingOpcache}>
                  {savingOpcache ? 'Menyimpan...' : 'Simpan & Terapkan OPcache'}
                </button>
              </div>
            </div>

          <!-- ================= TAB 9: LOGS ================= -->
          {:else if activeTab === 'logs'}
            <div class="tab-pane full-height-pane">
              <div class="flex justify-between items-center mb-3">
                <div class="flex gap-2">
                  <button
                    class="btn btn-sm {logType === 'fpm' ? 'btn-primary' : 'btn-ghost'}"
                    onclick={() => { logType = 'fpm'; fetchLogs(activeVersion!, 'fpm'); }}
                  >
                    <i class="ri-error-warning-line"></i> Error Log (php-fpm.log)
                  </button>
                  <button
                    class="btn btn-sm {logType === 'slow' ? 'btn-primary' : 'btn-ghost'}"
                    onclick={() => { logType = 'slow'; fetchLogs(activeVersion!, 'slow'); }}
                  >
                    <i class="ri-timer-line"></i> Slow Log
                  </button>
                </div>
                <div class="flex gap-2">
                  <button class="btn btn-ghost btn-sm" onclick={() => fetchLogs(activeVersion!, logType)} disabled={loadingLogs}>
                    <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i> Refresh
                  </button>
                  <button class="btn btn-danger btn-sm" onclick={clearLogs}>
                    <i class="ri-delete-bin-line"></i> Kosongkan Log
                  </button>
                </div>
              </div>

              <div class="terminal-log-viewer flex-1">
                {#if loadingLogs}
                  <div class="editor-loading">
                    <i class="ri-loader-4-line spin text-2xl"></i>
                    <span>Mengambil data log terbaru...</span>
                  </div>
                {:else}
                  <pre class="log-pre mono">{logContent}</pre>
                {/if}
              </div>
            </div>

          <!-- ================= TAB 10: PHPINFO ================= -->
          {:else if activeTab === 'phpinfo'}
            <div class="tab-pane full-height-pane">
              <div class="flex justify-between items-center mb-3">
                <div>
                  <h4 class="section-title">Diagnostik phpinfo() — PHP {activeVersion}</h4>
                  <p class="section-desc">Ringkasan konfigurasi internal, library dependensi, dan environment runtime.</p>
                </div>
                <button class="btn btn-ghost btn-sm" onclick={() => fetchPhpInfo(activeVersion!)} disabled={loadingPhpInfo}>
                  <i class="ri-refresh-line {loadingPhpInfo ? 'spin' : ''}"></i> Refresh Info
                </button>
              </div>

              <div class="terminal-log-viewer flex-1">
                {#if loadingPhpInfo}
                  <div class="editor-loading">
                    <i class="ri-loader-4-line spin text-2xl"></i>
                    <span>Mengeksekusi php -i...</span>
                  </div>
                {:else}
                  <pre class="log-pre mono">{phpInfoContent}</pre>
                {/if}
              </div>
            </div>
          {/if}
        </main>
      </div>
  {/if}
</Modal>

<Modal
  open={Boolean(showSshModal && sshModalExt)}
  size="xl"
  onclose={() => (showSshModal = false)}
>
  {#snippet header()}
    {#if sshModalExt}
      <div class="flex items-center gap-3">
        <div class="header-icon">
          <i class="ri-terminal-box-line"></i>
        </div>
        <div class="modal__head-text">
          <h3 class="modal__title">Perintah Terminal / SSH: {sshModalExt.name}</h3>
          <p class="modal__sub">
            Ekstensi PHP {activeVersion} &bull; Paket APT: <span class="mono text-accent">{sshModalExt.package_name || `php${activeVersion}-${sshModalExt.name}`}</span>
          </p>
        </div>
      </div>
    {/if}
  {/snippet}

  {#if sshModalExt}
    <div class="flex flex-col gap-3">
      <div class="ssh-notice">
        <i class="ri-information-line"></i>
        <span>
          Jika Anda ingin menginstal atau mengelola ekstensi ini langsung melalui sesi terminal SSH server, gunakan perintah siap-pakai berikut:
        </span>
      </div>

      <!-- Perintah Install -->
      <div class="terminal-cmd-box">
        <div class="terminal-cmd-header flex justify-between items-center">
          <span class="cmd-type-label"><i class="ri-download-2-line"></i> Perintah Instalasi (Install)</span>
          <button
            type="button"
            class="btn btn-xs btn-ghost"
            onclick={() =>
              copyToClipboard(
                sshModalExt?.install_command ||
                  `sudo DEBIAN_FRONTEND=noninteractive apt-get update && sudo apt-get install -y php${activeVersion}-${sshModalExt?.name} && sudo systemctl restart php${activeVersion}-fpm`,
                'Perintah Install'
              )}
          >
            <i class="ri-file-copy-line"></i> Salin Perintah
          </button>
        </div>
        <pre class="terminal-snippet mono"><code>{sshModalExt.install_command || `sudo DEBIAN_FRONTEND=noninteractive apt-get update && sudo apt-get install -y php${activeVersion}-${sshModalExt.name} && sudo systemctl restart php${activeVersion}-fpm`}</code></pre>
      </div>

      <!-- Perintah Uninstall (Jika bukan core) -->
      {#if !sshModalExt.is_core}
        <div class="terminal-cmd-box mt-3">
          <div class="terminal-cmd-header flex justify-between items-center">
            <span class="cmd-type-label text-warning"><i class="ri-delete-bin-line"></i> Perintah Copot (Uninstall)</span>
            <button
              type="button"
              class="btn btn-xs btn-ghost"
              onclick={() =>
                copyToClipboard(
                  sshModalExt?.uninstall_command ||
                    `sudo DEBIAN_FRONTEND=noninteractive apt-get remove -y php${activeVersion}-${sshModalExt?.name} && sudo systemctl restart php${activeVersion}-fpm`,
                  'Perintah Uninstall'
                )}
            >
              <i class="ri-file-copy-line"></i> Salin Perintah
            </button>
          </div>
          <pre class="terminal-snippet mono"><code>{sshModalExt.uninstall_command || `sudo DEBIAN_FRONTEND=noninteractive apt-get remove -y php${activeVersion}-${sshModalExt.name} && sudo systemctl restart php${activeVersion}-fpm`}</code></pre>
        </div>
      {/if}

      <!-- Pilihan Instalasi Otomatis 1-Klik -->
      <div class="one-click-promo mt-4">
        <div class="flex justify-between items-center flex-wrap gap-2">
          <div>
            <span class="font-semibold text-text text-sm">Tidak ingin repot buka terminal SSH?</span>
            <p class="text-xs text-subtle">FikPanel dapat mengeksekusi instalasi paket ini secara otomatis di latar belakang.</p>
          </div>
          {#if sshModalExt.installed}
            {#if !sshModalExt.is_core}
              <button
                type="button"
                class="btn btn-sm btn-danger"
                onclick={() => {
                  const ext = sshModalExt;
                  showSshModal = false;
                  if (ext) uninstallExtension(ext);
                }}
              >
                <i class="ri-delete-bin-line"></i> Copot via Panel
              </button>
            {/if}
          {:else}
            <button
              type="button"
              class="btn btn-sm btn-primary"
              onclick={() => {
                const ext = sshModalExt;
                showSshModal = false;
                if (ext) installExtension(ext);
              }}
            >
              <i class="ri-download-2-line"></i> Pasang Otomatis Sekarang
            </button>
          {/if}
        </div>
      </div>
    </div>
  {/if}

  {#snippet footer()}
    <button type="button" class="btn btn-ghost" onclick={() => (showSshModal = false)}>Tutup</button>
  {/snippet}
</Modal>
</div>
