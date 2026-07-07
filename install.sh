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

detect_data_dir() {
    if [[ -n "${MONETARIUM_HOME:-}" ]]; then
        echo "$MONETARIUM_HOME"
        return
    fi
    local home="$HOME"
    case "$(uname -s)" in
        Darwin) echo "${home}/Library/Application Support/Monetarium" ;;
        *)      echo "${home}/.monetarium" ;;
    esac
}
DATA_DIR="$(detect_data_dir)"
WALLET_CONF="$DATA_DIR/monetarium-wallet.conf"
NODE_CONF="$DATA_DIR/monetarium.conf"

# ctl config — written to the ctl's own appdata dir so "monetarium-ctl"
# works without --configfile on every invocation.
# Go's dcrutil.AppDataDir("monetarium-ctl", false) returns:
#   Linux:   ~/.monetarium-ctl
#   macOS:   ~/Library/Application Support/Monetarium-ctl
detect_ctl_data_dir() {
    if [[ -n "${MONETARIUM_CTL_HOME:-}" ]]; then
        echo "$MONETARIUM_CTL_HOME"
        return
    fi
    local home="$HOME"
    case "$(uname -s)" in
        Darwin) echo "${home}/Library/Application Support/Monetarium-ctl" ;;
        *)      echo "${home}/.monetarium-ctl" ;;
    esac
}
CTL_DIR="$(detect_ctl_data_dir)"
CTL_CONF="$CTL_DIR/monetarium-ctl.conf"

MANIFEST="$DATA_DIR/install.manifest"

# Mining & ticket buyer settings (set by prompts)
MINING_ENABLED=false
MINING_CORES=""
TICKETS_ENABLED=false
TICKET_LIMIT=""
TICKET_BALANCE=""
VOTING_ENABLED=false

# Wallet addresses (populated after services start)
MINING_ADDR=""
CONSOLIDATION_ADDR=""

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

has_tty() {
    { exec 3</dev/tty; } 2>/dev/null && exec 3<&- && return 0
    return 1
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
    local dest="${INSTALL_DIR}/${bin_name}"

    if [[ -f "$dest" ]]; then
        info "Skipping ${bin_name} - already installed at $dest"
        return
    fi

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
    sudo mkdir -p "$INSTALL_DIR"
    sudo mv "$found" "${INSTALL_DIR}/${bin_name}"
    rm -rf "$tmpdir"

    info "Installed ${bin_name} -> ${INSTALL_DIR}/${bin_name}"
}

# --------------------------------------------------------------------------
# 2b. Ensure INSTALL_DIR is on PATH for interactive shell use
#     (the systemd/launchd services call binaries by absolute path,
#     so this step is purely for convenient `monetarium-ctl ...` usage).
# --------------------------------------------------------------------------
path_needs_update() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) return 1 ;;
        *) return 0 ;;
    esac
}

add_install_dir_to_path() {
    local rc_file marker="# Added by monetarium install.sh"

    case "$(basename "${SHELL:-}")" in
        zsh)  rc_file="$HOME/.zshrc" ;;
        bash) rc_file="$HOME/.bashrc" ;;
        *)    rc_file="$HOME/.profile" ;;
    esac

    if ! grep -qF "$marker" "$rc_file" 2>/dev/null; then
        {
            echo ""
            echo "$marker"
            echo "export PATH=\"$INSTALL_DIR:\$PATH\""
        } >> "$rc_file"
        info "Added $INSTALL_DIR to PATH in $rc_file. Run 'source $rc_file' or open a new terminal to use it."
    else
        info "$rc_file already updates PATH for $INSTALL_DIR."
    fi

    export PATH="$INSTALL_DIR:$PATH"
}

# --------------------------------------------------------------------------
# 3. Prompt for passphrase (hidden)
# --------------------------------------------------------------------------
# In curl | bash, stdin is a pipe — interactive commands need /dev/tty
# so the user can type. In headless CI there is no /dev/tty, so read
# from stdin (the pipe the CI feeds).
needs_tty() {
    [[ ! -t 0 ]] && has_tty
}

read_tty() {
    if needs_tty; then
        read -r "$@" < /dev/tty || true
    else
        read -r "$@" || true
    fi
}

prompt_passphrase() {
    local pass1 pass2
    while true; do
        read_tty -r -s -p "Enter wallet private passphrase: " pass1
        echo
        read_tty -r -s -p "Confirm passphrase: " pass2
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
# 3b. Prompt for mining
# --------------------------------------------------------------------------
prompt_mining() {
    if ! has_tty; then
        return
    fi

    local answer
    read_tty -r -p "Enable CPU mining? (y/N): " answer
    case "$answer" in
        [yY]|[yY][eE][sS]) MINING_ENABLED=true ;;
    esac

    if $MINING_ENABLED; then
        local cpu_count recommended
        cpu_count=$(nproc 2>/dev/null || sysctl -n hw.logicalcpu 2>/dev/null || echo 2)
        recommended=$((cpu_count / 2))
        [[ "$recommended" -lt 1 ]] && recommended=1

        read_tty -r -p "You have ${cpu_count} CPU core(s). How many cores to use for mining? (recommended: ${recommended}): " answer
        MINING_CORES="${answer:-$recommended}"
        while ! [[ "$MINING_CORES" =~ ^[0-9]+$ ]]; do
            read_tty -r -p "Please enter a valid number: " answer
            MINING_CORES="${answer:-$recommended}"
        done
    fi
}

# --------------------------------------------------------------------------
# 3c. Prompt for ticket buyer
# --------------------------------------------------------------------------
prompt_ticket_buyer() {
    if ! has_tty; then
        return
    fi

    local answer
    read_tty -r -p "Enable automatic ticket purchase for staking? (y/N): " answer
    case "$answer" in
        [yY]|[yY][eE][sS]) TICKETS_ENABLED=true ;;
    esac

    if $TICKETS_ENABLED; then
        read_tty -r -p "Maximum number of tickets per purchase (ticketbuyer.limit): " answer
        TICKET_LIMIT="${answer:-5}"
        while ! [[ "$TICKET_LIMIT" =~ ^[0-9]+$ ]]; do
            read_tty -r -p "Please enter a valid number: " answer
            TICKET_LIMIT="$answer"
        done

        read_tty -r -p "Minimum VAR balance to keep in wallet (ticketbuyer.balancetomaintainabsolute): " answer
        TICKET_BALANCE="${answer:-0}"
        while ! [[ "$TICKET_BALANCE" =~ ^[0-9]+(\.[0-9]+)?$ ]]; do
            read_tty -r -p "Please enter a valid number: " answer
            TICKET_BALANCE="$answer"
        done
    fi

    # Ask about auto-voting regardless of ticket buyer answer.
    read_tty -r -p "Enable automatic voting? (Y/n): " answer
    case "$answer" in
        [nN]|[nN][oO]) VOTING_ENABLED=false ;;
        *) VOTING_ENABLED=true ;;
    esac
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
addpeer=176.113.164.216:9508
addpeer=134.249.62.43:9508
addpeer=62.216.37.206:9508
EOF

    # Wallet config — this is where the plaintext passphrase lives.
    local tb_enabled=0; $TICKETS_ENABLED && tb_enabled=1
    local vote_enabled=0; $VOTING_ENABLED && vote_enabled=1
    local tkt_limit="${TICKET_LIMIT:-1}"
    local tkt_balance="${TICKET_BALANCE:-1}"

    cat > "$WALLET_CONF" <<EOF
; monetarium-wallet.conf - generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
; WARNING: 'pass' below stores the wallet unlock passphrase in plaintext
; so the service can auto-unlock the wallet on boot for unattended
; ticket buying / voting. See the warning printed by install.sh.
username=${RPC_USER}
password=${RPC_PASS}
pass=${WALLET_PASSPHRASE}
enablevoting=${vote_enabled}
enableticketbuyer=${tb_enabled}
ticketbuyer.limit=${tkt_limit}
ticketbuyer.balancetomaintainabsolute=${tkt_balance}
; Gap limit for address discovery
gaplimit=20
accountgaplimit=10
EOF

    chmod 600 "$NODE_CONF" "$WALLET_CONF"
    chown "$(id -u):$(id -g)" "$NODE_CONF" "$WALLET_CONF"

    # ctl config — so "monetarium-ctl getinfo" works without flags.
    mkdir -p "$CTL_DIR"
    cat > "$CTL_CONF" <<EOF
; monetarium-ctl.conf - generated by install.sh on $(date -u +"%Y-%m-%dT%H:%M:%SZ")
; RPC credentials matching the node config.
rpcuser=${RPC_USER}
rpcpass=${RPC_PASS}
EOF
    chmod 600 "$CTL_CONF"
    chown "$(id -u):$(id -g)" "$CTL_CONF"

    info "Config files written to $DATA_DIR (permissions set to 600)."
}

# --------------------------------------------------------------------------
# 5. Create wallet
# --------------------------------------------------------------------------
wallet_data_dir() {
    local home="$HOME"
    case "$(uname -s)" in
        Darwin) echo "${home}/Library/Application Support/Monetarium-wallet" ;;
        Linux)  echo "${home}/.monetarium-wallet" ;;
    esac
}

create_wallet() {
    local wallet_data wallet_db
    wallet_data="$(wallet_data_dir)"
    wallet_db="${wallet_data}/mainnet/wallet.db"

    if [[ -f "$wallet_db" ]]; then
        info "Wallet database already exists at $wallet_db — skipping creation."
        return 0
    fi

    info "Creating wallet. Follow the interactive prompts."
    info "When asked, type 'yes' to use the passphrase you just entered."

    if needs_tty; then
        monetarium-wallet --create --configfile="$WALLET_CONF" < /dev/tty
    else
        monetarium-wallet --create --configfile="$WALLET_CONF"
    fi

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
# 8. Post-install: configure mining & ticket buyer
# --------------------------------------------------------------------------
configure_mining_and_voting() {
    # Always write generate status to node config, even if RPC isn't available.
    if ! has_tty; then
        local gen_value="false"
        $MINING_ENABLED && gen_value="true"
        {
            echo ""
            echo "; Mining configuration — added post-install by install.sh"
            echo "generate=${gen_value}"
        } >> "$NODE_CONF" 2>/dev/null || true
        info "Non-interactive mode — skipped wallet address polling (no TTY)."
        return
    fi

    info "Waiting for wallet RPC to become available..."

    local i=0
    until monetarium-ctl --wallet getinfo >/dev/null 2>&1; do
        i=$((i + 1))
        if [[ $i -gt 60 ]]; then
            local gen_value="false"
            $MINING_ENABLED && gen_value="true"
            {
                echo ""
                echo "; Mining configuration — added post-install by install.sh"
                echo "generate=${gen_value}"
            } >> "$NODE_CONF" 2>/dev/null || true
            info "Wallet RPC not ready after 60 seconds."
            info "Configure mining/ticket addresses manually:"
            info "  Edit ${NODE_CONF}: add miningaddr=<address> if missing"
            return
        fi
        sleep 2
    done

    green "Wallet RPC ready."

    local config_changed=false
    local gen_value="false"
    $MINING_ENABLED && gen_value="true"

    # Always get a mining address and write generate + miningaddr to node config.
    local mining_addr
    mining_addr=$(monetarium-ctl --wallet getnewaddress 2>/dev/null || true)
    {
        echo ""
        echo "; Mining configuration — added post-install by install.sh"
        echo "generate=${gen_value}"
    } >> "$NODE_CONF"
    config_changed=true
    if [[ -n "$mining_addr" ]]; then
        MINING_ADDR="$mining_addr"
        echo "miningaddr=${mining_addr}" >> "$NODE_CONF"
    fi

    # Separate address for ticket buyer fee consolidation (only when enabled).
    if $TICKETS_ENABLED; then
        local cons_addr
        cons_addr=$(monetarium-ctl --wallet getnewaddress 2>/dev/null || true)
        if [[ -n "$cons_addr" ]]; then
            CONSOLIDATION_ADDR="$cons_addr"
            monetarium-ctl --wallet setvotefeeconsolidationaddress "default" "$cons_addr" >/dev/null 2>&1 || true
        fi
    fi

    if $config_changed; then
        info "Restarting node to apply mining configuration..."
        case "$(uname -s)" in
            Linux)
                sudo systemctl restart monetarium-node
                ;;
            Darwin)
                if launchctl kickstart -k "gui/$(id -u)/com.monetarium.node" 2>/dev/null; then
                    :
                else
                    launchctl unload "$HOME/Library/LaunchAgents/com.monetarium.node.plist" 2>/dev/null || true
                    launchctl load "$HOME/Library/LaunchAgents/com.monetarium.node.plist"
                fi
                ;;
        esac

        sleep 3
        i=0
        until monetarium-ctl getinfo >/dev/null 2>&1; do
            i=$((i + 1))
            [[ $i -gt 30 ]] && break
            sleep 2
        done

        if $MINING_ENABLED && [[ -n "$MINING_CORES" ]]; then
            monetarium-ctl setgenerate true "$MINING_CORES" 2>/dev/null || true
        fi
    fi

    echo ""
    green "============================================="
    green "  Configuration Summary"
    green "============================================="
    if $MINING_ENABLED; then
        green "  Mining:             enabled (${MINING_CORES} core(s))"
    else
        green "  Mining:             disabled (generate=0)"
    fi
    if [[ -n "$MINING_ADDR" ]]; then
        green "  Mining reward addr: ${MINING_ADDR}"
    fi
    if $TICKETS_ENABLED && [[ -n "$CONSOLIDATION_ADDR" ]]; then
        green "  Ticket buying:      enabled"
        green "  Ticket limit:       ${TICKET_LIMIT}"
        green "  Min wallet balance: ${TICKET_BALANCE}"
        green "  Fee consolidation:  ${CONSOLIDATION_ADDR}"
    else
        green "  Ticket buying:      disabled"
    fi
    if $VOTING_ENABLED; then
        green "  Auto voting:        enabled"
    else
        green "  Auto voting:        disabled"
    fi
    green "============================================="
}

# --------------------------------------------------------------------------
# 9. Install manifest
# --------------------------------------------------------------------------
write_manifest() {
    local wallet_db
    wallet_db="$(wallet_data_dir)/mainnet/wallet.db"

    cat > "$MANIFEST" <<EOF
# Install manifest — files created by monetarium install.sh
# Run date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")
#
# To uninstall, stop the services first:
#   Linux:  sudo systemctl disable --now monetarium-node monetarium-wallet
#   macOS:  launchctl unload ~/Library/LaunchAgents/com.monetarium.node.plist
#           launchctl unload ~/Library/LaunchAgents/com.monetarium.wallet.plist
#
# Then remove the files listed below.
# ⚠️  Back up and remove the wallet database FIRST if it holds funds!
#     The wallet seed was shown during installation — without it the
#     wallet.db is unrecoverable.

binaries:
  ${INSTALL_DIR}/monetarium-node
  ${INSTALL_DIR}/monetarium-wallet
  ${INSTALL_DIR}/monetarium-ctl

configs:
  ${NODE_CONF}
  ${WALLET_CONF}
  ${CTL_CONF}

wallet database (back up seed before removing):
  ${wallet_db}

data directories:
  ${DATA_DIR}
  ${CTL_DIR}

EOF

    {
        echo "mining:"
        if $MINING_ENABLED; then
            echo "  enabled:  true"
            echo "  cores:    ${MINING_CORES}"
        else
            echo "  enabled:  false"
        fi
        if [[ -n "$MINING_ADDR" ]]; then
            echo "  address:  ${MINING_ADDR}"
        fi
        echo ""
        echo "ticket buyer:"
        if $TICKETS_ENABLED; then
            echo "  enabled:  true"
            echo "  limit:    ${TICKET_LIMIT:-1}"
            echo "  balance:  ${TICKET_BALANCE:-1}"
        else
            echo "  enabled:  false"
        fi
        if [[ -n "$CONSOLIDATION_ADDR" ]]; then
            echo "  fee consolidation addr:   ${CONSOLIDATION_ADDR}"
        fi
        echo ""
    } >> "$MANIFEST"

    case "$(uname -s)" in
        Linux)
            cat >> "$MANIFEST" <<EOF
systemd service units:
  /etc/systemd/system/monetarium-node.service
  /etc/systemd/system/monetarium-wallet.service
EOF
            ;;
        Darwin)
            cat >> "$MANIFEST" <<EOF
launchd plists:
  ${HOME}/Library/LaunchAgents/com.monetarium.node.plist
  ${HOME}/Library/LaunchAgents/com.monetarium.wallet.plist
EOF
            ;;
    esac

    # Shell rc file that was modified (if any) — detect by marker
    local rc_file marker="# Added by monetarium install.sh"
    case "$(basename "${SHELL:-}")" in
        zsh)  rc_file="$HOME/.zshrc" ;;
        bash) rc_file="$HOME/.bashrc" ;;
        *)    rc_file="$HOME/.profile" ;;
    esac
    if grep -qF "$marker" "$rc_file" 2>/dev/null; then
        echo "shell rc (PATH addition):" >> "$MANIFEST"
        echo "  ${rc_file}" >> "$MANIFEST"
    fi

    echo "" >> "$MANIFEST"
    echo "This manifest: ${MANIFEST}" >> "$MANIFEST"

    info "Install manifest written to $MANIFEST"
}

# --------------------------------------------------------------------------
# 9. Big warning
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

    if path_needs_update; then
        add_install_dir_to_path
    fi

    prompt_passphrase

    prompt_mining
    prompt_ticket_buyer

    write_configs
    create_wallet

    case "$(uname -s)" in
        Linux)  install_systemd_service ;;
        Darwin) install_launchd_service ;;
    esac

    unset WALLET_PASSPHRASE

    configure_mining_and_voting

    green "Installation complete."
    write_manifest
    print_warning
}

main "$@"
