// This explicit development command configures accounts for a local PostgreSQL
// instance without printing or passing credentials as process arguments.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/reelcanon/reelcanon/backend/internal/database"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Local account setup failed:", err)
		os.Exit(1)
	}
}
func run() error {
	path, err := filepath.Abs("../.env")
	if err != nil {
		return err
	}
	values, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("read repository .env")
	}
	owner, err := url.Parse(values["DATABASE_URL"])
	if err != nil || owner.User == nil {
		return fmt.Errorf("invalid database configuration")
	}
	if owner.Hostname() != "127.0.0.1" && owner.Hostname() != "localhost" && owner.Hostname() != "::1" {
		return fmt.Errorf("this command only configures a loopback development database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, owner.String())
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer db.Close()
	if err = database.ApplyMigrations(ctx, db, "../database/migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin setup transaction")
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='reelcanon_accounts')").Scan(&exists); err != nil {
		return fmt.Errorf("read account role")
	}
	accountURL := values["USER_DATABASE_URL"]
	if accountURL == "" {
		if exists {
			return fmt.Errorf("role reelcanon_accounts already exists; configure USER_DATABASE_URL before retrying")
		}
		password := make([]byte, 32)
		if _, err = rand.Read(password); err != nil {
			return err
		}
		secret := base64.RawURLEncoding.EncodeToString(password)
		// Generated base64url contains no SQL metacharacters.
		if _, err = tx.ExecContext(ctx, "CREATE ROLE reelcanon_accounts LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT PASSWORD '"+secret+"'"); err != nil {
			return fmt.Errorf("create account role")
		}
		account := *owner
		account.User = url.UserPassword("reelcanon_accounts", secret)
		accountURL = account.String()
	} else {
		account, parseErr := url.Parse(accountURL)
		if parseErr != nil || account.User == nil || account.User.Username() != "reelcanon_accounts" || account.Host != owner.Host || account.Path != owner.Path || !exists {
			return fmt.Errorf("existing USER_DATABASE_URL must reference reelcanon_accounts in this local database")
		}
	}
	tables := "users,user_credentials,user_auth_identities,sessions,auth_login_requests,user_movie_progress,mobile_login_challenges,mobile_sessions,mobile_refresh_tokens,progress_operations"
	for _, query := range []string{
		"GRANT USAGE ON SCHEMA public TO reelcanon_accounts",
		"GRANT SELECT ON movies TO reelcanon_accounts",
		"GRANT SELECT,INSERT,UPDATE,DELETE ON " + tables + " TO reelcanon_accounts",
		"REVOKE ALL ON " + tables + " FROM PUBLIC",
	} {
		if _, err = tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("set account permissions")
		}
	}
	if raw := values["API_DATABASE_URL"]; raw != "" {
		api, parseErr := url.Parse(raw)
		if parseErr != nil || api.User == nil || api.User.Username() == owner.User.Username() {
			return fmt.Errorf("API_DATABASE_URL must use a separate catalog role")
		}
		role := pgx.Identifier{api.User.Username()}.Sanitize()
		if _, err = tx.ExecContext(ctx, "REVOKE ALL ON "+tables+" FROM "+role); err != nil {
			return fmt.Errorf("restrict catalog role")
		}
	}
	values["AUTH_ENABLED"] = "true"
	values["USER_DATABASE_URL"] = accountURL
	if values["AUTH_PUBLIC_URL"] == "" {
		values["AUTH_PUBLIC_URL"] = "http://127.0.0.1:8080"
	}
	if values["AUTH_ALLOWED_ORIGINS"] == "" {
		values["AUTH_ALLOWED_ORIGINS"] = "http://127.0.0.1:5173,http://localhost:5173,http://localhost:8080"
	}
	values["AUTH_MAIL_MODE"] = "file"
	values["AUTH_MAIL_DIR"] = "../.local/mail"
	if values["AUTH_STATE_KEY"] == "" {
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return err
		}
		values["AUTH_STATE_KEY"] = base64.StdEncoding.EncodeToString(key)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit account permissions")
	}
	// Preserve comments and unrelated configuration. This file is ignored by Git.
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read local configuration")
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, key := range []string{"AUTH_ENABLED", "USER_DATABASE_URL", "AUTH_PUBLIC_URL", "AUTH_ALLOWED_ORIGINS", "AUTH_MAIL_MODE", "AUTH_MAIL_DIR", "AUTH_STATE_KEY"} {
		found := false
		for i, line := range lines {
			if strings.HasPrefix(line, key+"=") {
				lines[i] = key + "=" + values[key]
				found = true
			}
		}
		if !found {
			lines = append(lines, key+"="+values[key])
		}
	}
	if err = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return fmt.Errorf("write local configuration; account role was created")
	}
	fmt.Println("Local accounts enabled. Credentials remain in .env. Start make api and make dev-mail.")
	return nil
}
