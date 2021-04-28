/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"fmt"

	"github.com/zhj0811/gohfc/pkg/parseBlock"
)

type EventClientImpl struct {
	*sdkHandler
}

// if channelName ,chaincodeName is nil that use by client_sdk.yaml set value
func (ec *EventClientImpl) ListenEventFullBlock(channelName string, startNum int64) (chan parseBlock.Block, error) {
	if len(channelName) == 0 {
		channelName = ec.client.Channel.ChannelId
	}
	if channelName == "" {
		return nil, fmt.Errorf("ListenEventFullBlock channelName is empty ")
	}
	ch := make(chan parseBlock.Block)
	err := ec.client.ListenForFullBlock(context.Background(), *ec.identity, startNum, eventPeer, channelName, ch)
	if err != nil {
		return nil, err
	}

	return ch, nil
}

// if channelName ,chaincodeName is nil that use by client_sdk.yaml set value
func (ec *EventClientImpl) ListenEventFilterBlock(channelName string, startNum int64) (chan EventBlockResponse, error) {
	if len(channelName) == 0 {
		channelName = ec.client.Channel.ChannelId
	}
	if channelName == "" {
		return nil, fmt.Errorf("ListenEventFilterBlock  channelName is empty ")
	}
	ch := make(chan EventBlockResponse)
	err := ec.client.ListenForFilteredBlock(context.Background(), *ec.identity, startNum, eventPeer, channelName, ch)
	if err != nil {
		return nil, err
	}

	return ch, nil
}
