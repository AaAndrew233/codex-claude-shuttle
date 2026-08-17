package claudeprocess

func Running() (bool, error) {
	return platformRunning()
}
