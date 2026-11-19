package git

// readOnlyArgs stops a read command such as git status from taking the
// optional index lock, which a canceled command would leave behind.
func readOnlyArgs(args []string) []string {
	return append([]string{"--no-optional-locks"}, args...)
}
