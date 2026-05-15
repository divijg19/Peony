package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoStandaloneBloomCommandPackage(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "bloom")
	_, err := os.Stat(path)
	if err == nil {
		t.Fatal("cmd/bloom should not exist for single-binary distribution")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("stat cmd/bloom: %v", err)
	}
}

func TestReleaseWorkflowBuildsSinglePeonyBinaryAssets(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "release.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	source := string(data)
	for _, want := range []string{
		"RELEASE_TAG:",
		"inputs.tag",
		"peony_${RELEASE_TAG}_${goos}_${goarch}",
		"go vet ./...",
		"go test ./...",
		"go build \\",
		"-trimpath",
		"./cmd/peony",
		"tar --sort=name",
		"checksums.txt",
		"manifest.txt",
		"fail_on_unmatched_files: true",
		"softprops/action-gh-release@v3",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
	if strings.Contains(source, "cmd/bloom") || strings.Contains(source, "/bloom") {
		t.Fatal("release workflow should not build or package a standalone bloom binary")
	}
}

func TestCIWorkflowChecksFormattingTestsVetAndBuild(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	source := string(data)
	for _, want := range []string{
		"actions/checkout@v6",
		"actions/setup-go@v6",
		"gofmt -l .",
		"go vet ./...",
		"go test ./...",
		"go build -trimpath -o /tmp/peony ./cmd/peony",
		"bash install.sh --help",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("ci workflow missing %q", want)
		}
	}
}
