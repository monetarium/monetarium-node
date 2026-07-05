#!/usr/bin/env bash
#
# install.sh — installer for monetarium-node + monetarium-wallet + monetarium-ctl
#
# Flow:
#   1. Detect OS (Linux/macOS) and arch, download the right binaries.
#   2. Prompt for the wallet's private passphrase (hidden input).
#   3. Write that passphrase into the wallet's config file so it can
#      auto-unlock on start (needed for unattended ticket buying / voting).
#   4. Install + enable a systemd service (Linux) or launchd daemon (macOS)
#      that starts node + wallet and unlocks the wallet automatically.
#   5. Print a large, impossible-to-miss warning about what this implies.
#
# ⚠️  Read the warning at the bottom of this file (and the one the script
#     prints) before running this on a machine that holds real funds.

set -euo pipefail

# --------------------------------------------------------------------------
# Config
# --------------------------------------------------------------------------
GITHUB_ORG="monetarium"
REPO_NODE="monetarium-node"
REPO_WALLET="monetarium-wallet"
REPO_CTL="monetarium-ctl"

INSTALL_DIR="/usr/local/bin"
DATA_DIR="${MONETARIUM_HOME:-$HOME/.monetarium}"
WALLET_CONF="$DATA_DIR/monetarium-wallet.conf"
NODE_CONF="$DATA_DIR/monetarium.conf"

RPC_USER="monetarium"
RPC_PASS="$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)"

# --------------------------------------------------------------------------
# Helpers
# --------------------------------------------------------------------------
red()   { printf '\033[1;31m%s\033[0m\n' "$*"; }
green() { printf '\033[1;32m%s\033[0m\n' "$*"; }
info()  { printf '\033[1;34m[install]\033[0m %s\n' "$*"; }
die()   { red "ERROR: $*"; exit 1; }

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "Required command '$1' not found. Please install it and re-run."
}

# --------------------------------------------------------------------------
# 1. Detect OS / arch
# --------------------------------------------------------------------------
detect_platform() {
    local os arch
    case "$(uname -s)" in
        Linux)  os="linux" ;;
        Darwin) os="darwin" ;;
        *) die "Unsupported OS: $(uname -s). This script supports Linux and macOS only." ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) die "Unsupported architecture: $(uname -m)" ;;
    esac

    echo "${os}-${arch}"
}

# --------------------------------------------------------------------------
# 2. Download binaries
# --------------------------------------------------------------------------
latest_release_url() {
    # $1 = repo name, $2 = platform string (e.g. linux_amd64)
    local repo="$1" platform="$2"
    local api="https://api.github.com/repos/${GITHUB_ORG}/${repo}/releases/latest"

    local asset_url
    asset_url="$(curl -fsSL "$api" \
        | grep -o "\"browser_download_url\": *\"[^\"]*${platform}[^\"]*\"" \
        | head -n1 \
        | sed -E 's/.*"(https[^"]+)"/\1/')"

    if [[ -z "$asset_url" ]]; then
        die "Could not find a release asset for ${repo} matching platform '${platform}'. Check https://github.com/${GITHUB_ORG}/${repo}/releases manually."
    fi
    echo "$asset_url"
}

download_binary() {
    # $1 = repo, $2 = binary name to install, $3 = platform
    local repo="$1" bin_name="$2" platform="$3"
    local url tmpdir archive

    info "Fetching latest release info for ${repo}..."
    url="$(latest_release_url "$repo" "$platform")"

    tmpdir="$(mktemp -d)"
    archive="$tmpdir/$(basename "$url")"

    info "Downloading ${bin_name} from: $url"
    curl -fsSL -o "$archive" "$url"

    case "$archive" in
        *.tar.gz|*.tgz)
            tar -xzf "$archive" -C "$tmpdir"
            ;;
        *.zip)
            require_cmd unzip
            unzip -q "$archive" -d "$tmpdir"
            ;;
        *)
            # Assume it's a raw binary
            cp "$archive" "$tmpdir/$bin_name"
            ;;
    esac

    local found
    found="$(find "$tmpdir" -type f -name "$bin_name" | head -n1)"
    [[ -n "$found" ]] || die "Could not locate '${bin_name}' binary inside downloaded archive."

    chmod +x "$found"
    sudo mv "$found" "${INSTALL_DIR}/${bin_name}"
    rm -rf "$tmpdir"

    info "Installed ${bin_name} -> ${INSTALL_DIR}/${bin_name}"
}

# --------------------------------------------------------------------------
# 3. Prompt for passphrase (hidden)
# --------------------------------------------------------------------------
prompt_passphrase() {
    local pass1 pass2
    while true; do
        read -r -s -p "Enter wallet private passphrase: " pass1
        echo
        read -r -s -p "Confirm passphrase: " pass2
        echo
        if [[ -z "$pass1" ]]; then
            red "Passphrase cannot be empty. Try again."
            continue
        fi
        if [[ "$pass1" != "$pass2" ]]; then
            red "Passphrases did not match. Try again."
            continue
        fi
        WALLET_PASSPHRASE="$pass1"
        break
    done
}

# --------------------------------------------------------------------------
# 4. Write config files
# --------------------------------------------------------------------------
write_configs() {
    mkdir -p "$DATA_DIR"
    chmod 700 "$DATA_DIR"

    # Node config
    cat > "$NODE_CONF" <<EOF
; monetarium.conf - generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
rpcuser=${RPC_USER}
rpcpass=${RPC_PASS}
addpeer=134.249.62.43:9508
EOF

    # Wallet config — this is where the plaintext passphrase lives.
    cat > "$WALLET_CONF" <<EOF
; monetarium-wallet.conf - generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
; WARNING: 'pass' below stores the wallet unlock passphrase in plaintext
; so the service can auto-unlock the wallet on boot for unattended
; ticket buying / voting. See the warning printed by install.sh.
username=${RPC_USER}
password=${RPC_PASS}
pass=${WALLET_PASSPHRASE}
enablevoting=1
enableticketbuyer=1
EOF

    chmod 600 "$NODE_CONF" "$WALLET_CONF"
    chown "$(id -u):$(id -g)" "$NODE_CONF" "$WALLET_CONF"

    info "Config files written to $DATA_DIR (permissions set to 600)."
}

# --------------------------------------------------------------------------
# 5. Create wallet
# --------------------------------------------------------------------------
create_wallet() {
    local wallet_db="$HOME/.monetarium-wallet/mainnet/wallet.db"

    if [[ -f "$wallet_db" ]]; then
        info "Wallet database already exists at $wallet_db — skipping creation."
        return 0
    fi

    info "Creating wallet. Follow the interactive prompts."
    info "When asked, type 'yes' to use the passphrase you just entered."

    monetarium-wallet --create --configfile="$WALLET_CONF"

    [[ -f "$wallet_db" ]] || \
        die "Wallet creation failed — database not found at $wallet_db"

    green "Wallet created successfully."
}

# --------------------------------------------------------------------------
# 6a. systemd service (Linux)
# --------------------------------------------------------------------------
install_systemd_service() {
    local node_unit="/etc/systemd/system/monetarium-node.service"
    local wallet_unit="/etc/systemd/system/monetarium-wallet.service"
    local svc_user="${USER:-$(id -un)}"

    sudo tee "$node_unit" > /dev/null <<EOF
[Unit]
Description=Monetarium full node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${svc_user}
ExecStart=${INSTALL_DIR}/monetarium-node --configfile=${NODE_CONF}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    sudo tee "$wallet_unit" > /dev/null <<EOF
[Unit]
Description=Monetarium wallet (auto-unlock, ticket buying / voting enabled)
After=monetarium-node.service
Requires=monetarium-node.service

[Service]
Type=simple
User=${svc_user}
ExecStart=${INSTALL_DIR}/monetarium-wallet --configfile=${WALLET_CONF}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable --now monetarium-node.service
    sudo systemctl enable --now monetarium-wallet.service

    info "systemd services installed and started:"
    info "  systemctl status monetarium-node"
    info "  systemctl status monetarium-wallet"
}

# --------------------------------------------------------------------------
# 7b. launchd plist (macOS)
# --------------------------------------------------------------------------
install_launchd_service() {
    local plist_dir="$HOME/Library/LaunchAgents"
    mkdir -p "$plist_dir"

    local node_plist="$plist_dir/com.monetarium.node.plist"
    local wallet_plist="$plist_dir/com.monetarium.wallet.plist"

    cat > "$node_plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.monetarium.node</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/monetarium-node</string>
        <string>--configfile=${NODE_CONF}</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>${DATA_DIR}/node.log</string>
    <key>StandardErrorPath</key><string>${DATA_DIR}/node.err.log</string>
</dict>
</plist>
EOF

    cat > "$wallet_plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.monetarium.wallet</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/monetarium-wallet</string>
        <string>--configfile=${WALLET_CONF}</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>${DATA_DIR}/wallet.log</string>
    <key>StandardErrorPath</key><string>${DATA_DIR}/wallet.err.log</string>
</dict>
</plist>
EOF

    chmod 600 "$node_plist" "$wallet_plist"

    launchctl unload "$node_plist" >/dev/null 2>&1 || true
    launchctl unload "$wallet_plist" >/dev/null 2>&1 || true
    launchctl load "$node_plist"
    launchctl load "$wallet_plist"

    info "launchd agents installed and loaded:"
    info "  launchctl list | grep monetarium"
}

# --------------------------------------------------------------------------
# 8. Big warning
# --------------------------------------------------------------------------
print_warning() {
    red   "=============================================================="
    red   "                       !!!  WARNING  !!!"
    red   "=============================================================="
    red   " This script has written your wallet's PRIVATE PASSPHRASE in"
    red   " PLAINTEXT to:"
    red   "     ${WALLET_CONF}"
    red   ""
    red   " This is required so the wallet can auto-unlock and buy"
    red   " tickets / vote WITHOUT any further action from you. It also"
    red   " means:"
    red   "   - Anyone who can read that file (root, a backup, a"
    red   "     compromised process, a misconfigured permission) can"
    red   "     unlock your wallet and move your funds."
    red   "   - This machine is now effectively a HOT WALLET that stays"
    red   "     unlocked 24/7. There is no PIN, 2FA, or manual approval"
    red   "     step standing between an attacker and your coins."
    red   "   - If this server is compromised, funds can be drained"
    red   "     automatically, with no prompt and no warning."
    red   ""
    red   " Recommended precautions:"
    red   "   - Only put funds on this wallet that you can afford to"
    red   "     lose, sized for ticket-buying/voting purposes only."
    red   "   - Keep the bulk of your holdings in a separate, offline"
    red   "     or hardware-secured wallet, not this one."
    red   "   - Restrict SSH/root access to this machine tightly, keep"
    red   "     it patched, and monitor it actively."
    red   "   - Rotate the passphrase and re-run this script if you"
    red   "     ever suspect the machine has been compromised."
    red   "=============================================================="
}

# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------
main() {
    require_cmd curl
    require_cmd tar
    require_cmd sudo

    local platform
    platform="$(detect_platform)"
    info "Detected platform: $platform"

    download_binary "$REPO_NODE"   "monetarium-node"   "$platform"
    download_binary "$REPO_WALLET" "monetarium-wallet" "$platform"
    download_binary "$REPO_CTL"    "monetarium-ctl"    "$platform"

    prompt_passphrase
    write_configs
    create_wallet

    case "$(uname -s)" in
        Linux)  install_systemd_service ;;
        Darwin) install_launchd_service ;;
    esac

    unset WALLET_PASSPHRASE

    green "Installation complete."
    print_warning
}

main "$@"
