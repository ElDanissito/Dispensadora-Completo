# Compila y ejecuta la PoC contra los vectores de prueba oficiales (v2) y compara
# los resultados con `resultados-esperados.md`. Equivalente a run-poc.sh pero para
# Windows con MSVC (cl.exe), que es el toolchain disponible cuando no hay
# gcc/clang/zig. Criterio de éxito: los resultados coinciden.
#
# Uso:   powershell -ExecutionPolicy Bypass -File run-poc.ps1
# Detecta el Developer Command Prompt de Visual Studio (vcvars64.bat) vía vswhere,
# y si no, cae a gcc/clang/zig si están en el PATH.

$ErrorActionPreference = 'Stop'
$dir  = $PSScriptRoot
$VECT = Join-Path $dir '..\..\especificaciones\vectores-prueba'
$PUB  = Join-Path $VECT 'llave-publica-k1.txt'
$NOW  = '1752460900'   # RTC de referencia (resultados-esperados.md)
$exe  = Join-Path $dir 'verificar.exe'

$srcs = @('verificar.c', 'monocypher\monocypher.c', 'monocypher\monocypher-ed25519.c')

# --- localizar y ejecutar el compilador -----------------------------------
function Find-Vcvars {
    $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
    if (Test-Path $vswhere) {
        $inst = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath 2>$null
        if ($inst) {
            $vc = Join-Path $inst 'VC\Auxiliary\Build\vcvars64.bat'
            if (Test-Path $vc) { return $vc }
        }
    }
    # Fallback: buscar en ubicaciones típicas (incluye BuildTools).
    $cand = Get-ChildItem "C:\Program Files*\Microsoft Visual Studio\*\*\VC\Auxiliary\Build\vcvars64.bat" -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($cand) { return $cand.FullName }
    return $null
}

$compiled = $false
$vcvars = Find-Vcvars
if ($vcvars) {
    Write-Host "Compilador: MSVC (cl.exe)"
    $srcList = $srcs -join ' '
    $build = "call `"$vcvars`" && cd /d `"$dir`" && cl /nologo /O2 /W3 /I monocypher /Fe:verificar.exe $srcList"
    cmd /c $build | Out-Null
    if ($LASTEXITCODE -eq 0 -and (Test-Path $exe)) { $compiled = $true }
} else {
    foreach ($cc in @('gcc','clang')) {
        if (Get-Command $cc -ErrorAction SilentlyContinue) {
            Write-Host "Compilador: $cc"
            & $cc -O2 -Wall -o $exe $srcs -Imonocypher
            if ($LASTEXITCODE -eq 0) { $compiled = $true }
            break
        }
    }
}
if (-not $compiled) {
    Write-Error "No pude compilar: no encontré MSVC (cl.exe) ni gcc/clang. Ver README.md."
    exit 127
}
Write-Host "Compilado: $exe`n"

# --- ejecutar y comparar --------------------------------------------------
function Code([string[]]$tokenPaths) {
    # Devuelve la 3ª columna (código) de cada línea de salida, en orden.
    # El banner del programa va a stderr (no se captura aquí); solo tomamos
    # de stdout las líneas de resultado (contienen "->").
    # Nota: la ruta puede contener espacios, así que separamos por "->" y no
    # por espacios; el código es el primer token tras la flecha.
    $out = & $exe --pubkey $PUB --now $NOW @tokenPaths
    $out | Where-Object { $_ -match '->\s+(\S+)' } | ForEach-Object {
        if ($_ -match '->\s+(\S+)') { $matches[1] }
    }
}

$fail = @()
function Check($desc, $exp, $got) {
    if ($got -eq $exp) {
        "  [OK] {0,-32} {1}" -f $desc, $got
    } else {
        "  [X ] {0,-32} esperado={1} obtenido={2}" -f $desc, $exp, $got
        $script:fail += $desc
    }
}

Write-Host "Casos individuales:"
Check 'token-valido'     'OK'            (Code @((Join-Path $VECT 'token-valido.txt')))
Check 'token-expirado'   'EXPIRED'       (Code @((Join-Path $VECT 'token-expirado.txt')))
Check 'token-firma-mala' 'BAD_SIGNATURE' (Code @((Join-Path $VECT 'token-firma-mala.txt')))

Write-Host "`nAnti-reuso (mismo token dos veces en la misma sesión):"
$doble = Code @((Join-Path $VECT 'token-valido.txt'), (Join-Path $VECT 'token-valido.txt'))
Check '1er uso' 'OK'           $doble[0]
Check '2do uso' 'ALREADY_USED' $doble[1]

Write-Host ""
if ($fail.Count -eq 0) {
    Write-Host "RESULTADO: TODO COINCIDE con resultados-esperados.md (v2)"
    exit 0
} else {
    Write-Host "RESULTADO: $($fail.Count) caso(s) NO coinciden"
    exit 1
}
