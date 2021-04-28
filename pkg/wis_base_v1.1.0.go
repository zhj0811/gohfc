/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"github.com/zhj0811/gohfc/pkg/parseBlock"
)

type WisHandler struct {
	PeerConfMap      map[string]PeerConfig
	OrdererConf      OrdererConfig
	Mspids           string
	Pubkeys          string
	Prikeys          string
	CryptoFamilys    string
	Channeluuids     string
	ChaincodeName    string
	ChaincodeVersion string
	EventPeer        string
	OrderName        string
	Args             []string
	Ide              *Identity
	FaCli            *FabricClient
}

var wis_logger = logging.MustGetLogger("event")

func (w *WisHandler) Query() (*QueryResponse, error) {
	// 初始化
	err := w.Init(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Init Err : %s", err.Error())
	}
	// 处理数据
	cc, err := w.getChainCodeObj()
	if err != nil {
		return nil, err
	}

	peers := w.getPeers()

	fmt.Println(peers)
	fmt.Println(w.FaCli.Peers)

	qRes, err := w.FaCli.Query(*w.Ide, *cc, peers)
	if err != nil {
		wis_logger.Debug("Query Err = ", err.Error())
		return nil, err
	}
	return qRes[0], nil
}

func (w *WisHandler) Invoke() (*InvokeResponse, error) {

	err := w.Init(context.Background())
	if err != nil {
		return nil, fmt.Errorf("Init Err : %s", err.Error())
	}

	cc, err := w.getChainCodeObj()
	if err != nil {
		return nil, err
	}

	peers := w.getPeers()

	return w.FaCli.Invoke(*w.Ide, *cc, peers, w.OrderName)
}

func (w *WisHandler) ListenEventFullBlock(response chan<- EventBlockResponse) error {
	err := w.Init(context.Background())
	if err != nil {
		return fmt.Errorf("Init Err : %s", err.Error())
	}
	err = w.FaCli.ListenForFilteredBlock(context.Background(), *w.Ide, 0, w.EventPeer, w.Channeluuids, response)
	if err != nil {
		return err
	}

	return nil
}

func (w *WisHandler) ListenForFullBlock(response chan<- parseBlock.Block) error {
	err := w.Init(context.Background())
	if err != nil {
		return fmt.Errorf("Init Err : %s", err.Error())
	}

	err = w.FaCli.ListenForFullBlock(context.Background(), *w.Ide, -1, w.EventPeer, w.Channeluuids, response)
	if err != nil {
		return err
	}

	return nil
}

func (w *WisHandler) ListenForFullBlockByIndex(blockHeight uint64, response chan<- parseBlock.Block) error {
	err := w.Init(context.Background())
	if err != nil {
		return errors.WithMessage(err, "init failed")
	}

	err = w.FaCli.ListenForFullBlock(context.Background(), *w.Ide, int64(blockHeight), w.EventPeer, w.Channeluuids, response)
	if err != nil {
		return err
	}
	return nil
}

func (w *WisHandler) QueryWithClose() (*QueryResponse, error) {
	// 初始化
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := w.Init(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "init wishandler failed")
	}
	// 处理数据
	cc, err := w.getChainCodeObj()
	if err != nil {
		return nil, err
	}

	var peers []string
	for peer := range w.PeerConfMap {
		peers = append(peers, peer)
	}

	//fmt.Println(peers)
	//fmt.Println(w.FaCli.Peers)

	qRes, err := w.FaCli.Query(*w.Ide, *cc, peers)
	if err != nil {
		wis_logger.Debug("Query Err = ", err.Error())
		return nil, err
	}
	return qRes[0], nil
}

func (w *WisHandler) InvokeWithClose() (*InvokeResponse, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := w.Init(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "init wisHandler failed")
	}

	cc, err := w.getChainCodeObj()
	if err != nil {
		return nil, err
	}

	var peers []string
	for peer := range w.PeerConfMap {
		peers = append(peers, peer)
	}

	return w.FaCli.Invoke(*w.Ide, *cc, peers, w.OrderName)
}

func (w *WisHandler) Init(ctx context.Context) error {
	format := logging.MustStringFormatter("%{shortfile} %{time:2006-01-02 15:04:05.000} [%{module}] %{level:.4s} : %{message}")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)

	logging.SetBackend(backendFormatter).SetLevel(logging.DEBUG, "main")

	fabricClient := &FabricClient{}
	w.FaCli = fabricClient

	pubkey := findCurt(w.Pubkeys)
	prikey := findCurt(w.Prikeys)

	identity, err := loadCertFromFile(pubkey, prikey)
	if err != nil {
		wis_logger.Error("LoadCertFromFile err = ", err)
		return err
	}
	identity.MspId = w.Mspids
	w.Ide = identity

	var crypto CryptoSuite
	switch w.CryptoFamilys {
	case "ecdsa":
		cryptoConfig := CryptoConfig{
			Family:    w.CryptoFamilys,
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		}

		crypto, err = NewECCryptSuiteFromConfig(cryptoConfig)
		if err != nil {
			return err
		}

	case "gm":
		cryptoConfig := CryptoConfig{
			Family:    w.CryptoFamilys,
			Algorithm: "P256SM2",
			Hash:      "SM3",
		}

		crypto, err = NewECCryptSuiteFromConfig(cryptoConfig)
		if err != nil {
			return err
		}
	default:
		return ErrInvalidAlgorithmFamily
	}
	w.FaCli.Crypto = crypto

	peers := make(map[string]*Peer)
	for peerName, peerConf := range w.PeerConfMap {
		peer, err := NewPeerFromConfig(ctx, fabricClient.Channel, peerConf, crypto)
		if err != nil {
			return fmt.Errorf("Peer NewPeerFromConfig err :%s", err.Error())
		}
		peers[peerName] = peer
		w.FaCli.Peers = peers
	}

	if "" != w.OrderName {
		orderers := make(map[string]*Orderer)
		order, err := NewOrdererFromConfig(ctx, fabricClient.Channel, w.OrdererConf)
		if err != nil {
			return fmt.Errorf("Order NewOrdererFromConfig err : %s", err.Error())
		}
		orderers[w.OrderName] = order
		w.FaCli.Orderers = orderers
	}

	if "" != w.EventPeer {
		eventpeers := make(map[string]*Peer)
		eventpeer, err := NewPeerFromConfig(ctx, fabricClient.Channel, w.PeerConfMap[w.EventPeer], crypto)
		if err != nil {
			return fmt.Errorf("EventPeer NewPeerFromConfig err: %s", err.Error())
		}
		eventpeers[w.EventPeer] = eventpeer
		w.FaCli.EventPeers = eventpeers
	}

	return nil
}

func (w *WisHandler) getChainCodeObj() (*ChainCode, error) {
	channelid := w.Channeluuids
	chaincodeName := w.ChaincodeName
	chaincodeVersion := w.ChaincodeVersion
	mspId := w.Mspids
	if channelid == "" || chaincodeName == "" || mspId == "" {
		return nil, fmt.Errorf("channelid or ccname or ccver  or mspId is empty")
	}

	chaincode := ChainCode{
		ChannelId: channelid,
		Type:      ChaincodeSpec_GOLANG,
		Name:      chaincodeName,
		Version:   chaincodeVersion,
		Args:      w.Args,
	}

	return &chaincode, nil
}

func (w *WisHandler) getPeers() []string {
	var peers []string
	for peer := range w.PeerConfMap {
		peers = append(peers, peer)
	}
	return peers
}

func findCurt(path string) string {
	list, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("ReadDir : ", err)
		fmt.Println(path)
		wis_logger.Debug(err.Error())
		return ""
	}
	var file os.FileInfo
	for _, item := range list {
		if !item.IsDir() {
			if file == nil {
				file = item
			} else if item.ModTime().After(file.ModTime()) {
				file = item
			}
		}
	}
	return filepath.Join(path, file.Name())
}
