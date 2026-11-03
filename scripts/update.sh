#!/usr/bin/env bash
# ==============================================================================
# FikPanel — Production Server Updater Script
# Skrip pembaruan otomatis untuk FikPanel tanpa menyentuh data atau website
# Mendukung pembaruan ke versi terbaru atau versi tertentu (upgrade/downgrade)
# ==============================================================================

set -euo pipefail

# Warna terminal untuk output estetis
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Verifikasi hak akses root
if [ "$(id -u)" -ne 0 ]; then
    echo -e "${RED}[ERROR] Skrip pembaruan ini wajib dijalankan sebagai root (gunakan sudo).${NC}" >&2
    exit 1
fi

INSTALL_DIR="/opt/fikpanel"

if [ ! -d "$INSTALL_DIR" ]; then
    echo -e "${RED}[ERROR] Direktori FikPanel ($INSTALL_DIR) tidak ditemukan. Apakah FikPanel sudah terpasang?${NC}" >&2
    exit 1
fi

# Bersihkan layar hanya jika terminal interaktif (TTY) dan variabel TERM valid
if [ -t 1 ] && [ -n "${TERM:-}" ] && [ "$TERM" != "dumb" ] && [ "$TERM" != "unknown" ]; then
    clear 2>/dev/null || true
fi
echo -e "${CYAN}${BOLD}"
echo "  ███████╗██╗██╗  ██╗██████╗  █████╗ ███╗   ██╗███████╗██╗     "
echo "  ██╔════╝██║██║ ██╔╝██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     "
echo "  █████╗  ██║█████╔╝ ██████╔╝███████║██╔██╗ ██║█████╗  ██║     "
echo "  ██╔══╝  ██║██╔═██╗ ██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     "
echo "  ██║     ██║██║  ██╗██║     ██║  ██║██║ ╚████║███████╗███████╗"
echo "  ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝"
echo -e "${NC}"
echo -e "${BOLD}FikPanel Server Control Panel — Auto-Updater${NC}"
echo -e "Waktu Pembaruan: $(date '+%Y-%m-%d %H:%M:%S')\n"

# 1. Tampilkan Versi Lama
OLD_VER="unknown"
if [ -x "$INSTALL_DIR/bin/fikpanel" ]; then
    OLD_VER=$("$INSTALL_DIR/bin/fikpanel" version 2>/dev/null || echo "terpasang")
fi
echo -e "[*] Versi saat ini terpasang : ${YELLOW}${OLD_VER}${NC}"

# 2. Deteksi Arsitektur Mesin
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) ARCH="amd64" ;;
esac

echo -e "[*] Arsitektur sistem        : ${CYAN}linux-${ARCH}${NC}"

# 3. Penentuan Target Versi
TARGET_VER="${1:-${TARGET_VERSION:-latest}}"
if [ "$TARGET_VER" = "latest" ] || [ -z "$TARGET_VER" ]; then
    DOWNLOAD_URL="https://github.com/FikFikk/fikpanel/releases/latest/download/fikpanel-linux-${ARCH}.tar.gz"
    echo -e "[*] Target pembaruan         : ${GREEN}Rilis Terbaru (latest)${NC}"
else
    TAG="$TARGET_VER"
    # Normalisasi tag: tambahkan awalan 'v' jika pengguna hanya mengetik angka (misal: 0.3.1 -> v0.3.1)
    if [[ "$TAG" != v* ]]; then
        TAG="v$TAG"
    fi
    DOWNLOAD_URL="https://github.com/FikFikk/fikpanel/releases/download/${TAG}/fikpanel-linux-${ARCH}.tar.gz"
    echo -e "[*] Target pembaruan         : ${GREEN}${TAG}${NC}"
fi

# 4. Backup Biner Lama untuk Pencegahan Kegagalan
if [ -f "$INSTALL_DIR/bin/fikpanel" ]; then
    echo -e "[*] Membuat cadangan biner lama ke fikpanel.bak..."
    install -m 755 "$INSTALL_DIR/bin/fikpanel" "$INSTALL_DIR/bin/fikpanel.bak"
fi

# 5. Unduh Biner dari GitHub Releases
TEMP_FILE="/tmp/fikpanel-update.tar.gz"

echo -e "[*] Mengunduh paket biner FikPanel (~8.3 MB)..."
echo -e "    ${CYAN}Sumber rilis: ${DOWNLOAD_URL}${NC}\n"

# Gunakan --progress-bar agar proses unduhan terpantau jelas di terminal
if ! curl -fL --progress-bar --connect-timeout 20 --retry 3 "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
    echo -e "\n${RED}[ERROR] Gagal mengunduh biner FikPanel dari GitHub Release.${NC}"
    echo -e "${YELLOW}Pastikan koneksi internet aktif atau rilis '${TARGET_VER}' tersedia di https://github.com/FikFikk/fikpanel/releases${NC}"
    rm -f "$TEMP_FILE"
    exit 1
fi
echo -e "${GREEN}[✓] Unduhan selesai.${NC}\n"

# 6. Pasang Biner Baru secara Atomik
echo -e "[*] Memasang biner baru..."
TEMP_EXTRACT_DIR=$(mktemp -d /tmp/fikpanel-extract.XXXXXX)
tar -xzf "$TEMP_FILE" -C "$TEMP_EXTRACT_DIR"

if [ -f "$TEMP_EXTRACT_DIR/fikpanel" ]; then
    # Pastikan direktori tujuan tersedia
    mkdir -p "$INSTALL_DIR/bin" /usr/local/bin
    # Gunakan 'install -m 755' untuk penggantian inode secara atomik (mencegah error Text file busy)
    install -m 755 "$TEMP_EXTRACT_DIR/fikpanel" "$INSTALL_DIR/bin/fikpanel"
    install -m 755 "$TEMP_EXTRACT_DIR/fikpanel" /usr/local/bin/fikpanel
    rm -rf "$TEMP_EXTRACT_DIR"
else
    echo -e "${RED}[ERROR] Berkas biner fikpanel tidak ditemukan dalam paket rilis.${NC}"
    rm -rf "$TEMP_EXTRACT_DIR" "$TEMP_FILE"
    exit 1
fi
rm -f "$TEMP_FILE"

# 7. Sinkronkan Skrip Updater Lokal ke Versi Terbaru
if [ -d "$INSTALL_DIR/scripts" ]; then
    echo -e "[*] Menyinkronkan skrip pembaru lokal ($INSTALL_DIR/scripts/update.sh)..."
    curl -sSL --connect-timeout 10 "https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/update.sh" -o "$INSTALL_DIR/scripts/update.sh" 2>/dev/null || true
    chmod +x "$INSTALL_DIR/scripts/update.sh" 2>/dev/null || true
fi

# 8. Verifikasi Eksekusi Biner Baru
NEW_VER=$("$INSTALL_DIR/bin/fikpanel" version 2>/dev/null || echo "")
if [ -z "$NEW_VER" ]; then
    echo -e "\n${RED}[!] Biner baru gagal dieksekusi.${NC}"
    echo -e "${YELLOW}[*] Melakukan rollback ke versi sebelumnya...${NC}"
    if [ -f "$INSTALL_DIR/bin/fikpanel.bak" ]; then
        install -m 755 "$INSTALL_DIR/bin/fikpanel.bak" "$INSTALL_DIR/bin/fikpanel"
        install -m 755 "$INSTALL_DIR/bin/fikpanel.bak" /usr/local/bin/fikpanel
        if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files 2>/dev/null | grep -q "fikpanel.service"; then
            if command -v systemd-run >/dev/null 2>&1; then
                systemd-run --no-block sh -c "sleep 2 && systemctl restart fikpanel.service"
            else
                nohup sh -c "sleep 2 && systemctl restart fikpanel.service" >/dev/null 2>&1 &
            fi
        fi
        echo -e "${GREEN}[✓] Berhasil dipulihkan ke versi cadangan.${NC}"
    fi
    echo -e "Periksa detail log: journalctl -u fikpanel -n 30 --no-pager"
    exit 1
fi

# 9. Tampilkan Ringkasan Keberhasilan Pembaruan
echo -e "\n=============================================================="
echo -e "${GREEN}${BOLD}✓ FikPanel Berhasil Diperbarui ke ${NEW_VER}!${NC}"
echo -e "=============================================================="
echo -e "• Database dan konfigurasi website : ${GREEN}Aman 100% (Tidak Tersentuh)${NC}"
echo -e "• Biner Cadangan                   : ${INSTALL_DIR}/bin/fikpanel.bak"
echo -e "• Lokasi Biner                     : ${INSTALL_DIR}/bin/fikpanel dan /usr/local/bin/fikpanel"

# 10. Restart Service Daemon FikPanel
if command -v systemctl >/dev/null 2>&1; then
    echo -e "• Status Layanan Daemon            : ${CYAN}Me-restart service fikpanel...${NC}"
    echo -e "==============================================================\n"
    systemctl daemon-reload 2>/dev/null || true
    echo -e "${GREEN}[✓] Biner baru siap aktif. Menjadwalkan restart layanan fikpanel...${NC}"
    echo -e "[*] Silakan muat ulang (refresh) dashboard di peramban Anda dalam 2-3 detik.\n"
    # Lepaskan eksekusi restart dari sesi terminal saat ini menggunakan systemd-run / nohup
    # agar tidak terjadi pemutusan paksa (502 Bad Gateway) saat terminal berjalan di dalam Web Dashboard
    if command -v systemd-run >/dev/null 2>&1; then
        systemd-run --no-block sh -c "sleep 1 && (systemctl restart fikpanel 2>/dev/null || systemctl restart fikpanel.service 2>/dev/null || true)" >/dev/null 2>&1 || (nohup sh -c "sleep 1 && (systemctl restart fikpanel 2>/dev/null || systemctl restart fikpanel.service 2>/dev/null || true)" >/dev/null 2>&1 &)
    else
        nohup sh -c "sleep 1 && (systemctl restart fikpanel 2>/dev/null || systemctl restart fikpanel.service 2>/dev/null || true)" >/dev/null 2>&1 &
    fi
else
    echo -e "• Status Layanan Daemon            : Standalone (systemctl tidak ditemukan)"
    echo -e "==============================================================\n"
fi
