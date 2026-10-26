<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { apiFetch } from '../lib/api';
  import type { CronJob, CronLog, ServerTimeInfo } from '../lib/types';
  import { notify } from '../lib/stores/toast.svelte';
  import { confirmDialog } from '../lib/stores/confirm.svelte';

  let jobs = $state<CronJob[]>([]);
  let loading = $state(true);
  let showCreateModal = $state(false);
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
  let submitting = $state(false);

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
      return '⚡ Dijalankan setiap menit secara terus-menerus (60 kali per jam).';
    }

    // 2. Interval menit (step), misal */15 * * * * atau */5 * * * *
    if (m.startsWith('*/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const step = m.replace('*/', '');
      return `⚡ Dijalankan setiap ${step} menit sekali secara berulang.`;
    }

    // 3. Setiap jam pada menit tertentu, misal 0 * * * * atau 30 * * * *
    if (!m.includes('*') && !m.includes('/') && h === '*' && d === '*' && mo === '*' && dw === '*') {
      const mPad = m.padStart(2, '0');
      return `⏱️ Dijalankan setiap jam pada menit ke-${mPad} (${mPad === '00' ? 'tepat di awal pergantian jam' : `menit ke-${m}`}).`;
    }

    // 4. Interval jam (step), misal 0 */2 * * *
    if (h.startsWith('*/') && d === '*' && mo === '*' && dw === '*') {
      const stepH = h.replace('*/', '');
      const mPad = m === '*' ? 'setiap menit' : `menit ke-${m.padStart(2, '0')}`;
      return `⏱️ Dijalankan setiap ${stepH} jam sekali pada ${mPad}.`;
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
      return `🕒 Dijalankan setiap hari pada ${timeLabel}${timeHint}.`;
    }

    // 6. Setiap bulan pada tanggal tertentu, misal 0 12 12 * * atau 0 0 12 * *
    if (d !== '*' && mo === '*' && dw === '*') {
      let tglDesc = `tanggal ${d}`;
      if (d === '1') tglDesc = 'tanggal 1 (awal bulan)';
      else if (d === 'L') tglDesc = 'hari terakhir setiap bulan';
      return `📆 Dijalankan setiap bulan pada ${tglDesc} ${timeLabel}${timeHint}.`;
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
      return `📅 Dijalankan setiap ${hariDesc} pada ${timeLabel}${timeHint}.`;
    }

    // 8. Tahunan / Bulan spesifik, misal 0 0 1 1 * atau 0 12 12 8 *
    if (d !== '*' && mo !== '*' && dw === '*') {
      const namaBulan = MONTH_NAMES[mo] || `bulan ${mo}`;
      return `🗓️ Dijalankan setiap tahun pada tanggal ${d} ${namaBulan} ${timeLabel}${timeHint}.`;
    }

    // 9. Kombinasi Tanggal dan Hari dalam seminggu
    if (d !== '*' && dw !== '*') {
      const parts = dw.split(',').map((x) => DOW_NAMES[x.trim()] || `hari ${x}`);
      return `🗓️ Dijalankan pada tanggal ${d} dan setiap hari ${parts.join(', ')} pada ${timeLabel}${timeHint}.`;
    }

    // 10. Fallback pola crontab lainnya
    return `⚙️ Dijalankan berdasarkan pola Crontab: Menit (${m}), Jam (${h}), Tanggal (${d}), Bulan (${mo}), Hari (${dw})${timeHint}.`;
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
      return `⚡ Tugas akan dijalankan setiap ${genMinuteInterval} menit sekali secara berulang.`;
    }
    if (activeTab === 'hourly') {
      return `⏱️ Tugas akan dijalankan setiap jam pada menit ke-${String(genHourlyMinute).padStart(2, '0')}.`;
    }
    if (activeTab === 'daily') {
      if (timezoneBasis === 'client') {
        const srvHour = (genDailyHour - diff + 24) % 24;
        return `🕒 Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz} (otomatis dikonversi ke ${String(srvHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz} waktu server).`;
      } else {
        const cltHour = (genDailyHour + diff + 24) % 24;
        return `🕒 Berjalan setiap hari pukul ${String(genDailyHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${serverTz} server time (setara pukul ${String(cltHour).padStart(2, '0')}:${String(genDailyMinute).padStart(2, '0')} ${clientTz} Anda).`;
      }
    }
    if (activeTab === 'weekly') {
      const dayName = WEEKDAYS.find((w) => w.id === genWeeklyDay)?.label || 'Hari';
      if (timezoneBasis === 'client') {
        return `📅 Berjalan setiap hari ${dayName} pukul ${String(genWeeklyHour).padStart(2, '0')}:${String(genWeeklyMinute).padStart(2, '0')} ${clientTz}.`;
      } else {
        return `📅 Berjalan setiap ${dayName} pukul ${String(genWeeklyHour).padStart(2, '0')}:${String(genWeeklyMinute).padStart(2, '0')} ${serverTz}.`;
      }
    }
    if (activeTab === 'monthly') {
      return `📆 Berjalan setiap tanggal ${genMonthlyDate} pukul ${String(genMonthlyHour).padStart(2, '0')}:${String(genMonthlyMinute).padStart(2, '0')} (${timezoneBasis === 'client' ? clientTz : serverTz}).`;
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

  async function handleCreateJob() {
    if (!formName.trim() || !formCommand.trim()) {
      notify('Nama tugas dan perintah eksekusi wajib diisi', 'warning');
      return;
    }

    try {
      submitting = true;
      await apiFetch('/api/cron/jobs', {
        method: 'POST',
        body: JSON.stringify({
          name: formName,
          schedule_type: activeTab === 'custom' ? 'custom' : activeTab,
          spec: fullCronExpression,
          command: formCommand,
          timeout_sec: formTimeout,
          save_log: formSaveLog,
        }),
      });

      notify('Tugas cron berhasil dijadwalkan!', 'success');
      showCreateModal = false;
      resetForm();
      await loadJobs();
    } catch (err: any) {
      notify(err.message || 'Gagal menambahkan tugas cron', 'error');
    } finally {
      submitting = false;
    }
  }

  function resetForm() {
    formName = '';
    activeTab = 'daily';
    timezoneBasis = 'client';
    genDailyHour = 12;
    genDailyMinute = 0;
    formCommand = '';
    selectedPreset = 'custom';
    formTimeout = 3600;
    formSaveLog = true;
    updateCrontabFromGenerator();
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
      confirmLabel: 'Hapus',
      variant: 'danger',
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

  async function viewLogs(job: CronJob) {
    selectedJob = job;
    showLogModal = true;
    try {
      loadingLogs = true;
      const res = await apiFetch<{ ok: boolean; logs: CronLog[] }>(`/api/cron/jobs/${job.id}/logs`);
      logs = res?.logs || [];
    } catch (err: any) {
      notify(err.message || 'Gagal memuat log', 'error');
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

<div class="cron-view">
  <!-- Header Bar -->
  <header class="page-head">
    <div class="page-title-wrap">
      <h1 class="page-title">
        <i class="ri-time-line"></i> Penjadwal Tugas (Cron Manager)
      </h1>
      <p class="page-sub">
        Jadwalkan eksekusi skrip background, backup otomatis, dan maintenance server dengan sinkronisasi zona waktu WIB dan server UTC.
      </p>
    </div>
    <div class="page-actions">
      <!-- Live Server Time Pill (ala HestiaCP) -->
      <div class="time-pill">
        <span class="time-item">
          <i class="ri-server-line"></i>
          <strong>Server Time:</strong> {formatDate(serverNow)}, {formatTime(serverNow)} <span class="tz-badge">{serverTimeInfo?.timezone_name || 'UTC'}</span>
        </span>
        <span class="time-sep">|</span>
        <span class="time-item">
          <i class="ri-user-location-line"></i>
          <strong>Waktu Anda:</strong> {formatTime(clientNow)} <span class="tz-badge highlight">{getClientTimezoneLabel()}</span>
        </span>
      </div>

      <button class="btn btn-ghost" onclick={loadJobs} disabled={loading}>
        <i class="ri-refresh-line {loading ? 'spin' : ''}"></i> Refresh
      </button>
      <button class="btn btn-primary" onclick={() => { resetForm(); showCreateModal = true; }}>
        <i class="ri-add-line"></i> Tambah Tugas
      </button>
    </div>
  </header>

  <!-- Table Card -->
  <div class="panel-card">
    <div class="table-wrap">
      <table class="data-table">
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
              <td colspan="6" class="table-empty">
                <i class="ri-loader-4-line spin text-lg"></i> Memuat daftar tugas terjadwal...
              </td>
            </tr>
          {:else if jobs.length === 0}
            <tr>
              <td colspan="6" class="table-empty">
                Belum ada tugas cron yang didaftarkan. Klik "Tambah Tugas" untuk menjadwalkan pekerjaan otomatis.
              </td>
            </tr>
          {:else}
            {#each jobs as job (job.id)}
              <tr>
                <td>
                  <div class="job-name-col">
                    <span class="status-indicator {job.enabled ? 'active' : 'paused'}" title={job.enabled ? 'Aktif' : 'Dijeda'}></span>
                    <div>
                      <span class="font-bold block">{job.name}</span>
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
                  {job.command}
                </td>
                <td>
                  {#if job.last_status === 'success'}
                    <span class="status-badge success"><i class="ri-checkbox-circle-fill"></i> Berhasil</span>
                  {:else if job.last_status === 'failed'}
                    <span class="status-badge failed"><i class="ri-close-circle-fill"></i> Gagal</span>
                  {:else if job.last_status === 'running'}
                    <span class="status-badge running"><i class="ri-loader-4-line spin"></i> Berjalan</span>
                  {:else}
                    <span class="text-muted">-</span>
                  {/if}
                </td>
                <td class="text-muted">
                  {formatTimestamp(job.last_run_at)}
                </td>
                <td class="text-right actions-col">
                  <button class="btn-icon" title="Jalankan Sekarang" onclick={() => runJobNow(job)}>
                    <i class="ri-play-line"></i>
                  </button>
                  <button class="btn-icon" title={job.enabled ? 'Jeda Tugas' : 'Aktifkan'} onclick={() => toggleJob(job)}>
                    <i class={job.enabled ? 'ri-pause-line' : 'ri-play-circle-line'}></i>
                  </button>
                  <button class="btn-icon" title="Riwayat Log" onclick={() => viewLogs(job)}>
                    <i class="ri-file-list-3-line"></i>
                  </button>
                  <button class="btn-icon danger" title="Hapus Tugas" onclick={() => deleteJob(job)}>
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
</div>

<!-- Modal Tambah Tugas (Desain Modern HestiaCP + aaPanel Style) -->
{#if showCreateModal}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showCreateModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showCreateModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-time-line"></i>
          <div>
            <h3>Tambah Tugas Terjadwal (Cron Job)</h3>
            <span class="modal-sub">Konfigurasi otomatisasi skrip server dengan konversi zona waktu real-time.</span>
          </div>
        </div>
        <div class="modal-head-right">
          <div class="server-clock-tag">
            <i class="ri-time-line"></i>
            <span>System time: <strong>{formatDate(serverNow)}, {formatTime(serverNow)}</strong></span>
          </div>
          <button class="btn-close" onclick={() => (showCreateModal = false)}>&times;</button>
        </div>
      </div>

      <form onsubmit={(e) => { e.preventDefault(); handleCreateJob(); }} class="form-body">
        <!-- Nama Tugas -->
        <div class="form-group">
          <label for="form-name">Nama Tugas Terjadwal</label>
          <input id="form-name" type="text" bind:value={formName} placeholder="Misal: Backup Database Tiap Jam 12 WIB" required />
        </div>

        <!-- Schedule Builder Container (HestiaCP Visual Tabs) -->
        <div class="schedule-builder-card">
          <div class="builder-head">
            <label class="builder-label"><i class="ri-calendar-schedule-line"></i> Penjadwalan Eksekusi</label>
            <!-- Tabs Menu -->
            <div class="schedule-tabs">
              <button
                type="button"
                class="tab-btn {activeTab === 'minutes' ? 'active' : ''}"
                onclick={() => { activeTab = 'minutes'; updateCrontabFromGenerator(); }}
              >
                MENIT
              </button>
              <button
                type="button"
                class="tab-btn {activeTab === 'hourly' ? 'active' : ''}"
                onclick={() => { activeTab = 'hourly'; updateCrontabFromGenerator(); }}
              >
                PER JAM
              </button>
              <button
                type="button"
                class="tab-btn {activeTab === 'daily' ? 'active' : ''}"
                onclick={() => { activeTab = 'daily'; updateCrontabFromGenerator(); }}
              >
                HARIAN
              </button>
              <button
                type="button"
                class="tab-btn {activeTab === 'weekly' ? 'active' : ''}"
                onclick={() => { activeTab = 'weekly'; updateCrontabFromGenerator(); }}
              >
                MINGGUAN
              </button>
              <button
                type="button"
                class="tab-btn {activeTab === 'monthly' ? 'active' : ''}"
                onclick={() => { activeTab = 'monthly'; updateCrontabFromGenerator(); }}
              >
                BULANAN
              </button>
              <button
                type="button"
                class="tab-btn {activeTab === 'custom' ? 'active' : ''}"
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
                  <select bind:value={genMinuteInterval} onchange={updateCrontabFromGenerator} class="select-inline">
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
                  <select bind:value={genHourlyMinute} onchange={updateCrontabFromGenerator} class="select-inline">
                    <option value={0}>00 (Awal jam)</option>
                    <option value={5}>05</option>
                    <option value={15}>15</option>
                    <option value={30}>30 (Setengah jam)</option>
                    <option value={45}>45</option>
                  </select>
                </div>
              </div>

            <!-- TAB: DAILY (Dengan Fitur Konversi WIB vs UTC) -->
            {:else if activeTab === 'daily'}
              <div class="tab-content">
                <div class="tz-switch-bar">
                  <span class="tz-prompt">Basis Zona Waktu:</span>
                  <div class="tz-pills">
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'client' ? 'active' : ''}"
                      onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                    >
                      <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                    </button>
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'server' ? 'active' : ''}"
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
                      <select id="daily-hour" bind:value={genDailyHour} onchange={updateCrontabFromGenerator}>
                        {#each Array(24) as _, i}
                          <option value={i}>{String(i).padStart(2, '0')}:00 {i === 12 ? '(Siang)' : i === 0 ? '(Tengah Malam)' : ''}</option>
                        {/each}
                      </select>
                    </div>
                    <span class="time-sep-colon">:</span>
                    <div class="field-item">
                      <label for="daily-min">Menit (0-59)</label>
                      <select id="daily-min" bind:value={genDailyMinute} onchange={updateCrontabFromGenerator}>
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
                  <div class="tz-pills">
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'client' ? 'active' : ''}"
                      onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                    >
                      <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                    </button>
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'server' ? 'active' : ''}"
                      onclick={() => { timezoneBasis = 'server'; updateCrontabFromGenerator(); }}
                    >
                      <i class="ri-server-line"></i> {getServerTimezoneLabel()}
                    </button>
                  </div>
                </div>

                <div class="input-inline-wrap wrap-weekly">
                  <div class="field-item">
                    <label for="weekly-day">Hari</label>
                    <select id="weekly-day" bind:value={genWeeklyDay} onchange={updateCrontabFromGenerator}>
                      {#each WEEKDAYS as w}
                        <option value={w.id}>{w.label}</option>
                      {/each}
                    </select>
                  </div>
                  <div class="field-item">
                    <label for="weekly-hour">Jam</label>
                    <select id="weekly-hour" bind:value={genWeeklyHour} onchange={updateCrontabFromGenerator}>
                      {#each Array(24) as _, i}
                        <option value={i}>{String(i).padStart(2, '0')}</option>
                      {/each}
                    </select>
                  </div>
                  <div class="field-item">
                    <label for="weekly-min">Menit</label>
                    <select id="weekly-min" bind:value={genWeeklyMinute} onchange={updateCrontabFromGenerator}>
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
                  <div class="tz-pills">
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'client' ? 'active' : ''}"
                      onclick={() => { timezoneBasis = 'client'; updateCrontabFromGenerator(); }}
                    >
                      <i class="ri-map-pin-user-line"></i> {getClientTimezoneLabel()}
                    </button>
                    <button
                      type="button"
                      class="tz-btn {timezoneBasis === 'server' ? 'active' : ''}"
                      onclick={() => { timezoneBasis = 'server'; updateCrontabFromGenerator(); }}
                    >
                      <i class="ri-server-line"></i> {getServerTimezoneLabel()}
                    </button>
                  </div>
                </div>

                <div class="input-inline-wrap wrap-weekly">
                  <div class="field-item">
                    <label for="monthly-date">Tanggal (1-31)</label>
                    <select id="monthly-date" bind:value={genMonthlyDate} onchange={updateCrontabFromGenerator}>
                      {#each Array(31) as _, i}
                        <option value={i + 1}>Tanggal {i + 1}</option>
                      {/each}
                    </select>
                  </div>
                  <div class="field-item">
                    <label for="monthly-hour">Jam</label>
                    <select id="monthly-hour" bind:value={genMonthlyHour} onchange={updateCrontabFromGenerator}>
                      {#each Array(24) as _, i}
                        <option value={i}>{String(i).padStart(2, '0')}</option>
                      {/each}
                    </select>
                  </div>
                  <div class="field-item">
                    <label for="monthly-min">Menit</label>
                    <select id="monthly-min" bind:value={genMonthlyMinute} onchange={updateCrontabFromGenerator}>
                      {#each [0, 15, 30, 45] as m}
                        <option value={m}>{String(m).padStart(2, '0')}</option>
                      {/each}
                    </select>
                  </div>
                </div>
              </div>

            <!-- TAB: CUSTOM (5-Field Direct Input) -->
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

            <!-- 5 Kolom Crontab Visual Box (HestiaCP Style) -->
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

            <!-- Real-time Human Explanation Card -->
            <div class="explanation-box">
              <div class="explanation-row">
                <span class="badge-tag mono">{fullCronExpression}</span>
                <span class="explanation-text">{scheduleHumanExplanation}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Command Presets (aaPanel Style) -->
        <div class="form-group">
          <div class="label-row">
            <label for="form-preset">Pilih Preset Perintah Cepat (Opsional)</label>
            <span class="hint-inline">Gunakan template bawaan untuk backup & maintenance</span>
          </div>
          <select id="form-preset" value={selectedPreset} onchange={onPresetSelect}>
            <option value="custom">-- Kustom / Tulis Perintah Sendiri --</option>
            <option value="mysql_backup">📦 Backup Database MySQL ke /backup/mysql/ (mysqldump)</option>
            <option value="docker_prune">🐳 Bersihkan Container & Image Docker Tak Terpakai (docker system prune)</option>
            <option value="laravel_artisan">⚙️ Laravel Scheduler (php artisan schedule:run)</option>
            <option value="nginx_reload">🔄 Reload Konfigurasi Web Server (systemctl reload nginx)</option>
            <option value="curl_webhook">🌐 Ping Webhook / Heartbeat URL (curl -fsS)</option>
            <option value="clean_logs">🧹 Hapus Log Nginx Lama (>7 Hari)</option>
          </select>
        </div>

        <!-- Shell Command Textarea -->
        <div class="form-group">
          <label for="form-cmd">Perintah Eksekusi Shell (Linux Command)</label>
          <textarea
            id="form-cmd"
            bind:value={formCommand}
            rows="3"
            class="mono"
            placeholder="/usr/local/bin/backup.sh atau docker system prune -af"
            required
          ></textarea>
          <span class="security-warning">
            <i class="ri-shield-check-line"></i>
            Perintah berbahaya seperti <code>shutdown</code>, <code>init 0</code>, <code>mkfs</code>, atau <code>passwd</code> dilarang dijalankan demi kestabilan server.
          </span>
        </div>

        <!-- Checkbox Simpan Log -->
        <div class="checkbox-row">
          <label class="custom-chk">
            <input type="checkbox" bind:checked={formSaveLog} />
            <span>Simpan riwayat log eksekusi (output terminal dan status kode keluar)</span>
          </label>
        </div>

        <!-- Modal Actions -->
        <div class="modal-actions">
          <button type="button" class="btn btn-ghost" onclick={() => (showCreateModal = false)}>Batal</button>
          <button type="submit" class="btn btn-primary" disabled={submitting}>
            {submitting ? 'Menyimpan...' : 'Simpan Tugas Cron'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Modal Riwayat Log -->
{#if showLogModal && selectedJob}
  <div
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onclick={(e) => { if (e.target === e.currentTarget) showLogModal = false; }}
    onkeydown={(e) => { if (e.key === 'Escape') showLogModal = false; }}
  >
    <div class="modal-card modal-wide">
      <div class="modal-head">
        <div class="modal-title-wrap">
          <i class="ri-file-list-3-line"></i>
          <div>
            <h3>Riwayat Eksekusi: {selectedJob.name}</h3>
            <span class="modal-sub mono text-xs">{selectedJob.command}</span>
          </div>
        </div>
        <button class="btn-close" onclick={() => (showLogModal = false)}>&times;</button>
      </div>

      <div class="log-container">
        {#if loadingLogs}
          <div class="table-empty">
            <i class="ri-loader-4-line spin text-lg"></i> Memuat log eksekusi...
          </div>
        {:else if logs.length === 0}
          <div class="table-empty">
            Belum ada catatan eksekusi untuk tugas ini. Klik ikon play pada tabel untuk menjalankan sekarang.
          </div>
        {:else}
          <div class="log-list">
            {#each logs as log (log.id)}
              <div class="log-item">
                <div class="log-meta">
                  <span class="status-badge {log.status}">
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

      <div class="modal-actions">
        <button type="button" class="btn btn-ghost" onclick={() => (showLogModal = false)}>Tutup</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .cron-view {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .page-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }
  .page-title {
    font-size: 1.4rem;
    font-weight: 700;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin: 0;
  }
  .page-sub {
    font-size: 0.85rem;
    color: var(--text-muted);
    margin: 0.25rem 0 0 0;
  }
  .page-actions {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .time-pill {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    padding: 0.4rem 0.8rem;
    border-radius: var(--radius-lg);
    font-size: 0.76rem;
    color: var(--text-muted);
  }
  .time-item {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .time-sep {
    color: var(--border);
  }
  .tz-badge {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.1rem 0.35rem;
    border-radius: 4px;
    font-size: 0.68rem;
    font-family: var(--font-mono);
  }
  .tz-badge.highlight {
    background: var(--tint-accent);
    color: var(--accent);
    border-color: var(--ring-accent);
  }
  .panel-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    overflow: hidden;
  }
  .table-wrap {
    overflow-x: auto;
  }
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
    text-align: left;
  }
  .data-table th {
    background: var(--bg-surface-soft);
    color: var(--text-subtle);
    padding: 0.75rem 1rem;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    border-bottom: 1px solid var(--border);
  }
  .data-table td {
    padding: 0.85rem 1rem;
    border-bottom: 1px solid var(--border-soft);
    color: var(--text);
  }
  .data-table tr:hover {
    background: var(--bg-hover);
  }
  .job-name-col {
    display: flex;
    align-items: center;
    gap: 0.65rem;
  }
  .status-indicator {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .status-indicator.active {
    background: var(--accent);
    box-shadow: 0 0 8px var(--ring-accent);
  }
  .status-indicator.paused {
    background: var(--text-subtle);
  }
  .spec-cell {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .badge-spec {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    font-family: var(--font-mono);
    font-size: 0.76rem;
    width: fit-content;
  }
  .spec-human {
    font-size: 0.72rem;
    color: var(--text-muted);
  }
  .col-cmd {
    max-width: 280px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-muted);
  }
  .status-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-size: 0.75rem;
    font-weight: 600;
  }
  .status-badge.success { color: var(--accent); }
  .status-badge.failed { color: var(--danger); }
  .status-badge.running { color: var(--warning); }
  .btn-icon {
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    padding: 0.35rem;
    border-radius: 6px;
    font-size: 1rem;
    transition: background 0.15s, color 0.15s;
  }
  .btn-icon:hover {
    background: var(--bg-hover);
    color: var(--text);
  }
  .btn-icon.danger:hover {
    background: var(--tint-danger);
    color: var(--danger);
  }
  .text-right { text-align: right; }
  .mono { font-family: var(--font-mono); }
  .text-muted { color: var(--text-muted); }
  .text-subtle { color: var(--text-subtle); }
  .font-bold { font-weight: 600; }
  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { 100% { transform: rotate(360deg); } }

  /* Modal Styling */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-overlay);
    backdrop-filter: blur(8px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: var(--z-modal);
    padding: 1rem;
  }
  .modal-card {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: var(--radius-xl);
    width: 100%;
    max-width: 540px;
    padding: 1.5rem;
    box-shadow: var(--shadow-modal);
    max-height: 90vh;
    overflow-y: auto;
  }
  .modal-card.modal-wide {
    max-width: 820px;
  }
  .modal-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1.25rem;
    border-bottom: 1px solid var(--border-soft);
    padding-bottom: 1rem;
  }
  .modal-title-wrap {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .modal-title-wrap i {
    font-size: 1.4rem;
    color: var(--primary);
  }
  .modal-title-wrap h3 {
    margin: 0;
    font-size: 1.05rem;
    color: var(--text);
  }
  .modal-sub {
    font-size: 0.78rem;
    color: var(--text-muted);
    margin-top: 0.15rem;
    display: block;
  }
  .modal-head-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .server-clock-tag {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    font-size: 0.72rem;
    color: var(--text-muted);
  }
  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-subtle);
    font-size: 1.4rem;
    cursor: pointer;
  }
  .btn-close:hover { color: var(--text); }
  .form-body {
    display: flex;
    flex-direction: column;
    gap: 1.15rem;
  }
  .form-group label {
    display: block;
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--text);
    margin-bottom: 0.35rem;
  }
  .form-group input, .form-group select, .form-group textarea {
    width: 100%;
    padding: 0.65rem 0.8rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--text);
    font-size: 0.82rem;
    outline: none;
    box-sizing: border-box;
  }
  .form-group input:focus, .form-group select:focus, .form-group textarea:focus {
    border-color: var(--primary);
  }
  .label-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.35rem;
  }
  .label-row label {
    margin-bottom: 0;
  }
  .hint-inline {
    font-size: 0.72rem;
    color: var(--text-muted);
  }

  /* Schedule Builder Card (HestiaCP Inspired) */
  .schedule-builder-card {
    background: var(--bg-surface-soft);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
  }
  .builder-head {
    padding: 0.75rem 1rem;
    background: var(--bg-inset);
    border-bottom: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
  }
  .builder-label {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text);
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .schedule-tabs {
    display: flex;
    gap: 0.35rem;
    overflow-x: auto;
    padding-bottom: 2px;
  }
  .tab-btn {
    background: transparent;
    border: 1px solid transparent;
    padding: 0.35rem 0.7rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.04em;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;
  }
  .tab-btn:hover {
    color: var(--text);
    background: var(--bg-hover);
  }
  .tab-btn.active {
    background: var(--bg-surface);
    color: var(--accent);
    border-color: var(--border);
    box-shadow: var(--shadow-sm);
  }
  .builder-body {
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .tab-content {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .content-desc {
    font-size: 0.78rem;
    color: var(--text-muted);
  }
  .custom-tab-head {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .quick-cron-examples {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
    padding-top: 0.2rem;
  }
  .quick-examples-label {
    font-size: 0.72rem;
    color: var(--text-subtle);
    font-weight: 500;
  }
  .example-chip {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
    font-size: 0.72rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .example-chip:hover {
    background: var(--tint-accent);
    color: var(--accent-strong);
    border-color: var(--accent);
  }
  .input-inline-wrap {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    font-size: 0.8rem;
    color: var(--text);
    flex-wrap: wrap;
  }
  .select-inline {
    padding: 0.45rem 0.75rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.8rem;
    outline: none;
  }
  .tz-switch-bar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    background: var(--bg-inset);
    padding: 0.5rem 0.75rem;
    border-radius: 8px;
    border: 1px solid var(--border-soft);
    flex-wrap: wrap;
  }
  .tz-prompt {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-muted);
  }
  .tz-pills {
    display: flex;
    gap: 0.35rem;
  }
  .tz-btn {
    background: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text-muted);
    padding: 0.3rem 0.65rem;
    border-radius: 6px;
    font-size: 0.72rem;
    font-weight: 500;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 0.35rem;
    transition: all 0.15s;
  }
  .tz-btn.active {
    background: var(--tint-accent);
    border-color: var(--accent);
    color: var(--accent);
    font-weight: 600;
  }
  .time-inputs {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .field-item {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .field-item label {
    font-size: 0.7rem;
    color: var(--text-subtle);
  }
  .field-item select {
    padding: 0.45rem 0.6rem;
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.8rem;
  }
  .time-sep-colon {
    font-size: 1.2rem;
    font-weight: bold;
    color: var(--text-muted);
    margin-top: 1rem;
  }
  .wrap-weekly {
    gap: 1rem;
  }

  /* 5-Field Crontab Grid (HestiaCP Inspired) */
  .crontab-grid-box {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 0.5rem;
    background: var(--bg-inset);
    padding: 0.75rem;
    border-radius: 8px;
    border: 1px solid var(--border-soft);
  }
  .cron-field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .cron-field-label {
    font-size: 0.68rem;
    color: var(--text-subtle);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .cron-field-input {
    width: 100%;
    padding: 0.45rem 0.5rem;
    background: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 6px;
    color: var(--text);
    font-size: 0.85rem;
    text-align: center;
    font-weight: bold;
  }
  .cron-field-input:disabled {
    background: var(--bg-inset);
    color: var(--text);
    opacity: 0.9;
    border-color: var(--border-soft);
  }
  .cron-field-sub {
    font-size: 0.65rem;
    color: var(--text-subtle);
    text-align: center;
  }

  /* Explanation Box */
  .explanation-box {
    background: var(--tint-info);
    border: 1px solid var(--border-strong);
    border-radius: 8px;
    padding: 0.75rem 1rem;
  }
  .explanation-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }
  .badge-tag {
    background: var(--bg-surface);
    color: var(--info);
    border: 1px solid var(--border);
    padding: 0.25rem 0.5rem;
    border-radius: 6px;
    font-size: 0.8rem;
    font-weight: bold;
  }
  .explanation-text {
    font-size: 0.78rem;
    color: var(--text);
    line-height: 1.4;
  }

  /* Security Warning */
  .security-warning {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.72rem;
    color: var(--text-subtle);
    margin-top: 0.35rem;
  }
  .security-warning code {
    background: var(--bg-inset);
    padding: 0.1rem 0.3rem;
    border-radius: 4px;
    color: var(--danger);
  }

  .checkbox-row {
    padding-top: 0.25rem;
  }
  .custom-chk {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text-muted);
    cursor: pointer;
  }
  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    margin-top: 0.75rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border-soft);
  }
  .log-container {
    max-height: 60vh;
    overflow-y: auto;
  }
  .log-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .log-item {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.75rem;
  }
  .log-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.75rem;
    margin-bottom: 0.5rem;
  }
  .log-output {
    background: var(--bg-inset);
    border: 1px solid var(--border);
    padding: 0.6rem;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    font-size: 0.75rem;
    margin: 0;
    overflow-x: auto;
    white-space: pre-wrap;
    max-height: 180px;
  }
  .table-empty {
    text-align: center;
    padding: 2.5rem;
    color: var(--text-subtle);
  }
</style>
