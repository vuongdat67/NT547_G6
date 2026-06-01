param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path,
    [string[]]$PaperDirs = @("..\tex", "..\tex\13"),
    [ValidateSet("auto", "native", "browser", "texlive", "none")]
    [string]$SvgConverter = "auto",
    [switch]$EnablePdfCrop,
    [switch]$SkipPdfCrop,
    [switch]$SkipEvalReport,
    [switch]$SkipExperimentRunner,
    [switch]$SkipPublishResults
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-SvgConverter {
    param([string]$Mode)

    if ($Mode -eq "none") {
        return $null
    }

    $candidates = @(
        @{ Name = "inkscape"; Kind = "inkscape" },
        @{ Name = "magick"; Kind = "magick" },
        @{ Name = "rsvg-convert"; Kind = "rsvg" }
    )

    $browserPaths = @(
        @{ Kind = "chrome"; Path = "C:\Program Files\Google\Chrome\Application\chrome.exe" },
        @{ Kind = "chrome"; Path = "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe" },
        @{ Kind = "msedge"; Path = "C:\Program Files\Microsoft\Edge\Application\msedge.exe" },
        @{ Kind = "msedge"; Path = "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" }
    )

    $findNative = {
        foreach ($candidate in $candidates) {
            $cmd = Get-Command $candidate.Name -ErrorAction SilentlyContinue
            if ($null -ne $cmd) {
                return @{ Kind = $candidate.Kind; Path = $cmd.Source }
            }
        }
        return $null
    }

    $findBrowser = {
        foreach ($browser in $browserPaths) {
            if (Test-Path $browser.Path) {
                return $browser
            }
        }
        return $null
    }

    $findTexLive = {
        $lua = Get-Command lualatex -ErrorAction SilentlyContinue
        if ($null -eq $lua) {
            return $null
        }
        return @{ Kind = "texlive"; Path = $lua.Source }
    }

    switch ($Mode) {
        "native" {
            return (& $findNative)
        }
        "browser" {
            return (& $findBrowser)
        }
        "texlive" {
            return (& $findTexLive)
        }
        "auto" {
            $native = (& $findNative)
            if ($null -ne $native) {
                return $native
            }
            $browser = (& $findBrowser)
            if ($null -ne $browser) {
                return $browser
            }
            return (& $findTexLive)
        }
    }

    return $null
}

function Convert-ToFileUri {
    param([string]$Path)

    $abs = (Resolve-Path $Path).Path -replace '\\', '/'
    return "file:///$abs"
}

function Convert-LengthToPoints {
    param(
        [string]$Length,
        [double]$FallbackPt
    )

    if ([string]::IsNullOrWhiteSpace($Length)) {
        return $FallbackPt
    }

    if ($Length -notmatch '^\s*([0-9]*\.?[0-9]+)\s*(px|pt|in|cm|mm)?\s*$') {
        return $FallbackPt
    }

    $value = [double]$matches[1]
    $unit = $matches[2]
    if ([string]::IsNullOrWhiteSpace($unit) -or $unit -eq "px") {
        return $value * 0.75
    }

    switch ($unit) {
        "pt" { return $value }
        "in" { return $value * 72.0 }
        "cm" { return $value * 28.3464567 }
        "mm" { return $value * 2.83464567 }
        default { return $FallbackPt }
    }
}

function Get-SvgCanvasSizePt {
    param([string]$SvgPath)

    $head = (Get-Content -Path $SvgPath -TotalCount 5) -join "`n"
    $widthAttr = $null
    $heightAttr = $null
    $viewWidth = 720.0
    $viewHeight = 420.0

    if ($head -match 'width="([^"]+)"') {
        $widthAttr = $matches[1]
    }
    if ($head -match 'height="([^"]+)"') {
        $heightAttr = $matches[1]
    }
    if ($head -match 'viewBox="\s*[-0-9.]+\s+[-0-9.]+\s+([0-9.]+)\s+([0-9.]+)\s*"') {
        $viewWidth = [double]$matches[1]
        $viewHeight = [double]$matches[2]
    }

    $widthPt = Convert-LengthToPoints -Length $widthAttr -FallbackPt ($viewWidth * 0.75)
    $heightPt = Convert-LengthToPoints -Length $heightAttr -FallbackPt ($viewHeight * 0.75)

    return [pscustomobject]@{
        WidthPt = [Math]::Round($widthPt, 2)
        HeightPt = [Math]::Round($heightPt, 2)
    }
}

function Convert-SvgToContentSizedPdfViaBrowser {
    param(
        [string]$BrowserPath,
        [string]$SvgPath,
        [string]$PdfPath
    )

    $size = Get-SvgCanvasSizePt -SvgPath $SvgPath
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("svg2pdf-browser-" + [guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    try {
        $htmlPath = Join-Path $tmpDir "print.html"
        $svgUri = Convert-ToFileUri -Path $SvgPath
        $w = [string]::Format([System.Globalization.CultureInfo]::InvariantCulture, "{0:0.##}", $size.WidthPt)
        $h = [string]::Format([System.Globalization.CultureInfo]::InvariantCulture, "{0:0.##}", $size.HeightPt)
        $html = @"
<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <style>
    @page { size: ${w}pt ${h}pt; margin: 0; }
    html, body { margin: 0; padding: 0; width: ${w}pt; height: ${h}pt; overflow: hidden; }
    img { display: block; width: ${w}pt; height: ${h}pt; }
  </style>
</head>
<body>
  <img src="$svgUri" alt="figure" />
</body>
</html>
"@
        Set-Content -Path $htmlPath -Value $html -Encoding ASCII

        $htmlUri = Convert-ToFileUri -Path $htmlPath
        & $BrowserPath --headless --disable-gpu --allow-file-access-from-files --print-to-pdf-no-header "--print-to-pdf=$PdfPath" $htmlUri | Out-Null
        if (-not (Test-Path $PdfPath)) {
            throw "Browser conversion did not produce PDF: $PdfPath"
        }
    }
    finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Get-ToolPath {
    param([string]$Name)

    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $cmd) {
        return $null
    }
    return $cmd.Source
}

function Get-PdfPageSizeLine {
    param(
        [string]$PdfInfoPath,
        [string]$PdfPath
    )

    if ([string]::IsNullOrWhiteSpace($PdfInfoPath) -or -not (Test-Path $PdfPath)) {
        return $null
    }

    try {
        $line = & $PdfInfoPath $PdfPath 2>$null | Select-String -Pattern "Page size"
        if ($null -eq $line) {
            return $null
        }
        return ($line | Select-Object -First 1).Line.Trim()
    }
    catch {
        return $null
    }
}

function Crop-PdfToContent {
    param(
        [string]$PdfCropPath,
        [string]$PdfInfoPath,
        [string]$PdfPath
    )

    if ([string]::IsNullOrWhiteSpace($PdfCropPath) -or -not (Test-Path $PdfPath)) {
        return
    }

    $tmp = Join-Path (Split-Path $PdfPath -Parent) (([System.IO.Path]::GetFileNameWithoutExtension($PdfPath)) + ".crop.pdf")
    $before = Get-PdfPageSizeLine -PdfInfoPath $PdfInfoPath -PdfPath $PdfPath

    try {
        & $PdfCropPath "--margins" "2" $PdfPath $tmp | Out-Null
        if (Test-Path $tmp) {
            Move-Item -Path $tmp -Destination $PdfPath -Force
            $after = Get-PdfPageSizeLine -PdfInfoPath $PdfInfoPath -PdfPath $PdfPath
            if (-not [string]::IsNullOrWhiteSpace($after) -and $after -ne $before) {
                Write-Host ("Cropped {0}: {1} -> {2}" -f [System.IO.Path]::GetFileName($PdfPath), $before, $after)
            }
        }
    }
    catch {
        if (Test-Path $tmp) {
            Remove-Item -Path $tmp -Force -ErrorAction SilentlyContinue
        }
        Write-Warning ("pdfcrop failed for {0}: {1}" -f $PdfPath, $_.Exception.Message)
    }
}

function Convert-SvgToPdf {
    param(
        [hashtable]$Converter,
        [string]$SvgPath,
        [string]$PdfPath
    )

    switch ($Converter.Kind) {
        "inkscape" {
            & $Converter.Path $SvgPath "--export-type=pdf" "--export-filename=$PdfPath" | Out-Null
            break
        }
        "magick" {
            & $Converter.Path convert $SvgPath $PdfPath | Out-Null
            break
        }
        "rsvg" {
            & $Converter.Path "-f" "pdf" "-o" $PdfPath $SvgPath | Out-Null
            break
        }
        "chrome" {
            Convert-SvgToContentSizedPdfViaBrowser -BrowserPath $Converter.Path -SvgPath $SvgPath -PdfPath $PdfPath
            break
        }
        "msedge" {
            Convert-SvgToContentSizedPdfViaBrowser -BrowserPath $Converter.Path -SvgPath $SvgPath -PdfPath $PdfPath
            break
        }
        "texlive" {
            $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ("svg2pdf-" + [guid]::NewGuid().ToString("N"))
            New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
            try {
                $tmpSvg = Join-Path $tmpDir "input.svg"
                $tmpTex = Join-Path $tmpDir "convert.tex"
                Copy-Item -Path $SvgPath -Destination $tmpSvg -Force

                $texContent = @"
\\documentclass[border=0pt]{standalone}
\\usepackage{svg}
\\svgsetup{inkscapelatex=false}
\\begin{document}
\\includesvg{input}
\\end{document}
"@
                Set-Content -Path $tmpTex -Value $texContent -Encoding ASCII

                Push-Location $tmpDir
                try {
                    & $Converter.Path --shell-escape -interaction=nonstopmode -halt-on-error convert.tex | Out-Null
                }
                finally {
                    Pop-Location
                }

                $tmpPdf = Join-Path $tmpDir "convert.pdf"
                if (-not (Test-Path $tmpPdf)) {
                    $fallbackBrowsers = @(
                        "C:\Program Files\Google\Chrome\Application\chrome.exe",
                        "C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
                        "C:\Program Files\Microsoft\Edge\Application\msedge.exe",
                        "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"
                    )
                    $fallback = $fallbackBrowsers | Where-Object { Test-Path $_ } | Select-Object -First 1
                    if ($null -eq $fallback) {
                        throw "TeX Live conversion failed and no browser fallback found. Install inkscape or use -SvgConverter browser."
                    }
                    $uri = Convert-ToFileUri -Path $SvgPath
                    & $fallback --headless --disable-gpu --allow-file-access-from-files "--print-to-pdf=$PdfPath" $uri | Out-Null
                    if (-not (Test-Path $PdfPath)) {
                        throw "TeX Live conversion failed and browser fallback did not produce PDF."
                    }
                    break
                }
                Copy-Item -Path $tmpPdf -Destination $PdfPath -Force
            }
            finally {
                Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
            }
            break
        }
        default {
            throw "Unsupported converter kind: $($Converter.Kind)"
        }
    }
}

Push-Location $RepoRoot
try {
    if (-not $SkipEvalReport) {
        Write-Host "[1/4] Generating eval report artifacts..."
        & go run ./cmd/eval_report
    }

    if (-not $SkipExperimentRunner) {
        Write-Host "[2/4] Generating experiment artifacts..."
        & go run ./cmd/experiment_runner
    }

    if (-not $SkipPublishResults) {
        Write-Host "[3/4] Generating publication tables and figures..."
        & go run ./cmd/publish_results
    }

    $repoRootResolved = (Resolve-Path ".").Path
    $publicationDir = Join-Path $repoRootResolved "artifacts\publication"
    if (-not (Test-Path $publicationDir)) {
        throw "Missing publication directory: $publicationDir"
    }

    # Mirror linked deployment evidence into onchain/<network> so signet/regtest are always visible there.
    foreach ($network in @("regtest", "signet")) {
        $src = Join-Path $repoRootResolved ("artifacts\linked_acs_{0}.json" -f $network)
        if (Test-Path $src) {
            $dstDir = Join-Path $repoRootResolved ("artifacts\onchain\{0}" -f $network)
            New-Item -ItemType Directory -Path $dstDir -Force | Out-Null
            $dst = Join-Path $dstDir "latest_linked_acs.json"
            Copy-Item -Path $src -Destination $dst -Force
        }
    }

    $tableFiles = @(Get-ChildItem -Path $publicationDir -Filter "table_*.tex" -File)
    $figureFiles = @(Get-ChildItem -Path $publicationDir -Filter "fig_*.svg" -File)
    $converter = Get-SvgConverter -Mode $SvgConverter
    $tryPdfCrop = $EnablePdfCrop -and -not $SkipPdfCrop
    $pdfCropTool = if ($tryPdfCrop) { Get-ToolPath -Name "pdfcrop" } else { $null }
    $pdfInfoTool = Get-ToolPath -Name "pdfinfo"

    $figureAssets = @()
    foreach ($fig in $figureFiles) {
        $sourcePdf = Join-Path $publicationDir ($fig.BaseName + ".pdf")
        $preferBrowserSizedPdf = ($null -ne $converter) -and (($converter.Kind -eq "chrome") -or ($converter.Kind -eq "msedge"))

        if ($preferBrowserSizedPdf -or -not (Test-Path $sourcePdf)) {
            if ($null -ne $converter) {
                Convert-SvgToPdf -Converter $converter -SvgPath $fig.FullName -PdfPath $sourcePdf
            }
        }

        if ((Test-Path $sourcePdf) -and ($null -ne $pdfCropTool) -and (-not $preferBrowserSizedPdf)) {
            Crop-PdfToContent -PdfCropPath $pdfCropTool -PdfInfoPath $pdfInfoTool -PdfPath $sourcePdf
        }

        $figureAssets += [pscustomobject]@{
            Name = $fig.Name
            BaseName = $fig.BaseName
            SvgPath = $fig.FullName
            PdfPath = if (Test-Path $sourcePdf) { $sourcePdf } else { $null }
        }
    }

    Write-Host "[4/4] Syncing publication assets into paper folders..."
    foreach ($paperRel in $PaperDirs) {
        $paperDir = Join-Path $repoRootResolved $paperRel
        if (-not (Test-Path $paperDir)) {
            Write-Warning "Skip missing paper directory: $paperDir"
            continue
        }

        $paperDirResolved = (Resolve-Path $paperDir).Path
        $tableDir = Join-Path $paperDirResolved "tables"
        $imgDir = Join-Path $paperDirResolved "img"
        New-Item -ItemType Directory -Path $tableDir -Force | Out-Null
        New-Item -ItemType Directory -Path $imgDir -Force | Out-Null

        $obsoleteTables = @(
            "table_main_results.tex",
            "table_seed_stats.tex",
            "table_onchain_runs.tex"
        )
        foreach ($name in $obsoleteTables) {
            $legacy = Join-Path $tableDir $name
            if (Test-Path $legacy) {
                Remove-Item -Path $legacy -Force
            }
        }

        $obsoleteFigures = @(
            "fig_baseline_success.svg",
            "fig_baseline_success.pdf",
            "fig_onchain_success.svg",
            "fig_onchain_success.pdf"
        )
        foreach ($name in $obsoleteFigures) {
            $legacy = Join-Path $imgDir $name
            if (Test-Path $legacy) {
                Remove-Item -Path $legacy -Force
            }
        }

        foreach ($table in $tableFiles) {
            Copy-Item -Path $table.FullName -Destination (Join-Path $tableDir $table.Name) -Force
        }

        foreach ($fig in $figureAssets) {
            $targetSvg = Join-Path $imgDir $fig.Name
            $targetPdf = Join-Path $imgDir ($fig.BaseName + ".pdf")
            Copy-Item -Path $fig.SvgPath -Destination $targetSvg -Force

            if (-not [string]::IsNullOrWhiteSpace($fig.PdfPath)) {
                Copy-Item -Path $fig.PdfPath -Destination $targetPdf -Force
            }
            elseif (-not (Test-Path $targetPdf)) {
                Write-Warning "Missing source PDF and no converter output available for: $($fig.Name)"
            }
        }

        Write-Host ("Synced {0} tables and {1} figures to {2}" -f $tableFiles.Count, $figureAssets.Count, $paperDirResolved)
    }

    if ($null -eq $converter) {
        Write-Host "Completed sync. SVG copied to paper img folders; PDF conversion used existing publication PDFs only."
    }
    else {
        Write-Host ("Completed sync with converter: {0}" -f $converter.Path)
    }

    if ($tryPdfCrop -and ($null -eq $pdfCropTool)) {
        Write-Host "Note: pdfcrop was requested but not found; browser/native sizing was used without pdfcrop."
    }
}
finally {
    Pop-Location
}
