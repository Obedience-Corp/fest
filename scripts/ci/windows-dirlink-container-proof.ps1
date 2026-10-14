#Requires -Version 7.0
<#
.SYNOPSIS
  CF0006 phase004 task02 native Windows proof: dirlink, config, and chaining container_fs tests
  plus fest.exe config CLI readback inside Windows Server Core as ContainerUser.

  Filesystem-mutating work runs only in container temp paths (never on the host workspace).
  Host must be Windows (windows-2025 CI). Do not run on macOS/Linux hosts.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$CommonPath = Join-Path $PSScriptRoot 'windows-servercore-container.ps1'
. $CommonPath

$ContainerImage = $script:FestServerCoreImage

function Compile-WindowsTestBinary {
    param(
        [string]$OutPath,
        [string]$Package
    )
    $env:CGO_ENABLED = '0'
    & go test -c -tags container_fs -o $OutPath $Package
    if ($LASTEXITCODE -ne 0) {
        throw "go test -c -tags container_fs -o $OutPath $Package failed with exit code $LASTEXITCODE"
    }
}

function Compile-FestBinary {
    param([string]$OutPath)
    $env:CGO_ENABLED = '0'
    & go build -o $OutPath ./cmd/fest
    if ($LASTEXITCODE -ne 0) {
        throw "go build -o $OutPath ./cmd/fest failed with exit code $LASTEXITCODE"
    }
}

try {
    Assert-WindowsProofHost
    Push-Location $RepoRoot

    Report-DockerServices
    Start-DockerServiceIfPresent
    Report-DockerCapability

    $workHost = Join-Path $env:TEMP ("fest-dirlink-proof-{0}" -f [guid]::NewGuid().ToString('n'))
    New-Item -ItemType Directory -Path $workHost | Out-Null
    $containerId = $null
    try {
        Write-ProofSection 'Compile Windows test binaries and fest.exe (host temp only)'
        $hostBin = Join-Path $workHost 'host-bin'
        New-Item -ItemType Directory -Path $hostBin | Out-Null
        Compile-WindowsTestBinary -OutPath (Join-Path $hostBin 'dirlink.test.exe') -Package './internal/dirlink'
        Compile-WindowsTestBinary -OutPath (Join-Path $hostBin 'config.test.exe') -Package './internal/config'
        Compile-WindowsTestBinary -OutPath (Join-Path $hostBin 'chaining.test.exe') -Package './internal/chaining'
        Compile-FestBinary -OutPath (Join-Path $hostBin 'fest.exe')

        $containerWork = 'C:\fest-dirlink-proof'
        $containerBin = "$containerWork\bin"
        $innerScriptHost = Join-Path $PSScriptRoot 'windows-dirlink-container-inner.ps1'
        $innerScriptContainer = "$containerWork\windows-dirlink-container-inner.ps1"

        Write-ProofSection 'Windows Server Core fixture (ContainerUser, isolated ACL in inner script)'
        $containerId = New-ServerCoreIdleContainer -Image $ContainerImage
        try {
            Invoke-DockerExecAsAdmin $containerId "mkdir $containerWork && mkdir $containerBin" | Out-Null
            Grant-ContainerUserFullControl -ContainerId $containerId -ContainerPath $containerWork

            Invoke-DockerCli -DockerArgs @(
                'cp', (Join-Path $hostBin 'dirlink.test.exe'), "${containerId}:${containerBin}\dirlink.test.exe"
            ) | Out-Null
            Invoke-DockerCli -DockerArgs @(
                'cp', (Join-Path $hostBin 'config.test.exe'), "${containerId}:${containerBin}\config.test.exe"
            ) | Out-Null
            Invoke-DockerCli -DockerArgs @(
                'cp', (Join-Path $hostBin 'chaining.test.exe'), "${containerId}:${containerBin}\chaining.test.exe"
            ) | Out-Null
            Invoke-DockerCli -DockerArgs @(
                'cp', (Join-Path $hostBin 'fest.exe'), "${containerId}:${containerWork}\fest.exe"
            ) | Out-Null
            Invoke-DockerCli -DockerArgs @(
                'cp', $innerScriptHost, "${containerId}:${innerScriptContainer}"
            ) | Out-Null

            $aclRoot = "$containerWork\config-acl-root"
            $aclRepos = "$aclRoot\config-repos"
            $aclDenySrc = "$containerWork\external-deny"
            $aclSetup = @(
                "mkdir `"$aclRepos`""
                "mkdir `"$aclDenySrc`""
                "icacls `"$aclRepos`" /deny ContainerUser:(OI)(CI)(W,M)"
            ) -join ' && '
            Invoke-DockerExecAsAdmin $containerId $aclSetup | Out-Null

            $ps51 = 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
            $innerCmd = @(
                "set `"HOME=$containerWork`""
                "set `"USERPROFILE=$containerWork`""
                "set `"TEMP=$containerWork\tmp`""
                "set `"TMP=$containerWork\tmp`""
                "mkdir $containerWork\tmp 2>nul"
                "`"$ps51`" -NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$innerScriptContainer`" -WorkRoot `"$containerWork`""
            ) -join ' && '

            Write-ProofSection 'Run dirlink/config/chaining proof as ContainerUser (PowerShell 5.1 in container)'
            $execOut = Invoke-DockerExecAsContainerUser $containerId $innerCmd
            Write-Host $execOut

            if ($execOut -notmatch 'Windows dirlink/config/chaining container proof: PASS') {
                throw "inner proof did not report PASS. Output:`n$execOut"
            }

            Write-Host ''
            Write-Host "CF0006 dirlink native proof: ContainerUser + $ContainerImage + container_fs tests + fest config CLI"
        } finally {
            Remove-ServerCoreContainer $containerId
            $containerId = $null
        }
    } finally {
        Remove-Item -LiteralPath $workHost -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-ProofSection 'Host workspace unchanged (compile used host TEMP only)'
    $dirty = & git status --porcelain 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "git status skipped or failed: $dirty"
    } elseif ($dirty) {
        Write-Host $dirty
        Write-Host 'Note: host git status is non-empty (expected if CI scripts are uncommitted); no FS tests ran on host workspace.'
    } else {
        Write-Host 'Host working tree clean.'
    }
} finally {
    if ($containerId) {
        Remove-ServerCoreContainer $containerId
    }
    Pop-Location
}
