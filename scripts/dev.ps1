# Skrip otomasi live-reload untuk Go (Air) dan Svelte (Vite HMR)
param (
    [int]$Port = 1405
)

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "   FikPanel Live Development Environment (Auto-Reload)    " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Pastikan port 1405 dan 5173 bersih dari proses lama yang menggantung
Get-Process -Name "fikpanel", "air" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

# 2. Periksa ketersediaan Air
$airCmd = Get-Command air -ErrorAction SilentlyContinue
if (-not $airCmd) {
    Write-Host "[!] 'air' tidak ditemukan di PATH. Mengunduh dan memasang 'air'..." -ForegroundColor Yellow
    go install github.com/air-verse/air@latest
    $airPath = Join-Path (go env GOPATH) "bin\air.exe"
} else {
    $airPath = $airCmd.Source
}

Write-Host "[+] Menjalankan Go backend dengan Air (port $Port)..." -ForegroundColor Green
$goProc = Start-Process -FilePath $airPath -NoNewWindow -PassThru

Write-Host "[+] Menjalankan Svelte frontend dengan Vite (port 5173)..." -ForegroundColor Green
$webProc = Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "npm", "run", "dev" -WorkingDirectory (Join-Path $PSScriptRoot "..\web") -NoNewWindow -PassThru

Write-Host "`n----------------------------------------------------------" -ForegroundColor DarkCyan
Write-Host "  Frontend (Vite HMR):   http://localhost:5173" -ForegroundColor White
Write-Host "  Backend (Go API):       http://127.0.0.1:$Port" -ForegroundColor White
Write-Host "  Mode Hot-Reload:        Aktif (Go + Svelte)" -ForegroundColor Yellow
Write-Host "  Hentikan server:        Tekan Ctrl+C" -ForegroundColor DarkGray
Write-Host "----------------------------------------------------------`n" -ForegroundColor DarkCyan

# Tangani shutdown bersih saat pengguna menekan Ctrl+C
try {
    # Loop monitor proses
    while (-not $goProc.HasExited -and -not $webProc.HasExited) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host "`n[*] Menghentikan server development..." -ForegroundColor Yellow
    
    # Hentikan tree proses Go dan Air
    if ($goProc -and -not $goProc.HasExited) {
        cmd.exe /c "taskkill /PID $($goProc.Id) /T /F" 2>$null
    }
    
    # Hentikan tree proses Vite dan Node
    if ($webProc -and -not $webProc.HasExited) {
        cmd.exe /c "taskkill /PID $($webProc.Id) /T /F" 2>$null
    }

    # Pastikan biner fikpanel benar-benar dihentikan
    Get-Process -Name "fikpanel" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

    Write-Host "[v] Seluruh proses berhasil dihentikan." -ForegroundColor Green
}
