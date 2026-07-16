// Package imapmail es un cliente IMAP mínimo para leer el buzón de conciliación
// (grabibot) sobre TLS. Solo hace lo que necesita la conciliación: login,
// seleccionar INBOX, traer los correos NO leídos de un remitente y marcarlos como
// leídos DESPUÉS de procesarlos.
//
// Credenciales: llegan por config.IMAPConfig (desde .env), nunca en claro en el
// repo (CLAUDE.md §4, ADR-013). Se usa App Password de Gmail, no la contraseña
// real.
package imapmail

import (
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"dispensadoras/software/internal/config"
)

// Client envuelve la conexión IMAP.
type Client struct {
	c *imapclient.Client
}

// Dial abre la conexión TLS y hace login. El llamador debe llamar Close().
func Dial(cfg config.IMAPConfig) (*Client, error) {
	c, err := imapclient.DialTLS(cfg.Addr(), nil)
	if err != nil {
		return nil, fmt.Errorf("imapmail: conectando a %s: %w", cfg.Addr(), err)
	}
	if err := c.Login(cfg.User, cfg.Pass).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("imapmail: login como %s falló: %w", cfg.User, err)
	}
	return &Client{c: c}, nil
}

// Close cierra la sesión (LOGOUT + cierre del socket).
func (cl *Client) Close() error {
	_ = cl.c.Logout().Wait()
	return cl.c.Close()
}

// SelectInbox selecciona INBOX en modo lectura/escritura (para poder marcar
// \Seen) y devuelve el número de mensajes del buzón.
func (cl *Client) SelectInbox() (uint32, error) {
	data, err := cl.c.Select("INBOX", nil).Wait()
	if err != nil {
		return 0, fmt.Errorf("imapmail: SELECT INBOX: %w", err)
	}
	return data.NumMessages, nil
}

// RawMessage es un correo crudo (RFC 5322) traído del buzón, con su UID.
type RawMessage struct {
	UID imap.UID
	Raw []byte
}

// FetchUnseenFrom busca los correos NO leídos (\Seen ausente) cuyo header From
// contiene `from`, y devuelve sus bytes crudos. Usa BODY.PEEK (Peek:true) para NO
// marcarlos leídos al traerlos: la marca \Seen se pone explícitamente con
// MarkSeen tras procesar cada correo (control de idempotencia + reintentos).
func (cl *Client) FetchUnseenFrom(from string) ([]RawMessage, error) {
	// NOOP antes de buscar: en una sesión con el buzón ya seleccionado, el servidor
	// solo informa de los correos que LLEGARON durante la sesión cuando el cliente
	// manda un comando. NOOP es el comando canónico para sondear correo nuevo
	// (RFC 3501 §6.1.2); sin él, un correo recibido tras el SELECT no aparece en el
	// SEARCH y la conciliación nunca lo vería.
	if err := cl.c.Noop().Wait(); err != nil {
		return nil, fmt.Errorf("imapmail: NOOP: %w", err)
	}

	criteria := &imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
		Header:  []imap.SearchCriteriaHeaderField{{Key: "From", Value: from}},
	}
	sd, err := cl.c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("imapmail: SEARCH: %w", err)
	}
	uids := sd.AllUIDs()
	if len(uids) == 0 {
		return nil, nil
	}

	opts := &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}}, // BODY.PEEK[]
	}
	msgs, err := cl.c.Fetch(imap.UIDSetNum(uids...), opts).Collect()
	if err != nil {
		return nil, fmt.Errorf("imapmail: FETCH: %w", err)
	}

	out := make([]RawMessage, 0, len(msgs))
	for _, m := range msgs {
		if len(m.BodySection) == 0 {
			continue
		}
		out = append(out, RawMessage{UID: m.UID, Raw: m.BodySection[0].Bytes})
	}
	return out, nil
}

// MarkSeen marca los UIDs dados con la bandera \Seen (correo procesado). Se llama
// tras persistir el movimiento, para que un correo ya contabilizado no se vuelva
// a listar en el próximo poll.
func (cl *Client) MarkSeen(uids ...imap.UID) error {
	if len(uids) == 0 {
		return nil
	}
	store := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Flags:  []imap.Flag{imap.FlagSeen},
		Silent: true,
	}
	cmd := cl.c.Store(imap.UIDSetNum(uids...), store, nil)
	if _, err := cmd.Collect(); err != nil {
		return fmt.Errorf("imapmail: STORE \\Seen: %w", err)
	}
	return nil
}
