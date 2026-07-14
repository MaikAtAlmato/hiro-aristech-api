# hiro-aristech-api — Deployment

## Konfiguration (Environment-Variablen)

Vollständige Liste in `deployment/.env.template` (kopieren nach `deployment/local.env` für lokale Entwicklung — diese Datei ist bewusst **nicht** in Git getrackt, da sie echte Secrets enthält).

| Variable | Pflicht | Standard | Beschreibung |
|----------|---------|----------|--------------|
| `BARDIOC_ENDPOINT` | ja | — | Hostname der HIRO/Bardioc-Instanz |
| `BARDIOC_USE_TLS` | nein | `true` | TLS für die Bardioc-Verbindung |
| `BARDIOC_USERNAME` | ja | — | Bardioc-Service-Account |
| `BARDIOC_PASSWORD` | ja | — | Bardioc-Service-Account-Passwort |
| `BARDIOC_CLIENT_ID` | ja | — | OAuth-Client-ID |
| `BARDIOC_CLIENT_SECRET` | ja | — | OAuth-Client-Secret |
| `BARDIOC_SCOPE` | ja | — | Graph-Scope (Organisation/Projekt-ID) |
| `JWT_SECRET` | ja | — | Signier-Secret für die von `/auth` ausgestellten Bearer-Token |
| `JWT_TTL` | nein | `15m` | Gültigkeitsdauer des Bearer-Tokens |
| `API_KEY` | ja | — | Geteiltes Secret, das Aristech in jedem Request als `X-Api-Key`-Header mitschicken muss |
| `HTTP_PORT` | nein | `8080` | Port der öffentlichen API |
| `MONITORING_PORT` | nein | `8081` | Port für `GET /health` |
| `LOG_LEVEL` | nein | `info` | `debug`/`info`/`warn`/`error` |

Weitere, seltener benötigte Bardioc-Tuning-Variablen (Connection-Pool-Größen, Timeouts, WebSocket-Parameter) sind in `bitbucket.org/almatoag/bardioc-go/config` dokumentiert — für den Regelbetrieb reichen die Defaults.

`API_KEY` und `JWT_SECRET` sollten zufällige, ausreichend lange Werte sein (z. B. GUID/UUID). Es gibt keine Rotation eingebaut — Wertänderung erfordert einen Neustart und Abstimmung mit Aristech (neuer `X-Api-Key` muss dort hinterlegt werden).

## Weg 1: Direktes Binary-Deployment

Der Service läuft nativ (kein Docker) als systemd-Unit auf der Linux-VPS. Deployment über `deployment/deploy.ps1` (PowerShell, von Windows aus):

```powershell
.\deployment\deploy.ps1
```

Das Skript:
1. Baut `linux/amd64` (`CGO_ENABLED=0`) aus `./cmd`.
2. Kopiert das Binary per `scp` auf die VPS (`/opt/hiro-aristech-api/hiro-aristech-api.new`).
3. Verschiebt es an seinen finalen Platz, setzt Owner/Rechte, startet den systemd-Service (`hiro-aristech-api`) neu.
4. Prüft `systemctl is-active` und ruft `GET /health` ab, um den erfolgreichen Neustart zu bestätigen.

Voraussetzungen: SSH-Key unter `$HOME\.ssh\id_starto_rsa` mit Zugriff auf die VPS (`root@87.106.222.185`), Go-Toolchain lokal installiert. Die eigentliche Konfiguration (`deployment/local.env`-Äquivalent) liegt bereits auf der VPS unter `/opt/hiro-aristech-api/` — das Skript deployt nur das Binary, keine Config.

Feste Werte im Skript (bei Bedarf anpassen): VPS-Host, SSH-Key-Pfad, Remote-Verzeichnis, Service-Name — siehe Kopf von `deployment/deploy.ps1`.

## Weg 2: Docker-Image

Falls der Service auf eigener Infrastruktur betrieben werden soll.

### Voraussetzungen

- Docker Desktop mit BuildKit (`DOCKER_BUILDKIT=1`).
- SSH-Zugriff auf die privaten `bitbucket.org/almatoag`-Module (der Build lädt sie live via `go build`, siehe unten).
- **Windows-Besonderheit:** Git-Bash/MinGW's `ssh-agent` erzeugt keinen Socket, den Docker Desktop in den Linux-Build-Container mounten kann. Stattdessen den Windows-nativen `OpenSSH Authentication Agent`-Dienst nutzen:

  ```powershell
  Get-Service ssh-agent   # sollte "Running" sein, sonst: Start-Service ssh-agent
  ssh-add "$HOME\.ssh\id_ed25519"
  ```

  Danach funktioniert `--ssh default` ohne expliziten Socket-Pfad, weil Docker Desktop den nativen Windows-Agent automatisch andockt.

### Image bauen

```powershell
$env:DOCKER_BUILDKIT = "1"
docker build -f build/package/docker/Dockerfile --ssh default -t hiro-aristech-api:vX.Y.Z .
```

Der Dockerfile (`build/package/docker/Dockerfile`) ist zweistufig:
- **Builder-Stage** (`golang:1.26.4-alpine3.23`): kompiliert statisch (`CGO_ENABLED=0`), lädt die privaten Bitbucket-Module per SSH-Mount.
- **Production-Stage** (`alpine:3.23`): nur das fertige Binary, non-root User (`appuser`, UID 1000), Datei-Rechte `555` (read+execute, kein write).

Image-Größe: ~17 MB komprimiert (~53 MB entpackt).

Die Go-Version im Builder-Image muss `go.mod`s `go`-Direktive (aktuell `1.26.4`) erfüllen oder höher sein, sonst schlägt der Build mit einer Toolchain-Versionsfehlermeldung fehl.

### Als `.tar` exportieren (für Versand ohne Registry)

```powershell
docker save hiro-aristech-api:vX.Y.Z -o build/package/docker/hiro-aristech-api-vX.Y.Z.tar
```

Empfänger lädt es mit:

```bash
docker load -i hiro-aristech-api-vX.Y.Z.tar
docker run -p 8080:8080 -p 8081:8081 --env-file .env hiro-aristech-api:vX.Y.Z
```

(`.env` mit denselben Variablen wie in der Tabelle oben befüllen.)

### Alternative: Registry statt `.tar`-Versand

Für laufenden Betrieb (statt einmaligem Versand) ist eine Registry sauberer:

- **Docker Hub**: kostenloser Account, `docker login`, `docker tag hiro-aristech-api:vX.Y.Z <user>/hiro-aristech-api:vX.Y.Z`, `docker push`. Repo auf `private` stellen, Aristech als Collaborator einladen (oder `public`, falls unproblematisch).
- **GitHub Container Registry (ghcr.io)**: Personal Access Token mit `write:packages`, `docker login ghcr.io`, Tag als `ghcr.io/<org>/hiro-aristech-api:vX.Y.Z`, push. Zugriff über GitHub-Package-Settings regeln.

In beiden Fällen zieht Aristech dann selbst per `docker pull`, statt eine Datei geschickt zu bekommen — sauberer bei Updates.

## Health-Check nach jedem Deployment

Egal welcher Weg: nach dem Start immer verifizieren, dass der Service tatsächlich läuft und die Bardioc-Verbindung steht (ein `200 {"status":"OK"}` von `/health` sagt nur, dass der Prozess lebt, prüft aber keine Bardioc-Konnektivität separat):

```bash
curl http://<host>:8081/health
```

Und ein echter End-to-End-Check gegen die API selbst (mit gültigem `X-Api-Key`):

```bash
curl -H "X-Api-Key: <secret>" "http://<host>:8080/api/v1/auth?phone=<bekannte-testnummer>"
```
