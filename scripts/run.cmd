@echo off
REM PaperValet bundle 启动脚本（Windows CMD）。
REM 在解压后的 papervalet-windows-amd64\ 目录里执行；自动定位 config.json。
REM
REM 用法：
REM   run.cmd                REM 启动
REM   run.cmd --version      REM 透传任意参数

setlocal

set "SCRIPT_DIR=%~dp0"
if "%SCRIPT_DIR:~-1%"=="\" set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

if defined PAPERVALET_HOME (
    set "HOME_DIR=%PAPERVALET_HOME%"
) else (
    set "HOME_DIR=%SCRIPT_DIR%"
)

set "BIN=%HOME_DIR%\bin\papervalet.exe"
set "CONFIG=%HOME_DIR%\config.json"

if not exist "%CONFIG%" if exist "%HOME_DIR%\config.example.json" (
    echo [WARN] %CONFIG% 不存在，已从 config.example.json 复制
    echo        请编辑后填入 api_id / api_hash 再启动
    copy /Y "%HOME_DIR%\config.example.json" "%CONFIG%" >NUL
)

if not exist "%BIN%" (
    echo [ERROR] 找不到可执行文件: %BIN% 1>&2
    exit /b 1
)

set "PAPERVALET_HOME=%HOME_DIR%"
cd /d "%HOME_DIR%"

"%BIN%" -config "%CONFIG%" %*
endlocal