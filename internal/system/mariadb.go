package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// allHosts is the list of hosts to create MariaDB users for
// localhost = Unix socket, 127.0.0.1 = IPv4, ::1 = IPv6
var allHosts = []string{"localhost", "127.0.0.1", "::1"}

// escapeIdent escapes a MariaDB identifier (database/user name) for use inside backticks.
func escapeIdent(s string) string {
	return strings.ReplaceAll(s, "`", "``")
}

// escapeString escapes a MariaDB string literal for use inside single quotes.
func escapeString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `''`)
	return s
}

// MariaDBCreateDatabase creates a new MariaDB database
func MariaDBCreateDatabase(dbName string) error {
	sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", escapeIdent(dbName))
	return execMariaDB(sql)
}

// MariaDBDropDatabase drops a MariaDB database
func MariaDBDropDatabase(dbName string) error {
	sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", escapeIdent(dbName))
	return execMariaDB(sql)
}

// MariaDBCreateUser creates a MariaDB user for all localhost variants
func MariaDBCreateUser(username, password string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';", escapeString(username), escapeString(host), escapeString(password))
		if err := execMariaDB(sql); err != nil {
			return err
		}
	}
	return nil
}

// MariaDBDropUser drops a MariaDB user from all localhost variants
func MariaDBDropUser(username string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s';", escapeString(username), escapeString(host))
		execMariaDB(sql) // ignore errors — user might not exist on all hosts
	}
	return nil
}

// MariaDBGrantAccess grants a user access to a database on all hosts with the specified privilege level.
// Supported levels: readonly, readwrite, full
func MariaDBGrantAccess(username, dbName string, privileges ...string) error {
	level := "full"
	if len(privileges) > 0 && privileges[0] != "" {
		level = privileges[0]
	}

	var grantSQL string
	switch level {
	case "readonly":
		grantSQL = "SELECT"
	case "readwrite":
		grantSQL = "SELECT, INSERT, UPDATE, DELETE"
	default: // full
		grantSQL = "SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER, INDEX, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, TRIGGER, REFERENCES"
	}

	escapedDB := escapeIdent(dbName)
	escapedUser := escapeString(username)

	for _, host := range allHosts {
		escapedHost := escapeString(host)
		// Revoke first to reset any existing privileges
		revokeSQL := fmt.Sprintf("REVOKE ALL PRIVILEGES ON `%s`.* FROM '%s'@'%s';", escapedDB, escapedUser, escapedHost)
		execMariaDB(revokeSQL) // ignore errors — may not have grants yet

		sql := fmt.Sprintf("GRANT %s ON `%s`.* TO '%s'@'%s';", grantSQL, escapedDB, escapedUser, escapedHost)
		if err := execMariaDB(sql); err != nil {
			return err
		}
	}
	execMariaDB("FLUSH PRIVILEGES;")
	return nil
}

// MariaDBChangePassword changes a MariaDB user password on all hosts
func MariaDBChangePassword(username, newPassword string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s';", escapeString(username), escapeString(host), escapeString(newPassword))
		execMariaDB(sql) // ignore errors — user might not exist on all hosts
	}
	return nil
}

func execMariaDB(sql string) error {
	// Use sudo to run mysql as root — required because panel runs as jcwt-panel user
	// On Ubuntu with unix_socket auth, `sudo mysql` authenticates as the OS root user
	cmd := exec.Command("sudo", "mysql", "-e", sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mariadb error: %s: %s", err, string(output))
	}
	return nil
}
