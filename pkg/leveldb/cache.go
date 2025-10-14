package leveldb

import (
	"fmt"
)

func UpdateCache(dbName, cid string) {
	err := PutToCache([]byte(dbName), []byte(cid))
	if err != nil {
		fmt.Printf("Warning: could not write to cache: %v\n", err)
	}
}

func ReadCache(dbName string) (string, bool) {
	cid, err := GetFromCache([]byte(dbName))
	if err != nil {
		return "", false
	}
	return string(cid), true
}
