package icu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	webZeppRegistrationBase     = "https://api-user.huami.com"
	webZeppAccountBase          = "https://account.huami.com"
	webZeppAppName              = "com.huami.webapp"
	zeppRegistrationRedirectURI = "https://s3-us-west-2.amazonaws.com/hm-registration/successsignin.html"
)

// ZeppWebLogin signs in with the community-documented Zepp web client.
// An empty baseURL selects the Huami passport and account hosts. A custom
// baseURL replaces both hosts, for example when testing with icu-mock.
func ZeppWebLogin(email, password, baseURL string) (*ZeppAuthResult, error) {
	registrationBase, accountBase := webZeppRegistrationBase, webZeppAccountBase
	if baseURL != "" {
		if err := validateZeppWebBaseURL(baseURL); err != nil {
			return nil, err
		}
		registrationBase = strings.TrimRight(baseURL, "/")
		accountBase = registrationBase
	}

	client := &http.Client{Timeout: zeppAuthTimeout, CheckRedirect: stopZeppAuthRedirect}
	registration, err := registerZeppWeb(client, registrationBase, email, password)
	if err != nil {
		return nil, fmt.Errorf("web registration: %w", err)
	}

	auth, err := exchangeZeppWebAccess(client, accountBase, registration)
	if err != nil {
		return nil, fmt.Errorf("web session: %w", err)
	}

	return auth, nil
}

func validateZeppWebBaseURL(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.ForceQuery || parsed.Fragment != "" {
		return errors.New("invalid Zepp web auth base URL")
	}

	host := strings.TrimSuffix(parsed.Hostname(), ".")
	if parsed.Scheme == "https" || (parsed.Scheme == "http" && (strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback())) {
		return nil
	}

	return errors.New("web authentication requires HTTPS or a loopback URL")
}

type zeppWebRegistration struct {
	Access      string `json:"access"`
	CountryCode string `json:"country_code"`
	Region      string `json:"region"`
}

func registerZeppWeb(client *http.Client, baseURL, email, password string) (zeppWebRegistration, error) {
	endpoint := baseURL + "/registrations/" + url.PathEscape(email) + "/tokens"
	form := url.Values{
		"password":      {password},
		"client_id":     {"HuaMi"},
		"token":         {"access"},
		"state":         {"REDIRECTION"},
		"redirect_uri":  {zeppRegistrationRedirectURI},
		"json_response": {"true"},
		"app_name":      {webZeppAppName},
	}

	resp, err := postZeppWebForm(client, endpoint, form)
	if err != nil {
		return zeppWebRegistration{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return zeppWebRegistration{}, fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var registration zeppWebRegistration
	if err := json.NewDecoder(resp.Body).Decode(&registration); err != nil {
		return zeppWebRegistration{}, errors.New("invalid registration JSON")
	}
	if registration.Access == "" {
		return zeppWebRegistration{}, errors.New("missing access code in registration response")
	}

	return registration, nil
}

func (r zeppWebRegistration) country() string {
	if code := strings.ToUpper(strings.TrimSpace(r.CountryCode)); code != "" {
		return code
	}

	region, _, _ := strings.Cut(strings.ToLower(r.Region), "-")
	switch region {
	case "cn":
		return "CN"
	case "eu", "de":
		return "DE"
	case "ru":
		return "RU"
	case "sg":
		return "SG"
	default:
		return "US"
	}
}

func exchangeZeppWebAccess(client *http.Client, baseURL string, registration zeppWebRegistration) (*ZeppAuthResult, error) {
	country := registration.country()
	form := url.Values{
		"country_code":       {country},
		"app_name":           {webZeppAppName},
		"third_name":         {"huami"},
		"grant_type":         {"access_token"},
		"code":               {registration.Access},
		"device_id":          {"02:00:00:00:00:00"},
		"device_model":       {"web"},
		"app_version":        {"4.3.0"},
		"allow_registration": {"false"},
		"dn":                 {"account.huami.com,api-user.huami.com,api-mifit.huami.com,auth.huami.com,api-open.huami.com"},
	}

	resp, err := postZeppWebForm(client, baseURL+"/v2/client/login", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return decodeZeppLogin(resp, country)
}

func postZeppWebForm(client *http.Client, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode())) //nolint:noctx // authentication is bounded by the client timeout
	if err != nil {
		return nil, errors.New("invalid request URL")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("App_name", webZeppAppName)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		// A transport error can include the registration URL with the email.
		return nil, errors.New("web authentication request failed")
	}

	return resp, nil
}
