// Comando server: backend web del piloto de dispensadoras.
//
//	server [-db dispensadoras.db] [-addr :8080] [-seed]
//
// Sirve la página pública por máquina (GET /m/{id}) y un panel de administración
// mínimo en /admin (protegido con Basic Auth). Las credenciales del panel se
// toman de ADMIN_USER / ADMIN_PASS (por defecto admin/changeme, con aviso).
package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dispensadoras/software/internal/concil"
	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/dsptoken"
	"dispensadoras/software/internal/store"
	"dispensadoras/software/internal/web"
)

// defaultEnvPath es el archivo de secretos local (git-ignored), relativo a
// /software. Trae GRABI_IMAP_* (conciliación) y opcionalmente GRABI_BREB_KEY_*.
const defaultEnvPath = ".env"

// defaultPrivPath es la ubicación de la llave privada relativa a /software
// (misma que usa el CLI dsp). Nunca se commitea (.keys está en .gitignore).
const defaultPrivPath = ".keys/private-k1.key"

// loadPrivate carga la llave privada de firma desde la env DSP_PRIVATE_KEY o,
// si no está, desde el archivo .keys/private-k1.key. Devuelve nil (sin error)
// si no hay llave: el servidor arranca igual, solo que "simular pago" avisará.
func loadPrivate() ed25519.PrivateKey {
	if env := os.Getenv("DSP_PRIVATE_KEY"); env != "" {
		sk, err := dsptoken.DecodePrivate(env)
		if err != nil {
			log.Printf("AVISO: DSP_PRIVATE_KEY inválida: %v (simular pago quedará deshabilitado)", err)
			return nil
		}
		return sk
	}
	data, err := os.ReadFile(defaultPrivPath)
	if err != nil {
		log.Printf("AVISO: no hay llave privada en %s ni DSP_PRIVATE_KEY; 'simular pago' quedará deshabilitado. Ejecuta 'dsp keygen'.", defaultPrivPath)
		return nil
	}
	sk, err := dsptoken.DecodePrivate(string(data))
	if err != nil {
		log.Printf("AVISO: llave privada en %s inválida: %v", defaultPrivPath, err)
		return nil
	}
	return sk
}

func main() {
	db := flag.String("db", "dispensadoras.db", "ruta del archivo SQLite")
	addr := flag.String("addr", ":8080", "dirección de escucha")
	seed := flag.Bool("seed", false, "cargar datos de demostración si la base está vacía")
	allowSim := flag.Bool("allow-sim", false, "habilita POST /m/{id}/simular-pago (atajo de pruebas; NO en producción)")
	enableConcil := flag.Bool("concil", false, "arranca la conciliación de pagos por correo (requiere GRABI_IMAP_* en .env)")
	concilInterval := flag.Duration("concil-interval", 12*time.Second, "cada cuánto revisa el correo la conciliación")
	payWindow := flag.Duration("pay-window", web.DefaultPayWindow, "ventana de validez de pago de las órdenes")
	flag.Parse()

	// Cargar secretos locales (.env, git-ignored). Las variables reales del
	// entorno tienen prioridad; el .env solo rellena lo que falte.
	if _, err := config.LoadDotEnv(defaultEnvPath); err != nil {
		log.Printf("AVISO: no se pudo leer %s: %v", defaultEnvPath, err)
	}

	st, err := store.Open(*db)
	if err != nil {
		log.Fatalf("abriendo base %s: %v", *db, err)
	}
	defer st.Close()

	if *seed {
		if err := seedDemo(st); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	adminUser := envOr("ADMIN_USER", "admin")
	adminPass := os.Getenv("ADMIN_PASS")
	if adminPass == "" {
		adminPass = "changeme"
		log.Printf("AVISO: ADMIN_PASS no está definido; usando 'changeme'. Defínelo antes de exponer el panel.")
	}

	priv := loadPrivate()
	srv, err := web.New(st, adminUser, adminPass, priv, *allowSim, *payWindow)
	if err != nil {
		log.Fatalf("construyendo servidor: %v", err)
	}

	// Contexto de apagado limpio (Ctrl-C / SIGTERM): detiene la conciliación.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var mailer *reconnMailer
	if *enableConcil {
		mailer = startConciliacion(ctx, st, srv, priv, *concilInterval)
		if mailer != nil {
			defer mailer.Close()
		}
	} else {
		log.Printf("conciliación por correo DESHABILITADA (usa -concil para activarla)")
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("dispensadoras web escuchando en %s (db=%s)", *addr, *db)
	log.Printf("  público: http://localhost%s/m/M001", *addr)
	log.Printf("  panel:   http://localhost%s/admin (usuario %q)", *addr, adminUser)
	if *allowSim {
		log.Printf("  AVISO: -allow-sim activo (simular-pago habilitado; solo para pruebas)")
	}
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// startConciliacion arranca el poller de conciliación en una goroutine. Devuelve
// el mailer para poder cerrarlo al apagar. Si faltan credenciales o llave de
// firma, avisa y devuelve nil (el servidor web sigue funcionando).
func startConciliacion(ctx context.Context, st *store.Store, srv *web.Server, priv ed25519.PrivateKey, interval time.Duration) *reconnMailer {
	imapCfg, err := config.LoadIMAP()
	if err != nil {
		log.Printf("AVISO: conciliación NO arranca: %v", err)
		return nil
	}
	if priv == nil {
		log.Printf("AVISO: conciliación NO arranca: no hay llave privada para firmar el QR (ejecuta 'dsp keygen' o define DSP_PRIVATE_KEY)")
		return nil
	}
	mailer := &reconnMailer{cfg: imapCfg, log: log.Default()}
	svc := concil.New(st, mailer, srv, log.Default())
	go svc.Run(ctx, interval)
	return mailer
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// seedDemo carga una máquina y unos productos de ejemplo si aún no hay máquinas.
func seedDemo(st *store.Store) error {
	ctx := context.Background()
	machines, err := st.ListMachines(ctx)
	if err != nil {
		return err
	}
	if len(machines) > 0 {
		return nil // ya hay datos; no duplicar
	}
	if err := st.CreateMachine(ctx, "M001", "Demo — Cafetería Cali", "k1"); err != nil {
		return err
	}
	type prod struct {
		name  string
		slot  int
		price int64
		stock int
	}
	demo := []prod{
		{"Papas Margarita", 3, 3000, 8},
		{"Coca-Cola 400ml", 5, 3500, 6},
		{"Agua Cristal 600ml", 7, 2500, 0}, // agotado, para ver el estado
	}
	for _, d := range demo {
		id, err := st.CreateProduct(ctx, d.name)
		if err != nil {
			return err
		}
		if err := st.SetSlot(ctx, "M001", d.slot, id, d.price, d.stock); err != nil {
			return err
		}
	}
	log.Printf("seed: máquina M001 con %d productos de demo", len(demo))
	return nil
}
