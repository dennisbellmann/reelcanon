package accounts

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/reelcanon/reelcanon/backend/internal/testdb"
	"golang.org/x/oauth2"
)

type mobileFixture struct {
	s           *Service
	db          *sql.DB
	handler     http.Handler
	signer      jose.Signer
	googleToken string
}

func newMobileFixture(t *testing.T) *mobileFixture {
	t.Helper()
	db := testdb.Open(t)
	t.Setenv("AUTH_PUBLIC_URL", "http://127.0.0.1:8080")
	t.Setenv("AUTH_ALLOWED_ORIGINS", "")
	t.Setenv("AUTH_MAIL_MODE", "file")
	t.Setenv("AUTH_MAIL_DIR", t.TempDir())
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_MOBILE_CLIENT_IDS", "android-client")
	s, err := New(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
	if err != nil {
		t.Fatal(err)
	}
	f := &mobileFixture{s: s, db: db, signer: signer}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/keys" {
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
		} else {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "provider-access", "token_type": "Bearer", "id_token": f.googleToken})
		}
	}))
	t.Cleanup(provider.Close)
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	s.google = &googleLogin{verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(context.Background(), provider.URL+"/keys"), &oidc.Config{ClientID: "web-client"}), config: oauth2.Config{ClientID: "web-client", ClientSecret: "server-only", RedirectURL: s.publicURL + "/api/v1/auth/google/callback", Endpoint: oauth2.Endpoint{AuthURL: "https://google.test/auth", TokenURL: provider.URL + "/token"}}, cipher: aead}
	f.handler = s.Handler(http.NotFoundHandler())
	if _, err = db.Exec("INSERT INTO movies(imdb_id,title) VALUES('tt0068646','Test')"); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *mobileFixture) request(method, path, body, access string, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080/api/v1"+path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if access != "" {
		r.Header.Set("Authorization", "Bearer "+access)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Fatalf("status %d, expected %d: %s", w.Code, expected, w.Body.String())
	}
}
func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal("invalid response")
	}
	return result
}
func (f *mobileFixture) token(t *testing.T, subject, nonce string, overrides map[string]any) string {
	t.Helper()
	claims := map[string]any{"iss": "https://accounts.google.com", "aud": "web-client", "azp": "android-client", "sub": subject, "email": "same@example.test", "email_verified": true, "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	for k, v := range overrides {
		claims[k] = v
	}
	payload, _ := json.Marshal(claims)
	signed, err := f.signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func (f *mobileFixture) login(t *testing.T, subject string) mobileTokens {
	t.Helper()
	w := f.request("POST", "/auth/mobile/google/challenge", "{}", "", nil, nil)
	status(t, w, 200)
	challenge := decode[map[string]any](t, w)
	raw := f.token(t, subject, challenge["nonce"].(string), nil)
	body, _ := json.Marshal(map[string]any{"challengeId": challenge["challengeId"], "idToken": raw})
	w = f.request("POST", "/auth/mobile/google", string(body), "", nil, nil)
	status(t, w, 200)
	if len(w.Result().Cookies()) > 0 {
		t.Fatal("mobile login issued a browser cookie")
	}
	return decode[mobileTokens](t, w)
}
func (f *mobileFixture) write(t *testing.T, token, field string, value bool, revision int, id string) *httptest.ResponseRecorder {
	t.Helper()
	return f.request("PUT", "/mobile/me/progress/tt0068646/"+field, `{"value":`+strconv.FormatBool(value)+`}`, token, map[string]string{"If-Match": strconv.Quote(strconv.Itoa(revision)), "Idempotency-Key": id}, nil)
}
func TestWebAndMobileShareGoogleIdentityAndProgress(t *testing.T) {
	f := newMobileFixture(t)
	start := f.request("GET", "/auth/google", "", "", nil, nil)
	status(t, start, 303)
	redirect, _ := url.Parse(start.Header().Get("Location"))
	f.googleToken = f.token(t, "same-subject", redirect.Query().Get("nonce"), nil)
	callback := f.request("GET", "/auth/google/callback?state="+url.QueryEscape(redirect.Query().Get("state"))+"&code=test", "", "", nil, start.Result().Cookies())
	status(t, callback, 303)
	if strings.Contains(callback.Header().Get("Location"), "login-error") {
		t.Fatal("web OAuth failed")
	}
	cookies := callback.Result().Cookies()
	w := f.request("GET", "/me", "", "", nil, cookies)
	status(t, w, 200)
	var bootstrap struct {
		User Profile `json:"user"`
		CSRF string  `json:"csrfToken"`
	}
	bootstrap = decode[struct {
		User Profile `json:"user"`
		CSRF string  `json:"csrfToken"`
	}](t, w)
	mobile := f.login(t, "same-subject")
	if mobile.User.ID != bootstrap.User.ID {
		t.Fatal("web and mobile identities differ")
	}
	headers := map[string]string{"Origin": f.s.publicURL, "X-CSRF-Token": bootstrap.CSRF, "If-Match": "\"0\""}
	status(t, f.request("PUT", "/me/progress/tt0068646/watched", `{"value":true}`, "", headers, cookies), 200)
	remote := decode[[]Progress](t, f.request("GET", "/mobile/me/progress", "", mobile.AccessToken, nil, nil))
	if len(remote) != 1 || !remote[0].Watched {
		t.Fatal("web status missing on mobile")
	}
	status(t, f.write(t, mobile.AccessToken, "watchlist", true, 1, uuid.NewString()), 200)
	web := decode[[]Progress](t, f.request("GET", "/me/progress", "", "", nil, cookies))
	if !web[0].Watchlist || !web[0].Watched {
		t.Fatal("mobile status missing on web")
	}
	// Cookies cannot authorize mobile, and Bearer cannot bypass web CSRF.
	status(t, f.request("GET", "/mobile/me/progress", "", "", nil, cookies), 401)
	status(t, f.request("PUT", "/me/progress/tt0068646/watched", `{"value":false}`, mobile.AccessToken, headers, nil), 403)
	other := f.login(t, "different-subject")
	if other.User.ID == mobile.User.ID {
		t.Fatal("accounts merged by email")
	}
	rows := decode[[]Progress](t, f.request("GET", "/mobile/me/progress", "", other.AccessToken, nil, nil))
	if len(rows) != 0 {
		t.Fatal("account progress leaked")
	}
}
func TestMobileIdempotencyConflictsAndTombstones(t *testing.T) {
	f := newMobileFixture(t)
	tokens := f.login(t, "subject")
	id := uuid.NewString()
	w := f.write(t, tokens.AccessToken, "watched", true, 0, id)
	status(t, w, 200)
	original := w.Body.String()
	status(t, f.write(t, tokens.AccessToken, "watchlist", true, 1, uuid.NewString()), 200)
	repeat := f.write(t, tokens.AccessToken, "watched", true, 0, id)
	status(t, repeat, 200)
	if repeat.Body.String() != original {
		t.Fatal("receipt changed")
	}
	status(t, f.write(t, tokens.AccessToken, "watched", false, 0, id), 409)
	conflict := f.write(t, tokens.AccessToken, "watched", false, 1, uuid.NewString())
	status(t, conflict, 412)
	current := decode[struct {
		Current Progress `json:"current"`
	}](t, conflict).Current
	if current.WatchedRevision != 1 || current.WatchlistRevision != 2 {
		t.Fatal("field revisions lost")
	}
	status(t, f.write(t, tokens.AccessToken, "watched", false, 2, uuid.NewString()), 200)
	status(t, f.write(t, tokens.AccessToken, "watchlist", false, 3, uuid.NewString()), 200)
	rows := decode[[]Progress](t, f.request("GET", "/mobile/me/progress", "", tokens.AccessToken, nil, nil))
	if len(rows) != 1 || rows[0].Revision != 4 || rows[0].Watched || rows[0].Watchlist {
		t.Fatal("tombstone lost")
	}
	// No-op creates a receipt but leaves the revision unchanged.
	noOp := f.write(t, tokens.AccessToken, "watched", false, 4, uuid.NewString())
	status(t, noOp, 200)
	if decode[Progress](t, noOp).Revision != 4 {
		t.Fatal("no-op incremented revision")
	}
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- f.write(t, tokens.AccessToken, "watched", true, 4, uuid.NewString()).Code
		}()
	}
	wg.Wait()
	close(codes)
	count := map[int]int{}
	for code := range codes {
		count[code]++
	}
	if count[200] != 1 || count[412] != 1 {
		t.Fatal("concurrent writes not serialized", count)
	}
}
func TestMobileRefreshReuseLogoutAndAccountRevocation(t *testing.T) {
	f := newMobileFixture(t)
	tokens := f.login(t, "subject")
	refresh := func(token string) *httptest.ResponseRecorder {
		return f.request("POST", "/auth/mobile/refresh", `{"refreshToken":"`+token+`"}`, "", nil, nil)
	}
	rotated := refresh(tokens.RefreshToken)
	status(t, rotated, 200)
	next := decode[mobileTokens](t, rotated)
	status(t, f.request("GET", "/mobile/me", "", tokens.AccessToken, nil, nil), 401)
	status(t, f.request("GET", "/mobile/me", "", next.AccessToken, nil, nil), 200)
	status(t, refresh(tokens.RefreshToken), 401)
	status(t, f.request("GET", "/mobile/me", "", next.AccessToken, nil, nil), 401)
	status(t, refresh(next.RefreshToken), 401)
	tokens = f.login(t, "subject")
	if _, err := f.db.Exec("UPDATE users SET session_version=session_version+1 WHERE id=$1", tokens.User.ID); err != nil {
		t.Fatal(err)
	}
	status(t, refresh(tokens.RefreshToken), 401)
	status(t, f.request("GET", "/mobile/me", "", tokens.AccessToken, nil, nil), 401)
	tokens = f.login(t, "subject")
	body := `{"refreshToken":"` + tokens.RefreshToken + `"}`
	status(t, f.request("POST", "/auth/mobile/logout", body, "", nil, nil), 200)
	status(t, f.request("POST", "/auth/mobile/logout", body, "", nil, nil), 200)
	status(t, f.request("POST", "/auth/mobile/logout", `{"refreshToken":"`+strings.Repeat("x", 43)+`"}`, "", nil, nil), 200)
	status(t, f.request("GET", "/mobile/me", "", tokens.AccessToken, nil, nil), 401)
	tokens = f.login(t, "subject")
	if _, err := f.db.Exec("UPDATE users SET status='blocked' WHERE id=$1", tokens.User.ID); err != nil {
		t.Fatal(err)
	}
	status(t, f.request("GET", "/mobile/me", "", tokens.AccessToken, nil, nil), 401)
	tx, _ := f.db.Begin()
	defer tx.Rollback()
	if _, _, err := googleAccount(context.Background(), tx, "subject", "same@example.test"); err == nil {
		t.Fatal("blocked subject recreated")
	}
}
func TestMobileGoogleRejectsInvalidSignedClaimsAndNonceReplay(t *testing.T) {
	f := newMobileFixture(t)
	for name, overrides := range map[string]map[string]any{"audience": {"aud": "other"}, "party": {"azp": "other"}, "expired": {"exp": time.Now().Add(-time.Hour).Unix()}, "issuer": {"iss": "https://evil.test"}, "nonce": {"nonce": "wrong"}, "subject": {"sub": ""}, "unverified": {"email_verified": false}} {
		t.Run(name, func(t *testing.T) {
			challenge := decode[map[string]any](t, f.request("POST", "/auth/mobile/google/challenge", "{}", "", nil, nil))
			raw := f.token(t, "subject", challenge["nonce"].(string), overrides)
			body, _ := json.Marshal(map[string]any{"challengeId": challenge["challengeId"], "idToken": raw})
			status(t, f.request("POST", "/auth/mobile/google", string(body), "", nil, nil), 401)
		})
	}
	challenge := decode[map[string]any](t, f.request("POST", "/auth/mobile/google/challenge", "{}", "", nil, nil))
	raw := f.token(t, "subject", challenge["nonce"].(string), nil)
	body, _ := json.Marshal(map[string]any{"challengeId": challenge["challengeId"], "idToken": raw})
	status(t, f.request("POST", "/auth/mobile/google", string(body), "", nil, nil), 200)
	status(t, f.request("POST", "/auth/mobile/google", string(body), "", nil, nil), 401)
}
