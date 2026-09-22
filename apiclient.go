package main

// This file is csax's second way to reach a deployment: calling a
// running `api` instance's admin endpoints over HTTP, authenticated
// as an operator, instead of touching Postgres directly. It exists
// alongside connectDB/buildEngine in config.go, not instead of them —
// see apiMode below for how a command picks which one applies.
//
// The commands with no matching api endpoint (users create/unlock,
// sessions revoke/revoke-all, ai query/logs/audit) are not touched by
// this file at all and keep working exactly as before regardless of
// whether CSAX_API_URL is set — api's own admin console was built
// deliberately without account-lock/unlock or cross-user session
// revocation power, and closing that gap is a decision for api's own
// project, not something this repo routes around.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
)

// apiMode reports whether a command should reach the deployment over
// HTTP instead of connecting to Postgres directly.
func apiMode(cfg csaxConfig) bool {
	return cfg.APIURL != ""
}

// sessionCache is what's persisted to disk between commands, so an
// operator isn't asked to log in again for every single command.
// Never holds the password — only tokens.
type sessionCache struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func sessionCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory for session cache: %w", err)
	}
	return filepath.Join(home, ".csax", "session.json"), nil
}

func loadSessionCache() (*sessionCache, error) {
	path, err := sessionCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cached session: %w", err)
	}
	var sc sessionCache
	if err := json.Unmarshal(data, &sc); err != nil {
		// A corrupt cache file should not block login — start fresh
		// rather than failing every command until a human intervenes.
		return nil, nil
	}
	return &sc, nil
}

func saveSessionCache(sc *sessionCache) error {
	path, err := sessionCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating session cache directory: %w", err)
	}
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func clearSessionCache() error {
	path, err := sessionCachePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// apiEnvelope mirrors api's own {"data":...}/{"error":...} response
// shape exactly — see crydensync-web's api/index.mdx.
type apiEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *apiErrorBody   `json:"error"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiCallError is what every apiClient method returns for a non-2xx
// response — the code is preserved so callers (and errorHint below)
// can branch on it, matching how api's own docs tell a client to
// handle its errors: on code, never on message text.
type apiCallError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *apiCallError) Error() string {
	if hint := errorHint(e.Code); hint != "" {
		return fmt.Sprintf("%s (%s: %s)", e.Message, e.Code, hint)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// errorHint expands the handful of codes an operator needs plain
// English for, not a bare status dump.
func errorHint(code string) string {
	switch code {
	case "not_operator":
		return "this account is not an admin on this deployment — grant it with the api repo's own grant-operator tool, then log in again"
	case "not_implemented_on_sqlite":
		return "this deployment runs on SQLite — the admin console isn't available at all there yet, core auth (login/health) still works, but no /v1/admin/* route does"
	default:
		return ""
	}
}

// apiClient wraps calls to a deployed api instance's admin surface,
// authenticated as an operator.
type apiClient struct {
	baseURL string
	http    *http.Client
	session *sessionCache
}

func newAPIClient(cfg csaxConfig) (*apiClient, error) {
	if cfg.APIURL == "" {
		return nil, fmt.Errorf("CSAX_API_URL is not set")
	}
	sc, err := loadSessionCache()
	if err != nil {
		return nil, err
	}
	return &apiClient{
		baseURL: strings.TrimRight(cfg.APIURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
		session: sc,
	}, nil
}

// ensureLoggedIn returns a valid access token: the cached one if it's
// still fresh, a refreshed one if not, or a brand new login if there's
// no cached session (or refreshing it failed) at all.
func (c *apiClient) ensureLoggedIn(cfg csaxConfig) (string, error) {
	if c.session != nil && time.Now().Before(c.session.ExpiresAt) {
		return c.session.AccessToken, nil
	}
	if c.session != nil && c.session.RefreshToken != "" {
		if tok, err := c.refresh(); err == nil {
			return tok, nil
		}
		// The refresh token itself may have been rotated away or
		// revoked server-side — fall through to a fresh login rather
		// than failing here.
	}
	return c.login(cfg)
}

func promptLine(label string) string {
	fmt.Print(label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptPassword(label string) (string, error) {
	fmt.Print(label)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return string(pw), nil
}

func (c *apiClient) login(cfg csaxConfig) (string, error) {
	email := cfg.APIEmail
	if email == "" {
		email = promptLine("Operator email: ")
	}
	password := cfg.APIPassword
	if password == "" {
		var err error
		password, err = promptPassword("Operator password: ")
		if err != nil {
			return "", err
		}
	}

	var loginResp struct {
		AccessToken          string   `json:"access_token"`
		RefreshToken         string   `json:"refresh_token"`
		SecondFactorRequired bool     `json:"second_factor_required"`
		PendingToken         string   `json:"pending_token"`
		Methods              []string `json:"methods"`
	}
	if err := c.rawPost("/v1/login", map[string]string{"email": email, "password": password}, &loginResp); err != nil {
		return "", err
	}

	if loginResp.SecondFactorRequired {
		return c.completeTOTPLogin(loginResp.PendingToken, loginResp.Methods)
	}
	return c.storeSession(loginResp.AccessToken, loginResp.RefreshToken)
}

// completeTOTPLogin handles the one second factor a terminal prompt
// can reasonably complete. An operator account enrolled only in
// passkeys or recovery codes can't finish logging in through csax at
// all yet — that's said plainly rather than csax hanging or failing
// with an unrelated-looking error.
func (c *apiClient) completeTOTPLogin(pendingToken string, methods []string) (string, error) {
	hasTOTP := false
	for _, m := range methods {
		if m == "totp" {
			hasTOTP = true
		}
	}
	if !hasTOTP {
		return "", fmt.Errorf("this operator account requires a second factor (%s) csax cannot complete from a terminal — TOTP is the only one supported here", strings.Join(methods, ", "))
	}

	code := promptLine("TOTP code: ")

	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.rawPost("/v1/login/totp", map[string]string{"pending_token": pendingToken, "code": code}, &tokens); err != nil {
		return "", err
	}
	return c.storeSession(tokens.AccessToken, tokens.RefreshToken)
}

func (c *apiClient) refresh() (string, error) {
	if c.session == nil || c.session.RefreshToken == "" {
		return "", fmt.Errorf("no cached refresh token")
	}
	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.rawPost("/v1/refresh", map[string]string{"refresh_token": c.session.RefreshToken}, &tokens); err != nil {
		return "", err
	}
	return c.storeSession(tokens.AccessToken, tokens.RefreshToken)
}

func (c *apiClient) storeSession(accessToken, refreshToken string) (string, error) {
	// Access tokens are short-lived JWTs (15 minutes by default); csax
	// treats one as good for 10 to leave margin for clock drift and
	// request latency rather than parsing the JWT's own exp claim.
	c.session = &sessionCache{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	if err := saveSessionCache(c.session); err != nil {
		return "", fmt.Errorf("caching session: %w", err)
	}
	return accessToken, nil
}

// rawPost is used only by login/refresh/TOTP-completion, which run
// before any access token exists — every other call goes through do,
// below, which attaches one.
func (c *apiClient) rawPost(path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("contacting api: %w", err)
	}
	defer resp.Body.Close()

	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decoding api response: %w", err)
	}
	if env.Error != nil {
		return &apiCallError{StatusCode: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decoding api data: %w", err)
		}
	}
	return nil
}

// do is what every admin command in later tiers calls. It attaches
// the bearer token, retries exactly once after a transparent refresh
// on a 401, and decodes the envelope.
func (c *apiClient) do(cfg csaxConfig, method, path string, body any, out any) error {
	token, err := c.ensureLoggedIn(cfg)
	if err != nil {
		return err
	}

	send := func(tok string) (*http.Response, error) {
		var reader io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			reader = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, c.baseURL+path, reader)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		return c.http.Do(req)
	}

	resp, err := send(token)
	if err != nil {
		return fmt.Errorf("contacting api: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		// The cached access token may have expired between
		// ensureLoggedIn's check above and this request landing —
		// refresh once and retry rather than surfacing a stale-token
		// error an operator can't do anything about.
		resp.Body.Close()
		newTok, rerr := c.refresh()
		if rerr != nil {
			return fmt.Errorf("session expired and refresh failed, run `csax login` again: %w", rerr)
		}
		resp, err = send(newTok)
		if err != nil {
			return fmt.Errorf("contacting api: %w", err)
		}
	}
	defer resp.Body.Close()

	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decoding api response: %w", err)
	}
	if env.Error != nil {
		return &apiCallError{StatusCode: resp.StatusCode, Code: env.Error.Code, Message: env.Error.Message}
	}
	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decoding api data: %w", err)
		}
	}
	return nil
}

func (c *apiClient) get(cfg csaxConfig, path string, out any) error {
	return c.do(cfg, http.MethodGet, path, nil, out)
}
func (c *apiClient) post(cfg csaxConfig, path string, body, out any) error {
	return c.do(cfg, http.MethodPost, path, body, out)
}
func (c *apiClient) put(cfg csaxConfig, path string, body, out any) error {
	return c.do(cfg, http.MethodPut, path, body, out)
}
func (c *apiClient) delete(cfg csaxConfig, path string, out any) error {
	return c.do(cfg, http.MethodDelete, path, nil, out)
}
