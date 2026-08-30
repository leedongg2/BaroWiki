@echo off
chcp 65001 > nul
rem 더블 클릭으로도 실행되는 위키 시작 파일
where pwsh > nul 2>&1
if %errorlevel%==0 (
    pwsh -NoProfile -ExecutionPolicy Bypass -File "%~dp0start.ps1" %*
) else (
    powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0start.ps1" %*
)
pause
