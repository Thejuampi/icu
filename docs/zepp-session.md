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

This prevents accidental repeated password exchanges. Reads use the current
credentials and return auth failures without starting a new login.

### November 19 incident

On November 19, 2026, between 12:18 and 12:21 EST, the reported test used
`/workspace/bin/icu`, compiled from `master` at `4982ee0`, to run
`zepp login --email EMAIL --reauthenticate`. The incident report records
Linux and a datacenter IP. Signing in from the CLI displaced the phone
session. Signing back in on the phone invalidated the CLI session. A second
explicit CLI login reused its saved UUID and displaced the phone again.
The repeated login guard worked. A changing UUID was unnecessary for
displacement in this incident.

The installed Windows `icu.exe` identified its build as a modified checkout
of `d737e971`. It and the #52 binary sent the same registration headers and
session fields to a loopback mock, including the Android `app_name` in the
second form. Both sent a UUID v4 and exchanged a mock US country code.
Only #52 stored the UUID in the synthetic config.

The earlier Zepp logins recorded in this investigation used `icu-mock`.
The real account check by the local agent was a read from
Intervals.icu with `icu wellness list`. The earlier statement that a
separate local Zepp login kept the phone signed in is withdrawn. The
available history provides no paired production comparison by client
identity or connection origin.

| Request detail | `icu` on #52 | `ha-zepp` web reference |
| --- | --- | --- |
| Credentials | `POST api-user-us2.zepp.com/v2/registrations/tokens`, encrypted form and 303 access code | `POST api-user.huami.com/registrations/<email>/tokens`, URL-encoded form over HTTPS and JSON access code |
| Registration fields | `client_id=HuaMi`, `country_code=US`, `region=us-west-2` | `client_id=HuaMi`, `app_name=com.huami.webapp`, `json_response=true`, optional country |
| Registration headers | Android app headers, `X-Hm-Ekv: 1` and Zepp 9.12.5 Android user agent | `App_name: com.huami.webapp`, form content type and `Accept-Language` |
| Session endpoint and grant | `POST api-mifit-us2.zepp.com/v2/client/login`, `grant_type=access_token` | `POST account.huami.com/v2/client/login`, `grant_type=access_token` |
| Session form | `app_name=com.huami.midong`, `device_model=android_phone`, installation UUID, Android `source`, version `9.12.5` | `app_name=com.huami.webapp`, `device_model=web`, `device_id=02:00:00:00:00:00`, version `4.3.0` |
| Session headers | Web app name, Firefox user agent, web origin and referrer | Form content type and the HTTP library's default headers |
| Session country | Returned `country_code`, default `US` | Returned `country_code`, requested country or region inference, with the area lookup and `US` as fallbacks |

`ha-zepp` reports that its web login coexists with the phone.
`amazfit-mcp` also uses a web form, with another account host, a generated
`web_` device ID and a separate app-token request. Both are community
clients. [ha-zepp implementation](https://github.com/yardeff/ha-zepp/blob/9142ca6fb10af15611a587894580cbdaaaf6def6/custom_components/zepp/api.py#L51-L186),
[ha-zepp README](https://github.com/yardeff/ha-zepp/blob/9142ca6fb10af15611a587894580cbdaaaf6def6/README.md),
[amazfit-mcp implementation](https://github.com/fjuliofontes/amazfit-mcp/blob/adf63107c437b8209aa66e761d1f278f0a201cdf/zepp_client.py#L76-L165).

`icu zepp login --web` runs the two-step exchange published by `ha-zepp`.
The web app name is explicit in both forms and both requests. The server's
country takes precedence over the returned region, with `US` as fallback.
The web flow uses the reference's fixed web device identifier. Successful
login saves the token bundle for the normal read commands. The CLI writes
the bundle only after both requests succeed. Either flow requires
`--reauthenticate` to replace an existing bundle.

### Production verification

With explicit authorization, test one `icu zepp login --web --email EMAIL`
from the datacenter against an isolated config while the phone is signed in.
The two requests are the web credential registration at `api-user.huami.com`
and the access-code exchange at `account.huami.com` listed above. Verify
that the phone still syncs a new sample, then read the corresponding sleep,
nightly HRV and band heart rate with the CLI. Verify the same conditions
after an authorized web reauthentication. A successful password exchange
can displace a server-side session, including a session held by the phone
or another web client. A separate local config only isolates the saved
credentials on disk. Phone sync and matching measurements on both passes
would support using that web flow in the tested environment.

For the Android SDK renewal flow, a separate check with explicit approval
and a valid session would use one `GET /v1/client/app_tokens` on its account
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
& "$scratch/icu.exe" zepp hrv --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp login --email fixture@example.invalid --zepp-base-url $mock
& "$scratch/icu.exe" zepp login --email fixture@example.invalid --password synthetic-only --zepp-base-url $mock --reauthenticate
```

The second login exits with a message about `--reauthenticate`. The explicit
login succeeds and uses the first login's UUID. The focused tests assert the
wire device ID, local persistence, failed reauthentication and migration from
configs without a UUID.

The mock serves the full CLI surface with synthetic data: profile, band data
(summary/steps/sleep/HR), all V2 wellness presets (readiness, hrv-sdnn,
hrv-rmssd, body-battery, hybridcharge, biocharge, health-summary, mood,
skin-temp, stress-minute, respiratory-rate, blood-pressure), sport-load, vo2,
weight, manual-data, second-heart-rate, spo2-windows, workouts and workout.
Only the `zepp login`/`zepp status` auth paths are not covered by the mock
(they exercise the auth flow itself).

Use a second config in the same loopback environment to verify the web path.

```powershell
$env:ICU_CONFIG_DIR = Join-Path $scratch 'web-config'
& "$scratch/icu.exe" zepp login --web --email fixture@example.invalid --password synthetic-only --zepp-base-url $mock
& "$scratch/icu.exe" zepp sleep --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp heart-rate --source band --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp readiness --oldest 2026-08-10 --newest 2026-08-10
& "$scratch/icu.exe" zepp login --web --reauthenticate --email fixture@example.invalid --password synthetic-only --zepp-base-url $mock
```

The mock verifies the explicit web identity on exchange. The commands read
synthetic sleep, per-minute heart rate, and nightly HRV in `extra.value.sleepHRV`.
The same reads work after explicit web reauthentication.

```bash
go test ./... -run 'TestZepp(Web|Login)' -count=1
```
