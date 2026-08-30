param(
    [string]$BaseUrl = "http://127.0.0.1:3010"
)

$ErrorActionPreference = "Continue"
$admin = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$user = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$anon = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$pass = 0
$fail = 0

function Post-Form { param([string]$Path, [hashtable]$Body, $Sess)
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -Method Post -Body $Body -WebSession $Sess -UseBasicParsing -TimeoutSec 30 } catch { return $null } }
function Get-Page { param([string]$Path, $Sess)
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -WebSession $Sess -UseBasicParsing -TimeoutSec 30 } catch { return $null } }
function Check { param([string]$Name, [bool]$Condition)
    if ($Condition) { Write-Host ("  OK   " + $Name); $script:pass++ } else { Write-Host ("  FAIL " + $Name) -ForegroundColor Red; $script:fail++ } }

Post-Form "/login" @{ id = "admin"; password = "admin1234!" } $admin | Out-Null
Post-Form "/register" @{ id = "privuser"; password = "privuser1234!"; password_check = "privuser1234!" } $user | Out-Null
Post-Form "/login" @{ id = "privuser"; password = "privuser1234!" } $user | Out-Null

# 비밀 문서를 만들고 삭제 -> 휴지통으로
$doc = "%EB%B9%84%EB%B0%80%EB%AC%B8%EC%84%9C%ED%85%8C%EC%8A%A4%ED%8A%B8"
Post-Form ("/edit/" + $doc) @{ content = "비밀 내용입니다"; send = "생성"; copyright_agreement = "on"; ver = "0" } $admin | Out-Null
Post-Form ("/delete/" + $doc) @{ send = "삭제"; copyright_agreement = "on" } $admin | Out-Null

$r = Get-Page "/trash" $admin
Check "관리자는 휴지통 열람 가능" ($null -ne $r -and $r.Content -match "비밀문서테스트")
$r = Get-Page "/trash" $user
Check "일반 사용자는 휴지통 접근 불가" ($null -eq $r -or $r.Content -notmatch "비밀문서테스트")
$r = Get-Page "/w/trash:%EB%B9%84%EB%B0%80%EB%AC%B8%EC%84%9C%ED%85%8C%EC%8A%A4%ED%8A%B8" $user
Check "일반 사용자는 휴지통 문서 열람 불가" ($null -eq $r -or $r.Content -notmatch "비밀 내용입니다")
$r = Get-Page "/search_page/1/%EB%B9%84%EB%B0%80%EB%AC%B8%EC%84%9C" $user
Check "검색 결과에서 휴지통 문서 제외" ($null -ne $r -and $r.Content -notmatch "휴지통:비밀문서테스트")
$r = Get-Page "/namespace/%ED%9C%B4%EC%A7%80%ED%86%B5/1" $user
Check "이름공간 목록에서 휴지통 문서 제외" ($null -ne $r -and $r.Content -notmatch "비밀문서테스트")
$r = Get-Page "/list/document/all/1" $anon
Check "전체 문서 목록에서 휴지통 문서 제외" ($null -ne $r -and $r.Content -notmatch "비밀문서테스트")

$r = Get-Page "/w/trash:%EB%B9%84%EB%B0%80%EB%AC%B8%EC%84%9C%ED%85%8C%EC%8A%A4%ED%8A%B8" $admin
Check "관리자는 휴지통 문서 열람 가능" ($null -ne $r -and $r.Content -match "비밀 내용입니다")

Write-Host ""
Write-Host ("PASS: " + $pass + " / FAIL: " + $fail)
