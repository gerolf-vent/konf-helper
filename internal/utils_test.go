package internal

import (
	"os"
	"strings"
)

//
// Container utilities
//

func isRunningInContainer() bool {
	// Check if we're running in a container by looking for container-specific indicators
	containerIndicators := []string{
		"/.dockerenv",
		"/run/.containerenv",
	}
	for _, indicator := range containerIndicators {
		if _, err := os.Stat(indicator); err == nil {
			return true
		}
	}

	// Check for container-specific cgroup patterns
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if len(content) > 0 && (strings.Contains(content, "docker") ||
			strings.Contains(content, "containerd") ||
			strings.Contains(content, "podman")) {
			return true
		}
	}

	return false
}
