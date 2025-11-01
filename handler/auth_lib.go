package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"example.com/app-api/model"
	lru "github.com/hashicorp/golang-lru/v2"
)

type Params struct {
	Memory      uint32 // in KiB
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

var hashParam = &Params{
	Memory:      64 * 1024, // 64 MiB
	Iterations:  3,
	Parallelism: 2,
	SaltLen:     16,
	KeyLen:      32,
}

// swagger: model LoginUser
type LoginUser struct {
	Email      string         `json:"email"`
	Name       string         `json:"name"`
	User       *model.User    `json:"user,omitempty"`
	Roles      []string       `json:"roles,omitempty"`
	Privileges map[string]any `json:"privileges,omitempty"`
}

var (
	nonces, _  = lru.New[string, struct{}](100000)
	noncesLock sync.Mutex
)

type Authenticate func(r *http.Request, resourece, action string) bool

func Secure(store model.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var claim *JwtClaims
		var err error
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token := auth[7:]
			claim, err = ParseHS256(token)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return
			}
		}
		if claim == nil {
			session, _ := r.Cookie("session")
			if session != nil {
				claim, err = ParseHS256(session.Value)
				if err != nil {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("unauthorized"))
					return
				}
			}
		}
		if claim != nil {
			user, err := store.User().GetByEmail(r.Context(), claim.Subject)
			if err != nil {
				if err.Error() == "NOT_FOUND" {
					slog.Warn("user not found", "email", claim.Subject)
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("unauthorized"))
					return
				}
				slog.Error("get user by id", "error", err)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return
			}
			if user == nil {
				slog.Warn("user not found", "email", claim.Subject)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return
			}
			if user.Email != claim.Subject {
				slog.Warn("user email not match", "email", user.Email, "subject", claim.Subject)
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte("unauthorized"))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(),
				HandlerCtxKeyUser, toLoginUser(user))))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func BasicHMAC(r *http.Request, resource, action string, shared []byte) bool {
	var hmacSignature, nonce, reqTime string
	if hmacSignature = r.Header.Get("X-Req-Signature"); hmacSignature == "" {
		slog.Warn("missing X-Req-Signature header", "url", r.URL.String(), "method", r.Method)
		return false
	}
	if nonce = r.Header.Get("X-Req-Nonce"); nonce == "" {
		slog.Warn("missing X-Nonce header", "url", r.URL.String(), "method", r.Method)
		return false
	}
	if len(nonce) < 20 {
		slog.Warn("X-Req-Nonce header too short", "url", r.URL.String(), "method", r.Method, "nonce", nonce)
		return false
	}
	if reqTime = r.Header.Get("X-Req-Timestamp"); reqTime == "" {
		slog.Warn("missing X-Req-Timestamp header", "url", r.URL.String(), "method", r.Method)
		return false
	}
	if rt, err := time.Parse(time.RFC3339, reqTime); err != nil {
		slog.Warn("invalid X-Req-Timestamp header", "url", r.URL.String(), "method", r.Method, "reqTime", reqTime, "error", err)
		return false
	} else if time.Since(rt) > 1*time.Minute || time.Until(rt) > 1*time.Minute {
		slog.Warn("X-Req-Timestamp header out of range", "url", r.URL.String(), "method", r.Method, "reqTime", reqTime)
		return false
	}
	if len(shared) > 0 {
		bodyHash := r.Header.Get("X-Body-Hash")
		var buf bytes.Buffer
		var ok bool
		if buf, ok = r.Context().Value(HandlerCtxKeyBody).(bytes.Buffer); !ok {
			slog.Warn("missing body in context", "url", r.URL.String(), "method", r.Method)
		}
		mac := hmac.New(sha256.New, []byte(nonce))
		mac.Write(buf.Bytes())
		rawBodyHash := mac.Sum(nil)
		calcBodyHash := base64.RawStdEncoding.EncodeToString(rawBodyHash)
		if bodyHash != calcBodyHash {
			slog.Warn("X-Body-Hash header does not match request body", "url", r.URL.String(), "method", r.Method,
				"body_hash", bodyHash, "calc_body_hash", calcBodyHash)
			return false
		}
		{
			noncesLock.Lock()
			defer noncesLock.Unlock()
			nkey := reqTime + "-" + nonce

			if _, exists := nonces.Get(nkey); exists {
				slog.Warn("nonce replay attack detected", "url", r.URL.String(), "method", r.Method, "nonce", nonce)
				return false
			}
			nonces.Add(nkey, struct{}{})
		}
		mac = hmac.New(sha256.New, shared)
		mac.Write([]byte(nonce))
		mac.Write([]byte(";"))
		mac.Write([]byte(reqTime))
		mac.Write([]byte(";"))
		mac.Write([]byte(r.Method))
		mac.Write([]byte(";"))
		mac.Write([]byte(r.URL.RequestURI()))
		mac.Write([]byte(";"))
		mac.Write([]byte(calcBodyHash))
		calcSignature := base64.RawStdEncoding.EncodeToString(mac.Sum(nil))
		if hmacSignature != calcSignature {
			slog.Warn("X-Req-Signature header does not match", "url", r.URL.String(),
				"nonce", nonce, "method", r.Method, "RequestURI", r.URL.RequestURI(),
				"calc_body_hash", calcBodyHash, "hmacSignature", hmacSignature,
				"calc_signature", calcSignature,
			)
			return false
		}
	}
	return true
}

func BasicAuthenticate(r *http.Request, resource, action string) bool {
	if luser, ok := r.Context().Value(HandlerCtxKeyUser).(*LoginUser); !ok {
		slog.Warn("missing login user in context")
	} else if luser == nil {
		slog.Warn("nil login user in context")
	} else if pm, ok := luser.Privileges[resource].(map[string]any); ok {
		if b, ok := pm[action].(bool); ok {
			if b {
				method := r.Method
				if method == http.MethodPatch || method == http.MethodPut || method == http.MethodDelete {
					if luser.User.Secret.Valid == false || luser.User.Secret.String == "" {
						slog.Warn("missing user secret for action", "user", luser.Email, "resource", resource, "action", action)
						return false
					}
					shared, err := base64.RawStdEncoding.DecodeString(luser.User.Secret.String)
					if err != nil {
						slog.Warn("invalid user secret for action", "user", luser.Email, "resource", resource, "action", action, "error", err)
						return false
					}
					if !BasicHMAC(r, resource, action, shared) {
						return false
					}
				}
				return true
			}
			slog.Warn("missing privilege for action", "resource", resource, "action", action, "user", luser.Email, "res.privileges", pm)
		} else {
			slog.Warn("missing privilege for action", "resource", resource, "action", action, "user", luser.Email, "res.privileges", pm)
		}
	} else {
		slog.Warn("missing privilege for resource", "resource", resource, "user", luser.Email, "privileges", luser.Privileges)
	}
	return false
}
