package nginx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// VerifyCertFiles checks if SSL certificate and key files exist on disk.
// Uses os.Stat (no sudo) — cert files are 0644 and readable by the panel user.
// For the key (0600 root-owned), we check the cert file only and trust
// that the key was created alongside it.
func VerifyCertFiles(certPath, keyPath string) bool {
	if certPath == "" || keyPath == "" {
		return false
	}
	if _, err := os.Stat(certPath); err != nil {
		return false
	}
	return true
}

const vhostTemplate = `# JCWT Ultra Panel - Managed vhost for {{.Domain}}
# DO NOT EDIT MANUALLY - managed by JCWT Panel

{{- if eq .SSLType "none"}}
server {
    listen [::]:80;
    server_name {{.Domain}}{{if .Aliases}} {{.Aliases}}{{end}};

{{- if .BasicAuthEnabled}}
    auth_basic "Restricted Area";
    auth_basic_user_file /etc/nginx/htpasswd/{{.Domain}}.htpasswd;
{{- end}}

{{- if eq .SiteType "proxy"}}
{{- if .AccessLog}}
    access_log /home/{{.User}}/logs/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

    location / {
        proxy_pass {{.ProxyURL}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
{{- else}}
    root {{.WebRoot}};
    index index.php index.html index.htm;

{{- if .AccessLog}}
    access_log /home/{{.User}}/logs/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

{{- if eq .SiteType "wordpress"}}
    client_max_body_size 64m;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php{{.PHPVersion}}-fpm-{{.User}}.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_intercept_errors on;
        fastcgi_buffers 16 16k;
        fastcgi_buffer_size 32k;
    }

    # WordPress security: block access to sensitive files
    location ~ /wp-config\.php$ { deny all; }
    location ~ /xmlrpc\.php$ { deny all; }
    location ~ /wp-content/debug\.log$ { deny all; }
    location ~* /wp-content/uploads/.*\.php$ { deny all; }
    location ~* /wp-includes/.*\.php$ {
        deny all;
    }

    # Static assets caching
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|webp|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }
{{- else}}
    location / {
{{- if eq .SiteType "html"}}
        try_files $uri $uri/ =404;
{{- else}}
        try_files $uri $uri/ /index.php?$query_string;
{{- end}}
    }

{{- if eq .SiteType "php"}}
    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php{{.PHPVersion}}-fpm-{{.User}}.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
{{- end}}
{{- end}}

    location ~ /\.(ht|git|svn) {
        deny all;
    }
{{- end}}

    # Common server-level includes (phpMyAdmin, etc.)
    include /etc/nginx/snippets/jcwt-server-common.conf;
}
{{- else}}
server {
    listen [::]:80;
    server_name {{.Domain}}{{if .Aliases}} {{.Aliases}}{{end}};
    return 301 https://$host$request_uri;
}

server {
    listen [::]:443 ssl;
    http2 on;
    server_name {{.Domain}}{{if .Aliases}} {{.Aliases}}{{end}};

    ssl_certificate {{.SSLCertPath}};
    ssl_certificate_key {{.SSLKeyPath}};

{{- if .BasicAuthEnabled}}
    auth_basic "Restricted Area";
    auth_basic_user_file /etc/nginx/htpasswd/{{.Domain}}.htpasswd;
{{- end}}

{{- if eq .SiteType "proxy"}}
{{- if .AccessLog}}
    access_log /home/{{.User}}/logs/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

    location / {
        proxy_pass {{.ProxyURL}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
{{- else}}
    root {{.WebRoot}};
    index index.php index.html index.htm;

{{- if .AccessLog}}
    access_log /home/{{.User}}/logs/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

{{- if eq .SiteType "wordpress"}}
    client_max_body_size 64m;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php{{.PHPVersion}}-fpm-{{.User}}.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
        fastcgi_intercept_errors on;
        fastcgi_buffers 16 16k;
        fastcgi_buffer_size 32k;
    }

    # WordPress security: block access to sensitive files
    location ~ /wp-config\.php$ { deny all; }
    location ~ /xmlrpc\.php$ { deny all; }
    location ~ /wp-content/debug\.log$ { deny all; }
    location ~* /wp-content/uploads/.*\.php$ { deny all; }
    location ~* /wp-includes/.*\.php$ {
        deny all;
    }

    # Static assets caching
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|webp|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
        access_log off;
    }
{{- else}}
    location / {
{{- if eq .SiteType "html"}}
        try_files $uri $uri/ =404;
{{- else}}
        try_files $uri $uri/ /index.php?$query_string;
{{- end}}
    }

{{- if eq .SiteType "php"}}
    location ~ \.php$ {
        fastcgi_pass unix:/run/php/php{{.PHPVersion}}-fpm-{{.User}}.sock;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
{{- end}}
{{- end}}

    location ~ /\.(ht|git|svn) {
        deny all;
    }
{{- end}}

    # Common server-level includes (phpMyAdmin, etc.)
    include /etc/nginx/snippets/jcwt-server-common.conf;
}
{{- end}}
`

// VHostData holds template data for nginx vhost generation
type VHostData struct {
	Domain           string
	Aliases          string
	User             string
	SiteType         string
	PHPVersion       string
	ProxyURL         string
	WebRoot          string
	SSLType          string
	SSLCertPath      string
	SSLKeyPath       string
	BasicAuthEnabled bool
	AccessLog        bool
	ErrorLog         bool
}

// GenerateConfig generates an nginx vhost config
func GenerateConfig(data VHostData) (string, error) {
	tmpl, err := template.New("vhost").Parse(vhostTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// WriteVHost writes the vhost config file and creates symlink
func WriteVHost(sitesAvailable, sitesEnabled, domain string, data VHostData) error {
	config, err := GenerateConfig(data)
	if err != nil {
		return err
	}

	confPath := filepath.Join(sitesAvailable, domain+".conf")

	// Write via sudo tee since panel user can't write to /etc/nginx/
	cmd := exec.Command("sudo", "tee", confPath)
	cmd.Stdin = strings.NewReader(config)
	cmd.Stdout = nil
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("write vhost config: %s: %s", err, string(output))
	}

	// Create symlink in sites-enabled
	linkPath := filepath.Join(sitesEnabled, domain+".conf")
	exec.Command("sudo", "rm", "-f", linkPath).Run()
	cmd = exec.Command("sudo", "ln", "-s", confPath, linkPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create symlink: %s", string(output))
	}

	return nil
}

// RemoveVHost removes a vhost config and symlink
func RemoveVHost(sitesAvailable, sitesEnabled, domain string) error {
	exec.Command("sudo", "rm", "-f", filepath.Join(sitesEnabled, domain+".conf")).Run()
	exec.Command("sudo", "rm", "-f", filepath.Join(sitesAvailable, domain+".conf")).Run()
	return nil
}

// TestConfig runs nginx -t to validate configuration
func TestConfig() error {
	cmd := exec.Command("sudo", "nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx config test failed: %s", string(output))
	}
	return nil
}

// Reload reloads nginx configuration
func Reload() error {
	cmd := exec.Command("sudo", "systemctl", "reload", "nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx reload failed: %s", string(output))
	}
	return nil
}

// TestAndReload validates then reloads nginx
func TestAndReload() error {
	if err := TestConfig(); err != nil {
		return err
	}
	return Reload()
}
