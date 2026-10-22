# Handover: Google-Konto und Fortschritt in Android

## Ziel

Mit demselben Google-Konto in der Android-App und im Web anmelden. Der Fortschritt
für „Gesehen“ und „Merkliste“ gehört einem serverseitigen Reel-Canon-Konto und wird
zwischen Web und Android abgeglichen. Eine Änderung im Web soll nach dem Abgleich in
Android erscheinen und umgekehrt.

## Verifizierter Ausgangspunkt

- Die Web-Anmeldung über Google funktioniert. Der aktive Web-Client gehört zum
  Google-Cloud-Projekt `reel-canon`; sein Secret liegt lokal in der ignorierten
  `.env`. Es gehört niemals in Gradle-Konfiguration oder das APK.
- Der Web-Client ist ein OAuth-Webclient mit lokalem Callback
  `http://127.0.0.1:8080/api/v1/auth/google/callback`. Die Google-App befindet sich
  im Testmodus. Diese Web-Redirect-URI ist kein Android-Redirect und richtet
  Credential Manager noch nicht für das Android-Paket ein.
- Der Web-Server kennt die Google-Identität anhand von Provider und stabilem Google-
  Subject. Gleiche E-Mail-Adressen allein dürfen keine Konten zusammenführen.
- Der Web-Fortschritt wird bereits über `GET /api/v1/me/progress` und die
  Revisions-Endpunkte geschrieben. Die API authentifiziert diese Endpunkte derzeit
  mit einer Web-Session, `HttpOnly`-Cookie und CSRF-Token.
- Android hat noch keine Anmeldung, Kontositzung oder Cloud-Synchronisierung. Der
  KMP-`CatalogRepository` liest und schreibt Fortschritt nur in Room. `progress`
  wird aktuell allein nach IMDb-ID indiziert; ein Konto- oder Besitzerfeld fehlt.
- Es war bei der Prüfung kein Android-Emulator oder Gerät mit `adb` verbunden.

## Architekturentscheidung

Die vorhandenen Browser-Cookies und CSRF-Header sind kein mobiler Auth-Vertrag.
Android soll über Credential Manager ein Google-ID-Token für die Server-Audience
erhalten und es einmalig an einen neuen, dokumentierten Mobile-Login-Endpunkt des
Go-Backends senden. Das Backend prüft Signatur, Issuer, Audience, Ablaufzeit,
Nonce und erlaubte Client-IDs. Es bindet die Sitzung ausschließlich an Google-
Provider plus Subject und gibt eine eigene kurzlebige Reel-Canon-App-Sitzung zurück.

Für die App-Sitzung ist ein widerrufbares, rotierendes Erneuerungsverfahren
vorgesehen. Die App übermittelt anschließend ihr Reel-Canon-Zugriffstoken als
Bearer-Token. Google-ID- und Refresh-Tokens werden weder als API-Zugangstoken
verwendet noch in Room gespeichert. Der Mobile-Login-Vertrag, Datenbankschema,
Token-Laufzeiten, Rotation, Wiederverwendungserkennung, Rate-Limits und
Widerrufsverhalten müssen vor der Implementierung im API-Vertrag festgeschrieben
werden; diese Endpunkte existieren derzeit nicht.

Die Google-Identität muss dem bestehenden serverseitigen Konto zugeordnet werden.
Bei unbekanntem Subject ist die vorgesehene Kontenerstellung ausdrücklich mit dem
Web-Verhalten abzustimmen. Gleiche E-Mail-Adressen sind kein Nachweis für eine
Verknüpfung. Ein vorhandenes Passwortkonto wird nicht stillschweigend mit Google
zusammengeführt.

## Umsetzungsreihenfolge

1. **Backend-Vertrag und Authentifizierung:** Mobile Login, Erneuerung, Logout,
   Kontostatus und Progress-Endpunkte definieren. Bearer-Sitzungen getrennt von
   Web-Cookies verwalten, Rotation und Widerruf implementieren und Google-ID-Tokens
   nur serverseitig gegen eine feste Client-ID-Allowlist prüfen. Den Account-Status
   weiterhin aus der authentifizierten Sitzung ableiten, nie aus einer vom Client
   gelieferten User-ID.
2. **Google-Cloud-Konfiguration:** Android-OAuth-Client für das Paket
   `com.reelcanon.app` und dessen tatsächlich verwendete Signatur-Fingerprints
   einrichten. Den Google-OAuth-Testmodus und die Testnutzer berücksichtigen. Die
   Web-Client-ID als serverseitige Audience für Credential Manager konfigurieren;
   nur nötige öffentliche Client-IDs in die App-Konfiguration geben. Keine Secrets
   oder privaten Schlüssel einbetten.
3. **Android-Anmeldung:** Credential Manager in `androidMain` kapseln und aus der
   gemeinsamen Compose-Oberfläche starten. Authentifizierung, Ladezustand, Abbruch,
   Netzwerkfehler, abgelaufene Sitzung und Abmeldung sichtbar behandeln. Tokens in
   einer Android-Plattformablage mit Keystore-Schutz verwalten; die konkrete
   Bibliothek gegen die zum Implementierungszeitpunkt aktuelle offizielle Android-
   Dokumentation prüfen.
4. **Konto und Room:** Gemeinsamen Account-/Session-State in `commonMain` ergänzen.
   Progress und ausstehende Änderungen lokal eindeutig dem angemeldeten Konto
   zuordnen. Room-Schemaänderung samt Migration erstellen. Gäste, Konto A und Konto
   B dürfen ihre lokalen Daten nie gegenseitig anzeigen. Gastfortschritt nur nach
   ausdrücklicher Nutzeraktion importieren.
5. **Abgleich:** Nach Login, App-Start, Wechsel in den Vordergrund und
   Netzrückkehr den Account-Fortschritt vom Server laden. App-Änderungen über
   Revisionsprüfung senden; `412`-Konflikte behandeln und entfernte Zustände als
   Tombstones erhalten. Für Offline-Änderungen eine dauerhafte Outbox mit stabilen
   Operations-IDs und serverseitiger Idempotenz ergänzen; die vorhandene Web-API
   bietet diese Idempotenz noch nicht. Ein blindes Überschreiben anhand von
   Gerätezeitstempeln ist ausgeschlossen.
6. **Abmeldung und Kontowechsel:** App-Sitzung serverseitig widerrufen, Token lokal
   entfernen und den bisherigen Account-Cache aus dem sichtbaren App-Zustand
   entfernen. Vorhandene Gastdaten und Outbox-Daten nicht unbemerkt einem anderen
   Konto zuordnen.

## Relevante Stellen im Repository

- `mobile/composeApp/src/commonMain/kotlin/com/reelcanon/mobile/CatalogRepository.kt`:
  vorhandenes lokales Room-Progress-Verhalten und Katalog-API.
- `mobile/composeApp/src/commonMain/kotlin/com/reelcanon/mobile/Database.kt`:
  Room-Entity/DAO; Progress hat derzeit keinen Kontobesitzer.
- `mobile/composeApp/src/androidMain/kotlin/com/reelcanon/mobile/AndroidRepository.kt`:
  Android-Factory für Repository und Room-Datenbank.
- `mobile/composeApp/src/commonMain/kotlin/com/reelcanon/mobile/ReelCanonApp.kt`:
  Compose-Oberfläche; Konto-Einstieg und Status gehören in die Einstellungen.
- `mobile/androidApp/src/main/kotlin/com/reelcanon/app/MainActivity.kt`:
  Android-Einstieg, Activity-Result-Integration und Repository-Erzeugung.
- `mobile/gradle/libs.versions.toml`, `mobile/composeApp/build.gradle.kts` und
  `mobile/androidApp/build.gradle.kts`: zentrale Android-/Kotlin-Abhängigkeiten.
- `backend/internal/accounts/google.go`: bestehender Browser-OIDC-Flow und
  Subject-Zuordnung.
- `backend/internal/accounts/progress.go` und `contracts/accounts-api.md`:
  bestehende Web-Progress-API, Revisionsregeln und Tombstones.
- `docs/mobile-app-implementation-plan.md`: bestehende Zielarchitektur für
  Credential Manager, mobile Token-Sitzungen, Outbox und Sync.

## Abnahmekriterien

- Google-Anmeldung in Android erzeugt bzw. findet serverseitig dieselbe Identität,
  die auch die Web-Anmeldung verwendet; keine Zuordnung allein per E-Mail.
- Nach Android-Anmeldung erscheint ein im Web gesetzter Status nach dem
  Serverabgleich in der App.
- Ein in Android gesetzter Status erscheint nach Neuladen im Web.
- Status von Merkliste und „Gesehen“ ändern sich unabhängig und erhalten die
  bestehende Revision-/Konfliktsemantik.
- Neuinstallation oder erneute Anmeldung lädt den Serverfortschritt wieder.
- Gäste, verschiedene Konten und deren lokale Caches bleiben getrennt; Logout
  hinterlässt keinen sichtbaren Fortschritt des vorherigen Kontos.
- Abgelaufene/gesperrte Sitzungen, Netzwerkfehler und Konflikte verlieren keine
  lokale Offline-Änderung und werden verständlich angezeigt.
- Keine Google- oder Reel-Canon-Tokens gelangen in Logs, Backup-Dateien, APK-
  Konstanten oder gewöhnliche Room-Daten.

## Lokale Verifikation

Für Emulator-Verifikation die Account-API lokal starten und Android im Emulator
mit `http://10.0.2.2:8080/api/v1` konfigurieren. `mobile/README.md` beschreibt
`scripts/start-android.ps1 -Test` und das vorhandene AVD `Pixel_8a_API_35`.
Abnahme mit derselben Google-Testnutzeridentität: Web setzt Film A auf gesehen,
Android gleicht ab; anschließend Android setzt Film B auf Merkliste, Web gleicht
ab. Zusätzlich Abmeldung, Kontowechsel, offline gespeicherte Änderung,
Wiederverbindung, wiederholte Outbox-Zustellung und Konflikt testen.

## Offizielle Integrationshinweise

- [Android Credential Manager](https://developer.android.com/identity/credential-manager)
- [OAuth für native Apps](https://developers.google.com/identity/protocols/oauth2/native-app)
