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

func InitDatabase() bool {

	fmt.Print("Connect CockroachDB… ")

	// Set client connection
	var poolConfig pgx.ConnPoolConfig
	config, err := pgx.ParseConnectionString(cfg.ConnectionString)
	if err != nil {
		panic("Can not parse your connection string")
	}

	poolConfig.ConnConfig = config
	poolConfig.AcquireTimeout = time.Minute

	fmt.Print(config.Host + " ")

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
	row := DB.QueryRow("SELECT COUNT(*) FROM dv.provider").Scan(&w)
	if row != nil {
		panic("Test query to 'dv.providers' table failed. Maybe no entries?")
	}

	fmt.Printf("(maxConnections: %v) Done\n", maxConn)
	return true
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
// 1) updates the dv.nodes table by inserting/updating its entry
//    based on the IP address (nodeid).
// 2) deletes all dv.nodes entries older than 60 minutes (offline
//    nodes!)
// 3) selects the lowest nodeid from dv.nodes table
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

		sql := `UPSERT INTO dv.nodes(NODEID, LASTACTIVITY) VALUES($1, NOW())`
		_, err = DB.Exec(sql, IPVal)
		if err != nil {
			LogInternalf("Failed to add/update nodes entry: %v", err)
			continue
		}

		sql = `DELETE FROM dv.nodes 
					WHERE LASTACTIVITY < NOW() - INTERVAL '60 minutes'`
		_, err = DB.Exec(sql)
		if err != nil {
			LogInternalf("Failed to cleanup outdated nodes: %v", err)
			continue
		}

		sql = `SELECT MIN(NODEID) AS NODEID FROM dv.nodes`
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
			sql = `DELETE FROM dv.data
						WHERE DURATION > 0 AND
						NOW() > CREATIONDATE + CONCAT(DURATION::text, ' days')::INTERVAL`
		*/
		sql = `DELETE FROM dv.data
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
