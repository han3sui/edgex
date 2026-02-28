param(
  [string]$ServiceName = "edge-gateway",
  [string]$BinaryPath = ".\\main.exe"
)

Stop-Process -Name $ServiceName -ErrorAction SilentlyContinue
Start-Process -FilePath $BinaryPath -WindowStyle Hidden
Start-Sleep -Seconds 2
Write-Host "Service started."
