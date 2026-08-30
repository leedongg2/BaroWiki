param(
    [string]$BaseUrl = "http://127.0.0.1:3010",
    [string]$UserId = "admin",
    [string]$UserPw = "admin1234!"
)

$ErrorActionPreference = "Continue"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$pass = 0
$fail = 0

function Post-Form { param([string]$Path, [hashtable]$Body)
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -Method Post -Body $Body -WebSession $session -UseBasicParsing -TimeoutSec 30 } catch { return $null } }
function Get-Page { param([string]$Path)
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -WebSession $session -UseBasicParsing -TimeoutSec 30 } catch { return $null } }
function Check { param([string]$Name, [bool]$Condition)
    if ($Condition) { Write-Host ("  OK   " + $Name); $script:pass++ } else { Write-Host ("  FAIL " + $Name) -ForegroundColor Red; $script:fail++ } }

Post-Form "/login" @{ id = $UserId; password = $UserPw } | Out-Null

$doc = "%EB%AC%B8%EB%8B%A8%ED%8E%B8%EC%A7%91%ED%85%8C%EC%8A%A4%ED%8A%B8"
$content = "= 첫 문단 =`n첫 문단 내용`n`n== 하위 문단 ==`n하위 내용`n`n= 둘째 문단 =`n둘째 내용`n"
Post-Form ("/edit/" + $doc) @{ content = $content; send = "생성"; copyright_agreement = "on"; ver = "0" } | Out-Null

$r = Get-Page ("/w/" + $doc)
Check "문단 편집 링크 표시" ($null -ne $r -and $r.Content -match "opennamu_heading_edit")
Check "문단 편집 링크 주소" ($null -ne $r -and $r.Content -match "section=2")

$r = Get-Page ("/edit/" + $doc + "?section=2")
Check "문단 편집 화면" ($null -ne $r -and $r.Content -match "둘째 내용" -and $r.Content -notmatch "첫 문단 내용")

# 문단 저장
$rev = ""
if ($r.Content -match 'name="ver" value="([0-9]+)"') { $rev = $Matches[1] }
Post-Form ("/edit/" + $doc) @{ content = "= 둘째 문단 =`n둘째 내용 수정됨`n"; send = "문단 편집"; copyright_agreement = "on"; ver = $rev; section = "2" } | Out-Null

$r = Get-Page ("/w/" + $doc)
Check "문단 저장 후 내용 반영" ($null -ne $r -and $r.Content -match "둘째 내용 수정됨")
Check "다른 문단 유지" ($null -ne $r -and $r.Content -match "첫 문단 내용" -and $r.Content -match "하위 내용")

# 하위 문단 편집 (1.1)
$r = Get-Page ("/edit/" + $doc + "?section=1.1")
Check "하위 문단 편집 화면" ($null -ne $r -and $r.Content -match "하위 내용" -and $r.Content -notmatch "둘째 내용")

$rev = ""
if ($r.Content -match 'name="ver" value="([0-9]+)"') { $rev = $Matches[1] }
Post-Form ("/edit/" + $doc) @{ content = "== 하위 문단 ==`n하위 내용 수정`n"; send = "하위 문단 편집"; copyright_agreement = "on"; ver = $rev; section = "1.1" } | Out-Null
$r = Get-Page ("/w/" + $doc)
Check "하위 문단 저장" ($null -ne $r -and $r.Content -match "하위 내용 수정" -and $r.Content -match "둘째 내용 수정됨")

Write-Host ""
Write-Host ("PASS: " + $pass + " / FAIL: " + $fail)
