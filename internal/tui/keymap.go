package tui

type keyHint struct {
	Key   string
	Label string
}

var browseKeyHints = []keyHint{
	{Key: "j/k", Label: "move"},
	{Key: "enter", Label: "inspect"},
	{Key: "/", Label: "search"},
	{Key: ":", Label: "command"},
	{Key: "a", Label: "capture"},
	{Key: "t", Label: "tend"},
	{Key: "r/e/A", Label: "resolve"},
	{Key: "x", Label: "release"},
	{Key: "h/l", Label: "scope"},
	{Key: "?", Label: "help"},
	{Key: "q", Label: "quit"},
}

var searchKeyHints = []keyHint{
	{Key: "enter", Label: "apply"},
	{Key: "ctrl+u", Label: "clear"},
	{Key: "esc", Label: "cancel"},
}

var commandKeyHints = []keyHint{
	{Key: "enter", Label: "run"},
	{Key: "ctrl+u", Label: "clear"},
	{Key: "esc", Label: "cancel"},
}

var filterKeyHints = []keyHint{
	{Key: "h/l", Label: "choose"},
	{Key: "enter", Label: "apply"},
	{Key: "esc", Label: "cancel"},
}

var captureKeyHints = []keyHint{
	{Key: "ctrl+s", Label: "save"},
	{Key: "esc", Label: "cancel"},
}

var tendKeyHints = []keyHint{
	{Key: "tab", Label: "field"},
	{Key: "ctrl+s", Label: "mark tended"},
	{Key: "esc", Label: "cancel"},
}

var releaseKeyHints = []keyHint{
	{Key: "y", Label: "confirm"},
	{Key: "n", Label: "cancel"},
	{Key: "esc", Label: "cancel"},
}

var helpKeyHints = []keyHint{
	{Key: "esc", Label: "close"},
	{Key: "?", Label: "close"},
	{Key: "q", Label: "close"},
}

func keyHelpLines(mode Mode) []string {
	return []string{
		activeLabelStyle.Render("Bloom keys"),
		"",
		labelStyle.Render("Browse"),
		"j/k or arrows move through the queue",
		"enter or Tab focuses detail",
		"h/l switches Ready, Resting, and All",
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
		": command prompt",
		"f choose what is shown",
		"R reload",
		"",
		labelStyle.Render("Leave"),
		"Esc closes prompts and sheets",
		"q quits from browse",
	}
}
