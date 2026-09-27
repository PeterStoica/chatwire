$ErrorActionPreference = 'Stop'
$repo = 'PeterStoica/chatwire'
$base = if ($env:CHATWIRE_BASE_URL) { $env:CHATWIRE_BASE_URL } else { "https://github.com/$repo/releases/latest/download" }
$arch = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq 'Arm64') { 'arm64' } else { 'amd64' }
$asset = "chatwire_windows_$arch.exe"
$dir = if ($env:CHATWIRE_INSTALL_DIR) { $env:CHATWIRE_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\chatwire' }
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
New-Item -ItemType Directory -Force -Path $dir, $tmp | Out-Null
try {
    Invoke-WebRequest -UseBasicParsing "$base/$asset" -OutFile (Join-Path $tmp $asset)
    Invoke-WebRequest -UseBasicParsing "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')
    $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { "$(($_ -split '\s+')[1])".TrimStart('*') -eq $asset } | Select-Object -First 1
    $want = if ($line) { ($line -split '\s+')[0] } else { '' }
    $got = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
    if (-not $want -or $want -ne $got) { throw 'chatwire: the download does not match its checksum; nothing was installed' }
    Copy-Item (Join-Path $tmp $asset) (Join-Path $dir 'chatwire.exe') -Force
}
finally {
    Remove-Item -Recurse -Force $tmp
}
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not (($userPath -split ';') -contains $dir)) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
}
$version = & (Join-Path $dir 'chatwire.exe') version
Write-Host "Installed $dir\chatwire.exe ($version)"
Write-Host "Next: open a new terminal, then run: chatwire setup"
