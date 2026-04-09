param(
    [string]$BitcoinCli = "C:\Program Files\Bitcoin\daemon\bitcoin-cli.exe",
    [string]$WalletName = "test",
    [int64]$FundSat = 3000000,
    [int64[]]$FeeSatProfiles = @(250, 500, 1000, 2500, 5000),
    [int[]]$Seeds = @(1, 2, 3),
    [string]$OutputDir = "artifacts/onchain/regtest/fee_profiles"
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
            -network regtest `
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

$csvPath = Join-Path $OutputDir "fee_profile_summary.csv"
$jsonPath = Join-Path $OutputDir "fee_profile_summary.json"
$records | Sort-Object feeSat, seed | Export-Csv -NoTypeInformation -Path $csvPath
$records | Sort-Object feeSat, seed | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 $jsonPath

Write-Host ""
Write-Host ("Summary CSV : {0}" -f $csvPath)
Write-Host ("Summary JSON: {0}" -f $jsonPath)

$records | Group-Object feeSat | ForEach-Object {
    $total = $_.Count
    $ok = ($_.Group | Where-Object success).Count
    Write-Host ("fee={0}: {1}/{2} success" -f $_.Name, $ok, $total)
}
