# ============================================================
#  위키 서버 실행 스크립트
#
#  사용법 :
#    .\start.ps1          (3000번 포트, 사용 중이면 자동으로 다음 포트)
#    .\start.ps1 8080     (포트 직접 지정)
#    .\start.ps1 -NoBrowser
#
#  실행 파일(onamu.exe)이 없으면 Go 로 자동 빌드를 시도한다.
#  종료는 Ctrl + C
# ============================================================

param(
    [int]$Port = 0,
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
if ([string]::IsNullOrEmpty($root)) { $root = (Get-Location).Path }
Set-Location -LiteralPath $root

function Test-PortFree {
    param([int]$TestPort)

    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Any, $TestPort)
        $listener.Start()
        $listener.Stop()
        return $true
    } catch {
        return $false
    }
}

function Find-Go {
    $candidates = @()

    $command = Get-Command go -ErrorAction SilentlyContinue
    if ($command) { $candidates += $command.Source }

    $candidates += "$env:USERPROFILE\go-sdk\go\bin\go.exe"
    $candidates += "C:\Program Files\Go\bin\go.exe"
    $candidates += "$env:LOCALAPPDATA\Programs\Go\bin\go.exe"

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) { return $candidate }
    }

    return $null
}

# ---------- 실행 파일 준비 ----------
$exe = Join-Path $root "onamu.exe"

if (-not (Test-Path -LiteralPath $exe)) {
    Write-Host "onamu.exe 가 없습니다. Go 로 빌드를 시도합니다..." -ForegroundColor Yellow

    if (-not (Test-Path -LiteralPath (Join-Path $root "main.go"))) {
        Write-Host "소스(main.go)도 없어서 빌드할 수 없습니다. onamu.exe 를 이 폴더에 두고 다시 실행하세요." -ForegroundColor Red
        exit 1
    }

    $go = Find-Go
    if (-not $go) {
        Write-Host "Go 를 찾지 못했습니다. https://go.dev/dl/ 에서 설치 후 다시 실행하세요." -ForegroundColor Red
        exit 1
    }

    & $go build -o $exe .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "빌드에 실패했습니다." -ForegroundColor Red
        exit 1
    }

    Write-Host "빌드 완료" -ForegroundColor Green
}

# ---------- 포트 결정 ----------
if ($Port -le 0) { $Port = 3000 }

$start_port = $Port
while (-not (Test-PortFree $Port)) {
    Write-Host ("포트 " + $Port + " 이(가) 사용 중입니다. 다음 포트를 확인합니다.") -ForegroundColor Yellow
    $Port++

    if ($Port -gt $start_port + 20) {
        Write-Host "빈 포트를 찾지 못했습니다." -ForegroundColor Red
        exit 1
    }
}

$url = "http://127.0.0.1:$Port"

Write-Host ""
Write-Host "  위키를 시작합니다 : $url" -ForegroundColor Green
Write-Host "  종료하려면 Ctrl + C" -ForegroundColor DarkGray
Write-Host ""

# ---------- 서버 실행 ----------
$process = Start-Process -FilePath $exe -ArgumentList $Port -WorkingDirectory $root -NoNewWindow -PassThru

try {
    # 서버가 응답할 때까지 대기 후 브라우저 열기
    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        if ($process.HasExited) { break }

        try {
            Invoke-WebRequest -Uri $url -UseBasicParsing -TimeoutSec 3 | Out-Null
            $ready = $true
            break
        } catch {
            if ($_.Exception.Response) { $ready = $true; break }
            Start-Sleep -Milliseconds 500
        }
    }

    if ($ready -and -not $NoBrowser) {
        Start-Process $url | Out-Null
    }

    # 서버가 바로 죽었으면 원인을 알려준다
    if ($process.HasExited) {
        Write-Host ""
        Write-Host ("  위키가 시작하자마자 종료되었습니다. (종료 코드 : " + $process.ExitCode + ")") -ForegroundColor Red
        Write-Host "  바로 위에 출력된 오류 메시지가 원인입니다." -ForegroundColor Yellow
        Write-Host ("  직접 실행해서 확인 : .\onamu.exe " + $Port) -ForegroundColor Yellow
        Write-Host "  이미 실행 중인 위키가 있는지 확인 : Get-Process onamu" -ForegroundColor DarkGray
        exit 1
    }

    Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Write-Host ""
        Write-Host "  위키를 종료했습니다." -ForegroundColor DarkGray
    }
}
