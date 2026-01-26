package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ri5hii/peony/internal/core"
	"github.com/ri5hii/peony/internal/storage"
)

const Version = "v0.2"

// PrintHelp prints the CLI usage text.
func PrintHelp() {
	fmt.Print(
		`Peony: a calm holding space for unfinished thoughts

Usage:
  peony <command> [args]

Commands:
  help, h                  Show this help
  version, -v              Show version
  add, a                   Capture a thought
  view, v                  View a thought by id
  tend, t                  list thoughts which are ready to be tended

Examples:
  peony add "I want to build a log cabin"
  peony view 12
`)
}

// openStore opens the database and returns a store and a close function.
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

// cmdAdd captures a new thought.
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

// cmdView prints a thought and its events.
func cmdView(args []string) int {
    // List mode: peony view
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
            const max = 60
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

    // Single-item mode: peony view <id>
    id, err := strconv.ParseInt(args[0], 10, 64)
    if err != nil || id <= 0 {
        fmt.Fprintln(os.Stderr, "view: invalid id")
        return 2
    }

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

    fmt.Printf("#%d (%s)\n", thought.ID, thought.CurrentState)
    fmt.Println(thought.Content)
    fmt.Printf("Created: %s\n", thought.CreatedAt.UTC().Format(time.RFC3339Nano))
    fmt.Printf("Updated: %s\n", thought.UpdatedAt.UTC().Format(time.RFC3339Nano))
    fmt.Printf("Tend counter: %d\n", thought.TendCounter)
    fmt.Printf("Eligible at: %s\n", thought.EligibilityAt.UTC().Format(time.RFC3339Nano))

    if thought.LastTendedAt != nil {
        fmt.Printf("Last tended: %s\n", thought.LastTendedAt.UTC().Format(time.RFC3339Nano))
    }
    if thought.Valence != nil {
        fmt.Printf("Valence: %d\n", *thought.Valence)
    }
    if thought.Energy != nil {
        fmt.Printf("Energy: %d\n", *thought.Energy)
    }

    if len(events) > 0 {
        fmt.Println("Events:")
        for _, ev := range events {
            trans := ""
            if ev.PreviousState != nil || ev.NextState != nil {
                ps := ""
                ns := ""
                if ev.PreviousState != nil {
                    ps = string(*ev.PreviousState)
                }
                if ev.NextState != nil {
                    ns = string(*ev.NextState)
                }
                trans = fmt.Sprintf("  (%s -> %s)", ps, ns)
            }

            if ev.Note != nil && strings.TrimSpace(*ev.Note) != "" {
                fmt.Printf("- %s  %s%s  %s\n", ev.At.UTC().Format(time.RFC3339Nano), ev.Kind, trans, *ev.Note)
            } else {
                fmt.Printf("- %s  %s%s\n", ev.At.UTC().Format(time.RFC3339Nano), ev.Kind, trans)
            }
        }
    }

    return 0
}

// cmdTend is the entry point for tend flows.
func cmdTend() int {

	return 0
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		PrintHelp()
		return
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "help", "h":
		PrintHelp()
		return

	case "version", "-v":
		fmt.Println("Peony " + Version)
		return

	case "add", "a":
		os.Exit(cmdAdd(rest))

	case "view", "v":
		os.Exit(cmdView(rest))

	case "tend", "t":
		os.Exit(cmdTend())

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		PrintHelp()
		os.Exit(2)
	}
}
