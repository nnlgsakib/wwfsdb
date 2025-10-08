package ipfsdb

import (
	"fmt"
	"os"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"google.golang.org/protobuf/proto"
)

// publishAsync updates the IPNS record for a database in the background
func publishAsync(sh *shell.Shell, dbName, cid string) {
	go func() {
		if err := Publish(sh, dbName, cid); err != nil {
			fmt.Fprintf(os.Stderr, "Error publishing to IPNS: %v\n", err)
		}
	}()
}

// Publish updates the IPNS record for a database
func Publish(sh *shell.Shell, dbName, cid string) error {
	entryData, err := GetFromCache([]byte("registry:" + dbName))
	if err != nil {
		return fmt.Errorf("database %s not found in registry", dbName)
	}

	var entry pb.RegistryEntry
	if err := proto.Unmarshal(entryData, &entry); err != nil {
		return err
	}

	_, err = sh.PublishWithDetails(cid, entry.KeyName, 0, 0, false)
	return err
}

