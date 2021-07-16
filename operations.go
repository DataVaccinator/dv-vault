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
	rPlugin1 := make(map[string]interface{})
	rPlugin1["name"] = "search"
	rPlugin1["vendow"] = "DataVaccinator"
	rPlugin1["license"] = "AGPL"

	rPlugin2 := make(map[string]interface{})
	rPlugin2["name"] = "publish"
	rPlugin2["vendow"] = "DataVaccinator"
	rPlugin2["license"] = "AGPL"

	// Compile result
	rResult := make(map[string]interface{})
	rResult["time"] = GetCurrentDateTime()
	rResult["version"] = SERVER_VERSION
	rResult["plugins"] = []interface{}{rPlugin1, rPlugin2}
	return generateResult(c, rResult)
}

// doAdd implements the "add" api operation
func doAdd(c echo.Context, clientRequest map[string]interface{}, isPublish bool) error {
	data := GetString(clientRequest["data"], "")
	uid := GetString(clientRequest["uid"], "")
	sid := GetInt(clientRequest["sid"], 0)
	words := GetStringArray(clientRequest["words"], []string{})
	duration := GetInt(clientRequest["duration"], 0)

	if data == "" || sid == 0 {
		return generateError(c, DV_MISSING_PARAM, "Missing data")
	}
	if len(data) > 1024*1024 {
		return generateError(c, DV_INVALID_PARAMSIZE, "Data bigger than 1MB")
	}
	if isPublish && (duration < 1 || duration > 365) {
		return generateError(c, DV_INVALID_PARAMSIZE, "Invalid duration range")
	}

	var err error
	var vid string
	var sql string
	for try := 0; try < 4; try++ {
		vid = GenerateVID()
		if !isPublish {
			// ADD function
			sql = "INSERT INTO data (VID, PAYLOAD, PROVIDERID, CREATIONDATE) " +
				"VALUES ($1, $2, $3, NOW())"
			_, err = DB.Exec(sql, vid, data, sid)
		} else {
			// PUBLISH function
			sql = "INSERT INTO data (VID, PAYLOAD, PROVIDERID, CREATIONDATE, DURATION) " +
				"VALUES ($1, $2, $3, NOW(), $4)"
			_, err = DB.Exec(sql, vid, data, sid, duration)
		}
		if err != nil {
			var pge pgx.PgError
			errors.As(err, &pge) // need to cast to get error codes
			if pge.Code == "23505" {
				// Duplicate key error. This might happen every now and then.
				// Therefore, retry up to 4 times.
				continue
			}
			LogInternalf("Failed to store payload (add/publish) with SQL: [%v] Error: %v", sql, pge)
			return generateError(c, DV_INTERNAL_ERROR,
				"Failed to store payload. Contact our support.")
		}
		break
	}
	if err != nil {
		LogInternalf("Failed to generate/insert some unique VID (add/publish)")
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to store payload. Contact our support.")
	}

	// Add any possible search words (only add, not publish)
	if len(words) > 0 && !isPublish {
		if insertSearchWords(vid, words) != true {
			deleteOneVID(vid) // Rollback payload entry
			return generateError(c, DV_INTERNAL_ERROR,
				"Failed to commit words insert. Contact our support.")
		}
	}

	logType := LOG_TYPE_ADD
	if isPublish {
		logType = LOG_TYPE_PUBLISH
	}
	go DoLog(logType, sid, vid)

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
		LogInternalf("Failed to start transaction (delete). Error: %v", err)
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to start a new transaction. Contact our support.")
	}

	// Concat ANY() statement
	in := "'{" + strings.Join(vids, ",") + "}'"

	// First delete any possible search words.
	sql := `DELETE FROM search WHERE VID IN(
		      SELECT VID FROM data WHERE VID=ANY(` + in + `::bytes[]) AND PROVIDERID=$1
			)`
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		LogInternalf("Failed to delete searchwords (delete) with SQL: [%v] Error: %v", sql, err)
		return generateError(c, DV_INTERNAL_ERROR, "Failed to delete search words")
	}

	// Now delete the payload data.
	sql = "DELETE FROM data WHERE VID=ANY(" + in + "::bytes[]) AND PROVIDERID=$1"
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		LogInternalf("Failed to delete payload (delete) with SQL: [%v] Error: %v", sql, err)
		return generateError(c, DV_INTERNAL_ERROR, "Failed to delete")
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		LogInternalf("Failed to commit delete. Error: %v", err)
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
	duration := 0
	sql := "SELECT PROVIDERID, DURATION FROM data WHERE VID=$1 AND PROVIDERID=$2"
	DB.QueryRow(sql, vid, sid).Scan(&pid, &duration)
	if pid < 1 {
		return generateError(c, DV_VID_NOT_FOUND, "Entry with this VID not found")
	}
	if duration != 0 {
		return generateError(c, DV_INVALID_FOR_PUBLISHED,
			"Published entries are not allowed to update")
	}

	// Delete any search words.
	// Start transaction
	tx, err := DB.Begin()
	if err != nil {
		LogInternalf("Failed to start db transaction (update). Error: %v", err)
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to create new transaction. Contact our support.")
	}
	sql = "DELETE FROM search WHERE VID=$1"
	_, err = tx.Exec(sql, vid)
	if err != nil {
		tx.Rollback()
		LogInternalf("Failed to delete words (update). SQL: %v Error: %v", sql, err)
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to delete searchwords. Contact our support.")
	}

	// Update dataset
	sql = "UPDATE data SET PAYLOAD=$1 WHERE VID=$2"
	_, err = tx.Exec(sql, data, vid)
	if err != nil {
		tx.Rollback()
		LogInternalf("Failed to delete payload (update). SQL: %v Error: %v", sql, err)
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
		LogInternalf("Failed to commit update. Error: %v", err)
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
func doGet(c echo.Context, clientRequest map[string]interface{}, isPublish bool) error {
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
	// NOTE: PROVIDERID has to match. Published entried are not returned.
	in := "'{" + strings.Join(vids, ",") + "}'"
	sql := ""
	var rows *pgx.Rows
	var err error
	if isPublish == false {
		// function "get"
		sql = `SELECT VID, PAYLOAD FROM data 
		    	WHERE VID=ANY(` + in + `::bytes[]) AND 
					PROVIDERID=$1 AND DURATION < 1`
		rows, err = DB.Query(sql, sid)
	} else {
		// function "getpublished"
		sql = `SELECT VID, PAYLOAD FROM data 
		    	WHERE VID=ANY(` + in + `::bytes[]) AND 
					DURATION > 0`
		rows, err = DB.Query(sql)
	}
	if err != nil {
		LogInternalf("Failed to query (get) with SQL: %v Error: %v", sql, err)
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
			LogInternalf("Unexpected error while processing query result (get). Error: %v", err)
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
	sql := "SELECT t1.VID FROM search t1\n"
	where := ""
	for i, word := range words {
		if !ValidateSearchWord(word) {
			return generateError(c, DV_INVALID_ENCODING, "Invalid search word encoding")
		}
		if i > 0 {
			sql += fmt.Sprintf("INNER JOIN search t%d ON (t1.VID = t%d.VID)\n",
				i+1, i+1)
		}
		where += fmt.Sprintf("t%d.WORD LIKE '"+word+"%%'\n    AND ", i+1)
	}
	where = where[:len(where)-4] // remove last "AND "
	sql += " WHERE " + where     // concat with where conditions

	// Filter provider association by putting results in a sub-query
	// which filters for provider id (sub-query seems more efficient here).
	// This avoids later confusion while requesting all vids found.
	sql = "SELECT VID FROM data WHERE VID IN(\n" + sql +
		"\n) AND PROVIDERID=$1\n"
	rows, err := DB.Query(sql, sid)
	if err != nil {
		LogInternalf("Query error in search. SQL: %v Error: %v", sql, err)
		return generateError(c, DV_INTERNAL_ERROR,
			"Failed to query searchwords. Contact our support.")
	}
	defer rows.Close()

	var results = []string{}
	for rows.Next() {
		var vid pgtype.Varchar
		err = rows.Scan(&vid)
		if err != nil {
			LogInternalf("Unexpected error while processing search result (search). Error: %v", err)
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
