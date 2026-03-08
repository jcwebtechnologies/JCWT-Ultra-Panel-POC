package php

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const poolTemplate = `; JCWT Ultra Panel - PHP-FPM pool for {{.User}}
; DO NOT EDIT MANUALLY - managed by JCWT Panel

[{{.User}}]
user = {{.User}}
group = {{.User}}

listen = /run/php/php{{.PHPVersion}}-fpm-{{.User}}.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = ondemand
pm.max_children = 5
pm.process_idle_timeout = 10s
pm.max_requests = 200

; Security
chdir = /
security.limit_extensions = .php

; PHP Settings
php_admin_value[memory_limit] = {{.MemoryLimit}}
php_admin_value[max_execution_time] = {{.MaxExecutionTime}}
php_admin_value[max_input_time] = {{.MaxInputTime}}
php_admin_value[max_input_vars] = {{.MaxInputVars}}
php_admin_value[post_max_size] = {{.PostMaxSize}}
php_admin_value[upload_max_filesize] = {{.UploadMaxFilesize}}
php_admin_value[open_basedir] = {{.WebRoot}}:/tmp
php_admin_value[disable_functions] = exec,passthru,shell_exec,system,proc_open,popen
php_admin_value[error_log] = /var/log/php/{{.User}}-error.log
{{.CustomDirectives}}
`

// PoolData holds template data for PHP-FPM pool generation
type PoolData struct {
	User              string
	PHPVersion        string
	WebRoot           string
	MemoryLimit       string
	MaxExecutionTime  int
	MaxInputTime      int
	MaxInputVars      int
	PostMaxSize       string
	UploadMaxFilesize string
	CustomDirectives  string
}

// GeneratePoolConfig generates a PHP-FPM pool configuration
func GeneratePoolConfig(data PoolData) (string, error) {
	// Process custom directives
	if data.CustomDirectives != "" {
		lines := strings.Split(data.CustomDirectives, ";")
		var processed []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				processed = append(processed, fmt.Sprintf("php_admin_value[%s] = %s",
					strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])))
			}
		}
		data.CustomDirectives = strings.Join(processed, "\n")
	}

	tmpl, err := template.New("pool").Parse(poolTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// WritePool writes a PHP-FPM pool config file
func WritePool(phpBaseDir, phpVersion, user string, data PoolData) error {
	config, err := GeneratePoolConfig(data)
	if err != nil {
		return err
	}

	poolDir := filepath.Join(phpBaseDir, phpVersion, "fpm", "pool.d")
	confPath := filepath.Join(poolDir, user+".conf")

	// Write via sudo tee since panel user can't write to /etc/php/
	cmd := exec.Command("tee", confPath)
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = nil // suppress tee's stdout echo
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write pool config: %s: %s", err, string(output))
	}

	return nil
}

// RemovePool removes a PHP-FPM pool config file
func RemovePool(phpBaseDir, phpVersion, user string) error {
	poolDir := filepath.Join(phpBaseDir, phpVersion, "fpm", "pool.d")
	confPath := filepath.Join(poolDir, user+".conf")
	cmd := exec.Command("rm", "-f", confPath)
	cmd.Run()
	return nil
}

// ReloadFPM gracefully reloads the PHP-FPM service for a specific version
func ReloadFPM(version string) error {
	service := fmt.Sprintf("php%s-fpm", version)
	cmd := exec.Command("systemctl", "reload", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload %s failed: %s", service, string(output))
	}
	return nil
}

// RestartFPM restarts the PHP-FPM service for a specific version
func RestartFPM(version string) error {
	service := fmt.Sprintf("php%s-fpm", version)
	cmd := exec.Command("systemctl", "restart", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restart %s failed: %s", service, string(output))
	}
	return nil
}

// AvailableVersions returns installed PHP versions
func AvailableVersions() []string {
	versions := []string{"8.2", "8.3", "8.4", "8.5"}
	var available []string
	for _, v := range versions {
		binary := fmt.Sprintf("/usr/bin/php%s", v)
		if _, err := os.Stat(binary); err == nil {
			available = append(available, v)
		}
	}
	if len(available) == 0 {
		return versions // Return all if none detected (dev mode)
	}
	return available
}
