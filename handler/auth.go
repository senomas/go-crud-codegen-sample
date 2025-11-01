package handler

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"example.com/app-api/model"
	"example.com/app-api/util"
	"example.com/app-api/util/jsql"
)

func AuthHandlerRegister(mux *http.ServeMux, store model.Store) {
	mux.HandleFunc("PUT /api/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		if err := AuthLogin(r.Context(), store, w, r); err != nil {
			writeInternalError(w, err)
		}
	})
	mux.HandleFunc("POST /api/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		if err := AuthRefresh(r.Context(), store, w, r); err != nil {
			writeInternalError(w, err)
		}
	})
	mux.HandleFunc("DELETE /api/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		if err := AuthLogout(r.Context(), store, w, r); err != nil {
			writeInternalError(w, err)
		}
	})
	mux.HandleFunc("GET /api/v1/auth", func(w http.ResponseWriter, r *http.Request) {
		if err := AuthGet(r.Context(), store, w, r); err != nil {
			writeInternalError(w, err)
		}
	})
	mux.HandleFunc("GET /api/v1/auth/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})
}

// swagger: model LoginObjectaRequest
type LoginObjectRequest struct {
	PublicKey string `json:"public_key,omitempty"`
	Email     string `json:"email"`
	Password  string `json:"password,omitempty"`
}

// swagger: model LoginObject
type LoginObject struct {
	PublicKey    string     `json:"public_key,omitempty"`
	Email        string     `json:"email"`
	Password     string     `json:"password,omitempty"`
	Token        string     `json:"token,omitempty"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	User         *LoginUser `json:"user,omitempty"`
}

var (
	secret               = ""
	TOKEN_EXPIRY         = 5 * time.Minute
	REFRESH_TOKEN_EXPIRY = 60 * time.Minute
	curve                = ecdh.P256()
	sPriv, _             = curve.GenerateKey(rand.Reader)
	sPub                 = sPriv.PublicKey()
	sPubXY, _            = util.EncodePubKey(sPub)
)

func init() {
	if str := os.Getenv("JWT_SECRET"); str != "" {
		secret = str
	} else {
		slog.Error("JWT_SECRET env var is not set, using default")
		os.Exit(1)
	}
	if str := os.Getenv("TOKEN_EXPIRY"); str != "" {
		if v, err := strconv.Atoi(str); err == nil {
			TOKEN_EXPIRY = time.Duration(v) * time.Minute
		} else {
			slog.Warn("invalid TOKEN_EXPIRY env var, using default", "err", err, "value", str)
		}
	}
	if str := os.Getenv("REFRESH_TOKEN_EXPIRY"); str != "" {
		if v, err := strconv.Atoi(str); err == nil {
			REFRESH_TOKEN_EXPIRY = time.Duration(v) * time.Minute
		} else {
			slog.Warn("invalid REFRESH_TOKEN_EXPIRY env var, using default", "err", err, "value", str)
		}
	}
}

func getUser(ctx context.Context, store model.Store, obj *LoginObject) *model.User {
	if obj.Email == "" {
		slog.Warn("email is empty")
		return nil
	}
	user, err := store.User().GetByEmail(ctx, obj.Email)
	if err != nil && err.Error() != "NOT_FOUND" {
		slog.Warn("failed to get user by email", "email", obj.Email, "err", err)
		return nil
	}
	if user == nil {
		slog.Warn("user not found", "email", obj.Email)
		return nil
	}
	if obj.Token != "" {
		claim, err := ParseHS256(obj.Token)
		if err != nil {
			slog.Warn("invalid token", "email", obj.Email, "err", err)
			return nil
		}
		if claim.Subject != user.Email {
			slog.Warn("email mismatch", "email", obj.Email, "token_email", claim.Subject)
			return nil
		}
		return user
	}
	if obj.RefreshToken != "" {
		claim, err := ParseHS256(obj.RefreshToken)
		if err != nil {
			slog.Warn("invalid token", "email", obj.Email, "err", err)
			return nil
		}
		if claim.Subject != user.Email {
			slog.Warn("email mismatch", "email", obj.Email, "token_email", claim.Subject)
			return nil
		}
		if user.Token.String != obj.RefreshToken {
			slog.Warn("refresh token mismatch", "email", obj.Email)
			return nil
		}
		return user
	}
	if obj.Password != "" {
		if ok, err := util.VerifyPassword(obj.Password, user.Password); err != nil {
			slog.Warn("failed to verify password", "email", obj.Email, "err", err)
			return nil
		} else if !ok {
			slog.Warn("invalid password", "email", obj.Email)
			return nil
		}
		user.Password = jsql.SecretValueNull()
		user.Token = jsql.SecretValueNull()
		return user
	}
	bb, _ := json.MarshalIndent(obj, "", "  ")
	slog.Warn("no authentication method provided", "email", obj.Email, "login_object", string(bb))
	return nil
}

func mergeMap(target map[string]any, source map[string]any) map[string]any {
	for k, sv := range source {
		if tv, ok := target[k]; ok {
			if svm, ok := sv.(map[string]any); ok {
				if tvm, ok := tv.(map[string]any); ok {
					target[k] = mergeMap(tvm, svm)
				} else {
					target[k] = sv
				}
			} else if svb, ok := sv.(bool); ok {
				if svb {
					target[k] = sv
				}
			} else {
				target[k] = sv
			}
		} else {
			target[k] = sv
		}
	}
	return target
}

func toLoginUser(user *model.User) *LoginUser {
	roles := []string{}
	mprivs := map[string]any{}
	for _, role := range user.Roles {
		roles = append(roles, role.Name)
		p := map[string]any{}
		err := json.Unmarshal([]byte(role.Privileges), &p)
		if err != nil {
			slog.Warn("failed to unmarshal role privileges", "role", role.Name, "privileges", role.Privileges, "err", err)
		}
		mprivs = mergeMap(mprivs, p)
	}
	cuser := *user
	cuser.Password = jsql.SecretValueNull()
	cuser.Token = jsql.SecretValueNull()
	return &LoginUser{
		Name:       user.Name,
		Email:      user.Email,
		Privileges: mprivs,
		Roles:      roles,
		User:       &cuser,
	}
}

// ShowUser   godoc
// @Summary      Get user By PK
// @Description  Get user By PK
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user  body     LoginObjectRequest true  "User object"
// @Success      200  {object}  LoginObject
// @Failure      400  {object}  HttpResult
// @Failure      404  {object}  HttpResult
// @Failure      500  {object}  HttpResult
// @Router       /auth [put]
func AuthLogin(ctx context.Context, store model.Store, w http.ResponseWriter, r *http.Request) error {
	var obj LoginObject
	if r.Header.Get("X-Req-Signature") == "" {
		obj.PublicKey = sPubXY
		_ = json.NewEncoder(w).Encode(obj)
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(&obj)
	if err != nil {
		slog.Warn("invalid body", "err", err)
		return fmt.Errorf("invalid body")
	}
	if obj.Email == "" {
		slog.Warn("email is empty")
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if obj.PublicKey == "" {
		slog.Warn("public key is empty")
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	cpub, err := util.DecodePubKey(obj.PublicKey)
	if err != nil {
		slog.Warn("invalid public key", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	shared, err := sPriv.ECDH(cpub)
	slog.Debug("computed shared secret", "email", obj.Email, "shared_len", base64.RawStdEncoding.EncodeToString(shared))
	if err != nil {
		slog.Warn("failed to compute shared secret", "err", err)
	}
	if !BasicHMAC(r, "auth", "login", shared) {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	obj.Token = ""
	obj.RefreshToken = ""
	var user *model.User
	user = getUser(ctx, store, &obj)
	obj.Password = ""
	if user == nil {
		slog.Warn("invalid credentials", "email", obj.Email)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	obj.Token, err = SignHS256(user.Email, TOKEN_EXPIRY)
	if err != nil {
		slog.Error("failed to sign token", "err", err)
		return fmt.Errorf("login failed")
	}
	obj.RefreshToken, err = SignHS256(user.Email, REFRESH_TOKEN_EXPIRY)
	if err != nil {
		slog.Error("failed to sign refresh token", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	user.Token = jsql.SecretValue(obj.RefreshToken)
	user.Secret = jsql.SecretValue(base64.RawStdEncoding.EncodeToString(shared))
	err = store.User().Update(ctx, *user, []model.UserField{
		model.UserField_Token, model.UserField_Secret,
	})
	if err != nil {
		slog.Error("failed to update user token", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	obj.User = toLoginUser(user)
	_ = json.NewEncoder(w).Encode(obj)
	return nil
}

func AuthRefresh(ctx context.Context, store model.Store, w http.ResponseWriter, r *http.Request) error {
	if !BasicHMAC(r, "auth", "refresh", nil) {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	var obj LoginObject
	err := json.NewDecoder(r.Body).Decode(&obj)
	if err != nil {
		slog.Warn("invalid body", "err", err)
		return fmt.Errorf("invalid body")
	}
	obj.Password = ""
	obj.Token = ""
	var user *model.User
	user = getUser(ctx, store, &obj)
	obj.RefreshToken = ""
	if user == nil {
		slog.Warn("invalid refresh credentials", "email", obj.Email)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if !user.Secret.Valid {
		slog.Warn("user secret is not valid", "email", obj.Email)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	var shared []byte
	shared, err = base64.RawStdEncoding.DecodeString(user.Secret.String)
	if err != nil {
		slog.Warn("failed to decode user secret", "email", obj.Email, "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	if !BasicHMAC(r, "auth", "refresh", shared) {
		slog.Warn("invalid hmac for refresh", "email", obj.Email)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	obj.Token, err = SignHS256(user.Email, TOKEN_EXPIRY)
	if err != nil {
		slog.Error("failed to sign token", "err", err)
		return fmt.Errorf("login failed")
	}
	obj.RefreshToken, err = SignHS256(user.Email, REFRESH_TOKEN_EXPIRY)
	if err != nil {
		slog.Error("failed to sign refresh token", "err", err)
		return fmt.Errorf("login failed")
	}
	user.Token = jsql.SecretValue(obj.RefreshToken)
	user.Secret = jsql.SecretValue(base64.RawStdEncoding.EncodeToString(shared))
	err = store.User().Update(ctx, *user, []model.UserField{
		model.UserField_Token, model.UserField_Secret,
	})
	if err != nil {
		slog.Error("failed to update user token", "err", err)
		return fmt.Errorf("refresh token failed")
	}
	obj.User = toLoginUser(user)
	_ = json.NewEncoder(w).Encode(obj)
	return nil
}

func AuthLogout(ctx context.Context, store model.Store, w http.ResponseWriter, r *http.Request) error {
	var obj LoginObject
	err := json.NewDecoder(r.Body).Decode(&obj)
	if err != nil {
		slog.Warn("invalid body", "err", err)
		return fmt.Errorf("invalid body")
	}
	obj.Password = ""
	obj.RefreshToken = ""
	var user *model.User
	user = getUser(ctx, store, &obj)
	obj.Token = ""
	if user == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	user.Token = jsql.SecretValueNull()
	user.Secret = jsql.SecretValueNull()
	err = store.User().Update(ctx, *user, []model.UserField{
		model.UserField_Token, model.UserField_Secret,
	})
	if err != nil {
		slog.Warn("failed to update user token", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
	return nil
}

func AuthGet(ctx context.Context, store model.Store, w http.ResponseWriter, r *http.Request) error {
	var obj LoginObject
	err := json.NewDecoder(r.Body).Decode(&obj)
	if err != nil {
		slog.Warn("invalid body", "err", err)
		return fmt.Errorf("invalid body")
	}
	obj.Password = ""
	obj.RefreshToken = ""
	var user *model.User
	user = getUser(ctx, store, &obj)
	obj.Token = ""
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
		return nil
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(model.User{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Roles: user.Roles,
	})
	return nil
}
