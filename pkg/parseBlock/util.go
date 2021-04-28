package parseBlock

import (
	"encoding/asn1"
	"fmt"
	"math"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/util"
)

type asn1Header struct {
	Number       int64
	PreviousHash []byte
	DataHash     []byte
}

// hash 获得当前block hash
func hash(b *common.BlockHeader) []byte {
	return util.ComputeSHA256(toBytes(b))
}

func toBytes(b *common.BlockHeader) []byte {
	asn1Header := asn1Header{
		PreviousHash: b.PreviousHash,
		DataHash:     b.DataHash,
	}
	if b.Number > uint64(math.MaxInt64) {
		panic(fmt.Errorf("Golang does not currently support encoding uint64 to asn1"))
	} else {
		asn1Header.Number = int64(b.Number)
	}
	result, err := asn1.Marshal(asn1Header)
	if err != nil {
		panic(err)
	}
	return result
}
