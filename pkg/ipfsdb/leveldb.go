package ipfsdb

import (
	"fmt"

	"github.com/syndtr/goleveldb/leveldb"
)

var db *leveldb.DB

func InitCache(path string) error {
	var err error
	db, err = leveldb.OpenFile(path, nil)
	if err != nil {
		return fmt.Errorf("failed to open leveldb: %w", err)
	}
	return nil
}

func CloseCache() {
	if db != nil {
		db.Close()
	}
}

func PutToCache(key, value []byte) error {
	if db == nil {
		return fmt.Errorf("cache not initialized")
	}
	return db.Put(key, value, nil)
}

func GetFromCache(key []byte) ([]byte, error) {
	if db == nil {
		return nil, fmt.Errorf("cache not initialized")
	}
	return db.Get(key, nil)
}

func DeleteFromCache(key []byte) error {
	if db == nil {
		return fmt.Errorf("cache not initialized")
	}
	return db.Delete(key, nil)
}
