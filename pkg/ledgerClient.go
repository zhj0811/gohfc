/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"strconv"

	"github.com/zhj0811/gohfc/pkg/parseBlock"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/pkg/errors"
)

type LedgerClientImpl struct {
	*sdkHandler
}

// queryByQscc 根据系统智能合约Qscc查询信息
// input: args: 请求信息，例如：args := []string{"GetChainInfo", channelName}（查询某一个通道信息）
//        channelName: 通道名字,通道为空时默认用配置文件中的通道
// output: []*QueryResponse: 返回信息，需要解析
func (lc *LedgerClientImpl) queryByQscc(args []string, channelName string) ([]*QueryResponse, error) {
	peerNames := getSendPeerName()
	if len(peerNames) == 0 {
		return nil, errors.New("cannot found peer address")
	}

	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}

	chaincode := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      args,
	}

	return lc.client.Query(*lc.identity, chaincode, []string{peerNames[0]})
}

// GetBlockByNumber 根据区块编号查询区块
// input: blockNum: 区块编号
//        channelName: 通道名称,通道为空时默认用配置文件中的通道
// output: *parseBlock.FilterBlock: 经过解析的区块
func (lc *LedgerClientImpl) GetBlockByNumber(blockNum uint64, channelName string) (*parseBlock.FilterBlock, error) {
	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}

	strBlockNum := strconv.FormatUint(blockNum, 10)
	args := []string{"GetBlockByNumber", channelName, strBlockNum}

	resps, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessage(err, "cannot got installed chaincode")
	} else if len(resps) == 0 {
		return nil, errors.New("no response")
	}

	if resps[0].Error != nil {
		return nil, resps[0].Error
	}

	data := resps[0].Response.Response.Payload
	block := new(common.Block)
	err = proto.Unmarshal(data, block)
	if err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}

	filterBlock := parseBlock.FilterParseBlock(block)

	return filterBlock, nil
}

// GetBlockHeight: 查询区块高度
// input: channelName: 通道名称,通道为空时默认用配置文件中的通道
func (lc *LedgerClientImpl) GetBlockHeight(channelName string) (uint64, error) {
	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}

	args := []string{"GetChainInfo", channelName}
	resps, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return 0, err
	} else if len(resps) == 0 {
		return 0, errors.New("no response")
	}

	if resps[0].Error != nil {
		return 0, resps[0].Error
	}

	data := resps[0].Response.Response.Payload
	chainInfo := new(common.BlockchainInfo)
	err = proto.Unmarshal(data, chainInfo)
	if err != nil {
		return 0, errors.WithMessage(err, "unmarshal block failed")
	}

	return chainInfo.Height, nil
}

// GetBlockHeightByEventPeer 向event peer发送请求，获得区块高度
// input: channelName: 通道名称,通道为空时默认用配置文件中的通道
func (lc *LedgerClientImpl) GetBlockHeightByEventPeer(channelName string) (uint64, error) {
	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}
	if eventPeer == "" {
		return 0, errors.New("event peername is empty")
	}

	args := []string{"GetChainInfo", channelName}
	chaincode := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      args,
	}

	resps, err := lc.client.queryByEvent(*lc.identity, chaincode, []string{eventPeer})
	if err != nil {
		return 0, err
	} else if len(resps) == 0 {
		return 0, errors.New("no response")
	}

	if resps[0].Error != nil {
		return 0, resps[0].Error
	}

	data := resps[0].Response.Response.Payload
	chainInfo := new(common.BlockchainInfo)
	err = proto.Unmarshal(data, chainInfo)
	if err != nil {
		return 0, errors.WithMessage(err, "unmarshal block failed")
	}

	return chainInfo.Height, nil
}

// GetBlockByTxID 根据transaction ID查询区块
// input: txid: 交易ID
//        channelName: 通道名称,通道为空时默认用配置文件中的通道
// output: *parseBlock.FilterBlock: 经过解析的区块
func (lc *LedgerClientImpl) GetBlockByTxID(txid string, channelName string) (*parseBlock.FilterBlock, error) {
	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}

	args := []string{"GetBlockByTxID", channelName, txid}

	resps, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessage(err, "can not get installed chaincodes")
	} else if len(resps) == 0 {
		return nil, errors.New("no response")
	}

	if resps[0].Error != nil {
		return nil, resps[0].Error
	}

	data := resps[0].Response.Response.Payload
	block := new(common.Block)
	err = proto.Unmarshal(data, block)
	if err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}

	filterBlock := parseBlock.FilterParseBlock(block)

	return filterBlock, nil
}

func (lc *LedgerClientImpl) GetFilterTxByTxID(txId string, channelName string) (*parseBlock.FilterTx, error) {
	if channelName == "" {
		channelName = lc.client.Channel.ChannelId
	}

	args := []string{"GetBlockByTxID", channelName, txId}

	resps, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessage(err, "can not get installed chaincodes")
	} else if len(resps) == 0 {
		return nil, errors.New("no response")
	}

	if resps[0].Error != nil {
		return nil, resps[0].Error
	}

	data := resps[0].Response.Response.Payload
	block := new(common.Block)
	err = proto.Unmarshal(data, block)
	if err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}
	res := parseBlock.FilterParseTransaction(block, txId)
	return res, nil
}
