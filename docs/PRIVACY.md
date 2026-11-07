# Datenschutzhinweise (Privacy)

Stand: 2026-11-07. Dieses Dokument beschreibt, wie der Ekanite-Server (Fork `schankst/ekanite`) personenbezogene Daten verarbeitet. Offene Issues, die diesen Text ändern können, sind jeweils verlinkt.

## 1. Verantwortlich & Geltungsbereich

- Dieses Dokument beschreibt die Datenverarbeitung durch den Ekanite-Server (Fork `schankst/ekanite`) im Betrieb.
- Verantwortlicher/Auftragsverarbeiter und konkrete Rechtsgrundlage hängen vom jeweiligen Betriebskontext ab und sind vom Betreiber zu bestimmen (siehe Abschnitt 3).

## 2. Verarbeitete Datenfelder (Datenkategorien)

- **Rohzeile (`Source`)**: die vollständige, unveränderte empfangene Logzeile. Sie kann personenbezogene Daten (PII) enthalten.
- **`SourceIP`** (Absender-IP der Verbindung; intern als `source_ip` geführt) bzw. **`dstip`** (bei FortiOS, sofern im Ereignis vorhanden): IP-Adressen.
- **Geparste Felder** (je nach Eingabeformat, vgl. RFC5424/FortiOS): z. B. `timestamp`, `host`, `app`, `pid`, `message_id`, `message` sowie weitere vom Parser erkannte Felder im geparsten Ereignis.
- **Metadaten der Verarbeitung**: z. B. Empfangszeit/Sequenz zur zeitlichen Zuordnung und Indexierung — keine eigenständige Personenbezogenheit, können aber zusammen mit anderen Feldern Rückschlüsse erlauben.

## 3. Zweck & Rechtsgrundlage

- **Zweck**: Syslog-Sammlung, Indexierung und Durchsuchbarkeit (Log-Analyse, Forensik, Fehlerdiagnose).
- **Rechtsgrundlage**: in der Regel berechtigtes Interesse des Betreibers bzw. Auftragsverarbeitung — nach Betriebskontext zu prüfen und zu dokumentieren. Es findet **kein** automatisierter Personenbezug/Profiling durch Ekanite selbst statt.

## 4. Speicherdauer

- Konfigurierbar über das Flag `-retention` (CLI-Default: `168h` / 7 Tage; Minimum laut Hilfe: 24 Stunden). Hinweis: Die `d`-Suffix-Formulierung/Handhabung ist Gegenstand von #44.
- Die Retention wird stündlich erzwungen (`runRetentionEnforcement` in `engine.go`); abgelaufene Tagesindizes werden gelöscht.
- **Betreiberhinweis**: Bei einem vom Default abweichenden `-retention`-Wert ist der tatsächlich konfigurierte Wert hier bzw. in der betrieblichen Dokumentation zu ergänzen.

## 5. Löschverfahren

- **Retention** löscht ganze Tagesindizes (`DeleteIndex` = `os.RemoveAll`). Optional überschreibt `-wipe` die Indexdateien zuvor mit Nullen (unwiderrufliche Löschung gegen naheliegende Wiederherstellung; hat Schreibkosten, Default aus).
- **Löschbeleg**: Mit `-auditlog <datei>` wird pro Retention-Löschung eine JSONL-Zeile geschrieben (`ts`, `index`, `wiped`) als nachvollziehbarer Löschbeleg (Default: deaktiviert). Datei wird mit `0600` angelegt.
- Eine Löschung/Redaktion einzelner Events ist **GAP-2 (#73)**; Auskunfts-/Löschansprüche (Art. 15/17 DSGVO) siehe **#73/#94** — jeweils noch offene Issues. Aktuell nur manuell über das Datadir durch den Betreiber möglich.

## 6. Zugriffskontrolle

- **Query-Ports**: Authentifizierung fehlt noch (**#78, S1**) — bis dahin nur auf Loopback binden (z. B. `-query localhost:...`). Nicht authentifiziert ins Netz exponieren.
- **Diagnostics** (`-diag`, expvar/pprof): ungeschützt (**#52**) — nur auf Loopback binden oder deaktivieren.
- **Dateisystem**: Das Datadir und alle Index-/Shard-Dateien werden mit `0700`/`0600` (Owner-only) angelegt; `ekanited` setzt zusätzlich eine restriktive `umask` (0077). Bestehende Indizes werden beim Öffnen auf diese Berechtigungen repariert (**#90**).

## 7. Verschlüsselung

- **Transport**: optional TLS/mTLS für den TCP-Eingang über `-tlspem`/`-tlskey` (aktuelle Flags; ECDSA/Chain-Reload siehe **#74**). UDP bleibt unverschlüsselt. Siehe auch README (TLS-Einschränkungen).
- **At rest**: Keine anwendungsseitige Verschlüsselung; Dateien liegen im Klartext (aber nur für den Owner lesbar, `0700`/`0600`, siehe §6). Entscheidung: Verschlüsselung auf Volume-Ebene (z. B. LUKS) wird empfohlen und ist als Betriebsentscheidung dokumentiert (**#90**); Backups siehe **OPS-5 (#70)**.

## 8. Backups

- Keine automatische Backup-Integration in Ekanite. Empfehlung: betrieblich abgesicherte Volume-Snapshots oder Restic (siehe **OPS-5, #70**). Backups enthalten dieselben Datenkategorien wie das Datadir und unterliegen denselben Schutzanforderungen.

## 9. Aufbewahrungs-/Compliance-Hinweis

- Anonymisierung (IP/UUID) geplant in **#93 (DSGVO-1)**; Datensparsamkeit (Feld-Storage statt vollständiger Rohzeile, wo möglich) geplant in **#95 (DSGVO-3)**.
- Diese Angaben sind bestandsnah zur aktuellen Implementierung und ersetzen keine individuelle Rechtsberatung oder betriebliche Verarbeitungsverzeichnis-Einträge.
