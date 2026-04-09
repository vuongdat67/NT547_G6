param(
    [string]$BitcoinCli = "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe",
    [string]$WalletName = "hehtlc_research",
    [int64]$FundSat = 200000,
    [int64[]]$FeeSatProfiles = @(250, 500, 1000, 2500, 5000),
    [int[]]$Seeds = @(1, 2, 3),
    [string]$OutputDir = "artifacts/onchain/signet/fee_profiles"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $BitcoinCli)) {
    throw "bitcoin-cli not found at path: $BitcoinCli"
}

$repoRoot = Split-Path $PSScriptRoot -Parent
Set-Location $repoRoot

New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null

$records = @()

foreach ($fee in $FeeSatProfiles) {
    foreach ($seed in $Seeds) {
        $artifact = Join-Path $OutputDir ("fee_{0}_seed_{1}.json" -f $fee, $seed)
        Write-Host ("Running fee={0} seed={1}" -f $fee, $seed)

        $output = go run ./scripts/deploy_linked_acs.go `
            -network signet `
            -bitcoin-cli $BitcoinCli `
            -wallet $WalletName `
            -artifact $artifact `
            -fund-sat $FundSat `
            -fee-sat $fee `
            -try-load-wallet 2>&1

        $ok = $LASTEXITCODE -eq 0
        if (-not $ok) {
            Write-Host ("FAILED fee={0} seed={1}" -f $fee, $seed)
            Write-Host ($output | Out-String)
        }

        $fundTxid = ""
        $spendTxid = ""
        if ($ok -and (Test-Path $artifact)) {
            try {
                $j = Get-Content $artifact | ConvertFrom-Json
                $fundTxid = $j.fundTxid
                $spendTxid = $j.spendTxid
            } catch {
                # Keep txid fields empty if artifact parse fails.
            }
        }

        $records += [pscustomobject]@{
            feeSat = $fee
            seed = $seed
            success = $ok
            artifact = $artifact
            fundTxid = $fundTxid
            spendTxid = $spendTxid
        }
    }
}

$summaryCsvPath = Join-Path $OutputDir "fee_profile_summary.csv"
$summaryJsonPath = Join-Path $OutputDir "fee_profile_summary.json"
$txidsCsvPath = Join-Path $OutputDir "fee_profile_txids.csv"
$txidsJsonPath = Join-Path $OutputDir "fee_profile_txids.json"

$ordered = $records | Sort-Object feeSat, seed
$ordered | Select-Object feeSat, seed, success, artifact | Export-Csv -NoTypeInformation -Path $summaryCsvPath
$ordered | Select-Object feeSat, seed, success, artifact | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $summaryJsonPath
$ordered | Select-Object feeSat, seed, fundTxid, spendTxid, artifact | Export-Csv -NoTypeInformation -Path $txidsCsvPath
$ordered | Select-Object feeSat, seed, fundTxid, spendTxid, artifact | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $txidsJsonPath

Write-Host ""
Write-Host ("Summary CSV : {0}" -f $summaryCsvPath)
Write-Host ("Summary JSON: {0}" -f $summaryJsonPath)
Write-Host ("TxIDs CSV   : {0}" -f $txidsCsvPath)
Write-Host ("TxIDs JSON  : {0}" -f $txidsJsonPath)

$records | Group-Object feeSat | ForEach-Object {
    $total = $_.Count
    $ok = ($_.Group | Where-Object success).Count
    Write-Host ("fee={0}: {1}/{2} success" -f $_.Name, $ok, $total)
}
