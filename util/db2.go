package util

import (
	"database/sql"
	"log/slog"
	"os"
	"strings"
	"time"
)

func GetDB2Conn(prefix string) *sql.DB {
	driver := "go_ibm_db"
	dsns := []string{}
	v := os.Getenv(prefix + "_HOST")
	if v != "" {
		dsns = append(dsns, "HOSTNAME="+v)
	} else {
		slog.Error("Environment variable for database host is not set", "var", prefix+"_HOST")
		os.Exit(1)
	}
	v = os.Getenv(prefix + "_PORT")
	if v != "" {
		dsns = append(dsns, "PORT="+v)
	} else {
		dsns = append(dsns, "PORT=50000")
	}
	v = os.Getenv(prefix + "_NAME")
	if v != "" {
		dsns = append(dsns, "DATABASE="+v)
	} else {
		slog.Error("Environment variable for database name is not set", "var", prefix+"_NAME")
		os.Exit(1)
	}
	v = os.Getenv(prefix + "_INSTANCE")
	if v != "" {
		dsns = append(dsns, "UID="+v)
	} else {
		dsns = append(dsns, "UID=db2inst1")
	}
	v = os.Getenv(prefix + "_PASSWORD")
	if v != "" {
		dsns = append(dsns, "PWD="+v)
	} else {
		slog.Error("Environment variable for database password is not set", "var", prefix+"_PASSWORD")
		os.Exit(1)
	}
	v = os.Getenv(prefix + "_SCHEMA")
	if v != "" {
		dsns = append(dsns, "CurrentSchema="+v)
	}
	dsn := strings.Join(dsns, ";")
	db, err := sql.Open(driver, dsn)
	if err != nil {
		for i := 0; i < 30 && err != nil; i++ {
			if i%5 == 0 {
				slog.Debug("Error opening db, retrying...", "error", err)
			}
			time.Sleep(2 * time.Second)
			db, err = sql.Open(driver, dsn)
			if err != nil {
				if strings.Contains(err.Error(), "USERNAME AND/OR PASSWORD INVALID") {
					slog.Error("Error opening db, invalid username or password", "error", err)
					os.Exit(1)
				}
			}
		}
		if err != nil {
			slog.Error("Error opening db", "error", err)
			os.Exit(1)
		}
	}
	err = db.Ping()
	if err != nil {
		for i := 0; i < 30 && err != nil; i++ {
			if i%5 == 0 {
				slog.Debug("Waiting for database to be ready (ping)...")
			}
			time.Sleep(2 * time.Second)
			err = db.Ping()
			if err != nil {
				if strings.Contains(err.Error(), "USERNAME AND/OR PASSWORD INVALID") {
					slog.Error("Error opening db, invalid username or password", "error", err)
					os.Exit(1)
				}
			}
		}
		if err != nil {
			slog.Error("Error pinging db", "error", err)
			os.Exit(1)
		}
	}
	return db
}
