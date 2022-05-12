package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
)

var flagPretty bool
var flagData string

// isManagement handles any commandline parameters. If a valid one
// is given, it will return false to make the main() function exit
// after calling.
// If no commandline param is given, it will simply return false.
// It will also execute the parameters then...
func isManagement() bool {
	// The first parameter is the name of the flag, the second is
	// the default value, and the third is the description of the flag.
	flag.BoolVar(&flagPretty, "p", false, "Pretty print JSON results")
	flag.StringVar(&flagData, "j", "", "JSON operation instructions like j='{\"op\":\"list\"}'")
	flag.Parse()

	if flagData == "" {
		return false
	}

	var request map[string]interface{}
	err := json.Unmarshal([]byte(flagData), &request)
	if err != nil {
		outError("Invalid or missing JSON data (-j)")
		return true
	}
	op := GetString(request["op"], "")

	if op == "" {
		outError("Missing op")
		return true
	}
	if op == "list" {
		opList()
		return true
	}
	if op == "add" {
		opAdd(request)
		return true
	}
	if op == "update" {
		opUpdate(request)
		return true
	}
	if op == "remove" {
		opRemove(request)
		return true
	}
	outError("Unknown or missing op parameter")
	return true
}

// opList does the list function
func opList() {
	sql := `SELECT providerid, name, description, ip, creationdate 
			FROM provider ORDER BY providerid`
	rows, err := DB.Query(sql)
	if err != nil {
		panic(fmt.Sprintf("Failed to query with SQL: %v Error: %v", sql, err))
	}
	defer rows.Close()

	results := make([]interface{}, 0)
	for rows.Next() {
		var sid pgtype.Int2
		var name pgtype.Varchar
		var description pgtype.Varchar
		var ip pgtype.Varchar
		var creationdate pgtype.Timestamptz
		err = rows.Scan(&sid, &name, &description, &ip, &creationdate)
		if err != nil {
			LogInternalf("Unexpected error while processing result (opList). Error: %v", err)
			continue
		}
		dLine := make(map[string]interface{})
		dLine["sid"] = sid.Int
		dLine["name"] = name.String
		dLine["desc"] = description.String
		dLine["ip"] = ip.String
		dLine["created"] = creationdate.Time
		results = append(results, dLine)
	}
	outResult(results)
}

// opAdd does the add function
func opAdd(request map[string]interface{}) {
	sid := GetInt(request["sid"], 0)
	name := GetString(request["name"], "")
	desc := GetString(request["desc"], "")
	pass := GetString(request["password"], "")
	ip := GetString(request["ip"], "")

	if name == "" || pass == "" || ip == "" {
		outError("Missing mandatory parameter (check name, pass, ip")
		return
	}
	if sid < 1 {
		outError("Invalid sid parameter")
		return
	}

	sql := "INSERT INTO provider (PROVIDERID, NAME, DESCRIPTION, PASSWORD, IP, CREATIONDATE) " +
		"VALUES ($1, $2, $3, $4, $5, NOW())"
	_, err := DB.Exec(sql, sid, name, desc, pass, ip)
	if err != nil {
		var pge pgx.PgError
		errors.As(err, &pge) // need to cast to get error codes
		if pge.Code == "23505" {
			outError("The sid you provided is allready in use!")
			return
		}
		LogInternalf("Failed to store new provider with SQL: [%v] Error: %v", sql, pge)
		outError("Failed to insert provider. Check your values!")
		return
	}
	outResult(nil)
}

// opUpdate does the add function
func opUpdate(request map[string]interface{}) {
	sid := GetInt(request["sid"], 0)
	name := GetString(request["name"], "--UNSET--")
	desc := GetString(request["desc"], "--UNSET--")
	pass := GetString(request["password"], "--UNSET--")
	ip := GetString(request["ip"], "--UNSET--")

	if sid < 1 {
		outError("Invalid sid parameter")
		return
	}

	type sqlExec struct {
		sql   string
		value interface{}
	}

	var sqlList []sqlExec

	if name != "--UNSET--" {
		var t = sqlExec{"UPDATE provider SET NAME=$2 WHERE PROVIDERID=$1", name}
		sqlList = append(sqlList, t)
	}
	if desc != "--UNSET--" {
		var t = sqlExec{"UPDATE provider SET DESCRIPTION=$2 WHERE PROVIDERID=$1", desc}
		sqlList = append(sqlList, t)
	}
	if pass != "--UNSET--" {
		var t = sqlExec{"UPDATE provider SET PASSWORD=$2 WHERE PROVIDERID=$1", pass}
		sqlList = append(sqlList, t)
	}
	if ip != "--UNSET--" {
		var t = sqlExec{"UPDATE provider SET IP=$2 WHERE PROVIDERID=$1", ip}
		sqlList = append(sqlList, t)
	}

	for _, command := range sqlList {
		ctag, err := DB.Exec(command.sql, sid, command.value)
		if err != nil {
			var pge pgx.PgError
			errors.As(err, &pge) // need to cast to get error codes
			LogInternalf("Failed to store new provider with SQL: [%v] Error: %v", command.sql, pge)
			outError("Failed to update provider. Check your values!")
			return
		}
		if ctag.RowsAffected() != 1 {
			outError("Failed to update provider entry. Check your sid.")
			return
		}
	}

	outResult(nil)
}

// opRemove does the remove function
func opRemove(request map[string]interface{}) {
	sid := GetInt(request["sid"], 0)
	force := GetBool(request["force"], false)
	if sid < 1 {
		outError("Invalid sid parameter")
		return
	}

	if !force {
		fmt.Printf("Do you really want to delete all data of service provider %d?\n", sid)
		conf := askForConfirmation("The deletion is final! Delete now?")
		if !conf {
			outError("Cancelled")
			return
		}
	}

	// start transaction
	tx, err := DB.Begin()
	if err != nil {
		panic(fmt.Sprintf("Failed to start transaction (remove). Error: %v", err))
	}

	// delete search words
	sql := `DELETE FROM search WHERE VID IN(
				SELECT VID FROM data WHERE providerid = $1
			)`
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		panic(fmt.Sprintf("Failed to delete searchwords (remove) with SQL: [%v] Error: %v", sql, err))
	}

	// delete VID entries
	sql = `DELETE FROM data WHERE providerid = $1`
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		panic(fmt.Sprintf("Failed to delete payloads (remove) with SQL: [%v] Error: %v", sql, err))
	}

	// delete service provider entry
	sql = `DELETE FROM provider WHERE providerid = $1`
	_, err = tx.Exec(sql, sid)
	if err != nil {
		tx.Rollback()
		panic(fmt.Sprintf("Failed to delete sid entry (remove) with SQL: [%v] Error: %v", sql, err))
	}

	// commit transaction
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		panic(fmt.Sprintf("Failed to commit remove statements. Error: %v", err))
	}

	outResult(nil)
}

// outResult outputs a result JSON after successful processing
// It will add "status":"OK" and put the results in "data" field.
// Submit nil for results to skip the "data" field.
func outResult(results interface{}) {
	rResult := make(map[string]interface{})
	rResult["status"] = "OK"
	if results != nil {
		rResult["data"] = results
	}

	var j []byte
	var err error
	if flagPretty {
		j, err = json.MarshalIndent(rResult, "", "  ")
	} else {
		j, err = json.Marshal(rResult)
	}
	if err != nil {
		panic("Error during JSON generation in outResult.")
	}
	fmt.Printf("%s\n", j)
}

// outError outputs a result JSON after failed processing
// It will add "status":"FAILURE" and put the description in "desc" field.
func outError(description string) {
	rResult := make(map[string]interface{})
	rResult["status"] = "FAILURE"
	rResult["desc"] = description

	var j []byte
	var err error
	if flagPretty {
		j, err = json.MarshalIndent(rResult, "", "  ")
	} else {
		j, err = json.Marshal(rResult)
	}
	if err != nil {
		panic("Error during JSON generation in outError.")
	}
	fmt.Printf("%s\n", j)
}
