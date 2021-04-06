package main

/*
This package contains the functions that handle the protocol api
operations.
*/

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
	"github.com/labstack/echo/v4"
)

// doCheck implements the "check" api operation
func doCheck(c echo.Context, clientRequest map[string]interface{}) error {
	// Announce that "search" functionality is available
	rPlugin := make(map[string]interface{})
	rPlugin["name"] = "search"
	rPlugin["vendow"] = "DataVaccinator"
	rPlugin["license"] = "AGPL"

	// Compile result
	rResult := make(map[string]interface{})
	rResult["time"] = GetCurrentDateTime()
	rResult["version"] = SERVER_VERSION
	rResult["plugins"] = []interface{}{rPlugin}
	return generateResult(c, rResult)
}

// doAdd implements the "add" api operation
func doAdd(c echo.Context, clientRequest map[string]interface{}) error {
	data := GetString(clientRequest["data"], "")
	uid := GetString(clientRequest["uid"], "")
	sid := GetInt(clientRequest["sid"], 0)
	words := GetStringArray(clientRequest["words"], []string{})

	if data == "" || sid == 0 {
		return generateError(c, DV_MISSING_PARAM, "Missing data")
	}
	if len(data) > 1024*1024 {
		return generateError(c, DV_INVALID_PARAMSIZE, "Data bigger than 1MB")
	}

	var try = 0
retry:
	vid := GenerateVID()

	sql := "INSERT INTO dv.data (VID, PAYLOAD, PROVIDERID, CREATIONDATE) " +
		"VALUES ($1, $2, $3, NOW())"
	_, err := DB.Exec(sql, vid, data, sid)
	if err != nil {
		var pge pgx.PgError
		errors.As(err, &pge) // need to cast to get error codes
		if pge.Code == "23505" {
			// Duplicate key error. Thiy might happen every now and then.
			// Therefore, try up to 4 times.
			try = try + 1
			if try < 4 {
				goto retry
			}
			fmt.Println("Failed unique VID generation in 4 tries!")
		}

		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to store payload. Contact our support.")
	}

	// Add any possible search words
	if len(words) > 0 {
		if insertSearchWords(vid, words) != true {
			deleteOneVID(vid) // Rollback payload entry
			return generateError(c, DV_INTERNAL_ERROR,
				"Failed to commit words insert. Contact our support.")
		}
	}

	go DoLog(LOG_TYPE_ADD, sid, vid)

	// Compile result
	rResult := make(map[string]interface{})
	rResult["uid"] = uid
	rResult["vid"] = vid
	return generateResult(c, rResult)
}

// doDelete implements the "delete" api operation
func doDelete(c echo.Context, clientRequest map[string]interface{}) error {
	uid := GetString(clientRequest["uid"], "")
	sid := GetInt(clientRequest["sid"], 0)
	vidList := GetString(clientRequest["vid"], "")

	vids := strings.SplitN(vidList, " ", -1)
	if len(vids) < 1 {
		return generateError(c, DV_MISSING_PARAM, "No VID?")
	}
	// Validate all VIDs for validity (also security).
	vids = MakeUnique(vids) // Ensure there are no duplicates
	for _, v := range vids {
		if !ValidateVID(v) {
			return generateError(c, DV_VID_NOT_FOUND, "Invalid VID "+v)
		}
	}

	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to start a new transaction. Contact our support.")
	}

	// Concat ANY() statement
	in := "'{" + strings.Join(vids, ",") + "}'"

	// First delete any possible search words.
	sql := `DELETE FROM dv.search WHERE VID IN(
		      SELECT VID FROM dv.data WHERE VID=ANY(` + in + `::bytes[]) AND PROVIDERID=$1
			)`
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR, "Failed to delete search words")
	}

	// Now delete the payload data.
	sql = "DELETE FROM dv.data WHERE VID=ANY(" + in + "::bytes[]) AND PROVIDERID=$1"
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR, "Failed to delete")
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to commit deletions. Contact our support.")
	}

	go DoLog(LOG_TYPE_DELETE, sid, vidList)

	// Compile result
	rResult := make(map[string]interface{})
	rResult["uid"] = uid
	return generateResult(c, rResult)
}

// doUpdate implements the "update" api operation
func doUpdate(c echo.Context, clientRequest map[string]interface{}) error {
	data := GetString(clientRequest["data"], "")
	vid := GetString(clientRequest["vid"], "")
	uid := GetString(clientRequest["uid"], "")
	sid := GetInt(clientRequest["sid"], 0)
	words := GetStringArray(clientRequest["words"], []string{})

	if data == "" || sid == 0 {
		return generateError(c, DV_MISSING_PARAM, "Missing data")
	}
	if len(data) > 1024*1024 {
		return generateError(c, DV_INVALID_PARAMSIZE, "Data bigger than 1MB")
	}
	if !ValidateVID(vid) {
		return generateError(c, DV_VID_NOT_FOUND, "Invalid VID")
	}

	// Validate VID
	pid := 0
	sql := "SELECT PROVIDERID FROM dv.data WHERE VID=$1 AND PROVIDERID=$2"
	DB.QueryRow(sql, vid, sid).Scan(&pid)
	if pid < 1 {
		return generateError(c, DV_VID_NOT_FOUND, "Entry with this VID not found")
	}

	// Delete any search words.
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to create new transaction. Contact our support.")
	}
	sql = "DELETE FROM dv.search WHERE VID=$1"
	_, err = tx.Exec(sql, vid)
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to delete searchwords. Contact our support.")
	}

	// Update dataset
	sql = "UPDATE dv.data SET PAYLOAD=$1 WHERE VID=$2"
	_, err = tx.Exec(sql, data, vid)
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to update payload. Contact our support.")
	}

	// Insert new searchwords
	if len(words) > 0 {
		if insertSearchWords(vid, words) != true {
			tx.Rollback()
			return generateError(c, DV_INTERNAL_ERROR,
				"Failed to commit words update/insert. Contact our support.")
		}
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to update. Contact our support.")
	}

	go DoLog(LOG_TYPE_UPDATE, sid, vid)

	// Compile result
	rResult := make(map[string]interface{})
	rResult["uid"] = uid
	return generateResult(c, rResult)
}

// doGet implements the "get" api operation
func doGet(c echo.Context, clientRequest map[string]interface{}) error {
	uid := GetString(clientRequest["uid"], "")
	sid := GetInt(clientRequest["sid"], 0)
	vidList := GetString(clientRequest["vid"], "")

	vids := strings.SplitN(vidList, " ", -1)
	if len(vids) < 1 {
		return generateError(c, DV_MISSING_PARAM, "No VID?")
	}
	// Validate all VIDs for validity (also security).
	// Also, create a map of vids for later result completion.
	vidMap := make(map[string]interface{})
	for _, v := range vids {
		if !ValidateVID(v) {
			return generateError(c, DV_VID_NOT_FOUND, "Invalid VID "+v)
		}
		vidMap[v] = true
	}

	// Concat ANY() statement and build the select.
	in := "'{" + strings.Join(vids, ",") + "}'"
	sql := "SELECT VID, PAYLOAD FROM dv.data WHERE VID=ANY(" + in + "::bytes[]) AND PROVIDERID=$1"
	rows, err := DB.Query(sql, sid)
	if err != nil {
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to query. Contact our support.")
	}
	defer rows.Close()

	results := make(map[string]interface{})
	for rows.Next() {
		var vid pgtype.Varchar
		var payload pgtype.Varchar
		err = rows.Scan(&vid, &payload)
		if err != nil {
			fmt.Println("Unexpected error while processing query result. Contact support.")
			continue
		}
		dResult := make(map[string]interface{})
		dResult["status"] = "OK"
		dResult["data"] = payload.String
		results[vid.String] = dResult
		// Remove found entry from vidMap list.
		delete(vidMap, vid.String)
	}

	// All vids that remained in vidMap are missing ones.
	// Add this to the results.
	for k := range vidMap {
		dResult := make(map[string]interface{})
		dResult["status"] = "NOTFOUND"
		dResult["data"] = false
		results[k] = dResult
	}

	go DoLog(LOG_TYPE_GET, sid, vidList)

	// Compile result
	rResult := make(map[string]interface{})
	rResult["uid"] = uid
	rResult["data"] = results
	return generateResult(c, rResult)
}

// doSearch implements the "search" api operation
func doSearch(c echo.Context, clientRequest map[string]interface{}) error {
	sid := GetInt(clientRequest["sid"], 0)
	uid := GetString(clientRequest["uid"], "")
	wds := GetString(clientRequest["words"], "")
	words := strings.SplitN(wds, " ", -1)

	words = MakeUnique(words) // remove any duplicates

	if len(words) < 1 {
		return generateError(c, DV_MISSING_PARAM, "Missing words")
	}

	// Combine search query
	sql := "SELECT t1.VID FROM dv.search t1\n"
	where := ""
	for i, word := range words {
		if !ValidateSearchWord(word) {
			return generateError(c, DV_INVALID_ENCODING, "Invalid search word encoding")
		}
		if i > 0 {
			sql += fmt.Sprintf("INNER JOIN dv.search t%d ON (t1.VID = t%d.VID)\n",
				i+1, i+1)
		}
		where += fmt.Sprintf("t%d.WORD LIKE '"+word+"%%'\n    AND ", i+1)
	}
	where = where[:len(where)-4] // remove last "AND "
	sql += " WHERE " + where     // concat with where conditions

	// Filter provider association by putting results in a sub-query
	// which filters for provider id (sub-query seems more efficient here).
	// This avoids later confusion while requesting all vids found.
	sql = "SELECT VID FROM dv.data WHERE VID IN(\n" + sql +
		"\n) AND PROVIDERID=$1\n"
	rows, err := DB.Query(sql, sid)
	if err != nil {
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to query searchwords. Contact our support.")
	}
	defer rows.Close()

	var results = []string{}
	for rows.Next() {
		var vid pgtype.Varchar
		err = rows.Scan(&vid)
		if err != nil {
			fmt.Println("Unexpected error while processing search query result. Contact support.")
			continue
		}
		results = append(results, vid.String)
	}

	// Compile result
	rResult := make(map[string]interface{})
	rResult["uid"] = uid
	rResult["vids"] = results
	return generateResult(c, rResult)
}
