package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"example.com/app-api/util"
)

func main() {
	// define optional flags
	drop := flag.Bool("drop", false, "Drop the target (optional)")
	dbn := flag.String("db", "db", "database name")
	prefix := flag.String("prefix", "DB", "Environment variable prefix (optional)")
	reaply := flag.Bool("reapply", false, "Reapply changed migrations (optional)")
	pattern := flag.String("pattern", "", "Pattern to match (optional)")
	exclude := flag.String("exclude", "", "Pattern to match (optional)")

	// parse command line arguments
	flag.Parse()

	// handle logic
	if *drop {
		fmt.Println("Drop option enabled")
	}
	if *pattern != "" {
		fmt.Printf("Pattern provided: %s\n", *pattern)
	}
	if *exclude != "" {
		fmt.Printf("Exclude pattern provided: %s\n", *exclude)
	}

	if len(flag.Args()) != 1 {
		slog.Error("Required one parameter")
		os.Exit(1)
	}

	db := util.GetPostgreSqlConn(*prefix)
	defer db.Close()

	{
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
		defer cancel()

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			slog.Error("failed to begin transaction", "err", err)
			os.Exit(1)
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, `
  	CREATE TABLE IF NOT EXISTS app_migration (
      filename VARCHAR(255) NOT NULL PRIMARY KEY,
      processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			commit_id VARCHAR(1023),
      file_hash VARCHAR(1023)
  	)`)
		if err != nil {
			log.Fatalf("failed to create app_migration table: %v", err)
		}

		_, err = tx.ExecContext(ctx, `LOCK TABLE app_migration IN EXCLUSIVE MODE`)
		if err != nil {
			log.Fatalf("failed to create app_migration table: %v", err)
		}

		if *drop {
			_, err = tx.ExecContext(ctx, `DELETE FROM app_migration`)
			if err != nil {
				log.Fatalf("failed to clear app_migration table: %v", err)
			}
		}

		err = tx.Commit()
		if err != nil {
			log.Fatalf("failed to commit transaction: %v", err)
		}
	}

	dir := path.Join(flag.Args()[0])
	ents, err := os.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	dfs := []string{}
	fs := []string{}
	var includeRX, excludeRX *regexp.Regexp
	if *pattern != "" {
		includeRX, err = regexp.Compile(*pattern)
		if err != nil {
			log.Fatalf("failed to compile pattern (%s): %v", *pattern, err)
		}
	}
	if *exclude != "" {
		excludeRX, err = regexp.Compile(*exclude)
		if err != nil {
			log.Fatalf("failed to compile exclude pattern (%s): %v", *exclude, err)
		}
	}
	for _, fn := range ents {
		lfn := strings.ToLower(fn.Name())
		if !strings.HasSuffix(lfn, ".sql") {
			continue
		}
		if excludeRX != nil && excludeRX.MatchString(fn.Name()) {
			fmt.Printf("Skipping (exclude) %s\n", fn.Name())
			continue
		}
		if includeRX != nil && !includeRX.MatchString(fn.Name()) {
			fmt.Printf("Skipping (not include) %s\n", fn.Name())
			continue
		}
		if strings.HasSuffix(lfn, ".drop.sql") {
			dfs = append(dfs, fn.Name())
		} else if strings.HasSuffix(lfn, ".sql") {
			fs = append(fs, fn.Name())
		}
	}
	sort.Slice(dfs, func(i, j int) bool {
		return strings.ToLower(filepath.Base(dfs[i])) > strings.ToLower(filepath.Base(dfs[j]))
	})
	sort.Slice(fs, func(i, j int) bool {
		return strings.ToLower(filepath.Base(fs[i])) < strings.ToLower(filepath.Base(fs[j]))
	})

	if *drop {
		fmt.Println("\nDrop files to be processed:")
		for _, f := range dfs {
			RunSQLFile(context.Background(), *dbn, db, dir, f, *reaply)
		}
	}
	fmt.Println("\nMigration files to be processed:")
	for _, f := range fs {
		RunSQLFile(context.Background(), *dbn, db, dir, f, *reaply)
	}
	fmt.Printf("\nMigration completed successfully.\n\n")
}
