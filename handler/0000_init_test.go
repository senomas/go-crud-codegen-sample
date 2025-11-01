package handler_test

import (
	"crypto/ecdh"
	"crypto/rand"
	"log"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"example.com/app-api/handler"
	"example.com/app-api/model"
	"example.com/app-api/util"
	_ "example.com/app-api/util"
)

var (
	token        string
	refreshToken string
	tbeforeDummy time.Time
	tafterDummy  time.Time
	shared       []byte
	curve        = ecdh.P256()
	sPriv, _     = curve.GenerateKey(rand.Reader)
	pubKey, _    = util.EncodePubKey(sPriv.PublicKey())
)

func TestSetup(t *testing.T) {
	store := model.GetStore()

	api := http.NewServeMux()

	handler.UserHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.RoleHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.ParamHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.AuthHandlerRegister(api, store)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", handler.Secure(store, api))

	httpLog := handler.HTTPLogger(slog.Default(), handler.LoggerOptions{
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     8 << 10, // 8 KB
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: httpLog(mux),
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()
}
