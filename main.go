package main

/*-------------------------------------------------------+
| DataVaccinator Vault Provider System
| Copyright (C) DataVaccinator
| https://www.datavaccinator.com/
+--------------------------------------------------------+
| Author: Data Vaccinator Development Team
+--------------------------------------------------------+
| This program is released as free software under the
| Affero GPL license. You can redistribute it and/or
| modify it under the terms of this license which you
| can read by viewing the included agpl.txt or online
| at www.gnu.org/licenses/agpl.html. Removal of this
| copyright header is strictly prohibited without
| written permission from the original author(s).
+--------------------------------------------------------*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/acme/autocert"
)

var SERVER_VERSION string

func main() {
	if SERVER_VERSION == "" {
		SERVER_VERSION = "0.0.1-devel"
	}

	fmt.Println(" __                                 ")
	fmt.Println("|  \\ _ |_ _ \\  /_  _ _. _  _ |_ _  _ ")
	fmt.Println("|__/(_|| (_| \\/(_|(_(_|| )(_|| (_)|  ")
	fmt.Println("")
	fmt.Println("Starting DataVaccinator Vault server V" + SERVER_VERSION)

	LoadConfig() // stores it in global configuration object

	InitDatabase() // assign global DB object here

	DoLog(LOG_TYPE_ERROR, 0, "Started service")

	e := echo.New()
	if cfg.DebugMode > 0 {
		e.Debug = true
	}
	if cfg.LetsEncrypt > 0 {
		// Prepare Let's Encrypt usage (echo framework)
		if _, err := os.Stat("certs/"); os.IsNotExist(err) {
			// certs/ folder does not exists. Create it...
			err := os.Mkdir("certs/", 0755)
			if err != nil {
				panic("Can not create certs/ directory. Check permissions!")
			}
		}
		e.AutoTLSManager.Cache = autocert.DirCache("certs/")
	}

	e.HideBanner = true // hide the echo framework banner during start
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK,
			"Welcome to DataVaccinator Vault V"+SERVER_VERSION)
	})

	e.GET("/ping", func(c echo.Context) error {
		// TODO: Maybe check other values with relevance for the end user

		// Check database availability
		_, err := DB.Exec(";")
		if err != nil {
			return c.String(http.StatusServiceUnavailable, "Service Unavailable")
		}

		return c.String(http.StatusOK, "OK")
	})

	e.POST("/", protocolHandler)          // bind protocol handler
	e.POST("/index.php", protocolHandler) // bind protocol handler (legacy)

	if cfg.LetsEncrypt > 0 {
		e.Logger.Fatal(e.StartAutoTLS(":" + strconv.Itoa(cfg.Port)))
	} else {
		e.Logger.Fatal(e.Start(":" + strconv.Itoa(cfg.Port)))
	}
}

// protocolHandler is the main handler for all calls to index.php (legacy)
// and main calls.
func protocolHandler(c echo.Context) error {
	// retrieve form parameter "json"
	js := c.FormValue("json")
	if js == "" {
		return generateError(c, DV_MISSING_PARAM, "Missing json field")
	}

	var clientRequest map[string]interface{}
	err := json.Unmarshal([]byte(js), &clientRequest)
	if err != nil {
		return generateError(c, DV_INVALID_ENCODING, "Invalid JSON")
	}
	op := GetString(clientRequest["op"], "invalid")
	version := GetInt(clientRequest["version"], 0)
	if version != 2 {
		return generateError(c, DV_OUTDATED, "Only protocol version >= 2 supported!")
	}

	if cfg.DebugMode > 0 {
		fmt.Printf("%v REQUEST: %v\n", c.RealIP(), clientRequest)
	}

	// handle all supported operations which do not need a login
	switch op {
	case "check":
		return doCheck(c, clientRequest)
	}

	// check login credentials
	err = checkCredentials(c, clientRequest)
	if err != nil {
		return generateError(c, DV_INVALID_PARTNER, err.Error())
	}

	// handle all supported operations which need a login
	switch op {
	case "add":
		return doAdd(c, clientRequest)
	case "delete":
		return doDelete(c, clientRequest)
	case "update":
		return doUpdate(c, clientRequest)
	case "get":
		return doGet(c, clientRequest)
	case "search":
		return doSearch(c, clientRequest)
	}

	// default is an unknown or unsupported operation
	return generateError(c, DV_MISSING_PARAM, "Invalid operation")
}

// checkCredentials verifies the given sid and spwd parameters.
// It returns an error in case of failure.
// It returns nil in case of success.
func checkCredentials(c echo.Context, clientRequest map[string]interface{}) error {
	sid := GetInt(clientRequest["sid"], 0)
	spwd := GetString(clientRequest["spwd"], "")
	if spwd == "" {
		return errors.New("Invalid credentials") // wrong password
	}

	var pwd string = ""
	DB.QueryRow("SELECT password FROM dv.provider WHERE providerid=$1", sid).Scan(&pwd)
	if pwd != spwd {
		go DoLog(LOG_TYPE_ERROR, sid, "Wrong sid/spwd")
		return errors.New("Invalid credentials") // wrong password
	}

	return nil // success
}

// generateResult creates a DataVaccinator style result for return.
// Submit the fields in resultMap. No need to set status (always OK).
func generateResult(c echo.Context, resultMap map[string]interface{}) error {
	resultMap["status"] = "OK" // add generic OK for generic results
	j, err := json.Marshal(resultMap)
	if err != nil {
		panic("Error during JSON generation in generateResult.")
	}
	if cfg.DebugMode > 0 {
		fmt.Printf("%v RETURN RESULT: %v\n", c.RealIP(), string(j))
	}
	return c.String(http.StatusOK, string(j))
}

// generateError creates a DataVaccinator style error return.
// Use error codes from DV_x constants. errorDesc is free text to give
// the receiver some hint about the problem.
func generateError(c echo.Context, errorCode int, errorDesc string) error {

	// Determine which error type this code is (simplified version)
	status := "INVALID"
	httpType := http.StatusOK
	if errorCode == DV_INTERNAL_ERROR {
		status = "ERROR"
		httpType = http.StatusInternalServerError
	}

	type errorStruct struct {
		Status  string `json:"status"`
		Code    int    `json:"code"`
		Desc    string `json:"desc"`
		Version string `json:"version"`
	}

	lstMsg := errorStruct{Status: status,
		Code:    errorCode,
		Desc:    errorDesc,
		Version: SERVER_VERSION,
	}

	jRequest, err := json.Marshal(lstMsg)
	if err != nil {
		panic("Error during JSON generation in generateError.")
	}
	if cfg.DebugMode > 0 {
		fmt.Printf("%v RETURN ERROR: %v\n", c.RealIP(), string(jRequest))
	}
	return c.String(httpType, string(jRequest))
}
