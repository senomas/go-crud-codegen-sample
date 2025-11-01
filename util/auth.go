package util

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	rnd "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"example.com/app-api/util/jsql"
	"golang.org/x/crypto/argon2"
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

func HashPassword(value jsql.Secret) (string, error) {
	if !value.Valid {
		return "", nil
	}
	salt := make([]byte, hashParam.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(value.String),
		salt, hashParam.Iterations,
		hashParam.Memory,
		hashParam.Parallelism,
		hashParam.KeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	phc := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		hashParam.Memory,
		hashParam.Iterations,
		hashParam.Parallelism,
		b64Salt,
		b64Hash)
	return phc, nil
}

func VerifyPassword(value string, password jsql.Secret) (bool, error) {
	if !password.Valid {
		return false, nil
	}
	if password.String == "" {
		return false, nil
	}
	parts := strings.Split(password.String, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		slog.Warn("invalid password format", "password", password.String)
		return false, errors.New("invalid PHC format")
	}
	if parts[2] != "v=19" {
		slog.Warn("unsupported argon2 version", "version", password.String)
		return false, errors.New("unsupported argon2 version")
	}

	var mem, iters uint32
	var par uint8
	for _, kv := range strings.Split(parts[3], ",") {
		k, v, _ := strings.Cut(kv, "=")
		switch k {
		case "m":
			u, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return false, err
			}
			mem = uint32(u)
		case "t":
			u, err := strconv.ParseUint(v, 10, 32)
			if err != nil {
				return false, err
			}
			iters = uint32(u)
		case "p":
			u, err := strconv.ParseUint(v, 10, 8)
			if err != nil {
				return false, err
			}
			par = uint8(u)
		}
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	got := argon2.IDKey([]byte(value), salt, iters, mem, par, uint32(len(want)))
	ok := subtle.ConstantTimeCompare(got, want) == 1
	return ok, nil
}

var curve = ecdh.P256()

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rnd.Intn(len(letters))]
	}
	return string(b)
}

func SetHMAC(req *http.Request, body []byte, shared []byte) {
	nonce := randomString(32)
	req.Header.Set("X-Req-Nonce", nonce)
	reqTime := time.Now().Format(time.RFC3339)
	req.Header.Set("X-Req-Timestamp", reqTime)
	mac := hmac.New(sha256.New, []byte(nonce))
	mac.Write(body)
	bodyHash := base64.RawStdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set("X-Body-Hash", bodyHash)
	mac = hmac.New(sha256.New, []byte(shared))
	mac.Write([]byte(nonce))
	mac.Write([]byte(";"))
	mac.Write([]byte(reqTime))
	mac.Write([]byte(";"))
	mac.Write([]byte(req.Method))
	mac.Write([]byte(";"))
	mac.Write([]byte(req.URL.RequestURI()))
	mac.Write([]byte(";"))
	mac.Write([]byte(bodyHash))
	req.Header.Set("X-Req-Signature", base64.RawStdEncoding.EncodeToString(mac.Sum(nil)))
}

func EncodePubKey(pub *ecdh.PublicKey) (string, error) {
	raw := pub.Bytes() // uncompressed: 0x04 || X || Y
	if len(raw) != 65 || raw[0] != 4 {
		return "", fmt.Errorf("unexpected pubkey length")
	}
	x := raw[1:33]
	y := raw[33:65]
	return base64.RawURLEncoding.EncodeToString(x) + "," + base64.RawURLEncoding.EncodeToString(y), nil
}

func DecodePubKey(pubXY string) (*ecdh.PublicKey, error) {
	xy := strings.SplitN(pubXY, ",", 2)
	if len(xy) != 2 {
		return nil, fmt.Errorf("invalid pubkey format")
	}
	x, err := base64.RawURLEncoding.DecodeString(xy[0])
	if err != nil {
		return nil, fmt.Errorf("invalid pubkey x: %w", err)
	}
	y, err := base64.RawURLEncoding.DecodeString(xy[1])
	if err != nil {
		return nil, fmt.Errorf("invalid pubkey y: %w", err)
	}
	raw := make([]byte, 65)
	raw[0] = 4
	copy(raw[1:33], x)
	copy(raw[33:65], y)
	return curve.NewPublicKey(raw)
}
