# 生成 Go gRPC stub（优先系统 protoc；否则回退 .tools\venv 的 grpc_tools）
$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot          # DBAWORK
$protoDir = Join-Path $root 'proto'
$outDir = Join-Path $root 'go-backend'
$pluginDir = Join-Path $env:USERPROFILE 'go\bin'
$protoFile = Join-Path $protoDir 'dbawork.proto'

if (-not (Test-Path $protoFile)) { throw "缺少 $protoFile" }

$protoc = Get-Command protoc -ErrorAction SilentlyContinue
if ($protoc) {
    Write-Output "使用系统 protoc: $($protoc.Source)"
    & $protoc.Source "-I" $protoDir `
        "--go_out=$outDir" "--go_opt=module=dbawork" `
        "--go-grpc_out=$outDir" "--go-grpc_opt=module=dbawork" `
        $protoFile
} else {
    $py = Join-Path $root '.tools\venv\Scripts\python.exe'
    if (-not (Test-Path $py)) {
        throw "未找到 protoc。请安装 protoc，或先创建工具虚拟环境：python -m venv .tools\venv && .tools\venv\Scripts\python.exe -m pip install grpcio-tools"
    }
    Write-Output "使用 grpc_tools.protoc: $py"
    $env:PATH = "$pluginDir;$env:PATH"
    & $py -m grpc_tools.protoc "-I$protoDir" `
        "--go_out=$outDir" "--go_opt=module=dbawork" `
        "--go-grpc_out=$outDir" "--go-grpc_opt=module=dbawork" `
        "--plugin=protoc-gen-go=$pluginDir\protoc-gen-go.exe" `
        "--plugin=protoc-gen-go-grpc=$pluginDir\protoc-gen-go-grpc.exe" `
        $protoFile
}
if ($LASTEXITCODE -ne 0) { throw "proto 生成失败（exit=$LASTEXITCODE）" }
Write-Output "生成完成: go-backend/proto/dbaworkv1/"
