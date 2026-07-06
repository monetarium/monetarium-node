#requires -Version 5.1
<#
.SYNOPSIS
    Installer for monetarium-node + monetarium-wallet + monetarium-ctl (Windows).

.DESCRIPTION
    1. Detects CPU architecture and downloads the matching Windows binaries.
    2. Prompts for the wallet's private passphrase (hidden input).
    3. Writes that passphrase into the wallet's config file so it can
       auto-unlock on start (needed for unattended ticket buying / voting).
    4. Registers Scheduled Tasks (Windows' equivalent of systemd/launchd
       here, since these binaries don't speak the Windows Service Control
       Manager protocol) that start node + wallet at boot.
    5. Prints a large, hard-to-miss warning about what this implies.

    Functions are split out and the "Main" entry point only runs when this
    file is executed directly -- dot-sourcing it (as the Pester tests do)
    loads the functions without running the installer.

.NOTES
    Must be run from an elevated ("Run as Administrator") PowerShell.
#>

[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingWriteHost', '')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingPlainTextForPassword', '')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSAvoidUsingUsernameAndPasswordParams', '')]
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '')]
[CmdletBinding()]
param(
    # Used by tests to be extra explicit that Main should not auto-run.
    [switch]$NoExec
)

$ErrorActionPreference = 'Stop'

# --------------------------------------------------------------------------
# Config
# --------------------------------------------------------------------------
$Script:GitHubOrg  = 'monetarium'
$Script:InstallDir = Join-Path $env:ProgramFiles 'Monetarium'
$Script:DataDir    = Join-Path $env:ProgramData 'Monetarium'
$Script:WalletConf = Join-Path $Script:DataDir 'monetarium-wallet.conf'
$Script:NodeConf   = Join-Path $Script:DataDir 'monetarium.conf'

# --------------------------------------------------------------------------
# Helpers
# --------------------------------------------------------------------------
function Write-Info {
    param([string]$Message)
    Write-Host "[install] $Message" -ForegroundColor Cyan
}

function Write-ErrAndThrow {
    param([string]$Message)
    Write-Host "ERROR: $Message" -ForegroundColor Red
    throw $Message
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# --------------------------------------------------------------------------
# 1. Detect platform
# --------------------------------------------------------------------------
function Get-Platform {
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'ARM64'  { 'arm64' }
        'AMD64'  { 'amd64' }
        default  { 'amd64' }
    }
    return "windows-$arch"
}

# --------------------------------------------------------------------------
# 2. Download binaries
# --------------------------------------------------------------------------
function Get-LatestReleaseAssetUrl {
    param(
        [Parameter(Mandatory)][string]$Repo,
        [Parameter(Mandatory)][string]$Platform
    )

    $api = "https://api.github.com/repos/$($Script:GitHubOrg)/$Repo/releases/latest"
    $release = Invoke-RestMethod -Uri $api -Headers @{ 'User-Agent' = 'monetarium-installer' }

    $asset = $release.assets | Where-Object { $_.name -like "*$Platform*" } | Select-Object -First 1
    if (-not $asset) {
        Write-ErrAndThrow "Could not find a release asset for '$Repo' matching platform '$Platform'. Check https://github.com/$($Script:GitHubOrg)/$Repo/releases manually."
    }
    return $asset.browser_download_url
}

function Install-Binary {
    param(
        [Parameter(Mandatory)][string]$Repo,
        [Parameter(Mandatory)][string]$BinaryName,
        [Parameter(Mandatory)][string]$Platform
    )

    Write-Info "Fetching latest release info for $Repo..."
    $url = Get-LatestReleaseAssetUrl -Repo $Repo -Platform $Platform

    $tmpDir = Join-Path $env:TEMP ([guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmpDir | Out-Null
    $archivePath = Join-Path $tmpDir (Split-Path $url -Leaf)

    Write-Info "Downloading $BinaryName from: $url"
    Invoke-WebRequest -Uri $url -OutFile $archivePath -UseBasicParsing

    if ($archivePath -like '*.zip') {
        Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force
    }

    $found = Get-ChildItem -Path $tmpDir -Recurse -Filter "$BinaryName.exe" | Select-Object -First 1
    if (-not $found) {
        Write-ErrAndThrow "Could not locate '$BinaryName.exe' inside the downloaded archive."
    }

    if (-not (Test-Path $Script:InstallDir)) {
        New-Item -ItemType Directory -Path $Script:InstallDir -Force | Out-Null
    }

    $destination = Join-Path $Script:InstallDir "$BinaryName.exe"
    Copy-Item -Path $found.FullName -Destination $destination -Force
    Remove-Item -Path $tmpDir -Recurse -Force

    Write-Info "Installed $BinaryName -> $destination"
}

function Add-InstallDirToPath {
    $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    if ($machinePath -notlike "*$($Script:InstallDir)*") {
        [Environment]::SetEnvironmentVariable('Path', "$machinePath;$($Script:InstallDir)", 'Machine')
        Write-Info "Added $($Script:InstallDir) to the system PATH (open a new shell to pick it up)."
    }
    if ($env:Path -notlike "*$($Script:InstallDir)*") {
        $env:Path = "$env:Path;$($Script:InstallDir)"
    }
}

# --------------------------------------------------------------------------
# 3. Prompt for passphrase (hidden)
# --------------------------------------------------------------------------
function ConvertFrom-SecureStringToPlainText {
    param([Parameter(Mandatory)][System.Security.SecureString]$SecureString)

    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureString)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

function Read-WalletPassphrase {
    while ($true) {
        $secure1 = Read-Host -Prompt 'Enter wallet private passphrase' -AsSecureString
        $secure2 = Read-Host -Prompt 'Confirm passphrase' -AsSecureString

        $plain1 = ConvertFrom-SecureStringToPlainText -SecureString $secure1
        $plain2 = ConvertFrom-SecureStringToPlainText -SecureString $secure2

        if ([string]::IsNullOrEmpty($plain1)) {
            Write-Host 'Passphrase cannot be empty. Try again.' -ForegroundColor Red
            continue
        }
        if ($plain1 -ne $plain2) {
            Write-Host 'Passphrases did not match. Try again.' -ForegroundColor Red
            continue
        }
        return $plain1
    }
}

function Get-RandomToken {
    param([int]$Length = 32)
    # Over-allocate raw bytes since stripping non-alphanumeric base64
    # characters (+, /, =) shortens the string unpredictably.
    $bytes = New-Object byte[] ($Length * 3)
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $alnum = [Convert]::ToBase64String($bytes) -replace '[^a-zA-Z0-9]', ''
    if ($alnum.Length -lt $Length) {
        Write-ErrAndThrow 'Failed to generate a sufficiently long random token.'
    }
    return $alnum.Substring(0, $Length)
}

# --------------------------------------------------------------------------
# 4. Write config files
# --------------------------------------------------------------------------
function Protect-ConfigFile {
    param([Parameter(Mandatory)][string]$Path)

    $acl = Get-Acl -Path $Path
    $acl.SetAccessRuleProtection($true, $false)  # disable inheritance, drop inherited rules

    foreach ($identity in @($env:USERNAME, 'SYSTEM', 'Administrators')) {
        $rule = New-Object Security.AccessControl.FileSystemAccessRule(
            $identity, 'FullControl', 'Allow'
        )
        $acl.AddAccessRule($rule)
    }
    Set-Acl -Path $Path -AclObject $acl
}

function Write-Config {
    param(
        [Parameter(Mandatory)][string]$Passphrase,
        [Parameter(Mandatory)][string]$RpcUser,
        [Parameter(Mandatory)][string]$RpcSecret
    )

    New-Item -ItemType Directory -Path $Script:DataDir -Force | Out-Null
    $timestamp = (Get-Date).ToUniversalTime().ToString('o')

    @"
; monetarium.conf - generated by install.ps1 on $timestamp
rpcuser=$RpcUser
rpcpass=$RpcSecret
addpeer=134.249.62.43:9508
"@ | Set-Content -Path $Script:NodeConf -Encoding UTF8

    @"
; monetarium-wallet.conf - generated by install.ps1 on $timestamp
; WARNING: 'pass' below stores the wallet unlock passphrase in plaintext
; so the scheduled task can auto-unlock the wallet at startup for
; unattended ticket buying / voting. See the warning printed by install.ps1.
username=$RpcUser
password=$RpcSecret
pass=$Passphrase
enablevoting=1
enableticketbuyer=1
"@ | Set-Content -Path $Script:WalletConf -Encoding UTF8

    Protect-ConfigFile -Path $Script:NodeConf
    Protect-ConfigFile -Path $Script:WalletConf

    Write-Info "Config files written to $($Script:DataDir) with access restricted to the current user, SYSTEM, and Administrators."
}

# --------------------------------------------------------------------------
# 5. Create wallet
# --------------------------------------------------------------------------
function Get-WalletDataDir {
    $localAppData = if ($env:LOCALAPPDATA) { $env:LOCALAPPDATA } else { $env:APPDATA }
    return Join-Path $localAppData 'Monetarium-wallet'
}

function Create-Wallet {
    $walletDb = Join-Path (Get-WalletDataDir) 'mainnet\wallet.db'

    if (Test-Path $walletDb) {
        Write-Info "Wallet database already exists at $walletDb — skipping creation."
        return
    }

    $walletExe = Join-Path $Script:InstallDir 'monetarium-wallet.exe'

    Write-Info "Creating wallet. Follow the interactive prompts."
    Write-Info "When asked, type 'yes' to use the passphrase you just entered."

    & $walletExe --create --configfile "$Script:WalletConf" 2>&1

    if (-not (Test-Path $walletDb)) {
        Write-ErrAndThrow "Wallet creation failed — database not found at $walletDb"
    }

    Write-Info 'Wallet created successfully.'
}

# --------------------------------------------------------------------------
# 6. Scheduled Tasks (Windows has no systemd/launchd; this is the closest
#    equivalent for auto-starting an unattended background process)
# --------------------------------------------------------------------------
function Install-ScheduledTask {
    $nodeExe   = Join-Path $Script:InstallDir 'monetarium-node.exe'
    $walletExe = Join-Path $Script:InstallDir 'monetarium-wallet.exe'

    $settings = New-ScheduledTaskSettingsSet `
        -StartWhenAvailable `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1) `
        -ExecutionTimeLimit ([TimeSpan]::Zero)

    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest

    $nodeAction = New-ScheduledTaskAction -Execute $nodeExe -Argument "--configfile=`"$($Script:NodeConf)`""
    $nodeTrigger = New-ScheduledTaskTrigger -AtStartup
    Register-ScheduledTask -TaskName 'MonetariumNode' -Action $nodeAction -Trigger $nodeTrigger `
        -Principal $principal -Settings $settings -Force | Out-Null

    # Give the node a head start before the wallet connects to it.
    $walletAction = New-ScheduledTaskAction -Execute $walletExe -Argument "--configfile=`"$($Script:WalletConf)`""
    $walletTrigger = New-ScheduledTaskTrigger -AtStartup
    $walletTrigger.Delay = 'PT15S'
    Register-ScheduledTask -TaskName 'MonetariumWallet' -Action $walletAction -Trigger $walletTrigger `
        -Principal $principal -Settings $settings -Force | Out-Null

    Start-ScheduledTask -TaskName 'MonetariumNode'
    Start-Sleep -Seconds 2
    Start-ScheduledTask -TaskName 'MonetariumWallet'

    Write-Info 'Scheduled Tasks installed and started: MonetariumNode, MonetariumWallet'
    Write-Info '  Get-ScheduledTask -TaskName MonetariumNode, MonetariumWallet'
}

# --------------------------------------------------------------------------
# 7. Big warning
# --------------------------------------------------------------------------
function Show-BigWarning {
    $bar = '=' * 70
    Write-Host ''
    Write-Host $bar -ForegroundColor Red
    Write-Host '                       !!!  WARNING  !!!' -ForegroundColor Red
    Write-Host $bar -ForegroundColor Red
    Write-Host " This script has written your wallet's PRIVATE PASSPHRASE in" -ForegroundColor Red
    Write-Host ' PLAINTEXT to:' -ForegroundColor Red
    Write-Host "     $($Script:WalletConf)" -ForegroundColor Red
    Write-Host ''
    Write-Host ' This is required so the wallet can auto-unlock and buy' -ForegroundColor Red
    Write-Host ' tickets / vote WITHOUT any further action from you. It also' -ForegroundColor Red
    Write-Host ' means:' -ForegroundColor Red
    Write-Host '   - Anyone who can read that file (an admin account, a backup,' -ForegroundColor Red
    Write-Host '     a compromised process, malware running as your user) can' -ForegroundColor Red
    Write-Host '     unlock your wallet and move your funds.' -ForegroundColor Red
    Write-Host '   - This machine is now effectively a HOT WALLET that stays' -ForegroundColor Red
    Write-Host '     unlocked 24/7. There is no PIN, 2FA, or manual approval' -ForegroundColor Red
    Write-Host '     step standing between an attacker and your coins.' -ForegroundColor Red
    Write-Host '   - If this machine is compromised, funds can be drained' -ForegroundColor Red
    Write-Host '     automatically, with no prompt and no warning.' -ForegroundColor Red
    Write-Host ''
    Write-Host ' Recommended precautions:' -ForegroundColor Red
    Write-Host '   - Only put funds on this wallet that you can afford to' -ForegroundColor Red
    Write-Host '     lose, sized for ticket-buying/voting purposes only.' -ForegroundColor Red
    Write-Host '   - Keep the bulk of your holdings in a separate, offline' -ForegroundColor Red
    Write-Host '     or hardware-secured wallet, not this one.' -ForegroundColor Red
    Write-Host '   - Keep Windows Update, antivirus, and firewall active and' -ForegroundColor Red
    Write-Host '     restrict who has admin/RDP access to this machine.' -ForegroundColor Red
    Write-Host '   - Rotate the passphrase and re-run this script if you' -ForegroundColor Red
    Write-Host '     ever suspect the machine has been compromised.' -ForegroundColor Red
    Write-Host $bar -ForegroundColor Red
}

# --------------------------------------------------------------------------
# Main
# --------------------------------------------------------------------------
function Main {
    if (-not (Test-IsAdministrator)) {
        Write-ErrAndThrow 'This script must be run from an elevated PowerShell (Run as Administrator).'
    }

    $platform = Get-Platform
    Write-Info "Detected platform: $platform"

    Install-Binary -Repo 'monetarium-node'   -BinaryName 'monetarium-node'   -Platform $platform
    Install-Binary -Repo 'monetarium-wallet' -BinaryName 'monetarium-wallet' -Platform $platform
    Install-Binary -Repo 'monetarium-ctl'    -BinaryName 'monetarium-ctl'    -Platform $platform

    Add-InstallDirToPath

    $passphrase = Read-WalletPassphrase
    $rpcUser = 'monetarium'
    $rpcSecret = Get-RandomToken -Length 32

    Write-Config -Passphrase $passphrase -RpcUser $rpcUser -RpcSecret $rpcSecret
    Remove-Variable -Name passphrase -ErrorAction SilentlyContinue

    Create-Wallet

    Install-ScheduledTask

    Write-Host ''
    Write-Host 'Installation complete.' -ForegroundColor Green
    Show-BigWarning
}

# Only auto-run when this file is executed directly.
# Dot-sourcing (". .\install.ps1") -- as the Pester tests do -- loads the
# functions above without invoking the installer.
if ($MyInvocation.InvocationName -ne '.' -and -not $NoExec) {
    Main
}
