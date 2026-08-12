# Monetarium Quick Start — Single-Command Installer

This guide walks you through installing a Monetarium full node + wallet with the
one-line installer, and then shows you how to verify everything is working.

> ⚠️ **Read the security warning at the end of this guide before you put real
> funds on this wallet.** The installer configures the wallet to stay unlocked
> automatically, which makes this machine a **hot wallet**.

---

## 1. What the installer does

Running `install.sh` performs the following steps:

1. Detects your OS (Linux/macOS) and architecture (amd64/arm64).
2. Downloads the latest release binaries to `/usr/local/bin`:
   - `monetarium-node` — the full node daemon
   - `monetarium-wallet` — the wallet (auto-unlock, ticket buying / voting)
   - `monetarium-ctl` — the RPC client you use to query and control both
3. Prompts you for a wallet passphrase and ticket-buying / voting preferences.
4. Writes configuration files and creates a new wallet (skipped if the wallet
   database already exists — see §9).
5. Installs and starts `systemd` services (Linux) or `launchd` agents (macOS)
   that run the node + wallet and unlock the wallet automatically.
6. Prompts you about optional CPU mining, derives a fresh mining address from
   the wallet, and **restarts the node** so it picks up the new
   `generate`/`miningaddr` setting.
7. Prints a summary of your configuration and a big security warning.

> Every interactive run derives a **new** mining address and a new
> fee-consolidation address from the wallet (`getnewaddress`), and the
> consolidation address is reset on the wallet. This is intentional — each
> re-run keeps the wallet's gain-splitting consolidation pointing at a fresh
> address — but it means the addresses printed in the summary change every
> time you run the installer.

## 2. Before you start

> **Windows?** This guide covers the `install.sh` script, which supports Linux
> and macOS only — it aborts on any other OS. Windows users should use the
> PowerShell installer (`install.ps1`) from the README instead; the rest of
> this guide does not apply.

**Requirements**

- Linux (x86_64 or arm64) or macOS (Intel or Apple Silicon)
- `curl`, `tar`, and `sudo` available
- ~16 GB free disk space, 2 GB RAM (see README for full specs)
- Outbound internet access; optionally open **inbound TCP port 9508** so other
  nodes can reach you
- On Linux, **SELinux must not be enforcing** — the installer checks this and
  aborts if it is. Temporarily disable with `sudo setenforce 0` (see the
  installer's message for the permanent option).

## 3. Install

Run the single-line installer:

```sh
curl -sSL https://raw.githubusercontent.com/monetarium/monetarium-node/main/install.sh | bash
```

Or with `wget`:

```sh
wget -qO- https://raw.githubusercontent.com/monetarium/monetarium-node/main/install.sh | bash
```

The installer is interactive. You will be asked:

| Prompt | What it means |
| ------ | ------------- |
| **Wallet private passphrase** | Unlocks the wallet. Chosen by you, entered twice. The wallet seed is shown during wallet creation — **write it down and store it offline.** |
| **Enable automatic ticket purchase for staking?** | Buy tickets automatically for Proof-of-Stake rewards. |
| **Maximum number of tickets per purchase** | `ticketbuyer.limit` (default `5`). |
| **Minimum VAR balance to keep in wallet** | `ticketbuyer.balancetomaintainabsolute` (default `0`). |
| **Enable automatic voting?** | Vote with your tickets automatically (default yes). |
| **Enable CPU mining?** | Built-in CPU mining (usually not worthwhile on mainnet; the installer asks how many cores). |

The installer also runs `monetarium-wallet --create` for the first-time wallet
creation and will ask you to confirm the passphrase.

When it finishes you'll see a **Configuration Summary** and the security warning —
unless the wallet RPC never came up within 60 seconds, in which case the summary
still appears but notes that no mining address was obtained. On a fresh install
mining stays disabled; **on a re-run the mining configuration carried over from
the previous install is kept**, and the summary says so and asks you to verify
`miningaddr=` manually. A manifest listing every file created is written to
`~/.monetarium/install.manifest`.

> Tip: if your shell doesn't see the commands after installing, run
> `source ~/.bashrc` (or `~/.zshrc`) or open a new terminal — the installer
> adds `/usr/local/bin` to your `PATH`.

## 4. What was installed

All paths are for **Linux**. macOS uses `~/Library/Application Support/Monetarium…`
instead of `~/.monetarium…`.

| Component | Location |
| --------- | -------- |
| Binaries | `/usr/local/bin/monetarium-node`, `/usr/local/bin/monetarium-wallet`, `/usr/local/bin/monetarium-ctl` |
| Node config | `~/.monetarium/monetarium.conf` |
| Wallet config | `~/.monetarium/monetarium-wallet.conf` (installer writes it here, not the wallet's default `~/.monetarium-wallet/` path) |
| ctl config | `~/.monetarium-ctl/monetarium-ctl.conf` |
| Node blockchain data | `~/.monetarium/data/mainnet/` |
| Wallet database | `~/.monetarium-wallet/mainnet/wallet.db` |
| Node logs | `~/.monetarium/logs/mainnet/monetarium.log` |
| Wallet logs | `~/.monetarium-wallet/logs/mainnet/monetarium-wallet.log` |
| Install manifest | `~/.monetarium/install.manifest` |
| Services (Linux) | `monetarium-node.service`, `monetarium-wallet.service` |
| Services (macOS) | `~/Library/LaunchAgents/com.monetarium.node.plist`, `com.monetarium.wallet.plist` |

Default ports: peer-to-peer **9508**, node RPC **9509**, wallet RPC **9510**.

## 5. Verify everything is working

Run `monetarium-ctl` queries against the node and the wallet (`--wallet`).
Because the installer wrote the ctl config for you, you don't need any flags.

### 5.1 Services are running

Linux:

```sh
systemctl status monetarium-node monetarium-wallet --no-pager
```

Both should show `active (running)`. A quick check:

```sh
systemctl is-active monetarium-node monetarium-wallet
# active
# active
```

macOS:

```sh
launchctl list | grep monetarium
```

### 5.2 Node is healthy

```sh
monetarium-ctl getinfo
```

Expect a block height, `connections` greater than 0, and an empty `"errors"`
field:

```json
{
  "version": 2010000,
  "protocolversion": 13,
  "blocks": 19233,
  "connections": 5,
  "testnet": false,
  "errors": ""
}
```

### 5.3 Node is fully synced

```sh
monetarium-ctl getblockcount
monetarium-ctl getblockchaininfo
```

The node is synced when `blocks` == `headers` == `syncheight` and
`verificationprogress` is `1` (or `"initialblockdownload": false`). Right after
installation it will take time to download the whole chain — this is normal.

### 5.4 Wallet is connected, unlocked, and synced

```sh
monetarium-ctl --wallet walletinfo
monetarium-ctl --wallet syncstatus
```

Check for `"daemonconnected": true`, `"unlocked": true`, and
`"synced": true`. The wallet also starts scanning from its birthday block, so
give it time after a fresh install.

### 5.5 Check your balance

```sh
monetarium-ctl --wallet getbalance
```

The `default` account shows several figures:

| Field | Meaning |
| ----- | ------- |
| `spendable` | Available to send / buy tickets now |
| `lockedbytickets` | VAR locked in live tickets (staking) |
| `immaturestakegeneration` | Staking rewards not yet mature |
| `unconfirmed` | Incoming, not yet in a block |
| `total` | Sum of everything on the account |

A quick one-liner equivalent is `monetarium-ctl --wallet getinfo`, which shows
`balance` and `unlocked_until`.

### 5.6 Staking (if ticket buying is enabled)

```sh
monetarium-ctl --wallet getstakeinfo
```

Shows `immature`, `unspent`, `voted`, `live`, and the ticket pool size. Tickets
mature after the initial ticket maturity window; live tickets earn voting
rewards.

### 5.7 Peers and network

```sh
monetarium-ctl getconnectioncount
monetarium-ctl getpeerinfo
```

`getconnectioncount` should be greater than 0. If it is always 0, see
Troubleshooting.

### 5.8 Mining (if enabled)

```sh
monetarium-ctl getmininginfo
monetarium-ctl getgenerate
```

## 6. Check logs

**Linux (systemd)** — the services log to the journal. Tail them live:

```sh
journalctl -u monetarium-node -f
journalctl -u monetarium-wallet -f
```

Or the last lines:

```sh
journalctl -u monetarium-node -n 100 --no-pager
```

The daemons also write to their own log files:

```sh
tail -f ~/.monetarium/logs/mainnet/monetarium.log
tail -f ~/.monetarium-wallet/logs/mainnet/monetarium-wallet.log
```

Log lines look like:

```
2026-08-01 18:42:58.355 [INF] SYNC: New block e64ee32666abd728... (height 19233, interval 2m24s)
```

`[ERR]` / `[WRN]` lines are worth investigating; steady `[INF] SYNC: New block`
lines mean the node is keeping up with the chain.

**macOS (launchd)** — output goes to files in the Monetarium data directory:

```sh
tail -f ~/Library/Application\ Support/Monetarium/node.log
tail -f ~/Library/Application\ Support/Monetarium/node.err.log
tail -f ~/Library/Application\ Support/Monetarium/wallet.log
tail -f ~/Library/Application\ Support/Monetarium/wallet.err.log
```

## 7. Quick command reference

| Task | Command |
| ---- | ------- |
| Node status | `monetarium-ctl getinfo` |
| Sync progress | `monetarium-ctl getblockcount` / `getblockchaininfo` |
| Wallet status | `monetarium-ctl --wallet getinfo` |
| Balance | `monetarium-ctl --wallet getbalance` |
| Staking info | `monetarium-ctl --wallet getstakeinfo` |
| Peer count | `monetarium-ctl getconnectioncount` |
| Mining status | `monetarium-ctl getmininginfo` |
| Start/stop services (Linux) | `sudo systemctl start/stop/restart monetarium-node monetarium-wallet` |
| All ctl commands | `monetarium-ctl --listcommands` |

## 8. Troubleshooting

**Services are not running / keep restarting**

```sh
journalctl -u monetarium-node -n 50 --no-pager
journalctl -u monetarium-wallet -n 50 --no-pager
```

Common causes:
- SELinux enforcing on Linux — set it permissive before installing.
- Wallet passphrase mismatch — if the wallet fails to unlock, the wallet service
  crashes. Re-run the installer and enter the **same** passphrase you set when
  the wallet was first created (see §9).
- Wallet database missing/corrupt — check the wallet log for errors.

**Node is running but `connections` is 0 / not syncing**

- Confirm outbound internet works: `curl -sS https://api.github.com/repos/monetarium/monetarium-node | head -1`.
- If you're behind a strict firewall, the installer already added
  `addpeer` entries to `~/.monetarium/monetarium.conf` — make sure they're still
  there.
- Check the node log for peer connection errors.

**`monetarium-ctl` can't connect**

- The services must be running (section 5.1).
- The ctl config at `~/.monetarium-ctl/monetarium-ctl.conf` must match the node's
  `rpcuser`/`rpcpass` — they do unless you edited the node config by hand.

**Balance stays 0 or missing transactions**

- The wallet scans from its creation time. Confirm it's synced:
  `monetarium-ctl --wallet syncstatus` and check the wallet log.
- If the wallet service crashed or keeps restarting, restart it with
  `sudo systemctl restart monetarium-wallet` (Linux) or re-load the launchd
  agent (macOS). Re-running the installer for a crashed wallet usually doesn't
  help — it rewrites the config, restarts the service, and re-derives
  addresses, but the wallet still rescans from its original creation time.

**CPU mining was enabled but the node won't start**

- `generate=true` in the node config requires a `miningaddr` to be set. Check
  `~/.monetarium/monetarium.conf` — both must be present. The installer only
  enables mining when it can obtain an address, so this usually only happens if
  you hand-edit the config.

## 9. Updating and uninstalling

**Update** — re-run the same installer command. On every run the installer
compares each installed binary's SHA-256 against the checksum published in the
release it comes from, and downloads + replaces any binary that changed (the
affected services are restarted when a binary is replaced). For **raw-binary**
distributions, if you're already on the latest release, re-running is a no-op
for the binaries (the installed hash matches, nothing is redownloaded). Releases
that ship as an **archive** (`.tar.gz`/`.tgz`/`.zip`) publish the checksum of
the archive, not of the installed binary, so the skip/matched-hash guarantee
does not apply there — archive releases are downloaded and re-verified on every
run. Your wallet database and blockchain data are preserved either way.

The installer **reuses your existing RPC credentials** on re-runs (it doesn't
rotate them), but it still rewrites the config files and can re-enable or
disable mining based on your answers. Re-running is **not** a way to change the
wallet passphrase — the wallet database keeps the passphrase it was created
with.

> ⚠️ **Enter the SAME wallet passphrase when it asks again.** The installer
> prompts for the passphrase on every run. `create_wallet` skips wallet
> creation when `wallet.db` already exists (so the database keeps its original
> passphrase), but `write_configs` unconditionally rewrites `pass=` in
> `~/.monetarium/monetarium-wallet.conf` with whatever you type. If you enter a
> different passphrase, the two no longer match, the wallet fails to unlock,
> and the service keeps crashing — the "passphrase mismatch" symptom in
> §8 Troubleshooting.

**Uninstall** — the manifest at `~/.monetarium/install.manifest` lists every file
and the exact commands to stop services. In short:

```sh
# Linux
sudo systemctl disable --now monetarium-node monetarium-wallet

# macOS
launchctl unload ~/Library/LaunchAgents/com.monetarium.node.plist
launchctl unload ~/Library/LaunchAgents/com.monetarium.wallet.plist
```

Then remove the files listed in the manifest.

> ⚠️ **Before deleting `~/.monetarium-wallet/mainnet/wallet.db` — move any funds
> out and make sure you have the wallet seed.** Without the seed the wallet
> database is unrecoverable.

## 10. Security warning — this is a hot wallet

**This applies to everyone who runs the installer, not just stakers.** The
installer always asks for a passphrase (even if you decline staking) and always
writes your **wallet passphrase in plaintext** to
`~/.monetarium/monetarium-wallet.conf` so the service can unlock the wallet on
boot. The wallet is left unlocked 24/7 — and even if you only mine, your block
rewards accumulate in exactly this wallet. Consequences:

- Anyone who can read that file (root, a backup, a compromised process) can
  unlock your wallet and move your funds.
- The wallet stays unlocked 24/7 — there is no PIN, 2FA, or approval step
  between an attacker and your coins.
- If the machine is compromised, funds can be drained automatically.

Recommended precautions:

- Only keep funds on this wallet that you can afford to lose — and remember
  that mined rewards land here too.
- Keep the bulk of your holdings in a separate offline or hardware-secured
  wallet.
- Restrict SSH/root access, keep the machine patched, and monitor it actively.
- If you ever suspect the machine has been compromised, treat the wallet as
  compromised: back up the seed and the wallet database, purge the passphrase
  and the config files, and re-create the wallet from the seed — do **not**
  just re-run the installer. Entering a new passphrase on a re-run will not
  change the wallet database's passphrase; it will only break the auto-unlock
  (see §9).
