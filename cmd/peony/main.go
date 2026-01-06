package main

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/ri5hii/peony/internal/storage"
)

const Version = "v0.1"

func PrintHelp() {
    fmt.Print(
        `Peony: a calm holding space for unfinished thoughts

Usage:
  peony <command> [args]

Commands:
  help, -h, --help         Show this help
  version, -v, --version   Show version
  add, -a, -add            Capture a thought
  view                     View a thought by id

Examples:
  peony add "i should email alex"
  peony view 12
`)
}

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

    // Optional but nice: keep an event trail consistent with your events table.
    err = st.AppendEvent(id, "captured", nil)
    if err != nil {
        fmt.Fprintf(os.Stderr, "add: append event: %v\n", err)
        return 1
    }

    fmt.Printf("Saved as #%d\n", id)
    return 0
}

func cmdView(args []string) int {
    if len(args) < 1 {
        fmt.Fprintln(os.Stderr, "view: missing id")
        return 2
    }

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

    fmt.Printf("#%d (%s)\n", thought.ID, thought.State)
    fmt.Println(thought.Content)
    fmt.Printf("Created: %s\n", thought.CreatedAt.UTC().Format(time.RFC3339Nano))
    fmt.Printf("Updated: %s\n", thought.UpdatedAt.UTC().Format(time.RFC3339Nano))

    if thought.RestUntil != nil {
        fmt.Printf("Rest until: %s\n", thought.RestUntil.UTC().Format(time.RFC3339Nano))
    }
    if thought.LastTendedAt != nil {
        fmt.Printf("Last tended: %s\n", thought.LastTendedAt.UTC().Format(time.RFC3339Nano))
    }

    if len(events) > 0 {
        fmt.Println("Events:")
        for _, ev := range events {
            if ev.Note != nil && strings.TrimSpace(*ev.Note) != "" {
                fmt.Printf("- %s  %s  %s\n", ev.At.UTC().Format(time.RFC3339Nano), ev.Kind, *ev.Note)
            } else {
                fmt.Printf("- %s  %s\n", ev.At.UTC().Format(time.RFC3339Nano), ev.Kind)
            }
        }
    }

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
    case "help", "-h", "--help":
        PrintHelp()
        return

    case "version", "-v", "--version":
        fmt.Println("Peony " + Version)
        return

    case "add", "-a", "-add":
        os.Exit(cmdAdd(rest))

    case "view":
        os.Exit(cmdView(rest))

    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
        PrintHelp()
        os.Exit(2)
    }
}