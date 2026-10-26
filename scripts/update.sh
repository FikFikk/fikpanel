#!/usr/bin/env bash
# ==============================================================================
# FikPanel — Production Server Updater Script
# Skrip pembaruan otomatis untuk FikPanel tanpa menyentuh data atau website
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
echo "  █████╗  ██║█████═╝ ██████╔╝███████║██╔██╗ ██║█████╗  ██║     "
echo "  ██╔══╝  ██║██╔═██╗ ██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     "
echo "  ██║     ██║██║ ╚██╗██║     ██║  ██║██║ ╚████║███████╗███████╗"
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

# 3. Backup Biner Lama untuk Pencegahan Kegagalan
if [ -f "$INSTALL_DIR/bin/fikpanel" ]; then
    echo -e "[*] Membuat cadangan biner lama ke fikpanel.bak..."
    cp "$INSTALL_DIR/bin/fikpanel" "$INSTALL_DIR/bin/fikpanel.bak"
fi

# 4. Unduh Biner Versi Terbaru dari GitHub Releases
echo -e "[*] Mengunduh rilis biner FikPanel terbaru..."
DOWNLOAD_URL="https://github.com/FikFikk/fikpanel/releases/latest/download/fikpanel-linux-${ARCH}.tar.gz"
TEMP_FILE="/tmp/fikpanel-update.tar.gz"

if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
    echo -e "${RED}[ERROR] Gagal mengunduh biner terbaru dari GitHub Release.${NC}"
    echo -e "${YELLOW}Periksa koneksi internet server atau halaman https://github.com/FikFikk/fikpanel/releases${NC}"
    rm -f "$TEMP_FILE"
    exit 1
fi

# 5. Pasang Biner Baru
echo -e "[*] Memasang biner baru..."
tar -xzf "$TEMP_FILE" -C "$INSTALL_DIR/bin/"
chmod +x "$INSTALL_DIR/bin/fikpanel"
rm -f "$TEMP_FILE"

# 6. Restart Service Daemon FikPanel
echo -e "[*] Me-restart layanan fikpanel.service..."
systemctl restart fikpanel.service || true
sleep 2

# 7. Verifikasi Keberhasilan Update
if systemctl is-active --quiet fikpanel.service; then
    NEW_VER=$("$INSTALL_DIR/bin/fikpanel" version 2>/dev/null || echo "terbaru")
    echo -e "\n=============================================================="
    echo -e "${GREEN}${BOLD}✓ FikPanel Berhasil Diperbarui ke ${NEW_VER}!${NC}"
    echo -e "=============================================================="
    echo -e "• Database dan konfigurasi website : ${GREEN}Aman 100% (Tidak Tersentuh)${NC}"
    echo -e "• Status Layanan Daemon            : ${GREEN}Aktif & Berjalan${NC}"
    echo -e "• Biner Cadangan                   : ${INSTALL_DIR}/bin/fikpanel.bak"
    echo -e "==============================================================\n"
else
    echo -e "\n${RED}[!] Pembaruan mengalami kendala saat menyalakan layanan.${NC}"
    echo -e "${YELLOW}[*] Melakukan rollback ke versi sebelumnya...${NC}"
    if [ -f "$INSTALL_DIR/bin/fikpanel.bak" ]; then
        cp "$INSTALL_DIR/bin/fikpanel.bak" "$INSTALL_DIR/bin/fikpanel"
        chmod +x "$INSTALL_DIR/bin/fikpanel"
        systemctl restart fikpanel.service || true
        echo -e "${GREEN}[✓] Berhasil dipulihkan ke versi cadangan.${NC}"
    fi
    echo -e "Periksa detail log: journalctl -u fikpanel -n 30 --no-pager"
    exit 1
fi
