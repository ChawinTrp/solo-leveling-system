# build.ps1
$env:GOOS="js"
$env:GOARCH="wasm"
go build -o main.wasm main.go
Write-Output "✅ WASM re-build complete at $(Get-Date -Format 'HH:mm:ss')"