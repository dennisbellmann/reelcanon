# Android-Verifikation · 2026-10-06

Die native Android-Vorschau liegt in `mobile/`. Gemeinsame Oberfläche und Logik
sind Kotlin/Compose Multiplatform; API und Web-App bleiben eigene Monorepo-Bereiche.
Die Anleitung steht in [mobile/README.md](../mobile/README.md).

## Tatsächlich ausgeführt

- Windows, Android Studio Q/JBR 21.0.10, Android SDK 37.0, Gradle 9.6.0,
  AGP 9.4.1, Kotlin 2.4.20, Compose Multiplatform 1.12.1.
- Vorhandener AVD `Pixel_8a_API_35`, Android 15, x86_64, `emulator-5554`.
- `:androidApp:assembleDebug`, `:androidApp:lintDebug`,
  `:composeApp:testAndroidHostTest` und `:androidApp:connectedDebugAndroidTest`
  mit `-Pandroid.testInstrumentationRunnerArguments.liveCatalog=true` erfolgreich.
- Sechs gemeinsame Tests: Suche nach Originaltitel/Regie, kombinierte Filter,
  Sortierung ohne Bewertung, unvollständige/doppelte Editionen, Web-Backup und
  ungültige Sicherungen.
- Vier Gerätetests: Lesen des tatsächlichen Go-Top-250-Katalogs; Navigation,
  Suche, Gesehen/Merkliste und Jahrzehnt-Album; echte Room-Persistenz mit Neustart
  des Repositorys, Offline-Poster und getrennte Fortschrittsfelder; Ablehnen
  unvollständiger Editionen und Merge neuerer Backups unter Erhalt anderer Filme.
- `make mobile-check` und `scripts/start-android.ps1 -SkipBuild` erfolgreich.
  Das Startskript installierte und startete das APK im vorhandenen Emulator.

## Manueller Test mit dem normalen Katalog

Die App lud den tatsächlichen Go-Katalog auf `10.0.2.2:8080` mit 250 Filmen,
deutschen TMDb-Metadaten und Postern. Startseite und Filmliste wurden visuell geprüft.
„Offline speichern“ lud **250/250 Poster** in dauerhaften App-Speicher.

Für „Die Verurteilten“ wurde ein Gesehen-Häkchen gesetzt. Danach wurde die App
beendet, ein APK-Update installiert und der Emulator in Flugmodus mit deaktiviertem
WLAN/Mobilfunk gestartet. Die API-Anfrage schlug erwartungsgemäß fehl; das Album
zeigte trotzdem 250 Filme, 250 gespeicherte Poster und **1/250 gesehen**. Das
gesehene Poster erschien farbig, ungesehene Poster monochrom. Damit ist die
Offline-Verfügbarkeit auch mit realen Metadaten/Bildern nach Prozess-Neustart und
APK-Update geprüft. Die Netzwerkverbindung wurde anschließend wieder aktiviert
und das vorübergehende Test-Häkchen entfernt.

Die isolierten Gerätetests verwenden eigene temporäre Datenbanken und Dateien;
der öffentliche Go-Katalog wird ausschließlich gelesen. Ein früherer Lint-Lauf
fand eine API-27-Theme-Einstellung und fehlende Netzwerk-XML-Attribute; diese
wurden korrigiert. Abschließender Lint: keine Fehler. Der Hinweis auf targetSdk 36
gegenüber compileSdk 37 bleibt sichtbar; Android 17 ist hier noch nicht getestet.

## Grenzen

iOS-Einstieg/Room/Ktor-Adapter und Xcode-Projekt sind vorbereitet, auf Windows
aber nicht kompiliert oder getestet. Native Konten, mobile Token-Sessions,
Cloud-Sync/Outbox und Store-Käufe sind noch nicht integriert. Die separate
Android-GitHub-Actions-Konfiguration ist hinzugefügt, aber hier nicht auf GitHub
ausgeführt. Keine Store-Veröffentlichung, kein Signing und keine echten Käufe.
