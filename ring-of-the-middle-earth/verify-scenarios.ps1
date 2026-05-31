# Empirically verifies the demo scenario order-sequences against a live engine.
# These exact sequences are baked into the UI's one-click scenario runner, so this
# script is the source of truth that they actually produce the observable outcome.
#
#   powershell -ExecutionPolicy Bypass -File verify-scenarios.ps1
param([string]$Base = "http://localhost:8080")

$ErrorActionPreference = "Stop"
$pass = 0; $fail = 0
function Check($name, $cond, $detail) {
  if ($cond) { Write-Host "  PASS  $name" -ForegroundColor Green; $script:pass++ }
  else { Write-Host "  FAIL  $name  -> $detail" -ForegroundColor Red; $script:fail++ }
}

# --- minimal client (manual JSON so single-element arrays stay arrays) ---
function Order($side, $type, $unit, $turn, $fields) {
  $sb = '{"orderType":"' + $type + '","playerId":"' + $side + '","unitId":"' + $unit + '","turn":' + $turn
  foreach ($k in $fields.Keys) {
    $v = $fields[$k]
    if ($k -eq 'pathIds' -or $k -eq 'newPathIds') {
      $sb += ',"' + $k + '":[' + ((@($v) | ForEach-Object { '"' + $_ + '"' }) -join ',') + ']'
    } else {
      $sb += ',"' + $k + '":"' + $v + '"'
    }
  }
  $sb += '}'
  try { return Invoke-RestMethod "$Base/order" -Method Post -ContentType "application/json" -Body $sb }
  catch { return @{ errorCode = "HTTP_ERR" } }
}
function Tick { Invoke-RestMethod "$Base/game/tick" -Method Post }
function State($side) { Invoke-RestMethod "$Base/game/state?side=$side" }
function Reset { Invoke-RestMethod "$Base/game/reset" -Method Post | Out-Null }
function Turn { (State "light").turn }

Write-Host "`n=== Endpoints ===" -ForegroundColor Cyan
$cu = Invoke-RestMethod "$Base/config/units"; Check "/config/units returns units" ($cu.units.Count -ge 13) "got $($cu.units.Count)"
$cm = Invoke-RestMethod "$Base/config/map"; Check "/config/map regions=22" ($cm.regions.Count -eq 22) "got $($cm.regions.Count)"
Check "/config/map paths>=35" ($cm.paths.Count -ge 35) "got $($cm.paths.Count)"

Write-Host "`n=== Scenario 1: Information Hiding ===" -ForegroundColor Cyan
Reset
Order "light" "ASSIGN_ROUTE" "ring-bearer" (Turn) @{ pathIds = @("shire-to-bree", "bree-to-weathertop") } | Out-Null
Order "dark"  "DEPLOY_NAZGUL" "witch-king" (Turn) @{ targetRegion = "bree" } | Out-Null
1..4 | ForEach-Object { Tick | Out-Null }
$dark = State "dark"; $light = State "light"
Check "Dark cannot see ring bearer region" ($dark.ringBearerRegion -eq $null -or $dark.ringBearerRegion -eq "") "got '$($dark.ringBearerRegion)'"
Check "Dark ring-bearer unit region empty" ($dark.units.'ring-bearer'.region -eq "") "got '$($dark.units.'ring-bearer'.region)'"
Check "Light DOES see ring bearer region" ($light.ringBearerRegion -ne "") "got '$($light.ringBearerRegion)'"
Check "Detection fired (Dark last-detected set)" ($dark.ringLastDetectedRegion -ne "" -and $dark.ringLastDetectedRegion -ne $null) "got '$($dark.ringLastDetectedRegion)'"

Write-Host "`n=== Scenario 2a: Saruman CorruptPath (same MAIA order type) ===" -ForegroundColor Cyan
Reset
Order "dark" "ASSIGN_ROUTE" "saruman" (Turn) @{ pathIds = @("fords-of-isen-to-isengard") } | Out-Null
Tick | Out-Null
Order "dark" "MAIA_ABILITY" "saruman" (Turn) @{ targetPathId = "fords-of-isen-to-edoras" } | Out-Null
Tick | Out-Null
$p = (State "dark").paths.'fords-of-isen-to-edoras'
Check "Saruman corrupted path: surveillance=3" ($p.surveillanceLevel -eq 3) "got $($p.surveillanceLevel)"
Check "Saruman corrupted path: corrupted=true" ($p.corrupted -eq $true) "got $($p.corrupted)"

Write-Host "`n=== Scenario 2b: Gandalf OpenPath (same MAIA order type) ===" -ForegroundColor Cyan
Reset
Order "dark" "DEPLOY_NAZGUL" "nazgul-2" (Turn) @{ targetRegion = "moria" } | Out-Null
Tick | Out-Null
Order "dark" "BLOCK_PATH" "nazgul-2" (Turn) @{ pathId = "moria-to-lothlorien" } | Out-Null
Tick | Out-Null
$pb = (State "light").paths.'moria-to-lothlorien'
Check "Path BLOCKED by Nazgul (no guard present)" ($pb.status -eq "BLOCKED") "got $($pb.status)"
Order "light" "ASSIGN_ROUTE" "gandalf" (Turn) @{ pathIds = @("rivendell-to-moria") } | Out-Null
Tick | Out-Null
Order "light" "MAIA_ABILITY" "gandalf" (Turn) @{ targetPathId = "moria-to-lothlorien" } | Out-Null
Tick | Out-Null
$po = (State "light").paths.'moria-to-lothlorien'
Check "Gandalf opened path: TEMPORARILY_OPEN" ($po.status -eq "TEMPORARILY_OPEN") "got $($po.status)"

Write-Host "`n=== Scenario 2c: FellowshipGuard denies a Nazgul block ===" -ForegroundColor Cyan
Reset
Order "light" "ASSIGN_ROUTE" "legolas" (Turn) @{ pathIds = @("rivendell-to-lothlorien") } | Out-Null
Order "dark"  "DEPLOY_NAZGUL" "nazgul-3" (Turn) @{ targetRegion = "emyn-muil" } | Out-Null
Tick | Out-Null
Order "dark" "BLOCK_PATH" "nazgul-3" (Turn) @{ pathId = "lothlorien-to-emyn-muil" } | Out-Null
Tick | Out-Null
$pg = (State "light").paths.'lothlorien-to-emyn-muil'
Check "Block FAILS while guard present (stays OPEN)" ($pg.status -eq "OPEN") "got $($pg.status)"

Write-Host "`n=== Win drive: Ring destroyed at Mount Doom (exactly-once GameOver) ===" -ForegroundColor Cyan
Reset
$route = @("shire-to-bree", "bree-to-rivendell", "rivendell-to-lothlorien", "lothlorien-to-emyn-muil", "emyn-muil-to-dead-marshes", "dead-marshes-to-mordor", "mordor-to-mount-doom")
Order "light" "ASSIGN_ROUTE" "ring-bearer" (Turn) @{ pathIds = $route } | Out-Null
1..7 | ForEach-Object { Tick | Out-Null }
$atDoom = (State "light").ringBearerRegion
Check "Ring Bearer reached mount-doom" ($atDoom -eq "mount-doom") "got '$atDoom'"
Order "light" "DESTROY_RING" "ring-bearer" (Turn) @{} | Out-Null
$final = Tick
Check "Game over" ($final.over -eq $true) "over=$($final.over)"
Check "Light Side wins" ($final.winner -eq "FREE_PEOPLES") "winner=$($final.winner)"

Write-Host "`n================  $pass passed, $fail failed  ================" -ForegroundColor Cyan
if ($fail -gt 0) { exit 1 } else { exit 0 }
