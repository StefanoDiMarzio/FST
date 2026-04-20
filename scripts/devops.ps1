param(
    [ValidateSet("init", "build", "up", "down", "restart", "logs", "status", "smoke", "clean")]
    [string]$Action = "up"
)

$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

function Ensure-EnvFile {
    if (-not (Test-Path ".env")) {
        Copy-Item ".env.example" ".env"
        Write-Host "Creato .env da .env.example"
    }
}

function Compose {
    docker compose @args
}

switch ($Action) {
    "init" {
        Ensure-EnvFile
        go mod tidy
    }
    "build" {
        Ensure-EnvFile
        Compose build
    }
    "up" {
        Ensure-EnvFile
        Compose up --build -d
        Write-Host "API:        http://localhost:8080/health"
        Write-Host "Metrics:    http://localhost:8080/metrics"
        Write-Host "Prometheus: http://localhost:9090"
        Write-Host "Grafana:    http://localhost:3000"
    }
    "down" {
        Compose down
    }
    "restart" {
        Ensure-EnvFile
        Compose up --build -d
    }
    "logs" {
        Compose logs -f --tail=200
    }
    "status" {
        Compose ps
    }
    "smoke" {
        Invoke-RestMethod "http://localhost:8080/health"
        Invoke-WebRequest "http://localhost:8080/metrics" -UseBasicParsing | Select-Object -ExpandProperty StatusCode
        Invoke-WebRequest "http://localhost:9090/-/ready" -UseBasicParsing | Select-Object -ExpandProperty StatusCode
        Invoke-WebRequest "http://localhost:3000/api/health" -UseBasicParsing | Select-Object -ExpandProperty StatusCode
    }
    "clean" {
        Compose down -v --remove-orphans
    }
}
