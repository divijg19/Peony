package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// OpenEditorWithTemplate opens a temp file in the user's editor, then parses and returns edited content and an optional note.
func OpenEditorWithTemplate(initialContent string, initialNote string) (content *string, note *string, err error) {
	// ContentHeader marks the start of the editable thought content section in the editor template.
	ContentHeader := "--- content ---"
	// NoteHeader marks the start of the optional note section in the editor template.
	NoteHeader := "--- note ---"

	file, err := os.CreateTemp("", "peonyTend.txt")
	if err != nil {
		return nil, nil, err
	}
	path := file.Name()

	defer func() {
		os.Remove(path)
	}()

	templateContent := "// Peony tend — edit freely.\n// Thought is under the content header; note is optional.\n// If you remove the note header, everything will be treated as the thought.\n"

	_, err = file.WriteString(templateContent + "\n\n" + ContentHeader + "\n" + initialContent + "\n" + NoteHeader + "\n" + initialNote)
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}

	err = file.Sync()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}

	err = file.Close()
	if err != nil {
		return nil, nil, err
	}

	editors := []string{"nano", "vim", "vi"}
	var editorPath string
	for _, e := range editors {
		if e == "" {
			continue
		}
		editorBin, err := exec.LookPath(e)
		if err == nil {
			editorPath = editorBin
			break
		}
	}
	if editorPath == "" {
		return nil, nil, fmt.Errorf("no editor found in nano/vim/vi is available")
	}

	cmd := exec.Command(editorPath, path)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	text := string(data)

	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, ln := range rawLines {
		lines = append(lines, strings.TrimRight(ln, "\r"))
	}

	stripTemplateLine := func(line string) (string, bool) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			return "", true
		}
		return line, false
	}

	effectiveLines := make([]string, 0, len(lines))
	for _, ln := range lines {
		if _, skip := stripTemplateLine(ln); skip {
			continue
		}
		effectiveLines = append(effectiveLines, ln)
	}

	contentIndex := -1
	noteIndex := -1
	for idx, line := range effectiveLines {
		if line == ContentHeader && contentIndex == -1 {
			contentIndex = idx
		}
		if line == NoteHeader && noteIndex == -1 {
			noteIndex = idx
		}
	}

	var contentText string
	var noteText string

	switch {
	case contentIndex != -1 && noteIndex != -1 && contentIndex < noteIndex:
		contentText = strings.Join(effectiveLines[contentIndex+1:noteIndex], "\n")
		if noteIndex+1 < len(effectiveLines) {
			noteText = strings.Join(effectiveLines[noteIndex+1:], "\n")
		} else {
			noteText = ""
		}

	case contentIndex != -1 && (noteIndex == -1 || noteIndex < contentIndex):
		contentText = strings.Join(effectiveLines[contentIndex+1:], "\n")
		noteText = ""

	case noteIndex != -1 && contentIndex == -1:
		contentText = strings.Join(effectiveLines[:noteIndex], "\n")
		if noteIndex+1 < len(effectiveLines) {
			noteText = strings.Join(effectiveLines[noteIndex+1:], "\n")
		} else {
			noteText = ""
		}

	default:
		contentText = strings.Join(effectiveLines, "\n")
		noteText = ""
	}

	contentText = strings.TrimSpace(contentText)
	if contentText == "" {
		return nil, nil, fmt.Errorf("edited content is empty")
	}

	c := contentText
	content = &c

	noteText = strings.TrimSpace(noteText)
	if noteText == "" {
		note = nil
	} else {
		n := noteText
		note = &n
	}

	return content, note, nil
}
