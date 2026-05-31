<#
  demo.ps1 — Windows/PowerShell demo helper for "Ring of the Middle Earth".
  Replaces the bash/make/jq commands in the video scripts. Run from the repo root:

      .\demo.ps1 <command>

  Commands that need NO Docker (local engine only):
      check     show there are NO unit-id literals in game logic (A2/B1)
      test      run all Go unit tests (25/25)
      run       start the engine + UI locally at http://localhost:8080
      verify    drive the live engine and assert all demo scenarios (15/15)

  Commands that need Docker (run '.\demo.ps1 up' first):
      up        start ONLY Kafka + Schema Registry (fast; enough for K1-K5)
      up-all    start the FULL stack incl. 3 Go nodes (heavy build; for Scenario 3)
      topics    create the 10 Kafka topics (K1)
      describe  kafka-topics --describe         (evidence for K1)
      schemas   register all Avro schemas (K2)
      subjects  list registered schema subjects (K2)
      evolve    register OrderValidated V2 while V1 stays (K3)
      down      stop and remove the stack
#>
param([Parameter(Position = 0)][string]$cmd = "help")

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root
$SR = "http://localhost:8085"
function GoPath { if (-not ($env:Path -like "*Go\bin*")) { $env:Path += ";C:\Program Files\Go\bin" } }

switch ($cmd) {

  "check" {
    Write-Host "Searching game logic for hardcoded unit ids (should find NOTHING)..." -ForegroundColor Cyan
    $hits = Select-String -Path .\option-b\internal\game\*.go, .\option-b\internal\engine\*.go `
      -Pattern '"witch-king"', '"gandalf"', '"sauron"', '"frodo"', '"saruman"', '"nazgul-2"'
    if ($hits) { $hits } else { Write-Host "  No matches -> game logic is fully config-driven (A2/B1). " -ForegroundColor Green }
  }

  "test" { GoPath; Push-Location option-b; go test ./...; Pop-Location }

  "run" {
    GoPath; Push-Location option-b
    Write-Host "Engine + UI -> http://localhost:8080/   (Ctrl+C to stop)" -ForegroundColor Green
    go run ./cmd/server -config ../config -ui ../ui
    Pop-Location
  }

  "verify" { powershell -ExecutionPolicy Bypass -File .\verify-scenarios.ps1 -Base "http://localhost:8080" }

  "up" {
    # Only the infra needed for the Kafka demo (K1-K5). Fast: just image pulls.
    docker compose up -d kafka schema-registry
    Write-Host "Waiting for Kafka + Schema Registry to be ready..." -ForegroundColor Cyan
    Start-Sleep -Seconds 12
    Write-Host "Now run:  .\demo.ps1 topics   then   .\demo.ps1 schemas" -ForegroundColor Green
  }

  "up-all" {
    # Full stack incl. the 3 Go engine nodes (heavy: builds Go images). For Scenario 3.
    docker compose up -d --build
    Write-Host "Full stack up. Light=:8081  Dark=:8082  (also run topics + schemas)" -ForegroundColor Green
  }

  "topics" {
    Get-Content -Raw .\kafka\create-topics.sh | docker compose exec -T -e BROKER=kafka:29092 -e REPL=1 kafka bash -s
  }

  "describe" {
    docker compose exec kafka kafka-topics --bootstrap-server kafka:29092 --describe | Select-Object -First 60
  }

  "schemas" {
    $map = [ordered]@{
      "game.orders.raw-value"       = "order-submitted.avsc"
      "game.orders.validated-value" = "order-validated.avsc"
      "game.events.unit-value"      = "unit-moved.avsc"
      "game.events.region-value"    = "region-control-changed.avsc"
      "game.events.path-value"      = "path-status-changed.avsc"
      "game.broadcast-value"        = "world-state-snapshot.avsc"
      "game.ring.position-value"    = "ring-bearer-moved.avsc"
      "game.ring.detection-value"   = "ring-bearer-detected.avsc"
      "game.dlq-value"              = "dlq-entry.avsc"
      "rotr.events.BattleResolved"  = "battle-resolved.avsc"
      "rotr.events.PathCorrupted"   = "path-corrupted.avsc"
      "rotr.events.GameOver"        = "game-over.avsc"
      "rotr.events.RingBearerSpotted" = "ring-bearer-spotted.avsc"
    }
    foreach ($subject in $map.Keys) {
      $schema = Get-Content -Raw ".\kafka\schemas\$($map[$subject])"
      $payload = @{ schema = $schema } | ConvertTo-Json -Compress
      Invoke-RestMethod -Method Post -ContentType "application/vnd.schemaregistry.v1+json" `
        -Uri "$SR/subjects/$subject/versions" -Body $payload | Out-Null
      Write-Host "  registered $subject" -ForegroundColor Green
    }
  }

  "subjects" { Invoke-RestMethod "$SR/subjects" | ConvertTo-Json }

  "evolve" {
    $schema = Get-Content -Raw .\kafka\schemas\order-validated-v2.avsc
    $payload = @{ schema = $schema } | ConvertTo-Json -Compress
    Write-Host "Registering OrderValidated V2 (adds nullable routeRiskScore)..." -ForegroundColor Cyan
    Invoke-RestMethod -Method Post -ContentType "application/vnd.schemaregistry.v1+json" `
      -Uri "$SR/subjects/game.orders.validated-value/versions" -Body $payload
    Write-Host "Versions now present:" -ForegroundColor Green
    Invoke-RestMethod "$SR/subjects/game.orders.validated-value/versions"
  }

  "down" { docker compose down -v }

  default {
    Write-Host "Usage: .\demo.ps1 <check|test|run|verify|up|topics|describe|schemas|subjects|evolve|down>" -ForegroundColor Yellow
  }
}
