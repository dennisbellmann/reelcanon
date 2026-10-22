package accounts

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionKey struct{}
type deliveryKey struct{}
type deliveryResult struct{ err error }
type user struct {
	ID, Email, Password, originalPassword, ConfirmSelector, ConfirmVerifier, originalConfirm, RecoverSelector, RecoverVerifier string
	Confirmed                                                                                                                  bool
	ConfirmExpiry, RecoverExpiry, LastAttempt, Locked                                                                          time.Time
	Attempts                                                                                                                   int
	Version                                                                                                                    int64
}

func (u *user) GetPID() string               { return u.Email }
func (u *user) PutPID(v string)              { u.Email = strings.ToLower(strings.TrimSpace(v)) }
func (u *user) GetEmail() string             { return u.Email }
func (u *user) PutEmail(v string)            { u.PutPID(v) }
func (u *user) GetPassword() string          { return u.Password }
func (u *user) PutPassword(v string)         { u.Password = v }
func (u *user) GetConfirmed() bool           { return u.Confirmed }
func (u *user) PutConfirmed(v bool)          { u.Confirmed = v }
func (u *user) GetConfirmSelector() string   { return u.ConfirmSelector }
func (u *user) PutConfirmSelector(v string)  { u.ConfirmSelector = v }
func (u *user) GetConfirmVerifier() string   { return u.ConfirmVerifier }
func (u *user) PutConfirmVerifier(v string)  { u.ConfirmVerifier = v }
func (u *user) GetRecoverSelector() string   { return u.RecoverSelector }
func (u *user) PutRecoverSelector(v string)  { u.RecoverSelector = v }
func (u *user) GetRecoverVerifier() string   { return u.RecoverVerifier }
func (u *user) PutRecoverVerifier(v string)  { u.RecoverVerifier = v }
func (u *user) GetRecoverExpiry() time.Time  { return u.RecoverExpiry }
func (u *user) PutRecoverExpiry(v time.Time) { u.RecoverExpiry = v }
func (u *user) GetAttemptCount() int         { return u.Attempts }
func (u *user) PutAttemptCount(v int)        { u.Attempts = v }
func (u *user) GetLastAttempt() time.Time    { return u.LastAttempt }
func (u *user) PutLastAttempt(v time.Time)   { u.LastAttempt = v }
func (u *user) GetLocked() time.Time         { return u.Locked }
func (u *user) PutLocked(v time.Time)        { u.Locked = v }

type storage struct{}

// Every email operation is executed in one transaction. Row locks make token
// consumption and password/lock updates atomic even for concurrent requests.
func txFrom(ctx context.Context) *sql.Tx { return ctx.Value(transactionKey{}).(*sql.Tx) }
func (*storage) load(ctx context.Context, predicate string, value string) (*user, error) {
	u := new(user)
	var confirmExpiry, recoverExpiry, lastAttempt, locked sql.NullTime
	err := txFrom(ctx).QueryRowContext(ctx, `SELECT u.id,c.email,c.password_hash,c.confirmed,
 c.confirm_selector,c.confirm_verifier,c.confirm_expires_at,
 c.recover_selector,c.recover_verifier,c.recover_expires_at,c.attempt_count,
 c.last_attempt_at,c.locked_at,u.session_version
 FROM users u JOIN user_credentials c ON c.user_id=u.id
 WHERE u.status='active' AND `+predicate+` FOR UPDATE OF u,c`, value).Scan(
		&u.ID, &u.Email, &u.Password, &u.Confirmed, &u.ConfirmSelector, &u.ConfirmVerifier, &confirmExpiry,
		&u.RecoverSelector, &u.RecoverVerifier, &recoverExpiry, &u.Attempts, &lastAttempt, &locked, &u.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.ConfirmExpiry = confirmExpiry.Time
	u.RecoverExpiry = recoverExpiry.Time
	u.LastAttempt = lastAttempt.Time
	u.Locked = locked.Time
	u.originalPassword = u.Password
	u.originalConfirm = u.ConfirmSelector
	return u, nil
}
func (s *storage) Load(ctx context.Context, key string) (authboss.User, error) {
	return s.load(ctx, "c.email=$1", strings.ToLower(strings.TrimSpace(key)))
}
func (s *storage) LoadByConfirmSelector(ctx context.Context, key string) (authboss.ConfirmableUser, error) {
	return s.load(ctx, "c.confirm_selector=$1 AND c.confirm_expires_at>now() AND c.confirm_selector<>''", key)
}
func (s *storage) LoadByRecoverSelector(ctx context.Context, key string) (authboss.RecoverableUser, error) {
	return s.load(ctx, "c.recover_selector=$1 AND c.recover_expires_at>now() AND c.recover_selector<>''", key)
}
func (*storage) New(context.Context) authboss.User { return &user{ID: uuid.NewString(), Version: 1} }
func (*storage) Create(ctx context.Context, value authboss.User) error {
	u := value.(*user)
	tx := txFrom(ctx)
	if _, err := tx.ExecContext(ctx, "SAVEPOINT create_account"); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO users(id) VALUES($1)", u.ID)
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO user_credentials(user_id,email,password_hash) VALUES($1,$2,$3)", u.ID, u.Email, u.Password)
	}
	if err != nil {
		_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT create_account")
		if rollbackErr != nil {
			return rollbackErr
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return authboss.ErrUserFound
		}
		return err
	}
	u.originalPassword = u.Password
	_, err = tx.ExecContext(ctx, "RELEASE SAVEPOINT create_account")
	return err
}
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
func (*storage) Save(ctx context.Context, value authboss.User) error {
	u := value.(*user)
	tx := txFrom(ctx)
	if u.Password != u.originalPassword {
		u.Attempts = 0
		u.LastAttempt = time.Time{}
		u.Locked = time.Time{}
	}
	if u.ConfirmSelector == "" {
		u.ConfirmExpiry = time.Time{}
	} else if u.ConfirmSelector != u.originalConfirm {
		u.ConfirmExpiry = time.Now().Add(24 * time.Hour)
	}
	if u.RecoverSelector == "" {
		u.RecoverExpiry = time.Time{}
	}
	result, err := tx.ExecContext(ctx, `UPDATE user_credentials SET password_hash=$2,confirmed=$3,
 confirm_selector=$4,confirm_verifier=$5,confirm_expires_at=$6,recover_selector=$7,
 recover_verifier=$8,recover_expires_at=$9,attempt_count=$10,last_attempt_at=$11,locked_at=$12 WHERE user_id=$1`,
		u.ID, u.Password, u.Confirmed, u.ConfirmSelector, u.ConfirmVerifier, nullableTime(u.ConfirmExpiry),
		u.RecoverSelector, u.RecoverVerifier, nullableTime(u.RecoverExpiry), u.Attempts, nullableTime(u.LastAttempt), nullableTime(u.Locked))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return authboss.ErrUserNotFound
	}
	if u.Password != u.originalPassword {
		err = tx.QueryRowContext(ctx, "UPDATE users SET session_version=session_version+1,updated_at=now() WHERE id=$1 RETURNING session_version", u.ID).Scan(&u.Version)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=$1", u.ID); err != nil {
			return err
		}
	}
	u.originalPassword = u.Password
	u.originalConfirm = u.ConfirmSelector
	return nil
}
