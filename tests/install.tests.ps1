#requires -Modules Pester

<#
.SYNOPSIS
    Pester tests for install.ps1 (Windows).

.DESCRIPTION
    Dot-sources install.ps1 (which, thanks to the InvocationName guard at
    the bottom of that file, only loads its functions and does not run
    Main). Network calls (Invoke-RestMethod / Invoke-WebRequest) and the
    real Windows-elevation check / Task Scheduler registration are mocked
    so the test never touches the real monetarium GitHub releases or the
    machine's actual scheduled tasks. Everything else (file writes, ACLs,
    zip extraction, config content) runs for real against a throwaway
    temp directory.
#>

BeforeAll {
    $Script:InstallScript = Join-Path $PSScriptRoot '..' 'install.ps1'
    . $Script:InstallScript -NoExec
}

Describe 'install.ps1' {

    BeforeEach {
        $Script:TestRoot = Join-Path $env:TEMP ([guid]::NewGuid())
        New-Item -ItemType Directory -Path $Script:TestRoot -Force | Out-Null

        # Redirect install/data dirs into the throwaway root instead of
        # Program Files / ProgramData.
        $Script:InstallDir = Join-Path $Script:TestRoot 'bin'
        $Script:DataDir    = Join-Path $Script:TestRoot 'data'
        $Script:WalletConf = Join-Path $Script:DataDir 'monetarium-wallet.conf'
        $Script:NodeConf   = Join-Path $Script:DataDir 'monetarium.conf'

        Mock Test-IsAdministrator { return $true }

        # Fake "latest release" API response: always one asset matching
        # whatever platform string was requested.
        Mock Invoke-RestMethod {
            param($Uri)
            if ($Uri -notmatch '/repos/[^/]+/([^/]+)/releases/latest') {
                throw "Unexpected Invoke-RestMethod call: $Uri"
            }
            $repo = $Matches[1]
            [PSCustomObject]@{
                assets = @(
                    [PSCustomObject]@{
                        name                = "${repo}-windows-amd64.zip"
                        browser_download_url = "https://fixtures.test/${repo}-windows-amd64.zip"
                    }
                )
            }
        }

        # Fake download: build a small real zip containing a stub .exe,
        # instead of hitting the network.
        Mock Invoke-WebRequest {
            param($Uri, $OutFile)
            if ($Uri -notmatch '/([a-zA-Z0-9_-]+)-windows-amd64\.zip$') {
                throw "Unexpected Invoke-WebRequest call: $Uri"
            }
            $binName = $Matches[1]
            $tmp = Join-Path $env:TEMP ([guid]::NewGuid())
            New-Item -ItemType Directory -Path $tmp | Out-Null
            Set-Content -Path (Join-Path $tmp "$binName.exe") -Value "mock exe for $binName"
            Compress-Archive -Path (Join-Path $tmp "$binName.exe") -DestinationPath $OutFile -Force
            Remove-Item -Path $tmp -Recurse -Force
        }

        # Don't touch the real Task Scheduler.
        Mock Register-ScheduledTask { }
        Mock Start-ScheduledTask { }

        # Wallet --create is tested end-to-end via Linux E2E; in Pester
        # the stub .exe isn't a real binary, so mock it as a no-op.
        Mock Create-Wallet { }
    }

    AfterEach {
        Remove-Item -Path $Script:TestRoot -Recurse -Force -ErrorAction SilentlyContinue
    }

    Context 'normal flow' {
        BeforeEach {
            Mock Read-Host {
                ConvertTo-SecureString -String 'test-passphrase-123' -AsPlainText -Force
            }
        }

        It 'refuses to run when not elevated' {
            Mock Test-IsAdministrator { return $false }
            { Main } | Should -Throw '*elevated*'
        }

        It 'downloads and installs all three binaries' {
            Main
            Test-Path (Join-Path $Script:InstallDir 'monetarium-node.exe')   | Should -BeTrue
            Test-Path (Join-Path $Script:InstallDir 'monetarium-wallet.exe') | Should -BeTrue
            Test-Path (Join-Path $Script:InstallDir 'monetarium-ctl.exe')    | Should -BeTrue
        }

        It 'writes a wallet config containing the passphrase and staking flags' {
            Main
            $content = Get-Content -Path $Script:WalletConf -Raw
            $content | Should -Match 'pass=test-passphrase-123'
            $content | Should -Match 'enablevoting=1'
            $content | Should -Match 'enableticketbuyer=1'
        }

        It 'writes a node config with rpc credentials' {
            Main
            $content = Get-Content -Path $Script:NodeConf -Raw
            $content | Should -Match 'rpcuser=monetarium'
            $content | Should -Match 'rpcpass='
        }

        It 'does not leave the passphrase visible in Main''s own console output' {
            $output = Main 6>&1 5>&1 3>&1 | Out-String
            $output | Should -Not -Match 'test-passphrase-123'
        }

        It 'locks down the wallet config ACL so only current user/SYSTEM/Administrators have access' {
            Main
            $acl = Get-Acl -Path $Script:WalletConf
            $identities = $acl.Access | ForEach-Object { $_.IdentityReference.Value }
            ($identities -join ';') | Should -Match 'SYSTEM'
            ($identities -join ';') | Should -Match 'Administrators'
        }

        It 'registers exactly two scheduled tasks, one for node and one for wallet' {
            Main
            Should -Invoke Register-ScheduledTask -Times 2 -Exactly
            Should -Invoke Register-ScheduledTask -ParameterFilter { $TaskName -eq 'MonetariumNode' }
            Should -Invoke Register-ScheduledTask -ParameterFilter { $TaskName -eq 'MonetariumWallet' }
        }

        It 'starts both scheduled tasks after registering them' {
            Main
            Should -Invoke Start-ScheduledTask -Times 2 -Exactly
        }

        It 'prints the big warning banner' {
            $output = Main 6>&1 | Out-String
            $output | Should -Match 'WARNING'
            $output | Should -Match 'PLAINTEXT'
        }
    }
}

Describe 'Get-Platform' {
    It 'maps AMD64 to windows-amd64' {
        $env:PROCESSOR_ARCHITECTURE = 'AMD64'
        Get-Platform | Should -Be 'windows-amd64'
    }

    It 'maps ARM64 to windows-arm64' {
        $env:PROCESSOR_ARCHITECTURE = 'ARM64'
        Get-Platform | Should -Be 'windows-arm64'
    }
}

Describe 'ConvertFrom-SecureStringToPlainText' {
    It 'round-trips a plaintext value through a SecureString' {
        $secure = ConvertTo-SecureString -String 'hello world' -AsPlainText -Force
        ConvertFrom-SecureStringToPlainText -SecureString $secure | Should -Be 'hello world'
    }
}
