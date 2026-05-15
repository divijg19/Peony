package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptHelpMentionsSingleBinaryAndAlias(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	script := filepath.Join("..", "..", "install.sh")
	output, err := exec.Command("bash", script, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("install help failed: %v\n%s", err, output)
	}

	text := string(output)
	for _, want := range []string{"peony", "bloom", "--version", "--bin-dir", "--alias", "--shell"} {
		if !strings.Contains(text, want) {
			t.Fatalf("install help missing %q:\n%s", want, text)
		}
	}

	data, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("read install script: %v", err)
	}
	source := string(data)
	if !strings.Contains(source, `install_binary "${extract_dir}/peony" "peony"`) {
		t.Fatal("install script does not install peony")
	}
	if strings.Contains(source, `install_binary "${extract_dir}/bloom" "bloom"`) {
		t.Fatal("install script should not install a standalone bloom binary")
	}
	if strings.Contains(source, `release archive did not contain a bloom binary`) {
		t.Fatal("install script should not require a bloom binary in release archives")
	}
	if !strings.Contains(source, "# >>> peony bloom alias >>>") {
		t.Fatal("install script does not include optional bloom alias block")
	}
}

func TestInstallScriptAliasBlockIsIdempotent(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	script := filepath.Join("..", "..", "install.sh")
	rcFile := filepath.Join(t.TempDir(), "shellrc")
	cmd := exec.Command("bash", "-c", `source "$1"; append_bloom_alias_block "$2"; append_bloom_alias_block "$2"`, "bash", script, rcFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("append alias block failed: %v\n%s", err, output)
	}

	data, err := os.ReadFile(rcFile)
	if err != nil {
		t.Fatalf("read rc file: %v", err)
	}
	if count := strings.Count(string(data), "# >>> peony bloom alias >>>"); count != 1 {
		t.Fatalf("alias block count = %d, want 1\n%s", count, data)
	}
}
