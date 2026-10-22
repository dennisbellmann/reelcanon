# Reel Canon Mobile

Eigenständiger Bereich im Monorepo für Kotlin/Compose Multiplatform. Die Android-App
nutzt dasselbe öffentliche Go-API wie die Web-App; sie benötigt keinen WebView.
Gemeinsame Oberfläche, Filter, API-Client und Room-Datenbank liegen in `composeApp`.
Android und iOS haben getrennte native Einstiegspunkte.

```text
mobile/
  composeApp/src/commonMain/  Oberfläche, Domäne, Room, Ktor, Poster-Downloads
  composeApp/src/commonTest/  Filter, Editionen und Backup-Validierung
  composeApp/src/androidMain/ Android-Datenbank und Bildcache
  composeApp/src/iosMain/     iOS-Datenbank und Compose-UIViewController
  composeApp/schemas/        Versionierte Room-Schemata
  androidApp/                Activity, Manifest, native Dateiauswahl, Gerätetests
  iosApp/                    SwiftUI-Einstieg und Xcode-Projekt (noch ungeprüft)
  gradle/libs.versions.toml  Zentral fixierte Bibliotheksversionen
```

## Funktionsumfang

- Veröffentlichte Sammlungen aus dem API, vollständige Filmmetadaten und Poster.
- Titel-/Originaltitel-/Regiesuche, Fortschritt, Jahrzehnt, Jahr, Genre, Regie,
  Mindestbewertung und Sortierung nach Rang, Titel, Jahr oder Bewertung.
- Filmdetails, Gesehen-Häkchen, Merkliste und gemeinsamer Fortschritt je IMDb-ID.
- Jahrzehnt-Album mit monochromen ungesehenen und farbigen gesehenen Postern.
- Automatisch dauerhaft gespeicherte Metadaten. „Offline speichern“ lädt Poster
  zusätzlich in dauerhaften App-Speicher; vorhandene Poster werden übersprungen,
  fehlgeschlagene können erneut geladen werden. Der normale Coil-Cache ist auf
  150 MiB begrenzt. Permanente Downloads bleiben bis zur App-Deinstallation erhalten.
- JSON-Sicherungen im bestehenden Web-Format `reel-canon-progress/v1`; Android
  verwendet die native Dateiauswahl. Import übernimmt neuere Einträge atomar und
  bewahrt andere Filme. Entfernte Häkchen bleiben als lokale Zustände gespeichert.

Der erste Start braucht eine erreichbare API. Bereits geladene Sammlungen, lokale
Häkchen und dauerhaft geladene Poster bleiben nach Neustart ohne Netzwerk verfügbar.
Eine ungültige/unvollständige Edition ersetzt den alten Snapshot nicht. Fehlende
Poster erhalten einen Platzhalter. Aus der Online-Entdeckung entfernte Sammlungen
bleiben lokal erhalten; dies ist derzeit ein kostenloser Katalog ohne Entitlements.
Gastfortschritt der App und des Browsers ist getrennt; Austausch erfolgt per Backup.

Android-Konten verwenden Credential Manager, eine verschlüsselte Ablage mit
Android-Keystore-Schlüssel, kurzlebige Zugangstokens und rotierende Refresh-Tokens.
Die Room-Outbox ist dauerhaft und konto-getrennt; Fortschritt wird je Feld mit
Serverrevision und idempotenter Operations-ID abgeglichen. Anmeldung, expliziter
Gastimport, Konfliktentscheidung und Abmelden sind in Android verfügbar. Der
Go-Server prüft mobile Google-ID-Tokens und stellt eigene widerrufbare Sitzungen
aus. Web-OAuth und Web-Fortschritt bleiben beim Browser-Cookie-Vertrag.

Google-Anmeldung benötigt weiterhin einen korrekt angelegten Android-OAuth-Client
und `GOOGLE_MOBILE_CLIENT_IDS` in der Serverumgebung. Die Cloud-Console-Konfiguration
ist noch nicht verifiziert; solange die Allowlist leer ist, meldet die API mobile
Google-Anmeldung als nicht eingerichtet. Das Android-Feature ist daher gegen die
lokale Test-OIDC-Instanz und mit Gerätetests geprüft, nicht mit einem echten
Google-Cloud-Client. iOS hat noch keinen nativen Login-Adapter. Käufe/Billing,
WorkManager und kommerzielle Entitlements sind nicht implementiert; es gibt keine
simulierten Freischaltungen oder Store-Veröffentlichung. IMDb Top 250 bleibt das
vorhandene Archiv vom 20.08.2025.

Für die lokale Google-Einrichtung muss im bestehenden Google-Cloud-Projekt ein
Android-OAuth-Client für `com.reelcanon.app` mit dem SHA-1 des Debug-Zertifikats
angelegt werden. Den Fingerabdruck zeigt
`./gradlew :androidApp:signingReport`. Die Android-Client-ID kommt in
`GOOGLE_MOBILE_CLIENT_IDS`; der bestehende Web-Client bleibt in `GOOGLE_CLIENT_ID`
und `GOOGLE_CLIENT_SECRET`. Release-Signierung hat einen eigenen Fingerabdruck und
muss separat konfiguriert werden. Die `.env` in diesem Workspace enthält derzeit
keine mobile Client-ID.

## Android auf diesem Windows-System

JDK 17+ (hier verwendbar: Android Studio Q/JBR 21), SDK `platforms;android-37.0`,
Build Tools 36.0.0 sowie ein Android-Emulator. `JAVA_HOME` muss auf einen intakten
JDK zeigen. Der ältere Android-Studio-JBR-Pfad dieses Rechners ist unvollständig;
das Startskript verwendet bei Bedarf die vorhandene Installation „Android Studio Q“.
`ANDROID_HOME` zeigt auf `%LOCALAPPDATA%\Android\Sdk`.

Im Monorepo-Stamm:

```powershell
./scripts/start-local.ps1
./scripts/start-android.ps1 -Test
```

Ist das Backend bereits gestartet, genügt die zweite Zeile. Das Skript startet
den vorhandenen `Pixel_8a_API_35` ohne Löschen seiner Daten, wartet auf den Boot,
baut, testet, installiert das APK und öffnet Reel Canon. Andere AVDs/Ports:
`-Avd Pixel_10 -Serial emulator-5556`. Es lädt keine neuen System-Images herunter.
Die API-Adresse im Debug-APK ist `http://10.0.2.2:8080/api/v1`: der Host aus dem Emulator.
Für ein USB-Gerät: `adb reverse tcp:8080 tcp:8080` und mit
`'-Preelcanon.apiBaseUrl=http://127.0.0.1:8080/api/v1'` bauen.

Alternativ direkt:

```powershell
$env:JAVA_HOME='C:\Program Files\Android\Android Studio Q\jbr'
cd mobile
./gradlew.bat :androidApp:assembleDebug
./gradlew.bat :androidApp:lintDebug :composeApp:testAndroidHostTest
./gradlew.bat :androidApp:connectedDebugAndroidTest
# Optional: vorhandenen lokalen Top-250-Katalog zusätzlich vom Emulator aus prüfen
./gradlew.bat :androidApp:connectedDebugAndroidTest '-Pandroid.testInstrumentationRunnerArguments.liveCatalog=true'
```

APK: `androidApp/build/outputs/apk/debug/androidApp-debug.apk`. Testberichte unter
`androidApp/build/reports/androidTests/connected/debug/` und
`composeApp/build/reports/tests/testAndroidHostTest/`. Gerätetests verwenden eigene
temporäre Datenbanken/Poster und verändern weder den normalen Katalog noch den
persönlichen Fortschritt der App. Der optionale Go-Smoke-Test liest nur das API.

Root-Make-Ziele: `mobile-check`, `mobile-test`, `android-debug`,
`android-device-test`, `android-bundle`. Zusätzliche Gradle-Parameter über
`MOBILE_ARGS`. Die separate Android-CI ruft dieselben Ziele auf, baut das APK und
führt die isolierten UI-/Offline-Tests auf API 35 aus. Das bestehende Web/Go-`make ci`
bleibt ohne Android-SDK lauffähig.

## iOS vorbereitet, auf Windows nicht geprüft

Auf einem Mac mit Xcode `iosApp/ReelCanon.xcodeproj` öffnen, API in `Config.xcconfig`
setzen und eine Signing-Team-ID wählen. Auf macOS aktiviert Gradle `iosArm64` und
`iosSimulatorArm64`, einschließlich Room-Codegenerierung und Ktor/Darwin.
Xcode baut und bindet das gemeinsame Framework über Gradle ein. Deployment-Ziel: iOS 16.
Die Dateisicherung verwendet auf iOS zunächst den gemeinsamen JSON-Textscreen;
native Dateiauswahl und System-Zurücknavigation stehen dort noch aus. iOS-Quellen,
Framework-Build und Xcode-Projekt sind hier **nicht kompiliert oder getestet**.

## Build-Konfiguration

Gradle 9.6.0 mit SHA-256-geprüftem Wrapper, AGP 9.4.1, Kotlin 2.4.20,
Compose Multiplatform 1.12.1. compileSdk 37 ist für die verwendeten Bibliotheken
erforderlich; targetSdk 36 und minSdk 26. Debug erlaubt HTTP nur für den lokalen
Host/Emulator. Ein Release benötigt explizit
`-Preelcanon.apiBaseUrl=https://your-host/api/v1`; Signierung und Store-Konfiguration
sind separat einzurichten. Geheimnisse gehören nicht in Gradle-Properties oder das APK.

Quellen für die Struktur und Kompatibilität:
[Android-KMP-Plugin](https://developer.android.com/kotlin/multiplatform/plugin),
[AGP 9.4](https://developer.android.com/build/releases/agp-9-4-0-release-notes),
[Compose-Versionen](https://kotlinlang.org/docs/multiplatform/compose-compatibility-and-versioning.html),
[Room](https://developer.android.com/kotlin/multiplatform/room),
[Coil](https://coil-kt.github.io/coil/getting_started/).
## UI languages

The app UI supports German, English, Spanish and French using shared catalogs in
`../localization`. Native resources are generated by
`node ../scripts/localization.mjs --write`. Device/app language selects the UI;
Android 13+ offers these languages in system app settings. See
[the localization guide](../docs/localization.md), including the separate scope
of movie metadata and the Windows/iOS verification limit.

