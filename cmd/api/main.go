package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type app struct {
	db *sql.DB
}

type apiError struct {
	Error string `json:"error"`
}

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fst_http_requests_total",
			Help: "Numero totale di richieste HTTP ricevute.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "fst_http_request_duration_seconds",
			Help:    "Durata delle richieste HTTP in secondi.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fst_http_requests_in_flight",
			Help: "Numero di richieste HTTP in corso.",
		},
	)
)

func main() {
	cfg := configFromEnv()

	db, err := sql.Open("mysql", cfg.dsn)
	if err != nil {
		log.Fatalf("database open: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.maxOpenConns)
	db.SetMaxIdleConns(cfg.maxIdleConns)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("database ping: %v", err)
	}

	a := &app{db: db}
	registerDBMetrics(db)
	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("FST API in ascolto su %s", cfg.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
}

type config struct {
	addr         string
	dsn          string
	maxOpenConns int
	maxIdleConns int
}

func configFromEnv() config {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		user := env("DB_USER", "root")
		pass := os.Getenv("DB_PASSWORD")
		host := env("DB_HOST", "127.0.0.1")
		port := env("DB_PORT", "3306")
		name := env("DB_NAME", "sft_db")
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci", user, pass, host, port, name)
	}

	return config{
		addr:         env("HTTP_ADDR", ":8080"),
		dsn:          dsn,
		maxOpenConns: envInt("DB_MAX_OPEN_CONNS", 25),
		maxIdleConns: envInt("DB_MAX_IDLE_CONNS", 25),
	}
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /api/v1/utenti", a.listUtenti)
	mux.HandleFunc("GET /api/v1/stazioni", a.listStazioni)
	mux.HandleFunc("GET /api/v1/sub-tratte", a.listSubTratte)
	mux.HandleFunc("GET /api/v1/materiale-rotabile", a.listMaterialeRotabile)
	mux.HandleFunc("GET /api/v1/materiale-rotabile/{id}/posti", a.listPostiMateriale)
	mux.HandleFunc("GET /api/v1/convogli", a.listConvogli)
	mux.HandleFunc("GET /api/v1/convogli/{id}/materiale", a.listMaterialeConvoglio)
	mux.HandleFunc("GET /api/v1/corse", a.listCorse)
	mux.HandleFunc("GET /api/v1/corse/{id}", a.getCorsa)
	mux.HandleFunc("GET /api/v1/corse/{id}/fermate", a.listFermateCorsa)
	mux.HandleFunc("GET /api/v1/corse/{id}/biglietti", a.listBigliettiCorsa)
	mux.HandleFunc("GET /api/v1/utenti/{id}/biglietti", a.listBigliettiUtente)
	mux.HandleFunc("GET /api/v1/prenotazioni", a.listPrenotazioni)
	mux.HandleFunc("GET /api/v1/posti/disponibili", a.listPostiDisponibili)
	mux.HandleFunc("GET /api/v1/occupazioni", a.listOccupazioni)
	mux.HandleFunc("GET /api/v1/richieste-esercizio", a.listRichiesteEsercizio)
	mux.HandleFunc("GET /api/v1/pagamenti", a.listPagamenti)
	return recoverMiddleware(metricsMiddleware(logMiddleware(mux)))
}

func (a *app) listUtenti(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id_utente, email, nome, cognome, telefono, ruolo, creato_il, attivo
		FROM utenti`
	filters := newFilters(" WHERE ")
	filters.addString(r, "ruolo", "ruolo = ?")
	filters.addBool(r, "attivo", "attivo = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY cognome, nome, id_utente"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database non disponibile")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *app) listStazioni(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id_stazione, nome, progressiva_km, ordine_linea, descrizione
		FROM stazioni
		ORDER BY ordine_linea`
	a.queryRows(w, r, query)
}

func (a *app) listSubTratte(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT st.id_sub_tratta,
		       st.id_stazione_da,
		       da.nome AS stazione_da,
		       st.id_stazione_a,
		       a.nome AS stazione_a,
		       st.km,
		       st.ordine_linea
		FROM sub_tratte st
		JOIN stazioni da ON da.id_stazione = st.id_stazione_da
		JOIN stazioni a ON a.id_stazione = st.id_stazione_a
		ORDER BY st.ordine_linea`
	a.queryRows(w, r, query)
}

func (a *app) listMaterialeRotabile(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id_materiale, codice, tipo, serie, nome, posti, autonomo, attivo, note
		FROM materiale_rotabile`
	filters := newFilters(" WHERE ")
	filters.addBool(r, "attivo", "attivo = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY codice"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) listPostiMateriale(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	query := `
		SELECT p.id_posto,
		       p.id_materiale,
		       mr.codice AS codice_materiale,
		       p.numero_posto,
		       p.tipo_posto
		FROM posti p
		JOIN materiale_rotabile mr ON mr.id_materiale = p.id_materiale
		WHERE p.id_materiale = ?
		ORDER BY p.numero_posto`
	a.queryRows(w, r, query, id)
}

func (a *app) listConvogli(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id_convoglio, codice, descrizione, data_creazione, attivo
		FROM convogli`
	filters := newFilters(" WHERE ")
	filters.addBool(r, "attivo", "attivo = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY codice"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) listMaterialeConvoglio(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	data := r.URL.Query().Get("data")
	if data == "" {
		data = time.Now().Format(time.DateOnly)
	}
	if !validDate(data) {
		writeError(w, http.StatusBadRequest, "parametro data non valido: usare YYYY-MM-DD")
		return
	}

	query := `
		SELECT cm.id_convoglio,
		       c.codice AS codice_convoglio,
		       cm.id_materiale,
		       mr.codice AS codice_materiale,
		       mr.tipo,
		       mr.serie,
		       mr.nome,
		       mr.posti,
		       cm.posizione,
		       cm.valido_dal,
		       cm.valido_al
		FROM convoglio_materiale cm
		JOIN convogli c ON c.id_convoglio = cm.id_convoglio
		JOIN materiale_rotabile mr ON mr.id_materiale = cm.id_materiale
		WHERE cm.id_convoglio = ?
		  AND cm.valido_dal <= ?
		  AND (cm.valido_al IS NULL OR cm.valido_al >= ?)
		ORDER BY cm.posizione`
	a.queryRows(w, r, query, id, data, data)
}

func (a *app) listCorse(w http.ResponseWriter, r *http.Request) {
	query := baseCorseQuery()
	filters := newFilters(" WHERE ")
	filters.addDate(r, "data", "co.data_corsa = ?")
	filters.addString(r, "stato", "co.stato = ?")
	filters.addString(r, "tipo_servizio", "co.tipo_servizio = ?")
	filters.addString(r, "direzione", "co.direzione = ?")
	filters.addInt(r, "partenza", "co.id_stazione_partenza = ?")
	filters.addInt(r, "arrivo", "co.id_stazione_arrivo = ?")
	if err := filters.err; err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	query += filters.sql() + " ORDER BY co.data_corsa, co.ora_partenza, co.id_corsa"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) getCorsa(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	query := baseCorseQuery() + " WHERE co.id_corsa = ?"
	a.queryOne(w, r, query, id)
}

func baseCorseQuery() string {
	return `
		SELECT co.id_corsa,
		       co.id_convoglio,
		       cv.codice AS codice_convoglio,
		       co.data_corsa,
		       co.direzione,
		       co.id_stazione_partenza,
		       sp.nome AS stazione_partenza,
		       co.id_stazione_arrivo,
		       sa.nome AS stazione_arrivo,
		       co.ora_partenza,
		       co.ora_arrivo,
		       co.tipo_servizio,
		       co.stato
		FROM corse co
		JOIN convogli cv ON cv.id_convoglio = co.id_convoglio
		JOIN stazioni sp ON sp.id_stazione = co.id_stazione_partenza
		JOIN stazioni sa ON sa.id_stazione = co.id_stazione_arrivo`
}

func (a *app) listFermateCorsa(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	query := `
		SELECT f.id_fermata,
		       f.id_corsa,
		       f.id_stazione,
		       s.nome AS stazione,
		       f.ordine_fermata,
		       f.ora_arrivo,
		       f.ora_partenza
		FROM fermate f
		JOIN stazioni s ON s.id_stazione = f.id_stazione
		WHERE f.id_corsa = ?
		ORDER BY f.ordine_fermata`
	a.queryRows(w, r, query, id)
}

func (a *app) listBigliettiCorsa(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	query, args, err := bigliettiQuery(r, "b.id_corsa = ?", id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.queryPaginatedRows(w, r, query, args...)
}

func (a *app) listBigliettiUtente(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	query, args, err := bigliettiQuery(r, "b.id_utente = ?", id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.queryPaginatedRows(w, r, query, args...)
}

func (a *app) listPrenotazioni(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT pr.id_prenotazione,
		       pr.id_biglietto,
		       b.codice_biglietto,
		       b.id_corsa,
		       pr.id_posto,
		       p.numero_posto,
		       p.tipo_posto,
		       mr.id_materiale,
		       mr.codice AS codice_materiale,
		       pr.stato,
		       pr.creata_il,
		       pr.modificata_il
		FROM prenotazioni pr
		JOIN biglietti b ON b.id_biglietto = pr.id_biglietto
		JOIN posti p ON p.id_posto = pr.id_posto
		JOIN materiale_rotabile mr ON mr.id_materiale = p.id_materiale`
	filters := newFilters(" WHERE ")
	filters.addString(r, "stato", "pr.stato = ?")
	filters.addInt(r, "id_biglietto", "pr.id_biglietto = ?")
	filters.addInt(r, "id_posto", "pr.id_posto = ?")
	filters.addInt(r, "id_corsa", "b.id_corsa = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY pr.creata_il DESC, pr.id_prenotazione DESC"
	a.queryPaginatedRows(w, r, query, filters.args...)
}

func bigliettiQuery(r *http.Request, requiredCondition string, requiredArg any) (string, []any, error) {
	query := `
		SELECT b.id_biglietto,
		       b.codice_biglietto,
		       b.id_utente,
		       CONCAT(u.nome, ' ', u.cognome) AS utente,
		       b.id_corsa,
		       b.id_stazione_partenza,
		       sp.nome AS stazione_partenza,
		       b.id_stazione_arrivo,
		       sa.nome AS stazione_arrivo,
		       b.km_viaggio,
		       b.prezzo,
		       b.stato,
		       b.acquistato_il
		FROM biglietti b
		JOIN utenti u ON u.id_utente = b.id_utente
		JOIN stazioni sp ON sp.id_stazione = b.id_stazione_partenza
		JOIN stazioni sa ON sa.id_stazione = b.id_stazione_arrivo`
	filters := newFilters(" WHERE ")
	filters.addRaw(requiredCondition, requiredArg)
	filters.addString(r, "stato", "b.stato = ?")
	if filters.err != nil {
		return "", nil, filters.err
	}
	query += filters.sql() + " ORDER BY b.acquistato_il DESC, b.id_biglietto DESC"
	return query, filters.args, nil
}

func (a *app) listPostiDisponibili(w http.ResponseWriter, r *http.Request) {
	idCorsa, err := requiredInt(r, "id_corsa")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	idPartenza, err := requiredInt(r, "id_stazione_partenza")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	idArrivo, err := requiredInt(r, "id_stazione_arrivo")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	query := `
		SELECT p.id_posto,
		       p.id_materiale,
		       mr.codice AS codice_materiale,
		       mr.tipo AS tipo_materiale,
		       p.numero_posto,
		       p.tipo_posto,
		       cm.posizione AS posizione_materiale
		FROM corse c
		JOIN convoglio_materiale cm
		  ON cm.id_convoglio = c.id_convoglio
		 AND cm.valido_dal <= c.data_corsa
		 AND (cm.valido_al IS NULL OR cm.valido_al >= c.data_corsa)
		JOIN materiale_rotabile mr ON mr.id_materiale = cm.id_materiale
		JOIN posti p ON p.id_materiale = mr.id_materiale
		WHERE c.id_corsa = ?
		  AND NOT EXISTS (
		    SELECT 1
		    FROM prenotazioni pr
		    JOIN biglietti b ON b.id_biglietto = pr.id_biglietto
		    JOIN stazioni bp ON bp.id_stazione = b.id_stazione_partenza
		    JOIN stazioni ba ON ba.id_stazione = b.id_stazione_arrivo
		    JOIN stazioni np ON np.id_stazione = ?
		    JOIN stazioni na ON na.id_stazione = ?
		    WHERE pr.id_posto = p.id_posto
		      AND b.id_corsa = c.id_corsa
		      AND pr.stato = 'attiva'
		      AND b.stato = 'confermato'
		      AND LEAST(bp.ordine_linea, ba.ordine_linea) < GREATEST(np.ordine_linea, na.ordine_linea)
		      AND GREATEST(bp.ordine_linea, ba.ordine_linea) > LEAST(np.ordine_linea, na.ordine_linea)
		  )
		ORDER BY cm.posizione, p.numero_posto`
	a.queryRows(w, r, query, idCorsa, idPartenza, idArrivo)
}

func (a *app) listOccupazioni(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT o.id_occupazione,
		       o.id_corsa,
		       o.id_sub_tratta,
		       da.nome AS stazione_da,
		       sa.nome AS stazione_a,
		       o.inizio_occupazione,
		       o.fine_occupazione,
		       o.direzione
		FROM occupazioni_sub_tratta o
		JOIN sub_tratte st ON st.id_sub_tratta = o.id_sub_tratta
		JOIN stazioni da ON da.id_stazione = st.id_stazione_da
		JOIN stazioni sa ON sa.id_stazione = st.id_stazione_a`
	filters := newFilters(" WHERE ")
	filters.addInt(r, "id_sub_tratta", "o.id_sub_tratta = ?")
	filters.addString(r, "direzione", "o.direzione = ?")
	if data := r.URL.Query().Get("data"); data != "" {
		if !validDate(data) {
			writeError(w, http.StatusBadRequest, "parametro data non valido: usare YYYY-MM-DD")
			return
		}
		filters.addRaw("DATE(o.inizio_occupazione) = ?", data)
	}
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY o.inizio_occupazione"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) listRichiesteEsercizio(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT r.id_richiesta,
		       r.id_utente_richiedente,
		       CONCAT(u.nome, ' ', u.cognome) AS richiedente,
		       r.tipo_richiesta,
		       r.id_corsa,
		       r.descrizione,
		       r.stato,
		       r.creata_il,
		       r.aggiornata_il
		FROM richieste_esercizio r
		JOIN utenti u ON u.id_utente = r.id_utente_richiedente`
	filters := newFilters(" WHERE ")
	filters.addString(r, "stato", "r.stato = ?")
	filters.addString(r, "tipo_richiesta", "r.tipo_richiesta = ?")
	filters.addInt(r, "id_corsa", "r.id_corsa = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY r.creata_il DESC"
	a.queryRows(w, r, query, filters.args...)
}

func (a *app) listPagamenti(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT p.id_pagamento,
		       p.id_biglietto,
		       b.codice_biglietto,
		       p.provider,
		       p.transaction_id,
		       p.importo,
		       p.stato,
		       p.creato_il,
		       p.aggiornato_il
		FROM pagamenti p
		JOIN biglietti b ON b.id_biglietto = p.id_biglietto`
	filters := newFilters(" WHERE ")
	filters.addString(r, "stato", "p.stato = ?")
	filters.addInt(r, "id_biglietto", "p.id_biglietto = ?")
	if filters.err != nil {
		writeError(w, http.StatusBadRequest, filters.err.Error())
		return
	}
	query += filters.sql() + " ORDER BY p.creato_il DESC, p.id_pagamento DESC"
	a.queryPaginatedRows(w, r, query, filters.args...)
}

func (a *app) queryRows(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore interrogazione database")
		log.Printf("query rows: %v", err)
		return
	}
	defer rows.Close()

	data, err := rowsToMaps(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore lettura risultati")
		log.Printf("scan rows: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (a *app) queryPaginatedRows(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	p, err := paginationFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	paginatedQuery := query + " LIMIT ? OFFSET ?"
	paginatedArgs := append(append([]any{}, args...), p.Limit, p.Offset)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, paginatedQuery, paginatedArgs...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore interrogazione database")
		log.Printf("query paginated rows: %v", err)
		return
	}
	defer rows.Close()

	data, err := rowsToMaps(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore lettura risultati")
		log.Printf("scan paginated rows: %v", err)
		return
	}

	p.Returned = len(data)
	writeJSON(w, http.StatusOK, paginatedResponse{
		Data:       data,
		Pagination: p,
	})
}

func (a *app) queryOne(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore interrogazione database")
		log.Printf("query one: %v", err)
		return
	}
	defer rows.Close()

	data, err := rowsToMaps(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "errore lettura risultati")
		log.Printf("scan one: %v", err)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusNotFound, "risorsa non trovata")
		return
	}
	writeJSON(w, http.StatusOK, data[0])
}

type paginatedResponse struct {
	Data       []map[string]any `json:"data"`
	Pagination pagination       `json:"pagination"`
}

type pagination struct {
	Limit    int `json:"limit"`
	Offset   int `json:"offset"`
	Returned int `json:"returned"`
}

func paginationFromRequest(r *http.Request) (pagination, error) {
	p := pagination{
		Limit:  50,
		Offset: 0,
	}

	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return pagination{}, fmt.Errorf("parametro limit non valido")
		}
		if parsed > 100 {
			return pagination{}, fmt.Errorf("parametro limit non valido: massimo 100")
		}
		p.Limit = parsed
	}

	if value := r.URL.Query().Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return pagination{}, fmt.Errorf("parametro offset non valido")
		}
		p.Offset = parsed
	}

	return p, nil
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		item := make(map[string]any, len(cols))
		for i, col := range cols {
			item[col] = normalizeDBValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func normalizeDBValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
			return v.Format(time.DateOnly)
		}
		return v.Format(time.RFC3339)
	default:
		return v
	}
}

type filters struct {
	prefix     string
	conditions []string
	args       []any
	err        error
}

func newFilters(prefix string) *filters {
	return &filters{prefix: prefix}
}

func (f *filters) addRaw(condition string, arg any) {
	f.conditions = append(f.conditions, condition)
	f.args = append(f.args, arg)
}

func (f *filters) addString(r *http.Request, name string, condition string) {
	value := r.URL.Query().Get(name)
	if value == "" || f.err != nil {
		return
	}
	f.addRaw(condition, value)
}

func (f *filters) addInt(r *http.Request, name string, condition string) {
	value := r.URL.Query().Get(name)
	if value == "" || f.err != nil {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		f.err = fmt.Errorf("parametro %s non valido", name)
		return
	}
	f.addRaw(condition, parsed)
}

func (f *filters) addDate(r *http.Request, name string, condition string) {
	value := r.URL.Query().Get(name)
	if value == "" || f.err != nil {
		return
	}
	if !validDate(value) {
		f.err = fmt.Errorf("parametro %s non valido: usare YYYY-MM-DD", name)
		return
	}
	f.addRaw(condition, value)
}

func (f *filters) addBool(r *http.Request, name string, condition string) {
	value := r.URL.Query().Get(name)
	if value == "" || f.err != nil {
		return
	}
	parsed, err := parseBoolInt(value)
	if err != nil {
		f.err = fmt.Errorf("parametro %s non valido: usare true/false oppure 1/0", name)
		return
	}
	f.addRaw(condition, parsed)
}

func (f *filters) sql() string {
	if len(f.conditions) == 0 {
		return ""
	}
	return f.prefix + strings.Join(f.conditions, " AND ")
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	value := r.PathValue(name)
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id non valido")
		return 0, false
	}
	return id, true
}

func requiredInt(r *http.Request, name string) (int, error) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0, fmt.Errorf("parametro %s obbligatorio", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("parametro %s non valido", name)
	}
	return parsed, nil
}

func parseBoolInt(value string) (int, error) {
	switch strings.ToLower(value) {
	case "1", "true", "t", "yes", "y":
		return 1, nil
	case "0", "false", "f", "no", "n":
		return 0, nil
	default:
		return 0, fmt.Errorf("invalid boolean")
	}
}

func validDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusResponseWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), status, time.Since(start).Round(time.Millisecond))
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusResponseWriter{ResponseWriter: w}

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		next.ServeHTTP(sw, r)

		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		labels := prometheus.Labels{
			"method": r.Method,
			"path":   routePattern(r),
			"status": strconv.Itoa(status),
		}
		httpRequestsTotal.With(labels).Inc()
		httpRequestDuration.With(labels).Observe(time.Since(start).Seconds())
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				writeError(w, http.StatusInternalServerError, "errore interno")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func routePattern(r *http.Request) string {
	// In Go 1.22, il pattern non è direttamente accessibile
	// Usiamo il path come fallback
	return r.URL.Path
}

func registerDBMetrics(db *sql.DB) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "fst_db_open_connections",
		Help: "Numero di connessioni database aperte.",
	}, func() float64 {
		return float64(db.Stats().OpenConnections)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "fst_db_in_use_connections",
		Help: "Numero di connessioni database in uso.",
	}, func() float64 {
		return float64(db.Stats().InUse)
	})
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "fst_db_idle_connections",
		Help: "Numero di connessioni database inattive.",
	}, func() float64 {
		return float64(db.Stats().Idle)
	})
	promauto.NewCounterFunc(prometheus.CounterOpts{
		Name: "fst_db_wait_count_total",
		Help: "Numero totale di attese per ottenere una connessione database.",
	}, func() float64 {
		return float64(db.Stats().WaitCount)
	})
}
