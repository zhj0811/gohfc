package gohfc

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"time"

	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
	"github.com/zhj0811/gohfc/pkg/parseBlock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

type EventConfig struct {
	CryptoConfig
	PeerByteConfig
	UserByteConfig
}

type PeerByteConfig NodeConfig

type NodeConfig struct {
	Url        string
	UseTLS     bool
	TlsBytes   []byte
	DomainName string
}

type UserByteConfig struct {
	CertBytes []byte
	KeyBytes  []byte
	MspID     string
}

type EventProvider struct {
	CryptoSuite
	*Peer
	*Identity
}

func newEndorsers(ctx context.Context, conf []*PeerByteConfig) ([]*Peer, error) {
	if conf == nil {
		return nil, nil
	}
	var peers []*Peer
	for _, peerConfig := range conf {
		p, err := newPeerFromPeerByteConfig(ctx, peerConfig)
		if err != nil {
			return nil, errors.WithMessage(err, "create Peer failed")
		}
		peers = append(peers, p)
	}
	return peers, nil
}

//newPeerFromPeerByteConfig 从结构体初始化Peer
func newPeerFromPeerByteConfig(ctx context.Context, conf *PeerByteConfig) (*Peer, error) {
	if conf == nil {
		return nil, nil
	}
	p := Peer{Uri: conf.Url, Name: conf.DomainName}
	if !conf.UseTLS {
		p.Opts = []grpc.DialOption{grpc.WithInsecure()}
	} else {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(conf.TlsBytes) {
			return nil, errors.Errorf("credentials: failed to append certificates with conf.TlsBytes for peer %s", conf.DomainName)
		}
		creds := credentials.NewClientTLSFromCert(cp, conf.DomainName)
		p.Opts = append(p.Opts, grpc.WithTransportCredentials(creds))
	}

	p.Opts = append(p.Opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(1) * time.Minute,
			Timeout:             time.Duration(20) * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize)))

	conn, err := grpc.DialContext(ctx, p.Uri, p.Opts...)
	if err != nil {
		return nil, fmt.Errorf("connect host=%s failed, err:%s\n", p.Uri, err.Error())
	}
	p.client = peer.NewEndorserClient(conn)

	return &p, nil
}

//newOrdererFromOrdererByteConfig 从结构体初始化Orderer
func newOrdererFromOrdererByteConfig(ctx context.Context, conf *OrdererByteConfig) (*Orderer, error) {
	if conf == nil {
		return nil, nil
	}
	or := Orderer{Uri: conf.Url, Name: conf.DomainName}
	if !conf.UseTLS {
		or.Opts = []grpc.DialOption{grpc.WithInsecure()}
	} else {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(conf.TlsBytes) {
			return nil, errors.Errorf("credentials: failed to append certificates with conf.TlsBytes for peer %s", conf.DomainName)
		}
		creds := credentials.NewClientTLSFromCert(cp, conf.DomainName)
		or.Opts = append(or.Opts, grpc.WithTransportCredentials(creds))
	}

	or.Opts = append(or.Opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(1) * time.Minute,
			Timeout:             time.Duration(20) * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize)))

	conn, err := grpc.DialContext(ctx, or.Uri, or.Opts...)
	if err != nil {
		return nil, fmt.Errorf("connect host=%s failed, err:%s\n", or.Uri, err.Error())
	}
	or.client = orderer.NewAtomicBroadcastClient(conn)

	return &or, nil
}

//newIdentityFromUserByteConfig 从结构体初始化Identity
func newIdentityFromUserByteConfig(conf *UserByteConfig) (*Identity, error) {
	cpb, _ := pem.Decode(conf.CertBytes)
	kpb, _ := pem.Decode(conf.KeyBytes)
	crt, err := x509.ParseCertificate(cpb.Bytes)
	if err != nil {
		return nil, errors.WithMessage(err, "parse Certificate failed")
	}
	key, err := x509.ParsePKCS8PrivateKey(kpb.Bytes)
	if err != nil {
		return nil, errors.WithMessage(err, "parse private key failed")
	}
	return &Identity{Certificate: crt, PrivateKey: key, MspId: conf.MspID}, nil
}

//ListenEventFullBlock 监听完整区块
func (ep *EventProvider) ListenEventFullBlock(channelName string, startNum int64) (chan parseBlock.Block, error) {
	if channelName == "" {
		return nil, errors.New("ListenEventFullBlock channelName is empty ")
	}
	ch := make(chan parseBlock.Block)
	listener, err := NewEventListener(context.Background(), ep.CryptoSuite, *ep.Identity, *ep.Peer, channelName, EventTypeFullBlock)
	if err != nil {
		return nil, errors.WithMessage(err, "create EventListener failed")
	}
	if startNum < 0 {
		err = listener.SeekNewest()
	} else {
		err = listener.SeekRange(uint64(startNum), math.MaxUint64)
	}
	if err != nil {
		return nil, errors.WithMessage(err, "listener send env failed")
	}
	listener.Listen(ch, nil)
	return ch, nil
}

//ListenEventFilterBlock 监听过滤后区块
func (ep *EventProvider) ListenEventFilterBlock(channelName string, startNum int64) (chan EventBlockResponse, error) {
	if channelName == "" {
		return nil, errors.New("ListenEventFullBlock channelName is empty ")
	}
	ch := make(chan EventBlockResponse)
	listener, err := NewEventListener(context.Background(), ep.CryptoSuite, *ep.Identity, *ep.Peer, channelName, EventTypeFiltered)
	if err != nil {
		return nil, errors.WithMessage(err, "create EventListener failed")
	}
	if startNum < 0 {
		err = listener.SeekNewest()
	} else {
		err = listener.SeekRange(uint64(startNum), math.MaxUint64)
	}
	if err != nil {
		return nil, errors.WithMessage(err, "listener send env failed")
	}
	listener.Listen(nil, ch)
	return ch, nil
}
