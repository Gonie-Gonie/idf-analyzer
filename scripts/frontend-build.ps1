$ErrorActionPreference = "Stop"

$dist = Join-Path $PSScriptRoot "..\frontend\dist"
$index = Join-Path $dist "index.html"
$tools = Join-Path $dist "tools.html"
$guide = Join-Path $dist "guide.html"
$settings = Join-Path $dist "settings.html"
$entry = Join-Path $dist "app.js"
$moduleDir = Join-Path $dist "js"

if (-not (Test-Path $index)) {
    throw "Missing frontend/dist/index.html"
}

if (-not (Test-Path $tools)) {
    throw "Missing frontend/dist/tools.html"
}

if (-not (Test-Path $guide)) {
    throw "Missing frontend/dist/guide.html"
}

if (-not (Test-Path $settings)) {
    throw "Missing frontend/dist/settings.html"
}

if (-not (Test-Path $entry)) {
    throw "Missing frontend/dist/app.js"
}

$modules = @(
    "actions.js",
    "analysis-views.js",
    "app-info.js",
    "geometry-loader.js",
    "geometry-view.js",
    "input-views.js",
    "layout.js",
    "main.js",
    "navigation.js",
    "profile-views.js",
    "sample.js",
    "settings-client.js",
    "shortcuts.js",
    "state.js",
    "tools.js",
    "view-history.js"
)

foreach ($module in $modules) {
    $path = Join-Path $moduleDir $module
    if (-not (Test-Path $path)) {
        throw "Missing frontend/dist/js/$module"
    }
}

$wailsPath = Join-Path $PSScriptRoot "..\wails.json"
$appInfo = Join-Path $moduleDir "app-info.js"
$wailsConfig = Get-Content -LiteralPath $wailsPath -Raw | ConvertFrom-Json
$productVersion = [string]$wailsConfig.info.productVersion
if ([string]::IsNullOrWhiteSpace($productVersion)) {
    throw "Missing info.productVersion in wails.json"
}

$appInfoText = Get-Content -LiteralPath $appInfo -Raw
if ($appInfoText -notmatch 'version:\s*"([^"]+)"') {
    throw "Missing bundled app version in frontend/dist/js/app-info.js"
}
if ($Matches[1] -ne $productVersion) {
    throw "App version mismatch: wails.json=$productVersion app-info.js=$($Matches[1])"
}
if ($appInfoText -notmatch ('outputFilename:\s*"idf-analyzer-v' + [regex]::Escape($productVersion) + '"')) {
    throw "App output filename does not match version $productVersion in frontend/dist/js/app-info.js"
}

$staticVersionChecks = @(
    @($index, 'data-app-version[^>]*>v' + [regex]::Escape($productVersion) + '<'),
    @($tools, 'data-app-brand-version[^>]*>IDF ANALYZER V' + [regex]::Escape($productVersion) + '<'),
    @($guide, 'data-app-brand-version[^>]*>IDF ANALYZER V' + [regex]::Escape($productVersion) + '<'),
    @($settings, 'data-app-brand-version[^>]*>IDF ANALYZER V' + [regex]::Escape($productVersion) + '<')
)
foreach ($check in $staticVersionChecks) {
    $path = [string]$check[0]
    $pattern = [string]$check[1]
    $text = Get-Content -LiteralPath $path -Raw
    if ($text -notmatch $pattern) {
        throw "Static app version placeholder in $path does not match $productVersion"
    }
}

$threeModule = Join-Path $dist "vendor\three.module.js"
if (-not (Test-Path $threeModule)) {
    throw "Missing frontend/dist/vendor/three.module.js"
}

$defaultSample = Join-Path $dist "samples\RefBldgLargeOfficeNew2004_Chicago.idf"
if (-not (Test-Path $defaultSample)) {
    throw "Missing frontend/dist/samples/RefBldgLargeOfficeNew2004_Chicago.idf"
}

Write-Host "Static frontend is ready."
