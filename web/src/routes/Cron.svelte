<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { CronJob, CronLog, ServerTimeInfo } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';
  import Modal from '../lib/components/ui/Modal.svelte';
  import '../styles/pages/cron.css';

  let jobs = $state<CronJob[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);
  let editingJob = $state<CronJob | null>(null);
  let showLogModal = $state(false);
  let selectedJob = $state<CronJob | null>(null);
  let logs = $state<CronLog[]>([]);
  let loadingLogs = $state(false);

  // Server Time & Timezone State
  let serverTimeInfo = $state<ServerTimeInfo | null>(null);
  let serverNow = $state<Date>(new Date());
  let clientNow = $state<Date>(new Date());
  let clockInterval: any = null;

  // Form State
  let formName = $state('');
  let formCommand = $state('');
  let formTimeout = $state(3600);
  let formSaveLog = $state(true);
  let formSaveLogFile = $state(false);
  let formLogFile = $state('');
  let submitting = $state(false);

  // Log Modal State (Dukungan Berkas Log Dinamis & Database History)
  let activeLogTab = $state<'file' | 'db'>('file');
  let fileLogContent = $state('');
  let fileLogPath = $state('');
  let loadingFileLog = $state(false);

  // Schedule Generator State (HestiaCP + aaPanel Style)
  type TabType = 'minutes' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom';
  let activeTab = $state<TabType>('daily');
  let timezoneBasis = $state<'client' | 'server'>('client'); // 'client' (misal WIB UTC+7) vs 'server' (UTC)

  // Generator inputs
  let genMinuteInterval = $state(15);
  let genHourlyMinute = $state(0);
  let genDailyHour = $state(12);
  let genDailyMinute = $state(0);
  let genWeeklyDay = $state(1); // 1 = Senin, 7 = Minggu
  let genWeeklyHour = $state(2);
  let genWeeklyMinute = $state(0);
  let genMonthlyDate = $state(1);
  let genMonthlyHour = $state(3);
  let genMonthlyMinute = $state(0);

  // 5-Field Standard Crontab Values
  let fieldMinute = $state('0');
  let fieldHour = $state('5');
  let fieldDay = $state('*');
  let fieldMonth = $state('*');
  let fieldDow = $state('*');

  // Command Preset
  let selectedPreset = $state('custom');

  const PRESETS: Record<string, string> = {
    custom: '',
    mysql_backup: "mkdir -p /backup/mysql && mysqldump --all-databases | gzip > /backup/mysql/db_$(date +%F_%H%M).sql.gz",
    docker_prune: "docker system prune -af --volumes",
    laravel_artisan: "php /var/www/html/artisan schedule:run >> /dev/null 2>&1",
    nginx_reload: "systemctl reload nginx",
    curl_webhook: "curl -fsS --retry 3 https://example.com/api/heartbeat-cron > /dev/null",
    clean_logs: "find /var/log/nginx/ -type f -name '*.log.*' -mtime +7 -delete"
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

  // Kalkulasi selisih jam (WIB / Client Time vs Server Time UTC)
  function getTimezoneDiffHours(): number {
    const clientOffsetSec = -(new Date().getTimezoneOffset() * 60);
    const serverOffsetSec = serverTimeInfo ? serverTimeInfo.offset_sec : 0;
    return Math.round((clientOffsetSec - serverOffsetSec) / 3600);
  }

  function getClientTimezoneLabel(): string {
    const offsetMin = -new Date().getTimezoneOffset();
    const sign = offsetMin >= 0 ? '+' : '-';
    const hrs = Math.floor(Math.abs(offsetMin) / 60);
    const isWIB = offsetMin === 420;
    return isWIB ? 'WIB (UTC+7)' : `Waktu Lokal (UTC${sign}${hrs})`;
  }

  function getServerTimezoneLabel(): string {
    if (!serverTimeInfo) return 'Waktu Server (UTC)';
    const offsetSec = serverTimeInfo.offset_sec;
    const hrs = Math.floor(offsetSec / 3600);
    const sign = hrs >= 0 ? '+' : '';
    return `${serverTimeInfo.timezone_name || 'UTC'} (UTC${sign}${hrs})`;
  }

  // Sinkronkan 5 kolom crontab dari tab generator
  function updateCrontabFromGenerator() {
    const diff = getTimezoneDiffHours();

    if (activeTab === 'minutes') {
      fieldMinute = `*/${genMinuteInterval}`;
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
        // Konversi dari WIB ke waktu server
        targetHour = (genDailyHour - diff + 24) % 24;
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
      if (timezoneBasis === 'client') {
        targetHour = (genMonthlyHour - diff + 24) % 24;
      }
      fieldMinute = String(genMonthlyMinute);
      fieldHour = String(targetHour);
      fieldDay = String(genMonthlyDate);
      fieldMonth = '*';
      fieldDow = '*';
    }
  }

  // Kamus nama bulan dan nama hari untuk interpretasi manusia
  const MONTH_NAMES: Record<string, string> = {
    '1': 'Januari', '2': 'Februari', '3': 'Maret', '4': 'April',
    '5': 'Mei', '6': 'Juni', '7': 'Juli', '8': 'Agustus',
    '9': 'September', '10': 'Oktober', '11': 'November', '12': 'Desember',
  };

  const DOW_NAMES: Record<string, string> = {
    '0': 'Minggu', '1': 'Senin', '2': 'Selasa', '3': 'Rabu',
    '4': 'Kamis', '5': 'Jumat', '6': 'Sabtu', '7': 'Minggu',
  };

  // Fungsi penerjemah ekspresi 5-kolom crontab ke kalimat bahasa Indonesia yang sangat gamblang
  function explainCustomCron(
    min: string,
    hr: string,
    dom: string,
    mon: string,
    dow: string,
    diffHours: number,
    clientTz: string
  ): string {
    const m = (min || '*').trim();
    const h = (hr || '*').trim();
    const d = (dom || '*').trim();
    const mo = (mon || '*').trim();
    const dw = (dow || '*').trim();

    // 1. Setiap menit
    if (m === '*' && h === '*' && d === '*' && mo === '*' && dw === '*') {
      return 'Dijalankan setiap menit secara terus-menerus (60 kali per jam).';
    }

    // 2. Interval menit (step), misal */15 * * * * atau */5 * * * *
    if (m.startsWith('*/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const step = m.replace('*/', '');
      return `Dijalankan setiap ${step} menit sekali secara berulang.`;
    }

    // 3. Setiap jam pada menit tertentu, misal 0 * * * * atau 30 * * * *
    if (!m.includes('*') && !m.includes('/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const mPad = m.padStart(2, '0');
      return `Dijalankan setiap jam pada menit ke-${mPad} (${mPad === '00' ? 'tepat di awal pergantian jam' : `menit ke-${m}`}).`;
    }

    // 4. Interval jam (step), misal 0 */2 * * *
    if (h.startsWith('*/') && d === '*' && mo === '*' && dw === '*') {
      const stepH = h.replace('*/', '');
      const mPad = m === '*' ? 'setiap menit' : `menit ke-${m.padStart(2, '0')}`;
      return `Dijalankan setiap ${stepH} jam sekali pada ${mPad}.`;
    }

    // Format label jam dan menit
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

      // Tambahkan konversi waktu pengguna (WIB) jika server berada di zona waktu berbeda (UTC)
      if (diffHours !== 0) {
        const clientH = (hNum + diffHours + 24) % 24;
        const clientHPad = String(clientH).padStart(2, '0');
        timeHint = ` (waktu server, setara ${clientHPad}:${mPad} ${clientTz})`;
      }
    } else if (h !== '*') {
      timeLabel = `jam ${h} (menit ${m})`;
    } else {
      timeLabel = `menit ${m}`;
    }

    // 5. Setiap hari pada jam tertentu, misal 0 12 * * * atau 0 5 * * *
    if (d === '*' && mo === '*' && dw === '*') {
      return `Dijalankan setiap hari pada ${timeLabel}${timeHint}.`;
    }

    // 6. Setiap bulan pada tanggal tertentu, misal 0 12 12 * * atau 0 0 12 * *
    if (d !== '*' && mo === '*' && dw === '*') {
      let tglDesc = `tanggal ${d}`;
      if (d === '1') tglDesc = 'tanggal 1 (awal bulan)';
      else if (d === 'L') tglDesc = 'hari terakhir setiap bulan';
      return `Dijalankan setiap bulan pada ${tglDesc} ${timeLabel}${timeHint}.`;
    }

    // 7. Setiap minggu pada hari tertentu, misal 0 12 * * 1 atau 0 5 * * 1-5
    if (d === '*' && mo === '*' && dw !== '*') {
      let hariDesc = '';
      if (dw === '1-5') hariDesc = 'hari kerja (Senin s/d Jumat)';
      else if (dw === '0,6' || dw === '6,7') hariDesc = 'akhir pekan (Sabtu & Minggu)';
      else {
        const parts = dw.split(',').map((x) => DOW_NAMES[x.trim()] || `hari ${x}`);
        hariDesc = `hari ${parts.join(', ')}`;
      }
      return `Dijalankan setiap ${hariDesc} pada ${timeLabel}${timeHint}.`;
    }

    // 8. Tahunan / Bulan spesifik, misal 0 0 1 1 * atau 0 12 12 8 *
    if (d !== '*' && mo !== '*' && dw === '*') {
      const namaBulan = MONTH_NAMES[mo] || `bulan ${mo}`;
      return `Dijalankan setiap tahun pada tanggal ${d} ${namaBulan} ${timeLabel}${timeHint}.`;
    }

    // 9. Kombinasi Tanggal dan Hari dalam seminggu
    if (d !== '*' && dw !== '*') {
      const parts = dw.split(',').map((x) => DOW_NAMES[x.trim()] || `hari ${x}`);
      return `Dijalankan pada tanggal ${d} dan setiap hari ${parts.join(', ')} pada ${timeLabel}${timeHint}.`;
    }

    // 10. Fallback pola crontab lainnya
    return `Dijalankan berdasarkan pola Crontab: Menit (${m}), Jam (${h}), Tanggal (${d}), Bulan (${mo}), Hari (${dw})${timeHint}.`;
  }

  // Isi preset cepat untuk ekspresi crontab kustom
  function applyCustomPreset(m: string, h: string, d: string, mo: string, dw: string) {
    fieldMinute = m;
    fieldHour = h;
    fieldDay = d;
    fieldMonth = mo;
    fieldDow = dw;
  }

  // Teks interpretasi jadwal yang sangat gamblang
  let scheduleHumanExplanation = $derived.by(() => {
    const diff = getTimezoneDiffHours();
    const clientTz = getClientTimezoneLabel();
    const serverTz = getServerTimezoneLabel();

    if (activeTab === 'minutes') {
      return `Tugas akan dijalankan setiap ${genMinuteInterval} menit sekali secara berulang.`;
    }
    if (activeTab === 'hourly') {
      return `Tugas akan dijalankan setiap jam pada menit ke-${String(genHourlyMinute).padStart(2, '0')}.`;
    }
    if (activeTab === 'daily') {
      if (timezoneBasis === 'client') {
        const srvHour = (genDailyHour - diff + 24) % 24;
        return `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz} (otomatis dikonversi ke ${String(srvHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz} waktu server).`;
      } else {
        const cltHour = (genDailyHour + diff + 24) % 24;
        return `Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz} waktu server (setara pukul ${String(cltHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz} Anda).`;
      }
    }
    if (activeTab === 'weekly') {
      const dayName = WEEKDAYS.find((w) => w.id === genWeeklyDay)?.label || 'Hari';
      if (timezoneBasis === 'client') {
        return `Berjalan setiap hari ${dayName} pukul ${String(genWeeklyHour).padStart(2, '0')}:${String(genWeeklyMinute).padStart(2, '0')} ${clientTz}.`;
      } else {
        return `Berjalan setiap ${dayName} pukul ${String(genWeeklyHour).padStart(2, '0')}:${String(genWeeklyMinute).padStart(2, '0')} ${serverTz}.`;
      }
    }
    if (activeTab === 'monthly') {
      return `Berjalan setiap tanggal ${genMonthlyDate} pukul ${String(genMonthlyHour).padStart(2, '0')}:${String(genMonthlyMinute).padStart(2, '0')} (${timezoneBasis === 'client' ? clientTz : serverTz}).`;
    }
    return explainCustomCron(fieldMinute, fieldHour, fieldDay, fieldMonth, fieldDow, diff, clientTz);
  });

  // String ekspresi crontab final
  let fullCronExpression = $derived(`${fieldMinute.trim()} ${fieldHour.trim()} ${fieldDay.trim()} ${fieldMonth.trim()} ${fieldDow.trim()}`);

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

  async function loadJobs() {
    try {
      loading = true;
      const res = await apiFetch<{ ok: boolean; jobs: CronJob[]; server_time?: ServerTimeInfo }>('/api/cron/jobs');
      if (res && res.jobs) {
        jobs = res.jobs;
      }
      if (res && res.server_time) {
        serverTimeInfo = res.server_time;
      }
    } catch (err: any) {
      notify(err.message || 'Gagal memuat daftar cron job', 'error');
    } finally {
      loading = false;
    }
  }

  // Tangani simpan tugas cron (tambah baru atau pembaruan konfigurasi)
  async function handleSubmitJob() {
    if (!formName.trim() || !formCommand.trim()) {
      notify('Nama tugas dan perintah eksekusi wajib diisi', 'warning');
      return;
    }

    try {
      submitting = true;
      const payload = {
        name: formName.trim(),
        schedule_type: activeTab === 'custom' ? 'custom' : activeTab,
        spec: fullCronExpression,
        command: formCommand.trim(),
        timeout_sec: formTimeout,
        save_log: formSaveLog,
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
      await loadJobs();
    } catch (err: any) {
      notify(err.message || (editingJob ? 'Gagal memperbarui tugas cron' : 'Gagal menambahkan tugas cron'), 'error');
    } finally {
      submitting = false;
    }
  }

  // Buka modal untuk mengedit tugas cron yang dipilih
  function openEditModal(job: CronJob) {
    editingJob = job;
    formName = job.name;
    formCommand = job.command;
    formTimeout = job.timeout_sec || 3600;
    formSaveLog = job.save_log;
    formSaveLogFile = !!job.log_file;
    formLogFile = job.log_file || '';

    // Muat 5 kolom crontab dari spec
    const parts = job.spec.trim().split(/\s+/);
    if (parts.length === 5) {
      fieldMinute = parts[0];
      fieldHour = parts[1];
      fieldDay = parts[2];
      fieldMonth = parts[3];
      fieldDow = parts[4];
    }

    // Tentukan tab aktif berdasarkan tipe jadwal tersimpan
    if (job.schedule_type && ['minutes', 'hourly', 'daily', 'weekly', 'monthly', 'custom'].includes(job.schedule_type)) {
      activeTab = job.schedule_type as TabType;
    } else {
      activeTab = 'custom';
    }

    // Gunakan basis server saat mengedit agar nilai tidak bergeser secara tidak sengaja
    timezoneBasis = 'server';
    if (parts.length === 5) {
      if (activeTab === 'minutes' && parts[0].startsWith('*/')) {
        genMinuteInterval = parseInt(parts[0].replace('*/', ''), 10) || 15;
      } else if (activeTab === 'hourly') {
        genHourlyMinute = parseInt(parts[0], 10) || 0;
      } else if (activeTab === 'daily') {
        genDailyHour = parseInt(parts[1], 10) || 0;
        genDailyMinute = parseInt(parts[0], 10) || 0;
      } else if (activeTab === 'weekly') {
        genWeeklyHour = parseInt(parts[1], 10) || 0;
        genWeeklyMinute = parseInt(parts[0], 10) || 0;
        genWeeklyDay = parseInt(parts[4], 10) || 1;
      } else if (activeTab === 'monthly') {
        genMonthlyDate = parseInt(parts[2], 10) || 1;
        genMonthlyHour = parseInt(parts[1], 10) || 0;
        genMonthlyMinute = parseInt(parts[0], 10) || 0;
      }
    }

    // Deteksi kesesuaian preset perintah
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
    selectedPreset = 'custom';
    formTimeout = 3600;
    formSaveLog = true;
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
      await apiFetch(`/api/cron/jobs/${job.id}/toggle`, {
        method: 'POST',
        body: JSON.stringify({ enabled: !job.enabled }),
      });
      job.enabled = !job.enabled;
      notify(`Tugas ${job.enabled ? 'diaktifkan' : 'dinonaktifkan'}`, 'info');
    } catch (err: any) {
      notify(err.message || 'Gagal mengubah status', 'error');
    }
  }

  async function runJobNow(job: CronJob) {
    try {
      notify(`Menjalankan tugas "${job.name}"...`, 'info');
      const res = await apiFetch<{ ok: boolean; log: CronLog }>(`/api/cron/jobs/${job.id}/run`, {
        method: 'POST',
      });
      if (res && res.log) {
        job.last_status = res.log.status;
        job.last_run_at = res.log.finished_at;
        if (res.log.status === 'success') {
          notify(`Tugas "${job.name}" selesai dijalankan (Kode: ${res.log.exit_code})`, 'success');
        } else {
          notify(`Tugas "${job.name}" selesai dengan status ${res.log.status}`, 'warning');
        }
      }
    } catch (err: any) {
      notify(err.message || 'Gagal mengeksekusi tugas', 'error');
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
      const res = await apiFetch<{ ok: boolean; path: string; content: string; message?: string }>(`/api/cron/jobs/${jobId}/file-log`);
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

  // Menerjemahkan ekspresi spec crontab ke teks bahasa Indonesia yang jelas
  function translateSpec(spec: string): string {
    const fields = spec.trim().split(/\s+/);
    if (fields.length === 5) {
      const [m, h, dom, mon, dow] = fields;
      const diff = getTimezoneDiffHours();
      const clientTz = getClientTimezoneLabel();
      return explainCustomCron(m, h, dom, mon, dow, diff, clientTz);
    }
    return spec;
  }

  onMount(() => {
    loadJobs();
    updateCrontabFromGenerator();

    clockInterval = setInterval(() => {
      clientNow = new Date();
      if (serverTimeInfo) {
        // Offset selisih client vs server
        const clientOffset = -(new Date().getTimezoneOffset() * 60);
        const diffMs = (serverTimeInfo.offset_sec - clientOffset) * 1000;
        serverNow = new Date(Date.now() + diffMs);
      } else {
        serverNow = new Date();
      }
    }, 1000);
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
        Jadwalkan eksekusi skrip background, backup otomatis, dan maintenance server dengan sinkronisasi zona waktu WIB dan server UTC.
      </p>
    </div>
    <div class="page-header__actions">
      <button class="btn btn--ghost" onclick={loadJobs} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn--primary" onclick={() => { resetForm(); showCreateModal = true; }}>
        <i class="ri-add-line"></i> Tambah Tugas
      </button>
    </div>
  </header>

  <!-- Tabel Daftar Tugas Cron -->
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
          {:else if jobs.length === 0}
            <tr>
              <td colspan="6">
                <div class="state state--empty">
                  Belum ada tugas cron yang didaftarkan. Klik "Tambah Tugas" untuk menjadwalkan pekerjaan otomatis.
                </div>
              </td>
            </tr>
          {:else}
            {#each jobs as job (job.id)}
              <tr>
                <td>
                  <div class="job-name-col">
                    <span class="dot {job.enabled ? 'dot--success' : ''}" title={job.enabled ? 'Aktif' : 'Dijeda'}></span>
                    <div>
                      <span class="font-semibold block">{job.name}</span>
                      <span class="text-subtle text-xs mono">ID #{job.id}</span>
                    </div>
                  </div>
                </td>
                <td>
                  <div class="spec-cell">
                    <span class="badge-spec mono">{job.spec}</span>
                    <span class="spec-human">{translateSpec(job.spec)}</span>
                  </div>
                </td>
                <td class="col-cmd mono" title={job.command}>
                  <div class="cmd-text">{job.command}</div>
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
                </td>
                <td>
                  {#if job.last_status === 'success'}
                    <span class="badge badge--success"><i class="ri-checkbox-circle-fill"></i> Berhasil</span>
                  {:else if job.last_status === 'failed'}
                    <span class="badge badge--danger"><i class="ri-close-circle-fill"></i> Gagal</span>
                  {:else if job.last_status === 'running'}
                    <span class="badge badge--warning"><i class="ri-loader-4-line spin"></i> Berjalan</span>
                  {:else}
                    <span class="text-muted">-</span>
                  {/if}
                </td>
                <td class="text-muted">
                  {formatTimestamp(job.last_run_at)}
                </td>
                <td class="text-right actions-col">
                  <button class="btn btn--icon btn--sm btn--ghost" title="Jalankan Sekarang" onclick={() => runJobNow(job)}>
                    <i class="ri-play-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--ghost" title={job.enabled ? 'Jeda Tugas' : 'Aktifkan'} onclick={() => toggleJob(job)}>
                    <i class={job.enabled ? 'ri-pause-line' : 'ri-play-circle-line'}></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--ghost" title="Edit Tugas" onclick={() => openEditModal(job)}>
                    <i class="ri-edit-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--ghost" title="Riwayat Log" onclick={() => viewLogs(job)}>
                    <i class="ri-file-list-3-line"></i>
                  </button>
                  <button class="btn btn--icon btn--sm btn--ghost btn--danger" title="Hapus Tugas" onclick={() => deleteJob(job)}>
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

  <!-- Modal Tambah / Edit Tugas Cron -->
  <Modal
    open={showCreateModal}
    title={editingJob ? 'Edit Tugas Terjadwal (Cron Job)' : 'Tambah Tugas Terjadwal (Cron Job)'}
    size="xl"
    onclose={() => { showCreateModal = false; editingJob = null; }}
  >
    <div class="modal-sub-bar">
      <span class="modal-sub">
        {editingJob ? 'Perbarui jadwal eksekusi, perintah shell, atau konfigurasi pencatatan log.' : 'Konfigurasi otomatisasi skrip server dengan konversi zona waktu real-time.'}
      </span>
      <div class="server-clock-tag">
        <i class="ri-time-line"></i>
        <span>System time: <strong>{formatDate(serverNow)}, {formatTime(serverNow)}</strong></span>
      </div>
    </div>

    <form id="cron-job-form" onsubmit={(e) => { e.preventDefault(); handleSubmitJob(); }} class="form-body">
      <!-- Nama Tugas -->
      <div class="field">
        <label class="field__label" for="form-name">Nama Tugas Terjadwal</label>
        <input id="form-name" class="input" type="text" bind:value={formName} placeholder="Misal: Backup Database Tiap Jam 12 WIB" required />
      </div>

      <!-- Kontainer Pembuat Jadwal Visual -->
      <div class="schedule-builder-card">
        <div class="builder-head">
          <span class="builder-label"><i class="ri-calendar-schedule-line"></i> Penjadwalan Eksekusi</span>
          <!-- Menu Tab Jadwal -->
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
          <!-- TAB: MINUTES -->
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

          <!-- TAB: HOURLY -->
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

          <!-- TAB: DAILY (Konversi WIB vs UTC) -->
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

          <!-- TAB: WEEKLY -->
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

          <!-- TAB: MONTHLY -->
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

          <!-- TAB: CUSTOM (Input Langsung 5 Kolom) -->
          {:else if activeTab === 'custom'}
            <div class="tab-content">
              <div class="custom-tab-head">
                <span class="content-desc">Ketik langsung nilai ekspresi 5 kolom crontab (* atau angka/rentang/step):</span>
                <div class="quick-cron-examples">
                  <span class="quick-examples-label">Contoh Cepat:</span>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '12', '*', '*', '*')}>
                    Setiap Hari Jam 12
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '5', '*', '*', '*')}>
                    Setiap Hari Jam 05
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '12', '12', '*', '*')}>
                    Tiap Bulan Tgl 12 Jam 12
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '0', '12', '*', '*')}>
                    Tiap Bulan Tgl 12 Jam 00
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '0', '1', '*', '*')}>
                    Tiap Awal Bulan (Tgl 1)
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('*/15', '*', '*', '*', '*')}>
                    Setiap 15 Menit
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '*', '*', '*', '*')}>
                    Setiap Jam
                  </button>
                  <button type="button" class="example-chip" onclick={() => applyCustomPreset('0', '9', '*', '*', '1')}>
                    Setiap Senin Jam 09
                  </button>
                </div>
              </div>
            </div>
          {/if}

          <!-- Kotak Visual 5 Kolom Crontab -->
          <div class="crontab-grid-box">
            <div class="cron-field">
              <span class="cron-field-label">Menit (Minute)</span>
              <input
                type="text"
                bind:value={fieldMinute}
                disabled={activeTab !== 'custom'}
                class="cron-field-input mono"
                placeholder="*"
              />
              <span class="cron-field-sub">0-59</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Jam (Hour)</span>
              <input
                type="text"
                bind:value={fieldHour}
                disabled={activeTab !== 'custom'}
                class="cron-field-input mono"
                placeholder="*"
              />
              <span class="cron-field-sub">0-23</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Tanggal (Day)</span>
              <input
                type="text"
                bind:value={fieldDay}
                disabled={activeTab !== 'custom'}
                class="cron-field-input mono"
                placeholder="*"
              />
              <span class="cron-field-sub">1-31</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Bulan (Month)</span>
              <input
                type="text"
                bind:value={fieldMonth}
                disabled={activeTab !== 'custom'}
                class="cron-field-input mono"
                placeholder="*"
              />
              <span class="cron-field-sub">1-12</span>
            </div>
            <div class="cron-field">
              <span class="cron-field-label">Hari (Day of Week)</span>
              <input
                type="text"
                bind:value={fieldDow}
                disabled={activeTab !== 'custom'}
                class="cron-field-input mono"
                placeholder="*"
              />
              <span class="cron-field-sub">0-7 (0=Min)</span>
            </div>
          </div>

          <!-- Kartu Penjelasan Real-time -->
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
          <label class="field__label" for="form-preset">Pilih Preset Perintah Cepat (Opsional)</label>
          <span class="hint-inline">Gunakan template bawaan untuk backup & maintenance</span>
        </div>
        <select id="form-preset" class="input" value={selectedPreset} onchange={onPresetSelect}>
          <option value="custom">-- Kustom / Tulis Perintah Sendiri --</option>
          <option value="mysql_backup">Backup Database MySQL ke /backup/mysql/ (mysqldump)</option>
          <option value="docker_prune">Bersihkan Container & Image Docker Tak Terpakai (docker system prune)</option>
          <option value="laravel_artisan">Laravel Scheduler (php artisan schedule:run)</option>
          <option value="nginx_reload">Reload Konfigurasi Web Server (systemctl reload nginx)</option>
          <option value="curl_webhook">Ping Webhook / Heartbeat URL (curl -fsS)</option>
          <option value="clean_logs">Hapus Log Nginx Lama (>7 Hari)</option>
        </select>
      </div>

      <!-- Textarea Perintah Shell -->
      <div class="field">
        <label class="field__label" for="form-cmd">Perintah Eksekusi Shell (Linux Command)</label>
        <textarea
          id="form-cmd"
          bind:value={formCommand}
          rows="3"
          class="input mono"
          placeholder="/usr/local/bin/backup.sh atau docker system prune -af"
          required
        ></textarea>
        <span class="security-warning">
          <i class="ri-shield-check-line"></i>
          Perintah berbahaya seperti <code>shutdown</code>, <code>init 0</code>, <code>mkfs</code>, atau <code>passwd</code> dilarang dijalankan demi kestabilan server.
        </span>
      </div>

      <!-- Checkbox Simpan Log Database -->
      <div class="checkbox-row">
        <label class="custom-chk">
          <input type="checkbox" bind:checked={formSaveLog} />
          <span>Simpan riwayat eksekusi ke database (Default history)</span>
        </label>
      </div>

      <!-- Checkbox Simpan Log ke Berkas Fisik (.log) -->
      <div class="checkbox-row">
        <label class="custom-chk">
          <input type="checkbox" bind:checked={formSaveLogFile} />
          <span class="font-semibold text-accent">Simpan output eksekusi ke berkas log (.log)</span>
        </label>
      </div>

      {#if formSaveLogFile}
        <div class="log-file-input-box">
          <div class="label-row">
            <label class="field__label" for="form-logfile">Jalur Direktori / Berkas Log (.log)</label>
            <span class="hint-inline">Folder dan file otomatis dibuat jika belum ada</span>
          </div>
          <div class="input-with-shortcuts">
            <input
              id="form-logfile"
              type="text"
              bind:value={formLogFile}
              placeholder="/var/cronlog/my-task.log atau /var/log/cron/task.log"
              class="input mono"
            />
          </div>
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
      </div>

      <!-- Pengalih Tab jika ada log_file -->
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

      <!-- Tab 1: Berkas Log Fisik (.log) -->
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
        <!-- Tab 2: Riwayat Database -->
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
                    <span class="badge {log.status === 'success' ? 'badge--success' : 'badge--danger'}">
                      {#if log.status === 'success'}
                        <i class="ri-checkbox-circle-fill"></i>
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
