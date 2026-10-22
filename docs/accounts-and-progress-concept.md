# Konten und serverseitiger Filmfortschritt

Stand: 2026-10-06. **Konzept mit erster lokaler Web-/Backend-Implementierung.**

Aktueller Umsetzungsstand und tatsächliche Abweichungen vom weitergehenden Entwurf:
[Konten lokal entwickeln](accounts-development.md),
[HTTP-Vertrag](../contracts/accounts-api.md).
Migration 0005 ist aktiv; der Entwurf unten umfasst zusätzlich spätere Schritte.
Der File-Mailer erlaubt Registrierung/Bestätigung/Reset ohne externen Mailserver.
Google bleibt ohne OAuth-Zugangsdaten deaktiviert. Produktions-SMTP, Apple und
Zugangsverknüpfung folgen später.

## Festgelegter Umfang

Kostenlose Code-Bibliotheken im bestehenden Go-Backend. Zunächst **E-Mail/Passwort
und Google**; Apple folgt mit vorhandenem Developer-Account und bleibt bis dahin
deaktiviert. Persönlicher Fortschritt liegt in unserer PostgreSQL-Datenbank.
Ohne Konto bleibt die bisherige lokale Gast-Nutzung möglich.

Öffentliche Domain, SMTP-Zugang und Google-OAuth-Client existieren noch nicht.
Die spätere Integration erhält Backend-Umgebungsvariablen für diese Konfiguration.
Native Apps und Listenverkäufe sind zunächst ausschließlich geplant:
[Umsetzungsplan](mobile-app-implementation-plan.md).

## Bibliotheken

| Bibliothek | Aufgabe |
| --- | --- |
| [Authboss v3](https://github.com/aarondl/authboss) | E-Mail/Passwort, Registrierung, Bestätigung, Passwort-Reset und Login-Sperren |
| [go-oidc](https://github.com/coreos/go-oidc) und [oauth2](https://pkg.go.dev/golang.org/x/oauth2) | Google-Code-Flow und geprüfte ID-Tokens |
| [SCS](https://github.com/alexedwards/scs) mit PostgreSQL Store | Serverseitige Sessions, Cookies, Token-Wechsel und Ablauf |
| [Goth](https://github.com/markbates/goth) | Optionaler vorhandener Provider-Adapter für Apple später |

Keine Auth-Dienstgebühr. Authboss und Goth sind MIT-lizenziert. Storage-/Session-
Adapter, Formulare und SMTP werden an unser Backend angebunden; Passwort-Hashing
und Tokenprüfung liefern Bibliotheken. Authboss verwendet auth/register/confirm/
recover/lock und denselben Session-Speicher wie Google. Goth ist für die erste
Google-Integration nicht erforderlich. Für Apple werden im gewählten Release
State/Nonce, Callback und vollständige Tokenprüfung überprüft; ein UserID aus
einem Provider-Helper allein genügt nicht. Versionen vor Umsetzung fixieren.

## Schema

[SQL-Entwurf](../database/proposals/accounts-and-progress.sql), aufbauend auf den
vorhandenen Migrationen 0001–0004. Er liegt außerhalb des aktiven Migrationsordners
und wird von make migrate nicht angewendet.

| Tabelle | Zweck |
| --- | --- |
| users | Interne UUID, Anzeigename, Status, Session-Version |
| user_credentials | Optionaler E-Mail-/Passwort-Zugang; Authboss-Bestätigung, Reset und Sperren |
| user_auth_identities | Google-/Apple-Zuordnung über Provider, Issuer und Subject |
| sessions | Serverseitige Sessions mit optionaler Nutzer-Zuordnung |
| auth_login_requests | Kurzlebige, einmalige Social-Login-Anfragen |
| user_movie_progress | Gesehen/Watchlist je Nutzer und Film, mit Revision |
| oauth_token_revocation_jobs | Späterer Apple-Widerruf, der Kontolöschung überlebt |

Fortschritt referenziert movies.id, Primärschlüssel (user_id, movie_id). API und
Backups verwenden IMDb-ID. Keine Bindung an Canon, Rang oder Edition: Ein Film
bleibt auch in einer späteren 1001-Movies-Liste abgehakt. watched_at ist bei
gesehenen Filmen gesetzt, ansonsten NULL, passend zum vorhandenen Backupformat.
Uncheck erhält die Watchlist. Leere Zustandszeilen bleiben mit Revision erhalten,
damit alte Geräte entfernte Häkchen nicht unbemerkt wiederherstellen.

## Login und Sessions

**E-Mail:** Authboss erzeugt Passwort-Hashes und Confirm-/Recover-Selector mit
gehashtem Verifier. Der Adapter erfüllt dessen User-/Storer-Schnittstellen.
Bestätigung und Reset haben Ablaufzeiten; erfolgreiche Tokens atomar verbrauchen.
E-Mail nach einer festen Policy normalisieren, ohne providerabhängige Punkt-/
Plus-Regeln. Reset-Antworten verraten keine registrierten Adressen. Login,
Registrierung und Mailversand begrenzen. Reset, Passwortänderung und Sperre erhöhen
session_version und widerrufen alte App-Sessions über Integrations-Hooks.

**Google:** Authorization Code Flow mit PKCE S256, zufälligem state/nonce,
zehn Minuten Gültigkeit und Browser-Bindung. Login-Anfrage atomar einmalig verbrauchen,
Code tauschen und ID-Token per Bibliothek gegen feste Signaturschlüssel, Issuer,
Client-ID und Ablaufzeit prüfen; zusätzlich nonce vergleichen. Zuordnung über
Issuer/Subject. Keine Google-Refresh-Tokens für reinen Login speichern.
[Google OIDC](https://developers.google.com/identity/openid-connect/openid-connect).

**Verknüpfen:** Gleiche E-Mail-Adressen führen nicht zum automatischen Zusammenlegen
von Konten. Ein weiterer Zugang verlangt eine frische Anmeldung im vorhandenen
Konto und den Nachweis des neuen Zugangs. Bereits fremd zugeordnete Identität: 409.
Der letzte nutzbare Zugang darf nur zusammen mit dem Konto entfernt werden.

SCS erneuert die Session-ID beim Login. Produktiv: Secure, HttpOnly, SameSite=Lax,
Path=/, kein Domain-Attribut; __Host-reelcanon_session. Die Sitzung bleibt bis zu
30 Tage inaktiv und maximal 30 Tage insgesamt gültig. Nutzer-ID, Version,
Authentifizierungszeit und CSRF-
Zustand serverseitig speichern. Keine Provider-Tokens im Browser-Speicher.
Persönliche Requests prüfen Nutzerstatus/Session-Version. Writes verlangen CSRF
und erlaubten Origin; private Antworten verwenden no-store.

Der SQL-Entwurf ergänzt die Store-Spalten token/data/expiry um user_id. Ein Adapter
setzt diese Zuordnung bei neuen authentifizierten Sessions vor Cookie-Ausgabe;
Fehler brechen den Login ab. Eine kleine Store-Hülle hasht Session-Tokens vor dem
PostgreSQL Store; dieselben Hashes werden für die Zuordnung genutzt. Sie muss den
vollständigen Store-Vertrag abdecken. Kein eigenes Session- oder JWT-Format erfinden.

**Apple später:** form_post braucht eine zusätzliche kurze Secure/HttpOnly-Cookie-
Bindung mit SameSite=None, da die normale Lax-Session beim POST fehlt. Client-
Secret serverseitig signieren; Refresh-Token verschlüsselt für Widerruf speichern.
Signierte Serverbenachrichtigungen anbinden. Goth übernimmt den gesamten Lifecycle
nicht automatisch. Reguläre Developer-Mitgliedschaft: 99 USD/Jahr bzw. lokaler Preis;
[Apple](https://developer.apple.com/programs/enroll/). Ein bestehender Zugang genügt.

## API, Geräte und Datenübernahme

Ursprünglich vorgeschlagene Endpunkte; den implementierten Vertrag und dessen
abweichende Pfade beschreibt [accounts-api.md](../contracts/accounts-api.md):

| Endpunkt | Zweck |
| --- | --- |
| GET /auth/google/start, GET /auth/google/callback | Google-Login |
| POST /auth/register, POST /auth/login | E-Mail-Registrierung und Login |
| POST /auth/confirm, POST /auth/recover, POST /auth/recover/complete | Bestätigung und Reset |
| GET /api/v1/me | Konto und CSRF-Token; anonym 401 |
| POST /api/v1/auth/logout, POST /api/v1/auth/logout-all | Session(s) beenden |
| GET /api/v1/me/progress | Eigener vollständiger Zustand einschließlich Revisionen |
| PUT/DELETE /api/v1/me/progress/{imdbId}/watched | Gesehen setzen/entfernen |
| PUT/DELETE /api/v1/me/progress/{imdbId}/watchlist | Watchlist setzen/entfernen |
| POST /api/v1/me/progress/import, GET /api/v1/me/progress/export | Backup übernehmen/herunterladen |
| DELETE /api/v1/me | Eigenes Konto löschen |

Nutzer-ID nur aus geprüfter Session ableiten; jede Query ist nutzerbezogen.
Unbekannte Film-IDs: 404. Writes verlangen If-Match mit Revision; neue Zeilen: 0.
Fehlend: 428; veraltet: 412 mit aktuellem Stand. Prüfung/Update und Erst-Insert
atomar. Zielwerte statt Toggle; anderes Feld erhalten. Bei passender Revision und
unverändertem Zustand kein neues Hochzählen. Web-Refresh bei Fokus, nach Writes
und in aktiven Ansichten alle 60 Sekunden.

Die vorhandene ProgressRepository erhält eine Cloud-Implementierung; Gast-Dexie
bleibt. Login bietet eine ausdrückliche lokale Übernahme. Import validiert wie
bisher Format, IDs, Duplikate und Datum, maximal 5 MB/20.000 Zeilen. Zunächst nur
fehlende Filmzeilen ergänzen; Cloud-Zeilen einschließlich entfernter Häkchen gewinnen.
Import transaktional/wiederholbar; Konflikte und unbekannte IDs anzeigen, Sicherung
erhalten. Gerätetimestamps dürfen keine Cloud-Werte überschreiben.

Konto-Caches nach UUID trennen und beim Logout löschen, ohne Cloud-Filme in den
Gast-Speicher zu kopieren. Cache leeren und Serverliste löschen sind verschiedene
Aktionen. Web schreibt zunächst nur online und zeigt Fehler an; 401 verlangt
Anmeldung statt stillen Gast-Wechsel. Native Apps erhalten dagegen eine lokale
Outbox mit Konfliktbehandlung gemäß mobilem Plan.

## Löschung, Rechte und Umsetzung

Frisch authentifizierte Kontolöschung entfernt Credentials, Identitäten, Sessions
und Fortschritt kaskadierend. Später vorhandene Apple-Tokens vorher in derselben
Transaktion als verschlüsselte Widerrufsaufträge übernehmen. Worker mit Backoff/
Lease, erfolgreiche Jobs löschen; nach sieben Tagen ungelöster Fehler alarmieren.
Aufträge haben keinen users-Fremdschlüssel.
[Apple-Lifecycle](https://developer.apple.com/documentation/technotes/tn3194-handling-account-deletions-and-revoking-tokens-for-sign-in-with-apple).

Read-only-Katalog-Pool behalten. USER_DATABASE_URL erhält Rechte auf persönliche
Tabellen und Katalog-Lesezugriff. Crawler/Admin brauchen keine Rechte auf Filmlisten.
Google-Login gibt keine Admin-Rechte. Secrets bleiben serverseitig; kein Vite-
Secret. Logger redigiert Passwörter, Codes, Tokens und Cookies.

Reihenfolge: Bibliotheks-/Adapter-Integration, reguläre Migration/Rollen, Auth-API,
Cloud-Repository/Konto/Import, Abnahme über Makefiles. Kriterien: vollständige
Registrierung/Bestätigung/Reset/Google-Flows, Nutzertrennung, Canons/Geräte,
Konflikte, Import ohne Wiederherstellung entfernter Häkchen, Logout/Sperre/Löschung.
Die Migration 0005 wurde lokal angewendet, Backend und Frontend kompiliert.
Die oben genannten Auth-Abnahmeprüfungen wurden noch nicht ausgeführt.

## Mailversand ohne Gmail

Reel Canon benötigt zunächst ausgehende Bestätigungs-/Reset-Mails. Ein eigener
SMTP-Mailserver ist möglich; Gmail ist nicht erforderlich. Optionen: vorhandener
SMTP-Zugang des Domainhosters oder selbst betriebener Postfix/Postal. Ein eigenes
Postfachsystem mit IMAP/Webmail ist für App-Mails nicht nötig.

Eigenbetrieb erfordert Domain, erreichbaren Server, geeignete feste IP und PTR,
SPF/DKIM/DMARC, TLS, Wartung und Bounce-Verarbeitung. Für direkte Zustellung muss
ausgehendes SMTP auf Port 25 beim Hoster erlaubt sein. Das Backend sendet
authentifiziert und verschlüsselt über den konfigurierten SMTP-Server.
Kein offenes Relay. Zustellbarkeit an Gmail, Outlook und iCloud vor Freigabe prüfen.

Empfehlung für den ersten App-Start: vorhandenen Domain-SMTP verwenden, sofern
verfügbar; Eigenbetrieb als unabhängig austauschbare SMTP-Option einplanen.
Lokale Entwicklung kann einen Mail-Catcher nutzen, der nichts extern versendet.
Postal ist eine umfangreichere Versandplattform und benötigt eigenen Betrieb;
Software kostenlos bedeutet nicht kostenlosen Serverbetrieb.

Quellen: [Postal](https://docs.postalserver.io/),
[Postal-Voraussetzungen](https://docs.postalserver.io/getting-started/prerequisites/),
[Google-Absenderanforderungen](https://support.google.com/mail/answer/81126).
