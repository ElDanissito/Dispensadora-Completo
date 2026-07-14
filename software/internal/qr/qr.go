// Package qr genera el QR del token para mostrarlo al cliente.
package qr

import "github.com/skip2/go-qrcode"

// WritePNG genera un QR (ECC nivel M, como recomienda el contrato §4) con el
// contenido dado y lo escribe como PNG en path. size es el lado en píxeles.
func WritePNG(content, path string, size int) error {
	return qrcode.WriteFile(content, qrcode.Medium, size, path)
}
