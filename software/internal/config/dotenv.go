// Package config carga configuración local desde un archivo .env, para que las
// credenciales (App Password de Gmail, etc.) vivan FUERA del repo (CLAUDE.md §4,
// spec §7.6). El archivo software/.env está en .gitignore y nunca se commitea.
package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv lee un archivo tipo .env (líneas KEY=VALUE) y define en el entorno
// del proceso las variables que aún NO estén definidas (las variables reales del
// entorno tienen prioridad, para poder sobreescribir en producción sin tocar el
// archivo). Ignora líneas en blanco y comentarios (#). No falla si el archivo no
// existe: devuelve (false, nil) para que el llamador decida si es obligatorio.
func LoadDotEnv(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		key, val, ok := strings.Cut(s, "=")
		if !ok {
			return true, fmt.Errorf("%s:%d: línea sin '=': %q", path, line, s)
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return true, err
			}
		}
	}
	return true, sc.Err()
}

// IMAPConfig son las credenciales de conexión al buzón de conciliación (grabibot).
type IMAPConfig struct {
	Host string
	Port string
	User string
	Pass string
}

// Addr devuelve "host:port" para el cliente IMAP.
func (c IMAPConfig) Addr() string { return c.Host + ":" + c.Port }

// LoadIMAP arma la configuración IMAP desde las variables GRABI_IMAP_*. Devuelve
// error si falta alguna (las credenciales solo vienen del entorno/.env; nunca del
// repo ni de argumentos en claro).
func LoadIMAP() (IMAPConfig, error) {
	c := IMAPConfig{
		Host: os.Getenv("GRABI_IMAP_HOST"),
		Port: os.Getenv("GRABI_IMAP_PORT"),
		User: os.Getenv("GRABI_IMAP_USER"),
		Pass: os.Getenv("GRABI_IMAP_PASS"),
	}
	if c.Port == "" {
		c.Port = "993"
	}
	var faltan []string
	if c.Host == "" {
		faltan = append(faltan, "GRABI_IMAP_HOST")
	}
	if c.User == "" {
		faltan = append(faltan, "GRABI_IMAP_USER")
	}
	if c.Pass == "" {
		faltan = append(faltan, "GRABI_IMAP_PASS")
	}
	if len(faltan) > 0 {
		return c, fmt.Errorf("faltan credenciales IMAP en el entorno/.env: %s", strings.Join(faltan, ", "))
	}
	return c, nil
}

// BreBKey devuelve la llave Bre-B (alias de cobro) de una máquina, leída de la
// variable GRABI_BREB_KEY_<MID> (ej. GRABI_BREB_KEY_M001). El VALOR de la llave
// vive fuera del repo (ADR-014). Devuelve "" si no está configurada.
func BreBKey(machineID string) string {
	return os.Getenv("GRABI_BREB_KEY_" + strings.ToUpper(machineID))
}

// UniqueAmountFallback indica si la conciliación debe usar el mecanismo LEGADO de
// "monto único" (desambiguador de pesos) en vez del nuevo match por monto exacto
// + nombre del pagador (ADR-018). Se activa con GRABI_MATCH_MODE=unique_amount;
// por defecto (vacío o "payer") se usa el modo por nombre.
func UniqueAmountFallback() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("GRABI_MATCH_MODE")), "unique_amount")
}
