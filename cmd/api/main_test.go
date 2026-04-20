package main

import (
	"os"
	"testing"
)

func TestEnvWithValue(t *testing.T) {
	// Setup: imposta variabile d'ambiente
	os.Setenv("TEST_KEY", "test_value")
	defer os.Unsetenv("TEST_KEY")

	// Test: leggi il valore
	result := env("TEST_KEY", "fallback")

	// Verifica: deve essere "test_value", non "fallback"
	if result != "test_value" {
		t.Errorf("env() dovrebbe tornare 'test_value', invece ha tornato '%s'", result)
	}
}

func TestEnvWithoutValue(t *testing.T) {
	// Setup: assicurati che la variabile non esista
	os.Unsetenv("NONEXISTENT_KEY")

	// Test: leggi con fallback
	result := env("NONEXISTENT_KEY", "fallback_value")

	// Verifica: deve essere il fallback
	if result != "fallback_value" {
		t.Errorf("env() dovrebbe tornare 'fallback_value', invece ha tornato '%s'", result)
	}
}

func TestEnvIntWithValue(t *testing.T) {
	// Setup: imposta variabile numerica
	os.Setenv("TEST_INT", "42")
	defer os.Unsetenv("TEST_INT")

	// Test: leggi il valore
	result := envInt("TEST_INT", 100)

	// Verifica: deve essere 42
	if result != 42 {
		t.Errorf("envInt() dovrebbe tornare 42, invece ha tornato %d", result)
	}
}

func TestEnvIntWithoutValue(t *testing.T) {
	// Setup: assicurati che la variabile non esista
	os.Unsetenv("NONEXISTENT_INT")

	// Test: leggi con fallback
	result := envInt("NONEXISTENT_INT", 99)

	// Verifica: deve essere il fallback
	if result != 99 {
		t.Errorf("envInt() dovrebbe tornare 99, invece ha tornato %d", result)
	}
}

func TestEnvIntWithInvalidValue(t *testing.T) {
	// Setup: imposta variabile NON numerica
	os.Setenv("TEST_BAD_INT", "not_a_number")
	defer os.Unsetenv("TEST_BAD_INT")

	// Test: leggi il valore (dovrebbe fallire e usare fallback)
	result := envInt("TEST_BAD_INT", 77)

	// Verifica: deve tornare il fallback perché il valore non è un numero
	if result != 77 {
		t.Errorf("envInt() dovrebbe tornare fallback 77 per valore non numerico, invece ha tornato %d", result)
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Setup: pulisci variabili
	os.Unsetenv("DB_DSN")
	os.Unsetenv("HTTP_ADDR")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")

	// Test: leggi configurazione con tutti i default
	cfg := configFromEnv()

	// Verifica: controlli sui valori di default
	if cfg.addr != ":8080" {
		t.Errorf("addr dovrebbe essere ':8080', invece è '%s'", cfg.addr)
	}

	if cfg.maxOpenConns != 25 {
		t.Errorf("maxOpenConns dovrebbe essere 25, invece è %d", cfg.maxOpenConns)
	}

	if cfg.maxIdleConns != 25 {
		t.Errorf("maxIdleConns dovrebbe essere 25, invece è %d", cfg.maxIdleConns)
	}
}
