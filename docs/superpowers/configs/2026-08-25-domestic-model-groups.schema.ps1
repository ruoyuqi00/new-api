$ErrorActionPreference = 'Stop'
$manifestPath = Join-Path $PSScriptRoot '2026-08-25-domestic-model-groups.json'
if (-not (Test-Path -LiteralPath $manifestPath)) {
  throw "manifest not found: $manifestPath"
}

$raw = Get-Content -LiteralPath $manifestPath -Raw
$doc = $raw | ConvertFrom-Json

function Assert-Equal($actual, $expected, $name) {
  if ($actual -ne $expected) {
    throw "$name expected '$expected' but got '$actual'"
  }
}

function Assert-True($condition, $name) {
  if (-not $condition) {
    throw "assertion failed: $name"
  }
}

Assert-Equal $doc.base_url 'https://api.herohao.top/v1' 'base_url'
Assert-Equal $doc.groups.'国模按量'.ratio 0.3 'usage group ratio'
Assert-Equal $doc.groups.'国模按次'.ratio 0.3 'call group ratio'

$usageModels = @($doc.groups.'国模按量'.models)
$callModels = @($doc.groups.'国模按次'.models)
Assert-Equal $usageModels.Count 6 'usage model count'
Assert-Equal $callModels.Count 6 'call model count'

Assert-True (@($usageModels | Where-Object { $_.public_name -match '-call$' }).Count -eq 0) 'usage names do not use call aliases'
Assert-True (@($callModels | Where-Object { $_.public_name -notmatch '-call$' }).Count -eq 0) 'call names use call aliases'
Assert-True (@($callModels | Where-Object { $_.upstream_name -match '-call$' }).Count -eq 0) 'upstream names do not use call aliases'

$allNames = @($usageModels.public_name) + @($callModels.public_name)
Assert-Equal (($allNames | Sort-Object -Unique).Count) 12 'public name uniqueness'
Assert-True ($raw -notmatch 'sk-[A-Za-z0-9]') 'manifest contains no API key'
Assert-True ($raw -notmatch '(?i)authorization') 'manifest contains no authorization field'

Write-Output "domestic model manifest valid: usage=$($usageModels.Count), call=$($callModels.Count), pending=$(@($usageModels + $callModels | Where-Object verification_state -eq 'pending').Count)"
