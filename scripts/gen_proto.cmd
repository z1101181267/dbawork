@echo off
rem DBAWORK 生成 Go gRPC stub（绕过 PowerShell 执行策略限制）
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0gen_proto.ps1"
