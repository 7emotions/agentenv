package shell

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGenerateInitScript_Unsupported(t *testing.T) {
	_, err := GenerateInitScript("fish")
	if err == nil {
		t.Error("expected error for unsupported shell 'fish'")
	}

	_, err = GenerateInitScript("powershell")
	if err == nil {
		t.Error("expected error for unsupported shell 'powershell'")
	}
}

func TestGenerateInitScript_Output(t *testing.T) {
	tests := []struct {
		shell string
	}{
		{"bash"},
		{"zsh"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			script, err := GenerateInitScript(tt.shell)
			if err != nil {
				t.Fatal(err)
			}
			if script == "" {
				t.Fatal("expected non-empty script")
			}

			// Check for essential content
			if !strings.Contains(script, "_agentenv_binary=") {
				t.Error("script missing binary path variable")
			}
			if !strings.Contains(script, "agentenv()") {
				t.Error("script missing agentenv function")
			}
			if !strings.Contains(script, "_internal_activate") {
				t.Error("script missing activate handler")
			}
			if !strings.Contains(script, "_internal_deactivate") {
				t.Error("script missing deactivate handler")
			}
			if !strings.Contains(script, "AGENTENV_ACTIVE") {
				t.Error("script missing AGENTENV_ACTIVE env var")
			}
			if !strings.Contains(script, "_agentenv_json_val") {
				t.Error("script missing JSON parser helper")
			}
		})
	}
}

func TestGenerateInitScript_BashSyntax(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	script, err := GenerateInitScript("bash")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "agentenv-bash-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(script); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Verify syntax with bash -n
	cmd := exec.Command("bash", "-n", tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n failed: %v\n%s", err, out)
	}

	// Verify set -euo pipefail compatibility (source only)
	cmd = exec.Command("bash", "-c", "set -euo pipefail && source "+tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("set -euo pipefail source failed: %v\n%s", err, out)
	}
}

func TestGenerateInitScript_ZshSyntax(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}

	script, err := GenerateInitScript("zsh")
	if err != nil {
		t.Fatal(err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "agentenv-zsh-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(script); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Verify syntax with zsh -n
	cmd := exec.Command("zsh", "-n", tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh -n failed: %v\n%s", err, out)
	}

	// Verify set -euo pipefail compatibility
	cmd = exec.Command("zsh", "-c", "set -euo pipefail && source "+tmpFile.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("set -euo pipefail source failed: %v\n%s", err, out)
	}
}
