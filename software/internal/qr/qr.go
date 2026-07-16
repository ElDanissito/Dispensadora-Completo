// Package qr genera el QR del token para mostrarlo al cliente.
package qr

import (
	"encoding/base64"

	"github.com/skip2/go-qrcode"
)

// WritePNG genera un QR (ECC nivel M, como recomienda el contrato §4) con el
// contenido dado y lo escribe como PNG en path. size es el lado en píxeles.
func WritePNG(content, path string, size int) error {
	return qrcode.WriteFile(content, qrcode.Medium, size, path)
}

// PNG genera el QR (ECC nivel M) y devuelve los bytes del PNG en memoria, para
// servirlo por HTTP o incrustarlo en la página sin escribir a disco.
func PNG(content string, size int) ([]byte, error) {
	return qrcode.Encode(content, qrcode.Medium, size)
}

// DataURI genera el QR y lo devuelve como URI `data:image/png;base64,...`,
// listo para el atributo src de una <img> (útil para mostrar el QR en la
// misma respuesta HTML, sin un endpoint aparte).
func DataURI(content string, size int) (string, error) {
	png, err := PNG(content, size)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
