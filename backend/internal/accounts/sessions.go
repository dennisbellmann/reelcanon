package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/alexedwards/scs/v2"
)

type sessionStore struct {
	db      *sql.DB
	manager *scs.SessionManager
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func (s *sessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT data FROM sessions WHERE token=$1 AND expiry>now()", tokenHash(token)).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return data, err == nil, err
}
func (s *sessionStore) DeleteCtx(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token=$1", tokenHash(token))
	return err
}
func (s *sessionStore) CommitCtx(ctx context.Context, token string, data []byte, expiry time.Time) error {
	// SCS calls Commit while holding its session mutex. Decode the supplied data;
	// calling GetString here would acquire that mutex again and deadlock.
	_, values, err := s.manager.Codec.Decode(data)
	if err != nil {
		return err
	}
	id, _ := values["user_id"].(string)
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions(token,data,expiry,user_id) VALUES($1,$2,$3,NULLIF($4,'')::uuid)
 ON CONFLICT(token) DO UPDATE SET data=excluded.data,expiry=excluded.expiry,user_id=excluded.user_id`, tokenHash(token), data, expiry, id)
	return err
}
func (s *sessionStore) Find(t string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), t)
}
func (s *sessionStore) Delete(t string) error { return s.DeleteCtx(context.Background(), t) }
func (s *sessionStore) Commit(t string, b []byte, e time.Time) error {
	return s.CommitCtx(context.Background(), t, b, e)
}

type clientState struct {
	ctx     context.Context
	manager *scs.SessionManager
}

func (s clientState) Get(key string) (string, bool) {
	key = "ab:" + key
	return s.manager.GetString(s.ctx, key), s.manager.Exists(s.ctx, key)
}

type clientStorage struct{ manager *scs.SessionManager }

func (s clientStorage) ReadState(r *http.Request) (authboss.ClientState, error) {
	return clientState{r.Context(), s.manager}, nil
}
func (s clientStorage) WriteState(_ http.ResponseWriter, state authboss.ClientState, events []authboss.ClientStateEvent) error {
	current, ok := state.(clientState)
	if !ok {
		return errors.New("missing session context")
	}
	for _, event := range events {
		key := "ab:" + event.Key
		switch event.Kind {
		case authboss.ClientStateEventPut:
			s.manager.Put(current.ctx, key, event.Value)
		case authboss.ClientStateEventDel:
			s.manager.Remove(current.ctx, key)
		case authboss.ClientStateEventDelAll:
			whitelist := strings.Split(event.Key, ",")
			for _, existing := range s.manager.Keys(current.ctx) {
				if !strings.HasPrefix(existing, "ab:") {
					continue
				}
				keep := false
				for _, allowed := range whitelist {
					if existing == "ab:"+allowed {
						keep = true
					}
				}
				if !keep {
					s.manager.Remove(current.ctx, existing)
				}
			}
		}
	}
	return nil
}
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
