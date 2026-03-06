package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateSystemUser creates an isolated system user for a site
func CreateSystemUser(username, webRootBase string) error {
	homeDir := filepath.Join(webRootBase, username)

	// Create user with restricted shell (requires sudo)
	cmd := exec.Command("sudo", "useradd",
		"--system",
		"--shell", "/usr/sbin/nologin",
		"--home-dir", homeDir,
		"--create-home",
		username,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create user %s: %s", username, string(output))
	}

	// Create web root directory
	webRoot := filepath.Join(homeDir, "htdocs")
	if err := os.MkdirAll(webRoot, 0750); err != nil {
		// Try with sudo if permission denied
		cmd = exec.Command("sudo", "mkdir", "-p", webRoot)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create web root: %s", string(output))
		}
	}

	// Set ownership
	cmd = exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", username, username), homeDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown: %s", string(output))
	}

	// Set permissions
	cmd = exec.Command("sudo", "chmod", "750", homeDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chmod home: %s", string(output))
	}

	cmd = exec.Command("sudo", "chmod", "750", webRoot)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chmod webroot: %s", string(output))
	}

	// Add www-data to user's group (so Nginx can read)
	cmd = exec.Command("sudo", "usermod", "-aG", username, "www-data")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add www-data to group: %s", string(output))
	}

	return nil
}

// WriteWelcomePage creates a default welcome page based on the site type
func WriteWelcomePage(webRoot, siteType, domain, username string) error {
	var indexContent, fileName string

	switch siteType {
	case "php":
		fileName = "index.php"
		indexContent = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s — Site Ready</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: linear-gradient(135deg, #f8fafc 0%%, #e2e8f0 100%%); color: #334155; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
        .card { max-width: 640px; width: 90%%; background: #ffffff; border-radius: 16px; padding: 48px; border: 1px solid #e2e5ef; box-shadow: 0 4px 24px rgba(0,0,0,0.06); }
        h1 { font-size: 2rem; background: linear-gradient(135deg, #2563eb, #3b82f6); -webkit-background-clip: text; -webkit-text-fill-color: transparent; margin-bottom: 8px; }
        .subtitle { color: #64748b; margin-bottom: 32px; }
        .badge { display: inline-block; background: #eff6ff; color: #2563eb; padding: 6px 14px; border-radius: 8px; font-size: 0.875rem; font-weight: 600; margin-bottom: 24px; }
        .steps { background: #f8fafc; border-radius: 12px; padding: 24px; margin-top: 24px; border: 1px solid #e2e5ef; }
        .steps h3 { color: #2563eb; font-size: 0.875rem; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 16px; }
        .step { display: flex; gap: 12px; margin-bottom: 12px; font-size: 0.9rem; color: #64748b; }
        .step-num { background: #eff6ff; color: #2563eb; width: 24px; height: 24px; border-radius: 50%%; display: flex; align-items: center; justify-content: center; font-size: 0.75rem; font-weight: bold; flex-shrink: 0; }
        code { background: #f1f5f9; padding: 2px 6px; border-radius: 4px; font-size: 0.85rem; color: #334155; font-family: inherit; font-weight: 600; }
        .info { display: flex; gap: 20px; margin-top: 24px; flex-wrap: wrap; }
        .info-item { font-size: 0.85rem; color: #94a3b8; }
        .info-item strong { color: #64748b; }
    </style>
</head>
<body>
    <div class="card">
        <h1>%s</h1>
        <p class="subtitle">Your PHP site is live and ready for deployment.</p>
        <div class="badge"><?php echo "PHP " . phpversion(); ?></div>
        <div class="info">
            <div class="info-item"><strong>Web Root:</strong> %s</div>
            <div class="info-item"><strong>User:</strong> %s</div>
        </div>
        <div class="steps">
            <h3>Next Steps</h3>
            <div class="step"><div class="step-num">1</div> Upload your application files to <code>%s</code></div>
            <div class="step"><div class="step-num">2</div> Point your domain's DNS A/AAAA record to this server</div>
            <div class="step"><div class="step-num">3</div> Configure SSL via the panel for HTTPS</div>
            <div class="step"><div class="step-num">4</div> Delete this file (<code>index.php</code>) after uploading your app</div>
        </div>
    </div>
</body>
</html>`, domain, domain, webRoot, username, webRoot)
	case "html":
		fileName = "index.html"
		indexContent = fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s — Site Ready</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: linear-gradient(135deg, #f8fafc 0%%, #e2e8f0 100%%); color: #334155; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
        .card { max-width: 640px; width: 90%%; background: #ffffff; border-radius: 16px; padding: 48px; border: 1px solid #e2e5ef; box-shadow: 0 4px 24px rgba(0,0,0,0.06); }
        h1 { font-size: 2rem; background: linear-gradient(135deg, #2563eb, #3b82f6); -webkit-background-clip: text; -webkit-text-fill-color: transparent; margin-bottom: 8px; }
        .subtitle { color: #64748b; margin-bottom: 32px; }
        .badge { display: inline-block; background: #eff6ff; color: #2563eb; padding: 6px 14px; border-radius: 8px; font-size: 0.875rem; font-weight: 600; margin-bottom: 24px; }
        .steps { background: #f8fafc; border-radius: 12px; padding: 24px; margin-top: 24px; border: 1px solid #e2e5ef; }
        .steps h3 { color: #2563eb; font-size: 0.875rem; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 16px; }
        .step { display: flex; gap: 12px; margin-bottom: 12px; font-size: 0.9rem; color: #64748b; }
        .step-num { background: #eff6ff; color: #2563eb; width: 24px; height: 24px; border-radius: 50%%; display: flex; align-items: center; justify-content: center; font-size: 0.75rem; font-weight: bold; flex-shrink: 0; }
        code { background: #f1f5f9; padding: 2px 6px; border-radius: 4px; font-size: 0.85rem; color: #334155; font-family: inherit; font-weight: 600; }
        .info { display: flex; gap: 20px; margin-top: 24px; flex-wrap: wrap; }
        .info-item { font-size: 0.85rem; color: #94a3b8; }
        .info-item strong { color: #64748b; }
    </style>
</head>
<body>
    <div class="card">
        <h1>%s</h1>
        <p class="subtitle">Your static site is live and ready for your content.</p>
        <div class="badge">Static HTML Site</div>
        <div class="info">
            <div class="info-item"><strong>Web Root:</strong> %s</div>
            <div class="info-item"><strong>User:</strong> %s</div>
        </div>
        <div class="steps">
            <h3>Getting Started</h3>
            <div class="step"><div class="step-num">1</div> Delete this default <code>index.html</code> file</div>
            <div class="step"><div class="step-num">2</div> Upload your website files to <code>%s</code></div>
            <div class="step"><div class="step-num">3</div> Make sure you have an <code>index.html</code> in the root</div>
            <div class="step"><div class="step-num">4</div> Point your domain's DNS A/AAAA record to this server</div>
            <div class="step"><div class="step-num">5</div> Configure SSL via the panel for HTTPS</div>
        </div>
    </div>
</body>
</html>`, domain, domain, webRoot, username, webRoot)
	default:
		// Proxies don't need a welcome page
		return nil
	}

	indexPath := filepath.Join(webRoot, fileName)

	// Write via sudo bash -c to handle permissions
	cmd := exec.Command("sudo", "bash", "-c", fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", indexPath, indexContent))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write welcome page: %s", string(output))
	}

	cmd = exec.Command("sudo", "chown", fmt.Sprintf("%s:%s", username, username), indexPath)
	cmd.Run()

	cmd = exec.Command("sudo", "chmod", "644", indexPath)
	cmd.Run()

	return nil
}

// DeleteSystemUser removes a system user and their home directory
func DeleteSystemUser(username string) error {
	// Force delete with home directory removal
	cmd := exec.Command("sudo", "userdel", "--force", "--remove", username)
	cmd.Run() // Ignore error — user might already be deleted

	// Also delete the group (userdel sometimes leaves it behind)
	cmd = exec.Command("sudo", "groupdel", username)
	cmd.Run() // Ignore error — group might not exist

	return nil
}

// ClearCrontab clears the crontab for a user
func ClearCrontab(username string) {
	cmd := exec.Command("sudo", "crontab", "-u", username, "-r")
	cmd.Run()
}

// UserExists checks if a system user exists
func UserExists(username string) bool {
	cmd := exec.Command("id", username)
	return cmd.Run() == nil
}
