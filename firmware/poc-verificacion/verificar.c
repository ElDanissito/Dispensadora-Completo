/*
 * PoC de verificación del token de dispensado — Departamento 03 (Firmware).
 *
 * Objetivo: demostrar EN EL PC, antes de comprar hardware, que la lógica
 * criptográfica del contrato funciona. Este mismo código de verificación es
 * el que se portará al ESP32 (Monocypher es portable a microcontroladores).
 *
 * Implementa la verificación EXACTA de `especificaciones/contrato-token.md` §5
 * y devuelve los códigos de error de §7.
 *
 * Actualizado al contrato v2 (ADR-006): el payload ya no lleva `iss` ni `iat`.
 * La validación pasa de firma -> mid -> exp -> jti (sin el paso de `iss`).
 *
 * IMPORTANTE (compatibilidad de firma):
 *   El servidor (Go, crypto/ed25519) firma con Ed25519 estándar RFC 8032, que
 *   usa SHA-512. Por eso aquí usamos crypto_ed25519_check() del módulo OPCIONAL
 *   monocypher-ed25519 (EdDSA + SHA-512), y NO crypto_eddsa_check() del núcleo
 *   de Monocypher, que por defecto usa BLAKE2b y NO sería compatible.
 *
 * La máquina SOLO tiene la llave pública (regla no negociable del proyecto).
 *
 * Compilación (ver README.md):
 *   gcc -O2 -o verificar verificar.c monocypher/monocypher.c monocypher/monocypher-ed25519.c -Imonocypher
 *
 * Uso:
 *   verificar --pubkey <llave-publica-b64> [--now <epoch_s>] <token.txt> [<token.txt> ...]
 *
 * Si se pasan varios tokens, comparten el registro de `jti` usados: así se puede
 * demostrar el anti-reuso (ALREADY_USED) pasando el mismo token dos veces.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <inttypes.h>

#include "monocypher.h"
#include "monocypher-ed25519.h"

/* ---- Constantes del contrato (parámetros de ESTA máquina) ---------------- */
/* Contrato v2: el payload ya NO lleva `iss` ni `iat` (ADR-006). La máquina
 * asume el emisor de forma implícita, así que no hay constante ISSUER.        */
#define MACHINE_ID   "M001"                /* mid que acepta esta máquina      */
#define ALG          "EdDSA"               /* header.alg obligatorio (v2)      */
#define TYP          "DSP"                 /* header.typ obligatorio           */
#define KID_LOCAL    "k1"                  /* único kid aprovisionado          */
#define NOW_DEFAULT  1752460900LL          /* RTC simulado (resultados-esperados.md) */

/* ---- Códigos de resultado (contrato §7) ---------------------------------- */
typedef enum {
    R_OK = 0,
    R_MALFORMED,
    R_BAD_SIGNATURE,
    R_UNKNOWN_KEY,
    R_WRONG_MACHINE,
    R_EXPIRED,
    R_ALREADY_USED
} result_t;   /* v2: se elimina R_BAD_ISSUER (ya no hay campo `iss`). */

static const char *result_str(result_t r) {
    switch (r) {
        case R_OK:            return "OK";
        case R_MALFORMED:     return "MALFORMED";
        case R_BAD_SIGNATURE: return "BAD_SIGNATURE";
        case R_UNKNOWN_KEY:   return "UNKNOWN_KEY";
        case R_WRONG_MACHINE: return "WRONG_MACHINE";
        case R_EXPIRED:       return "EXPIRED";
        case R_ALREADY_USED:  return "ALREADY_USED";
        default:              return "?";
    }
}

/* ---- Base64 / Base64URL decode ------------------------------------------- *
 * Acepta ambos alfabetos (+/ y -_) e ignora el padding '='. Sirve tanto para
 * la llave pública (base64 estándar) como para las partes del token (base64url
 * sin padding). Devuelve el nº de bytes escritos, o -1 si hay un carácter
 * inválido o no cabe en el buffer de salida.                                  */
static int b64_val(int c) {
    if (c >= 'A' && c <= 'Z') return c - 'A';
    if (c >= 'a' && c <= 'z') return c - 'a' + 26;
    if (c >= '0' && c <= '9') return c - '0' + 52;
    if (c == '+' || c == '-') return 62;
    if (c == '/' || c == '_') return 63;
    return -1; /* '=' y cualquier otro se tratan aparte / como inválido */
}

static int b64_decode(const char *in, size_t in_len, uint8_t *out, size_t out_cap) {
    uint32_t acc = 0;
    int      bits = 0;
    size_t   n = 0;
    for (size_t i = 0; i < in_len; i++) {
        int c = (unsigned char)in[i];
        if (c == '=' || c == '\r' || c == '\n' || c == ' ') continue;
        int v = b64_val(c);
        if (v < 0) return -1;
        acc = (acc << 6) | (uint32_t)v;
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            if (n >= out_cap) return -1;
            out[n++] = (uint8_t)((acc >> bits) & 0xFF);
        }
    }
    return (int)n;
}

/* ---- Extracción mínima de campos JSON ------------------------------------ *
 * El payload/header son JSON compacto y controlado por el servidor. Para la
 * PoC basta con un escáner simple de "clave":valor. En el ESP32 conviene un
 * parser robusto (p. ej. jsmn) o migrar a un formato binario (COSE/CBOR).      */

/* Devuelve puntero al primer carácter tras `"clave":` o NULL. */
static const char *json_after_key(const char *json, const char *key) {
    char pat[64];
    int  m = snprintf(pat, sizeof pat, "\"%s\":", key);
    if (m <= 0 || (size_t)m >= sizeof pat) return NULL;
    const char *p = strstr(json, pat);
    if (!p) return NULL;
    return p + m;
}

/* Copia el valor string de `"clave":"valor"` a out. 0 = ok, -1 = no está. */
static int json_get_str(const char *json, const char *key, char *out, size_t cap) {
    const char *p = json_after_key(json, key);
    if (!p || *p != '"') return -1;
    p++;
    size_t n = 0;
    while (*p && *p != '"') {
        if (n + 1 >= cap) return -1;
        out[n++] = *p++;
    }
    if (*p != '"') return -1;
    out[n] = '\0';
    return 0;
}

/* Lee el valor entero de `"clave":N`. 0 = ok, -1 = no está / no numérico. */
static int json_get_int(const char *json, const char *key, long long *out) {
    const char *p = json_after_key(json, key);
    if (!p) return -1;
    char *end;
    long long v = strtoll(p, &end, 10);
    if (end == p) return -1;
    *out = v;
    return 0;
}

/* ---- Registro de jti usados (anti-reuso) --------------------------------- *
 * En la PoC es un arreglo en memoria. En el ESP32 va a NVS/FRAM (persistente),
 * y se marca ANTES de dispensar (contrato §5, paso 10).                        */
#define JTI_MAX     512
#define JTI_LEN     64
static char jti_used[JTI_MAX][JTI_LEN];
static int  jti_count = 0;

static int jti_is_used(const char *jti) {
    for (int i = 0; i < jti_count; i++)
        if (strcmp(jti_used[i], jti) == 0) return 1;
    return 0;
}
static void jti_mark_used(const char *jti) {
    if (jti_count < JTI_MAX) {
        snprintf(jti_used[jti_count], JTI_LEN, "%s", jti);
        jti_count++;
    }
}

/* ---- Verificación (contrato §5, orden exacto) ---------------------------- */
static result_t verificar_token(const char *token, size_t tlen,
                                const uint8_t pubkey[32], long long now,
                                char *items_out, size_t items_cap) {
    if (items_out && items_cap) items_out[0] = '\0';

    /* Paso 1: separar en 3 partes por '.'. Debe haber EXACTAMENTE 2 puntos. */
    long p1 = -1, p2 = -1;
    int dots = 0;
    for (size_t i = 0; i < tlen; i++) {
        if (token[i] == '.') {
            dots++;
            if (dots == 1) p1 = (long)i;
            else if (dots == 2) p2 = (long)i;
        }
    }
    if (dots != 2 || p1 <= 0 || p2 <= p1 + 1 || (size_t)p2 + 1 >= tlen)
        return R_MALFORMED;

    const char *h_b64 = token;                 size_t h_len = (size_t)p1;
    const char *p_b64 = token + p1 + 1;         size_t p_len = (size_t)(p2 - p1 - 1);
    const char *s_b64 = token + p2 + 1;         size_t s_len = tlen - (size_t)p2 - 1;

    /* Paso 2: decodificar header y validar alg/typ; obtener kid. */
    char header[512];
    int hn = b64_decode(h_b64, h_len, (uint8_t *)header, sizeof header - 1);
    if (hn < 0) return R_MALFORMED;
    header[hn] = '\0';

    char alg[32], typ[16], kid[32];
    if (json_get_str(header, "alg", alg, sizeof alg) != 0 ||
        json_get_str(header, "typ", typ, sizeof typ) != 0 ||
        json_get_str(header, "kid", kid, sizeof kid) != 0)
        return R_MALFORMED;
    /* alg/typ incorrectos = token no válido (incluye intento de downgrade). */
    if (strcmp(alg, ALG) != 0 || strcmp(typ, TYP) != 0)
        return R_MALFORMED;

    /* Paso 3: buscar llave pública para ese kid. */
    if (strcmp(kid, KID_LOCAL) != 0)
        return R_UNKNOWN_KEY;

    /* Paso 4: verificar firma Ed25519 sobre (parte1 + "." + parte2). */
    uint8_t sig[64];
    int sn = b64_decode(s_b64, s_len, sig, sizeof sig);
    if (sn != 64) return R_BAD_SIGNATURE;
    /* signing_input = header_b64 + "." + payload_b64 = token[0 .. p2). */
    if (crypto_ed25519_check(sig, pubkey, (const uint8_t *)token, (size_t)p2) != 0)
        return R_BAD_SIGNATURE;

    /* Paso 5: decodificar payload. */
    char payload[1024];
    int pn = b64_decode(p_b64, p_len, (uint8_t *)payload, sizeof payload - 1);
    if (pn < 0) return R_MALFORMED;
    payload[pn] = '\0';

    /* Paso 6 (v2): mid == MACHINE_ID. */
    char mid[32];
    if (json_get_str(payload, "mid", mid, sizeof mid) != 0) return R_MALFORMED;
    if (strcmp(mid, MACHINE_ID) != 0) return R_WRONG_MACHINE;

    /* Paso 7 (v2): now (RTC) <= exp. */
    long long exp;
    if (json_get_int(payload, "exp", &exp) != 0) return R_MALFORMED;
    if (now > exp) return R_EXPIRED;

    /* Paso 8 (v2): jti no usado. */
    char jti[JTI_LEN];
    if (json_get_str(payload, "jti", jti, sizeof jti) != 0) return R_MALFORMED;
    if (jti_is_used(jti)) return R_ALREADY_USED;

    /* Paso 9 (v2): marcar jti como usado ANTES de dispensar (persistente en HW). */
    jti_mark_used(jti);

    /* Paso 10 (v2): "dispensar" items (en la PoC solo los reportamos). Copiamos el
     * arreglo `[...]` respetando el balance de corchetes. */
    if (items_out && items_cap) {
        const char *it = json_after_key(payload, "items");
        if (it && *it == '[') {
            int depth = 0;
            size_t n = 0;
            for (const char *p = it; *p && n + 1 < items_cap; p++) {
                items_out[n++] = *p;
                if (*p == '[') depth++;
                else if (*p == ']' && --depth == 0) break;
            }
            items_out[n] = '\0';
        }
    }
    return R_OK;
}

/* ---- Carga de archivos --------------------------------------------------- */
static int read_file(const char *path, char *buf, size_t cap) {
    FILE *f = fopen(path, "rb");
    if (!f) return -1;
    size_t n = fread(buf, 1, cap - 1, f);
    fclose(f);
    buf[n] = '\0';
    /* recortar espacios/saltos finales */
    while (n > 0 && (buf[n-1] == '\n' || buf[n-1] == '\r' ||
                     buf[n-1] == ' '  || buf[n-1] == '\t')) {
        buf[--n] = '\0';
    }
    return (int)n;
}

int main(int argc, char **argv) {
    const char *pubkey_path = NULL;
    long long   now = NOW_DEFAULT;
    const char *tokens[64];
    int         ntokens = 0;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--pubkey") == 0 && i + 1 < argc) {
            pubkey_path = argv[++i];
        } else if (strcmp(argv[i], "--now") == 0 && i + 1 < argc) {
            now = strtoll(argv[++i], NULL, 10);
        } else if (strncmp(argv[i], "--", 2) == 0) {
            fprintf(stderr, "opción desconocida: %s\n", argv[i]);
            return 2;
        } else if (ntokens < 64) {
            tokens[ntokens++] = argv[i];
        }
    }

    if (!pubkey_path || ntokens == 0) {
        fprintf(stderr,
            "uso: %s --pubkey <llave-publica-b64> [--now <epoch_s>] <token.txt> [...]\n",
            argv[0]);
        return 2;
    }

    /* Cargar llave pública (base64 estándar). Ed25519 = 32 bytes. */
    char pub_b64[256];
    if (read_file(pubkey_path, pub_b64, sizeof pub_b64) < 0) {
        fprintf(stderr, "no pude leer la llave pública: %s\n", pubkey_path);
        return 2;
    }
    uint8_t pubkey[32];
    if (b64_decode(pub_b64, strlen(pub_b64), pubkey, sizeof pubkey) != 32) {
        fprintf(stderr, "llave pública inválida (se esperan 32 bytes Ed25519)\n");
        return 2;
    }

    fprintf(stderr, "NOW = %lld  |  MACHINE_ID = %s  |  kid local = %s\n\n",
            now, MACHINE_ID, KID_LOCAL);

    int fallos = 0;
    for (int i = 0; i < ntokens; i++) {
        char token[2048];
        if (read_file(tokens[i], token, sizeof token) < 0) {
            fprintf(stderr, "no pude leer el token: %s\n", tokens[i]);
            fallos++;
            continue;
        }
        char items[256];
        result_t r = verificar_token(token, strlen(token), pubkey, now,
                                     items, sizeof items);
        printf("%-24s -> %s", tokens[i], result_str(r));
        if (r == R_OK && items[0]) printf("   dispensar items=%s", items);
        printf("\n");
    }
    return fallos ? 1 : 0;
}
