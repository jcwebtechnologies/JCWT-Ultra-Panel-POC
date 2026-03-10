package system

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var safeDomainRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// WriteLogrotateConfig creates a logrotate config for a site's access and error logs.
// Logs rotate daily, kept for 7 days, compressed.
func WriteLogrotateConfig(webRootBase, systemUser, domain string) error {
	if !safeDomainRegex.MatchString(domain) || !safeDomainRegex.MatchString(systemUser) {
		return fmt.Errorf("invalid domain or user for logrotate config")
	}

	logsDir := fmt.Sprintf("%s/%s/logs", webRootBase, systemUser)
	confName := fmt.Sprintf("jcwt-%s", domain)
	confPath := fmt.Sprintf("/etc/logrotate.d/%s", confName)

	config := fmt.Sprintf(`%s/%s-access.log
%s/%s-error.log {
    su %s %s
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 %s %s
    sharedscripts
    postrotate
        [ -f /run/nginx.pid ] && kill -USR1 $(cat /run/nginx.pid) 2>/dev/null || true
    endscript
}
`, logsDir, domain, logsDir, domain, systemUser, systemUser, systemUser, systemUser)

	cmd := exec.Command("sudo", "tee", confPath)
	cmd.Stdin = strings.NewReader(config)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write logrotate config: %s", string(output))
	}

	// Set proper ownership and permissions
	exec.Command("sudo", "chmod", "644", confPath).Run()

	return nil
}

// RemoveLogrotateConfig removes the logrotate config for a site.
func RemoveLogrotateConfig(domain string) error {
	if !safeDomainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain for logrotate removal")
	}
	confPath := fmt.Sprintf("/etc/logrotate.d/jcwt-%s", domain)
	cmd := exec.Command("sudo", "rm", "-f", confPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove logrotate config: %s", string(output))
	}
	return nil
}
