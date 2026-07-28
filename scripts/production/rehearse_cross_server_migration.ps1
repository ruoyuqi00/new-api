[CmdletBinding()]
param([switch]$Json)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$Prefix = 'yucore-migration-rehearsal'
$NetworkName = "$Prefix-network"
$MySqlSource = "$Prefix-mysql-source"
$MySqlTarget = "$Prefix-mysql-target"
$RedisSource = "$Prefix-redis-source"
$RedisTarget = "$Prefix-redis-target"
$MySqlSourceVolume = 'yucore-migration-rehearsal-mysql-source-data'
$MySqlTargetVolume = 'yucore-migration-rehearsal-mysql-target-data'
$RedisSourceVolume = 'yucore-migration-rehearsal-redis-source-data'
$RedisTargetVolume = 'yucore-migration-rehearsal-redis-target-data'
$MarkerOld = "$Prefix-marker-old"
$MarkerNew = "$Prefix-marker-new"
$Maintenance = "$Prefix-maintenance"
$Caddy = "$Prefix-caddy"

$script:CreatedContainers = New-Object 'System.Collections.Generic.List[string]'
$script:CreatedVolumes = New-Object 'System.Collections.Generic.List[string]'
$script:NetworkCreated = $false
$script:MySqlPassword = 'rehearsal-' + [Guid]::NewGuid().ToString('N')
$TempDirectory = Join-Path (
    [IO.Path]::GetTempPath()
) "$Prefix-temp-$([Guid]::NewGuid().ToString('N'))"
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

$Result = [ordered]@{
    mysql_forward_equal = $false
    mysql_rollback_equal = $false
    redis_forward_equal = $false
    maintenance_status_503 = $false
    maintenance_retry_after = $false
    forward_marker_new = $false
    rollback_marker_old = $false
    cleanup_complete = $false
}

function Invoke-Docker {
    param([Parameter(Mandatory = $true)][string[]]$Arguments)

    $previousErrorActionPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $commandOutput = @(& docker @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($exitCode -ne 0) {
        throw "Docker command failed: $($Arguments[0])"
    }
    return $commandOutput
}

function New-TrackedContainer {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    $null = Invoke-Docker -Arguments $Arguments
    $null = $script:CreatedContainers.Add($Name)
}

function Assert-RehearsalVolumeName {
    param([Parameter(Mandatory = $true)][string]$Name)

    $allowedVolumes = @(
        $MySqlSourceVolume,
        $MySqlTargetVolume,
        $RedisSourceVolume,
        $RedisTargetVolume
    )
    if (
        $allowedVolumes -cnotcontains $Name -or
        -not $Name.StartsWith("$Prefix-", [StringComparison]::Ordinal)
    ) {
        throw "Invalid rehearsal volume name: $Name"
    }
}

function Wait-Condition {
    param(
        [Parameter(Mandatory = $true)][scriptblock]$Condition,
        [Parameter(Mandatory = $true)][string]$Description,
        [int]$TimeoutSeconds = 90
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        try {
            if (& $Condition) {
                return
            }
        } catch {
        }
        Start-Sleep -Milliseconds 500
    } while ([DateTime]::UtcNow -lt $deadline)

    throw "Timed out waiting for $Description"
}

function Wait-MySql {
    param([Parameter(Mandatory = $true)][string]$Container)

    Wait-Condition -Description "$Container MySQL readiness" -Condition {
        $null = & docker exec `
            -e "MYSQL_PWD=$script:MySqlPassword" `
            $Container `
            mysqladmin ping -h 127.0.0.1 -uroot --silent 2>$null
        return $LASTEXITCODE -eq 0
    }
}

function Invoke-MySql {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$Sql
    )

    return Invoke-Docker -Arguments @(
        'exec',
        '-e', "MYSQL_PWD=$script:MySqlPassword",
        $Container,
        'mysql', '--default-character-set=utf8mb4', '-N', '-B', '-uroot', 'rehearsal', '-e', $Sql
    )
}

function Get-MySqlManifest {
    param([Parameter(Mandatory = $true)][string]$Container)

    $sql = @'
SET SESSION group_concat_max_len = 1048576;
SELECT
    'users',
    (SELECT COUNT(*) FROM users),
    (SELECT COALESCE(MAX(id), 0) FROM users),
    (SELECT SHA2(GROUP_CONCAT(HEX(JSON_ARRAY(
        ordinal_position, column_name, column_type, is_nullable,
        COALESCE(column_default, '<NULL>'), column_key,
        COALESCE(character_set_name, ''), COALESCE(collation_name, ''), extra
    )) ORDER BY ordinal_position SEPARATOR '|'), 256)
        FROM information_schema.columns
        WHERE table_schema = 'rehearsal' AND table_name = 'users'),
    (SELECT SHA2(COALESCE(GROUP_CONCAT(
        HEX(JSON_ARRAY(id, email)) ORDER BY id SEPARATOR '|'
    ), ''), 256) FROM users);
SELECT
    'tokens',
    (SELECT COUNT(*) FROM tokens),
    (SELECT COALESCE(MAX(id), 0) FROM tokens),
    (SELECT SHA2(GROUP_CONCAT(HEX(JSON_ARRAY(
        ordinal_position, column_name, column_type, is_nullable,
        COALESCE(column_default, '<NULL>'), column_key,
        COALESCE(character_set_name, ''), COALESCE(collation_name, ''), extra
    )) ORDER BY ordinal_position SEPARATOR '|'), 256)
        FROM information_schema.columns
        WHERE table_schema = 'rehearsal' AND table_name = 'tokens'),
    (SELECT SHA2(COALESCE(GROUP_CONCAT(
        HEX(JSON_ARRAY(id, user_id, token_name)) ORDER BY id SEPARATOR '|'
    ), ''), 256) FROM tokens);
SELECT
    'logs',
    (SELECT COUNT(*) FROM logs),
    (SELECT COALESCE(MAX(id), 0) FROM logs),
    (SELECT SHA2(GROUP_CONCAT(HEX(JSON_ARRAY(
        ordinal_position, column_name, column_type, is_nullable,
        COALESCE(column_default, '<NULL>'), column_key,
        COALESCE(character_set_name, ''), COALESCE(collation_name, ''), extra
    )) ORDER BY ordinal_position SEPARATOR '|'), 256)
        FROM information_schema.columns
        WHERE table_schema = 'rehearsal' AND table_name = 'logs'),
    (SELECT SHA2(COALESCE(GROUP_CONCAT(
        HEX(JSON_ARRAY(id, user_id, message)) ORDER BY id SEPARATOR '|'
    ), ''), 256) FROM logs);
'@
    return ((Invoke-MySql -Container $Container -Sql $sql) -join "`n").Trim()
}

function Export-MySqlDump {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$Path
    )

    $containerPath = "/tmp/rehearsal-$([IO.Path]::GetFileName($Path))"
    $null = Invoke-Docker -Arguments @(
        'exec', $Container,
        'sh', '-c',
        'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysqldump --default-character-set=utf8mb4 -uroot --single-transaction --skip-lock-tables --set-gtid-purged=OFF --databases rehearsal > "$1"',
        'rehearsal-dump', $containerPath
    )
    $null = Invoke-Docker -Arguments @(
        'cp', "${Container}:$containerPath", $Path
    )
    $null = Invoke-Docker -Arguments @(
        'exec', $Container, 'rm', '-f', $containerPath
    )
}

function Import-MySqlDump {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$Path
    )

    $containerPath = "/tmp/rehearsal-$([IO.Path]::GetFileName($Path))"
    $null = Invoke-Docker -Arguments @(
        'cp', $Path, "${Container}:$containerPath"
    )
    $null = Invoke-Docker -Arguments @(
        'exec', $Container,
        'sh', '-c',
        'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --default-character-set=utf8mb4 -uroot < "$1"',
        'rehearsal-restore', $containerPath
    )
    $null = Invoke-Docker -Arguments @(
        'exec', $Container, 'rm', '-f', $containerPath
    )
}

function Wait-Redis {
    param([Parameter(Mandatory = $true)][string]$Container)

    Wait-Condition -Description "$Container Redis readiness" -Condition {
        $response = @(& docker exec $Container redis-cli ping 2>$null)
        return $LASTEXITCODE -eq 0 -and ($response -join '').Trim() -eq 'PONG'
    }
}

function Get-RedisPatternCount {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$Pattern
    )

    $keys = @(
        Invoke-Docker -Arguments @(
            'exec', $Container, 'redis-cli', '--raw', '--scan', '--pattern', $Pattern
        ) | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_) }
    )
    return $keys.Count
}

function Get-RedisManifest {
    param([Parameter(Mandatory = $true)][string]$Container)

    $parts = New-Object 'System.Collections.Generic.List[string]'
    foreach ($pattern in @('string:*', 'hash:*', 'expiring:*', 'affinity:*', 'cooldown:*')) {
        $count = Get-RedisPatternCount -Container $Container -Pattern $pattern
        $null = $parts.Add("$pattern=$count")
    }
    $stringValue = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'GET', 'string:welcome')
    ) -join ''
    $hashName = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'HGET', 'hash:user:1', 'name')
    ) -join ''
    $hashRole = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'HGET', 'hash:user:1', 'role')
    ) -join ''
    $affinity = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'GET', 'affinity:user:1')
    ) -join ''
    $cooldown = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'GET', 'cooldown:provider:1')
    ) -join ''
    $null = $parts.Add("string=$stringValue")
    $null = $parts.Add("hash-name=$hashName")
    $null = $parts.Add("hash-role=$hashRole")
    $null = $parts.Add("affinity=$affinity")
    $null = $parts.Add("cooldown=$cooldown")
    return $parts -join "`n"
}

function Get-RedisTtl {
    param(
        [Parameter(Mandatory = $true)][string]$Container,
        [Parameter(Mandatory = $true)][string]$Key
    )

    $value = (
        Invoke-Docker -Arguments @('exec', $Container, 'redis-cli', '--raw', 'TTL', $Key)
    ) -join ''
    return [int]$value.Trim()
}

function New-NginxMarker {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][string]$ContentPath
    )

    New-TrackedContainer -Name $Name -Arguments @(
        'create', '--name', $Name, '--network', $NetworkName, 'nginx:1.27-alpine'
    )
    $null = Invoke-Docker -Arguments @(
        'cp', $ContentPath, "${Name}:/usr/share/nginx/html/index.html"
    )
    $null = Invoke-Docker -Arguments @('start', $Name)
}

function Set-CaddyUpstream {
    param(
        [Parameter(Mandatory = $true)][string]$ConfigPath
    )

    $null = Invoke-Docker -Arguments @(
        'cp', $ConfigPath, "${Caddy}:/etc/caddy/Caddyfile.next"
    )
    $null = Invoke-Docker -Arguments @(
        'exec', $Caddy, 'mv', '/etc/caddy/Caddyfile.next', '/etc/caddy/Caddyfile'
    )
    $null = Invoke-Docker -Arguments @(
        'exec', $Caddy, 'caddy', 'reload', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'
    )
}

function Get-HttpSnapshot {
    $request = [Net.HttpWebRequest]::Create('http://127.0.0.1:18080/')
    $request.AllowAutoRedirect = $false
    $request.Timeout = 2000
    try {
        $response = $request.GetResponse()
    } catch [Net.WebException] {
        $response = $_.Exception.Response
        if ($null -eq $response) {
            throw
        }
    }

    try {
        $reader = New-Object IO.StreamReader($response.GetResponseStream())
        try {
            $body = $reader.ReadToEnd()
        } finally {
            $reader.Dispose()
        }
        return [pscustomobject]@{
            Status = [int]$response.StatusCode
            RetryAfter = [string]$response.Headers['Retry-After']
            Body = $body
        }
    } finally {
        $response.Close()
    }
}

$ScriptFailure = $null
$CleanupFailure = $null

try {
    $null = Invoke-Docker -Arguments @('version', '--format', '{{.Server.Version}}')
    foreach ($image in @('mysql:8.4', 'redis:7-alpine', 'nginx:1.27-alpine', 'caddy:2-alpine')) {
        $null = Invoke-Docker -Arguments @('image', 'inspect', $image)
    }

    $existingContainers = @(
        Invoke-Docker -Arguments @('ps', '-aq', '--filter', "name=$Prefix")
    )
    $existingNetworks = @(
        Invoke-Docker -Arguments @(
            'network', 'ls', '-q', '--filter', "name=$Prefix"
        )
    )
    $existingVolumes = @(
        Invoke-Docker -Arguments @(
            'volume', 'ls', '-q', '--filter', "name=$Prefix"
        )
    )
    if (
        $existingContainers.Count -ne 0 -or
        $existingNetworks.Count -ne 0 -or
        $existingVolumes.Count -ne 0
    ) {
        throw 'Fixed-prefix rehearsal resources already exist'
    }
    $portOwner = @(
        Get-NetTCPConnection -LocalPort 18080 -State Listen -ErrorAction SilentlyContinue
    )
    if ($portOwner.Count -ne 0) {
        throw 'Local rehearsal port 18080 is already in use'
    }

    [IO.Directory]::CreateDirectory($TempDirectory) | Out-Null
    $null = Invoke-Docker -Arguments @('network', 'create', $NetworkName)
    $script:NetworkCreated = $true

    foreach ($volume in @(
        $MySqlSourceVolume,
        $MySqlTargetVolume,
        $RedisSourceVolume,
        $RedisTargetVolume
    )) {
        Assert-RehearsalVolumeName -Name $volume
        $null = Invoke-Docker -Arguments @(
            'volume', 'create',
            '--label', "yucore.rehearsal=$Prefix",
            $volume
        )
        $null = $script:CreatedVolumes.Add($volume)
    }

    foreach ($mysql in @(
        @{ Container = $MySqlSource; Volume = $MySqlSourceVolume },
        @{ Container = $MySqlTarget; Volume = $MySqlTargetVolume }
    )) {
        New-TrackedContainer -Name $mysql.Container -Arguments @(
            'run', '-d',
            '--name', $mysql.Container,
            '--network', $NetworkName,
            '--mount', "type=volume,source=$($mysql.Volume),target=/var/lib/mysql",
            '--health-cmd', 'mysqladmin ping -h 127.0.0.1 -uroot --silent',
            '--health-interval', '1s',
            '--health-timeout', '1s',
            '--health-retries', '60',
            '-e', "MYSQL_ROOT_PASSWORD=$script:MySqlPassword",
            '-e', 'MYSQL_DATABASE=rehearsal',
            'mysql:8.4'
        )
    }
    Wait-MySql -Container $MySqlSource
    Wait-MySql -Container $MySqlTarget

    $snow = [char]0x96EA
    $emoji = [char]::ConvertFromUtf32(0x1F600)
    $fixtureSql = @"
CREATE TABLE users (id BIGINT PRIMARY KEY, email VARCHAR(255) NOT NULL) CHARACTER SET utf8mb4;
CREATE TABLE tokens (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, token_name VARCHAR(64) NOT NULL) CHARACTER SET utf8mb4;
CREATE TABLE logs (id BIGINT PRIMARY KEY, user_id BIGINT NOT NULL, message VARCHAR(255) NOT NULL) CHARACTER SET utf8mb4;
INSERT INTO users VALUES (1, 'alpha@example.test'), (2, 'snow-$snow@example.test');
INSERT INTO tokens VALUES (10, 1, 'alpha-token'), (11, 2, 'emoji-$emoji-token');
INSERT INTO logs VALUES (100, 1, 'snow-$snow-log'), (101, 2, 'emoji-$emoji-log');
"@
    $null = Invoke-MySql -Container $MySqlSource -Sql $fixtureSql
    $sourceManifest = Get-MySqlManifest -Container $MySqlSource
    $forwardDump = Join-Path $TempDirectory 'forward.sql'
    Export-MySqlDump -Container $MySqlSource -Path $forwardDump
    Import-MySqlDump -Container $MySqlTarget -Path $forwardDump
    $targetManifest = Get-MySqlManifest -Container $MySqlTarget
    $Result.mysql_forward_equal = $sourceManifest -ceq $targetManifest

    $postCutoverSql = @"
INSERT INTO users VALUES (3, 'post-$snow-$emoji@example.test');
INSERT INTO logs VALUES (102, 3, 'post-cutover-$snow-$emoji');
"@
    $null = Invoke-MySql -Container $MySqlTarget -Sql $postCutoverSql
    $authoritativeManifest = Get-MySqlManifest -Container $MySqlTarget
    $rollbackDump = Join-Path $TempDirectory 'rollback.sql'
    Export-MySqlDump -Container $MySqlTarget -Path $rollbackDump
    Import-MySqlDump -Container $MySqlSource -Path $rollbackDump
    $rollbackManifest = Get-MySqlManifest -Container $MySqlSource
    $Result.mysql_rollback_equal = $authoritativeManifest -ceq $rollbackManifest

    foreach ($redis in @(
        @{ Container = $RedisSource; Volume = $RedisSourceVolume },
        @{ Container = $RedisTarget; Volume = $RedisTargetVolume }
    )) {
        New-TrackedContainer -Name $redis.Container -Arguments @(
            'run', '-d',
            '--name', $redis.Container,
            '--network', $NetworkName,
            '--mount', "type=volume,source=$($redis.Volume),target=/data",
            '--health-cmd', 'redis-cli ping',
            '--health-interval', '1s',
            '--health-timeout', '1s',
            '--health-retries', '60',
            'redis:7-alpine',
            'redis-server', '--save', '60', '1', '--appendonly', 'no'
        )
    }
    Wait-Redis -Container $RedisSource
    Wait-Redis -Container $RedisTarget

    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'SET', 'string:welcome', 'ready')
    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'HSET', 'hash:user:1', 'name', 'alpha', 'role', 'admin')
    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'SET', 'expiring:session', 'active', 'EX', '600')
    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'SET', 'affinity:user:1', 'provider-a')
    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'SET', 'cooldown:provider:1', 'blocked', 'EX', '300')
    $null = Invoke-Docker -Arguments @('exec', $RedisSource, 'redis-cli', 'SAVE')

    $redisSourceManifest = Get-RedisManifest -Container $RedisSource
    $sourceSessionTtl = Get-RedisTtl -Container $RedisSource -Key 'expiring:session'
    $sourceCooldownTtl = Get-RedisTtl -Container $RedisSource -Key 'cooldown:provider:1'
    $redisDump = Join-Path $TempDirectory 'dump.rdb'
    $null = Invoke-Docker -Arguments @('cp', "${RedisSource}:/data/dump.rdb", $redisDump)
    $null = Invoke-Docker -Arguments @('stop', $RedisTarget)
    $null = Invoke-Docker -Arguments @('cp', $redisDump, "${RedisTarget}:/data/dump.rdb")
    $null = Invoke-Docker -Arguments @('start', $RedisTarget)
    Wait-Redis -Container $RedisTarget

    $redisTargetManifest = Get-RedisManifest -Container $RedisTarget
    $targetSessionTtl = Get-RedisTtl -Container $RedisTarget -Key 'expiring:session'
    $targetCooldownTtl = Get-RedisTtl -Container $RedisTarget -Key 'cooldown:provider:1'
    $sessionTtlValid = (
        $sourceSessionTtl -gt 0 -and
        $sourceSessionTtl -le 600 -and
        $targetSessionTtl -gt 0 -and
        [Math]::Abs($sourceSessionTtl - $targetSessionTtl) -le 30
    )
    $cooldownTtlValid = (
        $sourceCooldownTtl -gt 0 -and
        $sourceCooldownTtl -le 300 -and
        $targetCooldownTtl -gt 0 -and
        [Math]::Abs($sourceCooldownTtl - $targetCooldownTtl) -le 30
    )
    $Result.redis_forward_equal = (
        $redisSourceManifest -ceq $redisTargetManifest -and
        $sessionTtlValid -and
        $cooldownTtlValid
    )

    $oldContent = Join-Path $TempDirectory 'old.html'
    $newContent = Join-Path $TempDirectory 'new.html'
    $maintenanceConfig = Join-Path $TempDirectory 'maintenance.conf'
    [IO.File]::WriteAllText($oldContent, "old-origin`n", $Utf8NoBom)
    [IO.File]::WriteAllText($newContent, "new-origin`n", $Utf8NoBom)
    [IO.File]::WriteAllText($maintenanceConfig, @'
server {
    listen 80;
    location / {
        add_header Retry-After 180 always;
        return 503 "maintenance\n";
    }
}
'@, $Utf8NoBom)

    New-NginxMarker -Name $MarkerOld -ContentPath $oldContent
    New-NginxMarker -Name $MarkerNew -ContentPath $newContent
    New-TrackedContainer -Name $Maintenance -Arguments @(
        'create', '--name', $Maintenance, '--network', $NetworkName, 'nginx:1.27-alpine'
    )
    $null = Invoke-Docker -Arguments @(
        'cp', $maintenanceConfig, "${Maintenance}:/etc/nginx/conf.d/default.conf"
    )
    $null = Invoke-Docker -Arguments @('start', $Maintenance)

    foreach ($marker in @(
        @{ Name = $MarkerOld; Body = 'old-origin' },
        @{ Name = $MarkerNew; Body = 'new-origin' }
    )) {
        Wait-Condition -Description "$($marker.Name) Nginx readiness" -Condition {
            $body = @(& docker exec $marker.Name wget -qO- http://127.0.0.1/ 2>$null)
            return $LASTEXITCODE -eq 0 -and ($body -join '').Trim() -eq $marker.Body
        }
    }
    Wait-Condition -Description "$Maintenance Nginx readiness" -Condition {
        $running = @(
            & docker inspect --format '{{.State.Running}}' $Maintenance 2>$null
        )
        return $LASTEXITCODE -eq 0 -and ($running -join '').Trim() -eq 'true'
    }

    $caddyOld = Join-Path $TempDirectory 'Caddyfile.old'
    $caddyMaintenance = Join-Path $TempDirectory 'Caddyfile.maintenance'
    $caddyNew = Join-Path $TempDirectory 'Caddyfile.new'
    [IO.File]::WriteAllText($caddyOld, ":80 {`n    reverse_proxy ${MarkerOld}:80`n}`n", $Utf8NoBom)
    [IO.File]::WriteAllText($caddyMaintenance, ":80 {`n    reverse_proxy ${Maintenance}:80`n}`n", $Utf8NoBom)
    [IO.File]::WriteAllText($caddyNew, ":80 {`n    reverse_proxy ${MarkerNew}:80`n}`n", $Utf8NoBom)

    New-TrackedContainer -Name $Caddy -Arguments @(
        'create',
        '--name', $Caddy,
        '--network', $NetworkName,
        '-p', '127.0.0.1:18080:80',
        'caddy:2-alpine',
        'caddy', 'run', '--config', '/etc/caddy/Caddyfile', '--adapter', 'caddyfile'
    )
    $null = Invoke-Docker -Arguments @(
        'cp', $caddyOld, "${Caddy}:/etc/caddy/Caddyfile"
    )
    $null = Invoke-Docker -Arguments @('start', $Caddy)
    Wait-Condition -Description 'Caddy old-origin readiness' -Condition {
        try {
            return (Get-HttpSnapshot).Body.Trim() -eq 'old-origin'
        } catch {
            return $false
        }
    }

    Set-CaddyUpstream -ConfigPath $caddyMaintenance
    Wait-Condition -Description 'Caddy maintenance state' -Condition {
        try {
            $snapshot = Get-HttpSnapshot
            return $snapshot.Status -eq 503 -and $snapshot.RetryAfter -eq '180'
        } catch {
            return $false
        }
    }
    $maintenanceSnapshot = Get-HttpSnapshot
    $Result.maintenance_status_503 = $maintenanceSnapshot.Status -eq 503
    $Result.maintenance_retry_after = $maintenanceSnapshot.RetryAfter -eq '180'

    Set-CaddyUpstream -ConfigPath $caddyNew
    Wait-Condition -Description 'Caddy new-origin state' -Condition {
        try {
            return (Get-HttpSnapshot).Body.Trim() -eq 'new-origin'
        } catch {
            return $false
        }
    }
    $Result.forward_marker_new = (Get-HttpSnapshot).Body.Trim() -eq 'new-origin'

    Set-CaddyUpstream -ConfigPath $caddyOld
    Wait-Condition -Description 'Caddy old-origin rollback state' -Condition {
        try {
            return (Get-HttpSnapshot).Body.Trim() -eq 'old-origin'
        } catch {
            return $false
        }
    }
    $Result.rollback_marker_old = (Get-HttpSnapshot).Body.Trim() -eq 'old-origin'
} catch {
    $ScriptFailure = $_
} finally {
    try {
        for (
            $containerIndex = $script:CreatedContainers.Count - 1;
            $containerIndex -ge 0;
            $containerIndex--
        ) {
            $container = $script:CreatedContainers[$containerIndex]
            $resolvedName = (
                Invoke-Docker -Arguments @('inspect', '--format', '{{.Name}}', $container)
            ) -join ''
            $resolvedName = $resolvedName.Trim().TrimStart('/')
            if (
                $resolvedName -cne $container -or
                -not $resolvedName.StartsWith("$Prefix-", [StringComparison]::Ordinal)
            ) {
                throw "Refusing to remove unverified container $resolvedName"
            }
            $null = Invoke-Docker -Arguments @('rm', '-f', $container)
        }

        for (
            $volumeIndex = $script:CreatedVolumes.Count - 1;
            $volumeIndex -ge 0;
            $volumeIndex--
        ) {
            $volume = $script:CreatedVolumes[$volumeIndex]
            Assert-RehearsalVolumeName -Name $volume
            $resolvedVolume = (
                Invoke-Docker -Arguments @(
                    'volume', 'inspect', '--format', '{{.Name}}', $volume
                )
            ) -join ''
            $resolvedVolume = $resolvedVolume.Trim()
            if (
                $resolvedVolume -cne $volume -or
                -not $resolvedVolume.StartsWith("$Prefix-", [StringComparison]::Ordinal)
            ) {
                throw "Refusing to remove unverified volume $resolvedVolume"
            }
            $null = Invoke-Docker -Arguments @('volume', 'rm', $volume)
        }

        if ($script:NetworkCreated) {
            $resolvedNetwork = (
                Invoke-Docker -Arguments @('network', 'inspect', '--format', '{{.Name}}', $NetworkName)
            ) -join ''
            $resolvedNetwork = $resolvedNetwork.Trim()
            if (
                $resolvedNetwork -cne $NetworkName -or
                -not $resolvedNetwork.StartsWith("$Prefix-", [StringComparison]::Ordinal)
            ) {
                throw "Refusing to remove unverified network $resolvedNetwork"
            }
            $null = Invoke-Docker -Arguments @('network', 'rm', $NetworkName)
        }

        if (Test-Path -LiteralPath $TempDirectory) {
            $resolvedTemp = [IO.Path]::GetFullPath($TempDirectory)
            $resolvedTempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
            $tempLeaf = Split-Path -Leaf $resolvedTemp
            if (
                -not $resolvedTemp.StartsWith($resolvedTempRoot, [StringComparison]::OrdinalIgnoreCase) -or
                -not $tempLeaf.StartsWith("$Prefix-temp-", [StringComparison]::Ordinal)
            ) {
                throw "Refusing to remove unverified temp path $resolvedTemp"
            }
            Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
        }

        $remainingContainers = @(
            Invoke-Docker -Arguments @(
                'ps', '-aq', '--filter', "name=$Prefix"
            )
        )
        $remainingNetworks = @(
            Invoke-Docker -Arguments @(
                'network', 'ls', '-q', '--filter', "name=$Prefix"
            )
        )
        $remainingVolumes = @(
            Invoke-Docker -Arguments @(
                'volume', 'ls', '-q', '--filter', "name=$Prefix"
            )
        )
        $Result.cleanup_complete = (
            $remainingContainers.Count -eq 0 -and
            $remainingNetworks.Count -eq 0 -and
            $remainingVolumes.Count -eq 0 -and
            -not (Test-Path -LiteralPath $TempDirectory)
        )
    } catch {
        $CleanupFailure = $_
        $Result.cleanup_complete = $false
    }
}

if ($null -ne $CleanupFailure) {
    throw $CleanupFailure
}
if ($null -ne $ScriptFailure) {
    throw $ScriptFailure
}

if ($Json) {
    $Result | ConvertTo-Json -Compress
} else {
    [pscustomobject]$Result
}
