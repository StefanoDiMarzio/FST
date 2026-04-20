CREATE TABLE IF NOT EXISTS utenti (
  id_utente INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  nome VARCHAR(100) NOT NULL,
  cognome VARCHAR(100) NOT NULL,
  telefono VARCHAR(30) NULL,
  ruolo ENUM('registrato','backoffice_amm','backoffice_esercizio','admin') NOT NULL DEFAULT 'registrato',
  creato_il DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  attivo TINYINT(1) NOT NULL DEFAULT 1
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS stazioni (
  id_stazione TINYINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  nome VARCHAR(120) NOT NULL UNIQUE,
  progressiva_km DECIMAL(7,3) NOT NULL,
  ordine_linea TINYINT UNSIGNED NOT NULL UNIQUE,
  descrizione TEXT NULL,
  CONSTRAINT chk_stazioni_ordine CHECK (ordine_linea BETWEEN 1 AND 10),
  CONSTRAINT chk_stazioni_km CHECK (progressiva_km >= 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS sub_tratte (
  id_sub_tratta TINYINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_stazione_da TINYINT UNSIGNED NOT NULL,
  id_stazione_a TINYINT UNSIGNED NOT NULL,
  km DECIMAL(7,3) NOT NULL,
  ordine_linea TINYINT UNSIGNED NOT NULL UNIQUE,
  CONSTRAINT fk_sub_da FOREIGN KEY (id_stazione_da) REFERENCES stazioni(id_stazione),
  CONSTRAINT fk_sub_a FOREIGN KEY (id_stazione_a) REFERENCES stazioni(id_stazione),
  CONSTRAINT chk_sub_km CHECK (km > 0),
  CONSTRAINT chk_sub_ordine CHECK (ordine_linea BETWEEN 1 AND 9),
  CONSTRAINT chk_sub_stazioni_diverse CHECK (id_stazione_da <> id_stazione_a)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS materiale_rotabile (
  id_materiale SMALLINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  codice VARCHAR(30) NOT NULL UNIQUE,
  tipo ENUM('carrozza','bagagliaio','automotrice','locomotiva') NOT NULL,
  serie VARCHAR(50) NULL,
  nome VARCHAR(120) NULL,
  posti SMALLINT UNSIGNED NOT NULL DEFAULT 0,
  autonomo TINYINT(1) NOT NULL DEFAULT 0,
  attivo TINYINT(1) NOT NULL DEFAULT 1,
  note TEXT NULL,
  CONSTRAINT chk_materiale_posti CHECK (posti >= 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS posti (
  id_posto INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_materiale SMALLINT UNSIGNED NOT NULL,
  numero_posto SMALLINT UNSIGNED NOT NULL,
  tipo_posto ENUM('standard','finestrino','corridoio') NOT NULL DEFAULT 'standard',
  CONSTRAINT fk_posti_materiale FOREIGN KEY (id_materiale) REFERENCES materiale_rotabile(id_materiale),
  CONSTRAINT uq_posto_materiale UNIQUE (id_materiale, numero_posto),
  CONSTRAINT chk_numero_posto CHECK (numero_posto > 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS convogli (
  id_convoglio INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  codice VARCHAR(50) NOT NULL UNIQUE,
  descrizione VARCHAR(255) NULL,
  data_creazione DATE NOT NULL,
  attivo TINYINT(1) NOT NULL DEFAULT 1
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS convoglio_materiale (
  id_convoglio INT UNSIGNED NOT NULL,
  id_materiale SMALLINT UNSIGNED NOT NULL,
  posizione SMALLINT UNSIGNED NOT NULL,
  valido_dal DATE NOT NULL,
  valido_al DATE NULL,
  PRIMARY KEY (id_convoglio, id_materiale, valido_dal),
  CONSTRAINT fk_cm_convoglio FOREIGN KEY (id_convoglio) REFERENCES convogli(id_convoglio),
  CONSTRAINT fk_cm_materiale FOREIGN KEY (id_materiale) REFERENCES materiale_rotabile(id_materiale),
  CONSTRAINT uq_cm_posizione UNIQUE (id_convoglio, posizione, valido_dal),
  CONSTRAINT chk_cm_posizione CHECK (posizione > 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS corse (
  id_corsa INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_convoglio INT UNSIGNED NOT NULL,
  data_corsa DATE NOT NULL,
  direzione ENUM('andata','ritorno') NOT NULL,
  id_stazione_partenza TINYINT UNSIGNED NOT NULL,
  id_stazione_arrivo TINYINT UNSIGNED NOT NULL,
  ora_partenza TIME NOT NULL,
  ora_arrivo TIME NOT NULL,
  tipo_servizio ENUM('festivo','feriale','straordinario') NOT NULL,
  stato ENUM('programmata','in_servizio','completata','cancellata') NOT NULL DEFAULT 'programmata',
  CONSTRAINT fk_corse_convoglio FOREIGN KEY (id_convoglio) REFERENCES convogli(id_convoglio),
  CONSTRAINT fk_corse_staz_p FOREIGN KEY (id_stazione_partenza) REFERENCES stazioni(id_stazione),
  CONSTRAINT fk_corse_staz_a FOREIGN KEY (id_stazione_arrivo) REFERENCES stazioni(id_stazione),
  CONSTRAINT chk_corse_stazioni_diverse CHECK (id_stazione_partenza <> id_stazione_arrivo)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS fermate (
  id_fermata INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_corsa INT UNSIGNED NOT NULL,
  id_stazione TINYINT UNSIGNED NOT NULL,
  ordine_fermata TINYINT UNSIGNED NOT NULL,
  ora_arrivo TIME NULL,
  ora_partenza TIME NULL,
  CONSTRAINT fk_fermate_corsa FOREIGN KEY (id_corsa) REFERENCES corse(id_corsa),
  CONSTRAINT fk_fermate_stazione FOREIGN KEY (id_stazione) REFERENCES stazioni(id_stazione),
  CONSTRAINT uq_fermata_stazione UNIQUE (id_corsa, id_stazione),
  CONSTRAINT uq_fermata_ordine UNIQUE (id_corsa, ordine_fermata)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS occupazioni_sub_tratta (
  id_occupazione INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_corsa INT UNSIGNED NOT NULL,
  id_sub_tratta TINYINT UNSIGNED NOT NULL,
  inizio_occupazione DATETIME NOT NULL,
  fine_occupazione DATETIME NOT NULL,
  direzione ENUM('andata','ritorno') NOT NULL,
  CONSTRAINT fk_occ_corsa FOREIGN KEY (id_corsa) REFERENCES corse(id_corsa),
  CONSTRAINT fk_occ_sub FOREIGN KEY (id_sub_tratta) REFERENCES sub_tratte(id_sub_tratta),
  CONSTRAINT chk_occ_intervallo CHECK (fine_occupazione > inizio_occupazione)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS biglietti (
  id_biglietto INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  codice_biglietto VARCHAR(40) NOT NULL UNIQUE,
  id_utente INT UNSIGNED NOT NULL,
  id_corsa INT UNSIGNED NOT NULL,
  id_stazione_partenza TINYINT UNSIGNED NOT NULL,
  id_stazione_arrivo TINYINT UNSIGNED NOT NULL,
  km_viaggio DECIMAL(7,3) NOT NULL,
  prezzo DECIMAL(10,2) NOT NULL,
  stato ENUM('pagamento_in_attesa','confermato','cancellato','rimborsato') NOT NULL DEFAULT 'pagamento_in_attesa',
  acquistato_il DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_biglietti_utente FOREIGN KEY (id_utente) REFERENCES utenti(id_utente),
  CONSTRAINT fk_biglietti_corsa FOREIGN KEY (id_corsa) REFERENCES corse(id_corsa),
  CONSTRAINT fk_biglietti_staz_p FOREIGN KEY (id_stazione_partenza) REFERENCES stazioni(id_stazione),
  CONSTRAINT fk_biglietti_staz_a FOREIGN KEY (id_stazione_arrivo) REFERENCES stazioni(id_stazione),
  CONSTRAINT chk_biglietti_stazioni_diverse CHECK (id_stazione_partenza <> id_stazione_arrivo),
  CONSTRAINT chk_biglietti_km CHECK (km_viaggio > 0),
  CONSTRAINT chk_biglietti_prezzo CHECK (prezzo >= 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS prenotazioni (
  id_prenotazione INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_biglietto INT UNSIGNED NOT NULL UNIQUE,
  id_posto INT UNSIGNED NOT NULL,
  stato ENUM('attiva','modificata','annullata') NOT NULL DEFAULT 'attiva',
  creata_il DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  modificata_il DATETIME NULL,
  CONSTRAINT fk_pren_biglietto FOREIGN KEY (id_biglietto) REFERENCES biglietti(id_biglietto),
  CONSTRAINT fk_pren_posto FOREIGN KEY (id_posto) REFERENCES posti(id_posto)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS pagamenti (
  id_pagamento INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_biglietto INT UNSIGNED NOT NULL,
  provider VARCHAR(40) NOT NULL DEFAULT 'Pay Steam',
  transaction_id VARCHAR(120) NULL UNIQUE,
  importo DECIMAL(10,2) NOT NULL,
  stato ENUM('avviato','successo','fallito','annullato') NOT NULL DEFAULT 'avviato',
  payload_richiesta JSON NULL,
  payload_risposta JSON NULL,
  creato_il DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  aggiornato_il DATETIME NULL,
  CONSTRAINT fk_pag_biglietto FOREIGN KEY (id_biglietto) REFERENCES biglietti(id_biglietto),
  CONSTRAINT chk_pag_importo CHECK (importo >= 0)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS richieste_esercizio (
  id_richiesta INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  id_utente_richiedente INT UNSIGNED NOT NULL,
  tipo_richiesta ENUM('treno_straordinario','cessazione_treno','modifica_orario') NOT NULL,
  id_corsa INT UNSIGNED NULL,
  descrizione TEXT NOT NULL,
  stato ENUM('aperta','approvata','respinta','eseguita') NOT NULL DEFAULT 'aperta',
  creata_il DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  aggiornata_il DATETIME NULL,
  CONSTRAINT fk_rich_utente FOREIGN KEY (id_utente_richiedente) REFERENCES utenti(id_utente),
  CONSTRAINT fk_rich_corsa FOREIGN KEY (id_corsa) REFERENCES corse(id_corsa)
) ENGINE=InnoDB;

CREATE INDEX idx_corse_data_stato ON corse(data_corsa, stato);
CREATE INDEX idx_corse_tratta ON corse(id_stazione_partenza, id_stazione_arrivo, data_corsa);
CREATE INDEX idx_fermate_corsa_ordine ON fermate(id_corsa, ordine_fermata);
CREATE INDEX idx_biglietti_utente ON biglietti(id_utente, acquistato_il);
CREATE INDEX idx_biglietti_corsa_stato ON biglietti(id_corsa, stato);
CREATE INDEX idx_prenotazioni_posto_stato ON prenotazioni(id_posto, stato);
CREATE INDEX idx_pagamenti_biglietto_stato ON pagamenti(id_biglietto, stato);
CREATE INDEX idx_occ_sub_intervallo ON occupazioni_sub_tratta(id_sub_tratta, inizio_occupazione, fine_occupazione);
