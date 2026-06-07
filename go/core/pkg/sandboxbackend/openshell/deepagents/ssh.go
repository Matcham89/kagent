package deepagents

// DefaultSSHLaunchCommand is the interactive CLI started when connecting to a
// Deep Agents harness sandbox via the UI terminal (unless plain shell is requested).
func DefaultSSHLaunchCommand() string {
	// dcode reads config from $HOME/.deepagents; bootstrap writes to DeepAgentsConfigDir, so
	// point HOME at DeepAgentsHome for the interactive session.
	return "cd " + DeepAgentsHome + " && HOME=" + DeepAgentsHome + " exec dcode"
}
