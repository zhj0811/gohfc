/*
Copyright: PeerFintech. All Rights Reserved.
*/

package parseBlock

import (
	"encoding/binary"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset"
	"github.com/hyperledger/fabric-protos-go/ledger/rwset/kvrwset"
	"github.com/hyperledger/fabric-protos-go/peer"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// FilterParseBlock 解析并过滤区块
// input: *common.Block: 经查询得到的区块
// output: *FilterBlock: 经过解析并过滤的区块
func FilterParseBlock(block *common.Block) *FilterBlock {
	var localBlock FilterBlock

	localBlock.TxNum = len(block.Data.Data)
	localBlock.BlockNum = block.Header.Number
	localBlock.TxHash = block.Header.DataHash
	localBlock.BlockHash = hash(block.Header)
	localBlock.PreBlockHash = block.Header.PreviousHash
	localBlock.LastConfigBlock = binary.LittleEndian.Uint64(getValueFromBlockMetadata(block, common.BlockMetadataIndex_LAST_CONFIG))
	transactionFilter := newTxValidationFlags(len(block.Data.Data))

	txBytes := getValueFromBlockMetadata(block, common.BlockMetadataIndex_TRANSACTIONS_FILTER)
	for index, b := range txBytes {
		transactionFilter[index] = b
	}

	for txIndex, data := range block.Data.Data {
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

		if common.HeaderType(chHeader.Type) == common.HeaderType_ENDORSER_TRANSACTION {
			transaction := &peer.Transaction{}
			if err := proto.Unmarshal(payload.Data, transaction); err != nil {
				parseBlockLogger.Errorf("Error unmarshalling transaction: %s\n", err)
				continue
			}
			chaincodeAction, err := utils.GetActionFromEnvelopeMsg(envelope)
			if err != nil {
				parseBlockLogger.Errorf("Error getting payloads from transaction actions: %s\n", err)
				continue
			}

			txReadWriteSet := &rwset.TxReadWriteSet{}
			if err := proto.Unmarshal(chaincodeAction.Results, txReadWriteSet); err != nil {
				parseBlockLogger.Errorf("Error unmarshalling chaincode action results: %s\n", err)
				continue
			}

			if len(chaincodeAction.Results) != 0 {
				for _, nsRwset := range txReadWriteSet.NsRwset {
					nsReadWriteSet := &NsReadWriteSet{}
					kvRWSet := &kvrwset.KVRWSet{}
					nsReadWriteSet.Namespace = nsRwset.Namespace
					if err := proto.Unmarshal(nsRwset.Rwset, kvRWSet); err != nil {
						parseBlockLogger.Errorf("Error unmarshaling tx read write set: %s\n", err)
						continue
					}
					nsReadWriteSet.KVRWSet = kvRWSet
					localTransaction.NsRwset = append(localTransaction.NsRwset, nsReadWriteSet)
				}
			}

			addFilterTransactionValidation(localTransaction, txIndex, transactionFilter)

			localBlock.Transactions = append(localBlock.Transactions, localTransaction)
		} else if common.HeaderType(chHeader.Type) == common.HeaderType_CONFIG {
			parseBlockLogger.Debugf("it's config block number : %d", block.Header.Number)
		}

	}

	return &localBlock
}

// addFilterTransactionValidation 添加验证信息
func addFilterTransactionValidation(tran *FilterTransaction, txIdx int, tf []uint8) error {
	if len(tf) > txIdx {
		tran.ValidationCode = tf[txIdx]
		tran.ValidationCodeName = peer.TxValidationCode_name[int32(tran.ValidationCode)]
		return nil
	}
	return errors.Errorf("invalid index or transaction filler. Index: %d", txIdx)
}
