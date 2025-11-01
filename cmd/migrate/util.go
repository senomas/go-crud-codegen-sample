package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

func getDB() (*sql.DB, error) {
	driver := "go_ibm_db"
	dsns := []string{}
	v := os.Getenv("DB_HOST")
	if v != "" {
		dsns = append(dsns, "HOSTNAME="+v)
	} else {
		dsns = append(dsns, "HOSTNAME=localhost")
	}
	v = os.Getenv("DB_PORT")
	if v != "" {
		dsns = append(dsns, "PORT="+v)
	} else {
		dsns = append(dsns, "PORT=50000")
	}
	v = os.Getenv("DBNAME")
	if v != "" {
		dsns = append(dsns, "DATABASE="+v)
	} else {
		dsns = append(dsns, "DATABASE=mwui")
	}
	v = os.Getenv("DB_INSTAANCE")
	if v != "" {
		dsns = append(dsns, "UID="+v)
	} else {
		dsns = append(dsns, "UID=db2inst1")
	}
	v = os.Getenv("DB2INST1_PASSWORD")
	if v != "" {
		dsns = append(dsns, "PWD="+v)
	} else {
		dsns = append(dsns, "PWD=pwd")
	}
	dsn := strings.Join(dsns, ";")
	db, err := sql.Open(driver, dsn)
	if err != nil {
		for i := 0; i < 60 && err != nil; i++ {
			if i%5 == 0 {
				fmt.Println("Waiting for database to be ready...")
			}
			time.Sleep(2 * time.Second)
			db, err = sql.Open(driver, dsn)
		}
		if err != nil {
			return nil, fmt.Errorf("Error opening db: %w", err)
		}
	}
	err = db.Ping()
	if err != nil {
		for i := 0; i < 60 && err != nil; i++ {
			if i%5 == 0 {
				fmt.Println("Waiting for database to be ready (ping)...")
			}
			time.Sleep(2 * time.Second)
			err = db.Ping()
		}
		if err != nil {
			return nil, fmt.Errorf("Error opening db: %w", err)
		}
	}
	return db, err
}
