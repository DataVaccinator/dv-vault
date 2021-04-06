package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GetCurrentDateTime returns a string in the following format:
// "YYYY-MM-DD HH-MM-SS"
func GetCurrentDateTime() string {
	now := time.Now()
	return string(now.Format("2006-01-02 15:04:05"))
}

// GetString casts an unknown interface to return as string.
// Use this to cast json results without triggering panic in
// case the type does not match (eg received float64 instead of string).
func GetString(clientRequest interface{}, asDefault string) (res string) {
	if clientRequest == nil {
		return asDefault
	}
	switch v := clientRequest.(type) {
	case float64:
		res = strconv.FormatFloat(clientRequest.(float64), 'f', -1, 64)
	case float32:
		res = strconv.FormatFloat(float64(clientRequest.(float32)), 'f', -1, 32)
	case int:
		res = strconv.FormatInt(int64(clientRequest.(int)), 10)
	case int64:
		res = strconv.FormatInt(clientRequest.(int64), 10)
	case uint:
		res = strconv.FormatUint(uint64(clientRequest.(uint)), 10)
	case uint64:
		res = strconv.FormatUint(clientRequest.(uint64), 10)
	case uint32:
		res = strconv.FormatUint(uint64(clientRequest.(uint32)), 10)
	case json.Number:
		res = clientRequest.(json.Number).String()
	case string:
		res = clientRequest.(string)
	case []byte:
		res = string(v)
	default:
		res = asDefault
	}
	return
}

// GetInt casts an unknown interface to return as int.
// Use this to cast json results without triggering panic in
// case the type does not match (eg received float64 instead of int).
func GetInt(clientRequest interface{}, asDefault int) (res int) {
	if clientRequest == nil {
		return asDefault
	}
	val := reflect.ValueOf(clientRequest)
	switch clientRequest.(type) {
	case int, int8, int16, int32, int64:
		res = int(val.Int())
	case uint, uint8, uint16, uint32, uint64:
		res = int(val.Uint())
	case float64:
		res = int(clientRequest.(float64))
	case float32:
		res = int(clientRequest.(float32))
	case string:
		res, _ = strconv.Atoi(strings.TrimSpace(clientRequest.(string)))
	case []byte:
		res, _ = strconv.Atoi(strings.TrimSpace(string(clientRequest.([]byte))))
	case json.Number:
		var resInt64 int64
		resInt64, _ = clientRequest.(json.Number).Int64()
		res = int(resInt64)
	default:
		res, _ = strconv.Atoi(fmt.Sprintf("%v", clientRequest))
	}
	return
}

// GetStringArray casts an unknown interface to return as string array.
// Use this to cast json results without triggering panic.
func GetStringArray(clientRequest interface{}, asDefault []string) []string {
	if clientRequest == nil {
		return asDefault
	}

	switch clientRequest.(type) {
	case []interface{}:
		// need switch because only switch statement can compare
		// type like this!
		// fmt.Printf("words: %v", clientRequest)
	default:
		return asDefault
	}

	var res = []string{}
	for _, value := range clientRequest.([]interface{}) {
		s := GetString(value, "")
		res = append(res, s)
	}
	return res
}

// GenerateVID generates a new VID.
// Actually, it is a 128 bit random number in hex encoding.
func GenerateVID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("Can not create random numbers? Weird...")
	}
	return hex.EncodeToString(bytes)
}

// ValidateVID verifies if the given string is a valid VID string
func ValidateVID(vid string) bool {
	// must be 16 bytes from 0-9A-Fa-f
	match, _ := regexp.MatchString("^[A-Fa-f0-9]+$", vid)
	if !match || len(vid) != 32 {
		return false
	}
	return true
}

// MakeUnique ensures that every entry in a string
// splice/array is unique
func MakeUnique(names []string) []string {
	flag := make(map[string]bool)
	var uniqueNames []string
	for _, name := range names {
		if flag[name] == false {
			flag[name] = true
			uniqueNames = append(uniqueNames, name)
		}
	}
	// unique names collected
	return uniqueNames
}

// deleteOneVID deletes one entry from data with its vid
// No security check here. Used for rollback mainly.
func deleteOneVID(vid string) bool {
	_, err := DB.Exec("DELETE FROM dv.data WHERE VID=$1", vid)
	if err != nil {
		return false
	}
	return true
}

// insertSearchWords inserts the given words into the
// database and assigns them to the given vid.
// No validation! No cleanup!
func insertSearchWords(vid string, words []string) bool {
	// add search words, needs at least one word
	words = MakeUnique(words) // ensure there are no duplicates
	// make a fast batch insert of the words
	tx, err := DB.Begin()
	if err != nil {
		return false
	}
	for _, word := range words {
		tx.Exec("INSERT INTO dv.search (VID, WORD) VALUES($1, $2)", vid, word)
	}
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return false
	}
	return true
}

// ValidateSearchWord verifies if the given string is a valid search word
func ValidateSearchWord(vid string) bool {
	// must be 16 bytes from 0-9A-Fa-f
	match, _ := regexp.MatchString("^[A-Fa-f0-9]+$", vid)
	if !match || len(vid) < 2 {
		return false
	}
	return true
}
