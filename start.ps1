# ============================================================
#  바로위키 로컬 실행 스크립트
#
#  사용법 :
#    .\start.ps1              3000번 포트로 연다 (쓰는 중이면 다음 포트)
#    .\start.ps1 8080         포트 직접 정하기
#    .\start.ps1 -Fresh       무조건 새로 빌드하고 연다
#    .\start.ps1 -Lan         같은 공유기 안 다른 기기(휴대폰 등)에서도 열 수 있게
#    .\start.ps1 -Data 임시    자료를 따로 떠서 연다 (원래 data.db 를 건드리지 않는다)
#    .\start.ps1 -NoBrowser   브라우저를 저절로 열지 않는다
#
#  소스가 실행 파일보다 새것이면 알아서 다시 빌드한다.
#  종료는 Ctrl + C
# ============================================================

param(
    [int]$Port = 0,
    [switch]$NoBrowser,
    [switch]$Fresh,
    [switch]$Lan,
    [string]$Data = ""
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

# 소스가 실행 파일보다 새것인지 본다.
#
#   예전에는 onamu.exe 가 "없을 때만" 빌드했다. 그래서 소스를 고쳐도 옛 실행 파일이 그대로 열려
#   새로 만든 기능이 하나도 보이지 않는 일이 있었다. 이제는 시각을 견주어 알아서 다시 빌드한다.
function Test-BuildNeeded {
    param([string]$ExePath)

    if (-not (Test-Path -LiteralPath $ExePath)) { return $true }

    $built = (Get-Item -LiteralPath $ExePath).LastWriteTimeUtc

    foreach ($pattern in @("*.go", "go.mod", "go.sum", "version.json")) {
        $newer = Get-ChildItem -LiteralPath $root -Filter $pattern -File -ErrorAction SilentlyContinue |
            Where-Object { $_.LastWriteTimeUtc -gt $built } |
            Select-Object -First 1

        if ($newer) { return $true }
    }

    foreach ($folder in @("route", "views", "lang")) {
        $path = Join-Path $root $folder
        if (-not (Test-Path -LiteralPath $path)) { continue }

        $newer = Get-ChildItem -LiteralPath $path -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.LastWriteTimeUtc -gt $built } |
            Select-Object -First 1

        if ($newer) { return $true }
    }

    return $false
}

function Get-LanAddress {
    try {
        $found = Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
            Where-Object {
                $_.IPAddress -ne "127.0.0.1" -and
                $_.PrefixOrigin -ne "WellKnown" -and
                $_.IPAddress -notlike "169.254.*"
            } |
            Select-Object -First 1

        if ($found) { return $found.IPAddress }
    } catch {
        # Get-NetIPAddress 가 없는 오래된 환경
    }

    try {
        $entry = [System.Net.Dns]::GetHostEntry([System.Net.Dns]::GetHostName())
        foreach ($address in $entry.AddressList) {
            if ($address.AddressFamily -eq "InterNetwork" -and $address.IPAddressToString -ne "127.0.0.1") {
                return $address.IPAddressToString
            }
        }
    } catch {
        # 이름 풀이 실패
    }

    return ""
}

# ---------- 실행 파일 준비 ----------
$exe = Join-Path $root "onamu.exe"

$need_build = $Fresh.IsPresent
if (-not $need_build) { $need_build = Test-BuildNeeded $exe }

if ($need_build) {
    $have_source = Test-Path -LiteralPath (Join-Path $root "main.go")
    $have_exe = Test-Path -LiteralPath $exe
    $go = $null

    if ($have_source) { $go = Find-Go }

    if (-not $have_source -or -not $go) {
        if ($have_exe) {
            Write-Host "새로 빌드하지 못했습니다. 이미 있는 onamu.exe 를 그대로 씁니다. (최신이 아닐 수 있습니다)" -ForegroundColor Yellow
        } elseif (-not $have_source) {
            Write-Host "소스(main.go)도 onamu.exe 도 없습니다." -ForegroundColor Red
            exit 1
        } else {
            Write-Host "Go 를 찾지 못했습니다. https://go.dev/dl/ 에서 설치한 뒤 다시 실행하세요." -ForegroundColor Red
            exit 1
        }
    } else {
        if ($have_exe) {
            Write-Host "소스가 더 새것입니다. 다시 빌드합니다..." -ForegroundColor Yellow
        } else {
            Write-Host "onamu.exe 가 없습니다. 빌드합니다..." -ForegroundColor Yellow
        }

        # 열려 있는 위키가 파일을 잡고 있으면 덮어쓸 수 없다. 새 이름으로 만든 뒤 바꿔 끼운다.
        $temp_exe = Join-Path $root "onamu.new.exe"
        & $go build -o $temp_exe .

        if ($LASTEXITCODE -ne 0) {
            Remove-Item -LiteralPath $temp_exe -Force -ErrorAction SilentlyContinue
            Write-Host "빌드에 실패했습니다. 위에 나온 오류를 확인하세요." -ForegroundColor Red
            exit 1
        }

        try {
            Move-Item -LiteralPath $temp_exe -Destination $exe -Force
        } catch {
            Remove-Item -LiteralPath $temp_exe -Force -ErrorAction SilentlyContinue
            Write-Host "onamu.exe 를 바꿔 끼우지 못했습니다. 이미 열려 있는 위키가 있는지 확인하세요." -ForegroundColor Red
            Write-Host "  열려 있는 것 보기 : Get-Process onamu" -ForegroundColor DarkGray
            exit 1
        }

        Write-Host "빌드 완료" -ForegroundColor Green
    }
}

# ---------- 자료 폴더 ----------
#   -Data 를 주면 그 폴더를 일터로 삼는다. 원래 data.db 는 손대지 않는다.
$work = $root

if ($Data -ne "") {
    $work = $Data
    if (-not [System.IO.Path]::IsPathRooted($work)) { $work = Join-Path $root $work }

    if (-not (Test-Path -LiteralPath $work)) {
        New-Item -ItemType Directory -Path $work | Out-Null
    }

    $work_db = Join-Path $work "data.db"
    if (-not (Test-Path -LiteralPath $work_db)) {
        $origin_db = Join-Path $root "data.db"

        if (Test-Path -LiteralPath $origin_db) {
            Copy-Item -LiteralPath $origin_db -Destination $work_db
            Write-Host "지금 자료를 그대로 떠 왔습니다 : $work_db" -ForegroundColor DarkGray
        } else {
            Write-Host "빈 위키로 시작합니다." -ForegroundColor DarkGray
        }

        $origin_files = Join-Path $root "data"
        if (Test-Path -LiteralPath $origin_files) {
            Copy-Item -LiteralPath $origin_files -Destination (Join-Path $work "data") -Recurse -ErrorAction SilentlyContinue
        }
    }

    Write-Host "자료 폴더 : $work" -ForegroundColor DarkGray
}

# ---------- 포트 정하기 ----------
if ($Port -le 0) { $Port = 3000 }

$start_port = $Port
while (-not (Test-PortFree $Port)) {
    Write-Host ("포트 " + $Port + " 이(가) 쓰이고 있습니다. 다음 포트를 봅니다.") -ForegroundColor Yellow
    $Port++

    if ($Port -gt $start_port + 20) {
        Write-Host "빈 포트를 찾지 못했습니다." -ForegroundColor Red
        exit 1
    }
}

$url = "http://127.0.0.1:$Port"

Write-Host ""
Write-Host "  바로위키를 엽니다" -ForegroundColor Green
Write-Host "    이 컴퓨터에서 : $url" -ForegroundColor Green

if ($Lan) {
    $lan_ip = Get-LanAddress

    if ($lan_ip -ne "") {
        Write-Host "    같은 공유기 안에서 : http://${lan_ip}:$Port" -ForegroundColor Green
        Write-Host "    (처음 한 번은 윈도 방화벽이 묻습니다. 「개인 네트워크」 를 허용해 주세요)" -ForegroundColor DarkGray
    } else {
        Write-Host "    이 컴퓨터의 주소를 찾지 못했습니다. ipconfig 로 확인해 주세요." -ForegroundColor Yellow
    }
}

Write-Host "  종료하려면 Ctrl + C" -ForegroundColor DarkGray
Write-Host ""

# ---------- 서버 열기 ----------
$process = Start-Process -FilePath $exe -ArgumentList $Port -WorkingDirectory $work -NoNewWindow -PassThru

try {
    # 서버가 대답할 때까지 기다렸다가 브라우저를 연다
    $ready = $false
    for ($i = 0; $i -lt 60; $i++) {
        if ($process.HasExited) { break }

        try {
            Invoke-WebRequest -Uri "$url/healthz" -UseBasicParsing -TimeoutSec 3 | Out-Null
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

    # 시작하자마자 죽었으면 까닭을 알려 준다
    if ($process.HasExited) {
        Write-Host ""
        Write-Host ("  위키가 시작하자마자 끝났습니다. (끝난 값 : " + $process.ExitCode + ")") -ForegroundColor Red
        Write-Host "  바로 위에 나온 오류가 까닭입니다." -ForegroundColor Yellow
        Write-Host ("  손으로 실행해 보기 : .\onamu.exe " + $Port) -ForegroundColor Yellow
        Write-Host "  이미 열려 있는 위키가 있는지 보기 : Get-Process onamu" -ForegroundColor DarkGray
        exit 1
    }

    Wait-Process -Id $process.Id -ErrorAction SilentlyContinue
} finally {
    if ($process -and -not $process.HasExited) {
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        Write-Host ""
        Write-Host "  위키를 닫았습니다." -ForegroundColor DarkGray
    }
}
