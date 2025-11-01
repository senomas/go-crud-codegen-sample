package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"
	"unicode"
)

// RunSQLFile splits a SQL file into statements and executes them in one tx.
func RunSQLFile(ctx context.Context, dbn string, db *sql.DB, dir, filename string, reapply bool) {
	b, err := os.ReadFile(path.Join(dir, filename))
	if err != nil {
		slog.Error("read file", "filename", filename, "err", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(bytes.NewReader(b))
	for i := 0; i < 10 && scanner.Scan(); i++ {
		ln := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(ln, "-- DB:") {
			name := strings.TrimSpace(ln[6:])
			if name != dbn {
				fmt.Printf("Skipping %s for DB (%s) (%s)\n", filename, name, dbn)
				return
			}
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("begin transaction", "filename", filename, "err", err)
		os.Exit(1)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `LOCK TABLE app_migration IN EXCLUSIVE MODE`)
	if err != nil {
		slog.Error("lock app_migration table", "filename", filename, "err", err)
		os.Exit(1)
	}

	// calculate sha256 of file content
	h := hmac.New(sha256.New, []byte("mwui-migrate"))
	h.Write(b)
	hash := base64.RawStdEncoding.EncodeToString(h.Sum(nil))

	if strings.HasPrefix(path.Base(filename), "0000-00") {
		fmt.Printf("applying migration %s\n", filename)
	} else {
		rows, err := tx.Query(`SELECT file_hash FROM app_migration WHERE filename = $1`, filename)
		if err != nil {
			slog.Error("query migration", "filename", filename, "err", err)
			os.Exit(1)
		}
		defer rows.Close()
		if rows.Next() {
			var prevHash string
			err = rows.Scan(&prevHash)
			if err != nil {
				slog.Error("scan migration", "filename", filename, "err", err)
				os.Exit(1)
			}
			if subtle.ConstantTimeCompare([]byte(prevHash), []byte(hash)) == 1 {
				fmt.Printf("SKIP! migration %s already applied\n", filename)
				return // already applied
			}
			if reapply {
				fmt.Printf("migration %s changed, re-applying\n", filename)
			} else {
				slog.Error("migration already applied with different content, aborting", "filename", filename)
				os.Exit(1)
			}
		} else {
			fmt.Printf("applying migration %s\n", filename)
		}
	}

	stmts, err := splitSQL(string(b))
	if err != nil {
		slog.Error("split SQL", "filename", filename, "err", err)
		os.Exit(1)
	}
	if len(stmts) == 0 {
		return
	}

	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, s); err != nil {
			slog.Error("exec migration", "filename", filename, "query", s, "err", err)
			os.Exit(1)
		}
	}
	if !strings.HasPrefix(path.Base(filename), "0000-00") {
		_, err = tx.ExecContext(ctx, `INSERT INTO app_migration (filename, file_hash) VALUES ($1, $2)`, filename, hash)
		if err != nil {
			slog.Error("record migration", "filename", filename, "err", err)
			os.Exit(1)
		}
	}
	err = tx.Commit()
	if err != nil {
		slog.Error("commit migration", "filename", filename, "err", err)
		os.Exit(1)
	}
}

// splitSQL splits on top-level semicolons; understands quotes, dollar-quoted strings, and comments.
func splitSQL(src string) ([]string, error) {
	var out []string
	var sb strings.Builder

	inS, inD := false, false        // ' ' , " "
	inLine, inBlock := false, false // --, /* */
	var dollar string               // "", or "$tag$"
	r := []rune(src)

	for i := 0; i < len(r); i++ {
		c := r[i]

		// handle line/block comments
		if !inS && !inD && dollar == "" && !inLine && !inBlock {
			if c == '-' && i+1 < len(r) && r[i+1] == '-' {
				inLine = true
			} else if c == '/' && i+1 < len(r) && r[i+1] == '*' {
				inBlock = true
			}
		}

		if inLine {
			if c == '\n' {
				inLine = false
				sb.WriteRune(c)
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(r) && r[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}

		// dollar-quoted strings: $tag$ ... $tag$
		if !inS && !inD {
			if dollar == "" && c == '$' {
				// try to read $tag$
				j := i + 1
				for j < len(r) && (unicode.IsLetter(r[j]) || unicode.IsDigit(r[j]) || r[j] == '_') {
					j++
				}
				if j < len(r) && r[j] == '$' {
					dollar = string(r[i : j+1]) // include both $
					sb.WriteString(dollar)
					i = j
					continue
				}
			} else if dollar != "" && c == '$' {
				// possible end $tag$
				if i+len(dollar)-1 < len(r) && string(r[i:i+len(dollar)]) == dollar {
					sb.WriteString(dollar)
					i += len(dollar) - 1
					dollar = ""
					continue
				}
			}
		}

		// normal quotes
		if dollar == "" {
			if c == '\'' && !inD {
				inS = !inS
			} else if c == '"' && !inS {
				inD = !inD
			}
		}

		// top-level semicolon splits statements
		if c == ';' && !inS && !inD && dollar == "" {
			out = append(out, sb.String())
			sb.Reset()
			continue
		}

		sb.WriteRune(c)
	}
	if strings.TrimSpace(sb.String()) != "" {
		out = append(out, sb.String())
	}
	return out, nil
}
