# Runs inside Windows Server Core as ContainerUser (Windows PowerShell 5.1 compatible).
param(
    [Parameter(Mandatory = $true)]
    [string]$WorkRoot
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$script:ReparseTagMountPoint = '0xa0000003'

$script:RequiredDirlinkTests = @(
    'TestCreateRemoveReadback',
    'TestRemoveRejectsPlainDirectory',
    'TestReplaceRollbackOnFinalizeError',
    'TestReplaceConfigStyleReadback',
    'TestUnicodeAndSpaceLinkSafeRemoval',
    'TestBrokenAndCyclicLinkSafeRemoval',
    'TestOfflineUNCTargetUsesDirectorySymlink'
)
$script:RequiredConfigTests = @(
    'TestRepoManagerConfigLinkLifecycle',
    'TestRepoManagerUseRecreatesMissingActiveLink',
    'TestRepoManagerRemoveSaveFailureRestoresActiveLinks',
    'TestRepoManagerRejectsCaseOnlyNameCollision'
)
$script:RequiredChainingTests = @(
    'TestCheckChaining',
    'TestLinkPhaseInput',
    'TestLinkPhaseInputFailureSurfaces',
    'TestFindCreatedPhaseDir',
    'TestIsWithinFestival',
    'TestExecuteChainLinkFailureThenRetry',
    'TestExecuteChainUnrelatedSameNamePhaseCollision',
    'TestExecuteChainConfigSaveFailureThenRetry'
)

function Write-Step([string]$Message) {
    Write-Host ""
    Write-Host "--- $Message ---"
}

function Assert-RequiredTestsPassed {
    param(
        [string]$Label,
        [string]$Output,
        [string[]]$Required
    )
    if ($Output -notmatch 'PASS') {
        throw "$Label output did not contain PASS"
    }
    foreach ($name in $Required) {
        if ($Output -match "--- SKIP:\s+$name") {
            throw "required test $name was SKIPPED in $Label"
        }
        if (-not ($Output.Contains("--- PASS: $name"))) {
            throw "required test $name did not PASS in $Label. Output:`n$Output"
        }
    }
    Write-Host "$Label`: required tests passed ($($Required.Count))"
}

function Get-TreeFingerprint {
    param([string]$Root)
    if (-not (Test-Path -LiteralPath $Root)) {
        return @()
    }
    $items = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue
    $rows = @()
    foreach ($f in $items) {
        $rel = $f.FullName.Substring($Root.Length).TrimStart('\')
        $hash = (Get-FileHash -LiteralPath $f.FullName -Algorithm SHA256).Hash
        $rows += ('{0}|{1}|{2}' -f $rel, $f.Length, $hash)
    }
    return ($rows | Sort-Object)
}

function Assert-ReparseIsLocalJunction {
    param([string]$LinkPath)
    if (-not (Test-Path -LiteralPath $LinkPath)) {
        throw "link path missing: $LinkPath"
    }
    $rp = & fsutil reparsepoint query $LinkPath 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "fsutil reparsepoint query failed for ${LinkPath}: $rp"
    }
    $text = $rp | Out-String
    if ($text -notmatch '(?i)Reparse Tag Value\s*:\s*0xa0000003') {
        throw "expected IO_REPARSE_TAG_MOUNT_POINT (junction) at $LinkPath; fsutil:`n$text"
    }
    Write-Host "reparse tag OK (local junction): $LinkPath"
}

function Invoke-GoTestBinary {
    param(
        [string]$ExePath,
        [string]$Label
    )
    Write-Step "go test binary: $Label"
    $testArgs = @('-test.v', '-test.count=1', '-test.timeout=5m')
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & $ExePath @testArgs 2>&1
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    $text = $out | Out-String
    Write-Host $text
    if ($code -ne 0) {
        throw "$Label failed with exit code $code"
    }
    return $text
}

function Invoke-FestCapture {
    param([string[]]$Arguments)
    $fest = Join-Path $WorkRoot 'fest.exe'
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $fest @Arguments 2>&1 | Out-String
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousPreference
    }
    return [pscustomobject]@{ ExitCode = [int]$code; Output = [string]$output }
}

function Invoke-Fest {
    param([string[]]$Arguments)
    $run = Invoke-FestCapture -Arguments $Arguments
    if ($run.Output) { Write-Host $run.Output }
    return $run.ExitCode
}

function Invoke-FestJsonShow {
    $run = Invoke-FestCapture -Arguments @('config', 'show', '--json')
    if ($run.ExitCode -ne 0) {
        throw "fest config show --json failed: $($run.Output)"
    }
    return ($run.Output | ConvertFrom-Json)
}

function Invoke-ConfigCLIProof {
    Write-Step 'fest.exe config add / use / switch / new-process readback'
    $cfgRoot = Join-Path $WorkRoot 'config-root'
    $srcA = Join-Path $WorkRoot 'external-a'
    $srcB = Join-Path $WorkRoot 'external-b'
    New-Item -ItemType Directory -Path (Join-Path $srcA 'user') -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $srcB 'user') -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $srcA 'user\settings.json') -Value '{"repo":"a"}' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $srcB 'user\settings.json') -Value '{"repo":"b"}' -Encoding UTF8
    $fpA = Get-TreeFingerprint -Root $srcA
    $fpB = Get-TreeFingerprint -Root $srcB

    $env:FEST_CONFIG_DIR = $cfgRoot
    $env:HOME = $WorkRoot
    $env:USERPROFILE = $WorkRoot

    if ((Invoke-Fest -Arguments @('config', 'add', 'repo-a', $srcA)) -ne 0) {
        throw 'fest config add repo-a failed'
    }
    if ((Invoke-Fest -Arguments @('config', 'use', 'repo-a')) -ne 0) {
        throw 'fest config use repo-a failed'
    }

    $show = Invoke-FestJsonShow
    if ($show.active -ne 'repo-a') {
        throw "active after use repo-a = $($show.active), want repo-a"
    }

    $activePath = $show.active_path
    Assert-ReparseIsLocalJunction -LinkPath $activePath

    $readA = Get-Content -LiteralPath (Join-Path $activePath 'user\settings.json') -Raw
    if ($readA -notmatch 'repo-a') {
        throw "readback through active link failed: $readA"
    }

    # New process readback (simulates restart).
    $fest = Join-Path $WorkRoot 'fest.exe'
    $procOut = & cmd /c "set FEST_CONFIG_DIR=$cfgRoot&& set HOME=$WorkRoot&& set USERPROFILE=$WorkRoot&& `"$fest`" config show --json" 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "new-process fest config show failed: $procOut"
    }
    $procShow = ($procOut | Out-String | ConvertFrom-Json)
    if ($procShow.active -ne 'repo-a') {
        throw "new-process active = $($procShow.active), want repo-a"
    }

    if ((Invoke-Fest -Arguments @('config', 'add', 'repo-b', $srcB)) -ne 0) {
        throw 'fest config add repo-b failed'
    }
    if ((Invoke-Fest -Arguments @('config', 'use', 'repo-b')) -ne 0) {
        throw 'fest config use repo-b failed'
    }
    $showB = Invoke-FestJsonShow
    if ($showB.active -ne 'repo-b') {
        throw "active after switch = $($showB.active), want repo-b"
    }
    Assert-ReparseIsLocalJunction -LinkPath $showB.active_path
    $readB = Get-Content -LiteralPath (Join-Path $showB.active_path 'user\settings.json') -Raw
    if ($readB -notmatch 'repo-b') {
        throw "readback after switch failed: $readB"
    }

    if ((Invoke-Fest -Arguments @('config', 'remove', 'repo-b', '--force')) -ne 0) {
        throw 'fest config remove repo-b failed'
    }
    if ((Invoke-Fest -Arguments @('config', 'remove', 'repo-a', '--force')) -ne 0) {
        throw 'fest config remove repo-a failed'
    }

    $fpAfterA = Get-TreeFingerprint -Root $srcA
    $fpAfterB = Get-TreeFingerprint -Root $srcB
    if (($fpA -join "`n") -ne ($fpAfterA -join "`n")) {
        throw 'external source tree A changed after removing managed links'
    }
    if (($fpB -join "`n") -ne ($fpAfterB -join "`n")) {
        throw 'external source tree B changed after removing managed links'
    }
    Write-Host 'config CLI: junction readback, switch, new-process readback, and source inventory OK'
}

function Invoke-ConfigCLIFailureProof {
    Write-Step 'fest.exe failure cases (no false active / no false success on collision)'
    $cfgRoot = Join-Path $WorkRoot 'config-fail-root'
    $src = Join-Path $WorkRoot 'external-fail'
    New-Item -ItemType Directory -Path (Join-Path $src 'user') -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $src 'user\settings.json') -Value '{}' -Encoding UTF8
    $env:FEST_CONFIG_DIR = $cfgRoot
    $env:HOME = $WorkRoot
    $env:USERPROFILE = $WorkRoot

    if ((Invoke-Fest -Arguments @('config', 'add', 'once', $src)) -ne 0) {
        throw 'setup add once failed'
    }
    if ((Invoke-Fest -Arguments @('config', 'use', 'once')) -ne 0) {
        throw 'setup use once failed'
    }

    $dupRun = Invoke-FestCapture -Arguments @('config', 'add', 'once', $src)
    if ($dupRun.ExitCode -eq 0) {
        throw "duplicate config add should fail; output: $($dupRun.Output)"
    }
    $show = Invoke-FestJsonShow
    if ($show.active -ne 'once') {
        throw "duplicate add must not change active; got $($show.active)"
    }
    if ($dupRun.Output -match '(?i)Active config set') {
        throw 'duplicate add printed false success messaging'
    }

    # Corrupt active path so Use cannot replace the link; manifest must roll back.
    $activePath = $show.active_path
    if (Test-Path -LiteralPath $activePath) {
        & cmd /c "rmdir `"$activePath`"" | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "remove active junction entry failed: $activePath"
        }
    }
    New-Item -ItemType Directory -Path $activePath -Force | Out-Null

    $src2 = Join-Path $WorkRoot 'external-fail-b'
    New-Item -ItemType Directory -Path (Join-Path $src2 'user') -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $src2 'user\settings.json') -Value '{}' -Encoding UTF8
    if ((Invoke-Fest -Arguments @('config', 'add', 'two', $src2)) -ne 0) {
        throw 'add two failed'
    }
    $useRun = Invoke-FestCapture -Arguments @('config', 'use', 'two')
    if ($useRun.ExitCode -eq 0) {
        throw "config use should fail when active path is a plain directory; output: $($useRun.Output)"
    }
    $after = Invoke-FestJsonShow
    if ($after.active -ne 'once') {
        throw "Use failure must not commit active=two; got $($after.active)"
    }
    if ($useRun.Output -match '(?i)Active config set to') {
        throw 'failed Use printed false success messaging'
    }
    Write-Host 'config CLI failure cases OK'
}

function Invoke-AclDeniedAddProof {
    Write-Step 'ACL denied config repo link creation (admin-prepared fixture)'
    $cfgRoot = Join-Path $WorkRoot 'config-acl-root'
    $denyTarget = Join-Path $WorkRoot 'external-deny'
    if (-not (Test-Path -LiteralPath $cfgRoot)) {
        throw 'missing admin-prepared config-acl-root fixture'
    }
    if (-not (Test-Path -LiteralPath $denyTarget)) {
        throw 'missing external-deny source directory'
    }

    $env:FEST_CONFIG_DIR = $cfgRoot
    $env:HOME = $WorkRoot
    $env:USERPROFILE = $WorkRoot

    $deniedRun = Invoke-FestCapture -Arguments @('config', 'add', 'denied', $denyTarget)
    if ($deniedRun.ExitCode -eq 0) {
        throw "config add should fail under deny ACL; output: $($deniedRun.Output)"
    }
    $show = Invoke-FestJsonShow
    if ($show.active -eq 'denied') {
        throw 'denied add must not mark config active'
    }
    if ($deniedRun.Output -match '(?i)Active config set') {
        throw 'denied add printed false success messaging'
    }
    Write-Host 'ACL denied add case OK'
}

Write-Step "fixture root: $WorkRoot"
$binDir = Join-Path $WorkRoot 'bin'
$dirlinkOut = Invoke-GoTestBinary -ExePath (Join-Path $binDir 'dirlink.test.exe') -Label 'dirlink'
Assert-RequiredTestsPassed -Label 'dirlink' -Output $dirlinkOut -Required $script:RequiredDirlinkTests

$configOut = Invoke-GoTestBinary -ExePath (Join-Path $binDir 'config.test.exe') -Label 'config'
Assert-RequiredTestsPassed -Label 'config' -Output $configOut -Required $script:RequiredConfigTests

$chainingOut = Invoke-GoTestBinary -ExePath (Join-Path $binDir 'chaining.test.exe') -Label 'chaining'
Assert-RequiredTestsPassed -Label 'chaining' -Output $chainingOut -Required $script:RequiredChainingTests

Invoke-ConfigCLIProof
Invoke-ConfigCLIFailureProof
Invoke-AclDeniedAddProof

Write-Host ''
Write-Host 'NOTE: Phase input chaining after festival restart is proven via chaining.test.exe'
Write-Host '(TestLinkPhaseInput*, TestLinkPhaseInputFailureSurfaces), not a full fest workflow CLI journey.'

Write-Host ''
Write-Host 'Windows dirlink/config/chaining container proof: PASS'
