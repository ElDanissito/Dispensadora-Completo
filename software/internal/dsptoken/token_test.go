package dsptoken

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
)

// helper: par de llaves + keystore para kid "k1".
func newTestKeys(t *testing.T) (ed25519.PrivateKey, MapKeyStore) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, MapKeyStore{DefaultKID: pub}
}

func validPayload() Payload {
	return Payload{
		Mid: "M001", Jti: "ord_test01",
		Exp: 2000, Items: []Item{{S: 3, Q: 1}, {S: 5, Q: 2}},
	}
}

func baseParams(keys KeyStore) VerifyParams {
	return VerifyParams{Keys: keys, MachineID: "M001", Now: 1500, Used: MemUsed{}}
}

func TestVerifyOK(t *testing.T) {
	priv, keys := newTestKeys(t)
	tok, err := Sign(priv, DefaultHeader(DefaultKID), validPayload())
	if err != nil {
		t.Fatal(err)
	}
	res := Verify(tok, baseParams(keys))
	if res.Code != OK {
		t.Fatalf("esperaba OK, obtuve %s", res.Code)
	}
	if res.Payload == nil || res.Payload.Mid != "M001" {
		t.Fatalf("payload no decodificado correctamente: %+v", res.Payload)
	}
}

func TestVerifyExpired(t *testing.T) {
	priv, keys := newTestKeys(t)
	tok, _ := Sign(priv, DefaultHeader(DefaultKID), validPayload())
	vp := baseParams(keys)
	vp.Now = 2001 // > exp (2000)
	if res := Verify(tok, vp); res.Code != Expired {
		t.Fatalf("esperaba EXPIRED, obtuve %s", res.Code)
	}
}

func TestVerifyBadSignature(t *testing.T) {
	priv, keys := newTestKeys(t)
	tok, _ := Sign(priv, DefaultHeader(DefaultKID), validPayload())
	parts := strings.Split(tok, ".")
	// Corromper el PRIMER char de la firma (base64url válido). No el último: el
	// último solo lleva 2 bits significativos y 'A'↔'B' ahí no cambia la firma
	// decodificada (bug que hacía este test flaky). El primero lleva 6 bits.
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	parts[2] = string(sig)
	if res := Verify(strings.Join(parts, "."), baseParams(keys)); res.Code != BadSignature {
		t.Fatalf("esperaba BAD_SIGNATURE, obtuve %s", res.Code)
	}
}

func TestVerifyWrongMachine(t *testing.T) {
	priv, keys := newTestKeys(t)
	tok, _ := Sign(priv, DefaultHeader(DefaultKID), validPayload())
	vp := baseParams(keys)
	vp.MachineID = "M999"
	if res := Verify(tok, vp); res.Code != WrongMachine {
		t.Fatalf("esperaba WRONG_MACHINE, obtuve %s", res.Code)
	}
}

func TestVerifyUnknownKey(t *testing.T) {
	priv, _ := newTestKeys(t)
	tok, _ := Sign(priv, DefaultHeader("k9"), validPayload())
	// keystore solo conoce k1
	_, keys := newTestKeys(t)
	if res := Verify(tok, baseParams(keys)); res.Code != UnknownKey {
		t.Fatalf("esperaba UNKNOWN_KEY, obtuve %s", res.Code)
	}
}

func TestVerifyAlreadyUsed(t *testing.T) {
	priv, keys := newTestKeys(t)
	tok, _ := Sign(priv, DefaultHeader(DefaultKID), validPayload())
	vp := baseParams(keys) // Used compartido entre las dos verificaciones
	if res := Verify(tok, vp); res.Code != OK {
		t.Fatalf("primer uso: esperaba OK, obtuve %s", res.Code)
	}
	if res := Verify(tok, vp); res.Code != AlreadyUsed {
		t.Fatalf("segundo uso: esperaba ALREADY_USED, obtuve %s", res.Code)
	}
}

func TestVerifyMalformed(t *testing.T) {
	_, keys := newTestKeys(t)
	cases := map[string]string{
		"sin puntos":    "abcdef",
		"dos partes":    "abc.def",
		"cuatro partes": "a.b.c.d",
		"header basura": "!!!.eyJ9.xxx",
	}
	for name, tok := range cases {
		if res := Verify(tok, baseParams(keys)); res.Code != Malformed {
			t.Errorf("%s: esperaba MALFORMED, obtuve %s", name, res.Code)
		}
	}
}

func TestVerifyRejectsBadAlg(t *testing.T) {
	priv, keys := newTestKeys(t)
	// header con alg distinto → debe rechazarse (anti-downgrade) como MALFORMED
	h := Header{Alg: "HS256", Typ: TypDSP, Kid: DefaultKID}
	tok, _ := Sign(priv, h, validPayload())
	if res := Verify(tok, baseParams(keys)); res.Code != Malformed {
		t.Fatalf("esperaba MALFORMED por alg inválido, obtuve %s", res.Code)
	}
}

// El orden del contrato §5 exige que la firma se valide ANTES que exp:
// un token expirado Y con firma mala debe reportar BAD_SIGNATURE.
func TestVerifyOrderSignatureBeforeExpiry(t *testing.T) {
	priv, keys := newTestKeys(t)
	p := validPayload()
	p.Exp = 500 // ya expirado respecto a now=1500
	tok, _ := Sign(priv, DefaultHeader(DefaultKID), p)
	parts := strings.Split(tok, ".")
	sig := []byte(parts[2])
	sig[len(sig)-1] ^= 0 // no-op para dejar claro; corrompemos abajo
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	parts[2] = string(sig)
	if res := Verify(strings.Join(parts, "."), baseParams(keys)); res.Code != BadSignature {
		t.Fatalf("esperaba BAD_SIGNATURE antes que EXPIRED, obtuve %s", res.Code)
	}
}
