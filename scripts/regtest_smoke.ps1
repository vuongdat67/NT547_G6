param(
    [string]$BitcoinCli = "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe",
    [string]$WalletName = "test",
    [decimal]$AmountBtc = 0.01,
    [decimal]$FeeRateSatVb = 1
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BitcoinCli)) {
    throw "bitcoin-cli not found at path: $BitcoinCli"
}

function Invoke-BtcCli {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
    $output = & $BitcoinCli -regtest "-rpcwallet=$WalletName" @Args 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "bitcoin-cli failed: $($output -join "`n")"
    }
    return $output
}

Write-Host "[1/6] Checking regtest block height..."
$startHeight = [int](Invoke-BtcCli getblockcount)

Write-Host "[2/6] Creating a new wallet address..."
$destAddr = (Invoke-BtcCli getnewaddress).Trim()

Write-Host "[3/6] Sending $AmountBtc BTC to wallet address..."
$amountText = $AmountBtc.ToString([System.Globalization.CultureInfo]::InvariantCulture)
$feeRateText = $FeeRateSatVb.ToString([System.Globalization.CultureInfo]::InvariantCulture)
$txid = (Invoke-BtcCli -named sendtoaddress "address=$destAddr" "amount=$amountText" "fee_rate=$feeRateText").Trim()

Write-Host "[4/6] Mining 1 block to confirm tx..."
$mineAddr = (Invoke-BtcCli getnewaddress).Trim()
$blockHashes = Invoke-BtcCli generatetoaddress 1 $mineAddr | ConvertFrom-Json

Write-Host "[5/6] Reading transaction details..."
$txInfo = Invoke-BtcCli gettransaction $txid | ConvertFrom-Json

Write-Host "[6/6] Building summary..."
$endHeight = [int](Invoke-BtcCli getblockcount)
$summary = [ordered]@{
    network = "regtest"
    wallet = $WalletName
    startHeight = $startHeight
    endHeight = $endHeight
    destinationAddress = $destAddr
    minedBlockHash = $blockHashes[0]
    txid = $txid
    amountBtc = $AmountBtc
    feeRateSatVb = $FeeRateSatVb
    confirmations = $txInfo.confirmations
    blockhash = $txInfo.blockhash
    trusted = $txInfo.trusted
    generatedAtUtc = (Get-Date).ToUniversalTime().ToString("o")
}

$artifactDir = Join-Path $PSScriptRoot "..\artifacts"
New-Item -ItemType Directory -Path $artifactDir -Force | Out-Null
$artifactPath = Join-Path $artifactDir "regtest_txids.json"
$summary | ConvertTo-Json -Depth 6 | Set-Content -Encoding UTF8 $artifactPath

Write-Host "Regtest smoke test success."
Write-Host "txid: $txid"
Write-Host "artifact: $artifactPath"
