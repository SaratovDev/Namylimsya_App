# ============================================================
#  backup-db.ps1 — бэкап PostgreSQL для prod-like стека.
#
#  Требования:
#    - Docker;
#    - запущенный стек namylimsya-prod (infra/docker-compose.prod.yml).
#
#  Использование (из корня репозитория):
#    .\infra\backup-db.ps1                 # дамп в infra\backups, хранить 14 шт.
#    .\infra\backup-db.ps1 -Keep 30        # хранить 30 последних
#
#  Восстановление: см. README → «Бэкапы и восстановление».
# ============================================================
param(
  [string]$ComposeProject = "namylimsya-prod",
  [int]$Keep = 14,
  [string]$BackupDir = "backups"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$backupPath = Join-Path $repoRoot $BackupDir
New-Item -ItemType Directory -Force -Path $backupPath | Out-Null

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$file = Join-Path $backupPath "namylimsya-$stamp.sql.gz"

$container = docker ps --format "{{.Names}}" |
  Where-Object { $_ -like "$ComposeProject-postgres-*" } |
  Select-Object -First 1

if (-not $container) {
  throw "Не найден контейнер Postgres ($ComposeProject-postgres-*). Стек запущен?"
}

Write-Host "Бэкап из $container -> $file"

# pg_dump выполняется ВНУТРИ контейнера, поэтому используем его собственные
# env-переменные POSTGRES_USER/POSTGRES_DB/POSTGRES_PASSWORD (не парсим .env).
docker exec $container sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -h localhost "$POSTGRES_DB"' |
  gzip > $file

if ($LASTEXITCODE -ne 0 -or -not (Test-Path $file)) {
  throw "pg_dump завершился с ошибкой"
}

$size = [math]::Round((Get-Item $file).Length / 1KB, 1)
Write-Host "Готово: $file ($size KB)"

# Ротация: оставляем последние $Keep дампов.
$files = Get-ChildItem -Path $backupPath -Filter "namylimsya-*.sql.gz" |
  Sort-Object LastWriteTime -Descending
if ($files.Count -gt $Keep) {
  $files | Select-Object -Skip $Keep | Remove-Item -Force
  Write-Host "Удалено старых бэкапов: $($files.Count - $Keep)"
}
