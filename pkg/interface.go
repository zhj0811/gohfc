/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"os"

	"github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	lb "github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/zhj0811/gohfc/pkg/parseBlock"

	"github.com/hyperledger/fabric-protos-go/orderer"
)

type Sdk interface {
	ChannelClient
	LedgerClient
	EventClient
	FabricNetworkClient
}

type ChannelClient interface {
	Invoke(args []string, transientMap map[string][]byte, channelName, chaincodeName string) (*InvokeResponse, error)
	Query(args []string, transientMap map[string][]byte, channelName, chaincodeName string) ([]*QueryResponse, error)
	//QueryTransaction(identity Identity, channelId string, txId string, peers []string) ([]*QueryTransactionResponse, error) {
}

type LedgerClient interface {
	// GetBlockByNumber 根据区块编号查询区块
	// input: blockNum: 区块编号
	//        channelName: 通道名称,通道为空时默认用配置文件中的通道
	// output: *parseBlock.FilterBlock: 经过解析的区块
	GetBlockByNumber(blockNum uint64, channelName string) (*parseBlock.FilterBlock, error)
	// GetBlockHeight: 查询区块高度
	// input: channelName: 通道名称,通道为空时默认用配置文件中的通道
	GetBlockHeight(channelName string) (uint64, error)
	// GetBlockHeightByEventPeer 向event peer发送请求，获得区块高度
	// input: channelName: 通道名称,通道为空时默认用配置文件中的通道
	GetBlockHeightByEventPeer(channelName string) (uint64, error)
	// GetBlockByTxID 根据transaction ID查询区块
	// input: txid: 交易ID
	//        channelName: 通道名称,通道为空时默认用配置文件中的通道
	// output: *parseBlock.FilterBlock: 经过解析的区块
	GetBlockByTxID(txid string, channelName string) (*parseBlock.FilterBlock, error)
	GetFilterTxByTxID(txId string, channelName string) (*parseBlock.FilterTx, error)
}

type EventClient interface {
	ListenEventFullBlock(channelName string, startNum int64) (chan parseBlock.Block, error)
	ListenEventFilterBlock(channelName string, startNum int64) (chan EventBlockResponse, error)
}

type FabricNetworkClient interface {
	CreateUpdateChannel(path, channelId string) error
	JoinChannel(channelId, userName, peerName string) (*PeerResponse, error)
	UpdateAnchorPeer(userName, path, channelId string) error
	InstallChainCode(req *InstallRequest, userName, peerName string) (*PeerResponse, error)
	InstantiateChainCode(req *ChainCode, policy string) (*orderer.BroadcastResponse, error)
}

type FabricClientHandler interface {
	//CreateUpdateChannel 创建通道，更新锚节点
	CreateUpdateChannel(env []byte, channelId string) error
	JoinChannel(channelId string) error
	InstallCCByPack(data []byte) error
	//InstallCCByCodeTar 仅支持golang
	InstallCCByCodeTar(fr *os.File, req *InstallRequest) error
	//operation: - upgrade - deploy
	InstantiateChainCode(req *ChainCode, operation, policy string) error
	//GetLastConfigBlock 获取最新配置块
	GetLastConfigBlock(channelID string) (*common.Block, error)
	//peer lifecycle chaincode install pkg.tar.gz
	LifecycleInstall(pkg string) (*lb.InstallChaincodeResult, error)
	//peer lifecycle chaincode approveformyorg
	ApproveForMyOrg(channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) error
	//peer lifecycle chaincode commit
	LifecycleCommit(channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) error
	InvokeOrQuery(args []string, transientMap map[string][]byte, channelName, chaincodeName string, isInit, isQuery bool) (*pb.ProposalResponse, error)
}
