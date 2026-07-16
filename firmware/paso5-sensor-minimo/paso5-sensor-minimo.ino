/*
 * Dispensadora — Firmware (Depto. 03)
 * PASO 5 (parte A): lectura MINIMA del sensor de caida E18-D80NK para calibrar.
 *
 * Placa:   ESP32 Dev Module (Arduino IDE)
 * Monitor: Serial (USB) a 115200.
 * Sensor:  E18-D80NK  ->  OUT en D26.
 *
 * Conexion y nivel logico (medido, ver hardware/inventario-actual.md):
 *   - E18 alimentado a 5V. Es NPN colector abierto: su OUT SOLO tira a GND al
 *     detectar, nunca empuja 5V -> no hay riesgo para el D26.
 *   - En reposo el OUT queda flotando (~2.2V, por debajo del HIGH fiable del
 *     ESP32). Por eso se usa pinMode(D26, INPUT_PULLUP): el pull-up interno lo
 *     sube a 3.3V limpio en reposo, y el sensor lo tira a 0V al detectar.
 *   - LOGICA: HIGH = LIBRE (nada delante) · LOW = DETECTADO (producto/mano).
 *
 * Objetivo de este sketch: ajustar la distancia del sensor (tornillo del E18)
 * viendo LIBRE/DETECTADO en vivo, antes de integrarlo al dispensado.
 */

const int  SENSOR_PIN       = 26;    /* D26 (OUT del E18) */
const int  SENSOR_LIBRE     = HIGH;  /* pull-up en reposo */
const int  SENSOR_DETECTADO = LOW;   /* NPN tira a GND al detectar */

int           estadoPrev = -1;       /* para imprimir solo en los cambios */
unsigned long ultimoHeartbeat = 0;

void setup() {
  Serial.begin(115200);
  delay(200);

  pinMode(SENSOR_PIN, INPUT_PULLUP);

  Serial.println();
  Serial.println("=== PASO 5A: calibracion del sensor E18-D80NK (D26) ===");
  Serial.println("HIGH = LIBRE   ·   LOW = DETECTADO");
  Serial.println("Pon la mano/producto delante y ajusta el tornillo del sensor.");
}

void loop() {
  int nivel = digitalRead(SENSOR_PIN);

  /* Imprimir solo cuando cambia el estado (evita inundar el Monitor). */
  if (nivel != estadoPrev) {
    estadoPrev = nivel;
    if (nivel == SENSOR_DETECTADO) Serial.println(">> DETECTADO (LOW)");
    else                           Serial.println("   LIBRE     (HIGH)");
  }

  /* Latido cada 2 s para saber que sigue vivo y ver el estado actual. */
  if (millis() - ultimoHeartbeat > 2000) {
    ultimoHeartbeat = millis();
    Serial.print("[estado] ");
    Serial.println(nivel == SENSOR_DETECTADO ? "DETECTADO" : "LIBRE");
  }

  delay(20);   /* muestreo suave, suficiente para calibrar a ojo */
}
