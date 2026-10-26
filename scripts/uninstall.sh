#!/usr/bin/env bash
# ==============================================================
# FikPanel Server Control Panel — Uninstaller Script
# Repositori Resmi: https://github.com/FikFikk/fikpanel
# Lisensi: MIT
# ==============================================================

set -e

# Pewarnaan teks terminal
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# Pastikan dijalankan sebagai root (super-user)
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] Skrip uninstaller harus dijalankan dengan hak akses root / sudo.${NC}"
    exit 1
fi

AUTO_CONFIRM=false
FULL_PURGE=false

for arg in "$@"; do
    case "$arg" in
        -y|--yes)
            AUTO_CONFIRM=true
            ;;
        --full)
            FULL_PURGE=true
            ;;
        -h|--help)
            echo "Penggunaan: sudo ./uninstall.sh [opsi]"
            echo "Opsi:"
            echo "  -y, --yes    Lewati konfirmasi interaktif"
            echo "  --full       Hapus FikPanel dan paket stack (Nginx, MySQL, PHP)"
            echo "  -h, --help   Tampilkan pesan bantuan"
            exit 0
            ;;
    esac
done

echo -e "${RED}${BOLD}"
echo "  ███████╗██╗██╗  ██╗██████╗  █████╗ ███╗   ██╗███████╗██╗"
echo "  ██╔════╝██║██║ ██╔╝██╔══██╗██╔══██╗████╗  ██║██╔════╝██║"
echo "  █████╗  ██║█████═╝ ██████╔╝███████║██╔██╗ ██║█████╗  ██║"
echo "  ██╔══╝  ██║██╔═██╗ ██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║"
echo "  ██║     ██║██║ ╚██╗██║     ██║  ██║██║ ╚████║███████╗███████╗"
echo "  ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝"
echo -e "${NC}"
echo -e "${BOLD}FikPanel Server Control Panel — Uninstaller${NC}\n"

# Fungsi pembaca input interaktif yang handal saat dijalankan via pipe
prompt_read() {
    local prompt_msg="$1"
    local default_value="$2"
    local var_name="$3"
    local user_input=""

    if [ -t 0 ]; then
        read -rp "$prompt_msg" user_input || user_input=""
    elif [ -r /dev/tty ]; then
        read -rp "$prompt_msg" user_input </dev/tty || user_input=""
    else
        user_input="$default_value"
    fi

    user_input="${user_input:-$default_value}"
    printf -v "$var_name" '%s' "$user_input"
}

if [ "$AUTO_CONFIRM" = false ]; then
    echo -e "${YELLOW}[PERINGATAN] Tindakan ini akan menghentikan daemon FikPanel dan menghapus seluruh direktori /opt/fikpanel (termasuk database SQLite).${NC}"
    prompt_read "Apakah Anda yakin ingin mencopot FikPanel dari server? [y/N]: " "N" CONFIRM_UNINSTALL
    if [[ ! "$CONFIRM_UNINSTALL" =~ ^[Yy] ]]; then
        echo -e "${BLUE}[*] Pencopotan dibatalkan oleh pengguna.${NC}"
        exit 0
    fi
fi

echo -e "\n[*] Menghentikan layanan daemon FikPanel..."
if systemctl is-active --quiet fikpanel.service 2>/dev/null; then
    systemctl stop fikpanel.service || true
fi

if systemctl is-enabled --quiet fikpanel.service 2>/dev/null; then
    systemctl disable fikpanel.service || true
fi

echo "[*] Menghapus systemd service unit fikpanel.service..."
rm -f /etc/systemd/system/fikpanel.service
systemctl daemon-reload
systemctl reset-failed fikpanel.service 2>/dev/null || true

echo "[*] Menghapus symlink global fikpanel..."
rm -f /usr/local/bin/fikpanel

echo "[*] Menghapus direktori instalasi FikPanel (/opt/fikpanel)..."
rm -rf /opt/fikpanel

# Tutup port firewall 1405 yang sebelumnya dibuka untuk FikPanel
if which ufw >/dev/null 2>&1; then
    ufw delete allow 1405/tcp 2>/dev/null || true
elif which firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
    firewall-cmd --permanent --remove-port=1405/tcp 2>/dev/null || true
    firewall-cmd --reload 2>/dev/null || true
fi

# Opsi pembersihan paket stack jika mode --full diaktifkan
if [ "$FULL_PURGE" = true ]; then
    echo -e "\n[*] Membersihkan paket stack yang dipasang (Nginx, MySQL, PHP)..."
    if which apt-get >/dev/null 2>&1; then
        export DEBIAN_FRONTEND=noninteractive
        apt-get purge -y nginx nginx-common mysql-server php-fpm 2>/dev/null || true
        apt-get autoremove -y 2>/dev/null || true
    fi
fi

echo -e "\n=============================================================="
echo -e "${GREEN}${BOLD}✓ FikPanel berhasil dicopot sepenuhnya dari server.${NC}"
echo -e "=============================================================="
echo -e "Seluruh berkas biner, layanan daemon, dan data FikPanel telah dibersihkan."
echo -e "Perintah instalasi FikPanel:"
echo -e "  ${CYAN}curl -sSL https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/install.sh | sudo bash${NC}\n"
