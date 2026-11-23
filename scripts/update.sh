#!/usr/bin/env bash
# ==============================================================================
# FikPanel — Update Bootstrapper
#
# Skrip ini TIDAK memasang biner sendiri. Tugasnya hanya mengunduh biner rilis
# stabil terbaru, memverifikasi checksum SHA-256-nya, lalu menyerahkan proses ke
# updater bawaan biner tersebut (`fikpanel update`), yang menangani:
#   - verifikasi checksum paket target, uji eksekusi biner baru
#   - cadangan biner + database, penggantian biner atomik
#   - restart fikpanel.service + health check, rollback otomatis bila gagal
#
# Dipakai oleh:
#   - Server lama (<= v0.3.19) yang menjalankan `fikpanel update`
#   - Pemulihan manual: curl -fsSL .../scripts/update.sh | sudo bash -s -- [versi] [--force]
#
# Penanda kompatibilitas (jangan dihapus): FIKPANEL_UPDATE_BOOTSTRAP
# ==============================================================================

set -euo pipefail

REPO="FikFikk/fikpanel"
INSTALL_DIR="/opt/fikpanel"
STAGE_DIR="$INSTALL_DIR/data/update/bootstrap"
LOCAL_SCRIPT="$INSTALL_DIR/scripts/update.sh"
SCRIPT_URL="https://raw.githubusercontent.com/$REPO/main/scripts/update.sh"
TAG_REGEX='^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$'

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

die() {
    echo -e "${RED}[ERROR] $*${NC}" >&2
    exit 1
}

info() {
    echo -e "[*] $*"
}

# Tentukan tag rilis stabil terbaru dari redirect GitHub (tanpa GitHub API, bebas rate-limit).
resolve_latest_tag() {
    local url
    url=$(curl -fsS -o /dev/null -w '%{redirect_url}' --connect-timeout 15 --max-time 30 \
        "https://github.com/$REPO/releases/latest") || return 1
    case "$url" in
        */releases/tag/*) printf '%s\n' "${url##*/releases/tag/}" ;;
        *) return 1 ;;
    esac
}

# Perbarui salinan lokal skrip ini secara atomik (tulis ke berkas sementara lalu rename),
# sehingga proses bash yang sedang membaca berkas lama tidak terganggu.
refresh_local_script() {
    [ -d "$INSTALL_DIR/scripts" ] || return 0
    local tmp
    tmp=$(mktemp "$INSTALL_DIR/scripts/.update.sh.XXXXXX") || return 0
    if curl -fsSL --connect-timeout 10 --max-time 30 "$SCRIPT_URL" -o "$tmp" \
        && grep -q 'FIKPANEL_UPDATE_BOOTSTRAP' "$tmp" \
        && bash -n "$tmp"; then
        chmod 755 "$tmp"
        mv -f "$tmp" "$LOCAL_SCRIPT"
    else
        rm -f "$tmp"
    fi
}

main() {
    local target="" force=""
    for arg in "$@"; do
        case "$arg" in
            --force|-f) force="--force" ;;
            -*) die "Opsi tidak dikenal: $arg" ;;
            *)
                if [ -z "$target" ]; then
                    target="$arg"
                fi
                ;;
        esac
    done
    target="${target:-${TARGET_VERSION:-latest}}"
    [ "${FORCE:-0}" = "1" ] && force="--force"

    [ "$(id -u)" -eq 0 ] || die "Skrip pembaruan wajib dijalankan sebagai root (gunakan sudo)."
    [ -d "$INSTALL_DIR" ] || die "Direktori FikPanel ($INSTALL_DIR) tidak ditemukan. Apakah FikPanel sudah terpasang?"
    for bin in curl tar sha256sum awk; do
        command -v "$bin" >/dev/null 2>&1 || die "Perintah '$bin' tidak ditemukan di sistem."
    done

    # Cegah rekursi: biner lama memanggil skrip ini, skrip ini memanggil biner baru.
    if [ "${FIKPANEL_UPDATE_BOOTSTRAP:-}" = "1" ]; then
        die "Bootstrapper terpanggil secara rekursif; dibatalkan tanpa perubahan."
    fi
    export FIKPANEL_UPDATE_BOOTSTRAP=1

    echo -e "${CYAN}${BOLD}FikPanel — Update Bootstrapper${NC}  ($(date '+%Y-%m-%d %H:%M:%S'))"

    local arch
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) die "Arsitektur $(uname -m) tidak didukung oleh rilis FikPanel." ;;
    esac

    local latest_tag tag
    latest_tag=$(resolve_latest_tag) || die "Gagal menentukan rilis stabil terbaru dari GitHub. Periksa koneksi internet."
    [[ "$latest_tag" =~ $TAG_REGEX ]] || die "Tag rilis terbaru tidak valid: $latest_tag"

    if [ "$target" = "latest" ]; then
        tag="$latest_tag"
    else
        tag="$target"
        [[ "$tag" == v* ]] || tag="v$tag"
        [[ "$tag" =~ $TAG_REGEX ]] || die "Format versi tidak valid: $target (contoh: v0.3.20)"
    fi

    local installed=""
    if [ -x "$INSTALL_DIR/bin/fikpanel" ]; then
        installed=$("$INSTALL_DIR/bin/fikpanel" version 2>/dev/null | awk '{print $2}' || true)
    fi
    info "Versi terpasang : ${YELLOW}${installed:-tidak diketahui}${NC}"
    info "Target          : ${GREEN}${tag}${NC} (rilis stabil terbaru: ${latest_tag})"

    # Pastikan fallback skrip lokal pada biner lama juga memakai bootstrapper ini, bukan skrip lama.
    refresh_local_script

    # Unduh dan verifikasi biner rilis terbaru; updater di dalamnya dipakai untuk memasang target.
    local asset="fikpanel-linux-${arch}.tar.gz"
    local base="https://github.com/$REPO/releases/download/$latest_tag"
    rm -rf "$STAGE_DIR"
    mkdir -p "$STAGE_DIR"
    chmod 700 "$STAGE_DIR"

    info "Mengunduh SHA256SUMS.txt dan $asset ($latest_tag)..."
    curl -fsSL --connect-timeout 20 --retry 3 "$base/SHA256SUMS.txt" -o "$STAGE_DIR/SHA256SUMS.txt" \
        || die "SHA256SUMS.txt untuk $latest_tag belum tersedia. Jika tag baru di-push, tunggu 2-3 menit lalu coba lagi."
    curl -fL --progress-bar --connect-timeout 20 --retry 3 "$base/$asset" -o "$STAGE_DIR/$asset" \
        || die "Gagal mengunduh $asset untuk $latest_tag."

    local expected actual
    expected=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1; exit }' "$STAGE_DIR/SHA256SUMS.txt")
    [ -n "$expected" ] || die "SHA256SUMS.txt tidak memuat checksum untuk $asset."
    actual=$(sha256sum "$STAGE_DIR/$asset" | awk '{print $1}')
    [ "$expected" = "$actual" ] || die "Checksum SHA-256 tidak cocok (diharapkan $expected, didapat $actual). Tidak ada perubahan."
    echo -e "${GREEN}[✓] Checksum SHA-256 cocok.${NC}"

    tar -xzf "$STAGE_DIR/$asset" -C "$STAGE_DIR" fikpanel || die "Arsip rilis tidak memuat biner fikpanel."
    chmod 755 "$STAGE_DIR/fikpanel"
    local staged_ver
    staged_ver=$("$STAGE_DIR/fikpanel" version 2>/dev/null | awk '{print $2}' || true)
    [ "$staged_ver" = "$latest_tag" ] || die "Biner unduhan melaporkan versi '${staged_ver}', bukan $latest_tag. Tidak ada perubahan."

    # Rilis lama belum memiliki updater aman; jangan pernah jatuh ke alur pemasangan lama.
    # Output ditampung dulu agar `grep -q` + pipefail tidak memberi hasil palsu akibat SIGPIPE.
    local help_out
    help_out=$("$STAGE_DIR/fikpanel" update --help 2>&1 || true)
    if ! grep -q -- '--foreground' <<<"$help_out"; then
        if [ "$installed" = "$tag" ] && [ -z "$force" ]; then
            echo -e "${GREEN}[✓] FikPanel sudah versi terbaru ($tag). Tidak ada perubahan.${NC}"
            exit 0
        fi
        die "Rilis $latest_tag belum memiliki updater aman. Tidak ada perubahan; coba lagi setelah rilis berikutnya terbit."
    fi

    echo -e "[*] Menyerahkan proses ke updater bawaan FikPanel ${latest_tag}...\n"
    if [ -n "$force" ]; then
        exec "$STAGE_DIR/fikpanel" update "$tag" --force
    fi
    exec "$STAGE_DIR/fikpanel" update "$tag"
}

# Seluruh logika berada di dalam main() dan baru dieksekusi pada baris terakhir, sehingga
# unduhan `curl | bash` yang terpotong tidak pernah menjalankan sebagian langkah.
main "$@"
