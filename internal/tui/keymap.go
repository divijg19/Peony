package tui

const browseKeyHelp = "j/k move  enter inspect  a capture  t tend  r rest  e evolve  A archive  x release  / search  f filter  ? help  q quit"

func keyHelpLines(mode Mode) []string {
	return []string{
		activeLabelStyle.Render("Bloom keys"),
		"",
		labelStyle.Render("Browse"),
		"j/k or arrows move through the queue",
		"enter or Tab focuses detail",
		"h/l switches Ready, Resting, Memory, and All",
		"Ctrl+D and Ctrl+U scroll detail when detail is focused",
		"",
		labelStyle.Render("Work"),
		"a capture a thought",
		"t tend a ready thought",
		"r rest a tended thought",
		"e evolve, A remember, x release permanently",
		"",
		labelStyle.Render("Find"),
		"/ search",
		"f filter",
		"R reload",
		"",
		labelStyle.Render("Leave"),
		"Esc closes prompts and sheets",
		"q quits from browse",
	}
}
