---
name: zepp-huami
description: "Zepp / Huami / Amazfit API integration in the icu CLI: auth flow, token management, health data retrieval (sleep, HR, SpO2, stress, PAI, workouts), regional hosts, DTOs, base64/binpack decoding pitfalls."
compatibility: opencode
metadata:
  cli: icu
  domain: zepp-api
  source: project-agent-skill
---

# Zepp / Huami / Amazfit Skill

## Authentication

Zepp has no OAuth or browser-based token flow. Auth is entirely API-based.

### Default login (web)

`icu zepp login --email E` uses the web client (`com.huami.webapp`). This is
the same flow the former `--web` flag selected. `--web` is still accepted as
a deprecated alias (one-line note on stderr) and does not change the request.

1. POST `api-user.huami.com/registrations/<email>/tokens` with
   `app_name=com.huami.webapp`, `client_id=HuaMi`, `json_response=true`
2. POST `account.huami.com/v2/client/login` with `grant_type=access_token`,
   `app_name=com.huami.webapp`, `third_name=huami`, `device_model=web`, and
   the fixed web `device_id`
3. Server returns JSON with `login_token`, `app_token`, `user_id`

### Unsafe Android login

`icu zepp login --android-unsafe` uses the Android app identity
(`com.huami.midong`). **It can sign out the phone Zepp app.** Do not use it
unless you have been asked to. No other command, flag, environment variable
or default reaches this path. `--web` and `--android-unsafe` cannot be combined.

1. POST encrypted credentials to `api-user-us2.zepp.com/v2/registrations/tokens`
   - Payload is URL-encoded, then AES-128-CBC encrypted (key=`xeNtBVqzDc6tuNTh`, iv=`MAAAYAAAAAAAAABg`), then PKCS#7 padded
   - Send **raw encrypted bytes** (NOT base64) as request body
   - Content-Type: `application/x-www-form-urlencoded`
   - Server responds with **303 redirect**; parse `access` and `country_code` from Location header
2. POST access token to `api-mifit-us2.zepp.com/v2/client/login`
   - Server returns JSON with `login_token`, `app_token`, `user_id`

### Interactive Password Input

Use `promptPasswordFlagOrInteractive(flagValue)` in cmd code. Falls back to `golang.org/x/term.ReadPassword` for masked terminal input when `--password` flag is omitted.

### Commands

- `icu zepp login --email E` — web login; saves tokens to `~/.icu/config.json`
- `icu zepp login --web --email E` — deprecated alias for the default web login
- `icu zepp login --android-unsafe --email E` — Android login; can sign out the phone app
- `icu zepp token --email E` — same path selection as login; outputs raw tokens as JSON and does NOT save to config
- Password is never stored; only `login_token`, `app_token`, `user_id`, `country_code` are saved

### Token Resolution Order

`--zepp-login-token` flag → `ZEPP_LOGIN_TOKEN` env → config file. Same for `ZEPP_APP_TOKEN`, `ZEPP_USER_ID`, `ZEPP_COUNTRY_CODE`.

## Regional Hosts

Country code from auth flow determines the data API host:

| Country Code  | Data Host                          |
|---------------|-------------------------------------|
| CN            | `https://api-mifit-cn.huami.com`   |
| DE,FR,IT,ES,GB,NL,PL,RU,TR,SE,NO,FI,DK | `https://api-mifit-de.huami.com` |
| default       | `https://api-mifit.huami.com`      |

Events endpoint is always `https://api-mifit.zepp.com`.

## Bug Patterns & Pitfalls

### 1. Gzip Decompression

**Never** manually set `Accept-Encoding: gzip` — Go's `net/http` Transport handles it automatically. Setting it manually overrides Go's transparent decompression and you'll get raw gzip bytes.

### 2. Auth Payload: Raw Bytes, Not Base64

The tokens endpoint accepts the AES-CBC encrypted payload as **raw bytes**, not base64-encoded. Base64 encoding causes a 400 error.

### 3. API Returns Strings for Numeric Fields

The events endpoints return `minStress`, `maxStress`, `avgStress`, `relaxProportion`, `odi`, `score`, `dailyPai`, `maxHr` etc. as **JSON strings** (e.g. `"minStress":"20"`). Use `flexInt` / `flexFloat` custom unmarshalers in `zepp_client.go` that accept both JSON number and JSON string.

### 4. Sentinels in Heart Rate Data

HR per-minute values 254 and 255 are sentinels ("no read" / "not required"). Map to bpm 0 but keep the minute timestamp.

### 5. Heart Rate Timestamps

`decodeBandDataHeartRate` must receive the date string to compute per-minute timestamps (midnight + index * 60s). Without a date, timestamps are 0.

### 6. Sleep Stage Minutes

Sleep `start`/`stop` in stage arrays are **minutes since midnight**, not epoch. Add date context from the parent day.

### 7. Workouts 500

The `/v1/sport/run/history.json` endpoint may require additional query params (`channel`, `country`, `cv`, `device`, `lang`, `timezone`, `v`) beyond the basic ones, depending on the region/user.

## Health Data Endpoints

| Data          | Path / Method                                       |
|---------------|-----------------------------------------------------|
| User profile  | `huami.health.getUserInfo.json` (on data host)      |
| Daily summary | `/v1/data/band_data.json` (base64-packed bins)      |
| Sleep         | Extracted from band_data summary (`slp` field)      |
| Heart rate    | Decoded from band_data `data_hr` (1 byte per minute) |
| Stress        | Events endpoint: `eventType=all_day_stress`         |
| SpO2          | Events endpoint: `eventType=blood_oxygen`           |
| PAI           | Events endpoint: `eventType=PaiHealthInfo`          |
| Workouts list | `/v1/sport/run/history.json`                        |
| Workout detail| `/v1/sport/run/detail.json` (delta-encoded series)  |

### Delta-Encoded Series (Workout Detail)

The `hr_split`, `pace_split`, `altitude_split`, `power_split`, `step_split` fields are base64-packed delta-encoded 2-byte signed shorts. First value is absolute; subsequent values are deltas. Sum cumulatively.

**Verification status:** the delta-encoded series is documented here (from the endpoint's documented contract), not verified against a real response — the live backend capture (2026-11-26) returned an empty `trackId`, so the detail response shape is unconfirmed. The newest-first `/v1/sport/{sport}/history.json` ordering is verified against that same live capture. The workouts CLI relies on newest-first + the `next` cursor to stop at the first item older than the range.

### Base64-Packed Summary (BandData)

The `summary` field is base64-encoded JSON with fields: `stp` (steps), `slp` (sleep), `goal`, `sn` (serial), `sync` (last sync epoch).

The `data_hr` field is base64-encoded binary with ONE unsigned byte per minute (1440 bytes = full day per-minute HR). Reading it as 2-byte shorts makes almost every value >= 254 (sentinel), so every point comes out as `{timestamp:0,bpm:0}`.

## Sleep Stage Modes

Sleep `stage[]` entries use minutes-since-midnight `start`/`stop`. `mode` is a raw integer: `4` = light, `5` = deep, `7` = awake, `8` = REM; any other value is `unknown`. The CLI exposes a derived `modeLabel` (`SleepStageModeLabel`) alongside the raw `mode`, but the raw value is always authoritative — new firmware may add modes and the label is best-effort, not part of the wire format. Cross-midnight stages wrap (stop < start). `startEpoch`/`endEpoch` anchor to the day's midnight for stages before the first wrap and to the next day for stages after it (so epochs are contiguous across the night). **Verification status: verified against a live backend capture (2026-11-26).** The synthetic `sleep_cross_midnight.json` fixture documents the minutes-since-midnight convention with a wrap, not a real band_data response; treat `modeLabel` as best-effort and the raw `mode` as authoritative.

## HRV Sources

Zepp nightly HRV is readiness `watch_score` `value.sleepHRV` (1-254 valid; 255 = not measured) — the value the app shows and the only value `wellness sync` writes. `hrv --metric sleephrv` (the default) reads it via `SleepHRVDays`. `hrv --metric rmssd` is the mean of the night's `HRVRMSSD` samples (an approximation, never written to intervals.icu); `hrv --metric sdnn` may be empty. `wellness sync` falls back to the sample mean as an `info:` line only, never written.

## V2 Event Presets: Object Values

The real V2 presets (readiness/watch_score, Charge/insight_data, Charge/real_data, DailyHealth/summary, Charge/stress_data, RespiratoryRate/real_data) carry `value` as an object — `readiness` keeps `sleepHRV`/`rdnsScore`/`hrvScore`, `insight_data` keeps `samples[]`, body battery (`Charge/real_data`) keeps `samples[]` with `total`/`mental`/`physical`/`status`/`jsonExtra`, health summary (`DailyHealth/summary`) keeps `samples[]` with `totalSteps`/`totalCalories`/`totalBurningDuration`/`stepGoal`/`calorieGoal`/`burningDurationGoal`/`hyroxCount`/`dateString`, stress-minute (`Charge/stress_data`) keeps `samples[]` with `s`/`u`/`stressInfo`/`minuteOfDay`, and respiratory rate keeps `measurements` (base64) + `timeZone`. The CLI does NOT invent a scalar from these: the record keeps `score: 0` (or `level: 0`/`value: 0`/`stress: 0`/`rate: 0`) and the full object under `extra.value`. Only `hrv_sdnn`/`HRVRMSSD` object payloads are decoded to a scalar (mean of valid samples). All of these are verified against the anonymized real-shape fixtures and a live backend capture (2026-11-26); mood and skin-temp (`Emotion/real_data`, `skinTemp/real_data`) returned no items and are unverified.

## Heart Rate App Source

`zepp heart-rate --source app` calls `/users/{id}/heartRate` and the endpoint returns ONE flat item per 7-day window (`type`, `generatedTime` in epoch seconds, `deviceSource`, `timeZone`, `heartRateData` base64, `deviceId`) — not per-day items with timestamp+value. The CLI stamps the item at `generatedTime * 1000`, keeps the base64 payload in `extra`, and reports heart rate as 0 (the payload format is not documented and is not decoded). Verified against the live backend capture (2026-11-26).

## Real-Account Verification (2026-11-26, read-only)

Verified against the real backend for this account: profile, summary (8 days), sleep (8 nights, stages/mode labels/epoch anchoring), heart-rate band (1440 points/day), heart-rate app (one flat item per 7-day window, `hr: 0`), HRV (rmssd = mean of samples, sleephrv = nightly value, sdnn empty), readiness (object payload; sleepHRV 43/33/35/35/36/39/36/41/44/42), body-battery (1440 samples/day), hybridcharge/biocharge (object payload), health-summary (e.g. 2360 steps/811 cal), stress-minute (object payload), respiratory-rate (base64 payload), spo2-windows (2026-11-24 ODI window), workouts (10 workouts, newest-first), generic events (body-battery preset).

Empty for this account (Sep 1 – Nov 26): mood, skin-temp, weight, second-heart-rate, blood-pressure watch, hrv sdnn.

Errors on the real backend (correct request form not found): stress/PAI/spo2 (legacy `/users/{id}/events` endpoint returns 500, single-day and multi-day), sport-load/vo2 (watch sport statistics endpoint returns 500, single-day and multi-day), manual-data/blood-pressure-user (HTTP 400, single-day and multi-day). Workout detail returns an empty `trackId` for every trackId checked. ECG is not available for US accounts (no ECG series exists in the CLI; the US backend does not support it).

## Second Heart Rate

`zepp second-heart-rate` returns the COS file index from `/users/me/fileInfo/events` (timestamp, date, URL, extra). The per-second blobs are NOT downloaded or parsed: the blob format is not documented anywhere in the repo or in prior plans, and it is not invented here. The library keeps the raw URL (for programmatic callers that fetch blobs themselves); the CLI redacts each URL (userinfo, query and fragment stripped) before printing because the URL may carry a temporary signature. Signed URLs are never safe to share — do not paste CLI output into shared places.

## Key Files

- `zepp_auth.go` — login flow, AES-CBC encryption, PKCS7 padding
- `zepp_client.go` — all data endpoint methods, decoders, `flexInt`/`flexFloat`
- `zepp_urls.go` — regional host mapping, URL builders
- `zepp.go` — DTOs, `SportTypeName` (24 sport types)
- `cmd/icu/cmd_zepp.go` — CLI commands
- `cmd/icu/read_password.go` — interactive masked password input (`golang.org/x/term`)
- `zepp_auth_test.go`, `zepp_client_test.go`, `zepp_test.go` — tests

## Sport Type Mapping

The Zepp sport type integers map as follows:
1=running, 2=walking, 3=cycling, 4=hiking, 5=swimming_pool, 6=open_water_swim,
7=elliptical, 8=rowing, 9=climbing, 10=treadmill, 11=strength_training,
12=yoga, 13=pilates, 14=indoor_cycling, 15=basketball, 16=football, 17=tennis,
18=badminton, 19=table_tennis, 20=golf, 21=skiing, 22=snowboarding,
23=jump_rope, 24=dance. Default: unknown.

## Troubleshooting

- **400 on login**: Check raw bytes vs base64 (bug #2). Check regional auth server.
- **500 on workout**: May need extra query params (bug #7). Run with custom test URLs.
- **Empty profile fields(userId/nickname/email)**: Real API may omit these for some accounts; gender/height/weight/birthday are usually present.
- **gzip decode errors**: Remove manual `Accept-Encoding: gzip` from request headers (bug #1).
- **Config loses tokens**: `config set` with missing flags can overwrite. Always use `zepp login` which preserves all 4 fields.
