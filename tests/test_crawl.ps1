param(
    [string]$BaseUrl = "http://127.0.0.1:3010",
    [string]$UserId = "admin",
    [string]$UserPw = "admin1234!"
)

$ErrorActionPreference = "Continue"
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-WebRequest -Uri ($BaseUrl + "/login") -Method Post -Body @{ id = $UserId; password = $UserPw } -WebSession $session -UseBasicParsing | Out-Null

$paths = @(
    "/w/FrontPage",
    "/w/%EC%9C%84%ED%82%A4%EC%9A%B4%EC%98%81:%EA%B3%B5%EC%A7%80%EC%82%AC%ED%95%AD",
    "/w/%EC%82%AC%EC%9A%A9%EC%9E%90:admin",
    "/w/%EC%95%84%EC%9D%B4%ED%94%BC%EC%82%AC%EC%9A%A9%EC%9E%90:127.0.0.1",
    "/edit/FrontPage",
    "/edit/FrontPage?section=1",
    "/history/FrontPage",
    "/acl/FrontPage",
    "/move/FrontPage",
    "/delete/FrontPage",
    "/topic/FrontPage",
    "/xref/FrontPage",
    "/raw/FrontPage",
    "/namespace",
    "/namespace/%EB%AC%B8%EC%84%9C/1",
    "/namespace/%EC%82%AC%EC%9A%A9%EC%9E%90/1",
    "/namespace_acl/%EC%9C%84%ED%82%A4%EC%9A%B4%EC%98%81",
    "/trash",
    "/file_trash",
    "/auth/ban",
    "/auth/ban/multiple",
    "/auth/warn",
    "/auth/user",
    "/auth/user/admin",
    "/auth/give",
    "/auth/list",
    "/auth/list/add",
    "/warn",
    "/recent_block",
    "/recent_block/ongoing/1",
    "/recent_changes",
    "/recent_discuss",
    "/manager",
    "/setting",
    "/setting/main",
    "/bbs/main",
    "/bbs/block",
    "/bbs/make",
    "/upload",
    "/list/document/all/1",
    "/list/file",
    "/list/image",
    "/user",
    "/change",
    "/other",
    "/vote",
    "/search_page/1/%ED%85%8C%EC%8A%A4%ED%8A%B8",
    "/random"
)

$fail = 0
$ok = 0
foreach ($path in $paths) {
    try {
        $r = Invoke-WebRequest -Uri ($BaseUrl + $path) -WebSession $session -UseBasicParsing -TimeoutSec 30
        if ($r.Content -match "Internal Server Error" -or $r.Content -match "panic") {
            Write-Host ("  FAIL " + $path + " (server error in body)") -ForegroundColor Red
            $fail++
        } else {
            $ok++
        }
    } catch {
        $status = ""
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        if ($status -eq 404 -or $status -eq 401 -or $status -eq 403) {
            $ok++
        } else {
            Write-Host ("  FAIL " + $path + " => " + $_.Exception.Message) -ForegroundColor Red
            $fail++
        }
    }
}

Write-Host ("crawl OK: " + $ok + " / FAIL: " + $fail)
