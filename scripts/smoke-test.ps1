$ErrorActionPreference = "Stop"

$checks = @(
    "http://localhost:8080/health",
    "http://localhost:8080/metrics",
    "http://localhost:9090/-/ready",
    "http://localhost:3000/api/health"
)

foreach ($url in $checks) {
    $response = Invoke-WebRequest $url -UseBasicParsing
    Write-Host "$url -> $($response.StatusCode)"
}
