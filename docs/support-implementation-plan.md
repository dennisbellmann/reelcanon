# Freiwillige Unterstützung: Anbieter und Umsetzungsplan

Stand: 06.10.2026. Recherche der offiziellen Anbieter- und Store-Regeln sowie
Abgleich mit dem bestehenden Projekt. Dies ist ein Plan; es werden weder Konten
angelegt noch Zahlungslinks oder Store-Produkte aktiviert.

## Empfehlung: Ko-fi Free für die Website

Für Reel Canon empfehle ich **Ko-fi im Free-Modus**, zunächst mit einmaligen
Beiträgen in EUR. Stripe für Kartenzahlungen verbinden; PayPal bei Bedarf
ergänzen. Die Anbieterwahl ist eine Empfehlung, noch keine eingerichtete Verbindung.

| Kriterium | Ko-fi | Buy Me a Coffee |
| --- | --- | --- |
| Monatliche Plattformgrundgebühr | Keine im Free-/Standard-Modus | Keine |
| Einmalige Unterstützung | Free: 0 %; Standard: 5 % | 5 % |
| Monatliche Unterstützung | 5 % Plattformgebühr | 5 % Plattformgebühr plus mögliche Zuschläge |
| Zahlungsabwicklung | PayPal/Stripe, eigene Gebühren zusätzlich | Stripe-Abwicklung, Gebühren und mögliche Zuschläge zusätzlich |
| Vorschlag für Reel Canon | Free-Modus, einmalige Unterstützung zuerst | Gute Alternative bei bevorzugter Bedienung |

Ko-fi startet neue Konten mit Standard-Funktionen. Unter Settings > Payment
„Get all of Ko-fi“ deaktivieren, um einmalige Unterstützung ohne Plattformgebühr
zu erhalten. EUR wird unterstützt. Zahlungen gehen an das verbundene PayPal-
oder Stripe-Konto. Es gibt weiterhin Zahlungsgebühren; 0 % Plattformgebühr
bedeutet nicht kostenlose Abwicklung.
[Ko-fi Gebühren und Währungen](https://help.ko-fi.com/hc/en-us/articles/360002506494-Does-Ko-fi-take-a-fee),
[Buy Me a Coffee Gebühren](https://help.buymeacoffee.com/en/articles/8105744-how-to-calculate-charges-on-your-payment).

Stripe ermöglicht Karten, Apple Pay und Google Pay auf der Ko-fi-Seite. Diese
Zahlungsarten ersetzen **nicht** Apple In-App Purchase oder Google Play Billing
in den Store-Apps.
[Ko-fi Stripe-Anbindung](https://help.ko-fi.com/hc/en-us/articles/360007522474-Connect-your-Stripe-account-and-start-earning).

Später können „Simple monthly tips“ ohne Mitgliedschaftsstufen und exklusive
Inhalte hinzukommen. Dafür fällt bei Ko-fi eine Plattformgebühr an. Zum Einstieg
ist weder ein Gold-Abo noch ein Mitgliederbereich notwendig.
[Ko-fi monatliche Unterstützung](https://help.ko-fi.com/hc/en-us/articles/4402945994001-Ko-fi-Memberships-and-Membership-Tiers).

## Ist Unterstützung in Apps möglich?

**Ja, aber der Zahlungsweg hängt von der Distribution ab.** Der Web-Link ist
einfacher als ein einheitlicher Ko-fi-Button in allen nativen Apps.

| Oberfläche | Vorgeschlagener Weg | Einordnung |
| --- | --- | --- |
| Website / Web-App im Browser / installierte Web-PWA | Externer Ko-fi-Link | Technisch einfacher Einstieg; keine Store-Abrechnung |
| Android außerhalb von Google Play | Externer Ko-fi-Link im Browser | Google-Play-Billing-Pflicht greift nicht allein durch Android; Regeln eines anderen Stores separat prüfen |
| Android über Google Play | Native, wiederholbare Trinkgeldkäufe als spätere Standardvariante planen | Externe direkte Beiträge haben eine bedingte Ausnahme; konkreten Ko-fi-Fall vor Freischaltung prüfen |
| iOS über den App Store | Native Trinkgeldkäufe über StoreKit als spätere Standardvariante planen | Entwickler-Trinkgelder über IAP sind ausdrücklich vorgesehen; externer Ko-fi-Link ist keine pauschal freigegebene Variante |

Eine als Store-App verpackte Website ist nicht automatisch eine freie Web-PWA.
Die Installations-/Distributionsform ist ausschlaggebend.

### Google Play

Googles Payments FAQ nimmt direkte freiwillige Beiträge aus, wenn 100 % an den
Creator gehen und keinerlei digitale Gegenleistung entsteht. Bereits ein Badge
oder freigeschalteter Inhalt verändert diese Einordnung. Andernfalls ist Billing
nach den jeweiligen Regeln erforderlich. Die FAQ erlaubt außerdem Android-
Distribution außerhalb von Play ohne dessen Billing.
[Google Payments FAQ](https://support.google.com/googleplay/android-developer/answer/10281818?hl=en).

**Bewertung für uns:** Ko-fi Free mit reinen einmaligen Beiträgen ist ein möglicher
Kandidat für die Ausnahme. Aus „0 % Plattformgebühr“ folgt aber keine bestätigte
Store-Zulässigkeit: Zahlungsabwicklung, Empfänger und Checkout müssen konkret
bewertet werden. Die zitierte FAQ erklärt nicht ausdrücklich die Behandlung aller
Zahlungsabwicklungsgebühren in dieser Ausnahme. Monatliche Beiträge mit
Plattformgebühr erhalten daher ebenfalls keine pauschale Freigabe im Plan.

Für die Play-App einen externen Unterstützungsbutton zunächst deaktiviert lassen.
Vor einem solchen Release den konkreten Fall über den offiziellen Policy-Support
klären und in den Review-Hinweisen dokumentieren; alternativ die native
Billing-Variante umsetzen. Dies ist eine Implementationsempfehlung, kein allgemeines
Verbot externer Beiträge. Regionale Zahlungsprogramme sind zusätzliche Optionen,
aber nicht Grundlage der ersten Umsetzung.

### Apple App Store

Apple sieht Entwickler-Trinkgelder über In-App Purchase ausdrücklich vor
(3.1.1). Die Ausnahme für freiwillige Geldgeschenke zwischen Einzelpersonen
setzt unter anderem vollständige Weitergabe und fehlende digitale Gegenleistung
voraus (3.2.1(vii)). Sie bestätigt keinen konkreten Ko-fi-Checkout für Reel Canon.
Die Sonderregel für genehmigte gemeinnützige Organisationen ist keine Grundlage
für unser persönliches Hobbyprojekt.
[Apple App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/).

**Bewertung für uns:** Für iOS native Trinkgeldprodukte mit StoreKit planen.
Keine generelle Zusage, dass Ko-fi-Links untersagt oder genehmigt sind. Für einen
externen Weg wäre eine eigene Prüfung der konkreten Umsetzung und Storefront
notwendig. Das Öffnen von Safari oder einer Zwischenwebsite allein beweist keine
Zulässigkeit. Keine verdeckte Weiterleitung oder andere Darstellung im Review.

## Umsetzung in drei Etappen

### 1. Unterstützerseite einrichten

- Dennis richtet die Ko-fi-Seite ein, verbindet den Zahlungsanbieter und nimmt
  dessen Identitäts-/Auszahlungsprüfung vor. Eine echte Profil-URL wird benötigt;
  der Plan erfindet keinen freien Benutzernamen.
- Name Reel Canon, EUR, Free-Modus und einmalige Unterstützung einstellen.
  Als Orientierung 5 € nennen; Betrag kann auf der Unterstützerseite gewählt werden.
- Kurzer Text zum Hobbyprojekt und Serverbetrieb, keine Leistungsversprechen,
  keine steuerliche Absetzbarkeit und keine Gegenleistung für Beiträge.
- Öffentliche Zahlungs-/Profilangaben und Beleg prüfen. Falls ein Ziel sichtbar
  werden soll, zunächst echte Betriebskosten statt eines fiktiven Finanzstands nennen.

### 2. Website einbinden

- In `frontend/src/routes/BackupPage.tsx` unter „Über Reel Canon“ einen kleinen
  Bereich mit „Reel Canon unterstützen“ und einem Satz zum Serverbetrieb vorsehen.
  Er muss auch für Gäste ohne Reel-Canon-Konto funktionieren.
- Die echte Zieladresse über eine neue öffentliche Konfiguration wie
  `VITE_SUPPORT_URL` setzen. Fehlende/ungültige URL blendet den gesamten Bereich
  aus. HTTPS und Host `ko-fi.com` sowie eine konfigurierte Profiladresse prüfen;
  keine frei vom Nutzer übergebene Weiterleitung.
- Als normaler Link öffnen, ohne eingebetteten Checkout, Widget-Script oder
  automatisch geladene Drittanbieterinhalte. Neue Registerkarte mit
  `noopener noreferrer`; externe Zielseite im Linktext/Hinweis verständlich machen.
- Texte zentral in `localization/{de,en,es,fr}.json` ergänzen und Ressourcen über
  den bestehenden Generator erzeugen. Barrierefreie Beschriftung und Tastaturfokus.
- Hinweis auf den externen Zahlungsanbieter in den Datenschutzinformationen
  berücksichtigen. Keine Filmhistorie, E-Mail oder Konto-ID an die URL anhängen.
- Kein Backend, Webhook oder Entitlement-System erforderlich: eine Unterstützung
  schaltet nichts frei. Abrechnung zunächst über Anbieterberichte; keine ungeprüfte
  öffentliche Anzeige „bisher eingegangen“.

Abnahme: Ohne konfigurierte URL keine defekte Schaltfläche; korrekter externer
Link in allen vier Sprachen auf Desktop und schmalem Bildschirm; keine
Drittanbieteranfrage vor dem Klick; Gast-/Kontofortschritt unverändert.
Passende Prüfungen: vorhandene Lokalisierungsprüfung und Frontend-Build sowie
gezielte UI-Prüfung der Sichtbarkeit und des Links. Kein kompletter Payment-Test
im eigenen Backend für diese Etappe.

### 3. Native Apps später ergänzen

- Gemeinsame Unterstützungsansicht in den Einstellungen von
  `mobile/composeApp/src/commonMain/kotlin/com/reelcanon/mobile/ReelCanonApp.kt`.
  Nicht beim Film-Abhaken oder bei jedem Start anzeigen.
- Kleines gemeinsames Interface für Laden, Kaufen und Kaufstatus; Android
  implementiert Play Billing, iOS StoreKit. Pro Distribution einen expliziten
  Modus vorsehen: `disabled`, `external` oder `storeTips`. Im Store-Build ist
  `external` nur nach der konkreten Prüfung verfügbar, nicht durch Sprachwahl.
- Wiederholbare einmalige Trinkgeldprodukte vorsehen, zum Beispiel kleine,
  mittlere und große Unterstützung. Gewünschte EUR-Beträge sind nur Ausgangspunkte;
  angezeigten Preis und Währung immer vom Store laden. Zunächst kein Trinkgeldabo.
- Produkte in Store-Konsolen konfigurieren und als wiederholbare Kaufart passend
  zur Plattform einrichten. Kein dauerhafter Unterstützerstatus, Badge oder
  freigeschalteter Inhalt. Consumable-Trinkgeld erzeugt keine wiederherstellbare
  Funktionsberechtigung; Werbefrei-Käufe wären getrennte Produkte.
- Ausstehende, abgebrochene und fehlgeschlagene Zahlungen behandeln; nur bestätigte
  Käufe als erfolgreich anzeigen. Verifikation und idempotente Verarbeitung
  vor plattformspezifischem Consume/Finish. Supportoberfläche mit Offline-/Fehlerzustand.
- Für Produktionsabrechnung Storeberichte nutzen, nicht lokale Dankeschön-Zähler.
  Kein Reel-Canon-Konto als Voraussetzung für reine Unterstützung.
- Android mit Billing-Testern prüfen; iOS benötigt Mac/Xcode und StoreKit-/Sandbox-
  Prüfung. Wiederholte Beiträge, Doppelereignisse, App-Neustart und Zahlungsausfall
  testen. Review-Hinweise erklären Freiwilligkeit, Produktart und fehlende Gegenleistung.

Die Store-Variante hat mehr Implementierungsaufwand und Store-Abzüge. Deshalb
zuerst Web-Unterstützung starten und App-Käufe nach Bedarf ergänzen. Eine App kann
bis dahin vollständig funktionieren, ohne einen Zahlungsbutton zu enthalten.

## Offene Eingaben und Status

- Ko-fi-Profil: `https://ko-fi.com/milkproduction` (Profilname Dennis Bellmann).
- Laut Dennis ist PayPal mit Ko-fi verbunden. Die öffentliche Ko-fi-Seite zeigte
  zusätzlich „Action required: check your payment setup to accept support“; daraus
  folgt nicht, dass die PayPal-Verbindung fehlt. Den Zahlungsstatus habe ich nicht
  weiter eingesehen und keine Testzahlung durchgeführt.
- Website-Einbindung in der Arbeitskopie umgesetzt: lokalisierter Hinweis unter
  „Über Reel Canon“ auf der Sicherungs-/Einstellungsseite und ein Link im globalen
  Seitenfuß. Die öffentliche Konfiguration `VITE_SUPPORT_URL` akzeptiert nur eine
  HTTPS-Profiladresse auf `ko-fi.com`; ohne gültige Konfiguration erscheint kein Link.
  Für lokale Builds ist die Adresse in `.env` gesetzt; `.env.example` dokumentiert
  denselben Wert. Die Bereitstellung auf der Produktionsseite ist nicht erfolgt.
- Die öffentlichen Ko-fi-Profiltexte wurden nach einem Fehler der Ko-fi-Seite beim
  Speichern noch nicht aktualisiert; im Browser trat `ReferenceError: $ is not defined`
  auf. Der bestehende Begrüßungstext bleibt daher sichtbar.
- Für „About“ freigegebener englischer Text (noch nicht gespeichert):
  > Reel Canon is my film album for exploring cinema classics and recording what I’ve watched. If you enjoy the project, a voluntary contribution helps cover its running costs and support its continued development. Contributions are optional and unlock no features or rewards. Thank you for supporting Reel Canon!
- Tatsächliche Abwicklungsgebühren und Belege sind noch nicht geprüft.
- Konkrete Store-Produkte und ein produktiver Billing-/StoreKit-Adapter fehlen.
- Nutzungsrechte für den öffentlich betriebenen, unterstützten Katalog bleiben
  entsprechend dem [Finanzierungskonzept](hobby-financing-concept.md) zu klären.
- Vor Store-Einreichung Regeln erneut prüfen. Die Dokumentenrecherche ist keine
  individuelle Store-Freigabe; der Support wurde nicht angeschrieben.

Die Empfehlung lautet: **Ko-fi Free auf der Website zuerst; native Unterstützung
später über die jeweiligen Store-Zahlungswege planen.**
