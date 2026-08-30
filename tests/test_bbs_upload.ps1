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
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -Method Post -Body $Body -WebSession $Sess -UseBasicParsing -TimeoutSec 60 }
    catch { return $null }
}

function Get-Page {
    param([string]$Path, $Sess = $null)
    if ($null -eq $Sess) { $Sess = $session }
    try { return Invoke-WebRequest -Uri ($BaseUrl + $Path) -WebSession $Sess -UseBasicParsing -TimeoutSec 60 }
    catch { return $null }
}

function Check {
    param([string]$Name, [bool]$Condition)
    if ($Condition) { Write-Host ("  OK   " + $Name); $script:pass++ }
    else { Write-Host ("  FAIL " + $Name) -ForegroundColor Red; $script:fail++ }
}

Post-Form "/login" @{ id = $UserId; password = $UserPw } | Out-Null

Write-Host "=== 게시판 ==="
Post-Form "/bbs/make" @{ bbs_name = "자유게시판"; bbs_type = "thread" } | Out-Null
$r = Get-Page "/bbs/main"
Check "게시판 생성" ($null -ne $r -and $r.Content -match "자유게시판")

# 방금 만든 게시판 번호 찾기
$bbs_id = ""
if ($r -and $r.Content -match '/bbs/in/(\d+)') { $bbs_id = $Matches[1] }
Check "게시판 번호 확인" ($bbs_id -ne "")

Post-Form ("/bbs/edit/" + $bbs_id) @{ title = "첫 번째 글"; content = "게시판 테스트 내용"; prefix = ""; tags = "공지, 테스트" } | Out-Null
$r = Get-Page ("/bbs/in/" + $bbs_id)
Check "게시글 작성" ($null -ne $r -and $r.Content -match "첫 번째 글")
Check "게시판 표 UI" ($null -ne $r -and $r.Content -match "namu_bbs_table")

Write-Host "=== 게시판 차단 ==="
$user_session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Post-Form "/register" @{ id = "bbsuser"; password = "bbsuser1234!"; password_check = "bbsuser1234!" } $user_session | Out-Null
Post-Form "/login" @{ id = "bbsuser"; password = "bbsuser1234!" } $user_session | Out-Null

$r = Post-Form ("/bbs/edit/" + $bbs_id) @{ title = "일반 사용자 글"; content = "차단 전 작성"; prefix = ""; tags = "" } $user_session
$r = Get-Page ("/bbs/in/" + $bbs_id)
Check "일반 사용자 글 작성 가능" ($null -ne $r -and $r.Content -match "일반 사용자 글")

Post-Form "/bbs/block" @{ user_name = "bbsuser"; bbs_id = $bbs_id; date_preset = "0"; why = "게시판 차단 테스트"; do_type = "" } | Out-Null
$r = Get-Page ("/bbs/in/" + $bbs_id) $user_session
Check "게시판 차단 안내" ($null -ne $r -and $r.Content -match "게시판 이용이 제한")
$r = Post-Form ("/bbs/edit/" + $bbs_id) @{ title = "차단 후 글"; content = "차단 후 작성"; prefix = ""; tags = "" } $user_session
$r2 = Get-Page ("/bbs/in/" + $bbs_id)
Check "차단된 사용자 글 작성 거부" ($null -ne $r2 -and $r2.Content -notmatch "차단 후 글")

Post-Form "/bbs/block" @{ user_name = "bbsuser"; bbs_id = $bbs_id; do_type = "release" } | Out-Null
$r = Post-Form ("/bbs/edit/" + $bbs_id) @{ title = "해제 후 글"; content = "해제 후 작성"; prefix = ""; tags = "" } $user_session
$r2 = Get-Page ("/bbs/in/" + $bbs_id)
Check "차단 해제 후 글 작성 가능" ($null -ne $r2 -and $r2.Content -match "해제 후 글")

Write-Host "=== 게시판 잠금 ==="
Post-Form ("/bbs/lock/" + $bbs_id) @{ lock = "1" } | Out-Null
$r = Post-Form ("/bbs/edit/" + $bbs_id) @{ title = "잠금 후 글"; content = "잠금 후 작성"; prefix = ""; tags = "" } $user_session
$r2 = Get-Page ("/bbs/in/" + $bbs_id)
Check "잠긴 게시판 글 작성 거부" ($null -ne $r2 -and $r2.Content -notmatch "잠금 후 글")
Post-Form ("/bbs/lock/" + $bbs_id) @{ } | Out-Null
$r2 = Get-Page ("/bbs/in/" + $bbs_id)
Check "잠금 해제" ($null -ne $r2 -and $r2.Content -notmatch "잠긴 게시판입니다")

Write-Host "=== 파일 업로드 (mp3 / mp4) ==="
Add-Type -AssemblyName System.Net.Http
function Upload-File {
    param([string]$FileName, [byte[]]$Bytes, [string]$Name)
    $handler = New-Object System.Net.Http.HttpClientHandler
    $handler.CookieContainer = New-Object System.Net.CookieContainer
    foreach ($c in $session.Cookies.GetCookies($BaseUrl)) {
        $handler.CookieContainer.Add((New-Object System.Net.Cookie($c.Name, $c.Value, "/", "127.0.0.1")))
    }
    $client = New-Object System.Net.Http.HttpClient($handler)
    $form = New-Object System.Net.Http.MultipartFormDataContent
    $content = New-Object System.Net.Http.ByteArrayContent(,$Bytes)
    $content.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::Parse("application/octet-stream")
    $form.Add($content, "f_data[]", $FileName)
    $form.Add((New-Object System.Net.Http.StringContent($Name)), "f_name")
    $form.Add((New-Object System.Net.Http.StringContent("direct_input")), "f_lice_sel")
    $form.Add((New-Object System.Net.Http.StringContent("테스트 업로드")), "f_lice")
    $response = $client.PostAsync(($BaseUrl + "/upload"), $form).Result
    $body = $response.Content.ReadAsStringAsync().Result
    $client.Dispose()
    return $body
}

$mp3_bytes = [byte[]](0x49,0x44,0x33,0x03,0x00,0x00,0x00,0x00,0x00,0x00) + (New-Object byte[] 200)
$mp4_bytes = [byte[]](0x00,0x00,0x00,0x18,0x66,0x74,0x79,0x70,0x6D,0x70,0x34,0x32) + (New-Object byte[] 200)

$body = Upload-File "test.mp3" $mp3_bytes "테스트음악"
Check "mp3 업로드" ($body -notmatch "확장자" -and $body -notmatch "extension")
$body = Upload-File "test.mp4" $mp4_bytes "테스트영상"
Check "mp4 업로드" ($body -notmatch "확장자" -and $body -notmatch "extension")

$r = Get-Page "/w/%ED%8C%8C%EC%9D%BC:%ED%85%8C%EC%8A%A4%ED%8A%B8%EC%9D%8C%EC%95%85.mp3"
Check "mp3 파일 문서 (오디오 재생기)" ($null -ne $r -and $r.Content -match "opennamu_audio")
$r = Get-Page "/w/%ED%8C%8C%EC%9D%BC:%ED%85%8C%EC%8A%A4%ED%8A%B8%EC%98%81%EC%83%81.mp4"
Check "mp4 파일 문서 (동영상 재생기)" ($null -ne $r -and $r.Content -match "opennamu_video")

# 문서에서 미디어 삽입
$content2 = "= 미디어 =`n[[파일:테스트음악.mp3]]`n[[파일:테스트영상.mp4]]`n"
Post-Form "/edit/%EB%AF%B8%EB%94%94%EC%96%B4%ED%85%8C%EC%8A%A4%ED%8A%B8" @{ content = $content2; send = "미디어"; copyright_agreement = "on"; ver = "0" } | Out-Null
$r = Get-Page "/w/%EB%AF%B8%EB%94%94%EC%96%B4%ED%85%8C%EC%8A%A4%ED%8A%B8"
Check "문서 내 오디오 재생기" ($null -ne $r -and $r.Content -match "opennamu_audio")
Check "문서 내 동영상 재생기" ($null -ne $r -and $r.Content -match "opennamu_video")

Write-Host "=== 파일 휴지통 ==="
Post-Form "/delete_file/file:%ED%85%8C%EC%8A%A4%ED%8A%B8%EC%9D%8C%EC%95%85.mp3" @{ with_doc = "1"; send = "파일 삭제 테스트"; copyright_agreement = "on" } | Out-Null
$r = Get-Page "/file_trash"
Check "파일휴지통에 보관" ($null -ne $r -and $r.Content -match "테스트음악")

Write-Host ""
Write-Host ("PASS: " + $pass + " / FAIL: " + $fail)
