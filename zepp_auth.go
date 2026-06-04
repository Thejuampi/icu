package icu

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	zeppEncryptionKey = "xeNtBVqzDc6tuNTh"
	zeppEncryptionIV  = "MAAAYAAAAAAAAABg"
	zeppTokensURL     = "https://api-user-us2.zepp.com/v2/registrations/tokens"
	zeppLoginURL      = "https://api-mifit-us2.zepp.com/v2/client/login"
	zeppDevicesURL    = "https://api-mifit.zepp.com/users/%s/devices"
)

type ZeppAuthResult struct {
	LoginToken  string
	AppToken    string
	UserID      string
	CountryCode string
}

func ZeppLogin(email, password string) (*ZeppAuthResult, error) {
	return ZeppLoginWithURLs(zeppTokensURL, zeppLoginURL, email, password)
}

// ZeppLoginWithURLs is the testable form of ZeppLogin. It accepts explicit URLs
// for the two auth endpoints so tests can point at httptest servers.
func ZeppLoginWithURLs(tokensURL, loginURL, email, password string) (*ZeppAuthResult, error) {
	accessToken, countryCode, err := getAccessTokenWithURL(tokensURL, email, password)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	result, err := performLoginWithURL(loginURL, accessToken, countryCode)
	if err != nil {
		return nil, fmt.Errorf("perform login: %w", err)
	}

	return result, nil
}

func getAccessToken(email, password string) (string, string, error) {
	return getAccessTokenWithURL(zeppTokensURL, email, password)
}

func getAccessTokenWithURL(tokensURL, email, password string) (string, string, error) {
	payload := url.Values{
		"emailOrPhone": {email},
		"state":        {"REDIRECTION"},
		"client_id":    {"HuaMi"},
		"password":     {password},
		"redirect_uri": {"https://s3-us-west-2.amazonaws.com/hm-registration/successsignin.html"},
		"region":       {"us-west-2"},
		"token":        {"access", "refresh"},
		"country_code": {"US"},
	}

	encodedPayload := payload.Encode()
	encrypted, err := encryptPayload([]byte(encodedPayload))
	if err != nil {
		return "", "", fmt.Errorf("encrypt payload: %w", err)
	}

	req, err := http.NewRequest("POST", tokensURL, bytes.NewReader(encrypted))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("app_name", "com.huami.midong")
	req.Header.Set("appname", "com.huami.midong")
	req.Header.Set("cv", "151689_9.12.5")
	req.Header.Set("v", "2.0")
	req.Header.Set("appplatform", "android_phone")
	req.Header.Set("vb", "202509151347")
	req.Header.Set("vn", "9.12.5")
	req.Header.Set("user-agent", "Zepp/9.12.5 (Pixel 4; Android 12; Density/2.75)")
	req.Header.Set("x-hm-ekv", "1")
	req.Header.Set("content-type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("accept-encoding", "gzip")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 303 {
		return "", "", fmt.Errorf("expected status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", "", errors.New("no redirect location in response")
	}

	parsedURL, err := url.Parse(location)
	if err != nil {
		return "", "", fmt.Errorf("parse redirect URL: %w", err)
	}

	query := parsedURL.Query()
	accessToken := query.Get("access")
	if accessToken == "" {
		return "", "", errors.New("no access token in redirect URL")
	}

	countryCode := query.Get("country_code")
	if countryCode == "" {
		countryCode = "US"
	}

	return accessToken, countryCode, nil
}

func performLogin(accessToken, countryCode string) (*ZeppAuthResult, error) {
	return performLoginWithURL(zeppLoginURL, accessToken, countryCode)
}

func performLoginWithURL(loginURL, accessToken, countryCode string) (*ZeppAuthResult, error) {
	deviceID := generateUUID()

	payload := url.Values{
		"code":               {accessToken},
		"device_id":          {deviceID},
		"device_model":       {"android_phone"},
		"app_version":        {"9.12.5"},
		"dn":                 {"api-mifit.zepp.com,api-user.zepp.com,api-mifit.zepp.com,api-watch.zepp.com,app-analytics.zepp.com,auth.zepp.com,api-analytics.zepp.com"},
		"third_name":         {"huami"},
		"source":             {"com.huami.watch.hmwatchmanager:9.12.5:151689"},
		"app_name":           {"com.huami.midong"},
		"country_code":       {countryCode},
		"grant_type":         {"access_token"},
		"allow_registration": {"false"},
		"lang":               {"en"},
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(payload.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("app_name", "com.huami.webapp")
	req.Header.Set("appname", "com.huami.webapp")
	req.Header.Set("origin", "https://user.zepp.com")
	req.Header.Set("referer", "https://user.zepp.com/")
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0")
	req.Header.Set("content-type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("accept-language", "en-US,en;q=0.5")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var result struct {
		TokenInfo struct {
			LoginToken string `json:"login_token"`
			AppToken   string `json:"app_token"`
			UserID     string `json:"user_id"`
		} `json:"token_info"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if result.TokenInfo.LoginToken == "" || result.TokenInfo.AppToken == "" {
		return nil, errors.New("missing login_token or app_token in response")
	}

	if result.TokenInfo.UserID == "" {
		return nil, errors.New("missing user_id in response")
	}

	return &ZeppAuthResult{
		LoginToken:  result.TokenInfo.LoginToken,
		AppToken:    result.TokenInfo.AppToken,
		UserID:      result.TokenInfo.UserID,
		CountryCode: countryCode,
	}, nil
}

func encryptPayload(data []byte) ([]byte, error) {
	key := []byte(zeppEncryptionKey)
	iv := []byte(zeppEncryptionIV)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	padded := pkcs7Pad(data, aes.BlockSize)
	ciphertext := make([]byte, len(padded))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	return append(data, padtext...)
}

// PKCS7PadForTest exposes pkcs7Pad for tests. The auth flow uses PKCS#7
// padding internally; the test verifies the padding scheme is correct.
func PKCS7PadForTest(data []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 || blockSize > 255 {
		return nil, errors.New("invalid block size")
	}

	return pkcs7Pad(data, blockSize), nil
}

func generateUUID() string {
	uuid := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, uuid); err != nil {
		panic(err)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}
