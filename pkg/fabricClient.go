package gohfc

import (
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/pkg/errors"
)

type FabricNetworkClientImpl struct {
	*sdkHandler
}

const (
	deploy  = "deploy"
	upgrade = "upgrade"
)

func (fc *FabricNetworkClientImpl) CreateUpdateChannel(path, channelId string) error {
	orderName := getSendOrderName()
	if orderName == "" {
		return errors.New("config orderer is err")
	}
	return fc.client.CreateUpdateChannel(*fc.identity, path, channelId, orderName)
}

func (fc *FabricNetworkClientImpl) JoinChannel(channelId, userName, peerName string) (*PeerResponse, error) {
	orderName := getSendOrderName()
	if orderName == "" {
		return nil, errors.New("config orderer is err")
	}
	id, ok := users[userName]
	if !ok {
		return nil, errors.New("user not exist")
	}
	return fc.client.JoinChannel(*id, channelId, peerName, orderName)
}

func (fc *FabricNetworkClientImpl) InstallChainCode(req *InstallRequest, userName, peerName string) (*PeerResponse, error) {
	orderName := getSendOrderName()
	if orderName == "" {
		return nil, errors.New("config orderer is err")
	}
	id, ok := users[userName]
	if !ok {
		return nil, errors.New("user not exist")
	}
	return fc.client.InstallChainCode(*id, req, peerName)
}

func (fc *FabricNetworkClientImpl) UpdateAnchorPeer(userName, path, channelId string) error {
	orderName := getSendOrderName()
	if orderName == "" {
		return errors.New("config orderer is err")
	}
	id, ok := users[userName]
	if !ok {
		return errors.New("user not exist")
	}

	return fc.client.CreateUpdateChannel(*id, path, channelId, orderName)
}

func (fc *FabricNetworkClientImpl) InstantiateChainCode(req *ChainCode, policy string) (*orderer.BroadcastResponse, error) {
	peerNames := getSendPeerName()
	orderName := getSendOrderName()
	if len(peerNames) == 0 || orderName == "" {
		return nil, errors.New("config peer orderer is err")
	}
	return fc.client.InstantiateChainCode(*fc.identity, req, []string{peerNames[0]}, orderName, deploy, policy, nil)
}
