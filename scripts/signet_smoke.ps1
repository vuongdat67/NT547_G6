param(
    [string]$BitcoinCli = "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe",
    [string]$WalletName = "test",
    [decimal]$AmountBtc = 0.0002,
    [decimal]$FeeRateSatVb = 1,
    [int]$TargetConfirmations = 1,
    [int]$PollSeconds = 30,
    [int]$MaxWaitMinutes = 60,
    [switch]$TryLoadWallet
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BitcoinCli)) {
    throw "bitcoin-cli not found at path: $BitcoinCli"
}

function Invoke-BtcCli {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    $output = & $BitcoinCli -signet @Args 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "bitcoin-cli failed: $($output -join "`n")"
    }
    return $output
}

function Invoke-BtcCliWallet {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    $output = & $BitcoinCli -signet "-rpcwallet=$WalletName" @Args 2>&1
    if ($LASTEXITCODE -ne 0) {
        $msg = $output -join "`n"
        if ($msg -match "error code: -18") {
            throw "Wallet '$WalletName' is not loaded on signet. Use -WalletName with a loaded wallet (check: bitcoin-cli -signet listwallets), or run with -TryLoadWallet. Raw error:`n$msg"
        }
        throw "bitcoin-cli wallet call failed: $msg"
    }
    return $output
}

Write-Host "[1/7] Checking signet node connectivity..."
$chainInfo = Invoke-BtcCli getblockchaininfo | ConvertFrom-Json
if ($chainInfo.chain -ne "signet") {
    throw "Connected chain is '$($chainInfo.chain)', expected 'signet'."
}
$startHeight = [int]$chainInfo.blocks

if ($TryLoadWallet) {
    Write-Host "[2/7] Trying to load wallet '$WalletName' if needed..."
    try {
        Invoke-BtcCli loadwallet $WalletName | Out-Null
    } catch {
        # Ignore if already loaded or load attempt is not needed.
    }
}

Write-Host "[3/7] Reading wallet balance..."
$balance = [decimal](Invoke-BtcCliWallet getbalance)
if ($balance -lt $AmountBtc) {
    throw "Insufficient balance in wallet '$WalletName'. Balance=$balance BTC, requested send=$AmountBtc BTC."
}

Write-Host "[4/7] Creating destination address..."
$destAddr = (Invoke-BtcCliWallet getnewaddress).Trim()

Write-Host "[5/7] Sending $AmountBtc BTC on signet..."
$amountText = $AmountBtc.ToString([System.Globalization.CultureInfo]::InvariantCulture)
$feeRateText = $FeeRateSatVb.ToString([System.Globalization.CultureInfo]::InvariantCulture)
$txid = (Invoke-BtcCliWallet -named sendtoaddress "address=$destAddr" "amount=$amountText" "fee_rate=$feeRateText").Trim()

Write-Host "[6/7] Fetching transaction details..."
$txInfo = Invoke-BtcCliWallet gettransaction $txid | ConvertFrom-Json

$deadline = (Get-Date).AddMinutes($MaxWaitMinutes)
$confirmations = [int]$txInfo.confirmations
if ($TargetConfirmations -gt 0 -and $confirmations -lt $TargetConfirmations) {
    Write-Host "Waiting for confirmations: target=$TargetConfirmations, current=$confirmations"
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds $PollSeconds
        try {
            $txInfo = Invoke-BtcCliWallet gettransaction $txid | ConvertFrom-Json
            $confirmations = [int]$txInfo.confirmations
            Write-Host "  confirmations=$confirmations"
            if ($confirmations -ge $TargetConfirmations) { break }
        } catch {
            Write-Host "  still waiting for wallet tx visibility..."
        }
    }
}

Write-Host "[7/7] Building artifact..."
$endHeight = [int](Invoke-BtcCli getblockcount)
$summary = [ordered]@{
    network = "signet"
    wallet = $WalletName
    startHeight = $startHeight
    endHeight = $endHeight
    destinationAddress = $destAddr
    txid = $txid
    amountBtc = $AmountBtc
    feeRateSatVb = $FeeRateSatVb
    targetConfirmations = $TargetConfirmations
    confirmations = $confirmations
    blockhash = $txInfo.blockhash
    trusted = $txInfo.trusted
    generatedAtUtc = (Get-Date).ToUniversalTime().ToString("o")
}

$artifactDir = Join-Path $PSScriptRoot "..\artifacts"
New-Item -ItemType Directory -Path $artifactDir -Force | Out-Null
$artifactPath = Join-Path $artifactDir "signet_txids.json"
$summary | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $artifactPath

Write-Host "Signet smoke test submitted successfully."
Write-Host "txid: $txid"
Write-Host "confirmations: $confirmations"
Write-Host "artifact: $artifactPath"
