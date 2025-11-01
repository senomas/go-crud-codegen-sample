package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoggerOptions controls what gets logged
type LoggerOptions struct {
	LogRequestBody  bool
	LogResponseBody bool
	MaxBodySize     int // bytes
}

// HTTPLogger returns a middleware that logs requests/responses to slog.
func HTTPLogger(log *slog.Logger, opt LoggerOptions) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	if opt.MaxBodySize == 0 {
		opt.MaxBodySize = 8 << 10 // default 8KB
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// --- Start Timer ---
			start := time.Now()

			// --- Capture Request ---
			reqBody := ""
			if opt.LogRequestBody && r.Body != nil {
				var buf bytes.Buffer
				tee := io.TeeReader(r.Body, &buf)
				body, _ := io.ReadAll(io.LimitReader(tee, int64(opt.MaxBodySize)))
				reqBody = string(body)
				r.Body = io.NopCloser(&buf) // restore body
				r = r.WithContext(context.WithValue(r.Context(), HandlerCtxKeyBody, buf))
			}

			reqHeaders := map[string]string{}
			for k, v := range r.Header {
				reqHeaders[k] = strings.Join(v, ", ")
			}

			// Print to console
			/* 			b, _ := json.MarshalIndent(reqHeaders, "", "  ")
			   			fmt.Println(string(b))
			*/
			// OR log with slog
			slog.Info("Request headers", "headers", reqHeaders)

			// --- Wrap Response ---
			rw := &respCapture{
				ResponseWriter: w,
				status:         http.StatusOK,
				maxBody:        opt.MaxBodySize,
				captureBody:    opt.LogResponseBody,
			}

			// --- Call next ---
			next.ServeHTTP(rw, r)
			dur := time.Since(start)

			// --- Prepare Log ---
			fields := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("url", r.URL.String()),
				slog.String("remote", r.RemoteAddr),
				slog.String("duration", dur.String()),
				slog.Int("status", rw.status),
				slog.Int64("resp_bytes", rw.bytes),
				slog.Any("headers", reqHeaders),
			}
			if opt.LogRequestBody {
				maskedBody := maskSensitiveJSON([]byte(reqBody))
				fields = append(fields, slog.String("req.body", truncate(string(maskedBody), opt.MaxBodySize)))
			}
			if opt.LogResponseBody {
				if r.Header.Get("Content-Type") == "application/json" {
					fields = append(fields, slog.String("resp.body", rw.preview()))
				}
			}

			log.LogAttrs(context.Background(), slog.LevelInfo, "http request", fields...)
		})
	}
}

// --- Helper types ---

type respCapture struct {
	http.ResponseWriter
	status      int
	bytes       int64
	buf         bytes.Buffer
	maxBody     int
	captureBody bool
}

func (r *respCapture) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *respCapture) Write(p []byte) (int, error) {
	if r.captureBody && r.buf.Len() < r.maxBody {
		remain := r.maxBody - r.buf.Len()
		if len(p) <= remain {
			r.buf.Write(p)
		} else {
			r.buf.Write(p[:remain])
		}
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

func (r *respCapture) preview() string {
	s := r.buf.String()
	if r.buf.Len() >= r.maxBody {
		s += "...[truncated]"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "...[truncated]"
	}
	return s
}

// GetUserFromRequest extracts user info (claims) from session cookie in request headers.
func GetUserFromRequest(r *http.Request) jwt.MapClaims {
	cookie := r.Header.Get("Cookie")
	if !strings.Contains(cookie, "session=") {
		return nil
	}

	// Extract token string safely
	tokenString := strings.Split(strings.Split(cookie, "session=")[1], ";")[0]

	// Parse the JWT
	token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims
	}
	return nil
}

func maskSensitiveJSON(raw []byte) []byte {
	var data interface{}

	// Try to parse JSON
	if err := json.Unmarshal(raw, &data); err != nil {
		// If not JSON, return raw (or mark as plain text)
		return raw
	}

	maskRecursive(&data)

	// Re-encode JSON
	masked, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return masked
}

// Recursively mask sensitive keys
func maskRecursive(v *interface{}) {
	switch val := (*v).(type) {
	case map[string]interface{}:
		for k, vv := range val {
			if isSensitiveKey(k) {
				val[k] = "***SECRET***"
			} else {
				maskRecursive(&vv)
				val[k] = vv
			}
		}
	case []interface{}:
		for i := range val {
			maskRecursive(&val[i])
		}
	}
}

// Helper to detect sensitive field names
func isSensitiveKey(k string) bool {
	sensitiveKeys := []string{"password", "secret", "token", "apikey", "key"}
	for _, s := range sensitiveKeys {
		if bytes.EqualFold([]byte(k), []byte(s)) {
			return true
		}
	}
	return false
}
