#Requires -Version 7.0
<#
.SYNOPSIS
  Compiles internal/activity multiproc/lock tests and runs them inside Windows Server Core.

  Filesystem-mutating tests run only on the container temp dir (never on the host workspace).
  Intended for the activity-lock-proof workflow on windows-2025 hosted runners.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$ContainerImage = 'mcr.microsoft.com/windows/servercore:ltsc2025'
$RequiredTests = @(
    'TestOpenLocked_ClosesHandleWhenLockFails',
    'TestEmit_DoesNotReportSuccessfulWriteWhenLockFails',
    'TestEmit_CancelledContextDoesNotWrite',
    'TestMultiprocessConcurrentEmitAndRotation'
)
$RunLinePattern = '(?m)^=== RUN[ \t]+(Test[A-Za-z0-9_]+)\r?$'

function Write-Section([string]$Title) {
    Write-Host ""
    Write-Host "=== $Title ==="
}

function Report-DockerServices {
    Write-Section 'Windows Docker services'
    $services = @(Get-Service -Name '*docker*' -ErrorAction SilentlyContinue)
    if ($services.Count -eq 0) {
        Write-Host 'No Windows services matching *docker* were found.'
        return
    }
    $services | Format-Table Name, Status, StartType -AutoSize | Out-String | Write-Host
}

function Start-DockerServiceIfPresent {
    foreach ($name in @('com.docker.service', 'docker')) {
        $svc = Get-Service -Name $name -ErrorAction SilentlyContinue
        if (-not $svc) { continue }
        if ($svc.Status -ne 'Running') {
            Write-Host "Starting service $name (was $($svc.Status))..."
            Start-Service -Name $name
            ($svc = Get-Service -Name $name).WaitForStatus('Running', (New-TimeSpan -Minutes 2))
        }
        Write-Host "Service $name is $($svc.Status)."
        return
    }
}

function Invoke-DockerCli([string[]]$DockerArgs) {
    & docker @DockerArgs
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($DockerArgs -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Ensure-WindowsContainerEngine {
    $dockerCli = Join-Path $env:ProgramFiles 'Docker\Docker\DockerCli.exe'
    $info = & docker info -f '{{.OSType}}' 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker info failed: $info"
    }
    $osType = ($info | Out-String).Trim()
    Write-Host "Docker daemon OSType: $osType"
    if ($osType -ieq 'windows') {
        return
    }
    if (-not (Test-Path -LiteralPath $dockerCli)) {
        throw @(
            "Docker daemon is in '$osType' mode; Windows container images require OSType=windows.",
            "DockerCli.exe was not found at $dockerCli.",
            "Hosted windows-2025 may not expose a switchable Windows engine — use a self-hosted",
            "Windows Server 2025 runner with Windows containers enabled (see D005 / 005 fixture lane)."
        ) -join ' '
    }
    Write-Host "Switching Docker Desktop to Windows containers via DockerCli.exe..."
    & $dockerCli -SwitchDaemon
    if ($LASTEXITCODE -ne 0) {
        throw "DockerCli.exe -SwitchDaemon failed with exit code $LASTEXITCODE"
    }
    Start-Sleep -Seconds 5
    $info = & docker info -f '{{.OSType}}' 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker info after engine switch failed: $info"
    }
    $osType = ($info | Out-String).Trim()
    Write-Host "Docker daemon OSType after switch: $osType"
    if ($osType -ine 'windows') {
        throw @(
            "Docker daemon is still '$osType' after -SwitchDaemon.",
            "Cannot run $ContainerImage on this runner without a Windows container engine."
        ) -join ' '
    }
}

function Report-DockerCapability {
    Write-Section 'Docker CLI capability'
    try {
        Invoke-DockerCli -DockerArgs @('version')
    } catch {
        Write-Host $_
        throw @(
            'Docker CLI cannot reach a daemon on this runner.',
            'If com.docker.service exists, start it; otherwise hosted windows-2025 may lack a',
            'runnable Windows container engine — use self-hosted Windows Server 2025 with Docker.'
        ) -join ' '
    }
    Invoke-DockerCli -DockerArgs @('info')
    Ensure-WindowsContainerEngine
}

function Invoke-DockerExec([string]$ContainerId, [string]$CmdLine) {
    $out = & docker exec $ContainerId cmd /c $CmdLine 2>&1
    $code = $LASTEXITCODE
    $text = $out | Out-String
    if ($code -ne 0) {
        throw "docker exec failed ($code): $text"
    }
    return $text
}

try {
    Push-Location $RepoRoot

    Report-DockerServices
    Start-DockerServiceIfPresent
    Report-DockerCapability

    $workHost = Join-Path $env:TEMP ("fest-activity-lock-proof-{0}" -f [guid]::NewGuid().ToString('n'))
    New-Item -ItemType Directory -Path $workHost | Out-Null
    try {
        $binPath = Join-Path $workHost 'activity.test.exe'
        Write-Section 'Compile internal/activity test binary'
        $env:CGO_ENABLED = '0'
        & go test -c -o $binPath ./internal/activity
        if ($LASTEXITCODE -ne 0) {
            throw "go test -c ./internal/activity failed with exit code $LASTEXITCODE"
        }

        $containerWork = 'C:\fest-activity-proof'
        $containerBin = "$containerWork\activity.test.exe"
        $containerLockRoot = "$containerWork\path-locks"

        Write-Section 'Windows Server Core fixture'
        Invoke-DockerCli -DockerArgs @('pull', $ContainerImage)
        $createOut = & docker create --pull=never $ContainerImage cmd /c "ping -n 3600 127.0.0.1 > nul" 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "docker create failed: $($createOut | Out-String)"
        }
        $containerId = ($createOut | Out-String).Trim()
        if ($containerId -notmatch '^[0-9a-f]{64}$') {
            throw "docker create returned an invalid container id: $containerId"
        }
        Write-Host "Container id: $containerId"

        try {
            Invoke-DockerCli -DockerArgs @('start', $containerId) | Out-Null
            Invoke-DockerExec $containerId "mkdir $containerWork" | Out-Null
            Invoke-DockerCli -DockerArgs @('cp', $binPath, "${containerId}:${containerBin}") | Out-Null

            $testCmd = @(
                "set `"HOME=$containerWork`""
                "set `"USERPROFILE=$containerWork`""
                "set `"TEMP=$containerWork\tmp`""
                "set `"TMP=$containerWork\tmp`""
                "set `"FEST_ACTIVITY_PATH_LOCK_DIR=$containerLockRoot`""
                "mkdir $containerWork\tmp"
                "mkdir $containerLockRoot"
                "cd /d $containerWork"
                "$containerBin -test.v -test.count=1 -test.timeout=5m"
            ) -join ' && '

            Write-Section 'Run activity lock suite inside container'
            $execOut = Invoke-DockerExec $containerId $testCmd
            Write-Host $execOut

            if ($execOut -notmatch 'PASS') {
                throw "activity suite output did not contain PASS"
            }
            $runs = [regex]::Matches($execOut, $RunLinePattern)
            foreach ($testName in $RequiredTests) {
                if (-not $execOut.Contains("=== RUN   $testName") -or
                    -not $execOut.Contains("--- PASS: $testName")) {
                    throw "required activity test $testName did not run and pass. Output:`n$execOut"
                }
            }
            if ($runs.Count -lt $RequiredTests.Count) {
                throw "expected at least $($RequiredTests.Count) activity tests; saw $($runs.Count). Output:`n$execOut"
            }
            Write-Host ""
            Write-Host "Windows container activity lock proof: $($runs.Count) tests executed in $ContainerImage"
        } finally {
            & docker rm -f $containerId 2>&1 | Out-Null
        }
    } finally {
        Remove-Item -LiteralPath $workHost -Recurse -Force -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}
