$ErrorActionPreference = "Stop"

$repo = "davis7dotsh/bgst"
$baseUrl = if ($env:BGST_BASE_URL) { $env:BGST_BASE_URL } else { "https://github.com/$repo/releases/latest/download" }
$installDir = if ($env:BGST_INSTALL_DIR) { $env:BGST_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "Programs\bgst" }
$machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
$arch = switch ($machine.ToUpperInvariant()) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default { throw "bgst: unsupported Windows architecture: $machine" }
}
$asset = "bgst-windows-$arch.exe"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("bgst-" + [System.Guid]::NewGuid())

try {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    $binaryPath = Join-Path $tempDir $asset
    $checksumsPath = Join-Path $tempDir "checksums.txt"

    Write-Host "Downloading bgst for windows/$arch..."
    Invoke-WebRequest "$baseUrl/$asset" -OutFile $binaryPath
    Invoke-WebRequest "$baseUrl/checksums.txt" -OutFile $checksumsPath

    $line = Get-Content $checksumsPath | Where-Object { $_ -match "^[0-9a-fA-F]{64}\s+\*?$([regex]::Escape($asset))$" } | Select-Object -First 1
    if (-not $line) { throw "bgst: release checksum is missing for $asset" }
    $expected = ($line -split "\s+")[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $binaryPath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "bgst: checksum verification failed" }

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item -Force $binaryPath (Join-Path $installDir "bgst.exe")

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $installDir) {
        $newPath = if ($userPath) { "$userPath;$installDir" } else { $installDir }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "Added $installDir to your user PATH. Open a new terminal to use it."
    }
    Write-Host "Installed bgst to $installDir\bgst.exe"
}
finally {
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tempDir
}
