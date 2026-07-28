$ErrorActionPreference = 'Stop'

function Invoke-CheckedDockerQuery {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    $previousErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $queryOutput = @(& docker @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($exitCode -ne 0) {
        throw "Docker cleanup proof query failed: $($Arguments[0])"
    }
    return $queryOutput
}

$script = Join-Path $PSScriptRoot '..\rehearse_cross_server_migration.ps1'
$scriptSource = Get-Content -Raw -LiteralPath $script
$expectedVolumes = @(
    'yucore-migration-rehearsal-mysql-source-data',
    'yucore-migration-rehearsal-mysql-target-data',
    'yucore-migration-rehearsal-redis-source-data',
    'yucore-migration-rehearsal-redis-target-data'
)
foreach ($volume in $expectedVolumes) {
    if (-not $scriptSource.Contains($volume)) {
        throw "missing fixed rehearsal volume: $volume"
    }
}
if (-not $scriptSource.Contains("'volume', 'rm'")) {
    throw 'rehearsal has no targeted volume cleanup'
}
if (-not $scriptSource.Contains("'volume', 'ls'")) {
    throw 'cleanup proof does not inspect rehearsal volumes'
}
foreach ($unicodeCodePoint in @('0x96EA', '0x1F600')) {
    if (-not $scriptSource.Contains($unicodeCodePoint)) {
        throw "missing deterministic Unicode fixture: $unicodeCodePoint"
    }
}
foreach ($hashContract in @(
    'information_schema.columns',
    'HEX(JSON_ARRAY',
    'SHA2('
)) {
    if (-not $scriptSource.Contains($hashContract)) {
        throw "missing MySQL hash contract: $hashContract"
    }
}
if (
    $scriptSource.Contains('[IO.File]::WriteAllLines') -or
    $scriptSource.Contains('$dumpText |')
) {
    throw 'MySQL dump bytes cross the PowerShell text pipeline'
}
foreach ($redirection in @('> "$1"', '< "$1"')) {
    if (-not $scriptSource.Contains($redirection)) {
        throw "missing in-container MySQL redirection: $redirection"
    }
}
foreach ($uncheckedQuery in @(
    '@(& docker ps -aq',
    '@(& docker network ls',
    '@(& docker volume ls'
)) {
    if ($scriptSource.Contains($uncheckedQuery)) {
        throw "unchecked Docker cleanup proof query: $uncheckedQuery"
    }
}
$output = & $script -Json
if ($LASTEXITCODE -ne 0) { throw "rehearsal failed with exit $LASTEXITCODE" }
$result = $output | ConvertFrom-Json
if (-not $result.mysql_forward_equal) { throw 'MySQL forward manifest mismatch' }
if (-not $result.mysql_rollback_equal) { throw 'MySQL rollback manifest mismatch' }
if (-not $result.redis_forward_equal) { throw 'Redis forward key mismatch' }
if (-not $result.maintenance_status_503) { throw 'Maintenance response was not 503' }
if (-not $result.maintenance_retry_after) { throw 'Maintenance response has no Retry-After' }
if (-not $result.forward_marker_new) { throw 'Forward proxy did not reach new marker' }
if (-not $result.rollback_marker_old) { throw 'Rollback proxy did not reach old marker' }
if (-not $result.cleanup_complete) { throw 'Disposable resources remain' }
$remainingContainers = @(
    Invoke-CheckedDockerQuery -Arguments @(
        'ps', '-aq', '--filter', 'name=yucore-migration-rehearsal'
    )
)
$remainingNetworks = @(
    Invoke-CheckedDockerQuery -Arguments @(
        'network', 'ls', '-q', '--filter', 'name=yucore-migration-rehearsal'
    )
)
$remainingVolumes = @(
    Invoke-CheckedDockerQuery -Arguments @(
        'volume', 'ls', '-q', '--filter', 'name=yucore-migration-rehearsal'
    )
)
if ($remainingContainers.Count -ne 0) { throw 'Disposable containers remain' }
if ($remainingNetworks.Count -ne 0) { throw 'Disposable networks remain' }
if ($remainingVolumes.Count -ne 0) { throw 'Disposable volumes remain' }
