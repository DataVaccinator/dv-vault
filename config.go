package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Configuration struct {
	ConnectionString string `json:"connectionString"`
	MaxConnections   int    `json:"maxConnections"`
	IP               string `json:"listenIP"`
	Port             int    `json:"listenPort"`
	LetsEncrypt      int    `json:"useLetsEncrypt"`
	DebugMode        int    `json:"debugMode"`
	IPExtractor      string `json:"IPExtractor"`
}

var cfg Configuration

func LoadConfig() {
	fmt.Print("Open config.json… ")
	file, err := os.Open("config.json")
	if err != nil {
		panic("Missing config.json")
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	cfg = Configuration{}
	err = decoder.Decode(&cfg)
	if err != nil {
		panic("Invalid config.json")
	}
	fmt.Println("Done")
}
