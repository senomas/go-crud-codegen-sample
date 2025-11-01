package model

import (
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var reDuplicate = regexp.MustCompile(`duplicate key value violates unique constraint "([a-zA-Z0-9_]+)"`)

type ErrorDuplicate struct {
	Table     string
	Constaint string
	Cols      []string
	Msg       string
}

func (e *ErrorDuplicate) Error() string {
	if len(e.Cols) > 0 {
		return fmt.Sprintf("duplicate value for %s (constraint=%s) (%s)", e.Table, e.Constaint, strings.Join(e.Cols, ", "))
	}
	return fmt.Sprintf("duplicate value in %s (constraint=%s)", e.Table, e.Constaint)
}

func (e *ErrorDuplicate) process(db *sql.DB) {
	rows, err := db.Query(`
		SELECT
		    rel.relname AS table_name,
		    att.attname AS column_name
		FROM pg_constraint con
		JOIN pg_class rel ON rel.oid = con.conrelid
		JOIN pg_attribute att ON att.attrelid = rel.oid AND att.attnum = ANY(con.conkey)
		WHERE con.conname = $1`, e.Constaint)
	if err != nil {
		slog.Error("Error querying duplicate constraint columns", "constraint", e.Constaint, "err", err)
		return
	}
	defer func() {
		_ = rows.Close()
	}()
	e.Cols = []string{}
	if rows.Next() {
		var colName string
		err = rows.Scan(&e.Table, &colName)
		if err != nil {
			slog.Error("Error scanning duplicate constraint column", "constraint", e.Constaint, "err", err)
			return
		}
		e.Cols = append(e.Cols, colName)
	} else {
		slog.Error("No columns found for duplicate constraint", "constraint", e.Constaint)
	}
}

func insertError(db *sql.DB, msg string, err error, args ...any) error {
	if res := reDuplicate.FindStringSubmatch(err.Error()); res != nil {
		if len(res) == 2 {
			edup := &ErrorDuplicate{
				Constaint: res[1],
				Msg:       err.Error(),
			}
			edup.process(db)
			return edup
		}
	}
	nargs := append(args, slog.Any("Error", err))
	slog.Error(msg, nargs...)
	return err
}

func updateError(db *sql.DB, msg string, err error, args ...any) error {
	if res := reDuplicate.FindStringSubmatch(err.Error()); res != nil {
		if len(res) == 2 {
			edup := &ErrorDuplicate{
				Constaint: res[1],
				Msg:       err.Error(),
			}
			edup.process(db)
			return edup
		}
	}
	nargs := append(args, slog.Any("Error", err))
	slog.Error(msg, nargs...)
	return err
}

func updateInsertError(err error) error {
	return err
}

func updateDeleteError(err error) error {
	return err
}

func deleteError(err error) error {
	return err
}
