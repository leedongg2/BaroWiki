param(
    [string]$BaseUrl = "http://127.0.0.1:3010",
    [string]$UserId = "admin",
    [string]$UserPw = "admin1234!"
)

$ErrorActionPreference = "Stop"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession

function Post-Form {
    param([string]$Path, [hashtable]$Body)
    try {
        $r = Invoke-WebRequest -Uri ($BaseUrl + $Path) -Method Post -Body $Body -WebSession $session -UseBasicParsing -TimeoutSec 30 -MaximumRedirection 5
        return $r
    } catch {
        Write-Host ("POST " + $Path + " => " + $_.Exception.Message)
        return $null
    }
}

function Get-Page {
    param([string]$Path)
    try {
        return Invoke-WebRequest -Uri ($BaseUrl + $Path) -WebSession $session -UseBasicParsing -TimeoutSec 30
    } catch {
        Write-Host ("GET " + $Path + " => " + $_.Exception.Message)
        return $null
    }
}

# 1. 회원가입
$r = Post-Form "/register" @{ id = $UserId; password = $UserPw; password_check = $UserPw }
Write-Host ("register status: " + $(if ($r) { $r.StatusCode } else { "fail" }))

# 2. 로그인
$r = Post-Form "/login" @{ id = $UserId; password = $UserPw }
Write-Host ("login status: " + $(if ($r) { $r.StatusCode } else { "fail" }))

$check = Get-Page "/user"
if ($check -and $check.Content -match [regex]::Escape($UserId)) { Write-Host "login OK" } else { Write-Host "login CHECK FAILED" }

# 3. 문서 만들기
$docs = @{
    "FrontPage" = "[목차]`n= 개요 =`n'''바로위키'''는 나무위키와 같은 사용 환경을 목표로 하는 위키입니다.`n`n= 기능 =`n * 이름공간`n * 사용자 차단/경고`n * 권한 관리`n * ACL`n * 파일 업로드 (mp3/mp4)`n * 게시판`n`n= 링크 =`n * [[위키운영:공지사항]]`n * [[바로위키:소개]]`n"
    "위키운영:공지사항" = "= 공지사항 =`n이 문서는 위키운영 이름공간의 문서입니다.`n"
    "바로위키:소개" = "= 소개 =`n바로위키에 대한 문서입니다.`n"
    "테스트 문서" = "[목차]`n= 개요 =`n테스트용 문서입니다. [[FrontPage|앞 문서]]로 돌아갈 수 있습니다.`n`n= 표 =`n|| 항목 || 값 ||`n|| 이름공간 || 8개 ||`n`n= 각주 =`n각주 테스트[* 각주 내용입니다.]`n"
}

foreach ($name in $docs.Keys) {
    $body = @{ content = $docs[$name]; send = "문서 생성"; copyright_agreement = "on"; ver = "0" }
    $r = Post-Form ("/edit/" + [uri]::EscapeDataString($name)) $body
    Write-Host ("edit " + $name + " => " + $(if ($r) { $r.StatusCode } else { "fail" }))
}

Write-Host "seed done"
