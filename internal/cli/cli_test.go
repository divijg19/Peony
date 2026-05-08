package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunPeonyTUILaunchesRunner(t *testing.T) {
	previous := TUIRunner
	defer func() {
		TUIRunner = previous
	}()

	called := 0
	TUIRunner = func() int {
		called++
		return 7
	}

	if code := RunPeony([]string{"tui"}); code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
	if called != 1 {
		t.Fatalf("runner calls = %d, want 1", called)
	}
}

func TestRunPeonyTUIRejectsArgsWithoutLaunchingRunner(t *testing.T) {
	previous := TUIRunner
	defer func() {
		TUIRunner = previous
	}()

	called := 0
	TUIRunner = func() int {
		called++
		return 0
	}

	code := RunPeony([]string{"tui", "--later"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if called != 0 {
		t.Fatalf("runner calls = %d, want 0", called)
	}
}

func TestRunPeonyHelpTUI(t *testing.T) {
	output := captureStdout(t, func() {
		if code := RunPeony([]string{"help", "tui"}); code != 0 {
			t.Fatalf("help tui exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(output, "peony tui") || !strings.Contains(output, "bloom") {
		t.Fatalf("unexpected tui help output: %q", output)
	}
}

func TestRunBloomLaunchesRunnerDirectly(t *testing.T) {
	previous := TUIRunner
	defer func() {
		TUIRunner = previous
	}()

	called := 0
	TUIRunner = func() int {
		called++
		return 0
	}

	if code := RunBloom(nil); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if called != 1 {
		t.Fatalf("runner calls = %d, want 1", called)
	}
}

func TestRunBloomHelpAndVersion(t *testing.T) {
	helpOutput := captureStdout(t, func() {
		if code := RunBloom([]string{"help"}); code != 0 {
			t.Fatalf("help exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(helpOutput, "Bloom") || !strings.Contains(helpOutput, "peony tui") {
		t.Fatalf("unexpected help output: %q", helpOutput)
	}

	versionOutput := captureStdout(t, func() {
		if code := RunBloom([]string{"--version"}); code != 0 {
			t.Fatalf("version exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(versionOutput, "Peony "+Version) {
		t.Fatalf("unexpected version output: %q", versionOutput)
	}
}

func TestRunBloomUnknownArgsReturnUsageError(t *testing.T) {
	code := RunBloom([]string{"add", "not-from-bloom"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writeEnd
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	if err := writeEnd.Close(); err != nil {
		t.Fatalf("close write end: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, readEnd); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
