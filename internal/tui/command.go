package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/divijg19/peony/internal/app"
	"github.com/divijg19/peony/internal/config"
	"github.com/divijg19/peony/internal/core"
)

const peonyVersion = "v0.4"

type commandSpec struct {
	Name    string
	Aliases []string
	Usage   string
	Help    string
}

var commandSpecs = []commandSpec{
	{Name: "help", Aliases: []string{"h"}, Usage: "help [command]", Help: "Show Bloom command help."},
	{Name: "version", Aliases: []string{"-v"}, Usage: "version", Help: "Show the Peony version."},
	{Name: "add", Aliases: []string{"a"}, Usage: "add [content]", Help: "Capture a thought, or open capture when content is omitted."},
	{Name: "view", Aliases: []string{"v"}, Usage: "view [id|state]", Help: "Read visible thoughts, a thought by id, or a state filter."},
	{Name: "tend", Aliases: []string{"t"}, Usage: "tend [id]", Help: "List ready thoughts or open a thought for tending."},
	{Name: "release", Aliases: []string{"r"}, Usage: "release <id>", Help: "Ask before permanently releasing a thought."},
	{Name: "evolve", Aliases: []string{"e"}, Usage: "evolve [id]", Help: "List evolved thoughts or mark one evolved."},
	{Name: "config", Aliases: []string{"configure", "c"}, Usage: "config [settleDuration <duration>|editor]", Help: "View or update configuration."},
	{Name: "tui", Usage: "tui", Help: "Report that Bloom is already open."},
}

func (m *Model) runCommand(line string) {
	args, err := parseCommandLine(line)
	if err != nil {
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.status = "Command could not be parsed."
		m.setOutput("Command error", []string{err.Error()}, OutputError, line, true)
		return
	}
	if len(args) == 0 {
		m.mode = ModeBrowse
		m.focus = FocusQueue
		m.status = "Command is empty."
		return
	}

	cmd := args[0]
	rest := args[1:]
	m.mode = ModeBrowse
	m.focus = FocusQueue
	m.clearOutput()
	m.status = ""

	switch canonicalCommand(cmd) {
	case "help", "h":
		m.setOutput("Help", commandHelp(rest), OutputHelp, "help", true)
		m.status = "Help opened."
	case "version", "-v":
		m.setOutput("Version", []string{"Peony " + peonyVersion}, OutputCommand, "version", false)
		m.status = "Version shown."
	case "add", "a":
		m.commandAdd(rest)
	case "view", "v":
		m.commandView(rest)
	case "tend", "t":
		m.commandTend(rest)
	case "release", "r":
		m.commandRelease(rest)
	case "evolve", "e":
		m.commandEvolve(rest)
	case "config", "configure", "c":
		m.commandConfig(rest)
	case "tui":
		m.setOutput("TUI", []string{"Bloom is already open."}, OutputCommand, "tui", false)
		m.status = "Already in Bloom."
	default:
		lines := []string{fmt.Sprintf("Unknown command: %s", cmd)}
		if suggestion := commandSuggestion(cmd); suggestion != "" {
			lines = append(lines, "Did you mean: "+suggestion+"?")
		}
		lines = append(lines, "Try : help")
		m.setOutput("Command error", lines, OutputError, line, true)
		m.status = "Command not recognized."
	}
}

func (m *Model) commandAdd(args []string) {
	content := strings.TrimSpace(strings.Join(args, " "))
	if content == "" {
		m.mode = ModeCapture
		m.focus = FocusPrompt
		m.addBox.Reset()
		m.addBox.Focus()
		m.status = ""
		m.setOutput("Capture", []string{"Capture opened."}, OutputCommand, "add", false)
		return
	}
	id, err := m.service.Capture(content)
	if err != nil {
		m.status = err.Error()
		m.setOutput("Command error", []string{err.Error()}, OutputError, "add", true)
		return
	}
	m.reloadPreserving(id)
	m.status = fmt.Sprintf("Saved as #%d.", id)
	m.setOutput("Capture", []string{fmt.Sprintf("Saved as #%d", id)}, OutputCommand, "add", false)
}

func (m *Model) commandView(args []string) {
	if len(args) == 0 {
		snapshot, err := m.service.SnapshotBloom(m.filter.appFilter(), m.query)
		if err != nil {
			m.commandError(err)
			return
		}
		lines := thoughtTable("Visible thoughts", bloomThoughts(snapshot.Thoughts), 10)
		m.setOutput("View", lines, OutputCommand, "view", len(lines) > 3)
		m.status = "View shown."
		return
	}
	if len(args) != 1 {
		m.setOutput("Command error", []string{"view: usage: view [id|state]"}, OutputError, "view", true)
		m.status = "Command needs one view target."
		return
	}
	arg := strings.TrimPrefix(args[0], "--")
	if id, err := strconv.ParseInt(arg, 10, 64); err == nil && id > 0 {
		item, err := m.service.Thought(id)
		if err != nil {
			m.commandError(err)
			return
		}
		lines := thoughtDetail(item)
		m.setOutput(fmt.Sprintf("Thought #%d", id), lines, OutputCommand, "view", true)
		m.status = fmt.Sprintf("Viewing #%d.", id)
		return
	}
	state, ok := parseState(arg)
	if !ok {
		m.setOutput("Command error", []string{"view: invalid filter"}, OutputError, "view", true)
		m.status = "Command filter was not recognized."
		return
	}
	snapshot, err := m.service.Snapshot(state, "")
	if err != nil {
		m.commandError(err)
		return
	}
	lines := thoughtTable("View "+string(state), gardenThoughts(snapshot.Thoughts), 10)
	m.setOutput("View "+string(state), lines, OutputCommand, "view "+string(state), len(lines) > 3)
	m.status = "View shown."
}

func (m *Model) commandTend(args []string) {
	if len(args) == 0 {
		thoughts, err := m.service.TendReady(10)
		if err != nil {
			m.commandError(err)
			return
		}
		lines := thoughtTable("Ready to tend", thoughts, 10)
		m.setOutput("Tend", lines, OutputCommand, "tend", len(lines) > 3)
		m.status = "Tend list shown."
		return
	}
	if len(args) != 1 {
		m.setOutput("Command error", []string{"tend: usage: tend [id]"}, OutputError, "tend", true)
		m.status = "Command needs one thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.setOutput("Command error", []string{"tend: invalid id"}, OutputError, "tend", true)
		m.status = "Command id was not valid."
		return
	}
	m.startTendByID(id)
}

func (m *Model) commandRelease(args []string) {
	if len(args) != 1 {
		m.setOutput("Command error", []string{"release: usage: release <id>"}, OutputError, "release", true)
		m.status = "Command needs a thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.setOutput("Command error", []string{"release: invalid id"}, OutputError, "release", true)
		m.status = "Command id was not valid."
		return
	}
	if _, err := m.service.Thought(id); err != nil {
		m.commandError(err)
		return
	}
	m.pendingReleaseID = id
	m.mode = ModeReleaseConfirm
	m.focus = FocusPrompt
	m.setOutput("Release", []string{fmt.Sprintf("Confirm release of #%d.", id)}, OutputWarning, "release", false)
	m.status = ""
}

func (m *Model) commandEvolve(args []string) {
	if len(args) == 0 {
		snapshot, err := m.service.Snapshot(core.StateEvolved, "")
		if err != nil {
			m.commandError(err)
			return
		}
		lines := thoughtTable("Evolved thoughts", gardenThoughts(snapshot.Thoughts), 10)
		m.setOutput("Evolved", lines, OutputCommand, "evolve", len(lines) > 3)
		m.status = "Evolved list shown."
		return
	}
	if len(args) != 1 {
		m.setOutput("Command error", []string{"evolve: usage: evolve [id]"}, OutputError, "evolve", true)
		m.status = "Command needs one thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.setOutput("Command error", []string{"evolve: invalid id"}, OutputError, "evolve", true)
		m.status = "Command id was not valid."
		return
	}
	if err := m.service.Evolve(id); err != nil {
		m.commandError(err)
		return
	}
	m.reloadPreserving(id)
	m.status = fmt.Sprintf("Evolved #%d.", id)
	m.setOutput("Evolve", []string{fmt.Sprintf("Evolved #%d.", id)}, OutputCommand, "evolve", false)
}

func (m *Model) commandConfig(args []string) {
	cfg, err := config.Load()
	if err != nil {
		m.commandError(fmt.Errorf("config: %w", err))
		return
	}
	if len(args) == 0 {
		lines := configLines(cfg)
		m.setOutput("Config", lines, OutputCommand, "config", len(lines) > 3)
		m.status = "Config shown."
		return
	}
	if len(args) >= 1 && (args[0] == "--editor" || args[0] == "editor") {
		m.setOutput("Config", []string{"Editor selection is interactive. Use peony config --editor outside Bloom for now."}, OutputWarning, "config editor", true)
		m.status = "Config editor selection needs the CLI."
		return
	}
	if len(args) >= 1 && (args[0] == "--settleDuration" || args[0] == "settleDuration") {
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			m.setOutput("Command error", []string{"config settleDuration: provide a duration, for example 24h"}, OutputError, "config", true)
			m.status = "Config duration missing."
			return
		}
		dur, err := time.ParseDuration(args[1])
		if err != nil {
			m.setOutput("Command error", []string{"config: invalid settle duration"}, OutputError, "config", true)
			m.status = "Config duration was invalid."
			return
		}
		cfg.SettleDuration = dur.String()
		core.SettleDuration = dur
		if err := config.Save(cfg); err != nil {
			m.commandError(fmt.Errorf("config: %w", err))
			return
		}
		lines := configLines(cfg)
		m.setOutput("Config", lines, OutputCommand, "config", len(lines) > 3)
		m.status = "Config saved."
		return
	}
	m.setOutput("Command error", []string{fmt.Sprintf("config: unknown argument %s", strings.Join(args, " "))}, OutputError, "config", true)
	m.status = "Config command was not recognized."
}

func (m *Model) commandError(err error) {
	m.status = err.Error()
	m.setOutput("Command error", []string{err.Error()}, OutputError, "command", true)
}

func (m *Model) startTendByID(id int64) {
	item, err := m.service.Thought(id)
	if err != nil {
		m.commandError(err)
		return
	}
	if !item.Ready {
		m.setOutput("Tend", []string{fmt.Sprintf("#%d is still settling.", id)}, OutputWarning, "tend", true)
		m.status = "This thought is still settling."
		return
	}
	m.mode = ModeTend
	m.focus = FocusPrompt
	m.tendID = id
	m.tendFocus = 0
	m.tendContent.SetValue(item.Thought.Content)
	m.tendNote.Reset()
	m.focusTendInput()
	m.setOutput("Tend", []string{fmt.Sprintf("Tending #%d.", id)}, OutputCommand, "tend", false)
	m.status = ""
}

func commandHelp(args []string) []string {
	if len(args) == 0 {
		lines := []string{"Peony commands"}
		for _, spec := range commandSpecs {
			lines = append(lines, fmt.Sprintf("%-28s %s", spec.Usage, spec.Help))
		}
		return lines
	}
	name := canonicalCommand(strings.TrimPrefix(args[0], "--"))
	for _, spec := range commandSpecs {
		if spec.Name == name {
			return []string{
				"peony " + spec.Name,
				spec.Help,
				"Usage: " + spec.Usage,
			}
		}
	}
	return []string{fmt.Sprintf("No help available for: %s", args[0])}
}

func canonicalCommand(value string) string {
	for _, spec := range commandSpecs {
		if value == spec.Name {
			return spec.Name
		}
		for _, alias := range spec.Aliases {
			if value == alias {
				return spec.Name
			}
		}
	}
	return value
}

func commandSuggestion(value string) string {
	for _, spec := range commandSpecs {
		if strings.HasPrefix(spec.Name, value) || strings.HasPrefix(value, spec.Name) {
			return spec.Name
		}
		for _, alias := range spec.Aliases {
			if strings.HasPrefix(alias, value) || strings.HasPrefix(value, alias) {
				return spec.Name
			}
		}
	}
	return ""
}

func parseCommandLine(line string) ([]string, error) {
	args := []string{}
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range strings.TrimSpace(line) {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in command")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func parseState(value string) (core.State, bool) {
	switch value {
	case "captured", "resting", "tended", "evolved", "released", "archived":
		return core.State(value), true
	default:
		return "", false
	}
}

func thoughtTable(title string, thoughts []core.Thought, limit int) []string {
	if len(thoughts) == 0 {
		return []string{title, "No thoughts yet."}
	}
	if limit <= 0 || limit > len(thoughts) {
		limit = len(thoughts)
	}
	lines := []string{title, "ID  STATE      TEND  UPDATED           OVERVIEW"}
	for _, th := range thoughts[:limit] {
		lines = append(lines, fmt.Sprintf("#%-3d %-10s %-5d %-16s %s",
			th.ID,
			th.CurrentState,
			th.TendCounter,
			th.UpdatedAt.UTC().Format("2006-01-02 15:04"),
			oneLine(th.Content, 60),
		))
	}
	if len(thoughts) > limit {
		lines = append(lines, fmt.Sprintf("%d more", len(thoughts)-limit))
	}
	return lines
}

func thoughtDetail(item app.BloomThought) []string {
	t := item.Thought
	lines := []string{
		fmt.Sprintf("#%d  %s  (tends: %d)", t.ID, t.CurrentState, t.TendCounter),
		"",
		"CONTENT",
		t.Content,
		"",
		"META",
		"Created:  " + t.CreatedAt.UTC().Format("2006-01-02 15:04Z"),
		"Updated:  " + t.UpdatedAt.UTC().Format("2006-01-02 15:04Z"),
		"Eligible: " + t.EligibilityAt.UTC().Format("2006-01-02 15:04Z"),
	}
	if len(item.Events) > 0 {
		lines = append(lines, "", "EVENTS")
		for _, event := range item.Events {
			lines = append(lines, fmt.Sprintf("- %s  %s", event.At.UTC().Format("2006-01-02"), event.Kind))
		}
	}
	return lines
}

func bloomThoughts(items []app.BloomThought) []core.Thought {
	thoughts := make([]core.Thought, 0, len(items))
	for _, item := range items {
		thoughts = append(thoughts, item.Thought)
	}
	return thoughts
}

func gardenThoughts(items []app.GardenThought) []core.Thought {
	thoughts := make([]core.Thought, 0, len(items))
	for _, item := range items {
		thoughts = append(thoughts, item.Thought)
	}
	return thoughts
}

func configLines(cfg config.Config) []string {
	path, err := config.ConfigPath()
	lines := []string{}
	if err == nil {
		lines = append(lines, "Config file: "+path, "")
	}
	lines = append(lines, "Current configuration")
	if strings.TrimSpace(cfg.Editor) == "" {
		lines = append(lines, "Editor: (unset)")
	} else {
		lines = append(lines, "Editor: "+cfg.Editor)
	}
	lines = append(lines, "SettleDuration: "+config.SettleDuration(cfg).String())
	return lines
}
