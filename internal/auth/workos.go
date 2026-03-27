package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/your-org/ledger-engine/internal/models"
)

type contextKey string

const userContextKey contextKey = "authenticated_user"

type WorkOSAuth struct {
	// HTTPClient is shared for outbound calls to WorkOS APIs.
	HTTPClient        *http.Client
	// Endpoint and credential configuration, loaded from env in NewWorkOSAuth().
	UserInfoURL       string
	AuthorizeURL      string
	AuthenticateURL   string
	ClientID          string
	ClientSecret      string
	RedirectURI       string
	PostLoginRedirect string
	CookieSecret      []byte
}

type workOSUserResponse struct {
	Email string `json:"email"`
}

type authenticateRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
}

type authenticateResponse struct {
	AccessToken string `json:"access_token"`
}

type sessionPayload struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

func NewWorkOSAuth() *WorkOSAuth {
	// Keep endpoint values overridable for local testing and staging.
	userInfoURL := os.Getenv("WORKOS_USERINFO_URL")
	if strings.TrimSpace(userInfoURL) == "" {
		userInfoURL = "https://api.workos.com/user_management/users/me"
	}

	authorizeURL := os.Getenv("WORKOS_AUTHORIZE_URL")
	if strings.TrimSpace(authorizeURL) == "" {
		authorizeURL = "https://api.workos.com/user_management/authorize"
	}

	authenticateURL := os.Getenv("WORKOS_AUTHENTICATE_URL")
	if strings.TrimSpace(authenticateURL) == "" {
		authenticateURL = "https://api.workos.com/user_management/authenticate"
	}

	postLoginRedirect := os.Getenv("WORKOS_POST_LOGIN_REDIRECT")
	if strings.TrimSpace(postLoginRedirect) == "" {
		postLoginRedirect = "/"
	}

	cookieSecret := os.Getenv("SESSION_COOKIE_SECRET")
	if strings.TrimSpace(cookieSecret) == "" {
		cookieSecret = "dev-change-me-session-cookie-secret"
	}

	return &WorkOSAuth{
		HTTPClient:        &http.Client{Timeout: 10 * time.Second},
		UserInfoURL:       userInfoURL,
		AuthorizeURL:      authorizeURL,
		AuthenticateURL:   authenticateURL,
		ClientID:          os.Getenv("WORKOS_CLIENT_ID"),
		ClientSecret:      os.Getenv("WORKOS_API_KEY"),
		RedirectURI:       os.Getenv("WORKOS_REDIRECT_URI"),
		PostLoginRedirect: postLoginRedirect,
		CookieSecret:      []byte(cookieSecret),
	}
}

func (a *WorkOSAuth) ConfigWarnings() []string {
	// Surface misconfiguration early at startup without hard-failing local development.
	warnings := make([]string, 0, 6)
	if strings.TrimSpace(a.ClientID) == "" {
		warnings = append(warnings, "WORKOS_CLIENT_ID is not set")
	} else if !strings.HasPrefix(strings.TrimSpace(a.ClientID), "client_") {
		warnings = append(warnings, "WORKOS_CLIENT_ID should start with 'client_' (publishable keys like 'pk_' are invalid here)")
	}
	if strings.TrimSpace(a.ClientSecret) == "" {
		warnings = append(warnings, "WORKOS_API_KEY is not set")
	}
	if strings.TrimSpace(a.RedirectURI) == "" {
		warnings = append(warnings, "WORKOS_REDIRECT_URI is not set")
	}
	if len(a.CookieSecret) < 32 {
		warnings = append(warnings, "SESSION_COOKIE_SECRET should be at least 32 characters")
	}
	if string(a.CookieSecret) == "dev-change-me-session-cookie-secret" ||
		string(a.CookieSecret) == "replace-this-with-a-long-random-secret" {
		warnings = append(warnings, "SESSION_COOKIE_SECRET is still using a default placeholder")
	}
	if strings.Contains(a.ClientSecret, "|") {
		warnings = append(warnings, "WORKOS_API_KEY contains '|' - verify key was copied correctly")
	}
	return warnings
}

func (a *WorkOSAuth) IsConfigured() bool {
	return strings.TrimSpace(a.ClientID) != "" &&
		strings.TrimSpace(a.ClientSecret) != "" &&
		strings.TrimSpace(a.RedirectURI) != ""
}

func (a *WorkOSAuth) ResolveEmail(ctx context.Context, bearerToken string) (string, error) {
	// We treat WorkOS as source of truth for identity and map that to a local user.
	if strings.TrimSpace(bearerToken) == "" {
		return "", errors.New("missing bearer token")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("workos user lookup failed with status %d", res.StatusCode)
	}

	var payload workOSUserResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.Email) == "" {
		return "", errors.New("workos response missing email")
	}

	return payload.Email, nil
}

func (a *WorkOSAuth) BuildAuthorizeURL(state string) (string, error) {
	// Build WorkOS authorize URL for Authorization Code flow.
	if !a.IsConfigured() {
		return "", errors.New("workos auth not configured")
	}

	u, err := url.Parse(a.AuthorizeURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("client_id", a.ClientID)
	q.Set("redirect_uri", a.RedirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (a *WorkOSAuth) ExchangeCode(ctx context.Context, code string) (string, error) {
	// Exchange one-time authorization code for access token.
	reqBody := authenticateRequest{
		GrantType:    "authorization_code",
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
		Code:         code,
		RedirectURI:  a.RedirectURI,
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.AuthenticateURL, strings.NewReader(string(rawBody)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := a.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("workos code exchange failed with status %d", res.StatusCode)
	}

	var payload authenticateResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", errors.New("workos code exchange returned empty access token")
	}

	return payload.AccessToken, nil
}

func (a *WorkOSAuth) NewRandomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *WorkOSAuth) EncodeSession(token string, ttl time.Duration) (string, error) {
	// Signed cookie payload; token remains inaccessible to frontend JS (HttpOnly cookie).
	payload := sessionPayload{
		AccessToken: token,
		ExpiresAt:   time.Now().Add(ttl).Unix(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(raw)
	signature := a.sign(encoded)
	return encoded + "." + signature, nil
}

func (a *WorkOSAuth) DecodeSession(v string) (string, error) {
	// Verify HMAC signature before trusting cookie contents.
	parts := strings.Split(v, ".")
	if len(parts) != 2 {
		return "", errors.New("invalid session format")
	}

	encoded := parts[0]
	signature := parts[1]
	if !hmac.Equal([]byte(signature), []byte(a.sign(encoded))) {
		return "", errors.New("invalid session signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return "", errors.New("session expired")
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", errors.New("missing access token")
	}

	return payload.AccessToken, nil
}

func (a *WorkOSAuth) sign(v string) string {
	mac := hmac.New(sha256.New, a.CookieSecret)
	_, _ = mac.Write([]byte(v))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func UserFromContext(ctx context.Context) (*models.User, bool) {
	u, ok := ctx.Value(userContextKey).(*models.User)
	return u, ok
}

func Middleware(db *models.UserModel, resolver *WorkOSAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Support either bearer tokens (API clients) or session cookie (browser flow).
			var token string
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				token = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			}
			if token == "" {
				cookie, err := r.Cookie("session")
				if err == nil && cookie != nil {
					token, _ = resolver.DecodeSession(cookie.Value)
				}
			}
			if token == "" {
				http.Error(w, "missing authentication", http.StatusUnauthorized)
				return
			}

			email, err := resolver.ResolveEmail(r.Context(), token)
			if err != nil {
				http.Error(w, "invalid authentication token", http.StatusUnauthorized)
				return
			}

			user, err := db.GetByEmail(r.Context(), email)
			if errors.Is(err, sql.ErrNoRows) {
				// First login auto-provisions a local user row.
				user, err = db.Insert(r.Context(), email, "active")
				if err != nil {
					http.Error(w, "failed to provision user", http.StatusInternalServerError)
					return
				}
			} else if err != nil {
				http.Error(w, "failed to load user", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
