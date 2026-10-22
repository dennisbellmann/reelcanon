package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/google/uuid"
)

// FileMail deliberately requires a loopback public URL. It is a development
// transport, not a production sender. Confirmation itself is never bypassed.
type FileMail struct {
	Directory string
	PublicURL string
}
type CapturedMail struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	To        []string  `json:"to"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
}

func (m FileMail) Load(...string) error { return nil }
func (m FileMail) Render(_ context.Context, page string, data authboss.HTMLData) ([]byte, string, error) {
	raw, key := "", "confirm"
	if value, ok := data["url"].(string); ok {
		raw = value
	}
	if value, ok := data["confirm_url"].(string); ok {
		raw = value
	}
	if value, ok := data["recover_url"].(string); ok {
		raw = value
		key = "reset"
	}
	parsed, err := url.Parse(raw)
	if err != nil || raw == "" {
		return nil, "", errors.New("missing mail link")
	}
	token := parsed.Query().Get("cnf")
	if key == "reset" {
		token = parsed.Query().Get("token")
	}
	if token == "" {
		return nil, "", errors.New("missing mail token")
	}
	link := m.PublicURL + "/account#" + url.Values{key: []string{token}}.Encode()
	body := fmt.Sprintf("Reel Canon\n\nÖffne diesen Link und bestätige die Aktion:\n%s\n\nFalls du diese Aktion nicht angefordert hast, ignoriere diese Nachricht.", link)
	if strings.HasSuffix(page, "html") {
		return []byte("<p>" + strings.ReplaceAll(html.EscapeString(body), "\n", "<br>") + "</p>"), "text/html", nil
	}
	return []byte(body), "text/plain", nil
}
func (m FileMail) Send(ctx context.Context, email authboss.Email) (err error) {
	defer func() {
		if result, ok := ctx.Value(deliveryKey{}).(*deliveryResult); ok {
			result.err = err
		}
	}()
	if err = os.MkdirAll(m.Directory, 0700); err != nil {
		return err
	}
	message := CapturedMail{uuid.NewString(), time.Now().UTC(), email.To, email.Subject, email.TextBody}
	if strings.Contains(email.TextBody, "#confirm=") {
		message.Subject = "Reel Canon: E-Mail-Adresse bestätigen"
	}
	if strings.Contains(email.TextBody, "#reset=") {
		message.Subject = "Reel Canon: Passwort zurücksetzen"
	}
	data, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return err
	}
	// Create exclusively; the mailbox ignores any file that is not a complete JSON document.
	file, err := os.OpenFile(filepath.Join(m.Directory, message.ID+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	return errors.Join(writeErr, closeErr)
}
