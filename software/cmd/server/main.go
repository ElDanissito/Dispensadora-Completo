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
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"dispensadoras/software/internal/store"
	"dispensadoras/software/internal/web"
)

func main() {
	db := flag.String("db", "dispensadoras.db", "ruta del archivo SQLite")
	addr := flag.String("addr", ":8080", "dirección de escucha")
	seed := flag.Bool("seed", false, "cargar datos de demostración si la base está vacía")
	flag.Parse()

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

	srv, err := web.New(st, adminUser, adminPass)
	if err != nil {
		log.Fatalf("construyendo servidor: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("dispensadoras web escuchando en %s (db=%s)", *addr, *db)
	log.Printf("  público: http://localhost%s/m/M001", *addr)
	log.Printf("  panel:   http://localhost%s/admin (usuario %q)", *addr, adminUser)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
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
