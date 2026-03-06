package system

import (
	"fmt"
	"os/exec"
)

// allHosts is the list of hosts to create MariaDB users for
// localhost = Unix socket, 127.0.0.1 = IPv4, ::1 = IPv6
var allHosts = []string{"localhost", "127.0.0.1", "::1"}

// MariaDBCreateDatabase creates a new MariaDB database
func MariaDBCreateDatabase(dbName string) error {
	sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
	return execMariaDB(sql)
}

// MariaDBDropDatabase drops a MariaDB database
func MariaDBDropDatabase(dbName string) error {
	sql := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName)
	return execMariaDB(sql)
}

// MariaDBCreateUser creates a MariaDB user for all localhost variants
func MariaDBCreateUser(username, password string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';", username, host, password)
		if err := execMariaDB(sql); err != nil {
			return err
		}
	}
	return nil
}

// MariaDBDropUser drops a MariaDB user from all localhost variants
func MariaDBDropUser(username string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s';", username, host)
		execMariaDB(sql) // ignore errors — user might not exist on all hosts
	}
	return nil
}

// MariaDBGrantAccess grants a user full access to a database on all hosts
func MariaDBGrantAccess(username, dbName string) error {
	for _, host := range allHosts {
		sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';", dbName, username, host)
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
		sql := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s';", username, host, newPassword)
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
