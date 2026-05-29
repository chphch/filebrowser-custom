package fbhttp

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/filebrowser/filebrowser/v2/users"
)

// ShortTokenExpiration bounds how long a one-time URL-bootstrap token is
// valid. The window is intentionally tight so that even if the token leaks
// (Cloudflare access logs, browser autocomplete, screenshot) the blast
// radius is small. The SPA exchanges it for the regular long-lived JWT on
// boot, then immediately strips the token from window.location.
const ShortTokenExpiration = 5 * time.Minute

// shortTokenStore tracks unconsumed short tokens by their jti so that each
// token is single-use. Process-local; on restart all outstanding tokens
// are invalidated, which is acceptable given the 5-minute TTL.
var (
	shortTokensMu sync.Mutex
	shortTokens   = make(map[string]time.Time) // jti -> expiry
)

// shortTokenClaims is the JWT payload for a bootstrap short token. It
// carries only the user ID and a type marker — no permissions, no scope —
// so even if the raw token is replayed as `X-Auth` against the normal
// endpoints, `extractor.ExtractToken` rejects it (claims shape mismatch
// against `authToken`).
type shortTokenClaims struct {
	UserID uint   `json:"uid"`
	Type   string `json:"typ"`
	jwt.RegisteredClaims
}

func newShortJTI() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func pruneShortTokens(now time.Time) {
	for jti, exp := range shortTokens {
		if now.After(exp) {
			delete(shortTokens, jti)
		}
	}
}

// shortTokenHandler issues a one-time, short-lived JWT bound to the
// authenticated user. Intended to be embedded in a URL (`?auth=<token>`)
// so that a cookie-less browser context (e.g. an in-app WebView opened
// from a third-party app) can bootstrap a session by exchanging it on
// page load.
var shortTokenHandler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	jti, err := newShortJTI()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	now := time.Now()
	exp := now.Add(ShortTokenExpiration)

	claims := &shortTokenClaims{
		UserID: d.user.ID,
		Type:   "short",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "File Browser Short",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(d.settings.Key)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	shortTokensMu.Lock()
	pruneShortTokens(now)
	shortTokens[jti] = exp
	shortTokensMu.Unlock()

	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(signed)); err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
})

// exchangeShortHandler trades a valid, unconsumed short token for a
// regular long-lived JWT. No `X-Auth` is required — this is the bootstrap
// path. The short token MUST come via the `X-Short-Auth` header (POST
// body) so it never lands in server access logs the way a query
// parameter would.
func exchangeShortHandler(tokenExpireTime time.Duration) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		shortJWT := r.Header.Get("X-Short-Auth")
		if shortJWT == "" {
			return http.StatusUnauthorized, nil
		}

		keyFunc := func(_ *jwt.Token) (interface{}, error) {
			return d.settings.Key, nil
		}
		var tk shortTokenClaims
		parser := jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
		)
		token, err := parser.ParseWithClaims(shortJWT, &tk, keyFunc)
		if err != nil || !token.Valid || tk.Type != "short" || tk.ID == "" {
			return http.StatusUnauthorized, nil
		}

		// Single-use consumption: succeed iff the jti is still in the
		// store. Deleting under the lock guarantees that two concurrent
		// exchanges for the same token can't both win.
		now := time.Now()
		shortTokensMu.Lock()
		_, ok := shortTokens[tk.ID]
		if ok {
			delete(shortTokens, tk.ID)
		}
		pruneShortTokens(now)
		shortTokensMu.Unlock()
		if !ok {
			return http.StatusUnauthorized, nil
		}

		var user *users.User
		user, err = d.store.Users.Get(d.server.Root, tk.UserID)
		if err != nil {
			return http.StatusUnauthorized, nil
		}

		return printToken(w, r, d, user, tokenExpireTime)
	}
}
