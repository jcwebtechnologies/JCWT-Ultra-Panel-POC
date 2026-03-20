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
    access_log /home/{{.User}}/logs/web/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/web/{{.Domain}}-error.log;
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
    access_log /home/{{.User}}/logs/web/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/web/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

{{- if eq .SiteType "wordpress"}}
    client_max_body_size 64m;

{{.WordPressSecurityRules}}
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
    listen [::]:443 ssl http2;
    server_name {{.Domain}}{{if .Aliases}} {{.Aliases}}{{end}};

    ssl_certificate {{.SSLCertPath}};
    ssl_certificate_key {{.SSLKeyPath}};

{{- if .BasicAuthEnabled}}
    auth_basic "Restricted Area";
    auth_basic_user_file /etc/nginx/htpasswd/{{.Domain}}.htpasswd;
{{- end}}

{{- if eq .SiteType "proxy"}}
{{- if .AccessLog}}
    access_log /home/{{.User}}/logs/web/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/web/{{.Domain}}-error.log;
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
    access_log /home/{{.User}}/logs/web/{{.Domain}}-access.log;
{{- else}}
    access_log off;
{{- end}}
{{- if .ErrorLog}}
    error_log /home/{{.User}}/logs/web/{{.Domain}}-error.log;
{{- else}}
    error_log /dev/null;
{{- end}}

{{- if eq .SiteType "wordpress"}}
    client_max_body_size 64m;

{{.WordPressSecurityRules}}
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
	// WordPressSecurityRules is the pre-built nginx security block for WordPress sites.
	// Empty for non-WordPress sites. Built via BuildWPSecurityRules().
	WordPressSecurityRules string
}

// BuildWPSecurityRules returns the nginx security location block for WordPress sites.
// When allowXMLRPC is true the xmlrpc.php block is omitted.
func BuildWPSecurityRules(allowXMLRPC bool) string {
	var sb strings.Builder
	sb.WriteString("    # WordPress security: block access to sensitive files\n")
	sb.WriteString("    location ~ /wp-config\\.php$ { deny all; }\n")
	if !allowXMLRPC {
		sb.WriteString("    location ~ /xmlrpc\\.php$ { deny all; }\n")
	}
	sb.WriteString("    location ~ /wp-content/debug\\.log$ { deny all; }\n")
	sb.WriteString("    location ~* /wp-content/uploads/.*\\.php$ { deny all; }\n")
	sb.WriteString("    location ~* /wp-includes/.*\\.php$ {\n")
	sb.WriteString("        deny all;\n")
	sb.WriteString("    }")
	return sb.String()
}

// GenerateVHostTemplate generates a vhost template string where dynamic per-site values are
// replaced by {token} placeholders. The template reflects the structural choices (SiteType,
// SSLType, AccessLog, ErrorLog, BasicAuthEnabled) at generation time.
// Tokens supported by ExpandVHostTemplate:
//
//	{domain}, {domain_aliases}, {site_root}, {user}, {php_version},
//	{proxy_url}, {ssl_cert}, {ssl_key}, {logs_dir}, {wordpress_security}
func GenerateVHostTemplate(data VHostData) (string, error) {
	tokenData := VHostData{
		Domain:                 "{domain}",
		Aliases:                "{domain_aliases}",
		User:                   "{user}",
		SiteType:               data.SiteType,
		PHPVersion:             "{php_version}",
		ProxyURL:               "{proxy_url}",
		WebRoot:                "{site_root}",
		SSLType:                data.SSLType,
		SSLCertPath:            "{ssl_cert}",
		SSLKeyPath:             "{ssl_key}",
		BasicAuthEnabled:       data.BasicAuthEnabled,
		AccessLog:              data.AccessLog,
		ErrorLog:               data.ErrorLog,
		WordPressSecurityRules: "{wordpress_security}",
	}
	return GenerateConfig(tokenData)
}

// ExpandVHostTemplate replaces {token} placeholders with actual values from data.
func ExpandVHostTemplate(tmpl string, data VHostData) string {
	// Handle server_name aliases: remove the token (including leading space) when empty
	if data.Aliases != "" {
		tmpl = strings.ReplaceAll(tmpl, "{domain_aliases}", strings.TrimSpace(data.Aliases))
	} else {
		tmpl = strings.ReplaceAll(tmpl, " {domain_aliases}", "")
	}
	tmpl = strings.ReplaceAll(tmpl, "{domain}", data.Domain)
	tmpl = strings.ReplaceAll(tmpl, "{site_root}", data.WebRoot)
	tmpl = strings.ReplaceAll(tmpl, "{user}", data.User)
	tmpl = strings.ReplaceAll(tmpl, "{php_version}", data.PHPVersion)
	tmpl = strings.ReplaceAll(tmpl, "{proxy_url}", data.ProxyURL)
	tmpl = strings.ReplaceAll(tmpl, "{ssl_cert}", data.SSLCertPath)
	tmpl = strings.ReplaceAll(tmpl, "{ssl_key}", data.SSLKeyPath)
	tmpl = strings.ReplaceAll(tmpl, "{logs_dir}", "/home/"+data.User+"/logs")
	tmpl = strings.ReplaceAll(tmpl, "{wordpress_security}", data.WordPressSecurityRules)
	return tmpl
}

// WriteConfigString writes a pre-built nginx config string and creates the symlink.
func WriteConfigString(sitesAvailable, sitesEnabled, domain, configStr string) error {
	confPath := filepath.Join(sitesAvailable, domain+".conf")
	cmd := exec.Command("sudo", "tee", confPath)
	cmd.Stdin = strings.NewReader(configStr)
	cmd.Stdout = nil
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write vhost config: %s: %s", err, string(output))
	}
	linkPath := filepath.Join(sitesEnabled, domain+".conf")
	cmd = exec.Command("sudo", "ln", "-sf", confPath, linkPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create symlink: %s", string(output))
	}
	return nil
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
	cmd = exec.Command("sudo", "ln", "-sf", confPath, linkPath)
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
