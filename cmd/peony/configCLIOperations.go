package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/divijg19/peony/internal/config"
	"github.com/divijg19/peony/internal/core"
)

var (
	runtimeConfig     config.Config
	runtimeConfigErr  error
	runtimeConfigOnce sync.Once
)

// loadRuntimeConfig loads config once and applies runtime overrides.
func loadRuntimeConfig() (config.Config, error) {
	runtimeConfigOnce.Do(func() {
		runtimeConfig, runtimeConfigErr = config.Load()
		core.SettleDuration = config.SettleDuration(runtimeConfig)
	})
	return runtimeConfig, runtimeConfigErr
}

// printConfig renders the current configuration to stdout.
func printConfig(cfg config.Config) int {
	path, pathErr := config.ConfigPath()
	if pathErr == nil {
		fmt.Printf("Config file: %s\n\n", path)
	}

	fmt.Println("Current configuration")
	if cfg.Editor == "" {
		fmt.Println("Editor: (unset)")
	} else {
		fmt.Printf("Editor: %s\n", cfg.Editor)
	}
	fmt.Printf("SettleDuration: %s\n", config.SettleDuration(cfg))
	return 0
}

// configureEditor scans for editors and saves the selected one.
func configureEditor(cfg config.Config) (config.Config, int) {
	editors := availableEditors()
	if len(editors) == 0 {
		fmt.Fprintln(os.Stderr, "config: no editors found on PATH")
		return cfg, 1
	}

	fmt.Println("Available editors:")
	for idx, editor := range editors {
		fmt.Printf("[%d] %s\n", idx, editor)
	}

	fmt.Print("Select editor by index: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: read: %v\n", err)
		return cfg, 1
	}
	line = strings.TrimSpace(line)
	if line == "" {
		fmt.Fprintln(os.Stderr, "config: no selection provided")
		return cfg, 1
	}
	idx, err := strconv.Atoi(line)
	if err != nil || idx < 0 || idx >= len(editors) {
		fmt.Fprintln(os.Stderr, "config: invalid editor index")
		return cfg, 2
	}

	cfg.Editor = editors[idx]
	return cfg, 0
}

// configureSettleDuration prompts for and sets the settle duration.
func configureSettleDuration(cfg config.Config, durationValue string) (config.Config, int) {
	if strings.TrimSpace(durationValue) == "" {
		fmt.Print("Settle duration (e.g. 18h, 2h30m): ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "config: read: %v\n", err)
			return cfg, 1
		}
		durationValue = strings.TrimSpace(line)
	}

	dur, err := time.ParseDuration(strings.TrimSpace(durationValue))
	if err != nil {
		fmt.Fprintln(os.Stderr, "config: invalid settle duration")
		return cfg, 2
	}

	cfg.SettleDuration = dur.String()
	core.SettleDuration = dur
	return cfg, 0
}

// cmdConfigure handles `peony config`.
func cmdConfigure(args []string) int {
	cfg, cfgErr := loadRuntimeConfig()
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", cfgErr)
	}

	if len(args) == 0 {
		return printConfig(cfg)
	}

	var (
		setEditor       bool
		setSettle       bool
		settleValue     string
		unrecognizedArg string
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--editor", "editor":
			setEditor = true
		case "--settleDuration", "settleDuration":
			setSettle = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				settleValue = args[i+1]
				i++
			}
		default:
			unrecognizedArg = arg
		}
	}

	if unrecognizedArg != "" {
		fmt.Fprintf(os.Stderr, "config: unknown argument %s\n", unrecognizedArg)
		return 2
	}

	if setEditor {
		var code int
		cfg, code = configureEditor(cfg)
		if code != 0 {
			return code
		}
	}

	if setSettle {
		var code int
		cfg, code = configureSettleDuration(cfg, settleValue)
		if code != 0 {
			return code
		}
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	return printConfig(cfg)
}
