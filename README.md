# FST API

API REST in Go per interrogare il database MySQL descritto in `database.sql.txt`.

Il servizio espone endpoint di consultazione per stazioni, tratte, materiale rotabile, convogli, corse, fermate, biglietti, pagamenti, occupazioni della linea e disponibilita dei posti.

## Requisiti

- Go 1.22 o superiore
- MySQL 8.0 o superiore
- Database creato con lo script `database.sql.txt`

## Configurazione

Il server legge la configurazione da variabili d'ambiente.

| Variabile | Default | Descrizione |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Indirizzo HTTP di ascolto |
| `DB_HOST` | `127.0.0.1` | Host MySQL |
| `DB_PORT` | `3306` | Porta MySQL |
| `DB_USER` | `root` | Utente MySQL |
| `DB_PASSWORD` | vuota | Password MySQL |
| `DB_NAME` | `sft_db` | Nome database |
| `DB_DSN` | generata dalle variabili sopra | DSN MySQL completa |
| `DB_MAX_OPEN_CONNS` | `25` | Connessioni massime aperte |
| `DB_MAX_IDLE_CONNS` | `25` | Connessioni massime idle |

Esempio PowerShell:

```powershell
$env:DB_USER="root"
$env:DB_PASSWORD="password"
$env:DB_HOST="127.0.0.1"
$env:DB_PORT="3306"
$env:DB_NAME="sft_db"
go run ./cmd/api
```

In alternativa si puo impostare direttamente `DB_DSN`:

```powershell
$env:DB_DSN="root:password@tcp(127.0.0.1:3306)/sft_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"
go run ./cmd/api
```

## Avvio

Installare le dipendenze e avviare il server:

```powershell
go mod tidy
go run ./cmd/api
```

Avvio completo con Docker Compose:

```powershell
.\scripts\devops.ps1 up
```

Verifica:

```powershell
curl http://localhost:8080/health
```

Risposta attesa:

```json
{"status":"ok"}
```

## DevOps Locale

Il repository include gli strumenti per eseguire l'intero stack in locale:

- `.github/workflows/devops.yml`: pipeline CI/CD GitHub Actions.
- `Dockerfile`: build multi-stage dell'API Go.
- `docker-compose.yml`: API, MySQL, Prometheus e Grafana.
- `.env.example`: variabili d'ambiente di riferimento.
- `docker/mysql/init/01-schema.sql`: schema SQL pulito usato all'avvio del database.
- `scripts/devops.ps1`: comandi operativi PowerShell.
- `scripts/devops.sh`: equivalente shell per ambienti Unix.
- `scripts/smoke-test.ps1`: controllo rapido dei servizi.

Comandi principali:

```powershell
.\scripts\devops.ps1 init
.\scripts\devops.ps1 build
.\scripts\devops.ps1 up
.\scripts\devops.ps1 status
.\scripts\devops.ps1 logs
.\scripts\devops.ps1 smoke
.\scripts\devops.ps1 down
.\scripts\devops.ps1 clean
```

Servizi esposti:

| Servizio | URL | Note |
| --- | --- | --- |
| API | `http://localhost:8080` | API REST FST |
| Health | `http://localhost:8080/health` | readiness applicativa e DB |
| Metrics | `http://localhost:8080/metrics` | metriche Prometheus |
| Prometheus | `http://localhost:9090` | raccolta metriche |
| Grafana | `http://localhost:3000` | dashboard, default `admin/admin` |
| MySQL | `localhost:3306` | database locale |

Per eliminare anche i volumi, quindi dati MySQL, Prometheus e Grafana:

```powershell
.\scripts\devops.ps1 clean
```

## CI/CD GitHub Actions

La pipeline [DevOps](.github/workflows/devops.yml) parte automaticamente su:

- `push` verso `main` o `master`;
- tag `v*`;
- `pull_request` verso `main` o `master`;
- avvio manuale da `workflow_dispatch`.

Job inclusi:

- `Go Test`: installa Go, verifica `go.mod`/`go.sum`, controlla `gofmt`, esegue `go test -race -cover ./...`.
- `Docker Build`: costruisce l'immagine dell'API con Buildx e la pubblica su GitHub Container Registry quando non e una pull request.
- `Compose Validation`: valida `docker-compose.yml`, avvia lo stack completo e fa smoke test su API, metriche, Prometheus e Grafana.
- `Observability Assets`: valida la configurazione Prometheus con `promtool` e il JSON della dashboard Grafana con `jq`.
- `Trivy Scan`: scansiona l'immagine Docker e pubblica i risultati SARIF nella sezione Security di GitHub.

Per pubblicare l'immagine su GHCR non servono secret aggiuntivi: usa `GITHUB_TOKEN` e il permesso `packages: write` definito nel workflow.

## Observability

L'API espone metriche Prometheus su `/metrics`.

Metriche custom principali:

- `fst_http_requests_total`: numero richieste per metodo, route e stato HTTP.
- `fst_http_request_duration_seconds`: istogramma delle latenze HTTP.
- `fst_http_requests_in_flight`: richieste in corso.
- `fst_db_open_connections`: connessioni database aperte.
- `fst_db_in_use_connections`: connessioni database in uso.
- `fst_db_idle_connections`: connessioni database inattive.
- `fst_db_wait_count_total`: attese per ottenere una connessione dal pool.

Prometheus raccoglie automaticamente le metriche dall'API. Grafana viene avviato con datasource Prometheus gia configurato e dashboard `FST API` provisionata in automatico.

Log:

```powershell
.\scripts\devops.ps1 logs
```

Le richieste HTTP vengono loggate su stdout con metodo, URL, status code e durata.

## Endpoint

Tutte le API restituiscono JSON. In caso di errore viene restituito:

```json
{"error":"messaggio errore"}
```

Per una spiegazione didattica di come e stata sviluppata l'API in Go, vedere [Programmare API in Go](docs/programmare-api-go.md).

Gli endpoint paginati accettano:

- `limit`: numero di record da restituire, default `50`, massimo `100`
- `offset`: numero di record da saltare, default `0`

La risposta degli endpoint paginati ha questa forma:

```json
{
  "data": [],
  "pagination": {
    "limit": 50,
    "offset": 0,
    "returned": 0
  }
}
```

### Health

```http
GET /health
```

Controlla che il server riesca a contattare il database.

### Utenti

```http
GET /api/v1/utenti
GET /api/v1/utenti?ruolo=registrato&attivo=true
```

Restituisce gli utenti senza esporre `password_hash`.

Filtri:

- `ruolo`: `registrato`, `backoffice_amm`, `backoffice_esercizio`, `admin`
- `attivo`: `true`, `false`, `1` oppure `0`

### Stazioni

```http
GET /api/v1/stazioni
```

Restituisce le stazioni ordinate per `ordine_linea`.

### Sub-tratte

```http
GET /api/v1/sub-tratte
```

Restituisce le sub-tratte con nome della stazione di partenza e arrivo.

### Materiale rotabile

```http
GET /api/v1/materiale-rotabile
GET /api/v1/materiale-rotabile?attivo=true
```

Filtri:

- `attivo`: `true`, `false`, `1` oppure `0`

Posti di un materiale rotabile:

```http
GET /api/v1/materiale-rotabile/{id}/posti
```

### Convogli

```http
GET /api/v1/convogli
GET /api/v1/convogli?attivo=true
```

Filtri:

- `attivo`: `true`, `false`, `1` oppure `0`

### Materiale di un convoglio

```http
GET /api/v1/convogli/{id}/materiale
GET /api/v1/convogli/{id}/materiale?data=2026-04-20
```

Restituisce la composizione del convoglio valida alla data richiesta. Se `data` non viene passata, usa la data corrente del server.

### Corse

```http
GET /api/v1/corse
GET /api/v1/corse?data=2026-04-20
GET /api/v1/corse?data=2026-04-20&stato=programmata&partenza=1&arrivo=5
```

Filtri:

- `data`: data corsa, formato `YYYY-MM-DD`
- `stato`: `programmata`, `in_servizio`, `completata`, `cancellata`
- `tipo_servizio`: `festivo`, `feriale`, `straordinario`
- `direzione`: `andata`, `ritorno`
- `partenza`: id stazione di partenza
- `arrivo`: id stazione di arrivo

Dettaglio corsa:

```http
GET /api/v1/corse/{id}
```

Fermate della corsa:

```http
GET /api/v1/corse/{id}/fermate
```

### Biglietti

Biglietti di una corsa:

```http
GET /api/v1/corse/{id}/biglietti
GET /api/v1/corse/{id}/biglietti?stato=confermato
GET /api/v1/corse/{id}/biglietti?limit=25&offset=50
```

Biglietti di un utente:

```http
GET /api/v1/utenti/{id}/biglietti
GET /api/v1/utenti/{id}/biglietti?stato=confermato
GET /api/v1/utenti/{id}/biglietti?limit=25&offset=50
```

Filtri:

- `stato`: `pagamento_in_attesa`, `confermato`, `cancellato`, `rimborsato`
- `limit`: record per pagina, default `50`, massimo `100`
- `offset`: record da saltare, default `0`

### Prenotazioni

```http
GET /api/v1/prenotazioni
GET /api/v1/prenotazioni?stato=attiva
GET /api/v1/prenotazioni?id_corsa=10
GET /api/v1/prenotazioni?limit=25&offset=50
```

Filtri:

- `stato`: `attiva`, `modificata`, `annullata`
- `id_biglietto`: id biglietto
- `id_posto`: id posto
- `id_corsa`: id corsa collegata tramite biglietto
- `limit`: record per pagina, default `50`, massimo `100`
- `offset`: record da saltare, default `0`

### Disponibilita posti

```http
GET /api/v1/posti/disponibili?id_corsa=10&id_stazione_partenza=1&id_stazione_arrivo=4
```

Parametri obbligatori:

- `id_corsa`
- `id_stazione_partenza`
- `id_stazione_arrivo`

L'endpoint restituisce i posti del convoglio associato alla corsa, escludendo le prenotazioni attive su biglietti confermati con intervallo di viaggio sovrapposto. La logica usa l'ordine delle stazioni sulla linea, come indicato nello schema SQL.

### Occupazioni sub-tratta

```http
GET /api/v1/occupazioni
GET /api/v1/occupazioni?data=2026-04-20
GET /api/v1/occupazioni?id_sub_tratta=3&direzione=andata
```

Filtri:

- `data`: data occupazione, formato `YYYY-MM-DD`
- `id_sub_tratta`: id della sub-tratta
- `direzione`: `andata` oppure `ritorno`

### Richieste esercizio

```http
GET /api/v1/richieste-esercizio
GET /api/v1/richieste-esercizio?stato=aperta
GET /api/v1/richieste-esercizio?tipo_richiesta=treno_straordinario
```

Filtri:

- `stato`: `aperta`, `approvata`, `respinta`, `eseguita`
- `tipo_richiesta`: `treno_straordinario`, `cessazione_treno`, `modifica_orario`
- `id_corsa`: id corsa collegata

### Pagamenti

```http
GET /api/v1/pagamenti
GET /api/v1/pagamenti?stato=successo
GET /api/v1/pagamenti?id_biglietto=12
GET /api/v1/pagamenti?limit=25&offset=50
```

Filtri:

- `stato`: `avviato`, `successo`, `fallito`, `annullato`
- `id_biglietto`: id del biglietto
- `limit`: record per pagina, default `50`, massimo `100`
- `offset`: record da saltare, default `0`

## Struttura progetto

```text
.
|-- cmd/
|   `-- api/
|       `-- main.go
|-- .github/
|   `-- workflows/
|       `-- devops.yml
|-- docker/
|   `-- mysql/
|       `-- init/
|           `-- 01-schema.sql
|-- docs/
|   `-- programmare-api-go.md
|-- observability/
|   |-- grafana/
|   `-- prometheus/
|-- scripts/
|-- database.sql.txt
|-- docker-compose.yml
|-- Dockerfile
|-- go.mod
|-- go.sum
`-- README.md
```

## Note di sicurezza

Questa versione implementa API di sola interrogazione e non include autenticazione/autorizzazione. Prima dell'uso in produzione e consigliato aggiungere:

- autenticazione JWT o sessioni server-side;
- autorizzazione basata sul campo `utenti.ruolo`;
- logging strutturato;
- rate limiting;
- endpoint di scrittura separati con transazioni esplicite.
