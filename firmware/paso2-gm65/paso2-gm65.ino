/*
 * Dispensadora — Firmware (Depto. 03)
 * PASO 2: Leer el lector QR GM65 por UART2 e imprimir el texto de cada QR.
 *
 * Placa:   ESP32 Dev Module (Arduino IDE)
 * Monitor: Serial (USB) a 115200 para ver los resultados.
 * Lector:  GM65 por UART2  ->  RX2 = GPIO16, TX2 = GPIO17
 *
 * CONEXION (ver explicacion abajo y checklist de seguridad):
 *   GM65 VCC  -> 3.3V del ESP32   (recomendado: asi su TX sale a 3.3V, seguro para el ESP32)
 *   GM65 GND  -> GND del ESP32    (GND comun obligatorio)
 *   GM65 TX   -> GPIO16 (RX2 del ESP32)   <- por aqui llega el texto del QR
 *   GM65 RX   -> GPIO17 (TX2 del ESP32)   <- solo se usa para enviarle comandos (opcional aqui)
 *
 * OJO NIVEL LOGICO: el GPIO del ESP32 NO tolera 5V. Si alimentas el GM65 a 5V,
 * su linea TX puede salir a 5V y dana el RX2. Por eso: alimentar el GM65 a 3.3V,
 * o usar level shifter / divisor en la linea TX. Medir con multimetro si hay duda.
 *
 * BAUDIOS: el GM65 sale de fabrica normalmente a 9600 bps. Si no lees nada,
 * prueba tambien 115200 (ver GM65_BAUD abajo). El baudio se cambia escaneando
 * los codigos de configuracion del manual del GM65.
 *
 * MODO DE DISPARO: para esta prueba conviene el GM65 en modo continuo / auto-sensado
 * (escanea solo al ver un codigo). Si tu modulo esta en modo "manual", dispara con
 * su boton/pin TRIG. Los codigos para cambiar el modo estan en el manual del GM65.
 */

// ----- UART2 hacia el GM65 -----
const int GM65_RX_PIN = 16;   // RX2 del ESP32  <- recibe del TX del GM65
const int GM65_TX_PIN = 17;   // TX2 del ESP32  -> va al RX del GM65
const long GM65_BAUD  = 9600; // si no lee, probar 115200

// ----- Framing por inactividad -----
// Un QR de 200+ caracteres llega como un chorro de bytes. Acumulamos en un
// buffer y damos por terminada la lectura cuando pasan FIN_LECTURA_MS sin que
// lleguen bytes nuevos. Asi imprimimos el token completo en una sola linea,
// sin depender de que el GM65 mande un terminador (CR/LF).
const unsigned long FIN_LECTURA_MS = 60;   // silencio que marca "fin del QR"
const size_t        BUFFER_MAX     = 1024; // holgado para el JWT (~250 chars)

String   buffer = "";
unsigned long ultimoByte = 0;

void setup() {
  Serial.begin(115200);
  delay(200);

  // UART2: baudios, 8N1, y los pines RX/TX explicitos.
  Serial2.begin(GM65_BAUD, SERIAL_8N1, GM65_RX_PIN, GM65_TX_PIN);

  buffer.reserve(BUFFER_MAX);

  Serial.println();
  Serial.println("=== PASO 2: lector GM65 por UART2 ===");
  Serial.print("Escuchando GM65 a ");
  Serial.print(GM65_BAUD);
  Serial.println(" bps (RX2=GPIO16, TX2=GPIO17).");
  Serial.println("Escanea un QR para ver su contenido...");
}

void loop() {
  // 1) Ir vaciando lo que llega del GM65 al buffer.
  while (Serial2.available() > 0) {
    char c = (char)Serial2.read();

    // Ignorar CR/LF que algunos GM65 anaden al final; el framing lo hace el timeout.
    if (c == '\r' || c == '\n') {
      ultimoByte = millis();
      continue;
    }

    if (buffer.length() < BUFFER_MAX) {
      buffer += c;
    }
    ultimoByte = millis();
  }

  // 2) Si hay algo en el buffer y ya no llegan bytes, es un QR completo.
  if (buffer.length() > 0 && (millis() - ultimoByte) > FIN_LECTURA_MS) {
    Serial.println("----- QR escaneado -----");
    Serial.print("Longitud: ");
    Serial.println(buffer.length());
    Serial.print("Contenido: ");
    Serial.println(buffer);
    Serial.println("------------------------");
    buffer = "";
  }
}
