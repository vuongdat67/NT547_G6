param(
    [ValidateSet("regtest", "signet")]
    [string]$Network,

    [string]$BitcoinDataDir = "$env:LOCALAPPDATA\Bitcoin",
    [string]$RpcUser = "vuong",
    [string]$RpcPassword = "hope"
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Network)) {
    throw "-Network is required: regtest or signet"
}

if ([string]::IsNullOrWhiteSpace($RpcUser) -or [string]::IsNullOrWhiteSpace($RpcPassword)) {
    throw "-RpcUser and -RpcPassword must be non-empty"
}

if (-not (Test-Path $BitcoinDataDir)) {
    New-Item -ItemType Directory -Path $BitcoinDataDir -Force | Out-Null
}

$configPath = Join-Path $BitcoinDataDir "bitcoin.conf"

switch ($Network) {
    "regtest" {
        $content = @"
regtest=1
server=1
rpcuser=$RpcUser
rpcpassword=$RpcPassword
[regtest]
rpcport=18443
fallbackfee=0.0001
"@
    }
    "signet" {
        $content = @"
# Global settings
server=1

# Signet specific settings
[signet]
rpcuser=$RpcUser
rpcpassword=$RpcPassword
rpcport=38332
rpcallowip=127.0.0.1
fallbackfee=0.0001
prune=1000
"@
    }
}

Set-Content -Path $configPath -Value $content -Encoding ASCII
Write-Host "Wrote $configPath for network '$Network'."
