package main

import (
	"fmt"
	"os"

	"dispensadoras/software/internal/bankmail"
	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/imapmail"
)

// ---------- concil-parse (offline) ----------

// cmdConcilParse parsea un correo crudo (.eml) y muestra los campos extraídos.
// No toca la red ni la base: sirve para calibrar el parser contra correos reales
// o contra la muestra (primer paso operativo, spec §9).
func cmdConcilParse(args []string) error {
	fs := newFlags("concil-parse")
	in := fs.String("in", "", "ruta de un correo crudo .eml (obligatorio)")
	fs.Parse(args)

	if *in == "" {
		return fmt.Errorf("-in es obligatorio (ruta a un .eml)")
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	meta, mv, err := bankmail.ParseEmail(raw)
	fmt.Printf("From:        %s  (allowlist: %v)\n", meta.FromAddr, bankmail.InAllowlist(meta.FromAddr))
	fmt.Printf("Message-ID:  %s\n", meta.MessageID)
	if !meta.ReceivedAt.IsZero() {
		fmt.Printf("Date:        %s\n", meta.ReceivedAt.In(bankmail.Bogota).Format("2006-01-02 15:04 -07:00"))
	}
	if err != nil {
		return fmt.Errorf("no se pudo extraer el movimiento del cuerpo: %w", err)
	}
	fmt.Println("--- movimiento extraído ---")
	fmt.Printf("Máquina:     %s  (raw: %q)\n", mv.MachineID, mv.MachineRaw)
	fmt.Printf("Monto (COP): %d\n", mv.AmountCOP)
	fmt.Printf("Pagador:     %s\n", mv.Payer)
	fmt.Printf("Cuenta:      %s\n", mv.Account)
	fmt.Printf("Llave:       %s\n", mv.BreBKey)
	fmt.Printf("Fecha/hora:  %s (%s %s)\n", mv.OccurredAt.Format("2006-01-02 15:04 -07:00"), mv.DateRaw, mv.TimeRaw)
	return nil
}

// ---------- concil-login (IMAP) ----------

// cmdConcilLogin conecta al buzón de conciliación (grabibot) con las credenciales
// del .env, hace login, selecciona INBOX y (opcional) lista los correos NO leídos
// del remitente oficial. Primera prueba del canal (spec: login IMAP OK).
func cmdConcilLogin(args []string) error {
	fs := newFlags("concil-login")
	envPath := fs.String("env", ".env", "ruta del archivo .env con GRABI_IMAP_*")
	list := fs.Bool("list", false, "además, lista los correos NO leídos del remitente oficial")
	fs.Parse(args)

	if _, err := config.LoadDotEnv(*envPath); err != nil {
		return fmt.Errorf("leyendo %s: %w", *envPath, err)
	}
	cfg, err := config.LoadIMAP()
	if err != nil {
		return err
	}

	fmt.Printf("Conectando a %s como %s …\n", cfg.Addr(), cfg.User)
	cl, err := imapmail.Dial(cfg)
	if err != nil {
		return err
	}
	defer cl.Close()

	n, err := cl.SelectInbox()
	if err != nil {
		return err
	}
	fmt.Printf("Login OK. INBOX tiene %d mensajes.\n", n)

	if *list {
		sender := bankmail.Allowlist[0]
		msgs, err := cl.FetchUnseenFrom(sender)
		if err != nil {
			return err
		}
		fmt.Printf("Correos NO leídos de %s: %d\n", sender, len(msgs))
		for _, m := range msgs {
			meta, mv, perr := bankmail.ParseEmail(m.Raw)
			if perr != nil || mv == nil {
				fmt.Printf("  UID %d  msgid=%s  [no parseable: %v]\n", m.UID, meta.MessageID, perr)
				continue
			}
			fmt.Printf("  UID %d  máquina=%s  monto=%d  hora=%s  pagador=%q\n",
				m.UID, mv.MachineID, mv.AmountCOP, mv.OccurredAt.Format("2006-01-02 15:04"), mv.Payer)
		}
		fmt.Println("(no se marcó ningún correo como leído; solo lectura)")
	}
	return nil
}
