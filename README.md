# hiro-aristech-api

HTTP-API zwischen dem Aristech-Voicebot und dem HIRO/Bardioc-Graphen. Aristech ruft diese API auf, um Anrufer anhand von Telefonnummer oder Namen zu identifizieren, deren Support-Tickets zu lesen, Automation-Issues (Reasoning-Engine-Trigger) anzulegen und den Intent-Katalog zu laden.

## Was diese API macht

- **Caller-Identifikation** (`GET /api/v1/auth`) — löst Anrufer per Telefonnummer oder exaktem Namen auf, liefert ein Bearer-Token zurück.
- **Fuzzy-Namenssuche** (`POST /api/v1/auth/match`) — für unvollständige/fehlerhafte Spracherkennungs-Ergebnisse (STT), liefert bis zu 5 Kandidaten mit Confidence-Score statt hart zu scheitern.
- **Ticket-Abfrage** (`GET /api/v1/tickets`) — Support-Tickets des authentifizierten Anrufers.
- **AutomationIssue anlegen/pollen** (`POST /api/v1/issues`, `GET /api/v1/issues/{issueId}/status`) — löst die Reasoning-Engine aus.
- **Intent-Katalog** (`GET /api/v1/intents`) — listet die im Graphen gepflegten Voicebot-Intents.

Vollständige Endpunkt-Dokumentation inkl. Request/Response-Beispielen: **[docs/Manual.md](docs/Manual.md)**
Deployment (VPS via systemd, Docker-Image bauen/exportieren): **[docs/Deployment.md](docs/Deployment.md)**

## Quickstart (lokal)

Voraussetzungen: Go 1.26+, Zugriff auf die privaten `bitbucket.org/almatoag`-Module (SSH-Key hinterlegt), eine laufende Bardioc/HIRO-Instanz.

```bash
cp deployment/.env.template deployment/local.env
# deployment/local.env mit echten Werten befüllen (BARDIOC_*, JWT_SECRET, API_KEY)

set -a; source deployment/local.env; set +a
go run ./cmd
```

Server läuft danach auf `:8080` (API) und `:8081` (`/health`, Monitoring-Port).

## Tests

```bash
go build ./...
go test ./...
```

Alle Tests sind mock-basiert (kein Zugriff auf eine echte HIRO-Instanz nötig).

## Auth-Modell

Jeder Request braucht den Header `X-Api-Key: <geteiltes Secret>` (siehe `API_KEY` env var) — das ist die grobe "ist das überhaupt Aristech"-Absicherung vor der gesamten API. Zusätzlich verlangen `/tickets`, `/issues*` und `/intents` einen `Authorization: Bearer <JWT>`, den man über `GET /api/v1/auth` erhält. `/auth` und `/auth/match` selbst brauchen keinen Bearer (sie sind der Weg, um überhaupt einen zu bekommen).

## Postman

Vorbereitete Collection mit allen Endpunkten: `docs/hiro-aristech-api.postman_collection.json`.

## Projektstruktur

```
cmd/               Einstiegspunkt (main.go)
internal/api/      HTTP-Handler, Routing, Request/Response-Typen (huma)
internal/auth/     JWT-Ausstellung/-Prüfung
internal/bardioc/  Graph-Repositories (Person/Ticket/Issue/Intent-Queries gegen HIRO)
internal/identity/ Identitätsauflösung + Fuzzy-Name-Matching (Business-Logik)
internal/config/   Env-basierte Konfiguration
build/package/docker/  Dockerfile + exportierte Images
docs/superpowers/  Design-/Plan-Dokumente vergangener Features (lokal, nicht getrackt)
```
