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

# 3. Pre-Deteksi Software yang Sudah Terpasang di Host Server
# Mencegah penimpaan konfigurasi pada server yang sudah memiliki proyek berjalan
HAS_NGINX=false; NGINX_VER=""
if which nginx >/dev/null 2>&1; then
    HAS_NGINX=true
    NGINX_VER=$(nginx -v 2>&1 | awk -F'/' '{print $2}' || echo "terpasang")
fi

HAS_APACHE=false; APACHE_VER=""
if which apache2 >/dev/null 2>&1; then
    HAS_APACHE=true
    APACHE_VER=$(apache2 -v 2>&1 | awk -F'/' 'NR==1{print $2}' | awk '{print $1}' || echo "terpasang")
elif which httpd >/dev/null 2>&1; then
    HAS_APACHE=true
    APACHE_VER=$(httpd -v 2>&1 | awk -F'/' 'NR==1{print $2}' | awk '{print $1}' || echo "terpasang")
fi

HAS_MYSQL=false; MYSQL_VER=""
if which mysql >/dev/null 2>&1 || which mysqld >/dev/null 2>&1; then
    HAS_MYSQL=true
    MYSQL_VER=$(mysql --version 2>/dev/null | awk '{print $5}' | tr -d ',' || echo "terpasang")
fi

HAS_PHP=false; PHP_VER=""
if which php >/dev/null 2>&1; then
    HAS_PHP=true
    PHP_VER=$(php -v 2>/dev/null | awk 'NR==1{print $2}' || echo "terpasang")
fi

HAS_NODE=false; NODE_VER=""
if which node >/dev/null 2>&1; then
    HAS_NODE=true
    NODE_VER=$(node -v 2>/dev/null || echo "terpasang")
fi

HAS_DOCKER=false; DOCKER_VER=""
if which docker >/dev/null 2>&1; then
    HAS_DOCKER=true
    DOCKER_VER=$(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo "terpasang")
fi

HAS_FAIL2BAN=false; FAIL2BAN_VER=""
if which fail2ban-client >/dev/null 2>&1; then
    HAS_FAIL2BAN=true
    FAIL2BAN_VER=$(fail2ban-client version 2>/dev/null || echo "terpasang")
fi

# Tampilkan ringkasan deteksi status software pada host
echo -e "\n${BOLD}==============================================================${NC}"
echo -e "${BOLD}            STATUS DETEKSI SOFTWARE HOST SERVER               ${NC}"
echo -e "${BOLD}==============================================================${NC}"
echo -e "  [${GREEN}WAJIB${NC}] FikPanel Core & SQLite  : ${GREEN}Akan Dipasang${NC} (Mandiri, tanpa MySQL)"

if [ "$HAS_NGINX" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  Nginx Web Server        : ${GREEN}SUDAH ADA${NC} (v${NGINX_VER} - Tidak akan ditimpa)"
elif [ "$HAS_APACHE" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  Apache Web Server       : ${GREEN}SUDAH ADA${NC} (v${APACHE_VER} - Tidak akan ditimpa)"
else
    echo -e "  [${CYAN}OPSI${NC}]  Nginx Web Server        : ${YELLOW}Belum Ada${NC} (Bisa dipasang sekarang/nanti)"
fi

if [ "$HAS_MYSQL" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  MySQL / MariaDB Server  : ${GREEN}SUDAH ADA${NC} (${MYSQL_VER} - Data aman)"
else
    echo -e "  [${CYAN}OPSI${NC}]  MySQL / MariaDB Server  : ${YELLOW}Belum Ada${NC} (Opsional, untuk database web)"
fi

if [ "$HAS_PHP" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  PHP FastCGI (FPM)       : ${GREEN}SUDAH ADA${NC} (v${PHP_VER})"
else
    echo -e "  [${CYAN}OPSI${NC}]  PHP FastCGI (FPM)       : ${YELLOW}Belum Ada${NC} (Opsional, untuk WordPress/PHP)"
fi

if [ "$HAS_NODE" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  Node.js & PM2           : ${GREEN}SUDAH ADA${NC} (${NODE_VER})"
else
    echo -e "  [${CYAN}OPSI${NC}]  Node.js & PM2           : ${YELLOW}Belum Ada${NC} (Opsional, untuk aplikasi JS)"
fi

if [ "$HAS_DOCKER" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  Docker Engine           : ${GREEN}SUDAH ADA${NC} (${DOCKER_VER})"
else
    echo -e "  [${CYAN}OPSI${NC}]  Docker Engine           : ${YELLOW}Belum Ada${NC} (Opsional, untuk container)"
fi

if [ "$HAS_FAIL2BAN" = true ]; then
    echo -e "  [${CYAN}OPSI${NC}]  Fail2ban Security       : ${GREEN}SUDAH ADA${NC} (${FAIL2BAN_VER})"
else
    echo -e "  [${CYAN}OPSI${NC}]  Fail2ban Security       : ${YELLOW}Belum Ada${NC} (Opsional, bisa via Web Panel)"
fi
echo -e "${BOLD}==============================================================${NC}"
echo -e "${CYAN}Catatan Penting:${NC}"
echo -e "• ${BOLD}FikPanel TIDAK membutuhkan MySQL${NC} untuk beroperasi (FikPanel memakai embedded SQLite)."
echo -e "• MySQL, PHP, Docker, dan Fail2ban bersifat ${BOLD}OPSIONAL${NC}."
echo -e "• Komponen yang tidak dipasang sekarang ${BOLD}bisa dipasang kapan saja secara 1-klik${NC}"
echo -e "  melalui Web Panel FikPanel (menu App Store, Databases, atau Keamanan)."
echo -e "• Komponen yang sudah terdeteksi di server ${BOLD}TIDAK AKAN DITIMPA${NC} agar konfigurasi aman.\n"

# Fungsi pembaca input interaktif yang handal saat dijalankan via pipe (curl | sudo bash)
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

# 4. Menu Interaktif Pemilihan Software Stack
echo -e "${BOLD}Pilih Profil Pemasangan Software Stack:${NC}"
echo -e "  ${GREEN}1) Full LNMP Stack${NC} (Nginx + MySQL 8.0 + PHP FPM)"
echo -e "     -> Cocok untuk server baru / AWS EC2 kosong untuk WordPress/Laravel."
echo -e "  ${CYAN}2) Node.js & Web Server${NC} (Nginx + Node.js 20 + PM2)"
echo -e "     -> Cocok untuk server aplikasi SSR / Express / Next.js / Nuxt."
echo -e "  ${BLUE}3) Reverse Proxy Saja${NC} (Hanya Nginx Web Server)"
echo -e "     -> Cocok jika backend berjalan di port aplikasi atau container Docker."
echo -e "  ${BOLD}4) Minimal FikPanel Core${NC} (Hanya Panel & SQLite — Sangat Ringan)"
echo -e "     -> MySQL, PHP, Docker, Fail2ban bisa dipasang nanti dari antarmuka Web Panel."
echo -e "  ${YELLOW}5) Kustom / Pilih Sendiri${NC} (Tanya satu-per-satu per komponen)"

prompt_read "Masukkan nomor pilihan [1-5] (default: 1): " "1" STACK_CHOICE

INSTALL_NGINX=false
INSTALL_APACHE=false
INSTALL_MYSQL=false
INSTALL_PHP=false
INSTALL_NODE=false
INSTALL_DOCKER=false
INSTALL_FAIL2BAN=false

case "$STACK_CHOICE" in
    1)
        INSTALL_NGINX=true
        INSTALL_MYSQL=true
        INSTALL_PHP=true
        ;;
    2)
        INSTALL_NGINX=true
        INSTALL_NODE=true
        ;;
    3)
        INSTALL_NGINX=true
        ;;
    4)
        # Hanya pasang panel FikPanel core
        echo -e "${BLUE}[*] Memilih mode Minimal FikPanel Core. Komponen lain dapat dipasang dari Web Panel kapan saja.${NC}"
        ;;
    5)
        echo -e "\n${CYAN}Konfigurasi Pilihan Kustom Komponen:${NC}"
        if [ "$HAS_NGINX" = true ] || [ "$HAS_APACHE" = true ]; then
            echo -e "  [i] Web server sudah terdeteksi di sistem. Melewati pertanyaan instalasi web server."
        else
            prompt_read "  Pasang Nginx Web Server? [Y/n]: " "Y" yn_nginx
            [[ "$yn_nginx" =~ ^[Yy] ]] && INSTALL_NGINX=true
        fi

        if [ "$HAS_MYSQL" = true ]; then
            echo -e "  [i] MySQL/MariaDB sudah terdeteksi di sistem. Melewati pertanyaan instalasi MySQL."
        else
            prompt_read "  Pasang MySQL 8.0 Database Server? (Bukan untuk panel, hanya untuk web app) [y/N]: " "N" yn_mysql
            [[ "$yn_mysql" =~ ^[Yy] ]] && INSTALL_MYSQL=true
        fi

        if [ "$HAS_PHP" = true ]; then
            echo -e "  [i] PHP sudah terdeteksi di sistem. Melewati pertanyaan instalasi PHP."
        else
            prompt_read "  Pasang PHP FastCGI (FPM) & ekstensi umum? [y/N]: " "N" yn_php
            [[ "$yn_php" =~ ^[Yy] ]] && INSTALL_PHP=true
        fi

        if [ "$HAS_NODE" = true ]; then
            echo -e "  [i] Node.js sudah terdeteksi di sistem. Melewati pertanyaan instalasi Node.js."
        else
            prompt_read "  Pasang Node.js Runtime & PM2? [y/N]: " "N" yn_node
            [[ "$yn_node" =~ ^[Yy] ]] && INSTALL_NODE=true
        fi

        if [ "$HAS_DOCKER" = true ]; then
            echo -e "  [i] Docker Engine sudah terdeteksi di sistem. Melewati pertanyaan instalasi Docker."
        else
            prompt_read "  Pasang Docker Engine & Compose? [y/N]: " "N" yn_docker
            [[ "$yn_docker" =~ ^[Yy] ]] && INSTALL_DOCKER=true
        fi

        if [ "$HAS_FAIL2BAN" = true ]; then
            echo -e "  [i] Fail2ban sudah terdeteksi di sistem. Melewati pertanyaan instalasi Fail2ban."
        else
            prompt_read "  Pasang Fail2ban Intrusion Prevention? (Bisa diaktifkan nanti dari Web Panel) [y/N]: " "N" yn_f2b
            [[ "$yn_f2b" =~ ^[Yy] ]] && INSTALL_FAIL2BAN=true
        fi
        ;;
    *)
        echo -e "${YELLOW}Pilihan tidak valid, menggunakan standar Full LNMP.${NC}"
        INSTALL_NGINX=true
        INSTALL_MYSQL=true
        INSTALL_PHP=true
        ;;
esac

# 5. Update Repositori Paket Sistem
echo -e "\n[*] Memperbarui katalog repositori paket sistem..."
if [ "$PKG_MGR" = "apt" ]; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y curl wget git tar ufw ca-certificates gnupg lsb-release
elif [ "$PKG_MGR" = "dnf" ] || [ "$PKG_MGR" = "yum" ]; then
    $PKG_MGR update -y
    $PKG_MGR install -y curl wget git tar ca-certificates gnupg epel-release
fi

# 6. Instalasi Komponen Stack yang Dipilih (Dengan Proteksi Smart Skip)

# --- Nginx ---
if [ "$INSTALL_NGINX" = true ]; then
    if [ "$HAS_NGINX" = true ]; then
        echo -e "\n${GREEN}[✓] Nginx sudah terpasang (v${NGINX_VER}). Melewati instalasi agar konfigurasi vhost existing tidak tertimpa.${NC}"
    else
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
fi

# --- Apache ---
if [ "$INSTALL_APACHE" = true ]; then
    if [ "$HAS_APACHE" = true ]; then
        echo -e "\n${GREEN}[✓] Apache sudah terpasang. Melewati instalasi agar konfigurasi existing aman.${NC}"
    else
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
fi

# --- MySQL 8.0 ---
if [ "$INSTALL_MYSQL" = true ]; then
    if [ "$HAS_MYSQL" = true ]; then
        echo -e "\n${GREEN}[✓] MySQL/MariaDB sudah terpasang (${MYSQL_VER}). Melewati instalasi agar database & data existing aman.${NC}"
    else
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
fi

# --- PHP FastCGI (FPM) ---
if [ "$INSTALL_PHP" = true ]; then
    if [ "$HAS_PHP" = true ]; then
        echo -e "\n${GREEN}[✓] PHP sudah terpasang (v${PHP_VER}). Melewati instalasi.${NC}"
    else
        echo -e "\n[+] Memasang PHP FastCGI (FPM) & ekstensi pendukung..."
        if [ "$PKG_MGR" = "apt" ]; then
            apt-get install -y php-fpm php-mysql php-curl php-gd php-mbstring php-xml php-zip php-bcmath
            # Deteksi unit service PHP FPM dinamis dari sistem (misal php8.5-fpm atau php8.3-fpm)
            PHP_FPM_SVC=$(systemctl list-unit-files --type=service 2>/dev/null | grep -E '^php[0-9.]*-fpm\.service' | awk '{print $1}' | head -n 1 || true)
            [ -z "$PHP_FPM_SVC" ] && PHP_FPM_SVC="php-fpm.service"
            systemctl enable "$PHP_FPM_SVC" || true
            systemctl restart "$PHP_FPM_SVC" || true
        else
            $PKG_MGR install -y php-fpm php-mysqlnd php-gd php-curl php-mbstring php-xml php-zip
            systemctl enable php-fpm || true
            systemctl restart php-fpm || true
        fi
        echo -e "${GREEN}[✓] PHP FPM berhasil dipasang dan dijalankan.${NC}"
    fi
fi

# --- Node.js & PM2 ---
if [ "$INSTALL_NODE" = true ]; then
    if [ "$HAS_NODE" = true ]; then
        echo -e "\n${GREEN}[✓] Node.js sudah terpasang (${NODE_VER}). Melewati instalasi runtime.${NC}"
    else
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
    # Pastikan PM2 terpasang untuk manajemen proses Node.js
    if ! which pm2 >/dev/null 2>&1; then
        echo "[*] Memasang PM2 process manager secara global..."
        npm install -g pm2 || true
    fi
fi

# --- Docker Engine ---
if [ "$INSTALL_DOCKER" = true ]; then
    if [ "$HAS_DOCKER" = true ]; then
        echo -e "\n${GREEN}[✓] Docker Engine sudah terpasang (${DOCKER_VER}). Melewati instalasi.${NC}"
    else
        echo -e "\n[+] Memasang Docker Engine & Docker Compose Plugin..."
        if [ "$PKG_MGR" = "apt" ]; then
            apt-get install -y docker.io docker-compose-plugin || curl -fsSL https://get.docker.com | sh
        else
            $PKG_MGR install -y docker-ce docker-ce-cli containerd.io || curl -fsSL https://get.docker.com | sh
        fi
        systemctl enable docker || true
        systemctl restart docker || true
        echo -e "${GREEN}[✓] Docker Engine berhasil dipasang dan dijalankan.${NC}"
    fi
fi

# --- Fail2ban ---
if [ "$INSTALL_FAIL2BAN" = true ]; then
    if [ "$HAS_FAIL2BAN" = true ]; then
        echo -e "\n${GREEN}[✓] Fail2ban sudah terpasang. Melewati instalasi.${NC}"
    else
        echo -e "\n[+] Memasang Fail2ban Intrusion Prevention..."
        if [ "$PKG_MGR" = "apt" ]; then
            apt-get install -y fail2ban
        else
            $PKG_MGR install -y fail2ban
        fi
        systemctl enable fail2ban || true
        systemctl restart fail2ban || true
        echo -e "${GREEN}[✓] Fail2ban berhasil dipasang dan diaktifkan.${NC}"
    fi
fi

# 7. Pemasangan Biner FikPanel & Konfigurasi Direktori
echo -e "\n[*] Memasang FikPanel Server Control Panel..."
INSTALL_DIR="/opt/fikpanel"
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/data"
mkdir -p "$INSTALL_DIR/bin"
mkdir -p "$INSTALL_DIR/scripts"
mkdir -p "/var/www/html"

# Salin skrip update & uninstall lokal atau unduh dari repositori
if [ -f "./scripts/update.sh" ]; then
    cp "./scripts/update.sh" "$INSTALL_DIR/scripts/update.sh"
    chmod +x "$INSTALL_DIR/scripts/update.sh"
else
    curl -fsSL "https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/update.sh" -o "$INSTALL_DIR/scripts/update.sh" || true
    chmod +x "$INSTALL_DIR/scripts/update.sh" 2>/dev/null || true
fi

if [ -f "./scripts/uninstall.sh" ]; then
    cp "./scripts/uninstall.sh" "$INSTALL_DIR/scripts/uninstall.sh"
    chmod +x "$INSTALL_DIR/scripts/uninstall.sh"
else
    curl -fsSL "https://raw.githubusercontent.com/FikFikk/fikpanel/main/scripts/uninstall.sh" -o "$INSTALL_DIR/scripts/uninstall.sh" || true
    chmod +x "$INSTALL_DIR/scripts/uninstall.sh" 2>/dev/null || true
fi

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

# Verifikasi keberadaan biner FikPanel
if [ ! -x "$INSTALL_DIR/bin/fikpanel" ]; then
    echo -e "${RED}[ERROR] Biner FikPanel tidak ditemukan atau gagal dipasang di $INSTALL_DIR/bin/fikpanel.${NC}"
    exit 1
fi
echo -e "${GREEN}[✓] Biner FikPanel terpasang: $INSTALL_DIR/bin/fikpanel${NC}"
ln -sf "$INSTALL_DIR/bin/fikpanel" /usr/local/bin/fikpanel || true

# 8. Membuat Systemd Unit Service
cat <<EOF > /etc/systemd/system/fikpanel.service
[Unit]
Description=FikPanel Server Control Panel Daemon
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/fikpanel serve --port 1405 --db ${INSTALL_DIR}/data/fikpanel.db
Restart=always
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable fikpanel.service
systemctl restart fikpanel.service || true
sleep 1

if systemctl is-active --quiet fikpanel.service; then
    echo -e "${GREEN}[✓] Layanan daemon FikPanel aktif dan berjalan normal.${NC}"
else
    echo -e "${YELLOW}[!] Peringatan: Layanan FikPanel belum aktif. Periksa log: journalctl -u fikpanel -n 20 --no-pager${NC}"
fi

# 9. Pindai dan Adopsi Otomatis Seluruh Infrastruktur Host (Nginx, Docker Compose, PM2, K8s)
echo -e "\n[*] Menjalankan Host Infrastructure Auto-Discovery & Reverse Adoption..."
if [ -x "$INSTALL_DIR/bin/fikpanel" ]; then
    "$INSTALL_DIR/bin/fikpanel" discover --db "${INSTALL_DIR}/data/fikpanel.db" --adopt || true
fi

# 10. Firewall Helper (Buka port 1405, 80, 443)
if which ufw >/dev/null 2>&1; then
    ufw allow 1405/tcp comment 'FikPanel Control Panel' || true
    ufw allow 80/tcp comment 'HTTP Web' || true
    ufw allow 443/tcp comment 'HTTPS Web' || true
elif which firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld; then
    firewall-cmd --permanent --add-port=1405/tcp || true
    firewall-cmd --permanent --add-port=80/tcp || true
    firewall-cmd --permanent --add-port=443/tcp || true
    firewall-cmd --reload || true
fi

# 11. Informasi Akses Selesai
PRIVATE_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "127.0.0.1")
PUBLIC_IP=$(curl -s4 --max-time 3 https://api.ipify.org 2>/dev/null || echo "$PRIVATE_IP")

echo -e "\n=============================================================="
echo -e "${GREEN}${BOLD}✓ Instalasi FikPanel Berhasil Diselesaikan!${NC}"
echo -e "=============================================================="
if [ -n "$PRIVATE_IP" ] && [ "$PRIVATE_IP" != "127.0.0.1" ]; then
    echo -e "Akses IP Privat    : ${CYAN}${BOLD}http://${PRIVATE_IP}:1405${NC} (atau https://${PRIVATE_IP}:1405)"
fi
echo -e "Akses IP Publik    : ${CYAN}${BOLD}http://${PUBLIC_IP}:1405${NC} (atau https://${PUBLIC_IP}:1405)"
echo -e "Username Standar   : ${BOLD}root${NC}"
echo -e "Password Standar   : ${YELLOW}(Dibuat saat login pertama atau kosong)${NC}"
echo -e "Status Layanan     : $(systemctl is-active fikpanel.service 2>/dev/null || echo 'unknown')"
echo -e "Direktori Berkas   : ${INSTALL_DIR}"
echo -e "==============================================================\n"
