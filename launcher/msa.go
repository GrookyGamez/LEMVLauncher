package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MSAAccount is a signed-in Minecraft account (Microsoft auth chain).
type MSAAccount struct {
	Name         string `json:"name"`         // Minecraft profile name
	UUID         string `json:"uuid"`         // Minecraft profile id (no dashes)
	AccessToken  string `json:"accessToken"`  // Minecraft services token (short-lived)
	RefreshToken string `json:"refreshToken"` // MSA refresh token (long-lived)
	ExpiresAt    int64  `json:"expiresAt"`    // unix seconds when AccessToken dies
}

func (a *MSAAccount) Valid() bool {
	return a != nil && a.AccessToken != "" && time.Now().Unix() < a.ExpiresAt-60
}

// Endpoints, overridable for tests (LEMV_MSA_* env vars).
func msaBase() string   { return envOr("LEMV_MSA_URL", "https://login.microsoftonline.com/consumers") }
func xblBase() string   { return envOr("LEMV_XBL_URL", "https://user.auth.xboxlive.com") }
func xstsBase() string  { return envOr("LEMV_XSTS_URL", "https://xsts.auth.xboxlive.com") }
func mcsvcBase() string { return envOr("LEMV_MCSVC_URL", "https://api.minecraftservices.com") }

const msaScope = "XboxLive.signin offline_access"

var msaHTTP = &http.Client{Timeout: 30 * time.Second}

// DeviceCode is step one: the user enters UserCode at VerificationURI.
type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// StartDeviceLogin asks Microsoft for a device code.
func StartDeviceLogin(clientID string) (*DeviceCode, error) {
	form := url.Values{"client_id": {clientID}, "scope": {msaScope}}
	var dc DeviceCode
	if err := postForm(msaBase()+"/oauth2/v2.0/devicecode", form, &dc); err != nil {
		return nil, err
	}
	if dc.DeviceCode == "" {
		return nil, fmt.Errorf("Microsoft didn't return a device code")
	}
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	return &dc, nil
}

// PollDeviceLogin waits for the user to approve, then runs the whole chain
// through Xbox Live and Minecraft services. It blocks until success, denial
// or expiry; progress strings go to p.
func PollDeviceLogin(clientID string, dc *DeviceCode, p func(string)) (*MSAAccount, error) {
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {clientID},
		"device_code": {dc.DeviceCode},
	}
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(dc.Interval) * time.Second)
		var tok struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			Error        string `json:"error"`
		}
		if err := postForm(msaBase()+"/oauth2/v2.0/token", form, &tok); err != nil {
			return nil, err
		}
		switch tok.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			dc.Interval += 2
			continue
		case "":
			return finishLogin(tok.AccessToken, tok.RefreshToken, p)
		default:
			return nil, fmt.Errorf("Microsoft sign-in failed: %s", tok.Error)
		}
	}
	return nil, fmt.Errorf("the sign-in code expired — try again")
}

// RefreshLogin renews an account from its refresh token.
func RefreshLogin(clientID string, acc *MSAAccount, p func(string)) (*MSAAccount, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {acc.RefreshToken},
		"scope":         {msaScope},
	}
	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
	}
	if err := postForm(msaBase()+"/oauth2/v2.0/token", form, &tok); err != nil {
		return nil, err
	}
	if tok.Error != "" || tok.AccessToken == "" {
		return nil, fmt.Errorf("couldn't refresh the Microsoft sign-in (%s) — sign in again", tok.Error)
	}
	return finishLogin(tok.AccessToken, tok.RefreshToken, p)
}

// finishLogin: MSA token -> XBL -> XSTS -> Minecraft services -> profile.
func finishLogin(msaToken, refresh string, p func(string)) (*MSAAccount, error) {
	if p == nil {
		p = func(string) {}
	}
	p("Talking to Xbox Live…")
	xblTok, hash, err := xblAuth(msaToken)
	if err != nil {
		return nil, err
	}
	p("Getting the Xbox security token…")
	xstsTok, err := xstsAuth(xblTok)
	if err != nil {
		return nil, err
	}
	p("Signing in to Minecraft…")
	mcTok, expires, err := mcLogin(hash, xstsTok)
	if err != nil {
		return nil, err
	}
	p("Fetching your profile…")
	name, uuid, err := mcProfile(mcTok)
	if err != nil {
		return nil, err
	}
	return &MSAAccount{
		Name: name, UUID: uuid,
		AccessToken: mcTok, RefreshToken: refresh,
		ExpiresAt: time.Now().Unix() + expires,
	}, nil
}

func xblAuth(msaToken string) (token, userHash string, err error) {
	body := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com",
			"RpsTicket": "d=" + msaToken,
		},
		"RelyingParty": "http://auth.xboxlive.com", "TokenType": "JWT",
	}
	var out struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			XUI []struct {
				UHS string `json:"uhs"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	if err := postJSON(xblBase()+"/user/authenticate", body, &out); err != nil {
		return "", "", fmt.Errorf("Xbox Live sign-in failed: %v", err)
	}
	if out.Token == "" || len(out.DisplayClaims.XUI) == 0 {
		return "", "", fmt.Errorf("Xbox Live returned an empty token")
	}
	return out.Token, out.DisplayClaims.XUI[0].UHS, nil
}

func xstsAuth(xblToken string) (string, error) {
	body := map[string]any{
		"Properties": map[string]any{
			"SandboxId": "RETAIL", "UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/", "TokenType": "JWT",
	}
	var out struct {
		Token string `json:"Token"`
		XErr  int64  `json:"XErr"`
	}
	if err := postJSON(xstsBase()+"/xsts/authorize", body, &out); err != nil {
		return "", fmt.Errorf("Xbox security token failed: %v", err)
	}
	switch out.XErr {
	case 0:
	case 2148916233:
		return "", fmt.Errorf("this Microsoft account has no Xbox profile — sign in at xbox.com once, then retry")
	case 2148916238:
		return "", fmt.Errorf("this is a child account — it must be added to a family by an adult first")
	default:
		return "", fmt.Errorf("Xbox security token error %d", out.XErr)
	}
	if out.Token == "" {
		return "", fmt.Errorf("Xbox returned an empty security token")
	}
	return out.Token, nil
}

func mcLogin(userHash, xstsToken string) (token string, expiresIn int64, err error) {
	body := map[string]any{"identityToken": fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken)}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := postJSON(mcsvcBase()+"/authentication/login_with_xbox", body, &out); err != nil {
		return "", 0, fmt.Errorf("Minecraft services sign-in failed: %v", err)
	}
	if out.AccessToken == "" {
		return "", 0, fmt.Errorf("Minecraft services returned no token")
	}
	if out.ExpiresIn <= 0 {
		out.ExpiresIn = 86400
	}
	return out.AccessToken, out.ExpiresIn, nil
}

func mcProfile(mcToken string) (name, uuid string, err error) {
	req, _ := http.NewRequest("GET", mcsvcBase()+"/minecraft/profile", nil)
	req.Header.Set("Authorization", "Bearer "+mcToken)
	resp, err := msaHTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var out struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", err
	}
	if resp.StatusCode == 404 || out.ID == "" {
		return "", "", fmt.Errorf("this Microsoft account doesn't own Minecraft: Java Edition")
	}
	return out.Name, out.ID, nil
}

func postForm(u string, form url.Values, out any) error {
	resp, err := msaHTTP.Post(u, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}

func postJSON(u string, body, out any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := msaHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
