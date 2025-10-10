package ipfsdb

import (
	"bytes"

	shell "github.com/ipfs/go-ipfs-api"
	pb "github.com/nnlgsakib/wwfsdb/pkg/ipfsdb/proto"
	"google.golang.org/protobuf/proto"
)

const (
	// MaxRowsPerPage defines the maximum number of rows a single Page object can hold.
	MaxRowsPerPage = 100
)

// LoadPage loads a Page from IPFS by its CID.
func LoadPage(sh *shell.Shell, pageCID string) (*pb.Page, error) {
	data, err := sh.Cat(pageCID)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(data)

	var page pb.Page
	if err := proto.Unmarshal(buf.Bytes(), &page); err != nil {
		return nil, err
	}

	return &page, nil
}
