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

func (m *Model) runCommand(line string) {
	args := strings.Fields(line)
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
	m.commandOutput = nil
	m.status = ""

	switch cmd {
	case "help", "h":
		m.commandOutput = commandHelp(rest)
		m.status = "Help opened."
	case "version", "-v":
		m.commandOutput = []string{"Peony " + peonyVersion}
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
		m.commandOutput = []string{"Bloom is already open."}
		m.status = "Already in Bloom."
	default:
		m.commandOutput = []string{fmt.Sprintf("Unknown command: %s", cmd), "Try : help"}
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
		m.commandOutput = []string{"Capture opened."}
		return
	}
	id, err := m.service.Capture(content)
	if err != nil {
		m.status = err.Error()
		m.commandOutput = []string{err.Error()}
		return
	}
	m.reloadPreserving(id)
	m.status = fmt.Sprintf("Saved as #%d.", id)
	m.commandOutput = []string{fmt.Sprintf("Saved as #%d", id)}
}

func (m *Model) commandView(args []string) {
	if len(args) == 0 {
		snapshot, err := m.service.SnapshotBloom(m.filter.appFilter(), m.query)
		if err != nil {
			m.commandError(err)
			return
		}
		m.commandOutput = thoughtTable("Visible thoughts", bloomThoughts(snapshot.Thoughts), 10)
		m.status = "View shown."
		return
	}
	if len(args) != 1 {
		m.commandOutput = []string{"view: usage: view [id|state]"}
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
		m.commandOutput = thoughtDetail(item)
		m.status = fmt.Sprintf("Viewing #%d.", id)
		return
	}
	state, ok := parseState(arg)
	if !ok {
		m.commandOutput = []string{"view: invalid filter"}
		m.status = "Command filter was not recognized."
		return
	}
	snapshot, err := m.service.Snapshot(state, "")
	if err != nil {
		m.commandError(err)
		return
	}
	m.commandOutput = thoughtTable("View "+string(state), gardenThoughts(snapshot.Thoughts), 10)
	m.status = "View shown."
}

func (m *Model) commandTend(args []string) {
	if len(args) == 0 {
		thoughts, err := m.service.TendReady(10)
		if err != nil {
			m.commandError(err)
			return
		}
		m.commandOutput = thoughtTable("Ready to tend", thoughts, 10)
		m.status = "Tend list shown."
		return
	}
	if len(args) != 1 {
		m.commandOutput = []string{"tend: usage: tend [id]"}
		m.status = "Command needs one thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.commandOutput = []string{"tend: invalid id"}
		m.status = "Command id was not valid."
		return
	}
	m.startTendByID(id)
}

func (m *Model) commandRelease(args []string) {
	if len(args) != 1 {
		m.commandOutput = []string{"release: usage: release <id>"}
		m.status = "Command needs a thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.commandOutput = []string{"release: invalid id"}
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
	m.commandOutput = []string{fmt.Sprintf("Confirm release of #%d.", id)}
	m.status = ""
}

func (m *Model) commandEvolve(args []string) {
	if len(args) == 0 {
		snapshot, err := m.service.Snapshot(core.StateEvolved, "")
		if err != nil {
			m.commandError(err)
			return
		}
		m.commandOutput = thoughtTable("Evolved thoughts", gardenThoughts(snapshot.Thoughts), 10)
		m.status = "Evolved list shown."
		return
	}
	if len(args) != 1 {
		m.commandOutput = []string{"evolve: usage: evolve [id]"}
		m.status = "Command needs one thought id."
		return
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		m.commandOutput = []string{"evolve: invalid id"}
		m.status = "Command id was not valid."
		return
	}
	if err := m.service.Evolve(id); err != nil {
		m.commandError(err)
		return
	}
	m.reloadPreserving(id)
	m.status = fmt.Sprintf("Evolved #%d.", id)
	m.commandOutput = []string{fmt.Sprintf("Evolved #%d.", id)}
}

func (m *Model) commandConfig(args []string) {
	cfg, err := config.Load()
	if err != nil {
		m.commandError(fmt.Errorf("config: %w", err))
		return
	}
	if len(args) == 0 {
		m.commandOutput = configLines(cfg)
		m.status = "Config shown."
		return
	}
	if len(args) >= 1 && (args[0] == "--editor" || args[0] == "editor") {
		m.commandOutput = []string{"Editor selection is interactive. Use peony config --editor outside Bloom for now."}
		m.status = "Config editor selection needs the CLI."
		return
	}
	if len(args) >= 1 && (args[0] == "--settleDuration" || args[0] == "settleDuration") {
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			m.commandOutput = []string{"config settleDuration: provide a duration, for example 24h"}
			m.status = "Config duration missing."
			return
		}
		dur, err := time.ParseDuration(args[1])
		if err != nil {
			m.commandOutput = []string{"config: invalid settle duration"}
			m.status = "Config duration was invalid."
			return
		}
		cfg.SettleDuration = dur.String()
		core.SettleDuration = dur
		if err := config.Save(cfg); err != nil {
			m.commandError(fmt.Errorf("config: %w", err))
			return
		}
		m.commandOutput = configLines(cfg)
		m.status = "Config saved."
		return
	}
	m.commandOutput = []string{fmt.Sprintf("config: unknown argument %s", strings.Join(args, " "))}
	m.status = "Config command was not recognized."
}

func (m *Model) commandError(err error) {
	m.status = err.Error()
	m.commandOutput = []string{err.Error()}
}

func (m *Model) startTendByID(id int64) {
	item, err := m.service.Thought(id)
	if err != nil {
		m.commandError(err)
		return
	}
	if !item.Ready {
		m.commandOutput = []string{fmt.Sprintf("#%d is still settling.", id)}
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
	m.commandOutput = []string{fmt.Sprintf("Tending #%d.", id)}
	m.status = ""
}

func commandHelp(args []string) []string {
	if len(args) == 0 {
		return []string{
			"Peony commands",
			"help [command]",
			"version",
			"add [content]",
			"view [id|state]",
			"tend [id]",
			"release <id>",
			"evolve [id]",
			"config [setting]",
			"tui",
		}
	}
	switch strings.TrimPrefix(args[0], "--") {
	case "add":
		return []string{"peony add", "Capture a thought.", "Usage: add [content]"}
	case "view":
		return []string{"peony view", "Read visible thoughts, a thought by id, or a state filter.", "Usage: view [id|captured|resting|tended|evolved|released|archived]"}
	case "tend":
		return []string{"peony tend", "List ready thoughts or open a thought for tending.", "Usage: tend [id]"}
	case "release":
		return []string{"peony release", "Ask before permanently releasing a thought.", "Usage: release <id>"}
	case "evolve":
		return []string{"peony evolve", "List evolved thoughts or mark one evolved.", "Usage: evolve [id]"}
	case "config":
		return []string{"peony config", "View or update configuration.", "Usage: config [settleDuration <duration>|editor]"}
	case "tui":
		return []string{"peony tui", "Bloom is already open."}
	default:
		return []string{fmt.Sprintf("No help available for: %s", args[0])}
	}
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
