# Sprache und Text · 2026-10-06

Reel Canon ist ein persönliches Filmalbum mit einer ruhigen, redaktionellen Stimme.
Web und App verwenden das Leitmotiv **Ein Leben voller Film**. Die Bedienung führt
mit klaren Aktionen durch die Sammlung. Technische Speichererklärungen stehen in
den Einstellungen; die Folgen einer Kontolöschung bleiben unmittelbar sichtbar.
Wiederholte Zwischenüberschriften, Beschreibungen oberhalb der Filmraster und
Hinweise zu öffentlichen APIs/Admin-Freigaben wurden entfernt. Sammlungsdetails
bleiben im Detaildialog. TMDb-Attribution steht bei „Über Reel Canon“.

## Eine Textbasis für Web und App

`localization/{de,en,es,fr}.json` enthält UTF-8-Text mit stabilen semantischen Schlüsseln.
Deutsch spricht mit „du“ an, Spanisch mit „tú“, Französisch mit „vous“. Englisch ist
direkt und freundlich. Übersetzungen bilden dieselbe Aussage ab, keine Wort-für-Wort-Kopie.
Die [Anweisung für Agenten](../localization/AGENTS.md) beschreibt den Arbeitsablauf.

Die React/Vite-App verwendet **i18next + react-i18next** mit typgeprüften Schlüsseln,
benannten Platzhaltern, CLDR-Pluralregeln und englischem Fallback. React maskiert
eingesetzte Werte. Die Sprachwahl steht unter `/backup` (Einstellungen); die bisherige
Backup-URL bleibt für gespeicherte Links erhalten. Ein gespeicherter Wunsch gewinnt
vor der Browser-Sprachliste. Regionale Varianten wie `es-MX` werden auf `es` abgebildet.
Blockierter Browser-Speicher verhindert den Sprachwechsel nicht. Ein Wechsel erhält
Route, Filter, Fortschritt und offene Dialoge; HTML-`lang`, Titel und Beschreibung
folgen der Sprache. IMDb-Bewertungen und Titelsortierung verwenden die aktive Locale.

Die native App erhält daraus generierte **Compose-Multiplatform-Ressourcen** mit
englischer Basis und `values-de`, `values-es`, `values-fr`. Die Ressourcenauswahl folgt
der System-/App-Sprache; Android 13+ bietet die vier Sprachen in den System-App-Einstellungen,
iOS deklariert sie im Bundle. Eine `Message`-Enum bietet typgeprüfte Zugriffe. Benannte
Platzhalter erlauben denselben Text in Web und App. Native Mengenanzeigen verwenden
sprachneutrale Zähler (`Filme: 1`); für weitere Mengen stehen generierte native
Pluralressourcen bereit. Navigation und Filterzustand verwenden stabile Werte.

Kontofehler tragen zusätzlich einen stabilen `code` im API-Ergebnis. Der bestehende
`error`-Text bleibt für alte Clients erhalten. Die Web-App übersetzt bekannte Codes
und zeigt bei unbekannten Fehlern eine lokale Nachricht. Gespeicherte Erfolgs- und
Fehlermeldungen werden erst beim Rendern übersetzt, auch nach einem Sprachwechsel.

## Prüfen und ändern

```sh
npm --prefix frontend run i18n:generate
npm --prefix frontend run i18n:check
```

Der Generator prüft alle vier Kataloge auf fehlende/zusätzliche/duplizierte Schlüssel,
leere Texte, Platzhalter, benötigte Pluralformen, Ressourcennamen-Kollisionen und
unbekannte Schlüssel im Code. Er verhindert neue harte JSX-Texte und prüft, ob native
Ressourcen aktuell sind. `--write` generiert Ressourcen und `LocalizedStrings.kt` neu.
Build und TypeScript-Prüfung führen die Prüfung automatisch aus, damit CI dieselben
Fehler findet. Nach Textänderungen generieren und die erzeugten Dateien mitführen.

## Umfang und Grenzen

Die vier Sprachfassungen betreffen die öffentliche Bedienoberfläche, einschließlich
Konten, Sicherungen, Filtern, Zustandsmeldungen und Accessibility-Labels. Der interne
Go-Admin bleibt deutsch. Titel, Inhaltsangaben, Länder/Genres und redaktionelle
Sammlungsbeschreibungen kommen unverändert aus dem vorhandenen Katalog; die TMDb-
Anreicherung lädt derzeit deutsche Metadaten. Diese Inhalte sind keine UI-Texte.
Vollständig lokalisierte Filminhalte erfordern eigene Katalog-Sprachfassungen samt
sprachabhängiger Anreicherung und API-/Cache-Schlüsseln. Die UI-Kataloge sollen solche
Inhalte nicht duplizieren oder vermeintlich übersetzen. iOS ist auf Windows nicht
kompilierbar und muss auf macOS gesondert überprüft werden.

## Quellen

- [i18next: Fallback](https://www.i18next.com/principles/fallback)
- [i18next: Plurals](https://www.i18next.com/translation-function/plurals)
- [i18next: TypeScript](https://www.i18next.com/overview/typescript)
- [react-i18next: useTranslation](https://react.i18next.com/latest/usetranslation-hook)
- [Kotlin: Localizing strings](https://kotlinlang.org/docs/multiplatform/compose-localize-strings.html)
- [W3C: Language declarations](https://www.w3.org/TR/i18n-html-tech-lang/)
