/*
Copyright: PeerFintech. All Rights Reserved.
*/

package parseBlock

import (
	"crypto/x509"
	"time"

	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	pb "github.com/hyperledger/fabric-protos-go/peer"
)

type Block struct {
	Header                *cb.BlockHeader     `json:"header,omitempty"`
	BlockTimeStamp        time.Time           `json:"block_time_stamp"`
	Transactions          []*Transaction      `json:"transactions,omitempty"`
	BlockCreatorSignature *SignatureMetadata  `json:"block_creator_signature,omitempty"`
	LastConfigBlockNumber *LastConfigMetadata `json:"last_config_block_number,omitempty"`
	TransactionFilter     []uint8             `json:"transaction_filter,omitempty"`
	OrdererKafkaMetadata  *OrdererMetadata    `json:"orderer_kafka_metadata,omitempty"`
	ChannelID             string              `json:"channel_id"`
	FirstTxTime           time.Time           `json:"first_tx_time"`
	Size                  uint64              `json:"size"`
	Config                string              `json:"config"` // json of config, use Config struct in config.go to json unmarshal
	Error                 error
}

type SignatureMetadata struct {
	SignatureHeader *SignatureHeader `json:"signature_header,omitempty"`
	Signature       []byte           `json:"signature,omitempty"`
}

type LastConfigMetadata struct {
	LastConfigBlockNum uint64             `json:"last_config_block_num,omitempty"`
	SignatureData      *SignatureMetadata `json:"signature_data,omitempty"`
}

type OrdererMetadata struct {
	LastOffsetPersisted uint64             `json:"last_offset_persisted,omitempty"`
	SignatureData       *SignatureMetadata `json:"signature_data,omitempty"`
}

type Transaction struct {
	Signature               []byte             `json:"signature,omitempty"`
	ChannelHeader           *ChannelHeader     `json:"channel_header,omitempty"`
	SignatureHeader         *SignatureHeader   `json:"signature_header,omitempty"`
	TxActionSignatureHeader *SignatureHeader   `json:"tx_action_signature_header,omitempty"`
	ChaincodeSpec           *ChaincodeSpec     `json:"chaincode_spec,omitempty"`
	Endorsements            []*Endorsement     `json:"endorsements,omitempty"`
	ProposalHash            []byte             `json:"proposal_hash,omitempty"`
	Events                  *pb.ChaincodeEvent `json:"events,omitempty"`
	Response                *pb.Response       `json:"response,omitempty"`
	NsRwset                 []*NsReadWriteSet  `json:"ns_read_write_Set,omitempty"`
	// Capture transaction validation code
	ValidationCode     uint8  `json:"validation_code"`
	ValidationCodeName string `json:"validation_code_name,omitempty"`
	Size               uint64 `json:"size"`
}

type ChannelHeader struct {
	Type        int32                      `json:"type,omitempty"`
	Version     int32                      `json:"version,omitempty"`
	Timestamp   *google_protobuf.Timestamp `json:"timestamp,omitempty"`
	ChannelId   string                     `json:"channel_id,omitempty"`
	TxId        string                     `json:"tx_id,omitempty"`
	Epoch       uint64                     `json:"epoch,omitempty"`
	ChaincodeId *pb.ChaincodeID            `json:"chaincode_id,omitempty"`
}
type ChaincodeSpec struct {
	Type        pb.ChaincodeSpec_Type `json:"type,omitempty"`
	ChaincodeId *pb.ChaincodeID       `json:"chaincode_id,omitempty"`
	Input       *ChaincodeInput       `json:"input,omitempty"`
	Timeout     int32                 `json:"timeout,omitempty"`
}

type ChaincodeInput struct {
	Args []string
}

type Endorsement struct {
	SignatureHeader *SignatureHeader `json:"signature_header,omitempty"`
	Signature       []byte           `json:"signature,omitempty"`
}

type SignatureHeader struct {
	Certificate *x509.Certificate
	MspId       string `json:"msp_id,omitempty"`
	Nonce       []byte `json:"nonce,omitempty"`
}

// NsReadWriteSet 读写集
type NsReadWriteSet struct {
	Namespace string           `json:"namespace,omitempty"`
	KVRWSet   *kvrwset.KVRWSet `json:"kVRWSet,omitempty"`
}

// FilterBlock 经过过滤的区块
type FilterBlock struct {
	TxNum           int                  //交易数量
	BlockNum        uint64               //区块编号
	BlockHash       []byte               //块哈希
	PreBlockHash    []byte               //前一个块哈希
	LastConfigBlock uint64               //最新的一个配置区块
	TxHash          []byte               //交易哈希
	Transactions    []*FilterTransaction `json:"transactions,omitempty"`
}

// FilterTransaction 交易读写集及验证码
type FilterTransaction struct {
	NsRwset            []*NsReadWriteSet `json:"ns_read_write_Set,omitempty"`
	ValidationCode     uint8             `json:"validation_code"`
	ValidationCodeName string            `json:"validation_code_name,omitempty"`
	Size               uint64            `json:"size"`
}
