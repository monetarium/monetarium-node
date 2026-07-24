#!/usr/bin/env bash
# Unit tests for install.sh helper functions.
#
# Tests check_selinux() and the generate/miningaddr guard logic
# without running the full install flow or touching real system state.
#
# Run: bash tests/test-install-unit.sh
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="$REPO_ROOT/install.sh"

PASS=0
FAIL=0

check() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        echo "  PASS  $desc"
        PASS=$((PASS + 1))
    else
        echo "  FAIL  $desc"
        FAIL=$((FAIL + 1))
    fi
}

# --------------------------------------------------------------------------
# Helper definitions (from install.sh lines 83-95)
# --------------------------------------------------------------------------
red()    { printf '\033[1;31m%s\033[0m\n' "$*"; }
green()  { printf '\033[1;32m%s\033[0m\n' "$*"; }
info()  { printf '\033[1;34m[install]\033[0m %s\n' "$*"; }
die()   { red "ERROR: $*"; exit 1; }

has_tty() {
    { exec 3</dev/tty; } 2>/dev/null && exec 3<&- && return 0
    return 1
}

INSTALL_DIR="/usr/local/bin"

# --------------------------------------------------------------------------
# check_selinux() — extracted from install.sh lines 771-802
# --------------------------------------------------------------------------
check_selinux() {
    [[ "$(uname -s)" != "Linux" ]] && return

    local mode
    mode="$(command -v getenforce >/dev/null && getenforce 2>/dev/null || echo "Unknown")"

    case "$mode" in
        Enforcing)
            red "=============================================================="
            red "           SELinux is ENFORCING — Installation will FAIL"
            red "=============================================================="
            red "SELinux is currently enforcing on this system. It will block:"
            red "  - Writing systemd unit files to /etc/systemd/system/"
            red "  - Services executing binaries from ${INSTALL_DIR}"
            red "  - Services accessing data directories under your home"
            red ""
            red "To proceed, set SELinux to permissive or disabled FIRST:"
            red "  Temporary (until reboot):  sudo setenforce 0"
            red "  Permanent:                 edit /etc/selinux/config, set"
            red "                             SELINUX=permissive, then reboot"
            red ""
            red "After installation you can re-enable it with 'sudo setenforce 1'"
            red "but you'll need a custom SELinux module for the monetarium"
            red "services and their data directories."
            red "=============================================================="
            die "SELinux is enforcing. Disable it before running this installer."
            ;;
        Permissive)
            info "SELinux is in permissive mode — check AVC logs if services fail to start."
            ;;
    esac
}

# --------------------------------------------------------------------------
# configure_mining_and_voting — extracted from install.sh lines 530-627
# with the RPC timeout reduced to 2 iterations for testing.
# --------------------------------------------------------------------------
configure_mining_and_voting() {
    if ! has_tty; then
        {
            echo ""
            echo "; Mining configuration — added post-install by install.sh"
            echo "generate=false"
        } >> "$NODE_CONF" 2>/dev/null || true
        info "Non-interactive mode — skipped wallet address polling (no TTY)."
        return
    fi

    info "Waiting for wallet RPC to become available..."

    local i=0
    until monetarium-ctl --wallet getinfo >/dev/null 2>&1; do
        i=$((i + 1))
        if [[ $i -gt ${RPC_TIMEOUT_THRESHOLD:-60} ]]; then
            {
                echo ""
                echo "; Mining configuration — added post-install by install.sh"
                echo "generate=false"
            } >> "$NODE_CONF" 2>/dev/null || true
            info "Wallet RPC not ready after 60 seconds."
            info "Configure mining/ticket addresses manually:"
            info "  Edit ${NODE_CONF}: add miningaddr=<address> then set generate=true"
            return
        fi
        sleep 2
    done

    green "Wallet RPC ready."

    local config_changed=false

    local mining_addr
    mining_addr=$(monetarium-ctl --wallet getnewaddress 2>/dev/null || true)

    local gen_value="false"
    if $MINING_ENABLED && [[ -n "$mining_addr" ]]; then
        gen_value="true"
    fi

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

    local cons_addr
    cons_addr=$(monetarium-ctl --wallet getnewaddress 2>/dev/null || true)
    if [[ -n "$cons_addr" ]]; then
        CONSOLIDATION_ADDR="$cons_addr"
    fi

    if $config_changed; then
        info "Restarting node to apply mining configuration..."
        sleep 1
    fi

    show_configuration_summary
}

show_configuration_summary() {
    echo ""
    green "============================================="
    green "  Configuration Summary"
    green "============================================="
    if $MINING_ENABLED && [[ -n "$MINING_ADDR" ]]; then
        green "  Mining:             enabled (${MINING_CORES} core(s))"
        green "  Mining reward addr: ${MINING_ADDR}"
    else
        if $MINING_ENABLED; then
            red "  Mining:             requested but disabled — no mining address obtained"
        else
            green "  Mining:             disabled"
        fi
    fi
    if $TICKETS_ENABLED; then
        green "  Ticket buying:      enabled"
        green "  Ticket limit:       ${TICKET_LIMIT}"
        green "  Min wallet balance: ${TICKET_BALANCE}"
    else
        green "  Ticket buying:      disabled"
    fi
    if [[ -n "$CONSOLIDATION_ADDR" ]]; then
        green "  Fee consolidation:  ${CONSOLIDATION_ADDR}"
    fi
    if $VOTING_ENABLED; then
        green "  Auto voting:        enabled"
    else
        green "  Auto voting:        disabled"
    fi
    green "============================================="
}

# --------------------------------------------------------------------------
# Test setup
# --------------------------------------------------------------------------
MOCK_DIR="$(mktemp -d)"
TEST_TMP="$(mktemp -d)"
NODE_CONF="$TEST_TMP/monetarium.conf"
MOCK_CTL_DIR="$TEST_TMP/ctl-bin"
mkdir -p "$MOCK_CTL_DIR"

# Mock monetarium-ctl: getinfo always ok, getnewaddress returns addresses
# controlled by GETNEWADDR_COUNT (0 = always empty).
cat > "$MOCK_CTL_DIR/monetarium-ctl" <<'MOCKEOF'
#!/usr/bin/env bash
case "$*" in
    *getinfo*) exit 0 ;;
    *getnewaddress*)
        count="${GETNEWADDR_COUNT:-1}"
        tracker="/tmp/.mock_ctl_$$"
        current=$(cat "$tracker" 2>/dev/null || echo 0)
        current=$((current + 1))
        echo "$current" > "$tracker"
        if [[ "$current" -le "$count" ]]; then
            echo "TTestAddr${current}"
        fi
        exit 0
        ;;
    *setvotefeeconsolidationaddress*) exit 0 ;;
    *) exit 1 ;;
esac
MOCKEOF
chmod +x "$MOCK_CTL_DIR/monetarium-ctl"

# --------------------------------------------------------------------------
# check_selinux() tests
# --------------------------------------------------------------------------
echo "============================================"
echo "  check_selinux() tests"
echo "============================================"

# Helper: run check_selinux in a subshell with controlled PATH and uname.
# Sets _selinux_rc (exit code) and _selinux_out (stdout+stderr text).
SELINUX_LOG="$TEST_TMP/selinux-log"
_selinux_test() {
    local mock_path="$1"
    local fake_uname="$2"
    rm -f "$SELINUX_LOG"
    (
        set +e
        export PATH="$mock_path":$PATH
        uname() { echo "$fake_uname"; }
        export -f uname
        check_selinux > "$SELINUX_LOG" 2>&1
    )
    _selinux_rc=$?
    _selinux_out=""
    [[ -f "$SELINUX_LOG" ]] && _selinux_out="$(cat "$SELINUX_LOG")"
}

# --- Enforcing: should die ---------------------------------------------------
cat > "$MOCK_DIR/getenforce" <<'EOF'
#!/bin/bash
echo "Enforcing"
EOF
chmod +x "$MOCK_DIR/getenforce"

_selinux_test "$MOCK_DIR" "Linux"
check "enforcing → exit 1" bash -c "test '$_selinux_rc' = '1'"
echo "$_selinux_out" | grep -q "ENFORCING" \
    && { echo "  PASS  enforcing → output mentions ENFORCING"; PASS=$((PASS + 1)); } \
    || { echo "  FAIL  enforcing → output mentions ENFORCING"; FAIL=$((FAIL + 1)); }

# --- Permissive: should succeed with info message -----------------------------
cat > "$MOCK_DIR/getenforce" <<'EOF'
#!/bin/bash
echo "Permissive"
EOF
chmod +x "$MOCK_DIR/getenforce"

_selinux_test "$MOCK_DIR" "Linux"
check "permissive → exit 0" bash -c "test '$_selinux_rc' = '0'"
echo "$_selinux_out" | grep -qi "permissive" \
    && { echo "  PASS  permissive → output mentions permissive"; PASS=$((PASS + 1)); } \
    || { echo "  FAIL  permissive → output mentions permissive"; FAIL=$((FAIL + 1)); }

# --- Disabled: should succeed silently ----------------------------------------
cat > "$MOCK_DIR/getenforce" <<'EOF'
#!/bin/bash
echo "Disabled"
EOF
chmod +x "$MOCK_DIR/getenforce"

_selinux_test "$MOCK_DIR" "Linux"
check "disabled → exit 0" bash -c "test '$_selinux_rc' = '0'"
check "disabled → no output" bash -c "test -z '$_selinux_out'"

# --- No getenforce binary: should succeed silently ----------------------------
rm -f "$MOCK_DIR/getenforce"

_selinux_test "$MOCK_DIR" "Linux"
check "no getenforce → exit 0" bash -c "test '$_selinux_rc' = '0'"
check "no getenforce → no output" bash -c "test -z '$_selinux_out'"

# --- macOS: no-op regardless of getenforce ------------------------------------
cat > "$MOCK_DIR/getenforce" <<'EOF'
#!/bin/bash
echo "Enforcing"
EOF
chmod +x "$MOCK_DIR/getenforce"

_selinux_test "$MOCK_DIR" "Darwin"
check "macOS → no-op, exit 0" bash -c "test '$_selinux_rc' = '0'"

# --------------------------------------------------------------------------
# generate/miningaddr guard tests
# --------------------------------------------------------------------------
echo ""
echo "============================================"
echo "  generate/miningaddr guard tests"
echo "============================================"

# Helper: run configure_mining_and_voting in an isolated subshell
# with controlled globals and mocks.
run_mining_test() {
    local mining_enabled="$1"
    local mining_cores="$2"
    local addr_count="$3"
    local tty_mode="$4"     # "true" or "false"
    local expect_generate="$5"
    local expect_addr="$6"
    local timeout_thresh="${7:-60}"

    # Fresh node config
    cat > "$NODE_CONF" <<EOF
; monetarium.conf - generated by install.sh on test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF

    # Reset globals
    MINING_ENABLED="$mining_enabled"
    MINING_CORES="$mining_cores"
    MINING_ADDR=""
    CONSOLIDATION_ADDR=""
    TICKETS_ENABLED=false
    VOTING_ENABLED=false

    # Reset mock call tracker
    rm -f "/tmp/.mock_ctl_$$"
    export GETNEWADDR_COUNT="$addr_count"
    export RPC_TIMEOUT_THRESHOLD="$timeout_thresh"

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        if [[ "$tty_mode" == "true" ]]; then
            has_tty() { return 0; }
        else
            has_tty() { return 1; }
        fi
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    # Assert
    local ok=true
    if ! grep -q "generate=${expect_generate}" "$NODE_CONF"; then
        echo "    FAIL expected generate=${expect_generate}:"
        grep "generate" "$NODE_CONF" | sed 's/^/      /'
        ok=false
    fi
    if [[ "$expect_addr" == "true" ]]; then
        if ! grep -q "miningaddr=" "$NODE_CONF"; then
            echo "    FAIL expected miningaddr= in config"
            ok=false
        fi
    else
        if grep -q "miningaddr=" "$NODE_CONF"; then
            echo "    FAIL unexpected miningaddr= in config"
            ok=false
        fi
    fi
    $ok
}

# --- Path 1: no TTY (non-interactive) -----------------------------------------
run_no_tty_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=true
    MINING_CORES="2"
    MINING_ADDR=""
    CONSOLIDATION_ADDR=""
    export GETNEWADDR_COUNT=1

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        has_tty() { return 1; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=false" "$NODE_CONF"
}
check "no TTY → generate=false" run_no_tty_test

# --- Path 2: RPC timeout (wallet not ready) -----------------------------------
run_rpc_timeout_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=true
    MINING_CORES="2"
    MINING_ADDR=""
    CONSOLIDATION_ADDR=""

    # Mock: getinfo always fails → triggers timeout path
    cat > "$MOCK_CTL_DIR/monetarium-ctl" <<'MOCKEOF'
#!/usr/bin/env bash
case "$*" in
    *getinfo*) exit 1 ;;
    *) exit 1 ;;
esac
MOCKEOF
    chmod +x "$MOCK_CTL_DIR/monetarium-ctl"

    export RPC_TIMEOUT_THRESHOLD=2
    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        export RPC_TIMEOUT_THRESHOLD=2
        has_tty() { return 0; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=false" "$NODE_CONF"
}
check "RPC timeout → generate=false" run_rpc_timeout_test

# Restore mock
cat > "$MOCK_CTL_DIR/monetarium-ctl" <<'MOCKEOF'
#!/usr/bin/env bash
case "$*" in
    *getinfo*) exit 0 ;;
    *getnewaddress*)
        count="${GETNEWADDR_COUNT:-1}"
        tracker="/tmp/.mock_ctl_$$"
        current=$(cat "$tracker" 2>/dev/null || echo 0)
        current=$((current + 1))
        echo "$current" > "$tracker"
        if [[ "$current" -le "$count" ]]; then
            echo "TTestAddr${current}"
        fi
        exit 0
        ;;
    *setvotefeeconsolidationaddress*) exit 0 ;;
    *) exit 1 ;;
esac
MOCKEOF
chmod +x "$MOCK_CTL_DIR/monetarium-ctl"

# --- Path 3a: RPC ready, address obtained → generate=true ---------------------
run_rpc_ready_ok_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=true
    MINING_CORES="2"
    MINING_ADDR=""
    export GETNEWADDR_COUNT=1

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        has_tty() { return 0; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=true" "$NODE_CONF" && grep -q "miningaddr=TTestAddr1" "$NODE_CONF"
}
check "RPC ready + address → generate=true + miningaddr" run_rpc_ready_ok_test

# --- Path 3b: mining NOT enabled → generate=false ----------------------------
run_mining_disabled_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=false
    MINING_CORES=""
    MINING_ADDR=""
    export GETNEWADDR_COUNT=1

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        has_tty() { return 0; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=false" "$NODE_CONF"
}
check "mining disabled → generate=false" run_mining_disabled_test

# --- Path 3c: getnewaddress returns empty → generate=false --------------------
run_empty_addr_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=true
    MINING_CORES="2"
    MINING_ADDR=""
    export GETNEWADDR_COUNT=0

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        has_tty() { return 0; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=false" "$NODE_CONF" && ! grep -q "miningaddr=" "$NODE_CONF"
}
check "empty getnewaddress → generate=false, no miningaddr" run_empty_addr_test

# --- Path 3d: two addresses → first addr used, generate=true ----------------
run_two_addrs_test() {
    cat > "$NODE_CONF" <<'EOF'
; monetarium.conf - test
rpcuser=monetarium
rpcpass=testpass123
addpeer=176.113.164.216:9508
EOF
    MINING_ENABLED=true
    MINING_CORES="2"
    MINING_ADDR=""
    export GETNEWADDR_COUNT=2

    (
        set +e
        export PATH="$MOCK_CTL_DIR":$PATH
        has_tty() { return 0; }
        show_configuration_summary() { :; }
        configure_mining_and_voting
    )

    grep -q "generate=true" "$NODE_CONF" && grep -q "miningaddr=TTestAddr1" "$NODE_CONF"
}
check "two getnewaddr calls → generate=true, first addr" run_two_addrs_test

# --- Summary: mining requested but no addr → shows "requested but disabled" ---
run_summary_no_addr_test() {
    (
        set +e
        MINING_ENABLED=true
        MINING_ADDR=""
        MINING_CORES="2"
        TICKETS_ENABLED=false
        VOTING_ENABLED=false
        CONSOLIDATION_ADDR=""
        output=$(show_configuration_summary 2>&1)
        echo "$output" | grep -q "requested but disabled"
    )
}
check "summary: mining requested but no addr → 'requested but disabled'" run_summary_no_addr_test

# --- Summary: mining enabled with addr → shows "enabled" --------------------
run_summary_with_addr_test() {
    (
        set +e
        MINING_ENABLED=true
        MINING_ADDR="TTestAddr"
        MINING_CORES="2"
        TICKETS_ENABLED=false
        VOTING_ENABLED=false
        CONSOLIDATION_ADDR=""
        output=$(show_configuration_summary 2>&1)
        echo "$output" | grep -q "enabled" && echo "$output" | grep -q "TTestAddr"
    )
}
check "summary: mining + addr → 'enabled' with address" run_summary_with_addr_test

# --- Summary: mining not requested → shows "disabled" ------------------------
run_summary_not_requested_test() {
    (
        set +e
        MINING_ENABLED=false
        MINING_ADDR=""
        MINING_CORES=""
        TICKETS_ENABLED=false
        VOTING_ENABLED=false
        CONSOLIDATION_ADDR=""
        output=$(show_configuration_summary 2>&1)
        echo "$output" | grep -q "disabled" && ! echo "$output" | grep -q "requested but disabled"
    )
}
check "summary: mining not requested → plain 'disabled'" run_summary_not_requested_test

# --------------------------------------------------------------------------
# Cleanup
# --------------------------------------------------------------------------
rm -f "/tmp/.mock_ctl_$$"
rm -rf "$MOCK_DIR" "$TEST_TMP"

echo ""
echo "== Results: $PASS passed, $FAIL failed =="
[[ "$FAIL" -eq 0 ]]
