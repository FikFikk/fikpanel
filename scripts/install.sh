#!/usr/bin/env bash
# ==============================================================================
# FikPanel — Production Server Installer Script
# Skrip instalasi otomatis untuk FikPanel dan software stack (Nginx, MySQL, PHP)
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
    echo -e "${RED}[ERROR] Skrip instalasi ini wajib dijalankan sebagai root (gunakan sudo).${NC}" >&2
    exit 1
fi

clear
echo -e "${CYAN}${BOLD}"
echo "  ███████╗██╗██╗  ██╗██████╗  █████╗ ███╗   ██╗███████╗██╗     "
echo "  ██╔════╝██║██║ ██╔╝██╔══██╗██╔══██╗████╗  ██║██╔════╝██║     "
echo "  █████╗  ██║█████═╝ ██████╔╝███████║██╔██╗ ██║█████╗  ██║     "
echo "  ██╔══╝  ██║██╔═██╗ ██╔═══╝ ██╔══██║██║╚██╗██║██╔══╝  ██║     "
echo "  ██║     ██║██║ ╚██╗██║     ██║  ██║██║ ╚████║███████╗███████╗"
echo "  ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝╚══════╝"
echo -e "${NC}"
echo -e "${BOLD}FikPanel Lightweight Server Control Panel — Installer${NC}"
echo -e "Waktu Sistem: $(date '+%Y-%m-%d %H:%M:%S')\n"

# 1. Deteksi Distribusi Linux
OS="unknown"
VER=""
PKG_MGR="unknown"

if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    VER=$VERSION_ID
fi

case "$OS" in
    ubuntu|debian)
        PKG_MGR="apt"
        ;;
    almalinux|rocky|centos|rhel|fedora)
        PKG_MGR="dnf"
        which dnf >/dev/null 2>&1 || PKG_MGR="yum"
        ;;
    *)
        echo -e "${YELLOW}[PERINGATAN] Distro '$OS' tidak diuji secara resmi. Mencoba dengan deteksi paket bawaan...${NC}"
        if which apt-get >/dev/null 2>&1; then
            PKG_MGR="apt"
        elif which dnf >/dev/null 2>&1; then
            PKG_MGR="dnf"
        elif which yum >/dev/null 2>&1; then
            PKG_MGR="yum"
        else
            echo -e "${RED}[ERROR] Package manager yang didukung (apt/dnf/yum) tidak ditemukan.${NC}"
            exit 1
        fi
        ;;
esac

echo -e "[*] Sistem Operasi terdeteksi: ${GREEN}${OS} ${VER}${NC} (Package Manager: ${CYAN}${PKG_MGR}${NC})"

# 2. Periksa RAM Fisik dan Alokasi Swap Otomatis (Mencegah OOM saat pasang MySQL 8)
TOTAL_RAM_MB=$(free -m | awk '/^Mem:/{print $2}')
SWAP_TOTAL_MB=$(free -m | awk '/^Swap:/{print $2}')

echo -e "[*] Total RAM Fisik: ${BOLD}${TOTAL_RAM_MB} MB${NC} | Total Swap: ${BOLD}${SWAP_TOTAL_MB} MB${NC}"

if [ "$TOTAL_RAM_MB" -lt 1900 ] && [ "$SWAP_TOTAL_MB" -lt 1024 ]; then
    echo -e "${YELLOW}[!] RAM server < 2 GB dan swap belum memadai. Membuat 2 GB swapfile otomatis...${NC}"
    if [ ! -f /swapfile ]; then
        fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
        chmod 600 /swapfile
        mkswap /swapfile
        swapon /swapfile
        echo '/swapfile none swap sw 0 0' >> /etc/fstab
        echo -e "${GREEN}[✓] Swapfile 2 GB berhasil dibuat dan diaktifkan.${NC}"
    fi
fi

# 3. Menu Interaktif Pemilihan Software Stack
echo -e "\n${BOLD}Pilih Paket Software Stack yang ingin dipasang bersama FikPanel:${NC}"
echo -e "  ${GREEN}1) Full LNMP${NC} (Nginx + MySQL 8.0 + PHP 8.3 FPM) [Sangat Direkomendasikan]"
echo -e "  ${CYAN}2) Reverse Proxy Saja${NC} (Hanya Nginx Web Server)"
echo -e "  ${BLUE}3) Minimal FikPanel Core${NC} (Panel saja — software dipasang nanti via antarmuka web)"
echo -e "  ${YELLOW}4) Kustom Stack${NC} (Pilih manual masing-masing software)"

read -rp "Masukkan nomor pilihan [1-4] (default: 1): " STACK_CHOICE
STACK_CHOICE=${STACK_CHOICE:-1}

INSTALL_NGINX=false
INSTALL_APACHE=false
INSTALL_MYSQL=false
INSTALL_PHP=false
INSTALL_NODE=false

case "$STACK_CHOICE" in
    1)
        INSTALL_NGINX=true
        INSTALL_MYSQL=true
        INSTALL_PHP=true
        ;;
    2)
        INSTALL_NGINX=true
        ;;
    3)
        # Hanya instal panel
        ;;
    4)
        echo -e "\n${CYAN}Konfigurasi Pilihan Kustom:${NC}"
        read -rp "  Pasang Nginx Web Server? [Y/n]: " yn_nginx
        [[ "${yn_nginx:-Y}" =~ ^[Yy] ]] && INSTALL_NGINX=true

        if [ "$INSTALL_NGINX" = false ]; then
            read -rp "  Pasang Apache HTTP Server sebagai gantinya? [y/N]: " yn_apache
            [[ "${yn_apache:-N}" =~ ^[Yy] ]] && INSTALL_APACHE=true
        fi

        read -rp "  Pasang MySQL 8.0 Database Server? [Y/n]: " yn_mysql
        [[ "${yn_mysql:-Y}" =~ ^[Yy] ]] && INSTALL_MYSQL=true

        read -rp "  Pasang PHP 8.3 FastCGI (FPM) & ekstensi umum? [Y/n]: " yn_php
        [[ "${yn_php:-Y}" =~ ^[Yy] ]] && INSTALL_PHP=true

        read -rp "  Pasang Node.js Runtime? [y/N]: " yn_node
        [[ "${yn_node:-N}" =~ ^[Yy] ]] && INSTALL_NODE=true
        ;;
    *)
        echo -e "${YELLOW}Pilihan tidak valid, menggunakan standar Full LNMP.${NC}"
        INSTALL_NGINX=true
        INSTALL_MYSQL=true
        INSTALL_PHP=true
        ;;
esac

# 4. Update Repositori Paket Sistem
echo -e "\n[*] Memperbarui katalog repositori paket sistem..."
if [ "$PKG_MGR" = "apt" ]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y curl wget git tar ufw ca-certificates gnupg lsb-release
elif [ "$PKG_MGR" = "dnf" ] || [ "$PKG_MGR" = "yum" ]; then
    $PKG_MGR update -y
    $PKG_MGR install -y curl wget git tar ca-certificates gnupg epel-release
fi

# 5. Instalasi Komponen Stack yang Dipilih

# --- Nginx ---
if [ "$INSTALL_NGINX" = true ]; then
    echo -e "\n[+] Memasang Nginx Web Server..."
    if [ "$PKG_MGR" = "apt" ]; then
        apt-get install -y nginx
    else
        $PKG_MGR install -y nginx
    fi
    systemctl enable nginx
    systemctl restart nginx || true
    echo -e "${GREEN}[✓] Nginx berhasil dipasang dan dijalankan.${NC}"
fi

# --- Apache ---
if [ "$INSTALL_APACHE" = true ]; then
    echo -e "\n[+] Memasang Apache HTTP Server..."
    if [ "$PKG_MGR" = "apt" ]; then
        apt-get install -y apache2
        systemctl enable apache2
        systemctl restart apache2 || true
    else
        $PKG_MGR install -y httpd
        systemctl enable httpd
        systemctl restart httpd || true
    fi
    echo -e "${GREEN}[✓] Apache berhasil dipasang dan dijalankan.${NC}"
fi

# --- MySQL 8.0 ---
if [ "$INSTALL_MYSQL" = true ]; then
    echo -e "\n[+] Memasang MySQL 8.0 Database Server..."
    if [ "$PKG_MGR" = "apt" ]; then
        apt-get install -y mysql-server
        systemctl enable mysql
        systemctl restart mysql || true
    else
        $PKG_MGR install -y mysql-server || $PKG_MGR install -y mariadb-server
        systemctl enable mysqld || systemctl enable mariadb
        systemctl restart mysqld || systemctl restart mariadb || true
    fi
    echo -e "${GREEN}[✓] Database server berhasil dipasang dan dijalankan.${NC}"
fi

# --- PHP 8.3 FPM ---
if [ "$INSTALL_PHP" = true ]; then
    echo -e "\n[+] Memasang PHP FastCGI (FPM) & ekstensi pendukung..."
    if [ "$PKG_MGR" = "apt" ]; then
        apt-get install -y php-fpm php-mysql php-curl php-gd php-mbstring php-xml php-zip php-bcmath
        systemctl enable php8.3-fpm || systemctl enable php-fpm || true
        systemctl restart php8.3-fpm || systemctl restart php-fpm || true
    else
        $PKG_MGR install -y php-fpm php-mysqlnd php-gd php-curl php-mbstring php-xml php-zip
        systemctl enable php-fpm
        systemctl restart php-fpm || true
    fi
    echo -e "${GREEN}[✓] PHP FPM berhasil dipasang dan dijalankan.${NC}"
fi

# --- Node.js ---
if [ "$INSTALL_NODE" = true ]; then
    echo -e "\n[+] Memasang Node.js Runtime..."
    if [ "$PKG_MGR" = "apt" ]; then
        curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
        apt-get install -y nodejs
    else
        curl -fsSL https://rpm.nodesource.com/setup_20.x | bash -
        $PKG_MGR install -y nodejs
    fi
    echo -e "${GREEN}[✓] Node.js $(node -v 2>/dev/null || true) berhasil dipasang.${NC}"
fi

# 6. Pemasangan Biner FikPanel & Konfigurasi Systemd
echo -e "\n[*] Memasang FikPanel Server Control Panel..."
INSTALL_DIR="/opt/fikpanel"
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/data"
mkdir -p "$INSTALL_DIR/bin"
mkdir -p "/var/www/html"

# Salin biner atau kompilasi jika Go terpasang, atau unduh rilis terbaru
if which go >/dev/null 2>&1 && [ -f "./cmd/fikpanel/main.go" ]; then
    echo "[*] Mengompilasi biner dari kode sumber lokal..."
    go build -ldflags="-s -w" -o "$INSTALL_DIR/bin/fikpanel" ./cmd/fikpanel
elif [ -f "./bin/fikpanel" ]; then
    cp "./bin/fikpanel" "$INSTALL_DIR/bin/fikpanel"
    chmod +x "$INSTALL_DIR/bin/fikpanel"
else
    # Fallback: Unduh biner rilis GitHub FikPanel
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) ARCH="amd64" ;;
    esac
    echo "[*] Mengunduh rilis FikPanel biner untuk linux-${ARCH}..."
    curl -fsSL "https://github.com/FikFikk/fikpanel/releases/latest/download/fikpanel-linux-${ARCH}.tar.gz" -o /tmp/fikpanel.tar.gz || true
    if [ -f /tmp/fikpanel.tar.gz ]; then
        tar -xzf /tmp/fikpanel.tar.gz -C "$INSTALL_DIR/bin/"
        chmod +x "$INSTALL_DIR/bin/fikpanel"
        rm -f /tmp/fikpanel.tar.gz
    fi
fi

# 7. Membuat Systemd Unit Service
cat <<EOF > /etc/systemd/system/fikpanel.service
[Unit]
Description=FikPanel Server Control Panel Daemon
After=network.target network-online.target nginx.service
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/fikpanel serve --port 8443 --db ${INSTALL_DIR}/data/fikpanel.db
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable fikpanel.service
systemctl restart fikpanel.service || true

# 8. Pindai dan Adopsi Otomatis Seluruh Infrastruktur Host (Nginx, Docker Compose, PM2, K8s)
echo -e "\n[*] Menjalankan Host Infrastructure Auto-Discovery & Reverse Adoption..."
if [ -x "$INSTALL_DIR/bin/fikpanel" ]; then
    "$INSTALL_DIR/bin/fikpanel" discover --db "${INSTALL_DIR}/data/fikpanel.db" --adopt || true
fi

# 9. Firewall Helper (Buka port 8443, 80, 443)
if which ufw >/dev/null 2>&1; then
    ufw allow 8443/tcp comment 'FikPanel Control Panel' || true
    ufw allow 80/tcp comment 'HTTP Web' || true
    ufw allow 443/tcp comment 'HTTPS Web' || true
fi

# 10. Informasi Akses Selesai
PUBLIC_IP=$(curl -s4 https://api.ipify.org || hostname -I | awk '{print $1}')
echo -e "\n=============================================================="
echo -e "${GREEN}${BOLD}✓ Instalasi FikPanel Berhasil Diselesaikan!${NC}"
echo -e "=============================================================="
echo -e "Alamat Akses Panel : ${CYAN}${BOLD}https://${PUBLIC_IP}:8443${NC}"
echo -e "                    (atau http://${PUBLIC_IP}:8443)"
echo -e "Username Standar   : ${BOLD}root${NC}"
echo -e "Password Standar   : ${YELLOW}(Dibuat saat login pertama atau kosong)${NC}"
echo -e "Status Layanan     : $(systemctl is-active fikpanel.service 2>/dev/null || echo 'unknown')"
echo -e "Direktori Berkas   : ${INSTALL_DIR}"
echo -e "==============================================================\n"
