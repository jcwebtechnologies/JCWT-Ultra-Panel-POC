package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// SyncCrontab writes cron entries for a system user
func SyncCrontab(username string, entries []CronEntry) error {
	var lines []string
	lines = append(lines, "# JCWT Ultra Panel - managed crontab")
	lines = append(lines, "# DO NOT EDIT MANUALLY")
	lines = append(lines, "")

	for _, e := range entries {
		if !e.Enabled {
			lines = append(lines, "# "+e.Schedule+" "+e.Command)
		} else {
			lines = append(lines, e.Schedule+" "+e.Command)
		}
	}
	lines = append(lines, "")

	crontab := strings.Join(lines, "\n")

	cmd := exec.Command("crontab", "-u", username, "-")
	cmd.Stdin = strings.NewReader(crontab)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write crontab: %s: %s", err, string(output))
	}

	return nil
}

// CronEntry represents a single cron job entry
type CronEntry struct {
	Schedule string
	Command  string
	Enabled  bool
}
