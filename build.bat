@echo off

set dest="dist/emeral.dll"

if not "%~1"=="" set dest="%~1"

go build -o "%dest%" -buildmode=c-shared main.go