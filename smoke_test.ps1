$base = 'http://127.0.0.1:18099'

# ===== TOTP 生成 + 登录（同一进程内完成，避免跨进程会话序列化问题）=====
$alpha = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
$secret = 'JBSWY3DPEHPK3PXP'
$bits = ''
foreach ($c in $secret.ToCharArray()) { $bits += [Convert]::ToString($alpha.IndexOf($c), 2).PadLeft(5, '0') }
$bytes = New-Object System.Collections.Generic.List[byte]
for ($i = 0; $i -lt $bits.Length; $i += 8) { $bytes.Add([Convert]::ToByte($bits.Substring($i, 8), 2)) }
$counter = [UInt64][Math]::Floor([DateTimeOffset]::UtcNow.ToUnixTimeSeconds() / 30)
$cb = [BitConverter]::GetBytes([System.Net.IPAddress]::HostToNetworkOrder([int64]$counter))
$hmac = New-Object System.Security.Cryptography.HMACSHA1
$hmac.Key = $bytes.ToArray()
$sum = $hmac.ComputeHash($cb)
$off = $sum[$sum.Length - 1] -band 0x0f
$num = ([uint32]$sum[$off] -shl 24) -bor ([uint32]$sum[$off+1] -shl 16) -bor ([uint32]$sum[$off+2] -shl 8) -bor [uint32]$sum[$off+3]
$code = (($num -band 0x7fffffff) % 1000000).ToString('D6')
Write-Host "TOTP=$code"

# PS 5.1 对字符串 Body 默认按 ANSI 编码发送，中文会变成 ?，必须显式转 UTF-8 字节
function SendJson($method, $uri, $obj) {
    $json = $obj | ConvertTo-Json
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
    Invoke-RestMethod -Uri "$base$uri" -Method $method -Body $bytes -ContentType 'application/json; charset=utf-8' -WebSession $session
}

$loginJson = [System.Text.Encoding]::UTF8.GetBytes(($loginBody = @{ username = 'admin'; password = 'TestPass123'; totp = $code } | ConvertTo-Json))
$login = Invoke-RestMethod -Uri "$base/api/v1/admin/login" -Method Post -Body $loginJson -ContentType 'application/json; charset=utf-8' -SessionVariable loginSession
if (-not $login.success) { Write-Host 'LOGIN FAILED, ABORT'; exit 1 }
$session = $loginSession

function Post($uri, $obj) { SendJson 'Post' $uri $obj }
function Put($uri, $obj) { SendJson 'Put' $uri $obj }
function Get2($uri) { Invoke-RestMethod -Uri "$base$uri" -Method Get -WebSession $session }

Write-Host '== 1. site-config (initial, should be empty) =='
(Get2 '/api/v1/site-config') | ConvertTo-Json -Compress

Write-Host '== 2. save inbox (valid email) =='
(Put '/api/v1/admin/settings' @{ inbox_email = 'Admin.Report@Example.com ' }) | ConvertTo-Json -Compress

Write-Host '== 3. site-config (after save) =='
(Get2 '/api/v1/site-config') | ConvertTo-Json -Compress

Write-Host '== 4. save inbox (invalid email -> expect error) =='
try { Put '/api/v1/admin/settings' @{ inbox_email = 'not-an-email' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 5. add blacklist (link without protocol auto-https, CN/EN comma people) =='
(Post '/api/v1/admin/add' @{ email = 'Bad.User@QQ.com'; ban_reason = '**诈骗** 与刷屏'; event_link = 'tieba.baidu.com/p/1234567890'; event_related_people = '张三，Li Si、Wang Wu'; banned_at = '' }) | ConvertTo-Json -Compress

Write-Host '== 5a. add with non-tieba domain (expect error) =='
try { Post '/api/v1/admin/add' @{ email = 'hack@qq.com'; ban_reason = 'x'; event_link = 'https://example.com/evil'; event_related_people = ''; banned_at = '' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 5b. add with http tieba link (expect error, must be https) =='
try { Post '/api/v1/admin/add' @{ email = 'hack2@qq.com'; ban_reason = 'x'; event_link = 'http://tieba.baidu.com/p/1'; event_related_people = ''; banned_at = '' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 6. public check (related_people_list + normalized link) =='
(Invoke-RestMethod -Uri "$base/api/v1/check?email=bad.user@qq.com") | ConvertTo-Json -Compress

Write-Host '== 7. list (find id) =='
$list = Get2 '/api/v1/admin/list?page=1&page_size=10'
$list | ConvertTo-Json -Depth 5 -Compress
$rec = $list.list | Where-Object { $_.email -eq 'bad.user@qq.com' }
Write-Host "RECORD_ID=$($rec.id)"

Write-Host '== 8. update record (change reason/link/people/email) =='
(Put "/api/v1/admin/update/$($rec.id)" @{ email = 'bad.user2@qq.com'; ban_reason = '更新后的原因'; event_link = 'https://tieba.baidu.com/p/987654321'; event_related_people = '赵六, Qian Qi'; banned_at = '2026-01-02T15:04' }) | ConvertTo-Json -Compress

Write-Host '== 9. check updated =='
(Invoke-RestMethod -Uri "$base/api/v1/check?email=bad.user2@qq.com") | ConvertTo-Json -Compress

Write-Host '== 10. update to duplicate email (expect conflict) =='
(Post '/api/v1/admin/add' @{ email = 'other@qq.com'; ban_reason = 'x'; event_link = 'https://tieba.baidu.com/p/111'; event_related_people = ''; banned_at = '' }) | Out-Null
try { Put "/api/v1/admin/update/$($rec.id)" @{ email = 'other@qq.com'; ban_reason = 'x'; event_link = ''; event_related_people = ''; banned_at = '' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 11. update with overlong reason (expect error) =='
$long = 'a' * 501
try { Put "/api/v1/admin/update/$($rec.id)" @{ email = 'bad.user2@qq.com'; ban_reason = $long; event_link = ''; event_related_people = ''; banned_at = '' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 12. change password with wrong old password (expect error) =='
try { Post '/api/v1/admin/password' @{ old_password = 'WrongOld9'; new_password = 'NewPass123' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 13. change password with weak new password (expect error) =='
try { Post '/api/v1/admin/password' @{ old_password = 'TestPass123'; new_password = 'weak' } | ConvertTo-Json -Compress } catch { Write-Host "EXPECTED ERROR: $($_.ErrorDetails.Message)" }

Write-Host '== 14. change password correctly =='
(Post '/api/v1/admin/password' @{ old_password = 'TestPass123'; new_password = 'NewPass456' }) | ConvertTo-Json -Compress

Write-Host '== 15. audit logs (actions) =='
(Get2 '/api/v1/admin/audit-logs?page=1&page_size=20').list | ForEach-Object { Write-Host "$($_.created_at) | $($_.user) | $($_.action) | $($_.target)" }

Write-Host '== 16. clear inbox (hide button) =='
(Put '/api/v1/admin/settings' @{ inbox_email = '' }) | ConvertTo-Json -Compress
(Get2 '/api/v1/site-config') | ConvertTo-Json -Compress
