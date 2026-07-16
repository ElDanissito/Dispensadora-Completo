/*
 * Dispensadora — Firmware (Depto. 03)
 * PASO 1: Inicializar pines de motor y probar el giro del Motor 1.
 *
 * Placa:   ESP32 Dev Module (Arduino IDE)
 * Fuente:  Motores por buck 12V->5V (NO por USB). GND comun obligatorio.
 *
 * LOGICA INVERSA (confirmada en hardware/inventario-actual.md):
 *   HIGH = motor QUIETO
 *   LOW  = motor MUEVE
 * Por eso los 4 pines arrancan en HIGH en setup(): ningun motor se
 * mueve solo durante el arranque/boot.
 *
 * Que hace este paso:
 *   - Deja los 4 motores quietos (HIGH).
 *   - Cada 5 s pulsa el Motor 1 (D27) a LOW durante ~1 s para verlo girar.
 *   - Imprime por Serial lo que va haciendo (Monitor Serie a 115200).
 *
 * Pinout real CORREGIDO por Daniel (el orden estaba al reves):
 *   Motor 1 = D27   Motor 2 = D14   Motor 3 = D12   Motor 4 = D13
 */

// ----- Pines de motor -----
const int MOTOR_1 = 27;  // D27
const int MOTOR_2 = 14;  // D14
const int MOTOR_3 = 12;  // D12
const int MOTOR_4 = 13;  // D13

// ----- Estados de la logica inversa (para que se lea claro) -----
const int MOTOR_QUIETO = HIGH;  // HIGH = no se mueve
const int MOTOR_MUEVE  = LOW;   // LOW  = gira

// ----- Tiempos de la prueba -----
const unsigned long PULSO_MS  = 1000;  // ~1 s girando
const unsigned long PAUSA_MS  = 5000;  // cada 5 s

void setup() {
  Serial.begin(115200);
  delay(200);  // pequena espera para que el Monitor Serie enganche

  // Configurar los 4 pines como salida.
  pinMode(MOTOR_1, OUTPUT);
  pinMode(MOTOR_2, OUTPUT);
  pinMode(MOTOR_3, OUTPUT);
  pinMode(MOTOR_4, OUTPUT);

  // Estado seguro de arranque: TODOS quietos (HIGH) por la logica inversa.
  digitalWrite(MOTOR_1, MOTOR_QUIETO);
  digitalWrite(MOTOR_2, MOTOR_QUIETO);
  digitalWrite(MOTOR_3, MOTOR_QUIETO);
  digitalWrite(MOTOR_4, MOTOR_QUIETO);

  Serial.println();
  Serial.println("=== PASO 1: motores inicializados en HIGH (quietos) ===");
  Serial.println("Probando Motor 1 (D27): pulso LOW ~1s cada 5s.");
}

void loop() {
  // Girar Motor 1 durante ~1 s.
  Serial.println("Motor 1 -> MUEVE (LOW)");
  digitalWrite(MOTOR_1, MOTOR_MUEVE);
  delay(PULSO_MS);

  // Detener Motor 1.
  Serial.println("Motor 1 -> QUIETO (HIGH)");
  digitalWrite(MOTOR_1, MOTOR_QUIETO);

  // Esperar antes del proximo pulso.
  delay(PAUSA_MS);
}
