package accounts

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
)

type mobileAuthKey struct{}
type mobileAuth struct {
	profile    Profile
	version    int64
	sessionID  string
	accessHash string
}
type mobileTokens struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	ExpiresIn    int     `json:"expiresIn"`
	User         Profile `json:"user"`
}

func (s *Service) authVersion(ctx context.Context) int64 {
	if auth, ok := ctx.Value(mobileAuthKey{}).(mobileAuth); ok {
		return auth.version
	}
	return s.session.GetInt64(ctx, "version")
}
func (s *Service) mobileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/mobile/google/challenge", s.mobileRate(s.mobileChallenge))
	mux.HandleFunc("POST /api/v1/auth/mobile/google", s.mobileRate(s.mobileGoogle))
	mux.HandleFunc("POST /api/v1/auth/mobile/refresh", s.mobileRate(s.mobileRefresh))
	mux.HandleFunc("POST /api/v1/auth/mobile/logout", s.mobileRate(s.mobileLogout))
	mux.HandleFunc("GET /api/v1/mobile/me", s.mobileBearer(func(w http.ResponseWriter, r *http.Request) {
		profile, ok := s.require(w, r)
		if !ok {
			return
		}
		writeJSON(w, 200, map[string]any{"user": profile})
	}))
	mux.HandleFunc("GET /api/v1/mobile/me/progress", s.mobileBearer(s.listProgress))
	mux.HandleFunc("PUT /api/v1/mobile/me/progress/{imdbId}/{field}", s.mobileBearer(s.changeProgress))
	mux.HandleFunc("POST /api/v1/mobile/me/progress/import", s.mobileBearer(s.importProgress))
}
func (s *Service) mobileRate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.rateAllowed(r) {
			w.Header().Set("Retry-After", "600")
			jsonError(w, 429, "too_many_attempts", "Zu viele Versuche.")
			return
		}
		next(w, r)
	}
}
func (s *Service) mobileBearer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || len(header) != 50 {
			jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
			return
		}
		var auth mobileAuth
		err := s.db.QueryRowContext(r.Context(), `SELECT u.id,COALESCE(c.email,i.email,''),u.session_version,s.id
   FROM mobile_sessions s JOIN users u ON u.id=s.user_id
   LEFT JOIN user_credentials c ON c.user_id=u.id LEFT JOIN user_auth_identities i ON i.user_id=u.id
   WHERE s.access_hash=$1 AND s.access_expires_at>now() AND s.expires_at>now() AND NOT s.revoked
   AND u.status='active' AND s.session_version=u.session_version`, tokenHash(header[7:])).Scan(&auth.profile.ID, &auth.profile.Email, &auth.version, &auth.sessionID)
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
			return
		}
		if err != nil {
			jsonError(w, 503, "account_unavailable", "Konto nicht verfügbar.")
			return
		}
		if expected := r.Header.Get("X-Account-ID"); expected != "" && expected != auth.profile.ID {
			jsonError(w, 409, "account_changed", "Konto geändert.")
			return
		}
		auth.accessHash = tokenHash(header[7:])
		next(w, r.WithContext(context.WithValue(r.Context(), mobileAuthKey{}, auth)))
	}
}
func (s *Service) mobileChallenge(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || len(s.mobileClients) == 0 {
		jsonError(w, 503, "google_unavailable", "Mobile Google-Anmeldung ist noch nicht eingerichtet.")
		return
	}
	var input struct{}
	if decodeBody(w, r, 16384, &input) != nil {
		jsonError(w, 400, "invalid_account", "Ungültige Anfrage.")
		return
	}
	id, nonce := uuid.NewString(), randomToken()
	if _, err := s.db.ExecContext(r.Context(), "INSERT INTO mobile_login_challenges(id,nonce_hash,expires_at) VALUES($1,$2,$3)", id, tokenHash(nonce), time.Now().Add(5*time.Minute)); err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung nicht verfügbar.")
		return
	}
	writeJSON(w, 200, map[string]any{"challengeId": id, "nonce": nonce, "serverClientId": s.google.config.ClientID, "expiresIn": 300})
}
func validateMobileClaims(token *oidc.IDToken, allowed map[string]bool, nonceHash string) (string, error) {
	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
		Party    string `json:"azp"`
	}
	if err := token.Claims(&claims); err != nil {
		return "", err
	}
	party := claims.Party
	if party == "" && len(token.Audience) == 1 {
		party = token.Audience[0]
	}
	if !allowed[party] || token.Subject == "" || !claims.Verified || claims.Email == "" || token.Nonce == "" || subtle.ConstantTimeCompare([]byte(tokenHash(token.Nonce)), []byte(nonceHash)) != 1 {
		return "", errUnauthenticated
	}
	return claims.Email, nil
}
func (s *Service) mobileGoogle(w http.ResponseWriter, r *http.Request) {
	if s.google == nil || len(s.mobileClients) == 0 {
		jsonError(w, 503, "google_unavailable", "Mobile Google-Anmeldung ist noch nicht eingerichtet.")
		return
	}
	var input struct {
		ChallengeID string `json:"challengeId"`
		IDToken     string `json:"idToken"`
	}
	if decodeBody(w, r, 16384, &input) != nil {
		jsonError(w, 400, "invalid_account", "Ungültige Anfrage.")
		return
	}
	if _, err := uuid.Parse(input.ChallengeID); err != nil {
		jsonError(w, 401, "google_login_failed", "Google-Anmeldung fehlgeschlagen.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := s.google.verifier.Verify(ctx, input.IDToken)
	if err != nil {
		jsonError(w, 401, "google_login_failed", "Google-Anmeldung fehlgeschlagen.")
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung nicht verfügbar.")
		return
	}
	defer tx.Rollback()
	var nonceHash string
	err = tx.QueryRowContext(ctx, "SELECT nonce_hash FROM mobile_login_challenges WHERE id=$1 AND expires_at>now() FOR UPDATE", input.ChallengeID).Scan(&nonceHash)
	if err != nil {
		jsonError(w, 401, "google_login_failed", "Google-Anmeldung fehlgeschlagen.")
		return
	}
	email, err := validateMobileClaims(token, s.mobileClients, nonceHash)
	if err != nil {
		jsonError(w, 401, "google_login_failed", "Google-Anmeldung fehlgeschlagen.")
		return
	}
	id, version, err := googleAccount(ctx, tx, token.Subject, email)
	if err != nil {
		jsonError(w, 401, "google_login_failed", "Google-Anmeldung fehlgeschlagen.")
		return
	}
	result := mobileTokens{AccessToken: randomToken(), RefreshToken: randomToken(), ExpiresIn: 900, User: Profile{ID: id, Email: email}}
	family := uuid.NewString()
	_, err = tx.ExecContext(ctx, `INSERT INTO mobile_sessions(id,user_id,session_version,access_hash,access_expires_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, family, id, version, tokenHash(result.AccessToken), time.Now().Add(15*time.Minute), time.Now().Add(30*24*time.Hour))
	if err == nil {
		_, err = tx.ExecContext(ctx, "INSERT INTO mobile_refresh_tokens(token_hash,session_id) VALUES($1,$2)", tokenHash(result.RefreshToken), family)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "DELETE FROM mobile_login_challenges WHERE id=$1", input.ChallengeID)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung nicht verfügbar.")
		return
	}
	writeJSON(w, 200, result)
}
func refreshInput(w http.ResponseWriter, r *http.Request) (string, bool) {
	var input struct {
		Token string `json:"refreshToken"`
	}
	if decodeBody(w, r, 16384, &input) != nil || len(input.Token) != 43 {
		jsonError(w, 400, "invalid_account", "Ungültige Anfrage.")
		return "", false
	}
	return tokenHash(input.Token), true
}
func (s *Service) mobileRefresh(w http.ResponseWriter, r *http.Request) {
	hash, ok := refreshInput(w, r)
	if !ok {
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 503, "session_unavailable", "Sitzung nicht verfügbar.")
		return
	}
	defer tx.Rollback()
	// Lock the user before the family, consistent with progress/logout serialization.
	var userID string
	if err = tx.QueryRowContext(r.Context(), `SELECT s.user_id FROM mobile_refresh_tokens t JOIN mobile_sessions s ON s.id=t.session_id WHERE t.token_hash=$1`, hash).Scan(&userID); err != nil {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	var version int64
	var active bool
	err = tx.QueryRowContext(r.Context(), "SELECT session_version,status='active' FROM users WHERE id=$1 FOR UPDATE", userID).Scan(&version, &active)
	if err != nil {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	var family string
	var spent, revoked bool
	var storedVersion int64
	var expiry time.Time
	err = tx.QueryRowContext(r.Context(), `SELECT s.id,t.spent,s.revoked,s.session_version,s.expires_at FROM mobile_refresh_tokens t JOIN mobile_sessions s ON s.id=t.session_id WHERE t.token_hash=$1 FOR UPDATE OF s,t`, hash).Scan(&family, &spent, &revoked, &storedVersion, &expiry)
	if err != nil {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	if spent || revoked || !active || storedVersion != version || !expiry.After(time.Now()) {
		if _, err = tx.ExecContext(r.Context(), "UPDATE mobile_sessions SET revoked=true WHERE id=$1", family); err != nil {
			jsonError(w, 503, "session_unavailable", "Sitzung nicht verfügbar.")
			return
		}
		if err = tx.Commit(); err != nil {
			jsonError(w, 503, "session_unavailable", "Sitzung nicht verfügbar.")
			return
		}
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	result := mobileTokens{AccessToken: randomToken(), RefreshToken: randomToken(), ExpiresIn: 900, User: Profile{ID: userID}}
	err = tx.QueryRowContext(r.Context(), `SELECT COALESCE(c.email,i.email,'') FROM users u LEFT JOIN user_credentials c ON c.user_id=u.id LEFT JOIN user_auth_identities i ON i.user_id=u.id WHERE u.id=$1`, userID).Scan(&result.User.Email)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE mobile_refresh_tokens SET spent=true WHERE token_hash=$1", hash)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "INSERT INTO mobile_refresh_tokens(token_hash,session_id) VALUES($1,$2)", tokenHash(result.RefreshToken), family)
	}
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE mobile_sessions SET access_hash=$2,access_expires_at=$3 WHERE id=$1", family, tokenHash(result.AccessToken), time.Now().Add(15*time.Minute))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		jsonError(w, 503, "session_unavailable", "Sitzung nicht verfügbar.")
		return
	}
	writeJSON(w, 200, result)
}
func (s *Service) mobileLogout(w http.ResponseWriter, r *http.Request) {
	hash, ok := refreshInput(w, r)
	if !ok {
		return
	}
	// User lock orders logout against in-flight progress updates.
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonError(w, 503, "logout_failed", "Abmelden fehlgeschlagen.")
		return
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(r.Context(), `SELECT u.id FROM users u JOIN mobile_sessions s ON s.user_id=u.id JOIN mobile_refresh_tokens t ON t.session_id=s.id WHERE t.token_hash=$1 FOR UPDATE OF u`, hash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		// Keep logout idempotent even after session cleanup removed the token.
		err = nil
	} else if err == nil {
		_, err = tx.ExecContext(r.Context(), `UPDATE mobile_sessions SET revoked=true WHERE id=(SELECT session_id FROM mobile_refresh_tokens WHERE token_hash=$1)`, hash)
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		jsonError(w, 503, "logout_failed", "Abmelden fehlgeschlagen.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// Recheck after acquiring the account lock: logout may have raced with middleware.
func (s *Service) validMobileWrite(ctx context.Context, tx *sql.Tx) bool {
	auth, ok := ctx.Value(mobileAuthKey{}).(mobileAuth)
	if !ok {
		return true
	}
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT NOT revoked AND access_expires_at>now() AND expires_at>now() AND access_hash=$2 FROM mobile_sessions WHERE id=$1`, auth.sessionID, auth.accessHash).Scan(&valid)
	return err == nil && valid
}
