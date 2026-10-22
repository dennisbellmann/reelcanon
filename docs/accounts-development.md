# Konten lokal ohne Mailserver entwickeln

Stand: 2026-10-06. Die erste Web-/Backend-Implementierung ist vorhanden.
Es werden keine echten E-Mails verschickt. Ein lokaler Mailfänger speichert
Bestätigungs- und Reset-Mails in `.local/mail/` und zeigt sie auf Port 8025.

## Start

Voraussetzung: der bestehende lokale PostgreSQL-Katalog und eine konfigurierte
`.env`. Vom Repository aus:

```sh
make accounts-local
make build
make api
make frontend-dev
make dev-mail
```

Die drei Serverbefehle laufen in eigenen Terminals. `make accounts-local`
wendet Migrationen an, erstellt die lokale Rolle `reelcanon_accounts` mit
zufälligem Passwort und ergänzt die ignorierte `.env`. Es funktioniert nur mit
einer Loopback-Datenbank. Bestehende Passwörter werden nicht geändert.
Der Katalogrolle werden Rechte auf persönliche Tabellen ausdrücklich entzogen.

Im vorhandenen Windows-Workspace alternativ:

```powershell
./scripts/stop-local.ps1
./scripts/start-local.ps1 -Rebuild
```

Anschließend: [Konto](http://127.0.0.1:8080/account),
[Entwicklungs-Postfach](http://127.0.0.1:8025).
`AUTH_PUBLIC_URL` bestimmt den Ursprung der E-Mail- und Google-Links.
Bei Vite auf 5173 wird die API über dessen bestehenden `/api`-Proxy angesprochen.
`AUTH_ALLOWED_ORIGINS` enthält zusätzlich erlaubte lokale Web-Ursprünge.
Hostname und Port müssen zum geöffneten Browser passen.

## Vorhandener Funktionsumfang

- E-Mail/Passwort: Registrierung, Bestätigung, erneuter Bestätigungslink, Login,
  zeitweilige Sperre nach Fehlversuchen, Passwort-Reset.
- Authboss 3.5.3 übernimmt Passwort-Hashing und Bestätigungs-/Reset-Tokens.
  Passwörter: mindestens 12 Zeichen, maximal 72 UTF-8-Bytes, bcrypt-Kostenfaktor 12.
- SCS 2.9.0 verwaltet PostgreSQL-Sitzungen. Cookie: HttpOnly, SameSite=Lax;
  Secure bei HTTPS und dauerhaft über Browser-Neustarts. Laufzeit und erlaubte
  Inaktivität betragen jeweils maximal 30 Tage.
  Cookies enthalten keine persönlichen Daten; Tokens werden vor Speicherung gehasht.
- Nutzerstatus und Session-Version werden bei persönlichen Zugriffen geprüft.
  Passwort-Reset widerruft alle Sitzungen. Abmelden, auf allen Geräten abmelden
  und Kontolöschung nach frischer Anmeldung sind vorhanden.
- Gesehen und Merkliste gelten je Nutzer/Film über alle Canons hinweg.
  Änderungen benötigen eine Verbindung zum Server. Gast-Dexie bleibt separat.
- Gastliste nur per Button übernehmen; Sicherungen exportieren/importieren.
  Bestehende Kontozeilen einschließlich entfernter Häkchen gewinnen beim Import.
  Unbekannte Filme werden gemeldet und nicht in den Katalog eingefügt.
- Web aktualisiert Kontostatus bei Fokus/alle 60 Sekunden und Kontofortschritt
  bei Fokus/alle 30 Sekunden. Konflikte werden angezeigt; keine stillen Offline-Writes.

## Google-Konfiguration

Der vorhandene Web-Login verwendet den serverseitigen Google-OIDC-Flow und
Browser-Cookies. Seine Konfiguration bleibt über `GOOGLE_CLIENT_ID`,
`GOOGLE_CLIENT_SECRET` und `AUTH_STATE_KEY` (32 Zufallsbytes als Standard-Base64)
in der ignorierten `.env`; `make accounts-local` erzeugt den lokalen State-Schlüssel.
Redirect-URI in Google: `AUTH_PUBLIC_URL/api/v1/auth/google/callback`.
Android fordert denselben serverseitigen Audience-Client an und benötigt zusätzlich
die getrennte Android-OAuth-Client-ID in `GOOGLE_MOBILE_CLIENT_IDS` für die Prüfung
des autorisierten Party-Claims. Der tatsächliche Google-Cloud-Client ist in diesem
Arbeitsschritt nicht verifiziert.

Authorization Code Flow mit PKCE S256, State, Nonce, zehn Minuten Gültigkeit,
Sitzungsbindung und atomarem Verbrauch des Login-Auftrags. PKCE-Verifier ist
mit AES-GCM verschlüsselt. go-oidc 3.16.0 prüft Signatur, Issuer, Audience und
Ablaufzeit. Konten werden über Google-Subject zugeordnet und nicht anhand gleicher
E-Mail-Adressen automatisch zusammengeführt. Zugangsdaten werden nicht ins
Frontend gegeben. Der Web-Login bleibt unverändert; mobile Token-Sessions teilen
die serverseitige Google-Subject-Zuordnung, aber nicht Browser-Cookies.

## Datenbank und API

Aktives Schema: `database/migrations/0007_mobile_sessions.sql` (nach
`0005_accounts.sql` und `0006_collection_presentation.sql`). Die aktuelle Migration
ergänzt mobile Challenges/Sessions/Refresh-Rotation, Idempotenzbelege und
Feldrevisionen. `database/proposals/accounts-and-progress.sql` bleibt der
weitergehende Entwurf für Apple und Kauf-Widerrufsjobs. `database/roles/accounts.sql`
beschreibt die Rechte der Account-Verbindung. `USER_DATABASE_URL` und
`API_DATABASE_URL` bleiben getrennt; der Account-Pool kann den Katalog nicht ändern.
Admin-Zugriff bleibt separat.

Der tatsächliche HTTP-Vertrag steht in
[accounts-api.md](../contracts/accounts-api.md).

## Grenzen dieser Entwicklungsstufe

Der File-Mailer erlaubt ausschließlich einen Loopback-Ursprung und ist kein
produktiver Versandweg. Vor öffentlichem Betrieb folgt ein SMTP-Adapter mit
Domain, TLS und Zustellbarkeitsprüfung. SMTP ist austauschbar über die
Authboss-Mailer-Schnittstelle; E-Mail-Bestätigung wird auch lokal nie übersprungen.

Android ergänzt Credential-Manager-Anmeldung, Keystore-geschützte Tokenablage,
Refresh-Rotation sowie eine dauerhafte, revisions- und idempotenzbewusste Offline-
Outbox. Das mobile HTTP-Verhalten und die Einrichtung stehen in
[mobile-api.md](../contracts/mobile-api.md). `GOOGLE_MOBILE_CLIENT_IDS` muss die
Android-OAuth-Client-ID als zulässigen `azp` enthalten; die Google-Cloud-Konfiguration
ist noch nicht verifiziert. iOS, Apple-Anmeldung, Verknüpfen mehrerer Zugänge und
Käufe bleiben zukünftige Arbeit. Die Auth-Oberfläche setzt keine Admin- oder
Kaufrechte.

Die Backend-Integrationstests prüfen Web-/Mobile-Kontoidentität, Revisionen,
Idempotenz, Tokenrotation und Widerruf mit einer lokalen OIDC-Testinstanz.
Android-Host-, Emulator-, Lint-, Frontend- und Web-E2E-Prüfungen sind in der
jeweiligen Projektanleitung dokumentiert. Ein echter Google-Cloud-OAuth-Client
wurde noch nicht verifiziert.
