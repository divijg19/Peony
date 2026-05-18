package cli

import (
	"crypto/sha256"
	"fmt"
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
	for _, want := range []string{"peony", "bloom", "checksum", "--version", "--bin-dir", "--alias", "--shell"} {
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
	for _, want := range []string{`checksums_url_for`, `curl -fsSL "${checksums_url}"`, `verify_checksum "${archive_path}" "${asset_name}" "${checksums_path}"`} {
		if !strings.Contains(source, want) {
			t.Fatalf("install script missing checksum behavior %q", want)
		}
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

func TestInstallScriptVerifyChecksum(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	script := filepath.Join("..", "..", "install.sh")
	tmpDir := t.TempDir()
	assetName := "peony_v1.2.3_linux_amd64.tar.gz"
	archivePath := filepath.Join(tmpDir, assetName)
	archiveBytes := []byte("peony archive bytes")
	if err := os.WriteFile(archivePath, archiveBytes, 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	sum := fmt.Sprintf("%x", sha256.Sum256(archiveBytes))
	if err := os.WriteFile(checksumPath, []byte(sum+"  "+assetName+"\n"), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}

	cmd := exec.Command("bash", "-c", `source "$1"; verify_checksum "$2" "$3" "$4"`, "bash", script, archivePath, assetName, checksumPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("verify checksum failed: %v\n%s", err, output)
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
