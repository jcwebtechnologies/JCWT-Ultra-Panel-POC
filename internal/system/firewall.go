package system

import (
	"fmt"
	"os/exec"
)

// SetupIPv6Firewall configures basic IPv6 firewall rules using ufw
func SetupIPv6Firewall(panelPort string) error {
	commands := [][]string{
		{"ufw", "--force", "reset"},
		{"ufw", "default", "deny", "incoming"},
		{"ufw", "default", "allow", "outgoing"},
		{"ufw", "allow", "22/tcp"},         // SSH
		{"ufw", "allow", "80/tcp"},         // HTTP
		{"ufw", "allow", "443/tcp"},        // HTTPS
		{"ufw", "allow", panelPort + "/tcp"}, // Panel
		{"ufw", "--force", "enable"},
	}

	for _, args := range commands {
		fullArgs := append([]string{"sudo"}, args...)
		cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("firewall command %v failed: %s: %s", args, err, string(output))
		}
	}

	return nil
}

// EnableIPv6InUFW ensures UFW is configured for IPv6
func EnableIPv6InUFW() error {
	// ufw on Ubuntu 24.04 has IPv6 enabled by default
	// Ensure it's set in the config
	cmd := exec.Command("sudo", "sed", "-i", "s/IPV6=no/IPV6=yes/", "/etc/default/ufw")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable IPv6 in ufw: %s", string(output))
	}
	return nil
}
