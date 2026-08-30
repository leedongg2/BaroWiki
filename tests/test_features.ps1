param(
    [string]$BaseUrl = "http://127.0.0.1:3010",
    [string]$UserId = "admin",
    [string]$UserPw = "admin1234!"
)

$ErrorActionPreference = "Continue"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$pass = 0
$fail = 0

function Post-Form {
    param([string]$Path, [hashtable]$Body, $Sess = $null)
    if ($null -eq $Sess) { $Sess = $session }
    try {
        return Invoke-WebRequest -Uri ($BaseUrl + $Path) -Method Post -Body $Body -WebSession $Sess -UseBasicParsing -TimeoutSec 30
    } catch {
        return $null
    }
}

function Get-Page {
    param([string]$Path, $Sess = $null)
    if ($null -eq $Sess) { $Sess = $session }
    try {
        return Invoke-WebRequest -Uri ($BaseUrl + $Path) -WebSession $Sess -UseBasicParsing -TimeoutSec 30
    } catch {
        return $null
    }
}

function Check {
    param([string]$Name, [bool]$Condition)
    if ($Condition) { Write-Host ("  OK   " + $Name); $script:pass++ }
    else { Write-Host ("  FAIL " + $Name) -ForegroundColor Red; $script:fail++ }
}

Post-Form "/login" @{ id = $UserId; password = $UserPw } | Out-Null

Write-Host "=== 1. 이름공간 ==="
$r = Get-Page "/w/%EC%9C%84%ED%82%A4%EC%9A%B4%EC%98%81:%EA%B3%B5%EC%A7%80%EC%82%AC%ED%95%AD"
Check "위키운영 문서 접근" ($null -ne $r -and $r.StatusCode -eq 200)
Check "제목이 한국어 이름공간으로 표시" ($null -ne $r -and $r.Content -match "위키운영")
$r = Get-Page "/namespace"
Check "이름공간 목록" ($null -ne $r -and $r.Content -match "아이피사용자" -and $r.Content -match "파일휴지통")

Write-Host "=== 2. 휴지통 ==="
Post-Form "/edit/%ED%9C%B4%EC%A7%80%ED%86%B5%20%ED%85%8C%EC%8A%A4%ED%8A%B8" @{ content = "삭제 대상 문서"; send = "생성"; copyright_agreement = "on"; ver = "0" } | Out-Null
Post-Form "/delete/%ED%9C%B4%EC%A7%80%ED%86%B5%20%ED%85%8C%EC%8A%A4%ED%8A%B8" @{ send = "삭제 테스트"; copyright_agreement = "on" } | Out-Null
$r = Get-Page "/w/%ED%9C%B4%EC%A7%80%ED%86%B5%20%ED%85%8C%EC%8A%A4%ED%8A%B8"
Check "삭제 후 문서 없음" ($null -eq $r)
$r = Get-Page "/trash"
Check "휴지통 목록에 표시" ($null -ne $r -and $r.Content -match "휴지통 테스트")
$r = Post-Form "/trash_restore/trash:%ED%9C%B4%EC%A7%80%ED%86%B5%20%ED%85%8C%EC%8A%A4%ED%8A%B8" @{ send = "복원 테스트" }
Check "휴지통 복원 요청" ($null -ne $r)
$r = Get-Page "/w/%ED%9C%B4%EC%A7%80%ED%86%B5%20%ED%85%8C%EC%8A%A4%ED%8A%B8"
Check "복원된 문서 접근" ($null -ne $r -and $r.StatusCode -eq 200 -and $r.Content -match "삭제 대상 문서")

Write-Host "=== 3. 사용자 차단 ==="
$user_session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Post-Form "/register" @{ id = "testuser"; password = "testuser1234!"; password_check = "testuser1234!" } $user_session | Out-Null
Post-Form "/login" @{ id = "testuser"; password = "testuser1234!" } $user_session | Out-Null
$r = Get-Page "/user" $user_session
Check "테스트 사용자 로그인" ($null -ne $r -and $r.Content -match "testuser")
# 이전 실행에서 남은 개별 권한 초기화
Post-Form "/auth/user/testuser" @{ change_auth = "user" } | Out-Null

# 60초 차단 : 차단 상태 동작 확인
$r = Post-Form "/auth/ban/testuser" @{ name = "testuser"; do_ban_type = "normal"; ban_option = ""; date_preset = ""; date_minute = "1"; why = "차단 테스트" }
Check "기간 차단 실행 (1분)" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "차단 안내 표시" ($null -ne $r -and $r.Content -match "차단된 사용자입니다")
$r = Post-Form "/edit/FrontPage" @{ content = "차단 중 편집"; send = "차단 테스트"; copyright_agreement = "on"; ver = "0" } $user_session
Check "차단 중 편집 거부" ($null -ne $r -and $r.Content -match "권한|authority|없습니다")
Post-Form "/auth/ban/testuser" @{ name = "testuser"; do_ban_type = "normal"; ban_option = "release"; why = "해제" } | Out-Null

# 3초 차단 : 자동 만료 확인
$r = Post-Form "/auth/ban/testuser" @{ name = "testuser"; do_ban_type = "normal"; ban_option = ""; date_preset = ""; date_second = "3"; why = "만료 테스트" }
Check "초 단위 차단 실행 (3초)" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "초 단위 차단 적용" ($null -ne $r -and $r.Content -match "차단된 사용자입니다")
Start-Sleep -Seconds 4
$r = Get-Page "/w/FrontPage" $user_session
Check "차단 만료 후 안내 사라짐" ($null -ne $r -and $r.Content -notmatch "차단된 사용자입니다")

Write-Host "=== 4. 무기한 차단 / 해제 ==="
$r = Post-Form "/auth/ban/testuser" @{ name = "testuser"; do_ban_type = "normal"; ban_option = ""; date_preset = "0"; why = "무기한 테스트" }
Check "무기한 차단 실행" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "무기한 차단 안내" ($null -ne $r -and $r.Content -match "무기한")
$r = Post-Form "/auth/ban/testuser" @{ name = "testuser"; do_ban_type = "normal"; ban_option = "release"; why = "해제 테스트" }
Check "차단 해제 실행" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "해제 후 안내 사라짐" ($null -ne $r -and $r.Content -notmatch "차단된 사용자입니다")

Write-Host "=== 5. 경고 ==="
$r = Post-Form "/auth/warn/testuser" @{ user_name = "testuser"; why = "경고 테스트" }
Check "경고 발급" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "경고 안내 표시" ($null -ne $r -and $r.Content -match "경고를 받았습니다")
$r = Post-Form "/warn_check" @{} $user_session
Check "경고 확인 처리" ($null -ne $r)
$r = Get-Page "/w/FrontPage" $user_session
Check "확인 후 경고 안내 사라짐" ($null -ne $r -and $r.Content -notmatch "경고를 받았습니다")

Write-Host "=== 6. 권한 관리 ==="
$r = Get-Page "/auth/user/testuser"
Check "사용자 권한 페이지" ($null -ne $r -and $r.Content -match "권한 그룹")
$r = Post-Form "/auth/user/testuser" @{ change_auth = "user"; plus_acl = "1"; plus_ban = "1" }
Check "개별 권한 저장" ($null -ne $r)
$r = Get-Page "/auth/user/testuser"
Check "개별 권한 반영" ($null -ne $r -and $r.Content -match "ACL 관리")

Write-Host "=== 7. 이름공간 ACL ==="
# 앞 단계에서 부여한 개별 권한 제거 (ACL 우회 방지)
Post-Form "/auth/user/testuser" @{ change_auth = "user" } | Out-Null
$r = Post-Form "/namespace_acl/%EC%9C%84%ED%82%A4%EC%9A%B4%EC%98%81" @{ view = ""; decu = ""; document_edit_acl = "admin"; document_move_acl = ""; document_delete_acl = ""; dis = ""; acl_edit = "" }
Check "이름공간 ACL 저장" ($null -ne $r)
$r = Post-Form "/edit/%EC%9C%84%ED%82%A4%EC%9A%B4%EC%98%81:%EA%B3%B5%EC%A7%80%EC%82%AC%ED%95%AD" @{ content = "일반 사용자 편집 시도"; send = "acl 테스트"; copyright_agreement = "on"; ver = "0" } $user_session
Check "이름공간 ACL 로 편집 차단" ($null -ne $r -and $r.Content -match "권한|authority")

Write-Host ""
Write-Host ("PASS: " + $pass + " / FAIL: " + $fail)
