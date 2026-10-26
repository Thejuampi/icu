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

Zepp has no OAuth or browser-based token flow. Auth is entirely API-based:

### Login Flow (email/password → tokens)

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

- `icu zepp token --email E` — outputs raw tokens as JSON, does NOT save to config
- `icu zepp login --email E` — logs in and saves tokens to `~/.icu/config.json`
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

HR per-minute values 254 and 255 are sentinels ("no read" / "not required"). Map to 0.

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
| Heart rate    | Decoded from band_data `data_hr` (2-byte LE shorts) |
| Stress        | Events endpoint: `eventType=all_day_stress`         |
| SpO2          | Events endpoint: `eventType=blood_oxygen`           |
| PAI           | Events endpoint: `eventType=PaiHealthInfo`          |
| Workouts list | `/v1/sport/run/history.json`                        |
| Workout detail| `/v1/sport/run/detail.json` (delta-encoded series)  |

### Delta-Encoded Series (Workout Detail)

The `hr_split`, `pace_split`, `altitude_split`, `power_split`, `step_split` fields are base64-packed delta-encoded 2-byte signed shorts. First value is absolute; subsequent values are deltas. Sum cumulatively.

### Base64-Packed Summary (BandData)

The `summary` field is base64-encoded JSON with fields: `stp` (steps), `slp` (sleep), `goal`, `sn` (serial), `sync` (last sync epoch).

The `data_hr` field is base64-encoded binary of 2-byte little-endian unsigned shorts (1440 entries = full day per-minute HR).

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
