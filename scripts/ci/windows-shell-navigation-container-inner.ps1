# Runs as ContainerUser in a fresh Windows PowerShell 5.1 process.
# All fixture writes stay inside the disposable Windows Server Core container.
param(
    [Parameter(Mandatory = $true)]
    [string]$WorkRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$festExe = Join-Path $WorkRoot 'fest.exe'
if (-not (Test-Path -LiteralPath $festExe -PathType Leaf)) {
    throw "missing built Fest executable: $festExe"
}
$env:PATH = "$WorkRoot$([IO.Path]::PathSeparator)$env:PATH"
$env:NO_COLOR = '1'

$campRoot = Join-Path $WorkRoot 'shell-camp'
$festivals = Join-Path $campRoot 'festivals'
$festivalName = 'shell-navigation'
$festival = Join-Path (Join-Path $festivals 'active') $festivalName
$unlinked = Join-Path (Join-Path $festivals 'active') 'without-link'
$project = Join-Path (Join-Path $campRoot 'projects') ('linked ' + [char]0x00E9 + ' project')
$env:CAMP_ROOT = $campRoot
$env:FEST_CONFIG_DIR = Join-Path $WorkRoot 'shell-config'

foreach ($directory in @(
    (Join-Path $campRoot '.campaign'), (Join-Path $festivals '.festival'),
    $festival, $unlinked, $project
)) {
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
}
Set-Content -LiteralPath (Join-Path $festival 'FESTIVAL_GOAL.md') -Value '# Shell navigation fixture' -Encoding ASCII
Set-Content -LiteralPath (Join-Path $unlinked 'FESTIVAL_GOAL.md') -Value '# Unlinked fixture' -Encoding ASCII

function Assert-Path {
    param([string]$Actual, [string]$Expected, [string]$Label)
    $actualPath = [IO.Path]::GetFullPath($Actual).TrimEnd('\')
    $expectedPath = [IO.Path]::GetFullPath($Expected).TrimEnd('\')
    if ($actualPath -ine $expectedPath) {
        throw "${Label}: got '$Actual', want '$Expected'"
    }
}

function Assert-Location {
    param([string]$Expected, [string]$Label)
    Assert-Path -Actual (Get-Location).Path -Expected $Expected -Label $Label
}

function Invoke-GoPrint {
    param([string[]]$GoArguments)
    $priorEncoding = [Console]::OutputEncoding
    try {
        [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
        $printed = @(& $festExe go @GoArguments --print)
        $code = $LASTEXITCODE
    } finally {
        [Console]::OutputEncoding = $priorEncoding
    }
    if ($code -ne 0 -or $printed.Count -ne 1 -or [string]::IsNullOrWhiteSpace([string]$printed[0])) {
        throw "fest go $($GoArguments -join ' ') --print returned $code or invalid stdout: $($printed -join '|')"
    }
    return [string]$printed[0]
}

function Invoke-FestStdout {
    param([string[]]$Arguments)
    $priorEncoding = [Console]::OutputEncoding
    try {
        [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
        $lines = @(& $festExe @Arguments)
        $code = $LASTEXITCODE
    } finally {
        [Console]::OutputEncoding = $priorEncoding
    }
    if ($code -ne 0) {
        throw "fest $($Arguments -join ' ') failed with exit code $code"
    }
    return [pscustomobject]@{ Lines = $lines; ExitCode = [int]$code }
}

function Invoke-FestFailure {
    param([string[]]$Arguments)
    $priorPreference = $ErrorActionPreference
    $priorEncoding = [Console]::OutputEncoding
    $ErrorActionPreference = 'Continue'
    try {
        [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
        $output = & $festExe @Arguments 2>&1 | Out-String
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $priorPreference
        [Console]::OutputEncoding = $priorEncoding
    }
    if ($code -eq 0) {
        throw "expected Fest to fail: $($Arguments -join ' ')"
    }
    return [string]$output
}

function Assert-FgoFailureKeepsLocation {
    param([string[]]$GoArguments, [string]$Label)
    $before = (Get-Location).Path
    $priorPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $failed = $false
    try {
        fgo @GoArguments | Out-Null
    } catch {
        $failed = $true
    } finally {
        $ErrorActionPreference = $priorPreference
    }
    if (-not $failed) {
        throw "${Label}: fgo reported success"
    }
    Assert-Location -Expected $before -Label "$Label changed location"
}

Write-Host '--- Fresh PowerShell 5.1 shell-init and native Fest navigation ---'
$initialEncoding = [Console]::OutputEncoding.WebName
$init = Invoke-FestStdout -Arguments @('shell-init', 'powershell')
$initLines = @($init.Lines)
if ($initLines.Count -eq 0) {
    throw 'fest shell-init powershell failed or returned an empty script'
}
Invoke-Expression ($initLines -join [Environment]::NewLine)
if (-not (Get-Command fgo -CommandType Function -ErrorAction SilentlyContinue)) {
    throw 'fgo was not installed by shell-init'
}

Set-Location -LiteralPath $campRoot
Assert-Path -Actual (Invoke-GoPrint -GoArguments @()) -Expected $festivals -Label 'camp fest go --print'
fgo
Assert-Location -Expected $festivals -Label 'fgo from camp'
Set-Location -LiteralPath $campRoot
fest go
Assert-Location -Expected $festivals -Label 'fest go wrapper without target from camp'
Write-Host 'camp go --print and fgo: OK'

Set-Location -LiteralPath $unlinked
$missingOutput = Invoke-FestFailure -Arguments @('go', 'project', '--print')
if ($missingOutput -notmatch '(?i)fest link') {
    throw "missing project link did not explain how to link: $missingOutput"
}
Assert-FgoFailureKeepsLocation -GoArguments @('project') -Label 'missing project link'
Write-Host 'missing-link hint and location preservation: OK'

Set-Location -LiteralPath $festival
$linkOutput = Invoke-FestStdout -Arguments @('link', $project, '--json')
$link = ($linkOutput.Lines -join [Environment]::NewLine) | ConvertFrom-Json
if (-not $link.success) {
    throw "fest link did not report success: $($linkOutput.Lines -join ' ')"
}
$navFile = Join-Path (Join-Path (Join-Path $campRoot '.campaign') 'fest') 'navigation.yaml'
if (-not (Test-Path -LiteralPath $navFile -PathType Leaf)) {
    throw "fest link did not persist navigation state: $navFile"
}
$readbackOutput = Invoke-FestStdout -Arguments @('link', '--show', '--json')
$readback = ($readbackOutput.Lines -join [Environment]::NewLine) | ConvertFrom-Json
if (-not $readback.linked) {
    throw 'separate fest.exe process did not read the persisted link'
}
Assert-Path -Actual $readback.project -Expected $project -Label 'persisted project link'
Write-Host 'fest link persisted and read back: OK'

Assert-Path -Actual (Invoke-GoPrint -GoArguments @('project')) -Expected $project -Label 'festival project --print'
Assert-Path -Actual (Invoke-GoPrint -GoArguments @()) -Expected $project -Label 'festival smart --print'
fgo project
Assert-Location -Expected $project -Label 'fgo project from festival'

Assert-Path -Actual (Invoke-GoPrint -GoArguments @('fest')) -Expected $festival -Label 'project fest --print'
Assert-Path -Actual (Invoke-GoPrint -GoArguments @()) -Expected $festival -Label 'project smart --print'
fgo fest
Assert-Location -Expected $festival -Label 'fgo fest from linked project'
Write-Host 'festival/project direct --print and fgo: OK'

fgo
Assert-Location -Expected $project -Label 'fgo smart from festival'
fgo
Assert-Location -Expected $festival -Label 'fgo smart from project'

Set-Location -LiteralPath $campRoot
fgo active $festivalName
Assert-Location -Expected $festival -Label 'fgo status target from camp'
Set-Location -LiteralPath $campRoot
fest go "active/$festivalName"
Assert-Location -Expected $festival -Label 'fest go wrapper from camp'
Write-Host 'smart navigation and status target: OK'

# Fest's link record intentionally remains, while the target disappears. The
# direct CLI still prints its stored path; fgo must refuse to cd into it.
Remove-Item -LiteralPath $project -Recurse -Force
Assert-Path -Actual (Invoke-GoPrint -GoArguments @('project')) -Expected $project -Label 'stale CLI link readback'
Assert-FgoFailureKeepsLocation -GoArguments @('project') -Label 'missing destination directory'
if ([Console]::OutputEncoding.WebName -ne $initialEncoding) {
    throw 'Fest navigation proof changed the session output encoding'
}

Write-Host 'Windows PowerShell 5.1 Fest shell navigation: PASS'
