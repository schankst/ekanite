# Betrieb: Backup & Restore (SOP)

*(Issue #70 — wiederholbares, manuelles Runbook; kein automatischer CI-Test)*

Dieses Dokument beschreibt eine einfache, nachvollziehbare Sicherungs- und Wiederherstellungsstrategie für das Ekanite-Datenverzeichnis (`-datadir`, z. B. `/var/lib/ekanite`).

## Hinweis zur Konsistenz

**Daten werden nur bei gestopptem Dienst konsistent gesichert.** Ekanite verwendet kein Online-Lock-/Quiesce-Verfahren für Backups. Während `ekanited` schreibt/indiziert, kann ein Snapshot oder Dateibackup einen inkonsistenten Zwischenstand erfassen. Für konsistente Sicherungen: Dienst stoppen → sichern/snapshotten → Dienst starten.

## Backup-Empfehlung

Wähle eine der folgenden Varianten (je nach Umgebung):

1. **Volume-Snapshot** (bevorzugt bei Block-Storage/VM/Container-Volume): Snapshot des Datadir-Volumes erstellen, nachdem der Dienst sauber gestoppt wurde. Vorteile: schnell, atomar auf Storage-Ebene, einfache Rücksicherung.
2. **Restic** (dateibasiert, plattformunabhängig): `restic backup` auf das Datadir ausführen, nachdem der Dienst sauber gestoppt wurde. Vorteile: Deduplikation, Verschlüsselung, Offsite/Repository-Backends.

**Vor dem Backup:** Datadir-Größe abschätzen (Indexdaten wachsen mit Retention/Volumen; siehe ggf. Metriken/Beobachtung aus dem Betrieb). Sicherstellen, dass ausreichend Speicherplatz für Snapshot/Repository vorhanden ist.

## Restore-Schritte (Konsistenzhinweis)

1. **Dienst stoppen** (kontrolliertes Maintenance-Fenster):
   - systemd: `sudo systemctl stop ekanited`
   - Container: Container stoppen (`docker stop ...`), Datadir-Volume nicht löschen.
2. **Datadir sichern/prüfen (optional, empfohlen):** Vor dem Überschreiben den aktuellen Stand zur Seite legen (z. B. Umbenennen), falls ein Rollback nötig wird.
3. **Snapshot/Restore einspielen:**
   - Volume-Snapshot: Snapshot auf das Ziel-Volume zurückspielen bzw. Volume aus Snapshot wiederherstellen und am ursprünglichen Pfad bereitstellen.
   - Restic: `restic restore <snapshot-id> --target <zielpfad>` und Inhalt so platzieren, dass `-datadir` wieder auf das wiederhergestellte Verzeichnis zeigt. Besitz/Rechte prüfen (z. B. für systemd-`StateDirectory`/Service-User).
4. **Dienst starten:**
   - systemd: `sudo systemctl start ekanited` und `sudo systemctl status ekanited` prüfen.
   - Container: Container mit demselben Volume-Mount neu starten.
5. **Nachprüfung:** Logs auf sauberen Start/Index-Recovery prüfen; Such-/Query-Test gegen bekannte historische Daten durchführen (siehe Test-Runbook).

## Test-Runbook (wiederholbar, manuell)

Ziel: Nachweis, dass ein Restore funktioniert (erster Ernstfall ≠ erster Test). Regelmäßig in einer Testumgebung wiederholen.

1. **Ausgangszustand herstellen:** Ekanite mit Test-Datadir starten, einige bekannte Logereignisse einspeisen und über die Query-Schnittstelle verifizieren (z. B. ein eindeutiger Suchbegriff liefert erwartete Treffer).
2. **Konsistentes Backup erzeugen:** Dienst stoppen → Volume-Snapshot erstellen **oder** `restic backup <datadir>` ausführen → Dienst starten (nur falls weiter getestet wird).
3. **Datenverlust simulieren:** Dienst stoppen. Datadir in der Testumgebung entfernen/entleeren (oder ein frisches, leeres Volume verwenden). **Nicht in Produktion ohne bestätigtes Backup.**
4. **Restore durchführen:** Schritte aus „Restore-Schritte“ anwenden (Snapshot zurückspielen oder `restic restore`). Dienst starten.
5. **Erfolg prüfen:**
   - Dienst läuft ohne Startfehler.
   - Die in Schritt 1 verifizierten Suchbegriffe liefern wieder die erwarteten Treffer/Zeitrahmen.
   - Abweichungen dokumentieren (Zeitpunkt des Backups, fehlende Events seit letztem konsistenten Stand sind erwartbar).
6. **Dokumentieren:** Datum, verwendete Methode (Snapshot/Restic), Snapshot-ID/Repository-Referenz, Ergebnis und Auffälligkeiten festhalten.

## Abgrenzung

- Kein Online-Backup, keine integrierte Backup-Automation in Ekanite.
- Für Upgrades: Dienst stoppen → Binary/Container ersetzen → starten; Datadir bleibt erhalten (siehe README „Deployment“). Vor größeren Upgrades zusätzlich konsistentes Backup gemäß diesem Runbook erstellen.
