/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import "fmt"

type ChannelClientImpl struct {
	*sdkHandler
}

// Invoke invoke cc ,if channelName ,chaincodeName is nil that use by client_sdk.yaml set value
func (cc *ChannelClientImpl) Invoke(args []string, transientMap map[string][]byte, channelName, chaincodeName string) (*InvokeResponse, error) {
	peerNames := getSendPeerName()
	orderName := getSendOrderName()
	if len(peerNames) == 0 || orderName == "" {
		return nil, fmt.Errorf("config peer order is err")
	}
	chaincode, err := getChainCodeObj(args, transientMap, channelName, chaincodeName)
	if err != nil {
		return nil, err
	}
	return cc.client.Invoke(*cc.identity, *chaincode, peerNames, orderName)
}

// Query query cc  ,if channelName ,chaincodeName is nil that use by client_sdk.yaml set value
func (cc *ChannelClientImpl) Query(args []string, transientMap map[string][]byte, channelName, chaincodeName string) ([]*QueryResponse, error) {
	peerNames := getSendPeerName()
	if len(peerNames) == 0 {
		return nil, fmt.Errorf("config peer order is err")
	}
	chaincode, err := getChainCodeObj(args, transientMap, channelName, chaincodeName)
	if err != nil {
		return nil, err
	}

	return cc.client.Query(*cc.identity, *chaincode, []string{peerNames[0]})
}
