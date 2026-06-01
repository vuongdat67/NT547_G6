param(
    [switch]$SkipPublish
)

$ErrorActionPreference = "Stop"
$env:GOCACHE = Join-Path (Get-Location) ".gocache"

Write-Host "== gofmt check =="
$files = gofmt -l .
if ($files) {
    Write-Host $files
    throw "gofmt reported unformatted files"
}

Write-Host "== go vet =="
go vet ./...

Write-Host "== go test =="
go test ./...

Write-Host "== eval report =="
go run ./cmd/eval_report

Write-Host "== experiment runner =="
go run ./cmd/experiment_runner

if (-not $SkipPublish) {
    Write-Host "== publish results =="
    go run ./cmd/publish_results
}

Write-Host "== artifact verification =="
go run ./cmd/verify_artifacts

Write-Host "All local checks passed."
