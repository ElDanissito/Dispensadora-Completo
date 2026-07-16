/*
 * Dispensadora — Firmware (Depto. 03)
 * PASO 5b: igual que el paso 5 (verificar token v2 + dispensar con sensor),
 *          pero el anti-reuso de jti ahora es PERSISTENTE en NVS (Preferences).
 *
 * Placa:   ESP32 Dev Module (Arduino IDE)
 * Monitor: Serial (USB) a 115200.
 * Lector:  GM65 por UART2 (RX2=GPIO16, TX2=GPIO17).
 * Sensor:  E18-D80NK OUT en D26, INPUT_PULLUP (HIGH=libre, LOW=detectado).
 *
 * Que hace:
 *   1. Recibe un token por GM65 (QR) o por USB (pegado).
 *   2. Verifica el token con la logica EXACTA del contrato v2 (§5),
 *      firma Ed25519 con Monocypher (RFC 8032 / SHA-512, compatible con Go).
 *   3. Imprime el codigo de resultado del contrato (§7).
 *   4. Si es OK, recorre los items y mueve el motor del slot (LOGICA INVERSA,
 *      LOW = mueve). Mapa slot->GPIO: slot 3 -> D27 (M1), slot 5 -> D14 (M2).
 *   5. Al mover cada unidad, ESPERA la caida del producto en el E18-D80NK
 *      dentro de un TIMEOUT:
 *         - detecta (LOW) dentro del tiempo -> unidad OK, corta el motor ya.
 *         - se agota el timeout sin detectar -> DISPENSE_FAIL para ese slot
 *           (contrato §7: registrar para reembolso / soporte).
 *
 * NUEVO EN PASO 5b — anti-reuso en NVS (memoria no volatil):
 *   Antes el registro de jti usados vivia solo en RAM y se PERDIA al reiniciar
 *   (un apagon permitia re-dispensar el mismo QR). Ahora cada jti usado se
 *   escribe en NVS (particion "nvs" de la flash, via la libreria Preferences).
 *   Al arrancar se recargan a una cache en RAM para busqueda rapida. Resultado:
 *   el ALREADY_USED SOBREVIVE a reinicios / cortes de energia.
 *
 * ORDEN DEL CONTRATO (§5, regla clave): el jti se marca como usado ANTES de
 * accionar motores. Se cumple porque jti_mark_used() ocurre dentro de
 * verificar_token() (paso 9), que retorna OK ANTES de que dispensemos, y esa
 * marca ahora se COMMITEA a flash de forma sincrona (Preferences.putX escribe
 * antes de retornar). Asi, si se corta la energia justo despues de marcar y
 * antes/ durante el giro del motor, al volver el jti YA figura como usado y el
 * QR no se puede re-dispensar. Un DISPENSE_FAIL NO "devuelve" el jti: el token
 * queda consumido y el fallo se gestiona como incidencia (reembolso).
 *
 * UTIL PARA PRUEBAS: enviar por USB el texto "!reset" borra el registro de jti
 * en NVS (deja la maquina como recien aprovisionada). No es un token valido, no
 * afecta la operacion normal; solo para repetir el ensayo de reinicio.
 *
 * RTC: todavia no hay DS3231. NOW fijo = 1752460900 (resultados-esperados.md).
 * SEGURIDAD: la maquina SOLO tiene la llave publica. Monocypher: los 4 archivos
 * estan en esta misma carpeta.
 */

#include <Arduino.h>   /* explicito: pinMode/digitalWrite/Serial/HIGH/LOW, etc. */
#include <Preferences.h>  /* NVS: registro persistente de jti usados */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

extern "C" {
  #include "monocypher.h"
  #include "monocypher-ed25519.h"
}

/* ---- Parametros de ESTA maquina (aprovisionamiento) ---------------------- */
#define MACHINE_ID  "M001"       /* mid que acepta esta maquina        */
#define ALG         "EdDSA"      /* header.alg obligatorio (v2)        */
#define TYP         "DSP"        /* header.typ obligatorio             */
#define KID_LOCAL   "k1"         /* unico kid aprovisionado            */

/* Llave publica k1 en base64 (especificaciones/vectores-prueba/llave-publica-k1.txt).
 * La maquina SOLO tiene la publica. Se decodifica a 32 bytes en setup().        */
const char *PUBKEY_B64 = "L46okF6kj8SN742gYX9zNQD1P+V2ryNh0j4Yw29c9js=";

/* RTC simulado hasta que llegue el DS3231 (resultados-esperados.md). */
const long long NOW_FIJO = 1752460900LL;

/* ---- UART2 hacia el GM65 (igual que el paso 2) --------------------------- */
const int  GM65_RX_PIN = 16;    /* RX2 <- TX del GM65 */
const int  GM65_TX_PIN = 17;    /* TX2 -> RX del GM65 */
const long GM65_BAUD   = 9600;  /* si no lee, probar 115200 */

/* ---- Motores (logica inversa: HIGH = quieto, LOW = mueve) ---------------- */
const int MOTOR_QUIETO = HIGH;
const int MOTOR_MUEVE  = LOW;

/* Todos los GPIO de motor: se inicializan en HIGH para que no giren en el boot. */
const int MOTOR_PINS[]  = { 27, 14, 12, 13 };   /* Motor 1..4 (pinout inventario) */
const int MOTOR_PINS_N  = sizeof(MOTOR_PINS) / sizeof(MOTOR_PINS[0]);

/* Mapa slot -> GPIO para el DEMO (los 2 motores usables):
 *   slot 3 -> D27 (Motor 1)   ·   slot 5 -> D14 (Motor 2)
 * Al ampliar la maquina, aqui se agregan mas filas. slot no mapeado -> -1. */
struct SlotMap { int slot; int gpio; };
const SlotMap SLOT_MAP[] = {
  { 3, 27 },   /* Motor 1 */
  { 5, 14 },   /* Motor 2 */
};
const int SLOT_MAP_N = sizeof(SLOT_MAP) / sizeof(SLOT_MAP[0]);

/* ---- Sensor de caida E18-D80NK (paso 5) --------------------------------- */
const int SENSOR_PIN       = 26;    /* D26 (OUT del E18), INPUT_PULLUP */
const int SENSOR_LIBRE     = HIGH;  /* pull-up en reposo = nada delante */
const int SENSOR_DETECTADO = LOW;   /* NPN tira a GND al detectar producto */

/* Tiempos de dispensado. Ahora el motor gira HASTA que el sensor confirme la
 * caida O hasta que venza MOTOR_TIMEOUT_MS (lo que ocurra primero). */
const unsigned long MOTOR_TIMEOUT_MS  = 4000; /* espera max. de caida por unidad */
const unsigned long ENTRE_UNIDADES_MS = 400;  /* pausa entre unidades del mismo slot */
const unsigned long ENTRE_ITEMS_MS    = 500;  /* pausa entre items distintos */

/* ---- Framing por inactividad -------------------------------------------- */
const unsigned long FIN_LECTURA_MS = 60;
const size_t        BUFFER_MAX     = 1024;

/* Lector de un stream (GM65 por Serial2, o USB por Serial). Se define aqui
 * arriba, antes de cualquier funcion, porque el IDE de Arduino auto-genera
 * los prototipos al inicio del archivo; si el struct estuviera mas abajo, el
 * prototipo de lector_service() lo usaria sin conocerlo aun. */
struct LectorStream {
  Stream       *io;
  const char   *nombre;
  String        buf;
  unsigned long ultimoByte;
};

/* ---- Codigos de resultado (contrato §7) --------------------------------- */
/* Nota: NO usar R_OK/W_OK/X_OK/F_OK como nombres: son macros POSIX de
 * <unistd.h> (modos de access()), que el core del ESP32 arrastra via
 * Arduino.h y romperian el enum. Por eso el prefijo RES_. */
typedef enum {
  RES_OK = 0,
  RES_MALFORMED,
  RES_BAD_SIGNATURE,
  RES_UNKNOWN_KEY,
  RES_WRONG_MACHINE,
  RES_EXPIRED,
  RES_ALREADY_USED
} result_t;

static const char *result_str(result_t r) {
  switch (r) {
    case RES_OK:            return "OK";
    case RES_MALFORMED:     return "MALFORMED";
    case RES_BAD_SIGNATURE: return "BAD_SIGNATURE";
    case RES_UNKNOWN_KEY:   return "UNKNOWN_KEY";
    case RES_WRONG_MACHINE: return "WRONG_MACHINE";
    case RES_EXPIRED:       return "EXPIRED";
    case RES_ALREADY_USED:  return "ALREADY_USED";
    default:                return "?";
  }
}

/* Llave publica ya decodificada (32 bytes Ed25519). */
static uint8_t g_pubkey[32];
static bool    g_pubkey_ok = false;

/* ---- Base64 / Base64URL decode (identico a la PoC) ---------------------- */
static int b64_val(int c) {
  if (c >= 'A' && c <= 'Z') return c - 'A';
  if (c >= 'a' && c <= 'z') return c - 'a' + 26;
  if (c >= '0' && c <= '9') return c - '0' + 52;
  if (c == '+' || c == '-') return 62;
  if (c == '/' || c == '_') return 63;
  return -1;
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

/* ---- Extraccion minima de JSON (identico a la PoC) ---------------------- */
static const char *json_after_key(const char *json, const char *key) {
  char pat[64];
  int  m = snprintf(pat, sizeof pat, "\"%s\":", key);
  if (m <= 0 || (size_t)m >= sizeof pat) return NULL;
  const char *p = strstr(json, pat);
  if (!p) return NULL;
  return p + m;
}

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

static int json_get_int(const char *json, const char *key, long long *out) {
  const char *p = json_after_key(json, key);
  if (!p) return -1;
  char *end;
  long long v = strtoll(p, &end, 10);
  if (end == p) return -1;
  *out = v;
  return 0;
}

/* ---- Registro de jti usados (anti-reuso; PERSISTENTE en NVS) ------------- *
 * Modelo: la verdad vive en NVS (flash); en RAM tenemos una CACHE para que la
 * busqueda (jti_is_used) sea O(n) en memoria y no toque flash en cada escaneo.
 *
 * Layout en NVS (namespace NVS_NS):
 *   clave "n"   (int)    -> cuantos jti hay guardados.
 *   clave "j<i>"(string) -> el jti i-esimo (i = 0..n-1). Con JTI_MAX=64 la
 *                           clave mas larga es "j63" (3 chars), muy por debajo
 *                           del limite de 15 chars de las claves NVS.
 *
 * Al arrancar, jti_load_from_nvs() rellena la cache desde NVS. Al marcar un
 * jti, se escribe PRIMERO en flash (commit sincrono) y luego en la cache; asi
 * un corte de energia no deja el estado "dispensado pero no marcado".
 *
 * Limite JTI_MAX=64: suficiente para el piloto. La poda de jti ya vencidos
 * (por exp) queda pendiente junto con el RTC DS3231 (§5 del depto 03). Si se
 * llena, jti_mark_used avisa por Serial y NO marca (se prioriza no bloquear la
 * venta); documentado como pendiente, no ocurre con el volumen del piloto.    */
#define JTI_MAX  64
#define JTI_LEN  64
static const char *NVS_NS = "dsp";   /* namespace NVS (<=15 chars) */

static char        jti_used[JTI_MAX][JTI_LEN];
static int         jti_count = 0;
static Preferences prefs;

/* Carga el registro persistente de NVS a la cache en RAM (una vez, en setup). */
static void jti_load_from_nvs(void) {
  jti_count = 0;
  prefs.begin(NVS_NS, /*readOnly=*/true);
  int n = prefs.getInt("n", 0);
  if (n < 0)        n = 0;
  if (n > JTI_MAX)  n = JTI_MAX;   /* clamp defensivo por si la cache es menor */
  for (int i = 0; i < n; i++) {
    char key[8];
    snprintf(key, sizeof key, "j%d", i);
    String v = prefs.getString(key, "");
    snprintf(jti_used[i], JTI_LEN, "%s", v.c_str());
  }
  jti_count = n;
  prefs.end();
}

static int jti_is_used(const char *jti) {
  for (int i = 0; i < jti_count; i++)
    if (strcmp(jti_used[i], jti) == 0) return 1;
  return 0;
}

/* Marca un jti como usado: PERSISTE en NVS (antes de dispensar, §5) y actualiza
 * la cache. La escritura a flash es sincrona: cuando putString/putInt retornan,
 * el dato ya esta comprometido y sobrevive a un reinicio. */
static void jti_mark_used(const char *jti) {
  if (jti_count >= JTI_MAX) {
    Serial.println("  AVISO: registro de jti LLENO (JTI_MAX). No se persiste; "
                   "pendiente poda por exp (RTC).");
    return;
  }
  int i = jti_count;

  /* 1) Persistir en flash primero (durabilidad ante corte de energia). */
  prefs.begin(NVS_NS, /*readOnly=*/false);
  char key[8];
  snprintf(key, sizeof key, "j%d", i);
  prefs.putString(key, jti);
  prefs.putInt("n", i + 1);
  prefs.end();

  /* 2) Reflejar en la cache en RAM. */
  snprintf(jti_used[i], JTI_LEN, "%s", jti);
  jti_count = i + 1;
}

/* Borra TODO el registro de jti en NVS y en RAM. Solo para pruebas ("!reset"):
 * deja la maquina como recien aprovisionada. NO se usa en operacion normal. */
static void jti_reset_all(void) {
  prefs.begin(NVS_NS, /*readOnly=*/false);
  prefs.clear();
  prefs.end();
  jti_count = 0;
}

/* ---- Verificacion (contrato §5, orden EXACTO — portado de la PoC) -------- */
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
    return RES_MALFORMED;

  const char *h_b64 = token;             size_t h_len = (size_t)p1;
  const char *p_b64 = token + p1 + 1;    size_t p_len = (size_t)(p2 - p1 - 1);
  const char *s_b64 = token + p2 + 1;    size_t s_len = tlen - (size_t)p2 - 1;

  /* Paso 2: header -> alg/typ/kid. */
  char header[512];
  int hn = b64_decode(h_b64, h_len, (uint8_t *)header, sizeof header - 1);
  if (hn < 0) return RES_MALFORMED;
  header[hn] = '\0';

  char alg[32], typ[16], kid[32];
  if (json_get_str(header, "alg", alg, sizeof alg) != 0 ||
      json_get_str(header, "typ", typ, sizeof typ) != 0 ||
      json_get_str(header, "kid", kid, sizeof kid) != 0)
    return RES_MALFORMED;
  if (strcmp(alg, ALG) != 0 || strcmp(typ, TYP) != 0)
    return RES_MALFORMED;

  /* Paso 3: kid conocido. */
  if (strcmp(kid, KID_LOCAL) != 0)
    return RES_UNKNOWN_KEY;

  /* Paso 4: firma Ed25519 sobre (header_b64 + "." + payload_b64) = token[0..p2). */
  uint8_t sig[64];
  int sn = b64_decode(s_b64, s_len, sig, sizeof sig);
  if (sn != 64) return RES_BAD_SIGNATURE;
  if (crypto_ed25519_check(sig, pubkey, (const uint8_t *)token, (size_t)p2) != 0)
    return RES_BAD_SIGNATURE;

  /* Paso 5: decodificar payload. */
  char payload[1024];
  int pn = b64_decode(p_b64, p_len, (uint8_t *)payload, sizeof payload - 1);
  if (pn < 0) return RES_MALFORMED;
  payload[pn] = '\0';

  /* Paso 6: mid == MACHINE_ID. */
  char mid[32];
  if (json_get_str(payload, "mid", mid, sizeof mid) != 0) return RES_MALFORMED;
  if (strcmp(mid, MACHINE_ID) != 0) return RES_WRONG_MACHINE;

  /* Paso 7: now <= exp. */
  long long exp;
  if (json_get_int(payload, "exp", &exp) != 0) return RES_MALFORMED;
  if (now > exp) return RES_EXPIRED;

  /* Paso 8: jti no usado. */
  char jti[JTI_LEN];
  if (json_get_str(payload, "jti", jti, sizeof jti) != 0) return RES_MALFORMED;
  if (jti_is_used(jti)) return RES_ALREADY_USED;

  /* Paso 9: marcar jti ANTES de dispensar. Persiste en NVS (sobrevive reinicio). */
  jti_mark_used(jti);

  /* Paso 10: reportar items (aqui aun no accionamos motores; eso es el paso 4). */
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
  return RES_OK;
}

/* ---- Dispensado (paso 4) ------------------------------------------------- */

/* slot -> GPIO segun SLOT_MAP; -1 si el slot no esta mapeado en esta maquina. */
static int slot_to_gpio(int slot) {
  for (int i = 0; i < SLOT_MAP_N; i++)
    if (SLOT_MAP[i].slot == slot) return SLOT_MAP[i].gpio;
  return -1;
}

/* Espera la caida del producto: el sensor a LOW dentro de timeout_ms.
 * Devuelve true si se detecto; false si se agoto el tiempo. Bucle de sondeo
 * apretado para no perder un pulso corto del producto al pasar por el haz. */
static bool esperar_caida(unsigned long timeout_ms) {
  unsigned long t0 = millis();
  while (millis() - t0 < timeout_ms) {
    if (digitalRead(SENSOR_PIN) == SENSOR_DETECTADO) return true;
  }
  return false;
}

/* Mueve el motor de un slot `q` veces (una por unidad). Por cada unidad gira
 * (LOW) HASTA que el sensor confirme la caida O venza MOTOR_TIMEOUT_MS, lo que
 * pase primero; luego apaga SIEMPRE (HIGH). Devuelve cuantas unidades FALLARON
 * (0 = todas confirmadas). Un slot sin motor mapeado cuenta como q fallos. */
static int dispensar_slot(int slot, int q) {
  if (q < 1) q = 1;   /* el contrato garantiza q>=1, pero por si acaso */

  int gpio = slot_to_gpio(slot);
  if (gpio < 0) {
    Serial.print("  slot "); Serial.print(slot);
    Serial.println(" SIN MOTOR mapeado -> no se puede dispensar (DISPENSE_FAIL).");
    return q;
  }

  int fallos = 0;
  for (int u = 0; u < q; u++) {
    Serial.print("  slot "); Serial.print(slot);
    Serial.print(" (GPIO "); Serial.print(gpio);
    Serial.print(") unidad "); Serial.print(u + 1);
    Serial.print("/"); Serial.print(q);
    Serial.print(" -> MUEVE, esperando caida (max "); Serial.print(MOTOR_TIMEOUT_MS);
    Serial.println(" ms)");

    digitalWrite(gpio, MOTOR_MUEVE);
    bool cayo = esperar_caida(MOTOR_TIMEOUT_MS);
    digitalWrite(gpio, MOTOR_QUIETO);   /* apagar SIEMPRE (seguridad) */

    if (cayo) {
      Serial.println("     -> CAIDA confirmada por sensor (OK)");
    } else {
      fallos++;
      Serial.println("     -> TIMEOUT sin deteccion (DISPENSE_FAIL en esta unidad)");
    }

    if (u + 1 < q) delay(ENTRE_UNIDADES_MS);
  }
  return fallos;
}

/* Recorre el string de items `[{"s":3,"q":1},{"s":5,"q":2}]` y dispensa cada
 * uno. Como puede haber varios items, no se puede usar json_get_int (solo
 * encuentra el primero): se avanza manualmente buscando cada par "s"/"q".
 * Devuelve el total de unidades que FALLARON (0 = dispensado completo OK). */
static int dispensar_items(const char *items) {
  const char *cur = items;
  int n = 0;
  int fallos_tot = 0;
  while ((cur = strstr(cur, "\"s\":")) != NULL) {
    char *end;
    long slot = strtol(cur + 4, &end, 10);
    const char *qp = strstr(cur, "\"q\":");
    long q = 1;
    if (qp) q = strtol(qp + 4, NULL, 10);

    if (n > 0) delay(ENTRE_ITEMS_MS);
    fallos_tot += dispensar_slot((int)slot, (int)q);
    n++;

    /* avanzar: si hay "q":, pasar de largo; si no, avanzar tras el "s":. */
    cur = qp ? qp + 4 : end;
  }
  if (n == 0) Serial.println("  (sin items que dispensar)");
  return fallos_tot;
}

/* ---- Procesar un token completo ya recibido ----------------------------- */
static void procesar_token(const char *fuente, const String &tok) {
  /* Comando de PRUEBA: "!reset" borra el registro de jti en NVS. No es token. */
  if (tok == "!reset") {
    jti_reset_all();
    Serial.println("----- !reset -----");
    Serial.println("Registro de jti BORRADO en NVS y RAM. Maquina como nueva.");
    Serial.println("------------------");
    return;
  }

  char items[256];
  result_t r = verificar_token(tok.c_str(), tok.length(), g_pubkey, NOW_FIJO,
                               items, sizeof items);
  Serial.println("----- token recibido -----");
  Serial.print("Fuente:   "); Serial.println(fuente);
  Serial.print("Longitud: "); Serial.println(tok.length());
  Serial.print("Resultado: "); Serial.println(result_str(r));

  /* Solo si OK: el jti YA quedo marcado dentro de verificar_token (paso 9),
   * antes de llegar aqui. Recien ahora accionamos motores (contrato §5). */
  if (r == RES_OK) {
    Serial.print("Dispensando items="); Serial.println(items);
    int fallos = dispensar_items(items);
    if (fallos == 0) {
      Serial.println("Dispensado: OK (todas las unidades confirmadas por sensor).");
    } else {
      /* Contrato §7: DISPENSE_FAIL -> "Hubo un problema, contactanos" + registrar
       * para reembolso. El jti YA quedo consumido; no se re-dispensa. */
      Serial.print("Dispensado: DISPENSE_FAIL (");
      Serial.print(fallos);
      Serial.println(" unidad(es) sin confirmar). Registrar para reembolso/soporte.");
    }
  }
  Serial.println("--------------------------");
}

/* ---- Lector de stream con framing por inactividad ----------------------- *
 * (El struct LectorStream se define arriba, antes de cualquier funcion, para
 *  evitar el error de los prototipos auto-generados del IDE de Arduino.)      */
static void lector_service(LectorStream &L) {
  while (L.io->available() > 0) {
    char c = (char)L.io->read();
    if (c == '\r' || c == '\n') { L.ultimoByte = millis(); continue; }
    if (L.buf.length() < BUFFER_MAX) L.buf += c;
    L.ultimoByte = millis();
  }
  if (L.buf.length() > 0 && (millis() - L.ultimoByte) > FIN_LECTURA_MS) {
    procesar_token(L.nombre, L.buf);
    L.buf = "";
  }
}

LectorStream lectorGM65;
LectorStream lectorUSB;

void setup() {
  Serial.begin(115200);
  delay(200);

  Serial2.begin(GM65_BAUD, SERIAL_8N1, GM65_RX_PIN, GM65_TX_PIN);

  /* Motores: estado seguro de arranque. TODOS quietos (HIGH) por logica inversa,
   * para que ninguno gire durante el boot (contrato: dispensar solo tras OK). */
  for (int i = 0; i < MOTOR_PINS_N; i++) {
    pinMode(MOTOR_PINS[i], OUTPUT);
    digitalWrite(MOTOR_PINS[i], MOTOR_QUIETO);
  }

  /* Sensor de caida: pull-up interno (HIGH=libre, LOW=detectado). NPN colector
   * abierto -> sin riesgo de 5V en el pin (ver inventario). */
  pinMode(SENSOR_PIN, INPUT_PULLUP);

  lectorGM65 = { &Serial2, "GM65 (QR)", "", 0 };
  lectorUSB  = { &Serial,  "USB (pegado)", "", 0 };
  lectorGM65.buf.reserve(BUFFER_MAX);
  lectorUSB.buf.reserve(BUFFER_MAX);

  /* Cargar el registro persistente de jti usados desde NVS a la cache en RAM.
   * Esto es lo que hace que el ALREADY_USED sobreviva a reinicios/apagones. */
  jti_load_from_nvs();

  /* Decodificar la llave publica una sola vez. */
  int n = b64_decode(PUBKEY_B64, strlen(PUBKEY_B64), g_pubkey, sizeof g_pubkey);
  g_pubkey_ok = (n == 32);

  Serial.println();
  Serial.println("=== PASO 5b: verificacion + dispensado + anti-reuso en NVS ===");
  Serial.print("MACHINE_ID="); Serial.print(MACHINE_ID);
  Serial.print("  kid="); Serial.print(KID_LOCAL);
  Serial.print("  NOW(fijo)="); Serial.println((long)NOW_FIJO);
  Serial.println("Mapa demo: slot 3 -> D27 (Motor 1), slot 5 -> D14 (Motor 2).");
  Serial.print("Sensor de caida en D26 (INPUT_PULLUP). Estado actual: ");
  Serial.println(digitalRead(SENSOR_PIN) == SENSOR_DETECTADO ? "DETECTADO" : "LIBRE");
  Serial.print("jti usados cargados de NVS: "); Serial.print(jti_count);
  Serial.println(" (persisten tras reinicio). Enviar \"!reset\" por USB los borra.");
  if (!g_pubkey_ok) {
    Serial.println("ERROR: la llave publica no decodifico a 32 bytes. Revisa PUBKEY_B64.");
  } else {
    Serial.println("Llave publica cargada (32 bytes).");
  }
  Serial.println("Escanea un QR con el GM65, o pega el texto del token aqui y envia.");
}

void loop() {
  if (!g_pubkey_ok) return;   /* sin llave no tiene sentido verificar */
  lector_service(lectorGM65);
  lector_service(lectorUSB);
}
