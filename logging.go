package main

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx"
)

const (
	LOG_TYPE_ADD    = 0
	LOG_TYPE_GET    = 1
	LOG_TYPE_UPDATE = 2
	LOG_TYPE_DELETE = 3
	LOG_TYPE_ERROR  = 9
)

// DoLog creates an entry in the audit table.
// You can run it async using go command to not slow down operations.
func DoLog(logType int, provId int, message string) {
	sql := `INSERT INTO dv.audit (LOGTYPE, LOGDATE, PROVIDERID, LOGCOMMENT)
              VALUES($1, NOW(), $2, $3)`

	_, err := DB.Exec(sql, logType, provId, message)
	if err != nil {
		var pge pgx.PgError
		errors.As(err, &pge) // need to cast to get error codes
		fmt.Printf("WARNING: Failed to insert to log table!\nError: '%v' (%v)",
			pge.Message, pge.Code)
	}
}