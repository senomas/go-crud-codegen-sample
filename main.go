package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"example.com/app-api/handler"
	"example.com/app-api/model"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Swagger Example API
// @version         1.0
// @description     This is a sample server celler server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// --- Bearer/JWT security definition (swaggo) ---
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Provide your JWT like: "Bearer <token>"

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	store := model.GetStore()

	api := http.NewServeMux()
	handler.UserHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.RoleHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.ParamHandlerRegister(api, "/api/v1", store, handler.BasicAuthenticate)
	handler.AuthHandlerRegister(api, store)
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mux.Handle("/api/v1/", handler.Secure(store, api))

	if os.Getenv("API_DOCS") == "true" {
		// Serve Swagger UI and JSON when in development
		mux.Handle("/docs/swagger.json", http.FileServer(http.Dir("/app")))
		mux.Handle("/docs/", httpSwagger.Handler(
			httpSwagger.URL("/docs/swagger.json"), // URL for swagger.json
		))
	} else {
		// Show a friendly message when in production
		mux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintln(w, "Swagger UI is disabled. Available only in development environment.")
		})
		mux.HandleFunc("/docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintln(w, `{"error": "Swagger spec disabled in production"}`)
		})
	}
	staticDir := "/app/static"
	static := http.FileServer(http.Dir(staticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticDir, r.URL.Path)
		fmt.Printf("Request for: (%s) (%s)\n", r.URL.String(), path)

		// If file exists, serve it
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fmt.Printf("Serving static file: %s\n", path)
			static.ServeHTTP(w, r)
			return
		}

		fmt.Printf("File not found, serving index.html: %s\n", path)
		// Otherwise, serve index.html
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	// mux.Handle("/", newProxy("http://mwui:80/"))

	httpLog := handler.HTTPLogger(slog.Default(), handler.LoggerOptions{
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     8 << 10, // 8 KB
	})

	go func() {
		appPort := os.Getenv("APP_PORT")
		if appPort == "" {
			appPort = "8080"
		}
		srv := &http.Server{
			Addr:    fmt.Sprintf(":%s", appPort),
			Handler: httpLog(mux),
		}
		err := srv.ListenAndServe()
		if err != nil {
			slog.Error("server error", "error", err)
		}
	}()

	appPort := os.Getenv("TLS_APP_PORT")
	if appPort == "" {
		appPort = "8443"
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", appPort),
		Handler: httpLog(mux),
	}
	err := srv.ListenAndServeTLS("/app/server.crt", "/app/server.key")
	if err != nil {
		slog.Error("server error", "error", err)
	}
}
