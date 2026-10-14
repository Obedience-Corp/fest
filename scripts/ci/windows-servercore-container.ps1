# Shared Windows Server Core container helpers for fest CI proof scripts.
# Host: PowerShell 7+. In-container steps should use scripts that target Windows PowerShell 5.1.
Set-StrictMode -Version Latest

$script:FestServerCoreImage = 'mcr.microsoft.com/windows/servercore:ltsc2025'

function Write-ProofSection([string]$Title) {
    Write-Host ""
    Write-Host "=== $Title ==="
}

function Assert-WindowsProofHost {
    if ($env:OS -ne 'Windows_NT') {
        throw @(
            'This proof runs only on a Windows host (e.g. windows-2025 CI).',
            'It starts Windows containers and must not mutate the host workspace for filesystem tests.',
            'On Unix/macOS use compile-only gates (just test windows-dirlink-cross-compile-gate).'
        ) -join ' '
    }
}

function Report-DockerServices {
    Write-ProofSection 'Windows Docker services'
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
            ($svc = Get-Service -Name $name).WaitForStatus('Running', (New-TimeSpan -Minutes  2))
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
            'Hosted windows-2025 may not expose a switchable Windows engine — use a self-hosted',
            'Windows Server 2025 runner with Windows containers enabled (see D005 / 005 fixture lane).'
        ) -join ' '
    }
    Write-Host 'Switching Docker Desktop to Windows containers via DockerCli.exe...'
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
            "Cannot run $script:FestServerCoreImage on this runner without a Windows container engine."
        ) -join ' '
    }
}

function Report-DockerCapability {
    Write-ProofSection 'Docker CLI capability'
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

function Invoke-DockerExecAsAdmin([string]$ContainerId, [string]$CmdLine) {
    $out = & docker exec $ContainerId cmd /c $CmdLine 2>&1
    $code = $LASTEXITCODE
    $text = $out | Out-String
    if ($code -ne 0) {
        throw "docker exec (admin) failed ($code): $text"
    }
    return $text
}

function Invoke-DockerExecAsContainerUser([string]$ContainerId, [string]$CmdLine) {
    $out = & docker exec -u ContainerUser $ContainerId cmd /c $CmdLine 2>&1
    $code = $LASTEXITCODE
    $text = $out | Out-String
    if ($code -ne 0) {
        throw "docker exec (ContainerUser) failed ($code): $text"
    }
    return $text
}

function New-ServerCoreIdleContainer([string]$Image = $script:FestServerCoreImage) {
    Invoke-DockerCli -DockerArgs @('pull', $Image) | Out-Null
    $createOut = & docker create --pull=never $Image cmd /c "ping -n 3600 127.0.0.1 > nul" 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker create failed: $($createOut | Out-String)"
    }
    $containerId = ($createOut | Out-String).Trim()
    if ($containerId -notmatch '^[0-9a-f]{64}$') {
        throw "docker create returned an invalid container id: $containerId"
    }
    Write-Host "Container id: $containerId"
    Invoke-DockerCli -DockerArgs @('start', $containerId) | Out-Null
    return $containerId
}

function Remove-ServerCoreContainer([string]$ContainerId) {
    if ($ContainerId) {
        & docker rm -f $ContainerId 2>&1 | Out-Null
    }
}

function Get-ContainerUserSid([string]$ContainerId) {
    $identity = Invoke-DockerExecAsContainerUser $ContainerId 'whoami /user /fo csv /nh'
    if ($identity -notmatch '(S-\d+(?:-\d+)+)') {
        throw "cannot resolve ContainerUser SID: $identity"
    }
    return $Matches[1]
}

function Grant-ContainerUserModify([string]$ContainerId, [string]$ContainerPath, [string]$UserSid) {
    $cmd = "icacls `"$ContainerPath`" /grant `"*${UserSid}:(OI)(CI)M`" /T"
    Invoke-DockerExecAsAdmin $ContainerId $cmd | Out-Null
}
