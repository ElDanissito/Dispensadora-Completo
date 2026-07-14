#!/usr/bin/env bash
# Compila y ejecuta la PoC contra los vectores de prueba oficiales, y compara
# los resultados con `resultados-esperados.md`. Criterio de éxito: coinciden.
#
# Uso:   bash run-poc.sh
# Detecta automáticamente el compilador: gcc, clang o `zig cc`.
set -u

cd "$(dirname "$0")"
VECT=../../especificaciones/vectores-prueba
PUB=$VECT/llave-publica-k1.txt
NOW=1752460900   # RTC de referencia (resultados-esperados.md)

# --- elegir compilador ----------------------------------------------------
if command -v gcc >/dev/null 2>&1;       then CC="gcc";     CCDESC="gcc"
elif command -v clang >/dev/null 2>&1;   then CC="clang";   CCDESC="clang"
elif command -v zig >/dev/null 2>&1;     then CC="zig cc";  CCDESC="zig cc"
else
    echo "ERROR: no encontré gcc, clang ni zig en el PATH." >&2
    echo "Instala uno (ver README.md) y vuelve a correr." >&2
    exit 127
fi
echo "Compilador: $CCDESC"

# --- compilar -------------------------------------------------------------
$CC -O2 -Wall -Wextra -o verificar \
    verificar.c monocypher/monocypher.c monocypher/monocypher-ed25519.c \
    -Imonocypher || { echo "FALLÓ la compilación" >&2; exit 1; }
echo "Compilado: ./verificar"
echo

# --- ejecutar y comparar --------------------------------------------------
# Formato: "archivo esperado". El caso doble demuestra el anti-reuso.
run() { ./verificar --pubkey "$PUB" --now "$NOW" "$@" 2>/dev/null; }

declare -a FAIL=()
check() { # <descripcion> <esperado> <salida-real>
    local desc="$1" exp="$2" got="$3"
    if [ "$got" = "$exp" ]; then
        printf "  ✔ %-32s %s\n" "$desc" "$got"
    else
        printf "  X %-32s esperado=%s obtenido=%s\n" "$desc" "$exp" "$got"
        FAIL+=("$desc")
    fi
}

extract() { awk '{print $3}'; }  # 3ª columna = código

echo "Casos individuales:"
check "token-valido"     OK            "$(run $VECT/token-valido.txt     | extract)"
check "token-expirado"   EXPIRED       "$(run $VECT/token-expirado.txt   | extract)"
check "token-firma-mala" BAD_SIGNATURE "$(run $VECT/token-firma-mala.txt | extract)"

echo
echo "Anti-reuso (mismo token dos veces en la misma sesión):"
DOBLE=$(run $VECT/token-valido.txt $VECT/token-valido.txt)
check "1er uso"  OK           "$(echo "$DOBLE" | sed -n 1p | extract)"
check "2do uso"  ALREADY_USED "$(echo "$DOBLE" | sed -n 2p | extract)"

echo
if [ ${#FAIL[@]} -eq 0 ]; then
    echo "RESULTADO: TODO COINCIDE con resultados-esperados.md ✅"
    exit 0
else
    echo "RESULTADO: ${#FAIL[@]} caso(s) NO coinciden ❌"
    exit 1
fi
