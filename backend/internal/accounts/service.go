package accounts

import (
	"bytes"
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aarondl/authboss/v3"
	_ "github.com/aarondl/authboss/v3/auth"
	"github.com/aarondl/authboss/v3/confirm"
	"github.com/aarondl/authboss/v3/defaults"
	_ "github.com/aarondl/authboss/v3/lock"
	_ "github.com/aarondl/authboss/v3/recover"
	_ "github.com/aarondl/authboss/v3/register"
	"github.com/alexedwards/scs/v2"
)

type Service struct {
	db            *sql.DB
	session       *scs.SessionManager
	boss          *authboss.Authboss
	logger        *slog.Logger
	publicURL     string
	origins       map[string]bool
	mailDirectory string
	google        *googleLogin
	mobileClients map[string]bool
	limitMu       sync.Mutex
	limits        map[string]attemptWindow
}
type attemptWindow struct {
	started time.Time
	count   int
}
type Profile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

var errUnauthenticated = errors.New("authentication required")

func New(db *sql.DB, logger *slog.Logger) (*Service, error) {
	publicURL := strings.TrimRight(os.Getenv("AUTH_PUBLIC_URL"), "/")
	parsed, err := url.Parse(publicURL)
	if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, errors.New("AUTH_PUBLIC_URL must be an origin")
	}
	local := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
	if parsed.Scheme != "https" && !(local && parsed.Scheme == "http") {
		return nil, errors.New("authentication requires HTTPS outside localhost")
	}
	if os.Getenv("AUTH_MAIL_MODE") != "file" || !local {
		return nil, errors.New("this development release requires AUTH_MAIL_MODE=file and a loopback AUTH_PUBLIC_URL")
	}
	directory := os.Getenv("AUTH_MAIL_DIR")
	if directory == "" {
		directory = "../.local/mail"
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	s := &Service{db: db, logger: logger, publicURL: publicURL, mailDirectory: directory, origins: map[string]bool{publicURL: true}, limits: make(map[string]attemptWindow)}
	for _, origin := range strings.Split(os.Getenv("AUTH_ALLOWED_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		u, e := url.Parse(origin)
		if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return nil, errors.New("invalid AUTH_ALLOWED_ORIGINS")
		}
		if local && (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
			return nil, errors.New("development origins must be loopback")
		}
		s.origins[origin] = true
	}
	sessions := scs.New()
	// Keep browser logins across restarts and up to 30 days between visits.
	sessions.Lifetime = 30 * 24 * time.Hour
	sessions.IdleTimeout = 30 * 24 * time.Hour
	sessions.Cookie.Name = "reelcanon_session"
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.Secure = parsed.Scheme == "https"
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	sessions.Cookie.Persist = true
	sessions.Store = &sessionStore{db: db, manager: sessions}
	sessions.ErrorFunc = func(w http.ResponseWriter, _ *http.Request, _ error) {
		jsonError(w, 503, "session_unavailable", "Die Sitzung ist vorübergehend nicht verfügbar.")
	}
	s.session = sessions
	ab := authboss.New()
	ab.Config.Core.ViewRenderer = defaults.JSONRenderer{}
	defaults.SetCore(&ab.Config, true, false)
	ab.Config.Core.Logger = quietLogger{}
	ab.Config.Core.ErrorHandler = safeErrors{}
	ab.Config.Core.Hasher = authboss.NewBCryptHasher(12)
	ab.Config.Storage.Server = &storage{}
	ab.Config.Storage.SessionState = clientStorage{sessions}
	ab.Config.Core.MailRenderer = FileMail{directory, publicURL}
	ab.Config.Core.Mailer = FileMail{directory, publicURL}
	ab.Config.Core.Redirector.(*defaults.Redirector).CorceRedirectTo200 = true
	reader := ab.Config.Core.BodyReader.(*defaults.HTTPBodyReader)
	emailRule := defaults.Rules{FieldName: "email", Required: true, MaxLength: 320, MustMatch: regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`), MatchError: "Valid email required"}
	passwordRule := defaults.Rules{FieldName: "password", Required: true, MinLength: 12, MaxLength: 72, AllowWhitespace: true}
	reader.Rulesets["register"] = []defaults.Rules{emailRule, passwordRule}
	reader.Rulesets["recover_end"] = []defaults.Rules{passwordRule}
	ab.Config.Paths.Mount = "/api/v1/auth"
	ab.Config.Paths.RootURL = publicURL
	ab.Config.Paths.AuthLoginOK = "/account"
	ab.Config.Paths.RegisterOK = "/account"
	ab.Config.Paths.ConfirmOK = "/account"
	ab.Config.Paths.ConfirmNotOK = "/account"
	ab.Config.Paths.RecoverOK = "/account"
	ab.Config.Paths.LockNotOK = "/account"
	ab.Config.Modules.MailRouteMethod = http.MethodPost
	ab.Config.Modules.MailNoGoroutine = true
	ab.Config.Modules.RecoverTokenDuration = time.Hour
	ab.Config.Modules.RecoverLoginAfterRecovery = false
	ab.Config.Modules.LockAfter = 5
	ab.Config.Modules.LockDuration = 15 * time.Minute
	if err = ab.Init("auth", "register", "confirm", "recover", "lock"); err != nil {
		return nil, err
	}
	ab.Config.Core.Router.Post("/confirm/resend", safeErrors{}.Wrap(func(w http.ResponseWriter, r *http.Request) error {
		var input struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			jsonError(w, 400, "invalid_email", "Ungültige E-Mail-Adresse.")
			return nil
		}
		value, err := ab.Config.Storage.Server.Load(r.Context(), input.Email)
		if err != nil && !errors.Is(err, authboss.ErrUserNotFound) {
			return err
		}
		if err == nil {
			account := value.(*user)
			if !account.Confirmed {
				module := confirm.Confirm{Authboss: ab}
				if err = module.StartConfirmation(r.Context(), account, true); err != nil {
					return err
				}
			}
		}
		writeJSON(w, 200, map[string]string{"status": "success"})
		return nil
	}))
	s.boss = ab
	ab.Events.After(authboss.EventAuth, func(_ http.ResponseWriter, r *http.Request, handled bool) (bool, error) {
		if handled {
			return false, nil
		}
		value, err := ab.CurrentUser(r)
		if err != nil {
			return false, err
		}
		u := value.(*user)
		return false, s.startSession(r.Context(), u.ID, u.Version)
	})
	// Clean up expired session data independently from request handling.
	s.google, err = newGoogleLogin(s)
	if err != nil {
		return nil, err
	}

	s.mobileClients = map[string]bool{}
	for _, id := range strings.Split(os.Getenv("GOOGLE_MOBILE_CLIENT_IDS"), ",") {
		if id = strings.TrimSpace(id); id != "" {
			s.mobileClients[id] = true
		}
	}
	return s, nil
}

func Disabled(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/me", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"enabled": false, "user": nil, "csrfToken": "", "providers": map[string]bool{"google": false}})
	})
	mux.Handle("/", next)
	return mux
}
func (s *Service) Handler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	s.mobileRoutes(mux)
	mux.HandleFunc("GET /api/v1/me", s.me)
	mux.HandleFunc("POST /api/v1/auth/logout", s.guard(s.logout))
	mux.HandleFunc("POST /api/v1/auth/logout-all", s.guard(s.logoutAll))
	for _, route := range []string{"login", "register", "confirm", "confirm/resend", "recover", "recover/end"} {
		mux.Handle("POST /api/v1/auth/"+route, s.emailHandler())
	}
	mux.HandleFunc("GET /api/v1/auth/google", s.googleStart)
	mux.HandleFunc("GET /api/v1/auth/google/callback", s.googleCallback)
	mux.HandleFunc("GET /api/v1/me/progress", s.listProgress)
	mux.HandleFunc("PUT /api/v1/me/progress/{imdbId}/{field}", s.guard(s.changeProgress))
	mux.HandleFunc("DELETE /api/v1/me/progress/{imdbId}/{field}", s.guard(s.changeProgress))
	mux.HandleFunc("POST /api/v1/me/progress/import", s.guard(s.importProgress))
	mux.HandleFunc("DELETE /api/v1/me", s.guard(s.deleteAccount))
	mux.HandleFunc("/api/v1/auth/", func(w http.ResponseWriter, _ *http.Request) {
		jsonError(w, 404, "unknown_action", "Unbekannte Kontoaktion.")
	})
	mux.Handle("/", next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/mobile/") || strings.HasPrefix(r.URL.Path, "/api/v1/mobile/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			mux.ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/api/v1/me") || strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			s.session.LoadAndSave(mux).ServeHTTP(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
func (s *Service) startSession(ctx context.Context, id string, version int64) error {
	if err := s.session.RenewToken(ctx); err != nil {
		return err
	}
	for _, key := range s.session.Keys(ctx) {
		s.session.Remove(ctx, key)
	}
	s.session.Put(ctx, "user_id", id)
	s.session.Put(ctx, "version", version)
	s.session.Put(ctx, "authenticated_at", time.Now().Unix())
	s.session.Put(ctx, "csrf", randomToken())
	return nil
}
func (s *Service) current(ctx context.Context) (*Profile, error) {
	if auth, ok := ctx.Value(mobileAuthKey{}).(mobileAuth); ok {
		return &auth.profile, nil
	}
	id := s.session.GetString(ctx, "user_id")
	if id == "" {
		return nil, errUnauthenticated
	}
	var profile Profile
	var version int64
	err := s.db.QueryRowContext(ctx, `SELECT u.id,COALESCE(c.email,i.email,''),u.session_version FROM users u
 LEFT JOIN user_credentials c ON c.user_id=u.id LEFT JOIN user_auth_identities i ON i.user_id=u.id
 WHERE u.id=$1 AND u.status='active'`, id).Scan(&profile.ID, &profile.Email, &version)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && version != s.session.GetInt64(ctx, "version")) {
		if e := s.session.Destroy(ctx); e != nil {
			return nil, e
		}
		return nil, errUnauthenticated
	}
	return &profile, err
}
func (s *Service) require(w http.ResponseWriter, r *http.Request) (*Profile, bool) {
	profile, err := s.current(r.Context())
	if errors.Is(err, errUnauthenticated) {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return nil, false
	}
	if err != nil {
		jsonError(w, 503, "account_unavailable", "Dein Konto konnte nicht geladen werden.")
		return nil, false
	}
	return profile, true
}
func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	profile, err := s.current(r.Context())
	if err != nil && !errors.Is(err, errUnauthenticated) {
		jsonError(w, 503, "account_unavailable", "Dein Konto konnte nicht geladen werden.")
		return
	}
	csrf := s.session.GetString(r.Context(), "csrf")
	if csrf == "" {
		csrf = randomToken()
		s.session.Put(r.Context(), "csrf", csrf)
	}
	writeJSON(w, 200, map[string]any{"enabled": true, "user": profile, "csrfToken": csrf, "providers": map[string]bool{"google": s.google != nil}, "developmentMailbox": "http://127.0.0.1:8025"})
}
func (s *Service) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if expected := r.Header.Get("X-Account-ID"); expected != "" && expected != s.session.GetString(r.Context(), "user_id") {
			jsonError(w, 409, "account_changed", "Das aktive Konto hat sich geändert. Lade die Seite erneut.")
			return
		}
		token := s.session.GetString(r.Context(), "csrf")
		if !s.origins[r.Header.Get("Origin")] || token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			jsonError(w, 403, "security_check_failed", "Die Sicherheitsprüfung ist fehlgeschlagen. Lade die Seite erneut.")
			return
		}
		next(w, r)
	}
}
func (s *Service) rateAllowed(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	s.limitMu.Lock()
	defer s.limitMu.Unlock()
	now := time.Now()
	window := s.limits[ip]
	if now.Sub(window.started) > 10*time.Minute {
		window = attemptWindow{started: now}
	}
	if window.count >= 30 {
		return false
	}
	if len(s.limits) >= 5000 {
		for key, value := range s.limits {
			if now.Sub(value.started) > 10*time.Minute {
				delete(s.limits, key)
			}
		}
		if len(s.limits) >= 5000 && window.count == 0 {
			return false
		}
	}
	window.count++
	s.limits[ip] = window
	return true
}
func (s *Service) emailHandler() http.Handler {
	handler := s.boss.LoadClientStateMiddleware(http.StripPrefix("/api/v1/auth", s.boss.Config.Core.Router))
	return s.guard(func(w http.ResponseWriter, r *http.Request) {
		if !s.rateAllowed(r) {
			w.Header().Set("Retry-After", "600")
			jsonError(w, 429, "too_many_attempts", "Zu viele Versuche. Bitte warte einige Minuten.")
			return
		}
		if s.session.GetString(r.Context(), "user_id") != "" {
			jsonError(w, 409, "logout_required", "Melde dich zuerst ab, um diese Kontoaktion auszuführen.")
			return
		}
		var body map[string]string
		if err := decodeBody(w, r, 16384, &body); err != nil {
			jsonError(w, 400, "invalid_account", "Ungültige Kontodaten.")
			return
		}
		if raw, ok := body["email"]; ok {
			normalized := strings.ToLower(strings.TrimSpace(raw))
			address, err := mail.ParseAddress(normalized)
			if err != nil || address.Address != normalized || len(normalized) > 320 {
				jsonError(w, 400, "invalid_email", "Bitte gib eine gültige E-Mail-Adresse ein.")
				return
			}
			body["email"] = normalized
		}
		if len(body["password"]) > 72 {
			jsonError(w, 400, "password_too_long", "Das Passwort darf höchstens 72 UTF-8-Bytes lang sein.")
			return
		}
		if (r.URL.Path == "/api/v1/auth/register" || r.URL.Path == "/api/v1/auth/recover/end") && utf8.RuneCountInString(body["password"]) < 12 {
			jsonError(w, 400, "password_too_short", "Das Passwort muss mindestens 12 Zeichen lang sein.")
			return
		}
		payload, _ := json.Marshal(body)
		r.Body = io.NopCloser(bytes.NewReader(payload))
		r.Header.Set("Content-Type", "application/json")
		tx, err := s.db.BeginTx(r.Context(), nil)
		if err != nil {
			jsonError(w, 503, "action_unavailable", "Die Kontoaktion ist vorübergehend nicht verfügbar.")
			return
		}
		defer tx.Rollback()
		delivery := new(deliveryResult)
		ctx := context.WithValue(r.Context(), transactionKey{}, tx)
		ctx = context.WithValue(ctx, deliveryKey{}, delivery)
		buffer := &bufferedResponse{headers: make(http.Header), status: 200}
		handler.ServeHTTP(buffer, r.WithContext(ctx))
		if delivery.err != nil || buffer.status >= 500 {
			_ = s.session.Destroy(r.Context())
			jsonError(w, 503, "action_failed", "Die Kontoaktion konnte nicht abgeschlossen werden.")
			return
		}
		if err = tx.Commit(); err != nil {
			_ = s.session.Destroy(r.Context())
			jsonError(w, 503, "action_save_failed", "Die Kontoaktion konnte nicht gespeichert werden.")
			return
		}
		for key, values := range buffer.headers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(buffer.status)
		_, _ = w.Write(buffer.body.Bytes())
	})
}

type bufferedResponse struct {
	headers http.Header
	status  int
	body    bytes.Buffer
	written bool
}

func (b *bufferedResponse) Header() http.Header { return b.headers }
func (b *bufferedResponse) WriteHeader(status int) {
	if !b.written {
		b.status = status
		b.written = true
	}
}
func (b *bufferedResponse) Write(data []byte) (int, error) {
	b.written = true
	return b.body.Write(data)
}

type quietLogger struct{}

func (quietLogger) Info(string)  {}
func (quietLogger) Error(string) {}

type safeErrors struct{}

func (safeErrors) Wrap(handler func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			jsonError(w, 500, "action_failed", "Die Kontoaktion konnte nicht abgeschlossen werden.")
		}
	})
}
func decodeBody(w http.ResponseWriter, r *http.Request, limit int64, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("unexpected data")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func jsonError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.session.Destroy(r.Context()); err != nil {
		jsonError(w, 503, "logout_failed", "Abmelden fehlgeschlagen.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Service) logoutAll(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.require(w, r)
	if !ok {
		return
	}
	if _, err := s.db.ExecContext(r.Context(), "UPDATE users SET session_version=session_version+1,updated_at=now() WHERE id=$1", profile.ID); err != nil {
		jsonError(w, 503, "logout_failed", "Abmelden fehlgeschlagen.")
		return
	}
	if err := s.session.Destroy(r.Context()); err != nil {
		jsonError(w, 503, "logout_failed", "Abmelden fehlgeschlagen.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *Service) deleteAccount(w http.ResponseWriter, r *http.Request) {
	profile, ok := s.require(w, r)
	if !ok {
		return
	}
	if time.Now().Unix()-s.session.GetInt64(r.Context(), "authenticated_at") > 600 {
		jsonError(w, 403, "recent_login_required", "Melde dich vor dem Löschen erneut an.")
		return
	}
	result, err := s.db.ExecContext(r.Context(), "DELETE FROM users WHERE id=$1 AND session_version=$2 AND status='active'", profile.ID, s.session.GetInt64(r.Context(), "version"))
	if err != nil {
		jsonError(w, 503, "delete_failed", "Das Konto konnte nicht gelöscht werden.")
		return
	}
	count, err := result.RowsAffected()
	if err != nil {
		jsonError(w, 503, "delete_failed", "Das Konto konnte nicht gelöscht werden.")
		return
	}
	if count == 0 {
		jsonError(w, 401, "authentication_required", "Bitte melde dich erneut an.")
		return
	}
	if err := s.session.Destroy(r.Context()); err != nil {
		jsonError(w, 503, "session_cleanup_failed", "Das Konto wurde gelöscht, aber die Sitzung konnte nicht entfernt werden.")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
