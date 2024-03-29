package main

// Q: Why are the passwords not encrypted/hashed?
// A: If the attacker gets the database with the passwords, he also
//    got the whole content. Thus, he also has all data.
//    By this, he no longer needs the passwords as he already has
//    all data. Therefore, we do not need to secure that here.
//    Also, consider it being much faster to not always calculating
//    bcrypt or sha2 hashed passwords for every request.

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"runtime"
	"strconv"
	"time"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
)

var DB *pgx.ConnPool
var DBHost string

func initDatabase() bool {
	// Set client connection
	var poolConfig pgx.ConnPoolConfig
	config, err := pgx.ParseConnectionString(cfg.ConnectionString)
	if err != nil {
		panic("Can not parse your connection string")
	}

	poolConfig.ConnConfig = config
	poolConfig.AcquireTimeout = time.Minute

	maxConn := cfg.MaxConnections
	if maxConn < 1 {
		// <1 is auto, which uses CPU cores (incl. hyperthreading) * 3
		// https://www.cockroachlabs.com/docs/v21.1/connection-pooling.html#sizing-connection-pools
		// "Many workloads perform best when the number of connections was
		// between 2 and 4 times the number of CPU cores in the cluster."
		maxConn = runtime.NumCPU() * 3
	}

	poolConfig.MaxConnections = maxConn

	// Connect to CockroachDB
	DB, err = pgx.NewConnPool(poolConfig)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		panic("Can not connect new pool to CockroachDB")
	}

	// Check the connection
	var w int
	err = DB.QueryRow("SELECT COUNT(*) FROM provider").Scan(&w)
	if err != nil {
		fmt.Printf("\n%v\n", err)
		panic("Test query to 'provider' table failed. Maybe no entries?")
	}

	// keep DB host in global variable for later use (eg in main() function)
	DBHost = config.Host

	return true
}

// shutdownDatabase closes all database connections for clean shutdown
func shutdownDatabase() {
	DB.Close()
}

// cleanupHeartBeat is called async to find and delete expired
// published entries in the database every hour.
//
// In order to prevent multiple vaccinator instances in a cluster
// calling the same deletion at the same time or to often,
// the following applies to make sure that only one instance is
// executing this cleanup stuff every hour.
//
// It does a loop every hour. There it
// 1) updates the nodes table by inserting/updating its entry
//    based on the IP address (nodeid).
// 2) deletes all nodes entries older than 60 minutes (offline
//    nodes!)
// 3) selects the lowest nodeid from nodes table
// 4) compares the lowest nodeid to its own nodeid
// 5a) if it has the lowest nodeid, it will do the cleanup
// 5b) if it does not have the smallest nodeid, it will do nothing
func cleanupHeartBeat() {
	IPVal, err := getMyIPVal()
	if err != nil {
		LogInternalf("Will not do background jobs because getMyIPVal() failed (%v).", err)
		return
	}
	if cfg.DebugMode > 0 {
		fmt.Println("My NODEID value is: " + strconv.Itoa(IPVal))
	}

	for range time.Tick(time.Hour) {
		// Do checks every hour

		sql := `UPSERT INTO nodes(NODEID, LASTACTIVITY) VALUES($1, NOW())`
		_, err = DB.Exec(sql, IPVal)
		if err != nil {
			LogInternalf("Failed to add/update nodes entry: %v", err)
			continue
		}

		sql = `DELETE FROM nodes 
					WHERE LASTACTIVITY < NOW() - INTERVAL '60 minutes'`
		_, err = DB.Exec(sql)
		if err != nil {
			LogInternalf("Failed to cleanup outdated nodes: %v", err)
			continue
		}

		sql = `SELECT MIN(NODEID) AS NODEID FROM nodes`
		var rows *pgx.Rows
		rows, err = DB.Query(sql)
		if err != nil {
			LogInternalf("Failed to get available nodes: %v", err)
			continue
		}

		rows.Next()

		var nodeId pgtype.Int8
		err = rows.Scan(&nodeId)
		if err != nil {
			LogInternalf("Failed to get available nodeid minimum value: %v", err)
			rows.Close()
			continue
		}
		rows.Close()

		// Compare the lowest IP from nodes table with my own IP
		dst := pgtype.Int8{Int: int64(IPVal), Status: pgtype.Present}
		if nodeId != dst {
			// I'm not the smallest node number
			if cfg.DebugMode > 0 {
				fmt.Printf("INFO: Someone else has to cleanup expired and published payloads.\n")
			}
			continue
		}

		// I have the smallest active IP from all nodes!
		// Thus, it's on me to cleanup things here!
		if cfg.DebugMode > 0 {
			fmt.Printf("Cleanup expired and published payloads.\n")
		}
		/*
			// slower version, but more easy to read
			sql = `DELETE FROM data
						WHERE DURATION > 0 AND
						NOW() > CREATIONDATE + CONCAT(DURATION::text, ' days')::INTERVAL`
		*/
		sql = `DELETE FROM data
					WHERE DURATION > 0 AND
					CAST(NOW() - CREATIONDATE AS INT) > DURATION * 86400`
		_, err = DB.Exec(sql)
		if err != nil {
			LogInternalf("Failed to delete published and expired data (cleanupHeartBeat): %v",
				err)
			continue
		}
	}
}

// getMyIPVal returns an integer number derived from my
// local outgoing IP address. It is maximum 10 digits.
func getMyIPVal() (int, error) {
	myIp := GetOutboundIP()
	if myIp == nil {
		return 0, errors.New("Failed to get my outgoing IP.")
	}
	r := net.ParseIP(myIp.String())
	if r == nil {
		return 0, errors.New("Failed to parse my outgoing IP.")
	}
	num := new(big.Int)
	num.SetBytes(r)

	myVal := num.Text(10)
	if len(myVal) > 10 {
		myVal = myVal[len(myVal)-10:]
	}
	return strconv.Atoi(myVal)
}

// keepAliveHeartBeat is called async to keep the DB connections
// to CockroachDB up and running. Without, we start getting errors
// like "write tcp 10.0.0.10:34678->10.0.0.10:26257: write: broken pipe"
func keepAliveHeartBeat() {
	for range time.Tick(time.Minute) {
		// Do checks every minute

		if cfg.DebugMode > 0 {
			fmt.Printf("Ping database connection.\n")
		}
		// Check database availability
		_, err := DB.Exec(";")
		if err != nil {
			fmt.Println("WARN: Database ping was not successful")
		}
	}
}
