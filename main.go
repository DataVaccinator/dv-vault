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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

var SERVER_VERSION string

var e *echo.Echo
var s http.Server

func main() {
	if SERVER_VERSION == "" {
		SERVER_VERSION = "0.0.1-devel"
	}

	fmt.Println(" __                                 ")
	fmt.Println("|  \\ _ |_ _ \\  /_  _ _. _  _ |_ _  _ ")
	fmt.Println("|__/(_|| (_| \\/(_|(_(_|| )(_|| (_)|  ")
	fmt.Println("")
	fmt.Println("Starting DataVaccinator Vault server V" + SERVER_VERSION)

	loadConfig() // stores it in global configuration object

	initDatabase() // assign global DB object here

	go cleanupHeartBeat() // start background task for DB cleanup

	// handle OS signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cleanupDV()
		os.Exit(0)
	}()

	// create echo framework handle
	e = echo.New()

	if cfg.Port < 1024 && cfg.RunAs != "" {
		// start background task for degrading root privileges
		go degradePrivileges(e, cfg.RunAs)
	}

	if cfg.DebugMode > 0 {
		e.Debug = true
		e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
			Format: "${remote_ip} method=${method}, uri=${uri}, status=${status}\n",
		}))
		fmt.Println("Debug-Mode is activated.")
	}

	switch strings.ToUpper(cfg.IPExtractor) {
	case "XFF":
		e.IPExtractor = echo.ExtractIPFromXFFHeader()
		fmt.Println("Determine IP by using X-Forwared-For header")
	case "REALIP":
		e.IPExtractor = echo.ExtractIPFromRealIPHeader()
		fmt.Println("Determine IP by using X-Real-IP header")
	default:
		e.IPExtractor = echo.ExtractIPDirect()
	}

	certsFolder := ""
	if cfg.LetsEncrypt > 0 {
		// Prepare Let's Encrypt usage (echo framework)
		certsFolder = cfg.CertFolder
		if certsFolder == "" {
			certsFolder = "certs" // Use default for saving certs
		}
		// Ensure last slash, make path absolute
		certsFolder = filepath.Clean(certsFolder) + "/"
		// Check if it exists. Create if needed.
		if _, err := os.Stat(certsFolder); os.IsNotExist(err) {
			// Given certs folder does not exist. Create it...
			fmt.Printf("Create missing certificate folder [%v]...\n", certsFolder)
			err := os.Mkdir(certsFolder, 0770) // 'rwxrwx---'
			if err != nil {
				panic("Can not create certs directory at [" + certsFolder + "]. Check permissions!")
			}
			fmt.Println("⇨ DONE")
			if cfg.RunAs != "" {
				if chown(certsFolder, cfg.RunAs) == false {
					panic("Failed chown on [" + certsFolder + "]. Check permissions!")
				}
			}
		}
	}

	if cfg.DisableIPCheck != 0 {
		fmt.Println("WARNING: IP-Check disabled! Do not use in production!")
	}

	if cfg.CORSDomains != "" {
		// Enable CORS (https://fetch.spec.whatwg.org/)
		domains := strings.Split(cfg.CORSDomains, ",")
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: domains,
			AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost},
		}))
		fmt.Printf("NOTE: Enabled CORS domains for \"%v\"\n", cfg.CORSDomains)
	}

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubdomains; preload")
			c.Response().Header().Add("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Add("X-Frame-Options", "SAMEORIGIN")
			c.Response().Header().Add("X-Content-Type-Options", "nosniff")
			c.Response().Header().Add("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
			c.Response().Header().Add("Pragma", "no-cache")
			c.Response().Header().Add("Server", "dv-vault")
			return next(c)
		}
	})

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

	DoLog(LOG_TYPE_NOTICE, 0, "Started service")

	serverAddress := cfg.IP + ":" + strconv.Itoa(cfg.Port)
	var sErr error
	if cfg.LetsEncrypt > 0 {
		// use own TLS server because echo standard uses TLS 1.0 and 1.2 and
		// allows usage of unsecure ciphers
		autoTLSManager := autocert.Manager{
			Prompt: autocert.AcceptTOS,
			// Cache certificates to avoid issues with rate limits
			Cache: autocert.DirCache(certsFolder),
		}
		// generate server with minimum TLS 1.3, using autocert.Manager
		s = http.Server{
			Addr:    serverAddress,
			Handler: e, // set Echo as handler
			TLSConfig: &tls.Config{
				GetCertificate: autoTLSManager.GetCertificate,
				NextProtos:     []string{acme.ALPNProto},
				MinVersion:     tls.VersionTLS13,
			},
		}
		fmt.Println("Start TLS service on " + serverAddress)
		sErr = s.ListenAndServeTLS("", "")
	} else {
		sErr = e.Start(serverAddress)
	}
	if sErr != nil {
		fmt.Printf("%v\n", sErr)
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

	// check login credentials
	err = checkCredentials(c, clientRequest)
	if err != nil {
		return generateError(c, DV_INVALID_PARTNER, err.Error())
	}

	// handle all supported operations which need a login
	switch op {
	case "check":
		return doCheck(c, clientRequest)
	case "add":
		return doAdd(c, clientRequest, false)
	case "publish":
		return doAdd(c, clientRequest, true)
	case "delete":
		return doDelete(c, clientRequest)
	case "update":
		return doUpdate(c, clientRequest)
	case "get":
		return doGet(c, clientRequest, false)
	case "getpublished":
		return doGet(c, clientRequest, true)
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
	if spwd == "" || sid < 1 {
		return errors.New("Invalid credentials")
	}

	clientIP := c.RealIP()
	var pwd string = ""
	var allowedIP string = ""
	sql := "SELECT password,ip FROM dv.provider WHERE providerid=$1"
	DB.QueryRow(sql, sid).Scan(&pwd, &allowedIP)
	if cfg.DisableIPCheck == 0 && !strings.Contains(allowedIP, clientIP) {
		go DoLog(LOG_TYPE_ERROR, sid, "Not allowed IP client address "+clientIP)
		return errors.New("Not allowed IP client address")
	}
	if pwd != spwd {
		go DoLog(LOG_TYPE_ERROR, sid, "Wrong sid/spwd")
		return errors.New("Invalid credentials")
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

// degradePrivileges waits until ListenerAddr is set and then
// tries to degrade the user of this process to the given user.
func degradePrivileges(e *echo.Echo, userName string) {
	for {
		adr := e.ListenerAddr()
		if adr != nil {
			degradeMe(userName)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// cleanupDV provides a clean shutdown of this tool
func cleanupDV() {
	DoLog(LOG_TYPE_NOTICE, 0, "Received stop signal. Stopping service.")
	if s.Handler != nil {
		err := s.Close() // close TLS framework (net connections)
		if err != nil {
			fmt.Printf("Failed closing TLS network connections. [%v]\n", err)
		}
	}
	err := e.Close() // close echo framework (net connections)
	if err != nil {
		fmt.Printf("Failed closing echo network connections. [%v]\n", err)
	}
	shutdownDatabase() // close database handles
	fmt.Println("DataVaccinator stopped regularily")
}
