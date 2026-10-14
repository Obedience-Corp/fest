package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This runs the emitted code in a child shell with a mocked Fest command.
// It changes only that process's working directory and requires no fixture
// files. The native Windows journey separately exercises the real fest.exe.
func TestPowerShellNavigationUsesCLIResultSafely(t *testing.T) {
	bin := "pwsh"
	if runtime.GOOS == "windows" {
		bin = "powershell.exe"
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("%s is unavailable: %v", bin, err)
	}
	script, err := Generate("powershell")
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Dir(cwd)
	const proof = `
function _fest_native_executable { return 'Test-FestNative' }
function Test-FestNative {
    $global:observed = @($args)
    $global:LASTEXITCODE = $global:mockExitCode
    if ($global:LASTEXITCODE -eq 0) { Write-Output $global:mockDestination }
}
$global:mockDestination = $env:FEST_SHELL_TEST_DEST
$global:mockExitCode = 0
$start = (Get-Location).Path
$priorEncoding = [Console]::OutputEncoding.WebName
fgo
if ((Get-Location).Path -ne $env:FEST_SHELL_TEST_DEST) { throw 'fgo did not navigate to CLI path' }
if (($global:observed -join '|') -ne 'go|--print') { throw "wrong no-arg CLI call: $($global:observed -join '|')" }
if ([Console]::OutputEncoding.WebName -ne $priorEncoding) { throw 'fgo changed session output encoding' }
Set-Location -LiteralPath $start
fest go
if ((Get-Location).Path -ne $env:FEST_SHELL_TEST_DEST) { throw 'fest go without target did not navigate' }
if (($global:observed -join '|') -ne 'go|--print') { throw "wrong no-arg fest go call: $($global:observed -join '|')" }
Set-Location -LiteralPath $start
fgo active 'my fest'
if ((Get-Location).Path -ne $env:FEST_SHELL_TEST_DEST) { throw 'fgo status target did not navigate' }
if (($global:observed -join '|') -ne 'go|active/my fest|--print') { throw "wrong status CLI call: $($global:observed -join '|')" }
$global:mockDestination = $start
fest go project
if ((Get-Location).Path -ne $start) { throw 'fest go wrapper did not navigate' }
if (($global:observed -join '|') -ne 'go|project|--print') { throw "wrong fest go CLI call: $($global:observed -join '|')" }
$global:mockDestination = $env:FEST_SHELL_TEST_DEST
fgo fest
if ((Get-Location).Path -ne $env:FEST_SHELL_TEST_DEST) { throw 'fgo fest did not navigate' }
if (($global:observed -join '|') -ne 'go|fest|--print') { throw "wrong fest CLI call: $($global:observed -join '|')" }
$null = fls active
if (($global:observed -join '|') -ne 'list|active') { throw "wrong fls CLI call: $($global:observed -join '|')" }
$previous = (Get-Location).Path
$global:mockExitCode = 7
try { fgo project; throw 'expected fgo project failure' } catch {
    if ($_.Exception.Message -like 'expected fgo project failure') { throw }
}
if ((Get-Location).Path -ne $previous) { throw 'failed CLI changed location' }
if (($global:observed -join '|') -ne 'go|project|--print') { throw "wrong project CLI call: $($global:observed -join '|')" }
$global:mockExitCode = 0
$global:mockDestination = Join-Path $previous 'definitely-missing-fest-destination'
try { fgo fest; throw 'expected invalid destination failure' } catch {
    if ($_.Exception.Message -like 'expected invalid destination failure') { throw }
}
if ((Get-Location).Path -ne $previous) { throw 'invalid destination changed location' }
Write-Output 'PowerShell navigation PASS'
`
	cmd := exec.Command(bin, "-NoProfile", "-NonInteractive", "-Command", script+proof)
	cmd.Env = append(os.Environ(), "FEST_SHELL_TEST_DEST="+destination)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PowerShell navigation PASS") {
		t.Fatalf("PowerShell navigation proof failed: %v\n%s", err, output)
	}
}
