package model

import (
	"database/sql"

	"example.com/app-api/util"
)

type StoreImpl struct {
	db *sql.DB
}

func GetStore() Store {
	return &StoreImpl{
		db: util.GetPostgreSqlConn("DB"),
	}
}
