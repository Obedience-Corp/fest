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

function Get-HostGitRoot {
    $gitCmd = (Get-Command git.exe -ErrorAction Stop).Source
    for ($dir = Split-Path -Parent $gitCmd; $dir; $dir = Split-Path -Parent $dir) {
        if (Test-Path -LiteralPath (Join-Path $dir 'cmd\git.exe')) {
            return $dir
        }
        $parent = Split-Path -Parent $dir
        if ($parent -eq $dir) { break }
    }
    throw "host git layout unexpected at $gitCmd (no parent with cmd\git.exe)"
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
            Invoke-DockerExecAsAdmin $containerId "mkdir $containerWork && mkdir $containerBin && mkdir $containerWork\tmp" | Out-Null
            $userSid = Get-ContainerUserSid -ContainerId $containerId
            Write-Host "ContainerUser SID: $userSid"
            Grant-ContainerUserModify -ContainerId $containerId -ContainerPath $containerWork -UserSid $userSid

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
            $gitRoot = Get-HostGitRoot
            Write-Host "Copying host Git from $gitRoot into container for config repository tests"
            Invoke-DockerCli -DockerArgs @(
                'cp', $gitRoot, "${containerId}:$containerWork\Git"
            ) | Out-Null

            $aclRoot = "$containerWork\config-acl-root"
            $aclRepos = "$aclRoot\config-repos"
            $aclDenySrc = "$containerWork\external-deny"
            Invoke-DockerExecAsAdmin $containerId "mkdir $aclRepos" | Out-Null
            Invoke-DockerExecAsAdmin $containerId "mkdir $aclDenySrc" | Out-Null
            Invoke-DockerCli -DockerArgs @('exec', $containerId, 'icacls.exe', $aclRepos,
                '/deny', "*${userSid}:(OI)(CI)M") | Out-Null

            $ps51 = 'C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe'
            $innerArgs = @(
                'exec', '-u', 'ContainerUser',
                '-e', "HOME=$containerWork",
                '-e', "USERPROFILE=$containerWork",
                '-e', "TEMP=$containerWork\tmp",
                '-e', "TMP=$containerWork\tmp",
                $containerId, $ps51, '-NoProfile', '-NonInteractive',
                '-ExecutionPolicy', 'Bypass', '-File', $innerScriptContainer,
                '-WorkRoot', $containerWork
            )

            Write-ProofSection 'Run dirlink/config/chaining proof as ContainerUser (PowerShell 5.1 in container)'
            $rawInnerOut = & docker @innerArgs 2>&1
            $innerExitCode = $LASTEXITCODE
            $execOut = $rawInnerOut | Out-String
            Write-Host $execOut
            if ($innerExitCode -ne 0) {
                throw "inner native proof failed ($innerExitCode); see container output above"
            }

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
