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
. (Join-Path $PSScriptRoot 'windows-servercore-container.ps1')

$ContainerImage = $script:FestServerCoreImage
$RequiredTests = @(
    'TestOpenLocked_ClosesHandleWhenLockFails',
    'TestEmit_DoesNotReportSuccessfulWriteWhenLockFails',
    'TestEmit_CancelledContextDoesNotWrite',
    'TestMultiprocessConcurrentEmitAndRotation'
)
$RunLinePattern = '(?m)^=== RUN[ \t]+(Test[A-Za-z0-9_]+)\r?$'

try {
    Assert-WindowsProofHost
    Push-Location $RepoRoot

    Report-DockerServices
    Start-DockerServiceIfPresent
    Report-DockerCapability

    $workHost = Join-Path $env:TEMP ("fest-activity-lock-proof-{0}" -f [guid]::NewGuid().ToString('n'))
    New-Item -ItemType Directory -Path $workHost | Out-Null
    try {
        $binPath = Join-Path $workHost 'activity.test.exe'
        Write-ProofSection 'Compile internal/activity test binary'
        $env:CGO_ENABLED = '0'
        & go test -c -tags container_fs -o $binPath ./internal/activity
        if ($LASTEXITCODE -ne 0) {
            throw "go test -c -tags container_fs ./internal/activity failed with exit code $LASTEXITCODE"
        }

        $containerWork = 'C:\fest-activity-proof'
        $containerBin = "$containerWork\activity.test.exe"
        $containerLockRoot = "$containerWork\path-locks"

        Write-ProofSection 'Windows Server Core fixture'
        $containerId = New-ServerCoreIdleContainer -Image $ContainerImage

        try {
            Invoke-DockerExecAsAdmin $containerId "mkdir $containerWork" | Out-Null
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

            Write-ProofSection 'Run activity lock suite inside container'
            $execOut = Invoke-DockerExecAsAdmin $containerId $testCmd
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
            Remove-ServerCoreContainer $containerId
        }
    } finally {
        Remove-Item -LiteralPath $workHost -Recurse -Force -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}
