# Zepp Android session analysis

The sample was Zepp Android `9.12.5-play` (`151689`), the build identified by
the CLI's existing login requests. The base APK came from a public mirror.
`apksigner verify` matched APKMirror's published signer SHA-256 fingerprint
`b145a7cb364e812592115efcb1e3a8bc7b5645c570493cb90a075c3593dc2acb`.
The base APK SHA-256 was
`2faf3a27af3b5b83ed2cf87000ddc878b6e2d2ae01d009d33461886410cc332b`.
The analysis used JADX 1.5.3 and Android manifest and resource dumps offline.
[Download listing](https://en.mobile-games-box.com/download/split/zepp/com.huami.watch.hmwatchmanager/23253783/2628584),
[APKMirror certificate](https://www.apkmirror.com/apk/zepp-inc/amazfit-watch/zepp-9-12-5-play-release/zepp-9-12-5-play-android-apk-download/).

## Password login

1. `com.huami.passport.api.MailService#login` sends
   `POST /v2/registrations/tokens` through `HttpDnsCache#getIdUrl` (default
   `api-user.zepp.com`). The form includes `emailOrPhone`, `password`,
   `state=REDIRECTION`, `client_id=HuaMi`, `redirect_uri`, `region`
   (default `us-west-2`), `country_code`, and repeated `token=access` and
   `token=refresh` fields. The client ID is an SDK default. The redirect
   handler reads the 303 `Location` for `access`, `refresh`, `expiration`,
   and region and country metadata, then stores the result in
   `com.huami.passport.user.Keeper#setLoginInfo`.
2. `com.huami.passport.user.AccountService` passes `access` to
   `com.huami.passport.v2.impl.AccountApiV2#loginV2`. It calls
   `com.huami.passport.net.WebAPI#loginV2`, which sends
   `POST /v2/client/login` through `HttpDnsCache#getAccountUrlIndex`
   (production index `api-mifit.zepp.com`). The form includes
   `grant_type=access_token`, `code`, `third_name=huami` for email,
   `app_name`, `device_id`, `device_model`, `app_version`, `country_code`,
   `dn`, `lang`, `source`, and `allow_registration`. The
   `source` format comes from `AppInfo#buildParams` and is
   `package:versionName:versionCode`.

`Utils#getAppInfo` reads `app_name` from the manifest's `hm_app_id`,
`com.huami.midong` in this APK. `EntityRequest#addSystemParams` also sets it
as a header. `Utils#getDeviceInfo` uses `hm_device_type=android_phone` and
`com.huami.passport.Keeper#readDeviceUuid`. The latter creates a random UUID
once and saves it as `device_uuid` in Android shared preferences.
The Go CLI uses `api-user-us2.zepp.com` and `api-mifit-us2.zepp.com`.
Its second request uses `com.huami.midong` in the form and
`com.huami.webapp` in the app-name headers.

## Token lifetime

- `LoginToken` and `TokenInfo` decode `token_info.login_token`,
  `app_token`, `user_id`, `ttl`, and `app_ttl`.
  `AccountApi#refreshLoginToken` timestamps the response and saves it
  through `com.huami.passport.Keeper#saveToken`.
- `AccountApi#getInnerToken` checks the saved token ages. A forced refresh
  or a login-token age over roughly three quarters of `ttl` triggers
  `AccountApi#syncLoginToken`. It uses `GET /v1/client/renew_login_token`
  and stores the returned login token and lifetime. The app token can be
  reused through half of `app_ttl`. After that, or on a forced refresh,
  `WebAPI#getAppToken` calls `GET /v1/client/app_tokens` and the caller
  persists the new app token. Both GETs come from `WebAPI#syncToken`, use
  the existing `login_token`, and resolve the account host from the cached
  domain metadata with the production index as fallback. `WebAPI` also
  contains a callback overload that sends a POST to the renewal endpoint.
- The email-service `refresh` credential belongs to the earlier access
  exchange. `LoginServiceAPI#refreshToken` has a separate `POST /access`
  flow for that credential. The SDK also contains `POST /v1/client/re_login`
  in `WebAPI#relogin`, a separate action that sends the login token and
  device identity when the app requests sign-in again.

## Session displacement and other clients

`mx3.m$a#i` reads `login_time` from a conflict response and passes it to
`hz.e0#x` through `#g`. `kz.g#m` also routes account error `0108` to
`hz.e0#x`. `hz.e0#y` launches the conflict screen, where
`MutexLoginActivity#onCreate` and `#f` display the other-device login
message. Whether Zepp keys revocation by account, `app_name`, `device_id`,
or a combination remains a server-side question. A 401 alone leaves token
expiry and revocation indistinguishable.

- Zepp Life (formerly Mi Fit) is a separate Android app with package
  `com.xiaomi.hm.health`. Its package ID gives no evidence of shared
  access to the same cloud data or independent sessions on the same account.
  [Zepp Life on Google Play](https://play.google.com/store/apps/details?id=com.xiaomi.hm.health).
- The web authorization flow documented by Zepp is partner OAuth.
  Its health data scopes require application registration, approved permissions
  and enterprise verification. The existence of a web app header in the CLI
  gives no evidence of a separate consumer session.
  [Zepp partner onboarding](https://devopen.zepp.com/docs/getting-started/partner-onboarding),
  [OAuth authorization flow](https://devopen.zepp.com/docs/authentication/authorization-flow).
- Zepp OS mini programs and watch widgets run in the watch environment.
  Their companion service runs inside the phone's Zepp app. The documented
  architecture gives no evidence of a separate authenticated health history
  client. [Zepp OS architecture](https://docs.zepp.com/docs/guides/architecture/arc/).

## CLI change and remaining verification

`icu zepp login` saves a CLI-generated UUID as `zeppDeviceId` on the first
successful login and reuses it for that config directory. With locally saved
credentials, a second password login requires `--reauthenticate` and the
default exits before the password prompt or any auth request. Old config files
can keep using their saved tokens and obtain a UUID on their next explicit
successful login. Local logout clears the credentials and retains the UUID.

This prevents accidental repeated password exchanges. Phone coexistence after
an actual password login is unverified. Reads use the current credentials and
return auth failures without starting a new login.

The first production check to consider, with explicit approval and an already
valid CLI session, is one `GET /v1/client/app_tokens` on the session's account
host (fallback `https://api-mifit.zepp.com`). `WebAPI#syncToken` sends
`login_token`, `os_version`, `source`, and `dn` in the query and the
manifest app name in the `app_name` header. Check whether the returned app
token can read data and whether the phone stays signed in. This can rotate or
invalidate credentials. The token in the query also risks exposure through
request logging.
An HTTP 401 ends the check. The distinct `GET /v1/client/renew_login_token`
has the same request fields and additionally risks rotating the login token.
A controlled comparison of two password exchanges that vary only
`device_id` would require a disposable account and could sign out either
session. Production renewal remains pending verification.

## Verify with icu-mock

In one PowerShell terminal from the repository root, start the mock with one
synthetic fixture. The Zepp data is generated by the mock.

```powershell
$fixtures = New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N')))
Set-Content -LiteralPath (Join-Path $fixtures.FullName 'synthetic.json') -Value '{}' -Encoding ascii
go run ./tools/mockserver -host 127.0.0.1 -port 0 -fixtures $fixtures.FullName -redirect-sink
```

In a second PowerShell terminal from the repository root, set `$mock` to the
URL on the `icu-mock listening on` line. This terminal uses a temporary home
and only loopback endpoints.

```powershell
$mock = 'http://127.0.0.1:PORT'
if ($mock -notmatch '^http://127\.0\.0\.1:[1-9][0-9]*$') { throw 'Set the loopback mock URL first' }
$scratch = (New-Item -ItemType Directory -Path (Join-Path ([IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString('N')))).FullName
go build -o "$scratch/icu.exe" ./cmd/icu
if ($LASTEXITCODE -ne 0) { throw 'CLI build failed' }
Get-ChildItem Env: | Where-Object Name -Match '^(INTERVALS_ICU_|ZEPP_|ICU_)' |
    ForEach-Object { Remove-Item -LiteralPath ('Env:' + $_.Name) }
$env:HOME = $env:USERPROFILE = $scratch
$env:ICU_CONFIG_DIR = Join-Path $scratch 'config'
$env:ICU_BASE_URL = $env:ZEPP_BASE_URL = $env:ZEPP_EVENTS_URL = $mock

& "$scratch/icu.exe" zepp login --email fixture@example.invalid --password synthetic-only --zepp-base-url $mock
& "$scratch/icu.exe" zepp sleep --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp heart-rate --source band --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp hrv --metric rmssd --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp login --email fixture@example.invalid --zepp-base-url $mock
& "$scratch/icu.exe" zepp login --email fixture@example.invalid --password synthetic-only --zepp-base-url $mock --reauthenticate
```

The second login exits with a message about `--reauthenticate`. The explicit
login succeeds and uses the first login's UUID. The focused tests assert the
wire device ID, local persistence, failed reauthentication and migration from
configs without a UUID.

```bash
go test ./cmd/icu -run 'TestZeppLogin' -count=1
```
