package main

import (
	"log"

	"github.com/emersion/go-imap/v2"

	"dispensadoras/software/internal/config"
	"dispensadoras/software/internal/imapmail"
)

// reconnMailer implementa concil.Mailer con reconexión perezosa: si la conexión
// IMAP no existe o se cae (Gmail cierra sesiones ociosas), se vuelve a conectar en
// el siguiente poll. Un fallo transitorio de red no tumba la conciliación.
type reconnMailer struct {
	cfg config.IMAPConfig
	log *log.Logger
	cl  *imapmail.Client
}

func (m *reconnMailer) ensure() error {
	if m.cl != nil {
		return nil
	}
	cl, err := imapmail.Dial(m.cfg)
	if err != nil {
		return err
	}
	if _, err := cl.SelectInbox(); err != nil {
		cl.Close()
		return err
	}
	m.cl = cl
	m.log.Printf("conciliación: conectado a %s como %s", m.cfg.Addr(), m.cfg.User)
	return nil
}

func (m *reconnMailer) drop() {
	if m.cl != nil {
		m.cl.Close()
		m.cl = nil
	}
}

func (m *reconnMailer) FetchUnseenFrom(from string) ([]imapmail.RawMessage, error) {
	if err := m.ensure(); err != nil {
		return nil, err
	}
	msgs, err := m.cl.FetchUnseenFrom(from)
	if err != nil {
		m.drop() // fuerza reconexión en el próximo ciclo
		return nil, err
	}
	return msgs, nil
}

func (m *reconnMailer) MarkSeen(uids ...imap.UID) error {
	if err := m.ensure(); err != nil {
		return err
	}
	if err := m.cl.MarkSeen(uids...); err != nil {
		m.drop()
		return err
	}
	return nil
}

func (m *reconnMailer) Close() { m.drop() }
