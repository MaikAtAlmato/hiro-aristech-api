# hiro-aristech-api — Manual

Vollständige Referenz aller HTTP-Endpunkte, der Auth-Mechanismen und der wichtigsten Business-Logik. Für Deployment siehe [Deployment.md](Deployment.md).

## Inhalt

- [Authentifizierung](#authentifizierung)
- [Fehlerformat](#fehlerformat)
- [Endpunkte](#endpunkte)
  - [GET /api/v1/auth](#get-apiv1auth)
  - [POST /api/v1/auth/match](#post-apiv1authmatch)
  - [GET /api/v1/tickets](#get-apiv1tickets)
  - [POST /api/v1/issues](#post-apiv1issues)
  - [GET /api/v1/issues/{issueId}/status](#get-apiv1issuesissueidstatus)
  - [GET /api/v1/intents](#get-apiv1intents)
  - [GET /health](#get-health)
- [Business-Logik im Detail](#business-logik-im-detail)
  - [Identitätsauflösung (/auth)](#identitätsauflösung-auth)
  - [Fuzzy-Name-Matching (/auth/match)](#fuzzy-name-matching-authmatch)
  - [Intent-Katalog und AutomationIssue-Erstellung](#intent-katalog-und-automationissue-erstellung)

## Authentifizierung

Zwei unabhängige Schichten, beide sind eigenständige Gates — keine ersetzt die andere:

1. **`X-Api-Key: <secret>`** — Pflicht-Header auf **jedem** Request an **jeden** Endpunkt (inkl. `/auth` und `/auth/match`, die sonst keinen Bearer brauchen). Fehlt er oder stimmt er nicht, kommt `401` mit `"invalid or missing API key"`, bevor überhaupt ein Handler ausgeführt wird. Wert kommt aus der `API_KEY` env var, Vergleich ist konstant-zeitig (Timing-Angriff-resistent).
2. **`Authorization: Bearer <JWT>`** — Pflicht auf `/tickets`, `/issues`, `/issues/{id}/status`, `/intents`. Man bekommt das Token von `GET /api/v1/auth`. Fehlt/ungültig → `401`.

`/health` (Monitoring-Port, separat von der API) braucht **keinen** der beiden Header — läuft auf einem eigenen `http.ServeMux`, nicht über die huma-Middleware.

## Fehlerformat

Alle Fehler kommen als huma-Standardfehler:

```json
{
  "status": 401,
  "title": "Unauthorized",
  "detail": "invalid or missing API key"
}
```

Status-Code-Konvention über alle Endpunkte hinweg:

| Code | Bedeutung |
|------|-----------|
| 400 | Ungültige/fehlende Eingabe (z. B. weder `phone` noch `name`) |
| 401 | Fehlender/ungültiger `X-Api-Key` oder `Authorization`-Bearer, oder kein passender Caller gefunden |
| 404 | Ressource nicht gefunden (z. B. unbekannte `intentId`, unbekannte `issueId`) |
| 409 | Mehrdeutiger Treffer (mehrere Personen passen, `/auth` kann sich nicht entscheiden) |
| 503 | Graph-Anfrage fehlgeschlagen (HIRO/Bardioc nicht erreichbar oder Query-Fehler) |
| 500 | Interner Fehler (z. B. Token-Ausstellung fehlgeschlagen) |

## Endpunkte

### `GET /api/v1/auth`

Authentifiziert einen Anrufer per Telefonnummer **oder** Namen (genau eines von beiden, nicht beides).

**Query-Parameter:**

| Feld | Pflicht | Beschreibung |
|------|---------|--------------|
| `phone` | einer von phone/name | z. B. `+491234567890` |
| `name` | einer von phone/name | Vollständiger Name, z. B. `Max Mustermann` |
| `domain` | optional | Komma-separierte E-Mail-Domains, schränkt Treffer ein, z. B. `almato.com,datagroup.com` |

Namens-Matching ist **exakt** (kein Fuzzy/Prefix), aber **case-insensitive** — "Max Mustermann", "MAX MUSTERMANN" und "max mustermann" matchen alle denselben gespeicherten Namen. Mehrteilige Vornamen werden nicht unterstützt (Split am ersten Leerzeichen: Vor- = erstes Wort, Nachname = Rest).

**Response `200`:**

```json
{
  "token": "eyJhbGciOi...",
  "expiresIn": 900,
  "name": "Max Mustermann",
  "valuemationExternalId": "vm-external-id",
  "msgraphExternalId": "ms-external-id",
  "msgraphAccountExternalId": "acct-external-id",
  "msgraphAccountStatus": "active"
}
```

Die `*ExternalId`/`msgraphAccount*`-Felder erscheinen nur, wenn die jeweilige Quelle einen Treffer/eine verknüpfte Account hatte.

**Fehler:** `400` (weder/beide Parameter gesetzt), `401` (kein Treffer), `409` (mehrdeutig — mehr als eine Person passt, oder die MSGraph- und Valuemation-Quelle sind sich uneinig), `503` (Graph-Fehler).

### `POST /api/v1/auth/match`

Pre-Check-Endpunkt für den Fall, dass die Spracherkennung des Namens unvollständig oder fehlerhaft sein könnte (z. B. nur ein Initial erkannt, oder ein Buchstabe verhört). Im Gegensatz zu `/auth` **fehlt nie mit 409** — liefert stattdessen bis zu 5 bewertete Kandidaten.

**Body:**

```json
{
  "firstName": { "resultRaw": "Maik", "resultNlp": "Maik" },
  "lastName": { "resultRaw": "Lander" },
  "domain": "almato.com",
  "tickerMessageId": "optional-log-correlation-id"
}
```

`firstName`/`lastName` sind STT-Ergebnis-Objekte mit bis zu 7 Feldern (`resultRaw`, `resultNlp`, `resultTagged`, `resultSlotted`, `resultStructured`, `resultLanguage`, `confidence`) — nur befüllen, was die Spracherkennung tatsächlich geliefert hat. Weitere Beispiel-Bodies: [docs/match-test.md](match-test.md).

**Matching-Ablauf** (Details siehe [unten](#fuzzy-name-matching-authmatch)):
1. Exaktes Prefix-Matching ("startet mit") über alle STT-Repräsentationstypen.
2. Findet das nichts, Fuzzy-Fallback per Editierdistanz — fängt Verhörer/Tippfehler ab, die reines Prefix-Matching nicht kann.

**Response `200`:**

```json
{
  "candidates": [
    {
      "name": "Maik Lander",
      "firstName": "Maik",
      "lastName": "Lander",
      "valuemationExternalId": "vm-1",
      "msgraphExternalId": "ms-1",
      "confidence": 100,
      "stagePoints": 50,
      "representationPoints": 30,
      "uniquenessPoints": 20
    }
  ]
}
```

`confidence = stagePoints + representationPoints + uniquenessPoints` (0-100). Leere Liste (`"candidates": []`) ist ein valides `200`, kein Fehler.

**Wichtig:** Ein Fuzzy-Treffer kann bis zu ~85 Confidence erreichen (nur 15 Punkte unter einem exakten Volltreffer) — **vor identitätsbezogenen Aktionen** (Ticket-Daten zeigen, Issue unter dieser Identität anlegen) sollte der Anrufer den Namen nochmal bestätigen, unabhängig vom Confidence-Wert.

**Fehler:** `400` (weder Vor- noch Nachname hatte irgendwelche STT-Daten), `503` (Graph-Fehler).

### `GET /api/v1/tickets`

Liefert die Valuemation-Support-Tickets des im Bearer-Token hinterlegten Anrufers.

**Header:** `Authorization: Bearer <JWT>` (aus `/auth`)

**Response `200`:**

```json
{
  "tickets": [
    {
      "id": "ticket-1",
      "subject": "VPN broken",
      "description": "...",
      "status": "OPEN",
      "priority": "2 MEDIUM",
      "createdAt": "2026-07-01T10:00:00Z",
      "closedAt": ""
    }
  ]
}
```

Leere Liste, falls der Anrufer keine Valuemation-Identität hat (z. B. nur über MSGraph aufgelöst) oder keine Tickets existieren.

**Fehler:** `401` (fehlender/ungültiger Bearer), `503`.

### `POST /api/v1/issues`

Legt ein `ogit/Automation/AutomationIssue` an, das die Reasoning-Engine aufgreift.

**Header:** `Authorization: Bearer <JWT>`

**Body:**

```json
{
  "intentId": "cln8sd29t...-intent-id",
  "subject": "Passwort zurücksetzen",
  "variables": {
    "/userMail": "anrufer@example.com",
    "/description": "..."
  },
  "originNode": "optional-graph-id",
  "scope": "optional-scope-override"
}
```

- `intentId` (optional): interne Graph-ID (`ogit/_id`) eines Intents aus `GET /api/v1/intents`. Wenn gesetzt, werden dessen feste Systemvariablen zuerst gemerged, dann `variables` (User-Werte gewinnen bei Konflikt).
- Ohne `intentId` ist `variables` das komplette Attribut-Set.
- `subject`: fällt zurück auf `variables["/subject"]`, dann auf `"Voicebot-Anliegen"`.
- `originNode`/`scope`: überschreiben gleichnamige Keys in `variables`, falls gesetzt.

**Response `200`:**

```json
{ "issueId": "cln8sd29t...-issue-id" }
```

**Fehler:** `401`, `404` (unbekannte `intentId`), `503`.

### `GET /api/v1/issues/{issueId}/status`

Pollt den aktuellen Status eines zuvor angelegten Issues.

**Header:** `Authorization: Bearer <JWT>`

**Response `200`:**

```json
{ "status": "PROCESSING" }
```

Mögliche Werte: `UNPROCESSED`, `PROCESSING`, `WAITING`, `RESOLVED`, `STOPPED`. `RESOLVED`/`STOPPED` sind Endzustände — dort aufhören zu pollen.

**Fehler:** `401`, `404` (unbekannte `issueId`), `503`.

### `GET /api/v1/intents`

Listet Intent-Knoten (`ogit/Automation/Intent`), optional gefiltert nach `/IntentType`.

**Header:** `Authorization: Bearer <JWT>`
**Query:** `intentType` (optional, komma-separiert, case-sensitive exakter Match, z. B. `Mainintents` oder `Mainintents,Subintents`). Leer/fehlend → alle Intents.

**Response `200`:**

```json
{
  "intents": [
    {
      "ogit/_id": "cln8sd29t...-intent-id",
      "ogit/description": "Führe diesen Intent aus, wenn...",
      "ogit/Automation/systemVariableNames": "/dont_process_ticket, /ParloaIntent",
      "ogit/Automation/systemVariableValues": "true, CreateTicket",
      "ogit/Automation/userVariables": "/userMail,/description",
      "/IntentType": "Mainintents",
      "/subject": "Template - Passwort zurücksetzen"
    }
  ]
}
```

Jeder Eintrag ist der **rohe** Graph-Knoten — jedes `ogit/*`- und `/`-präfigierte Feld wird unverändert durchgereicht, kein festes Schema. Das ist bewusst so: neue Felder am Intent-Knoten brauchen keine Code-Änderung hier.

**Fehler:** `401`, `503`.

### `GET /health`

Health-Check auf dem separaten Monitoring-Port (`MONITORING_PORT`, Standard `8081`). Kein `X-Api-Key`/Bearer nötig.

```json
{ "status": "OK", "components": [] }
```

## Business-Logik im Detail

### Identitätsauflösung (`/auth`)

Ein Anrufer kann in zwei unabhängigen Quellen existieren — MSGraph (Microsoft/AAD-synchronisierte Personen) und Valuemation (Support-Ticket-System). `/auth` fragt beide Quellen ab, gruppiert Duplikate innerhalb jeder Quelle (dieselbe Person kann mehrfach synchronisiert sein) und gleicht dann über Quellen hinweg ab:

1. **Beide Quellen leer** → `401` (kein Treffer).
2. **Nur eine Quelle hat genau einen Treffer(-Cluster)** → dieser wird genommen.
3. **Beide Quellen haben genau einen Treffer** → wird geprüft, ob's dieselbe Person ist (E-Mail, dann Telefonnummer, dann Name als Fallback-Vergleich). Ja → gemergte Identität mit beiden External-IDs. Nein → `409` (die Quellen widersprechen sich).
4. **Irgendeine Quelle hat mehr als einen Treffer-Cluster** → `409` (mehrdeutig), außer der `domain`-Parameter schränkt genug ein.

### Fuzzy-Name-Matching (`/auth/match`)

Läuft in zwei Stufen:

1. **Exaktes Prefix-Matching** — für jede STT-Repräsentation (raw/nlp/tagged/slotted/structured), die auf **beiden** Namensteilen Daten hat, wird case-insensitiv per "startet mit" gesucht. Ein Vorname-Initial wie `"M"` matcht `"Maik"`, `"Max"`, etc.
2. **Fuzzy-Fallback** — nur falls Schritt 1 über **alle** Repräsentationstypen hinweg **null** Kandidaten findet. Verkürzt den Suchpräfix auf die ersten 3 Zeichen pro Namensteil (Namensteile <3 Zeichen werden dabei übersprungen — zu unscharf), holt damit einen breiteren Kandidatenpool und filtert per Levenshtein-Distanz (Toleranz skaliert mit Namenslänge: `max(1, gerundet(Länge/4))`). Treffer aus diesem Fallback bekommen einen Abzug von 15 Punkten auf `stagePoints` (Floor bei 20), damit sie im Ranking sichtbar unter einem exakten Treffer gleicher Spezifität landen.

Scoring: `stagePoints` (0-50, wie spezifisch der Match war) + `representationPoints` (0-30, wie viele STT-Repräsentationen sich einig waren) + `uniquenessPoints` (0-20, wie eindeutig — sinkt bei mehreren gefundenen Personen).

### Intent-Katalog und AutomationIssue-Erstellung

Intents (`ogit/Automation/Intent`) haben **keine Graph-Kanten** — die Verknüpfung zu Variablen läuft ausschließlich über Namensgleichheit in den Attributfeldern (`ogit/Automation/systemVariableNames`/`Values`, `ogit/Automation/userVariables`). Empfehlung: Intents beim Bot-Start einmal laden und cachen (ändern sich selten), nicht bei jedem Anruf neu abfragen.

Beim Anlegen eines Issues über `POST /api/v1/issues` mit gesetzter `intentId` werden die festen Systemvariablen des Intents automatisch als `/`-präfigierte Attribute übernommen, User-Variablen kommen obendrauf und gewinnen bei Namenskonflikten.
