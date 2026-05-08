package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptHelpMentionsBothBinaries(t *testing.T) {
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
	if !strings.Contains(source, `install_binary "${extract_dir}/bloom" "bloom"`) {
		t.Fatal("install script does not install bloom")
	}
	if !strings.Contains(source, "# >>> peony bloom alias >>>") {
		t.Fatal("install script does not include optional bloom alias block")
	}
}
