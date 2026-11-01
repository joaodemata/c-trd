# Genera los archivos de /opt/ctrd/env/ del VPS a partir del .env local.
# La salida son comandos listos para pegar en la terminal SSH del VPS: crean caddy.env, mongo.env,
# dev.env, qa.env, sandbox.env y prod.env, y al final muestran los usuarios de Mongo a crear.
#
# Uso (PowerShell, desde la raiz del repo):
#   .\scripts\generate_env_files.ps1 -Domain midominio.com -AcmeEmail yo@correo.com
#   .\scripts\generate_env_files.ps1 -Domain midominio.com -AcmeEmail yo@correo.com | Set-Clipboard
#
# Las contrasenas de Mongo se generan nuevas en cada ejecucion: pega todo de una sola corrida.
# La salida tiene secretos: no la guardes en el repo ni la compartas.

param(
    [Parameter(Mandatory = $true)][string]$Domain,
    [Parameter(Mandatory = $true)][string]$AcmeEmail,
    [string]$EnvPath = ".env"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $EnvPath)) {
    throw "No existe $EnvPath"
}

# Lee el .env como KEY=VALUE, ignorando comentarios y lineas vacias
$source = @{}
foreach ($line in Get-Content $EnvPath) {
    $trimmed = $line.Trim()
    if ($trimmed -eq "" -or $trimmed.StartsWith("#")) { continue }
    $index = $trimmed.IndexOf("=")
    if ($index -lt 1) { continue }
    $key = $trimmed.Substring(0, $index).Trim()
    $value = $trimmed.Substring($index + 1).Trim()
    if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
        $value = $value.Substring(1, $value.Length - 2)
    }
    $source[$key] = $value
}

# Contrasena alfanumerica (segura dentro de la URI de Mongo sin escapar)
function New-Password([int]$length = 32) {
    $chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789".ToCharArray()
    $bytes = New-Object byte[] $length
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)
    $rng.Dispose()
    $result = ""
    foreach ($b in $bytes) { $result += $chars[$b % $chars.Length] }
    return $result
}

# Valor entre comillas simples: Docker Compose y el shell no interpretan $ ni espacios
function Format-Value([string]$key, [string]$value) {
    if ($value.Contains("'")) {
        throw "El valor de $key tiene comillas simples, cambialo a mano"
    }
    return "$key='$value'"
}

# Imprime un comando que crea el archivo en el VPS con el contenido dado
function Write-EnvFile([string]$name, [string[]]$lines) {
    Write-Output "sudo tee /opt/ctrd/env/$name > /dev/null <<'EOF'"
    foreach ($l in $lines) { Write-Output $l }
    Write-Output "EOF"
    Write-Output ""
}

# Variables del .env que se copian igual a todos los entornos (APIs externas y secretos de proveedores)
$sharedKeys = @("JM_CTRD_STOCK_API_URL", "JM_CTRD_API_KEY")
$sharedKeys += $source.Keys | Where-Object { $_ -like "JM_CTRD_SECRET_*" } | Sort-Object

$baseDb = $source["JM_CTRD_MDB_NAME"]
if ([string]::IsNullOrWhiteSpace($baseDb)) { $baseDb = "ctrd" }

# Entornos: sufijo de la base, subdominio del frontend y si corren los cronjobs
$environments = @(
    @{ Name = "dev";     Front = "dev.app.$Domain";     Cron = "false" },
    @{ Name = "qa";      Front = "qa.app.$Domain";      Cron = "false" },
    @{ Name = "sandbox"; Front = "sandbox.app.$Domain"; Cron = "true" },
    @{ Name = "prod";    Front = "app.$Domain";         Cron = "true" }
)

Write-Output "sudo mkdir -p /opt/ctrd/env"
Write-Output ""

Write-EnvFile "caddy.env" @(
    (Format-Value "CTRD_DOMAIN" $Domain),
    (Format-Value "CTRD_ACME_EMAIL" $AcmeEmail)
)

$rootPassword = New-Password
Write-EnvFile "mongo.env" @(
    (Format-Value "MONGO_INITDB_ROOT_USERNAME" "root"),
    (Format-Value "MONGO_INITDB_ROOT_PASSWORD" $rootPassword)
)

$mongoUsers = @()
foreach ($environment in $environments) {
    $db = "$($baseDb)_$($environment.Name)"
    $password = New-Password
    $mongoUsers += "db.createUser({ user: `"$db`", pwd: `"$password`", roles: [{ role: `"readWrite`", db: `"$db`" }] })"

    $lines = @(
        (Format-Value "JM_CTRD_MDB_HOST" "mongodb://$($db):$($password)@mongo:27017/$($db)?authSource=admin&replicaSet=rs0"),
        (Format-Value "JM_CTRD_MDB_NAME" $db),
        (Format-Value "JM_CTRD_MDB_USER" $db),
        (Format-Value "JM_CTRD_CORS_ORIGINS" "https://$($environment.Front)"),
        (Format-Value "JM_CTRD_CRON_ENABLED" $environment.Cron)
    )
    foreach ($key in $sharedKeys) {
        if ($source.ContainsKey($key)) {
            $lines += Format-Value $key $source[$key]
        }
    }

    Write-EnvFile "$($environment.Name).env" $lines
}

Write-Output "sudo chmod 640 /opt/ctrd/env/*.env"
Write-Output "id github-runner >/dev/null 2>&1 && sudo chgrp -R github-runner /opt/ctrd/env"
Write-Output ""

# keyFile del replica set rs0 (obligatorio con autenticacion). Solo se crea si no existe;
# 999 es el usuario mongodb dentro de la imagen oficial
Write-Output "sudo mkdir -p /opt/ctrd/mongo"
Write-Output "[ -f /opt/ctrd/mongo/keyfile ] || openssl rand -base64 756 | sudo tee /opt/ctrd/mongo/keyfile > /dev/null"
Write-Output "sudo chown 999:999 /opt/ctrd/mongo/keyfile && sudo chmod 400 /opt/ctrd/mongo/keyfile"
Write-Output ""
Write-Output "# ---------------------------------------------------------------------------"
Write-Output "# Despues del primer 'docker compose up -d mongo', crear los usuarios de cada entorno:"
Write-Output "#   docker compose -f deploy/docker-compose.yml exec mongo mongosh -u root -p '$rootPassword' --authenticationDatabase admin"
Write-Output "# y pegar dentro de mongosh:"
Write-Output "use admin"
foreach ($user in $mongoUsers) { Write-Output $user }
