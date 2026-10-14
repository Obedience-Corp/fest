# Fest shell integration for Windows PowerShell 5.1 and PowerShell 7.
# Add to $PROFILE: fest shell-init powershell | Out-String | Invoke-Expression
# Provides fgo, fls, and a cd-aware fest go wrapper.

function _fest_native_executable {
    # The fest function below shadows the command name. Resolve the native
    # executable explicitly so every wrapper invokes the CLI, not itself.
    $native = Get-Command fest -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($null -eq $native) {
        throw 'fest: executable not found in PATH'
    }
    return $native.Source
}

function _fest_invoke {
    param([string[]] $Arguments)
    $native = _fest_native_executable
    # Go writes UTF-8 to pipes. Windows PowerShell 5.1 otherwise decodes
    # native output with the console code page, corrupting Unicode paths.
    $previousEncoding = [Console]::OutputEncoding
    try {
        [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
        & $native @Arguments
        $exitCode = $LASTEXITCODE
    } finally {
        [Console]::OutputEncoding = $previousEncoding
    }
    if ($exitCode -ne 0) {
        throw "fest: command failed with exit code $exitCode"
    }
}

function _fest_go_to {
    param([string[]] $GoArguments)
    $native = _fest_native_executable
    # Capture only stdout. Fest writes interactive pickers and errors to stderr.
    $previousEncoding = [Console]::OutputEncoding
    try {
        [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
        $destinations = @(& $native go @GoArguments --print)
        $exitCode = $LASTEXITCODE
    } finally {
        [Console]::OutputEncoding = $previousEncoding
    }
    if ($exitCode -ne 0) {
        throw "fgo: navigation failed with exit code $exitCode; location unchanged"
    }
    if ($destinations.Count -ne 1 -or [string]::IsNullOrWhiteSpace([string] $destinations[0])) {
        throw 'fgo: Fest did not return one destination; location unchanged'
    }
    $destination = [string] $destinations[0]
    if (-not (Test-Path -LiteralPath $destination -PathType Container)) {
        throw "fgo: destination is not a directory: $destination"
    }
    Set-Location -LiteralPath $destination -ErrorAction Stop
}

function _fest_status_target {
    param([object[]] $Arguments)
    if ($Arguments.Count -ge 2 -and $Arguments[0] -in @(
        'active', 'planning', 'ready', 'ritual',
        'completed', 'someday', 'archived', 'dungeon')) {
        return @("$($Arguments[0])/$($Arguments[1])")
    }
    return $Arguments
}

function fgo {
    $arguments = @($args)
    $first = if ($arguments.Count -gt 0) { [string] $arguments[0] } else { '' }
    switch ($first) {
        { $_ -in @('--help', '-h', 'help') } {
            _fest_invoke -Arguments @('go', '--help')
            return
        }
        'link' {
            _fest_invoke -Arguments (@('go') + $arguments)
            return
        }
        'unlink' {
            _fest_invoke -Arguments @('unlink')
            return
        }
        { $_ -in @('map', 'unmap') } {
            _fest_invoke -Arguments (@('go') + $arguments)
            return
        }
        'list' {
            $native = _fest_native_executable
            $previousEncoding = [Console]::OutputEncoding
            try {
                [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
                $destinations = @(& $native go list --interactive --print)
                $exitCode = $LASTEXITCODE
            } finally {
                [Console]::OutputEncoding = $previousEncoding
            }
            if ($exitCode -ne 0) {
                _fest_invoke -Arguments @('go', 'list')
                return
            }
            if ($destinations.Count -eq 0) { return } # Picker cancelled.
            if ($destinations.Count -ne 1 -or
                -not (Test-Path -LiteralPath ([string] $destinations[0]) -PathType Container)) {
                throw 'fgo: picker returned an invalid destination; location unchanged'
            }
            Set-Location -LiteralPath ([string] $destinations[0]) -ErrorAction Stop
            return
        }
        { $_ -in @('project', 'fest') } {
            _fest_go_to -GoArguments @($first)
            return
        }
        default {
            if ($first.StartsWith('-')) {
                _fest_go_to -GoArguments @('shortcut', $first.Substring(1))
                return
            }
            $target = @(_fest_status_target -Arguments $arguments)
            _fest_go_to -GoArguments $target
            return
        }
    }
}

function fls {
    $arguments = @($args)
    if ($arguments.Count -gt 0 -and $arguments[0] -in @('--help', '-h', 'help')) {
        _fest_invoke -Arguments @('list', '--help')
        return
    }
    _fest_invoke -Arguments (@('list') + $arguments)
}

function fest {
    $arguments = @($args)
    if ($arguments.Count -eq 0 -or $arguments[0] -notin @('go', 'g')) {
        _fest_invoke -Arguments $arguments
        return
    }
    $goArguments = @()
    if ($arguments.Count -gt 1) {
        $goArguments = @($arguments[1..($arguments.Count - 1)])
    }
    $first = if ($goArguments.Count -gt 0) { [string] $goArguments[0] } else { '' }
    if ($first -in @('--help', '-h', 'help', 'link', 'unlink', 'map', 'unmap', 'move', 'completions')) {
        _fest_invoke -Arguments (@('go') + $goArguments)
        return
    }
    $target = @(_fest_status_target -Arguments $goArguments)
    _fest_go_to -GoArguments $target
}
