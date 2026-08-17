package codexprocess

// Running reports whether the Codex desktop application is active.
func Running() (bool, error) {
	return platformRunning()
}
