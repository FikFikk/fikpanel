<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { CronJob, CronLog, ServerTimeInfo, SystemCronEntry } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/cron.css';

  let jobs = $state<CronJob[]>([]);
  let systemEntries = $state<SystemCronEntry[]>([]);
  let loading = $state(true);
  let loadingSystem = $state(false);
  let activeView = $state<'panel' | 'system'>('panel');
  let searchQuery = $state('');
  let statusFilter = $state<'all' | 'enabled' | 'paused' | 'failed'>('all');

  let showCreateModal = $state(false);
  let editingJob = $state<CronJob | null>(null);
  let showLogModal = $state(false);
  let selectedJob = $state<CronJob | null>(null);
  let logs = $state<CronLog[]>([]);
  let loadingLogs = $state(false);
  let stoppingJobId = $state<number | null>(null);

  // Waktu server & zona waktu
  let serverTimeInfo = $state<ServerTimeInfo | null>(null);
  let serverNow = $state<Date>(new Date());
  let clockInterval: ReturnType<typeof setInterval> | null = null;

  // State formulir tambah / edit tugas
  let formName = $state('');
  let formCommand = $state('');
  let formWorkingDir = $state('');
  let formTimeout = $state(3600);
  let formSaveLog = $state(true);
  let formNotifyOnFail = $state(true);
  let formSaveLogFile = $state(false);
  let formLogFile = $state('');
  let submitting = $state(false);

  // State modal riwayat log (berkas fisik .log & database SQLite)
  let activeLogTab = $state<'file' | 'db'>('file');
  let fileLogContent = $state('');
  let fileLogPath = $state('');
  let loadingFileLog = $state(false);

  // Pembuat jadwal visual 5-kolom crontab
  type TabType = 'minutes' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom';
  let activeTab = $state<TabType>('daily');
  let timezoneBasis = $state<'client' | 'server'>('client');

  let genMinuteInterval = $state(15);
  let genHourlyMinute = $state(0);
  let genDailyHour = $state(12);
  let genDailyMinute = $state(0);
  let genWeeklyDay = $state(1); // 1 = Senin, 0 = Minggu
  let genWeeklyHour = $state(2);
  let genWeeklyMinute = $state(0);
  let genMonthlyDate = $state(1);
  let genMonthlyHour = $state(3);
  let genMonthlyMinute = $state(0);

  let fieldMinute = $state('0');
  let fieldHour = $state('5');
  let fieldDay = $state('*');
  let fieldMonth = $state('*');
  let fieldDow = $state('*');

  let selectedPreset = $state('custom');

  // Preset perintah siap produksi untuk server Linux
  const PRESETS: Record<string, string> = {
    custom: '',
    mysql_backup:
      'mkdir -p /var/backups/fikpanel/mysql && ( [ -f /etc/mysql/debian.cnf ] && mysqldump --defaults-file=/etc/mysql/debian.cnf --all-databases --single-transaction || mysqldump -u root --all-databases --single-transaction ) | gzip > /var/backups/fikpanel/mysql/all_$(date +%F_%H%M).sql.gz && find /var/backups/fikpanel/mysql -name "*.sql.gz" -mtime +7 -delete',
    docker_prune: 'docker image prune -af && docker builder prune -af',
    laravel_artisan: 'php artisan schedule:run',
    nginx_reload: 'nginx -t && systemctl reload nginx',
    curl_webhook: 'curl -fsS --max-time 15 --retry 3 https://example.com/api/heartbeat-cron',
    clean_logs: 'find /var/log/nginx/ -type f -name "*.log.*.gz" -mtime +14 -delete',
  };

  const WEEKDAYS = [
    { id: 1, label: 'Senin' },
    { id: 2, label: 'Selasa' },
    { id: 3, label: 'Rabu' },
    { id: 4, label: 'Kamis' },
    { id: 5, label: 'Jumat' },
    { id: 6, label: 'Sabtu' },
    { id: 0, label: 'Minggu' },
  ];

  // Ringkasan statistik real-time tugas FikPanel
  let stats = $derived.by(() => {
    let active = 0;
    let running = 0;
    let failed = 0;
    for (const j of jobs) {
      if (j.enabled) active++;
      if (j.is_running || j.last_status === 'running') running++;
      if (j.last_status === 'failed' || j.last_status === 'timeout') failed++;
    }
    return { total: jobs.length, active, running, failed };
  });

  // Penyaringan tabel berdasarkan pencarian dan status
  let filteredJobs = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    return jobs.filter((j) => {
      if (statusFilter === 'enabled' && !j.enabled) return false;
      if (statusFilter === 'paused' && j.enabled) return false;
      if (statusFilter === 'failed' && j.last_status !== 'failed' && j.last_status !== 'timeout') return false;
      if (!q) return true;
      return (
        j.name.toLowerCase().includes(q) ||
        j.command.toLowerCase().includes(q) ||
        j.spec.toLowerCase().includes(q) ||
        (j.log_file || '').toLowerCase().includes(q)
      );
    });
  });

  // Apakah ada tugas yang sedang berjalan (memicu polling cepat otomatis tiap 3 detik)
  let hasRunningJob = $derived(jobs.some((j) => j.is_running || j.last_status === 'running'));

  // Kalkulasi selisih jam klien terhadap waktu server
  function getTimezoneDiffHours(): number {
    const clientOffsetSec = -(new Date().getTimezoneOffset() * 60);
    const serverOffsetSec = serverTimeInfo ? serverTimeInfo.offset_sec : 0;
    return Math.round((clientOffsetSec - serverOffsetSec) / 3600);
  }

  function getClientTimezoneLabel(): string {
    const offsetMin = -new Date().getTimezoneOffset();
    const sign = offsetMin >= 0 ? '+' : '-';
    const hrs = Math.floor(Math.abs(offsetMin) / 60);
    return offsetMin === 420 ? 'WIB (UTC+7)' : `Waktu Lokal (UTC${sign}${hrs})`;
  }

  function getServerTimezoneLabel(): string {
    if (!serverTimeInfo) return 'Waktu Server';
    const hrs = Math.floor(serverTimeInfo.offset_sec / 3600);
    const sign = hrs >= 0 ? '+' : '';
    return `${serverTimeInfo.timezone_name || 'UTC'} (UTC${sign}${hrs})`;
  }

  // Sinkronkan 5 kolom crontab dari tab generator visual (konversi zona waktu aman di kedua arah +/-)
  function updateCrontabFromGenerator() {
    const diff = getTimezoneDiffHours();

    if (activeTab === 'minutes') {
      fieldMinute = genMinuteInterval <= 1 ? '*' : `*/${genMinuteInterval}`;
      fieldHour = '*';
      fieldDay = '*';
      fieldMonth = '*';
      fieldDow = '*';
    } else if (activeTab === 'hourly') {
      fieldMinute = String(genHourlyMinute);
      fieldHour = '*';
      fieldDay = '*';
      fieldMonth = '*';
      fieldDow = '*';
    } else if (activeTab === 'daily') {
      let targetHour = genDailyHour;
      if (timezoneBasis === 'client') {
        targetHour = ((genDailyHour - diff) % 24 + 24) % 24;
      }
      fieldMinute = String(genDailyMinute);
      fieldHour = String(targetHour);
      fieldDay = '*';
      fieldMonth = '*';
      fieldDow = '*';
    } else if (activeTab === 'weekly') {
      let targetHour = genWeeklyHour;
      let targetDow = genWeeklyDay;
      if (timezoneBasis === 'client') {
        const rawHour = genWeeklyHour - diff;
        if (rawHour < 0) {
          targetHour = rawHour + 24;
          targetDow = (targetDow - 1 + 7) % 7;
        } else if (rawHour >= 24) {
          targetHour = rawHour - 24;
          targetDow = (targetDow + 1) % 7;
        } else {
          targetHour = rawHour;
        }
      }
      fieldMinute = String(genWeeklyMinute);
      fieldHour = String(targetHour);
      fieldDay = '*';
      fieldMonth = '*';
      fieldDow = String(targetDow);
    } else if (activeTab === 'monthly') {
      let targetHour = genMonthlyHour;
      let targetDate = genMonthlyDate;
      if (timezoneBasis === 'client') {
        const rawHour = genMonthlyHour - diff;
        if (rawHour < 0) {
          targetHour = rawHour + 24;
          targetDate = targetDate > 1 ? targetDate - 1 : 28;
        } else if (rawHour >= 24) {
          targetHour = rawHour - 24;
          targetDate = targetDate < 31 ? targetDate + 1 : 1;
        } else {
          targetHour = rawHour;
        }
      }
      fieldMinute = String(genMonthlyMinute);
      fieldHour = String(targetHour);
      fieldDay = String(targetDate);
      fieldMonth = '*';
      fieldDow = '*';
    }
  }

  const MONTH_NAMES: Record<string, string> = {
    '1': 'Januari', '2': 'Februari', '3': 'Maret', '4': 'April',
    '5': 'Mei', '6': 'Juni', '7': 'Juli', '8': 'Agustus',
    '9': 'September', '10': 'Oktober', '11': 'November', '12': 'Desember',
  };

  const DOW_NAMES: Record<string, string> = {
    '0': 'Minggu', '1': 'Senin', '2': 'Selasa', '3': 'Rabu',
    '4': 'Kamis', '5': 'Jumat', '6': 'Sabtu', '7': 'Minggu',
  };

  // Menerjemahkan ekspresi 5-kolom crontab ke bahasa Indonesia yang mudah dipahami
  function explainCustomCron(
    min: string,
    hr: string,
    dom: string,
    mon: string,
    dow: string,
    diffHours: number,
    clientTz: string,
  ): string {
    const m = (min || '*').trim();
    const h = (hr || '*').trim();
    const d = (dom || '*').trim();
    const mo = (mon || '*').trim();
    const dw = (dow || '*').trim();

    if (m === '*' && h === '*' && d === '*' && mo === '*' && dw === '*') {
      return 'Dijalankan setiap menit (60 kali per jam).';
    }
    if (m.startsWith('*/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const step = m.replace('*/', '');
      return `Dijalankan setiap ${step} menit sekali.`;
    }
    if (!m.includes('*') && !m.includes('/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const mPad = m.padStart(2, '0');
      return `Dijalankan setiap jam pada menit ke-${mPad}.`;
    }
    if (h.startsWith('*/') && d === '*' && mo === '*' && dw === '*') {
      const stepH = h.replace('*/', '');
      const mPad = m === '*' ? 'setiap menit' : `menit ke-${m.padStart(2, '0')}`;
      return `Dijalankan setiap ${stepH} jam sekali pada ${mPad}.`;
    }

    const isSingleH = /^\d+$/.test(h);
    const isSingleM = /^\d+$/.test(m);
    let timeHint = '';
    let timeLabel = '';

    if (isSingleH && isSingleM) {
      const hNum = parseInt(h, 10);
      const mNum = parseInt(m, 10);
      const mPad = String(mNum).padStart(2, '0');
      const hPad = String(hNum).padStart(2, '0');
      timeLabel = `pukul ${hPad}:${mPad}`;
      if (diffHours !== 0) {
        const clientH = ((hNum + diffHours) % 24 + 24) % 24;
        timeHint = ` (server, setara ${String(clientH).padStart(2, '0')}:${mPad} ${clientTz})`;
      }
    } else if (h !== '*') {
      timeLabel = `jam ${h} (menit ${m})`;
    } else {
      timeLabel = `menit ${m}`;
    }

    if (d === '*' && mo === '*' && dw === '*') {
      return `Dijalankan setiap hari pada ${timeLabel}${timeHint}.`;
    }
    if (d !== '*' && mo === '*' && dw === '*') {
      let tglDesc = `tanggal ${d}`;
      if (d === '1') tglDesc = 'tanggal 1 (awal bulan)';
      else if (d.toUpperCase() === 'L') tglDesc = 'hari terakhir setiap bulan';
      return `Dijalankan setiap bulan pada ${tglDesc} ${timeLabel}${timeHint}.`;
    }
    if (d === '*' && mo === '*' && dw !== '*') {
      let hariDesc = '';
      if (dw === '1-5' || dw.toUpperCase() === 'MON-FRI') hariDesc = 'hari kerja (Senin–Jumat)';
      else if (dw === '0,6' || dw === '6,7') hariDesc = 'akhir pekan (Sabtu & Minggu)';
      else {
        const parts = dw.split(',').map((x) => DOW_NAMES[x.trim()] || `hari ${x}`);
        hariDesc = `hari ${parts.join(', ')}`;
      }
      return `Dijalankan setiap ${hariDesc} pada ${timeLabel}${timeHint}.`;
    }
    if (d !== '*' && mo !== '*' && dw === '*') {
      const namaBulan = MONTH_NAMES[mo] || `bulan ${mo}`;
      return `Dijalankan setiap tahun tanggal ${d} ${namaBulan} ${timeLabel}${timeHint}.`;
    }
    if (d !== '*' && dw !== '*') {
      const parts = dw.split(',').map((x) => DOW_NAMES[x.trim()] || `hari ${x}`);
      return `Dijalankan pada tanggal ${d} atau setiap hari ${parts.join(', ')} pada ${timeLabel}${timeHint}.`;
    }
    return `Pola Crontab: Menit (${m}), Jam (${h}), Tanggal (${d}), Bulan (${mo}), Hari (${dw})${timeHint}.`;
  }

  function applyCustomPreset(m: string, h: string, d: string, mo: string, dw: string) {
    fieldMinute = m;
    fieldHour = h;
    fieldDay = d;
    fieldMonth = mo;
    fieldDow = dw;
  }

  let scheduleHumanExplanation = $derived.by(() => {
    const diff = getTimezoneDiffHours();
    const clientTz = getClientTimezoneLabel();
    const serverTz = getServerTimezoneLabel();

    if (activeTab === 'minutes') {
      return genMinuteInterval <= 1
        ? 'Tugas dijalankan setiap 1 menit secara terus-menerus.'
        : `Tugas dijalankan setiap ${genMinuteInterval} menit sekali secara berulang.`;
    }
    if (activeTab === 'hourly') {
      return `Tugas dijalankan setiap jam pada menit ke-${String(genHourlyMinute).padStart(2, '0')}.`;
    }
    if (activeTab === 'daily') {
      if (timezoneBasis === 'client') {
        const srvHour = ((genDailyHour - diff) % 24 + 24) % 24;
        return diff === 0
          ? `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz}.`
          : `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz} (dikonversi ke ${String(srvHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz}).`;
      }
      const cltHour = ((genDailyHour + diff) % 24 + 24) % 24;
      return diff === 0
        ? `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz}.`
        : `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz} (setara ${String(cltHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz}).`;
    }
    if (activeTab === 'weekly') {
      const dayName = WEEKDAYS.find((w) => w.id === genWeeklyDay)?.label || 'Hari';
      return `Berjalan setiap hari ${dayName} pukul ${String(genWeeklyHour).padStart(2, '0')}:${String(genWeeklyMinute).padStart(2, '0')} (${timezoneBasis === 'client' ? clientTz : serverTz}).`;
    }
    if (activeTab === 'monthly') {
      return `Berjalan setiap tanggal ${genMonthlyDate} pukul ${String(genMonthlyHour).padStart(2, '0')}:${String(genMonthlyMinute).padStart(2, '0')} (${timezoneBasis === 'client' ? clientTz : serverTz}).`;
    }
    return explainCustomCron(fieldMinute, fieldHour, fieldDay, fieldMonth, fieldDow, diff, clientTz);
  });

  let fullCronExpression = $derived(
    `${fieldMinute.trim()} ${fieldHour.trim()} ${fieldDay.trim()} ${fieldMonth.trim()} ${fieldDow.trim()}`,
  );

  function onPresetSelect(e: Event) {
    const val = (e.target as HTMLSelectElement).value;
    selectedPreset = val;
    if (val !== 'custom' && PRESETS[val]) {
      formCommand = PRESETS[val];
    }
  }

  function formatTime(d: Date): string {
    return d.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  function formatDate(d: Date): string {
    return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
  }

  async function loadJobs(silent = false) {
    try {
      if (!silent) loading = true;
      const res = await apiFetch<{ ok: boolean; jobs: CronJob[]; server_time?: ServerTimeInfo }>('/api/cron/jobs');
      if (res && res.jobs) {
        jobs = res.jobs;
        if (selectedJob) {
          const fresh = res.jobs.find((j) => j.id === selectedJob?.id);
          if (fresh) selectedJob = fresh;
        }
      }
      if (res && res.server_time) {
        serverTimeInfo = res.server_time;
      }
    } catch (err: any) {
      if (!silent) notify(err.message || 'Gagal memuat daftar cron job', 'error');
    } finally {
      if (!silent) loading = false;
    }
  }

  async function loadSystemCrons() {
    try {
      loadingSystem = true;
      const res = await apiFetch<{ ok: boolean; entries: SystemCronEntry[] }>('/api/cron/system');
      systemEntries = res?.entries || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memindai cron sistem OS', 'error');
    } finally {
      loadingSystem = false;
    }
  }

  // Simpan tugas cron (tambah baru atau pembaruan in-place)
  async function handleSubmitJob() {
    if (!formName.trim() || !formCommand.trim()) {
      notify('Nama tugas dan perintah eksekusi wajib diisi', 'warning');
      return;
    }
    if (formSaveLogFile && !formLogFile.trim()) {
      notify('Jalur berkas log wajib diisi bila opsi simpan log disk diaktifkan', 'warning');
      return;
    }

    try {
      submitting = true;
      const payload = {
        name: formName.trim(),
        schedule_type: activeTab === 'custom' ? 'custom' : activeTab,
        spec: fullCronExpression,
        command: formCommand.trim(),
        working_dir: formWorkingDir.trim(),
        timeout_sec: Math.max(10, Math.min(86400, Number(formTimeout) || 3600)),
        save_log: formSaveLog,
        notify_on_fail: formNotifyOnFail,
        log_file: formSaveLogFile ? formLogFile.trim() : '',
      };

      if (editingJob) {
        await apiFetch(`/api/cron/jobs/${editingJob.id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
        notify('Tugas cron berhasil diperbarui!', 'success');
      } else {
        await apiFetch('/api/cron/jobs', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        notify('Tugas cron berhasil dijadwalkan!', 'success');
      }

      showCreateModal = false;
      editingJob = null;
      resetForm();
      await loadJobs(true);
    } catch (err: any) {
      notify(err.message || (editingJob ? 'Gagal memperbarui tugas cron' : 'Gagal menambahkan tugas cron'), 'error');
    } finally {
      submitting = false;
    }
  }

  function openEditModal(job: CronJob) {
    editingJob = job;
    formName = job.name;
    formCommand = job.command;
    formWorkingDir = job.working_dir || '';
    formTimeout = job.timeout_sec || 3600;
    formSaveLog = job.save_log;
    formNotifyOnFail = job.notify_on_fail !== false;
    formSaveLogFile = !!job.log_file;
    formLogFile = job.log_file || '';

    const parts = job.spec.trim().split(/\s+/);
    if (parts.length === 5) {
      fieldMinute = parts[0];
      fieldHour = parts[1];
      fieldDay = parts[2];
      fieldMonth = parts[3];
      fieldDow = parts[4];
    }

    if (job.schedule_type && ['minutes', 'hourly', 'daily', 'weekly', 'monthly', 'custom'].includes(job.schedule_type)) {
      activeTab = job.schedule_type as TabType;
    } else {
      activeTab = 'custom';
    }

    timezoneBasis = 'server';
    if (parts.length === 5) {
      if (activeTab === 'minutes') {
        genMinuteInterval = parts[0] === '*' ? 1 : parseInt(parts[0].replace('*/', ''), 10) || 15;
      } else if (activeTab === 'hourly') {
        genHourlyMinute = parseInt(parts[0], 10) || 0;
      } else if (activeTab === 'daily') {
        genDailyHour = parseInt(parts[1], 10) || 0;
        genDailyMinute = parseInt(parts[0], 10) || 0;
      } else if (activeTab === 'weekly') {
        genWeeklyHour = parseInt(parts[1], 10) || 0;
        genWeeklyMinute = parseInt(parts[0], 10) || 0;
        genWeeklyDay = parseInt(parts[4], 10) || 0;
      } else if (activeTab === 'monthly') {
        genMonthlyDate = parseInt(parts[2], 10) || 1;
        genMonthlyHour = parseInt(parts[1], 10) || 0;
        genMonthlyMinute = parseInt(parts[0], 10) || 0;
      }
    }

    selectedPreset = 'custom';
    for (const [k, cmd] of Object.entries(PRESETS)) {
      if (cmd && job.command.trim() === cmd.trim()) {
        selectedPreset = k;
        break;
      }
    }

    showCreateModal = true;
  }

  function resetForm() {
    editingJob = null;
    formName = '';
    activeTab = 'daily';
    timezoneBasis = 'client';
    genDailyHour = 12;
    genDailyMinute = 0;
    formCommand = '';
    formWorkingDir = '';
    selectedPreset = 'custom';
    formTimeout = 3600;
    formSaveLog = true;
    formNotifyOnFail = true;
    formSaveLogFile = false;
    formLogFile = '';
    updateCrontabFromGenerator();
  }

  function autoFillLogPath(prefix: string) {
    const slug = formName.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-') || 'task';
    formLogFile = `${prefix}/${slug}.log`;
  }

  async function toggleJob(job: CronJob) {
    try {
      const res = await apiFetch<{ ok: boolean; enabled: boolean; next_run_at?: number }>(`/api/cron/jobs/${job.id}/toggle`, {
        method: 'POST',
        body: JSON.stringify({ enabled: !job.enabled }),
      });
      jobs = jobs.map((item) =>
        item.id === job.id ? { ...item, enabled: res.enabled, next_run_at: res.next_run_at ?? 0 } : item,
      );
      notify(`Tugas ${res.enabled ? 'diaktifkan' : 'dijeda'}`, 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status', 'error');
    }
  }

  // Picu eksekusi manual secara terlepas (detached) agar aman saat pindah halaman
  async function runJobNow(job: CronJob) {
    try {
      notify(`Menjalankan tugas "${job.name}"...`, 'info');
      jobs = jobs.map((item) => (item.id === job.id ? { ...item, is_running: true, last_status: 'running' } : item));
      const res = await apiFetch<{ ok: boolean; running?: boolean; log?: CronLog; message?: string }>(
        `/api/cron/jobs/${job.id}/run`,
        { method: 'POST' },
      );
      if (res?.running) {
        notify(res.message || `Tugas "${job.name}" berjalan di latar belakang`, 'info');
        await loadJobs(true);
        return;
      }
      if (res?.log) {
        await loadJobs(true);
        if (res.log.status === 'success') {
          notify(`Tugas "${job.name}" selesai (${res.log.duration_ms}ms)`, 'success');
        } else {
          notify(`Tugas "${job.name}" selesai dengan status ${res.log.status.toUpperCase()} (Exit ${res.log.exit_code})`, 'warning');
        }
      }
    } catch (err: any) {
      await loadJobs(true);
      notify(err.message || 'Gagal mengeksekusi tugas', 'error');
    }
  }

  // Hentikan paksa tugas cron yang sedang berjalan beserta seluruh anak prosesnya
  async function stopJobNow(job: CronJob) {
    const ok = await confirmDialog({
      title: 'Hentikan Tugas Cron',
      message: `Hentikan paksa tugas "${job.name}" yang sedang berjalan beserta seluruh subprosesnya?`,
      confirmText: 'Hentikan',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      stoppingJobId = job.id;
      await apiFetch(`/api/cron/jobs/${job.id}/stop`, { method: 'POST' });
      notify(`Sinyal henti dikirim ke tugas "${job.name}"`, 'warning');
      setTimeout(() => loadJobs(true), 600);
    } catch (err: any) {
      notify(err.message || 'Gagal menghentikan tugas cron', 'error');
    } finally {
      stoppingJobId = null;
    }
  }

  async function deleteJob(job: CronJob) {
    const ok = await confirmDialog({
      title: 'Hapus Tugas Cron',
      message: `Apakah Anda yakin ingin menghapus tugas "${job.name}"? Riwayat eksekusi juga akan dihapus.`,
      confirmText: 'Hapus',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cron/jobs/${job.id}`, { method: 'DELETE' });
      jobs = jobs.filter((j) => j.id !== job.id);
      notify('Tugas cron berhasil dihapus', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal menghapus tugas', 'error');
    }
  }

  async function viewLogs(job: CronJob, preferredTab?: 'file' | 'db') {
    selectedJob = job;
    showLogModal = true;
    activeLogTab = preferredTab || (job.log_file ? 'file' : 'db');
    if (job.log_file) {
      loadFileLog(job.id);
    }
    loadDbLogs(job.id);
  }

  async function loadFileLog(jobId: number) {
    loadingFileLog = true;
    try {
      const res = await apiFetch<{ ok: boolean; path: string; content: string; message?: string }>(
        `/api/cron/jobs/${jobId}/file-log`,
      );
      if (res) {
        fileLogPath = res.path;
        fileLogContent = res.content || (res.message ? `${res.message}` : '(Berkas log masih kosong)');
      }
    } catch (err: any) {
      fileLogContent = `Gagal membaca berkas log: ${err.message}`;
    } finally {
      loadingFileLog = false;
    }
  }

  async function clearFileLog(jobId: number) {
    const ok = await confirmDialog({
      title: 'Kosongkan Berkas Log',
      message: 'Apakah Anda yakin ingin mengosongkan berkas log pada disk ini?',
      confirmText: 'Kosongkan',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cron/jobs/${jobId}/file-log/clear`, { method: 'POST' });
      fileLogContent = '(Berkas log telah dikosongkan)';
      notify('Berkas log berhasil dikosongkan', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal mengosongkan berkas log', 'error');
    }
  }

  async function clearDbLogs(jobId: number) {
    const ok = await confirmDialog({
      title: 'Hapus Riwayat Database',
      message: 'Hapus seluruh catatan riwayat eksekusi tugas ini dari database panel?',
      confirmText: 'Hapus Riwayat',
      tone: 'danger',
    });
    if (!ok) return;

    try {
      await apiFetch(`/api/cron/jobs/${jobId}/logs`, { method: 'DELETE' });
      logs = [];
      notify('Riwayat eksekusi database berhasil dibersihkan', 'success');
    } catch (err: any) {
      notify(err.message || 'Gagal membersihkan riwayat log', 'error');
    }
  }

  async function loadDbLogs(jobId: number) {
    try {
      loadingLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: CronLog[] }>(`/api/cron/jobs/${jobId}/logs`);
      logs = res?.logs || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat log database', 'error');
    } finally {
      loadingLogs = false;
    }
  }

  function formatTimestamp(ts: number): string {
    if (!ts) return '-';
    const d = new Date(ts * 1000);
    return d.toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function formatNextRun(ts?: number): string {
    if (!ts) return '-';
    const d = new Date(ts * 1000);
    const diffSec = Math.max(0, Math.round((d.getTime() - Date.now()) / 1000));
    const datePart = d.toLocaleString('id-ID', {
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    });
    if (diffSec < 60) return `${datePart} (< 1 mnt)`;
    if (diffSec < 3600) return `${datePart} (${Math.round(diffSec / 60)} mnt lagi)`;
    if (diffSec < 86400) return `${datePart} (${Math.floor(diffSec / 3600)} jam lagi)`;
    return datePart;
  }

  function translateSpec(spec: string): string {
    const fields = spec.trim().split(/\s+/);
    if (fields.length === 5) {
      const [m, h, dom, mon, dow] = fields;
      return explainCustomCron(m, h, dom, mon, dow, getTimezoneDiffHours(), getClientTimezoneLabel());
    }
    return spec;
  }

  // Polling cepat tiap 3 detik hanya saat ada tugas berstatus running
  $effect(() => {
    if (!hasRunningJob) return;
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') {
        loadJobs(true);
        if (showLogModal && selectedJob) {
          loadDbLogs(selectedJob.id);
        }
      }
    }, 3000);
    return () => clearInterval(timer);
  });

  onMount(() => {
    loadJobs();
    loadSystemCrons();
    updateCrontabFromGenerator();

    clockInterval = setInterval(() => {
      if (serverTimeInfo) {
        const clientOffset = -(new Date().getTimezoneOffset() * 60);
        const diffMs = (serverTimeInfo.offset_sec - clientOffset) * 1000;
        serverNow = new Date(Date.now() + diffMs);
      } else {
        serverNow = new Date();
      }
    }, 1000);

    const onVisibility = () => {
      if (document.visibilityState === 'visible') {
        loadJobs(true);
      }
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
    };
  });

  onDestroy(() => {
    if (clockInterval) clearInterval(clockInterval);
  });
</script>

<div class="pg-cron cron-view">
  <!-- Header Halaman -->
  <header class="page-header">
    <div class="page-header__titles">
      <h1 class="page-header__title">
        <i class="ri-time-line"></i> Penjadwal Tugas (Cron Manager)
      </h1>
      <p class="page-header__sub">
        Penjadwal presisi menit (:00) dengan isolasi process-group, proteksi timeout, retensi log otomatis, dan alert webhook — Zona Server: <strong>{getServerTimezoneLabel()}</strong>.
      </p>
    </div>
    <div class="page-header__actions">
      <button
        type="button"
        class="btn btn--ghost"
        onclick={() => { loadJobs(); loadSystemCrons(); }}
        disabled={loading}
      >
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Segarkan
      </button>
      <button
        type="button"
        class="btn btn--primary"
        onclick={() => { resetForm(); showCreateModal = true; }}
      >
        <i class="ri-add-line"></i> Tambah Tugas
      </button>
    </div>
  </header>

  <!-- Bar Tab & Filter -->
  <div class="cron-toolbar">
    <div class="cron-toolbar__left">
      <div class="tabs tabs--pill">
        <button
          type="button"
          class="tabs__tab"
          class:is-active={activeView === 'panel'}
          onclick={() => (activeView = 'panel')}
        >
          <i class="ri-calendar-check-line"></i>
          <span>Tugas FikPanel ({stats.total})</span>
        </button>
        <button
          type="button"
          class="tabs__tab"
          class:is-active={activeView === 'system'}
          onclick={() => { activeView = 'system'; if (systemEntries.length === 0) loadSystemCrons(); }}
        >
          <i class="ri-terminal-line"></i>
          <span>Cron Sistem OS ({systemEntries.length})</span>
        </button>
      </div>

      {#if activeView === 'panel'}
        <div class="cron-stat-pills">
          <button
            type="button"
            class="cron-pill"
            class:is-selected={statusFilter === 'all'}
            onclick={() => (statusFilter = 'all')}
          >
            Semua <strong>{stats.total}</strong>
          </button>
          <button
            type="button"
            class="cron-pill cron-pill--success"
            class:is-selected={statusFilter === 'enabled'}
            onclick={() => (statusFilter = 'enabled')}
          >
            Aktif <strong>{stats.active}</strong>
          </button>
          <button
            type="button"
            class="cron-pill"
            class:is-selected={statusFilter === 'paused'}
            onclick={() => (statusFilter = 'paused')}
          >
            Dijeda <strong>{stats.total - stats.active}</strong>
          </button>
          {#if stats.failed > 0}
            <button
              type="button"
              class="cron-pill cron-pill--danger"
              class:is-selected={statusFilter === 'failed'}
              onclick={() => (statusFilter = 'failed')}
            >
              Gagal <strong>{stats.failed}</strong>
            </button>
          {/if}
        </div>
      {/if}
    </div>

    {#if activeView === 'panel'}
      <div class="cron-search-box">
        <i class="ri-search-line"></i>
        <input
          type="text"
          class="cron-search-input"
          placeholder="Cari nama, jadwal, atau perintah..."
          bind:value={searchQuery}
        />
        {#if searchQuery}
          <button
            type="button"
            class="cron-search-clear"
            aria-label="Hapus pencarian"
            title="Hapus pencarian"
            onclick={() => (searchQuery = '')}
          >
            &times;
          </button>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Tab 1: Daftar Tugas Terkelola FikPanel -->
  {#if activeView === 'panel'}
    <div class="card">
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>NAMA TUGAS</th>
              <th>JADWAL CRONTAB</th>
              <th>PERINTAH EKSEKUSI</th>
              <th>STATUS TERAKHIR</th>
              <th>TERAKHIR DIJALANKAN</th>
              <th class="text-right">AKSI</th>
            </tr>
          </thead>
          <tbody>
            {#if loading && jobs.length === 0}
              <tr>
                <td colspan="6">
                  <div class="state state--loading">
                    <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar tugas terjadwal...
                  </div>
                </td>
              </tr>
            {:else if filteredJobs.length === 0}
              <tr>
                <td colspan="6">
                  <div class="state state--empty">
                    {jobs.length === 0
                      ? 'Belum ada tugas cron yang didaftarkan. Klik "Tambah Tugas" untuk menjadwalkan pekerjaan otomatis.'
                      : 'Tidak ada tugas cron yang cocok dengan filter pencarian.'}
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredJobs as job (job.id)}
                {@const isRunning = Boolean(job.is_running || job.last_status === 'running')}
                <tr>
                  <td>
                    <div class="job-name-col">
                      <span
                        class="dot {isRunning ? 'dot--warning' : (job.enabled ? 'dot--success' : '')}"
                        title={isRunning ? 'Sedang berjalan' : (job.enabled ? 'Aktif' : 'Dijeda')}
                      ></span>
                      <div>
                        <span class="font-semibold block">{job.name}</span>
                        <div class="job-meta-sub mono">
                          <span>ID #{job.id}</span>
                          <span>•</span>
                          <span title="Batas waktu eksekusi">{job.timeout_sec || 3600}s</span>
                          {#if job.notify_on_fail !== false}
                            <i class="ri-notification-3-line text-accent" title="Alert Webhook aktif bila gagal/timeout"></i>
                          {/if}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td>
                    <div class="spec-cell">
                      <span class="badge-spec mono">{job.spec}</span>
                      <span class="spec-human">{translateSpec(job.spec)}</span>
                      {#if job.enabled && job.next_run_at}
                        <span class="spec-next mono" title="Jadwal eksekusi berikutnya: {formatTimestamp(job.next_run_at)}">
                          <i class="ri-arrow-right-circle-line"></i> Berikutnya: {formatNextRun(job.next_run_at)}
                        </span>
                      {:else if !job.enabled}
                        <span class="spec-next text-subtle">Dijeda</span>
                      {/if}
                    </div>
                  </td>
                  <td class="col-cmd mono" title={job.command}>
                    <div class="cmd-text">{job.command}</div>
                    <div class="cmd-pills-row">
                      {#if job.working_dir}
                        <span class="meta-chip" title="Direktori kerja: {job.working_dir}">
                          <i class="ri-folder-2-line"></i> {job.working_dir}
                        </span>
                      {/if}
                      {#if job.log_file}
                        <button
                          type="button"
                          class="log-file-pill"
                          onclick={() => viewLogs(job, 'file')}
                          title="Klik untuk membuka isi berkas log pada disk"
                        >
                          <i class="ri-file-text-line"></i> {job.log_file}
                        </button>
                      {/if}
                    </div>
                  </td>
                  <td>
                    {#if isRunning}
                      <span class="badge badge--warning"><i class="ri-loader-4-line spin"></i> Berjalan</span>
                    {:else if job.last_status === 'success'}
                      <span class="badge badge--success"><i class="ri-checkbox-circle-fill"></i> Berhasil</span>
                    {:else if job.last_status === 'failed'}
                      <span class="badge badge--danger"><i class="ri-close-circle-fill"></i> Gagal</span>
                    {:else if job.last_status === 'timeout'}
                      <span class="badge badge--danger"><i class="ri-timer-flash-line"></i> Timeout</span>
                    {:else if job.last_status === 'stopped'}
                      <span class="badge badge--warning"><i class="ri-stop-circle-line"></i> Dihentikan</span>
                    {:else if job.last_status === 'interrupted'}
                      <span class="badge badge--warning"><i class="ri-error-warning-line"></i> Terputus</span>
                    {:else}
                      <span class="text-muted">-</span>
                    {/if}
                  </td>
                  <td class="text-muted">
                    {formatTimestamp(job.last_run_at)}
                  </td>
                  <td class="text-right actions-col">
                    {#if isRunning}
                      <button
                        type="button"
                        class="btn btn--icon btn--sm btn--ghost btn--danger"
                        aria-label="Hentikan paksa tugas {job.name}"
                        title="Hentikan Paksa (Kill Process Group)"
                        disabled={stoppingJobId === job.id}
                        onclick={() => stopJobNow(job)}
                      >
                        <i class="ri-stop-circle-line"></i>
                      </button>
                    {:else}
                      <button
                        type="button"
                        class="btn btn--icon btn--sm btn--ghost"
                        aria-label="Jalankan tugas {job.name} sekarang"
                        title="Jalankan Sekarang"
                        onclick={() => runJobNow(job)}
                      >
                        <i class="ri-play-line"></i>
                      </button>
                    {/if}
                    <button
                      type="button"
                      class="btn btn--icon btn--sm btn--ghost"
                      aria-label={job.enabled ? `Jeda tugas ${job.name}` : `Aktifkan tugas ${job.name}`}
                      title={job.enabled ? 'Jeda Tugas' : 'Aktifkan Tugas'}
                      onclick={() => toggleJob(job)}
                    >
                      <i class={job.enabled ? 'ri-pause-line' : 'ri-play-circle-line'}></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--icon btn--sm btn--ghost"
                      aria-label="Edit tugas {job.name}"
                      title="Edit Tugas"
                      onclick={() => openEditModal(job)}
                    >
                      <i class="ri-edit-line"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--icon btn--sm btn--ghost"
                      aria-label="Riwayat log tugas {job.name}"
                      title="Riwayat Log"
                      onclick={() => viewLogs(job)}
                    >
                      <i class="ri-file-list-3-line"></i>
                    </button>
                    <button
                      type="button"
                      class="btn btn--icon btn--sm btn--ghost btn--danger"
                      aria-label="Hapus tugas {job.name}"
                      title="Hapus Tugas"
                      onclick={() => deleteJob(job)}
                    >
                      <i class="ri-delete-bin-line"></i>
                    </button>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {:else}
    <!-- Tab 2: Cron Sistem OS (Read-Only dari crontab root, /etc/crontab, & /etc/cron.d/*) -->
    <div class="card">
      <div class="sys-cron-banner">
        <i class="ri-information-line"></i>
        <span>Menampilkan tugas cron sistem operasi secara hanya-baca dari <code>crontab -l</code>, <code>/etc/crontab</code>, dan <code>/etc/cron.d/*</code> agar tidak bertabrakan dengan jadwal FikPanel.</span>
      </div>
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>SUMBER BERKAS</th>
              <th>JADWAL</th>
              <th>USER</th>
              <th>PERINTAH OS</th>
            </tr>
          </thead>
          <tbody>
            {#if loadingSystem}
              <tr>
                <td colspan="4">
                  <div class="state state--loading">
                    <i class="ri-loader-4-line spin text-lg"></i> Memindai crontab sistem operasi...
                  </div>
                </td>
              </tr>
            {:else if systemEntries.length === 0}
              <tr>
                <td colspan="4">
                  <div class="state state--empty">
                    Tidak ditemukan entri aktif pada crontab sistem operasi.
                  </div>
                </td>
              </tr>
            {:else}
              {#each systemEntries as item, idx (`${item.file}-${idx}`)}
                <tr>
                  <td class="mono text-xs">
                    <span class="badge-spec">{item.source}</span>
                    <span class="text-muted sys-file-label">{item.file}</span>
                  </td>
                  <td class="mono"><span class="badge-spec">{item.schedule}</span></td>
                  <td class="mono text-xs">{item.user}</td>
                  <td class="col-cmd mono" title={item.command}>
                    <div class="cmd-text">{item.command}</div>
                  </td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Modal Tambah / Edit Tugas Cron -->
  <Modal
    open={showCreateModal}
    title={editingJob ? 'Edit Tugas Terjadwal (Cron Job)' : 'Tambah Tugas Terjadwal (Cron Job)'}
    size="xl"
    onclose={() => { showCreateModal = false; editingJob = null; }}
  >
    <div class="modal-sub-bar">
      <span class="modal-sub">
        {editingJob ? 'Perbarui jadwal eksekusi, batas timeout, perintah shell, atau pengaturan notifikasi.' : 'Konfigurasi otomatisasi skrip server dengan validasi sintaks crontab dan proteksi keamanan.'}
      </span>
      <div class="server-clock-tag">
        <i class="ri-time-line"></i>
        <span>Waktu Server: <strong>{formatDate(serverNow)}, {formatTime(serverNow)}</strong></span>
      </div>
    </div>

    <form id="cron-job-form" onsubmit={(e) => { e.preventDefault(); handleSubmitJob(); }} class="form-body">
      <!-- Nama Tugas -->
      <div class="field">
        <label class="field__label" for="form-name">Nama Tugas Terjadwal</label>
        <input id="form-name" class="input" type="text" bind:value={formName} placeholder="Misal: Backup Database Harian 02:00" required />
      </div>

      <!-- Kontainer Pembuat Jadwal Visual -->
      <div class="schedule-builder-card">
        <div class="builder-head">
          <span class="builder-label"><i class="ri-calendar-schedule-line"></i> Penjadwalan Eksekusi</span>
          <div class="tabs tabs--pill">
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'minutes'}
              onclick={() => { activeTab = 'minutes'; updateCrontabFromGenerator(); }}
            >
              MENIT
            </button>
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'hourly'}
              onclick={() => { activeTab = 'hourly'; updateCrontabFromGenerator(); }}
            >
              PER JAM
            </button>
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'daily'}
              onclick={() => { activeTab = 'daily'; updateCrontabFromGenerator(); }}
            >
              HARIAN
            </button>
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'weekly'}
              onclick={() => { activeTab = 'weekly'; updateCrontabFromGenerator(); }}
            >
              MINGGUAN
            </button>
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'monthly'}
              onclick={() => { activeTab = 'monthly'; updateCrontabFromGenerator(); }}
            >
              BULANAN
            </button>
            <button
              type="button"
              class="tabs__tab"
              class:is-active={activeTab === 'custom'}
              onclick={() => { activeTab = 'custom'; }}
            >
              CUSTOM (CRONTAB)
            </button>
          </div>
        </div>

        <div class="builder-body">
          {#if activeTab === 'minutes'}
            <div class="tab-content">
              <span class="content-desc">Jalankan perintah secara berkala setiap interval menit tertentu:</span>
              <div class="input-inline-wrap">
                <span>Jalankan setiap:</span>
                <select bind:value={genMinuteInterval} onchange={updateCrontabFromGenerator} class="input select-inline">
                  <option value={1}>1 Menit (* * * * *)</option>
                  <option value={2}>2 Menit (*/2 * * * *)</option>
                  <option value={5}>5 Menit (*/5 * * * *)</option>
                  <option value={10}>10 Menit (*/10 * * * *)</option>
                  <option value={15}>15 Menit (*/15 * * * *)</option>
                  <option value={20}>20 Menit (*/20 * * * *)</option>
                  <option value={30}>30 Menit (*/30 * * * *)</option>
                </select>
              </div>
            </div>

          {:else if activeTab === 'hourly'}
            <div class="tab-content">
              <span class="content-desc">Jalankan perintah sekali setiap satu jam pada menit yang ditentukan:</span>
              <div class="input-inline-wrap">
                <span>Pada menit ke:</span>
                <select bind:value={genHourlyMinute} onchange={updateCrontabFromGenerator} class="input select-inline">
                  <option value={0}>00 (Awal jam)</option>
                  <option value={5}>05</option>
                  <option value={15}>15</option>
                  <option value={30}>30 (Setengah jam)</option>
                  <option value={45}>45</option>
                </select>
              </div>
            </div>

          {:else if activeTab === 'daily'}
            <div class="tab-content">
              <div class="tz-switch-bar">
                <span class="tz-prompt">Basis Zona Waktu:</span>
                <div class="tabs tabs--pill">
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'client'}
                    onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                  </button>
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'server'}
                    onclick={() => { timezoneBasis = 'server'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-server-line"></i> {getServerTimezoneLabel()}
                  </button>
                </div>
              </div>

              <div class="input-inline-wrap">
                <span>Waktu Eksekusi Harian:</span>
                <div class="time-inputs">
                  <div class="field-item">
                    <label for="daily-hour">Jam (0-23)</label>
                    <select id="daily-hour" class="input" bind:value={genDailyHour} onchange={updateCrontabFromGenerator}>
                      {#each Array(24) as _, i}
                        <option value={i}>{String(i).padStart(2, '0')}:00 {i === 12 ? '(Siang)' : i === 0 ? '(Tengah Malam)' : ''}</option>
                      {/each}
                    </select>
                  </div>
                  <span class="time-sep-colon">:</span>
                  <div class="field-item">
                    <label for="daily-min">Menit (0-59)</label>
                    <select id="daily-min" class="input" bind:value={genDailyMinute} onchange={updateCrontabFromGenerator}>
                      {#each [0, 5, 10, 15, 20, 30, 40, 45, 50] as m}
                        <option value={m}>{String(m).padStart(2, '0')}</option>
                      {/each}
                    </select>
                  </div>
                </div>
              </div>
            </div>

          {:else if activeTab === 'weekly'}
            <div class="tab-content">
              <div class="tz-switch-bar">
                <span class="tz-prompt">Basis Zona Waktu:</span>
                <div class="tabs tabs--pill">
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'client'}
                    onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                  </button>
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'server'}
                    onclick={() => { timezoneBasis = 'server'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-server-line"></i> {getServerTimezoneLabel()}
                  </button>
                </div>
              </div>

              <div class="input-inline-wrap wrap-weekly">
                <div class="field-item">
                  <label for="weekly-day">Hari</label>
                  <select id="weekly-day" class="input" bind:value={genWeeklyDay} onchange={updateCrontabFromGenerator}>
                    {#each WEEKDAYS as w}
                      <option value={w.id}>{w.label}</option>
                    {/each}
                  </select>
                </div>
                <div class="field-item">
                  <label for="weekly-hour">Jam</label>
                  <select id="weekly-hour" class="input" bind:value={genWeeklyHour} onchange={updateCrontabFromGenerator}>
                    {#each Array(24) as _, i}
                      <option value={i}>{String(i).padStart(2, '0')}</option>
                    {/each}
                  </select>
                </div>
                <div class="field-item">
                  <label for="weekly-min">Menit</label>
                  <select id="weekly-min" class="input" bind:value={genWeeklyMinute} onchange={updateCrontabFromGenerator}>
                    {#each [0, 15, 30, 45] as m}
                      <option value={m}>{String(m).padStart(2, '0')}</option>
                    {/each}
                  </select>
                </div>
              </div>
            </div>

          {:else if activeTab === 'monthly'}
            <div class="tab-content">
              <div class="tz-switch-bar">
                <span class="tz-prompt">Basis Zona Waktu:</span>
                <div class="tabs tabs--pill">
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'client'}
                    onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                  </button>
                  <button
                    type="button"
                    class="tabs__tab"
                    class:is-active={timezoneBasis === 'server'}
                    onclick={() => { timezoneBasis = 'server'; updateCrontabFromGenerator(); }}
                  >
                    <i class="ri-server-line"></i> {getServerTimezoneLabel()}
                  </button>
                </div>
              </div>

              <div class="input-inline-wrap wrap-weekly">
                <div class="field-item">
                  <label for="monthly-date">Tanggal (1-31)</label>
                  <select id="monthly-date" class="input" bind:value={genMonthlyDate} onchange={updateCrontabFromGenerator}>
                    {#each Array(31) as _, i}
                      <option value={i + 1}>Tanggal {i + 1}</option>
                    {/each}
                  </select>
                </div>
                <div class="field-item">
                  <label for="monthly-hour">Jam</label>
                  <select id="monthly-hour" class="input" bind:value={genMonthlyHour} onchange={updateCrontabFromGenerator}>
                    {#each Array(24) as _, i}
                      <option value={i}>{String(i).padStart(2, '0')}</option>
                    {/each}
                  </select>
                </div>
                <div class="field-item">
                  <label for="monthly-min">Menit</label>
                  <select id="monthly-min" class="input" bind:value={genMonthlyMinute} onchange={updateCrontabFromGenerator}>
                    {#each [0, 15, 30, 45] as m}
                      <option value={m}>{String(m).padStart(2, '0')}</option>
                    {/each}
                  </select>
                </div>
              </div>
            </div>

          {:else if activeTab === 'custom'}
            <div class="tab-content">
              <div class="custom-tab-head">
                <span class="content-desc">Ketik ekspresi 5-kolom crontab (mendukung <code>*</code>, <code>*/N</code>, <code>A-B/S</code>, koma, <code>L</code>, <code>MON-FRI</code>):</span>
                <div class="quick-cron-examples">
                  <span class="quick-examples-label">Contoh Cepat:</span>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('*', '*', '*', '*', '*')}>Tiap Menit</button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('*/15', '*', '*', '*', '*')}>Tiap 15 Menit</button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '9-17/2', '*', '*', 'MON-FRI')}>Jam Kerja Tiap 2 Jam</button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '2', '*', '*', '*')}>Harian 02:00</button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '0', 'L', '*', '*')}>Akhir Bulan 00:00</button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '0', '1', '*', '*')}>Awal Bulan (Tgl 1)</button>
                </div>
              </div>
            </div>
          {/if}

          <!-- Grid 5 Kolom Crontab -->
          <div class="crontab-grid-box">
            <div class="cron-field">
              <span class="cron-field-label">Menit</span>
              <input type="text" bind:value={fieldMinute} disabled={activeTab !== 'custom'} class="cron-field-input mono" placeholder="*" />
              <span class="cron-field-sub">0-59</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Jam</span>
              <input type="text" bind:value={fieldHour} disabled={activeTab !== 'custom'} class="cron-field-input mono" placeholder="*" />
              <span class="cron-field-sub">0-23</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Tanggal</span>
              <input type="text" bind:value={fieldDay} disabled={activeTab !== 'custom'} class="cron-field-input mono" placeholder="*" />
              <span class="cron-field-sub">1-31 / L</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Bulan</span>
              <input type="text" bind:value={fieldMonth} disabled={activeTab !== 'custom'} class="cron-field-input mono" placeholder="*" />
              <span class="cron-field-sub">1-12 / JAN-DEC</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Hari</span>
              <input type="text" bind:value={fieldDow} disabled={activeTab !== 'custom'} class="cron-field-input mono" placeholder="*" />
              <span class="cron-field-sub">0-7 / SUN-SAT</span>
            </div>
          </div>

          <div class="explanation-box">
            <div class="explanation-row">
              <span class="badge badge--info mono">{fullCronExpression}</span>
              <span class="explanation-text">{scheduleHumanExplanation}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Preset Perintah -->
      <div class="field">
        <div class="label-row">
          <label class="field__label" for="form-preset">Preset Perintah Cepat (Opsional)</label>
          <span class="hint-inline">Template siap produksi untuk backup &amp; maintenance</span>
        </div>
        <select id="form-preset" class="input" value={selectedPreset} onchange={onPresetSelect}>
          <option value="custom">-- Kustom / Tulis Perintah Sendiri --</option>
          <option value="mysql_backup">Backup Database MySQL + Rotasi 7 Hari (mysqldump)</option>
          <option value="docker_prune">Bersihkan Image &amp; Cache Build Docker (docker prune)</option>
          <option value="laravel_artisan">Laravel Scheduler (php artisan schedule:run)</option>
          <option value="nginx_reload">Uji &amp; Reload Konfigurasi Nginx</option>
          <option value="curl_webhook">Ping Heartbeat / Webhook URL (curl -fsS)</option>
          <option value="clean_logs">Hapus Arsip Log Nginx Lama (&gt;14 Hari)</option>
        </select>
      </div>

      <!-- Textarea Perintah Shell -->
      <div class="field">
        <label class="field__label" for="form-cmd">Perintah Eksekusi Shell (Bash)</label>
        <textarea
          id="form-cmd"
          bind:value={formCommand}
          rows="3"
          class="input mono"
          placeholder="/usr/local/bin/backup.sh atau php artisan schedule:run"
          required
        ></textarea>
        <span class="security-warning">
          <i class="ri-shield-check-line"></i>
          Validasi server aktif: perintah destruktif (<code>shutdown</code>, <code>reboot</code>, <code>init 0/6</code>, <code>mkfs</code>, <code>rm -rf /</code>, <code>passwd</code>) diblokir otomatis.
        </span>
      </div>

      <!-- Direktori Kerja & Batas Waktu Eksekusi (Timeout) -->
      <div class="form-row-2col">
        <div class="field">
          <label class="field__label" for="form-workdir">Direktori Kerja (Opsional)</label>
          <input
            id="form-workdir"
            type="text"
            class="input mono"
            bind:value={formWorkingDir}
            placeholder="/home/fikpanel/web/domain.com"
          />
        </div>
        <div class="field">
          <label class="field__label" for="form-timeout">Batas Waktu / Timeout (Detik)</label>
          <input
            id="form-timeout"
            type="number"
            min="10"
            max="86400"
            class="input mono"
            bind:value={formTimeout}
          />
        </div>
      </div>

      <!-- Opsi Pencatatan Log & Notifikasi Kegagalan -->
      <div class="cron-options-grid">
        <label class="custom-chk">
          <input type="checkbox" bind:checked={formSaveLog} />
          <span>Simpan riwayat ke database (retensi otomatis 100 eksekusi terbaru)</span>
        </label>

        <label class="custom-chk">
          <input type="checkbox" bind:checked={formNotifyOnFail} />
          <span>Kirim notifikasi Webhook (Telegram/Discord) bila tugas gagal atau timeout</span>
        </label>

        <label class="custom-chk">
          <input type="checkbox" bind:checked={formSaveLogFile} />
          <span class="font-semibold text-accent">Simpan output eksekusi ke berkas log disk (.log, auto-rotasi 10 MB)</span>
        </label>
      </div>

      {#if formSaveLogFile}
        <div class="log-file-input-box">
          <div class="label-row">
            <label class="field__label" for="form-logfile">Jalur Berkas Log (.log)</label>
            <span class="hint-inline">Diizinkan: /var/log/, /var/cronlog/, /tmp/, /home/, /www/</span>
          </div>
          <input
            id="form-logfile"
            type="text"
            bind:value={formLogFile}
            placeholder="/var/cronlog/my-task.log atau /var/log/fikpanel/cron/task.log"
            class="input mono"
          />
          <div class="shortcut-pills">
            <span class="shortcut-label">Saran Cepat:</span>
            <button type="button" class="btn-shortcut" onclick={() => autoFillLogPath('/var/cronlog')}>/var/cronlog/...</button>
            <button type="button" class="btn-shortcut" onclick={() => autoFillLogPath('/var/log/cron')}>/var/log/cron/...</button>
            <button type="button" class="btn-shortcut" onclick={() => autoFillLogPath('/var/log/fikpanel/cron')}>/var/log/fikpanel/cron/...</button>
            <button type="button" class="btn-shortcut" onclick={() => autoFillLogPath('/tmp')}>/tmp/...</button>
          </div>
        </div>
      {/if}
    </form>

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => { showCreateModal = false; editingJob = null; }}>Batal</button>
      <button type="submit" form="cron-job-form" class="btn btn--primary" disabled={submitting}>
        {submitting ? 'Menyimpan...' : (editingJob ? 'Simpan Perubahan' : 'Simpan Tugas Cron')}
      </button>
    {/snippet}
  </Modal>

  <!-- Modal Riwayat Log -->
  <Modal
    open={showLogModal && !!selectedJob}
    title={selectedJob ? `Log Tugas: ${selectedJob.name}` : 'Log Tugas'}
    size="xl"
    onclose={() => (showLogModal = false)}
  >
    {#if selectedJob}
      <div class="modal-sub-bar">
        <span class="modal-sub mono text-xs">{selectedJob.command}</span>
        {#if selectedJob.is_running || selectedJob.last_status === 'running'}
          <span class="badge badge--warning"><i class="ri-loader-4-line spin"></i> Sedang Berjalan</span>
        {/if}
      </div>

      {#if selectedJob.log_file}
        <div class="tabs tabs--underline mb-4">
          <button
            type="button"
            class="tabs__tab"
            class:is-active={activeLogTab === 'file'}
            onclick={() => { activeLogTab = 'file'; if (selectedJob) loadFileLog(selectedJob.id); }}
          >
            <i class="ri-file-text-line"></i> Berkas Log (.log)
          </button>
          <button
            type="button"
            class="tabs__tab"
            class:is-active={activeLogTab === 'db'}
            onclick={() => { activeLogTab = 'db'; if (selectedJob) loadDbLogs(selectedJob.id); }}
          >
            <i class="ri-history-line"></i> Riwayat Database ({logs.length})
          </button>
        </div>
      {/if}

      {#if selectedJob.log_file && activeLogTab === 'file'}
        <div class="file-log-toolbar">
          <div class="file-path-info mono text-xs">
            <i class="ri-folder-open-line text-accent"></i>
            <span>{fileLogPath || selectedJob.log_file}</span>
          </div>
          <div class="toolbar-actions">
            <button
              type="button"
              class="btn btn--sm btn--secondary"
              onclick={() => { if (selectedJob) loadFileLog(selectedJob.id); }}
              disabled={loadingFileLog}
            >
              <i class="ri-refresh-line {loadingFileLog ? 'spin' : ''}"></i> Segarkan
            </button>
            <button
              type="button"
              class="btn btn--sm btn--danger"
              onclick={() => { if (selectedJob) clearFileLog(selectedJob.id); }}
            >
              <i class="ri-delete-bin-line"></i> Kosongkan Log
            </button>
          </div>
        </div>

        <div class="file-log-viewer">
          {#if loadingFileLog}
            <div class="state state--loading">
              <i class="ri-loader-4-line spin text-lg"></i> Membaca berkas log dari disk...
            </div>
          {:else}
            <pre class="file-log-pre mono">{fileLogContent}</pre>
          {/if}
        </div>
      {:else}
        <div class="file-log-toolbar">
          <div class="file-path-info text-xs">
            <i class="ri-database-2-line text-accent"></i>
            <span>Menampilkan maks. 50 riwayat terbaru (retensi otomatis 100/tugas)</span>
          </div>
          <div class="toolbar-actions">
            <button
              type="button"
              class="btn btn--sm btn--secondary"
              onclick={() => { if (selectedJob) loadDbLogs(selectedJob.id); }}
              disabled={loadingLogs}
            >
              <i class="ri-refresh-line {loadingLogs ? 'spin' : ''}"></i> Segarkan
            </button>
            {#if logs.length > 0}
              <button
                type="button"
                class="btn btn--sm btn--danger"
                onclick={() => { if (selectedJob) clearDbLogs(selectedJob.id); }}
              >
                <i class="ri-delete-bin-line"></i> Hapus Riwayat
              </button>
            {/if}
          </div>
        </div>

        <div class="log-container">
          {#if loadingLogs}
            <div class="state state--loading">
              <i class="ri-loader-4-line spin text-lg"></i> Memuat log eksekusi...
            </div>
          {:else if logs.length === 0}
            <div class="state state--empty">
              Belum ada catatan eksekusi untuk tugas ini. Klik ikon play pada tabel untuk menjalankan sekarang.
            </div>
          {:else}
            <div class="log-list">
              {#each logs as log (log.id)}
                <div class="log-item">
                  <div class="log-meta">
                    <span
                      class="badge {log.status === 'success'
                        ? 'badge--success'
                        : log.status === 'stopped' || log.status === 'interrupted'
                          ? 'badge--warning'
                          : 'badge--danger'}"
                    >
                      {#if log.status === 'success'}
                        <i class="ri-checkbox-circle-fill"></i>
                      {:else if log.status === 'timeout'}
                        <i class="ri-timer-flash-line"></i>
                      {:else if log.status === 'stopped'}
                        <i class="ri-stop-circle-line"></i>
                      {:else}
                        <i class="ri-close-circle-fill"></i>
                      {/if}
                      {log.status.toUpperCase()} (Exit Code: {log.exit_code})
                    </span>
                    <span class="text-muted">{formatTimestamp(log.started_at)} • Durasi {log.duration_ms}ms</span>
                  </div>
                  {#if log.output}
                    <pre class="log-output mono">{log.output}</pre>
                  {:else}
                    <span class="text-subtle italic text-xs">Perintah selesai tanpa output teks standar.</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    {/if}

    {#snippet footer()}
      <button type="button" class="btn btn--ghost" onclick={() => (showLogModal = false)}>Tutup</button>
    {/snippet}
  </Modal>
</div>
