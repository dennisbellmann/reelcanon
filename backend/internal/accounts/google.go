package accounts

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type googleLogin struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
	cipher   cipher.AEAD
}

func newGoogleLogin(s *Service) (*googleLogin, error) {
	id, secret := os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET")
	if id == "" && secret == "" {
		return nil, nil
	}
	if id == "" || secret == "" {
		return nil, errors.New("both Google credentials are required")
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("AUTH_STATE_KEY"))
	if err != nil || len(key) != 32 {
		return nil, errors.New("AUTH_STATE_KEY must contain 32 random bytes encoded as base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, errors.New("Google discovery is unavailable")
	}
	return &googleLogin{config: oauth2.Config{ClientID: id, ClientSecret: secret, RedirectURL: s.publicURL + "/api/v1/auth/google/callback", Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}, verifier: provider.Verifier(&oidc.Config{ClientID: id}), cipher: aead}, nil
}
func (s *Service) googleStart(w http.ResponseWriter, r *http.Request) {
	if s.google == nil {
		jsonError(w, 503, "google_unavailable", "Google-Anmeldung ist noch nicht eingerichtet.")
		return
	}
	if s.session.GetString(r.Context(), "user_id") != "" {
		http.Redirect(w, r, s.publicURL+"/account", http.StatusSeeOther)
		return
	}
	if !s.rateAllowed(r) {
		jsonError(w, 429, "login_rate_limited", "Zu viele Anmeldeversuche.")
		return
	}
	state, nonce, verifier := randomToken(), randomToken(), oauth2.GenerateVerifier()
	s.session.Put(r.Context(), "google_binding", randomToken())
	token, _, err := s.session.Commit(r.Context())
	if err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung konnte nicht vorbereitet werden.")
		return
	}
	iv := make([]byte, s.google.cipher.NonceSize())
	if _, err = rand.Read(iv); err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung konnte nicht vorbereitet werden.")
		return
	}
	encrypted := s.google.cipher.Seal(iv, iv, []byte(verifier), []byte(state))
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO auth_login_requests(state_hash,session_hash,nonce_hash,verifier_ciphertext,expires_at)
 VALUES($1,$2,$3,$4,$5)`, tokenHash(state), tokenHash(token), tokenHash(nonce), encrypted, time.Now().Add(10*time.Minute))
	if err != nil {
		jsonError(w, 503, "login_unavailable", "Anmeldung konnte nicht vorbereitet werden.")
		return
	}
	// Commit above persists data; explicitly issue the anonymous browser cookie.
	s.session.WriteSessionCookie(r.Context(), w, token, time.Now().Add(s.session.Lifetime))
	http.Redirect(w, r, s.google.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("nonce", nonce)), http.StatusSeeOther)
}
func (s *Service) googleCallback(w http.ResponseWriter, r *http.Request) {
	fail := func() { http.Redirect(w, r, s.publicURL+"/account#login-error=google", http.StatusSeeOther) }
	if s.google == nil {
		fail()
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || len(state) > 128 || code == "" || len(code) > 4096 || s.session.GetString(r.Context(), "google_binding") == "" {
		fail()
		return
	}
	var encrypted []byte
	var nonceHash string
	err := s.db.QueryRowContext(r.Context(), `DELETE FROM auth_login_requests WHERE state_hash=$1
 AND session_hash=$2 AND expires_at>now() RETURNING verifier_ciphertext,nonce_hash`, tokenHash(state), tokenHash(s.session.Token(r.Context()))).Scan(&encrypted, &nonceHash)
	if err != nil {
		fail()
		return
	}
	size := s.google.cipher.NonceSize()
	if len(encrypted) < size {
		fail()
		return
	}
	verifier, err := s.google.cipher.Open(nil, encrypted[:size], encrypted[size:], []byte(state))
	if err != nil {
		fail()
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := s.google.config.Exchange(ctx, code, oauth2.VerifierOption(string(verifier)))
	if err != nil {
		fail()
		return
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		fail()
		return
	}
	idToken, err := s.google.verifier.Verify(ctx, raw)
	if err != nil || subtle.ConstantTimeCompare([]byte(tokenHash(idToken.Nonce)), []byte(nonceHash)) != 1 {
		fail()
		return
	}
	var claims struct {
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
	}
	if err = idToken.Claims(&claims); err != nil || !claims.Verified || idToken.Subject == "" {
		fail()
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		fail()
		return
	}
	defer tx.Rollback()
	id, version, err := googleAccount(ctx, tx, idToken.Subject, claims.Email)
	if err != nil {
		fail()
		return
	}
	if err = tx.Commit(); err != nil {
		fail()
		return
	}
	if err = s.startSession(r.Context(), id, version); err != nil {
		fail()
		return
	}
	http.Redirect(w, r, s.publicURL+"/account", http.StatusSeeOther)
}

// Both clients resolve the same stable provider identity. Email is profile data.
func googleAccount(ctx context.Context, tx *sql.Tx, subject, email string) (string, int64, error) {
	var err error
	// Serialize account creation for this provider identity, without email-based merging.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "google:"+subject); err != nil {
		return "", 0, err
	}
	var id string
	var version int64
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.session_version FROM users u JOIN user_auth_identities i ON i.user_id=u.id
 WHERE i.provider='google' AND i.issuer='https://accounts.google.com' AND i.subject=$1 AND u.status='active' FOR UPDATE OF u`, subject).Scan(&id, &version)
	if errors.Is(err, sql.ErrNoRows) {
		// A blocked identity must never create another account.
		var exists bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_auth_identities WHERE provider='google' AND subject=$1)", subject).Scan(&exists); err != nil || exists {
			return "", 0, errUnauthenticated
		}
		id = uuid.NewString()
		version = 1
		if _, err = tx.ExecContext(ctx, "INSERT INTO users(id) VALUES($1)", id); err != nil {
			return "", 0, errUnauthenticated
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_auth_identities(user_id,provider,issuer,subject,email) VALUES($1,'google','https://accounts.google.com',$2,$3)`, id, subject, email); err != nil {
			return "", 0, errUnauthenticated
		}
	} else if err != nil {
		return "", 0, err
	}
	return id, version, nil
}
