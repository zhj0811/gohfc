/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"google.golang.org/grpc/grpclog"
)

//sdk handler
type sdkHandler struct {
	client          *FabricClient
	identity        *Identity
	discoveryConfig *DiscoveryConfig
}

type SdkImpl struct {
	ChannelClient
	LedgerClient
	EventClient
	FabricNetworkClient
}

const defaultUser = "_default"

var (
	handler         sdkHandler
	orgPeerMap      = make(map[string][]string)
	users           = make(map[string]*Identity)
	orderNames      []string
	peerNames       []string
	eventPeer       string
	orRulePeerNames []string
)

//New initialize Fabric sdk from config
func New(configPath string) (Sdk, error) {
	clientConfig, err := newClientConfig(configPath)
	if err != nil {
		return nil, errors.WithMessage(err, "create config failed")
	}
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, os.Stdout, os.Stderr))

	handler.client, err = newFabricClientFromConfig(*clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "new fabric client failed")
	}
	if err := NewUsers(clientConfig.Users); err != nil {
		return nil, errors.WithMessage(err, "create users failed")
	}
	defaultIdentity, ok := users[defaultUser]
	if !ok {
		return nil, errors.New("no '_default' user")
	}
	handler.identity = defaultIdentity

	if err = parsePolicy(); err != nil {
		return nil, errors.WithMessage(err, "parsePolicy failed")
	}

	handler.discoveryConfig = clientConfig.Discovery

	return &SdkImpl{
		&ChannelClientImpl{&handler},
		&LedgerClientImpl{&handler},
		&EventClientImpl{&handler},
		&FabricNetworkClientImpl{&handler},
	}, nil
}

// NewEventClient 从结构体初始化eventclient句柄
func NewEventClient(ctx context.Context, config *EventConfig) (EventClient, error) {
	p, err := newPeerFromPeerByteConfig(ctx, &config.PeerByteConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Peer failed")
	}

	id, err := newIdentityFromUserByteConfig(&config.UserByteConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	crypto, err := NewECCryptSuiteFromConfig(config.CryptoConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	return &EventProvider{CryptoSuite: crypto, Peer: p, Identity: id}, nil
}

//NewFabricClientHandler 生成FabricClientHandler句柄，适应于wischain使用
func NewFabricClientHandler(ctx context.Context, config ClientHandlerConfig) (FabricClientHandler, error) {
	p, err := newPeerFromPeerByteConfig(ctx, config.PeerByteConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Peer failed")
	}

	e, err := newEndorsers(ctx, config.EndorsersConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Endorsers failed")
	}

	or, err := newOrdererFromOrdererByteConfig(ctx, config.OrdererByteConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Orderer failed")
	}

	id, err := newIdentityFromUserByteConfig(config.UserByteConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}
	var crypto CryptoSuite
	if config.CryptoConfig == nil {
		crypto = defaultCryptSuite
	} else {
		crypto, err = NewECCryptSuiteFromConfig(*config.CryptoConfig)
		if err != nil {
			return nil, errors.WithMessage(err, "create CryptoSuite failed")
		}
	}

	return &ClientHandler{CryptoSuite: crypto, Peer: p, Orderer: or, Identity: id, Endorsers: e}, nil
}
