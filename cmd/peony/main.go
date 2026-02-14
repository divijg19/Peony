package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/divijg19/peony/internal/core"
	"github.com/divijg19/peony/internal/storage"
)

// Version is the current CLI version string.
const Version = "v0.4"

// PrintHelp prints the CLI usage and examples.
func PrintHelp() {
	fmt.Print(`Peony: a calm holding space for unfinished thoughts

Usage:
  peony <command> [args]

Commands:
  help, h        Show this help or detailed help for a command
  version, -v    Show version
  add, a         Capture a thought
  view, v        View the list of thoughts or a thought by id
  tend, t        List thoughts which are ready to be tended
  release, r     Clears a thought from peony
  evolve, e      Passes a thought into peony wider integration
  config, c      View and edit defaults for peony

Syntax:
  peony add [content]
  peony view [id]
  peony view [filter]
  peony tend [id]
  peony config [setting]

Examples:
  peony help view
  peony add "I want to build a log cabin"
  peony view 12
  peony view --archived

For detailed help on a command:
  peony help <command>
`)
}

// openStore opens the SQLite-backed store and returns a close function.
func openStore() (*storage.Store, func(), error) {
	var err error

	var dbPath string
	dbPath, err = storage.ResolveDBPath()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve db path: %w", err)
	}

	sqlDB, err := storage.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	var st *storage.Store
	st, err = storage.New(sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("new store: %w", err)
	}

	closeFn := func() {
		_ = sqlDB.Close()
	}
	return st, closeFn, nil
}

// cmdAdd captures a thought and appends the initial captured event.
func cmdAdd(args []string) int {
	content := strings.TrimSpace(strings.Join(args, " "))
	if content == "" {
		fmt.Print("What would you like to hold? ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "add: read: %v\n", err)
			return 1
		}
		content = strings.TrimSpace(line)
	}

	if content == "" {
		fmt.Fprintln(os.Stderr, "add: content is empty")
		return 1
	}

	st, closeDB, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "add: %v\n", err)
		return 1
	}
	defer closeDB()

	var id int64
	id, err = st.CreateThought(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "add: %v\n", err)
		return 1
	}

	next := core.StateCaptured
	err = st.AppendEvent(id, "captured", nil, &next, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "add: append event: %v\n", err)
		return 1
	}

	fmt.Printf("Saved as #%d\n", id)
	return 0
}

// cmdView shows a paginated list of thoughts or a single thought with its event history.
func cmdView(args []string) int {

	if len(args) == 0 {
		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "view: %v\n", err)
			return 1
		}
		defer closeDB()

		reader := bufio.NewReader(os.Stdin)
		pageSize := 10
		page := 0

		overview := func(s string) string {
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.TrimSpace(s)
			const max = 80
			if len(s) <= max {
				return s
			}
			return s[:max-1] + "…"
		}

		for {
			offset := page * pageSize
			thoughts, err := st.ListThoughtsByPagination(pageSize, offset)
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: %v\n", err)
				return 1
			}

			if len(thoughts) == 0 {
				if page == 0 {
					fmt.Println("No thoughts yet.")
					return 0
				}
				page--
				continue
			}

			fmt.Printf("Page %d\n", page+1)
			fmt.Printf("%-6s %-10s %-5s %-20s %s\n", "ID", "STATE", "TEND", "UPDATED", "OVERVIEW")
			for _, th := range thoughts {
				fmt.Printf("%-6d %-10s %-5d %-20s %s\n",
					th.ID,
					th.CurrentState,
					th.TendCounter,
					th.UpdatedAt.UTC().Format("2006-01-02 15:04"),
					overview(th.Content),
				)
			}

			fmt.Print("[n]ext, [p]rev, [q]uit: ")
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: read: %v\n", err)
				return 1
			}

			switch strings.ToLower(strings.TrimSpace(line)) {
			case "q":
				return 0
			case "p":
				if page > 0 {
					page--
				}
			default:
				if len(thoughts) == pageSize {
					page++
				}
			}
		}
	}
	if len(args) == 1 {
		if id, err := strconv.ParseInt(args[0], 10, 64); err == nil && id > 0 {
			st, closeDB, err := openStore()
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: %v\n", err)
				return 1
			}
			defer closeDB()

			thought, events, err := st.GetThought(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: %v\n", err)
				return 1
			}

			fmt.Printf("#%d  %s  (tends: %d)\n", thought.ID, thought.CurrentState, thought.TendCounter)

			now := time.Now().UTC()

			formatShortUTC := func(t time.Time) string {
				return t.UTC().Format("2006-01-02 15:04Z")
			}

			formatRelative := func(t time.Time, now time.Time) string {
				d := t.Sub(now)
				if d < 0 {
					d = -d
					switch {
					case d < time.Minute:
						return "just now"
					case d < time.Hour:
						return fmt.Sprintf("%dm ago", int(d.Minutes()))
					case d < 24*time.Hour:
						return fmt.Sprintf("%dh ago", int(d.Hours()))
					default:
						return fmt.Sprintf("%dd ago", int(d.Hours()/24))
					}
				}

				switch {
				case d < time.Minute:
					return "in <1m"
				case d < time.Hour:
					return fmt.Sprintf("in %dm", int(d.Minutes()))
				case d < 24*time.Hour:
					return fmt.Sprintf("in %dh", int(d.Hours()))
				default:
					return fmt.Sprintf("in %dd", int(d.Hours()/24))
				}
			}

			switch thought.CurrentState {
			case core.StateCaptured, core.StateResting:
				eligible := core.EligibleToSurface(thought, now)
				if eligible {
					fmt.Println("Eligible: yes")
				} else {
					fmt.Printf("Eligible: %s (at %s)\n", formatRelative(thought.EligibilityAt, now), formatShortUTC(thought.EligibilityAt))
				}
			case core.StateTended:
				fmt.Println("Needs resolution: rest/evolve/release/archive")
			case core.StateEvolved, core.StateReleased, core.StateArchived:
				fmt.Printf("Terminal: %s\n", thought.CurrentState)
			default:
				fmt.Printf("State: %s\n", thought.CurrentState)
			}

			fmt.Println()
			fmt.Println("CONTENT")
			fmt.Println(thought.Content)

			fmt.Println()
			fmt.Println("META")
			fmt.Printf("Created:  %s (%s)\n", formatShortUTC(thought.CreatedAt), formatRelative(thought.CreatedAt, now))
			fmt.Printf("Updated:  %s (%s)\n", formatShortUTC(thought.UpdatedAt), formatRelative(thought.UpdatedAt, now))
			fmt.Printf("Eligible: %s (%s)\n", formatShortUTC(thought.EligibilityAt), formatRelative(thought.EligibilityAt, now))

			if thought.LastTendedAt != nil {
				fmt.Printf("Last tended: %s (%s)\n", formatShortUTC(*thought.LastTendedAt), formatRelative(*thought.LastTendedAt, now))
			}
			if thought.Valence != nil {
				fmt.Printf("Valence: %d\n", *thought.Valence)
			}
			if thought.Energy != nil {
				fmt.Printf("Energy: %d\n", *thought.Energy)
			}

			if len(events) > 0 {
				fmt.Println()
				fmt.Println("EVENTS")
				for _, ev := range events {
					at := formatShortUTC(ev.At)

					transition := ""
					if ev.PreviousState != nil || ev.NextState != nil {
						prevState := ""
						nextState := ""
						if ev.PreviousState != nil {
							prevState = string(*ev.PreviousState)
						}
						if ev.NextState != nil {
							nextState = string(*ev.NextState)
						}

						if prevState == "" && nextState != "" {
							transition = " " + nextState
						} else if prevState != "" && nextState == "" {
							transition = " " + prevState
						} else if prevState != "" || nextState != "" {
							transition = fmt.Sprintf(" %s → %s", prevState, nextState)
						}
					}

					fmt.Printf("- %s  %s%s\n", at, ev.Kind, transition)
					if ev.Note != nil && strings.TrimSpace(*ev.Note) != "" {
						fmt.Printf("  note: %s\n", strings.TrimSpace(*ev.Note))
					}
				}
			}
			return 0
		}
		filter := strings.TrimSpace(strings.Join(args, " "))
		if after, ok := strings.CutPrefix(filter, "--"); ok {
			filter = after
		}

		switch filter {
		case "captured", "resting", "tended", "evolved", "released", "archived":

		default:
			fmt.Fprintln(os.Stderr, "view: invalid filter")
			return 2
		}
		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "view: %v\n", err)
			return 1
		}
		defer closeDB()

		reader := bufio.NewReader(os.Stdin)
		pageSize := 10
		page := 0

		overview := func(s string) string {
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.TrimSpace(s)
			const max = 80
			if len(s) <= max {
				return s
			}
			return s[:max-1] + "…"
		}

		for {
			offset := page * pageSize
			thoughts, err := st.FilterViewByPagination(pageSize, offset, filter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: %v\n", err)
				return 1
			}

			if len(thoughts) == 0 {
				if page == 0 {
					fmt.Println("No thoughts yet.")
					return 0
				}
				page--
				continue
			}

			fmt.Printf("Page %d\n", page+1)
			fmt.Printf("%-6s %-10s %-5s %-20s %s\n", "ID", "STATE", "TEND", "UPDATED", "OVERVIEW")
			for _, th := range thoughts {
				fmt.Printf("%-6d %-10s %-5d %-20s %s\n",
					th.ID,
					th.CurrentState,
					th.TendCounter,
					th.UpdatedAt.UTC().Format("2006-01-02 15:04"),
					overview(th.Content),
				)
			}

			fmt.Print("[n]ext, [p]rev, [q]uit: ")
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "view: read: %v\n", err)
				return 1
			}

			switch strings.ToLower(strings.TrimSpace(line)) {
			case "q":
				return 0
			case "p":
				if page > 0 {
					page--
				}
			default:
				if len(thoughts) == pageSize {
					page++
				}
			}
		}
	}
	return 0
}

// cmdTend lists eligible thoughts or runs the interactive tend flow for a specific thought ID.
func cmdTend(args []string) int {
	if len(args) == 0 {
		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}
		defer closeDB()

		reader := bufio.NewReader(os.Stdin)
		pageSize := 10
		page := 0

		overview := func(s string) string {
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.TrimSpace(s)
			const max = 60
			if len(s) <= max {
				return s
			}
			return s[:max-1] + "…"
		}

		for {
			offset := page * pageSize
			thoughts, err := st.ListTendThoughtsByPagination(pageSize, offset)
			if err != nil {
				fmt.Fprintf(os.Stderr, "tend: %v\n", err)
				return 1
			}

			if len(thoughts) == 0 {
				if page == 0 {
					fmt.Println("No thoughts yet.")
					return 0
				}
				page--
				continue
			}

			fmt.Printf("Page %d\n", page+1)
			fmt.Printf("%-6s %-10s %-5s %-20s %s\n", "ID", "STATE", "TEND", "UPDATED", "OVERVIEW")
			for _, th := range thoughts {
				fmt.Printf("%-6d %-10s %-5d %-20s %s\n",
					th.ID,
					th.CurrentState,
					th.TendCounter,
					th.UpdatedAt.UTC().Format("2006-01-02 15:04"),
					overview(th.Content),
				)
			}

			fmt.Print("[n]ext, [p]rev, [q]uit: ")
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "tend: read: %v\n", err)
				return 1
			}

			switch strings.ToLower(strings.TrimSpace(line)) {
			case "q":
				return 0
			case "p":
				if page > 0 {
					page--
				}
			default:
				if len(thoughts) == pageSize {
					page++
				}
			}
		}
	}

	if len(args) == 1 {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			fmt.Fprintln(os.Stderr, "tend: invalid id")
			return 2
		}

		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}
		defer closeDB()

		thought, _, err := st.GetTendThought(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}

		reader := bufio.NewReader(os.Stdin)

		editedContent, editedNote, err := OpenEditorWithTemplate(thought.Content, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: edit: %v\n", err)
			return 1
		}

		ok, err := promptYesNo(reader, "Are you satisfied with the changes?")
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}
		if !ok {
			return 0
		}

		mark, err := promptYesNo(reader, "Do you want to mark this thought as tended? (Your note will be saved only if you say yes.)")
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}

		if editedContent == nil {
			return 1
		}

		if err := st.UpdateThoughtContent(id, *editedContent); err != nil {
			fmt.Fprintf(os.Stderr, "tend: save: %v\n", err)
			return 1
		}

		if !mark {
			return 0
		}

		if err := st.MarkThoughtTended(id, editedNote); err != nil {
			fmt.Fprintf(os.Stderr, "tend: mark tended: %v\n", err)
			return 1
		}

		choice, err := promptChoice(reader, "What would you like to do next?", []string{"rest", "evolve", "release", "archive"})
		if err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}

		var next core.State
		switch choice {
		case "rest":
			next = core.StateResting
		case "evolve":
			next = core.StateEvolved
		case "release":
			next = core.StateReleased
		case "archive":
			next = core.StateArchived
		default:
			fmt.Fprintf(os.Stderr, "tend: unknown choice %q\n", choice)
			return 2
		}

		if err := st.TransitionPostTendResolutionStrict(id, next, nil); err != nil {
			fmt.Fprintf(os.Stderr, "tend: %v\n", err)
			return 1
		}

		return 0
	}
	return 0
}

// promptYesNo asks a yes/no question on stdin and returns the user's choice.
func promptYesNo(reader *bufio.Reader, question string) (bool, error) {
	for {
		fmt.Printf("%s [y/n]: ", question)
		line, err := reader.ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("read: %w", err)
		}
		s := strings.ToLower(strings.TrimSpace(line))
		switch s {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(os.Stderr, "Please answer yes or no.")
		}
	}
}

// promptChoice asks the user to select one of the provided choices and returns the selected value.
func promptChoice(reader *bufio.Reader, question string, choices []string) (string, error) {
	if len(choices) == 0 {
		return "", errors.New("no choices provided")
	}
	allowed := make(map[string]struct{}, len(choices))
	for _, c := range choices {
		allowed[strings.ToLower(strings.TrimSpace(c))] = struct{}{}
	}

	for {
		fmt.Printf("%s (%s): ", question, strings.Join(choices, "/"))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read: %w", err)
		}
		s := strings.ToLower(strings.TrimSpace(line))
		if _, ok := allowed[s]; ok {
			return s, nil
		}
		fmt.Fprintf(os.Stderr, "Please choose one of: %s\n", strings.Join(choices, ", "))
	}
}

// cmdRelease permanently removes a thought (and its event history).
func cmdRelease(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "release: usage: `peony release <id>`")
		return 2
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		fmt.Fprintln(os.Stderr, "release: invalid id")
		return 2
	}

	st, closeDB, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		return 1
	}
	defer closeDB()

	reader := bufio.NewReader(os.Stdin)
	ok, err := promptYesNo(reader, fmt.Sprintf("Release thought #%d? This will delete it.", id))
	if err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		return 1
	}
	if !ok {
		return 0
	}

	if err := st.ReleaseThought(id); err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		return 1
	}

	if err := st.ReindexThoughtIDs(); err != nil {
		fmt.Fprintf(os.Stderr, "release: reindex ids: %v\n", err)
		return 1
	}

	fmt.Printf("Released #%d.\n", id)
	return 0
}

// cmdEvolve displays evolved thoughts or marks a thought as evolved.
func cmdEvolve(args []string) int {
	if len(args) == 0 {
		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "evolve: %v\n", err)
			return 1
		}
		defer closeDB()

		reader := bufio.NewReader(os.Stdin)
		pageSize := 10
		page := 0

		overview := func(s string) string {
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.TrimSpace(s)
			const max = 60
			if len(s) <= max {
				return s
			}
			return s[:max-1] + "…"
		}

		for {
			offset := page * pageSize
			filter := "evolved"
			thoughts, err := st.FilterViewByPagination(pageSize, offset, filter)
			if err != nil {
				fmt.Fprintf(os.Stderr, "evolve: %v\n", err)
				return 1
			}

			if len(thoughts) == 0 {
				if page == 0 {
					fmt.Println("No thoughts yet.")
					return 0
				}
				page--
				continue
			}

			fmt.Printf("Page %d\n", page+1)
			fmt.Printf("%-6s %-10s %-5s %-20s %s\n", "ID", "STATE", "TEND", "UPDATED", "OVERVIEW")
			for _, th := range thoughts {
				fmt.Printf("%-6d %-10s %-5d %-20s %s\n",
					th.ID,
					th.CurrentState,
					th.TendCounter,
					th.UpdatedAt.UTC().Format("2006-01-02 15:04"),
					overview(th.Content),
				)
			}

			fmt.Print("[n]ext, [p]rev, [q]uit: ")
			line, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintf(os.Stderr, "evolve: read: %v\n", err)
				return 1
			}

			switch strings.ToLower(strings.TrimSpace(line)) {
			case "q":
				return 0
			case "p":
				if page > 0 {
					page--
				}
			default:
				if len(thoughts) == pageSize {
					page++
				}
			}
		}
	}
	if len(args) == 1 {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			fmt.Fprintln(os.Stderr, "evolve: invalid id")
			return 2
		}

		st, closeDB, err := openStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "evolve: %v\n", err)
			return 1
		}
		defer closeDB()

		if err := st.ToEvolve(id); err != nil {
			fmt.Fprintf(os.Stderr, "evolve: %v\n", err)
			return 1
		}

		fmt.Printf("Evolved #%d.\n", id)
	}
	return 0
}

func cmdHelp(args []string) int {
	if len(args) == 0 {
		PrintHelp()
		return 0
	}

	filter := strings.TrimPrefix(args[0], "--")

	switch filter {
	case "add", "--add":
		fmt.Print(`peony add — capture a thought

Description:
  Captures a new thought and stores it in the captured state.
  The thought will rest for a configured duration before becoming eligible to tend.

Syntax:
  peony add [content]
  peony a [content]

Examples:
  peony add "I wonder if I should learn Rust"
  peony add
  (prompts interactively if no content provided)

`)

	case "view", "--view":
		fmt.Print(`peony view — view thoughts

Description:
  View a paginated list of thoughts, a single thought by ID, or filter by state.
  Without arguments, shows all non-archived thoughts.

Syntax:
  peony view [id]
  peony view [--filter | filter]
  peony v [id]

Filters:
  captured, resting, tended, evolved, released, archived

Examples:
  peony view
  peony view 12
  peony view --archived
  peony view captured

`)

	case "tend", "--tend":
		fmt.Print(`peony tend — work with thoughts ready for tending

Description:
  Lists thoughts that are eligible to tend, or opens an interactive editor
  to tend a specific thought by ID.

Syntax:
  peony tend [id]
  peony t [id]

Examples:
  peony tend
  peony tend 5

`)

	case "release", "--release":
		fmt.Print(`peony release — remove a thought permanently

Description:
  Permanently deletes a thought and its event history from Peony.
  This action cannot be undone.

Syntax:
  peony release <id>
  peony r <id>

Examples:
  peony release 8
  peony r 3

`)

	case "evolve", "--evolve":
		fmt.Print(`peony evolve — mark a thought as evolved

Description:
  Transitions a thought into the evolved state, indicating it has been
  integrated into your wider workflow (e.g., a task manager or notes app).

Syntax:
  peony evolve [id]
  peony e [id]

Examples:
  peony evolve 7
  peony e
  (lists evolved thoughts if no ID provided)

`)

	case "config", "--config":
		fmt.Print(`peony config — view and configure defaults

Description:
  View or update configuration settings like editor and settle duration.

Syntax:
  peony config
  peony c
  peony config [--editor | editor]
  peony config [--settleDuration | settleDuration] 

Examples:
  peony config
  peony config --editor
  peony config settleDuration 24h
  peony c settleDuration

`)

	default:
		fmt.Fprintf(os.Stderr, "No help available for: %s\n", args[0])
		PrintHelp()
		return 2
	}
	return 0
}

// Main dispatches CLI commands to their corresponding handlers.
func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		PrintHelp()
		return
	}

	_, _ = loadRuntimeConfig()

	cmd := args[0]
	rest := args[1:]

	// Print only when the eligible count changes.
	shouldPrintNotice := cmd != "add" && cmd != "a" && cmd != "tend" && cmd != "t" && cmd != "help" && cmd != "h" && cmd != "version" && cmd != "-v"
	st, closeDB, err := openStore()
	if err == nil {
		defer closeDB()
		n, err := st.CountTendReady()
		if err == nil {
			if shouldPrintNotice && n > 0 {
				changed := st.DidCountTendChange(n)
				if changed {
					fmt.Fprintf(os.Stderr, "🌱 %d thoughts feel ready for tending. Run: peony tend\n", n)
				}
			}
		}
	}

	switch cmd {
	case "help", "h":
		os.Exit(cmdHelp(rest))
		return

	case "version", "-v":
		fmt.Println("Peony " + Version)
		return

	case "add", "a":
		os.Exit(cmdAdd(rest))

	case "view", "v":
		os.Exit(cmdView(rest))

	case "tend", "t":
		os.Exit(cmdTend(rest))

	case "release", "r":
		os.Exit(cmdRelease(rest))

	case "evolve", "e":
		os.Exit(cmdEvolve(rest))

	case "configure", "config", "c":
		os.Exit(cmdConfigure(rest))

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		PrintHelp()
		os.Exit(2)
	}
}
