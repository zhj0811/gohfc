package parseBlock

import (
	"github.com/hyperledger/fabric-protos-go/common"
	utils "github.com/hyperledger/fabric/protoutil"
)

type FilterTx struct {
	BlockNum  uint64 //区块编号
	Timestamp int64  `protobuf:"bytes,3,opt,name=timestamp,proto3" json:"timestamp,omitempty"` //秒
}

func FilterParseTransaction(block *common.Block, txId string) *FilterTx {
	res := &FilterTx{}
	res.BlockNum = block.Header.Number
	for _, data := range block.Data.Data {
		localTransaction := &FilterTransaction{}
		localTransaction.Size = uint64(len(data))
		envelope, err := utils.GetEnvelopeFromBlock(data)
		if err != nil {
			parseBlockLogger.Errorf("Error getting envelope: %s\n", err)
			continue
		}

		payload, err := utils.UnmarshalPayload(envelope.Payload)
		if err != nil {
			parseBlockLogger.Errorf("Error getting payload from envelope: %s\n", err)
			continue
		}
		chHeader, err := utils.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			parseBlockLogger.Errorf("Error unmarshaling channel header: %s\n", err)
			continue
		}
		if txId == chHeader.TxId {
			res.Timestamp = chHeader.Timestamp.Seconds
			return res
		}
	}
	return res
}
