/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Orderer expose API's to communicate with orderers.
type Orderer struct {
	Name   string
	Uri    string
	Opts   []grpc.DialOption
	caPath string
	client orderer.AtomicBroadcastClient
}

const timeout = 5

// Broadcast Broadcast envelope to orderer for execution.
func (o *Orderer) Broadcast(envelope *common.Envelope) (*orderer.BroadcastResponse, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bcc, err := o.client.Broadcast(ctx)
	if err != nil {
		return nil, err
	}
	defer bcc.CloseSend()
	bcc.Send(envelope)
	response, err := bcc.Recv()
	if err != nil {
		return nil, err
	}
	if response.Status != common.Status_SUCCESS {
		return nil, fmt.Errorf("unexpected status: %v", response.Status)
	}

	return response, err
}

// Deliver delivers envelope to orderer. Please note that new connection will be created on every call of Deliver.
func (o *Orderer) Deliver(envelope *common.Envelope) (*common.Block, error) {

	connection, err := grpc.Dial(o.Uri, o.Opts...)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to orderer: %s err is: %v", o.Name, err)
	}
	defer connection.Close()

	dk, err := orderer.NewAtomicBroadcastClient(connection).Deliver(context.Background())
	if err != nil {
		return nil, err
	}
	if err := dk.Send(envelope); err != nil {
		return nil, err
	}
	var block *common.Block
	timer := time.NewTimer(time.Second * time.Duration(timeout))
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return nil, ErrOrdererTimeout
		default:
			response, err := dk.Recv()
			if err != nil {
				return nil, err
			}
			switch t := response.Type.(type) {
			case *orderer.DeliverResponse_Status:
				if t.Status == common.Status_SUCCESS {
					return block, nil
				} else {
					return nil, fmt.Errorf("orderer response with status: %v", t.Status)
				}
			case *orderer.DeliverResponse_Block:
				block = response.GetBlock()

			default:
				return nil, fmt.Errorf("unknown response type from orderer: %s", t)
			}
		}
	}
}

func (o *Orderer) getGenesisBlock(identity Identity, crypto CryptoSuite, channelId string) (*common.Block, error) {

	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := createEnvWithSeekInfo(seekInfo, identity, crypto, channelId)
	if err != nil {
		return nil, errors.WithMessage(err, "create envelop with orderer.SeekInfo failed")
	}
	return o.Deliver(env)
}

func (o *Orderer) getLastConfigBlock(identity Identity, crypto CryptoSuite, channelID string) (*common.Block, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
	if err != nil {
		return nil, errors.WithMessage(err, "create envelop with orderer.SeekInfo failed")
	}
	newestBlock, err := o.Deliver(env)
	if err != nil {
		return nil, errors.WithMessage(err, "get newest block failede")
	}
	lci, err := utils.GetLastConfigIndexFromBlock(newestBlock)
	if err != nil {
		return nil, errors.WithMessage(err, "get last config index from block failed")
	}

	seekInfo = &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: lci}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: lci}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err = createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
	if err != nil {
		return nil, err
	}
	return o.Deliver(env)
}

func createEnvWithSeekInfo(seekInfo *orderer.SeekInfo, identity Identity, crypto CryptoSuite, channelID string) (*common.Envelope, error) {
	seekInfoBytes, err := proto.Marshal(seekInfo)
	if err != nil {
		return nil, err
	}

	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		return nil, err
	}

	headerBytes, err := channelHeader(common.HeaderType_DELIVER_SEEK_INFO, txId, channelID, 0, nil)
	signatureHeaderBytes, err := signatureHeader(creator, txId)
	if err != nil {
		return nil, err
	}
	header := header(signatureHeaderBytes, headerBytes)
	payloadBytes, err := payload(header, seekInfoBytes)
	if err != nil {
		return nil, err
	}
	payloadSignedBytes, err := crypto.Sign(payloadBytes, identity.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &common.Envelope{Payload: payloadBytes, Signature: payloadSignedBytes}, nil
}

// NewOrdererFromConfig create new Orderer from config
func NewOrdererFromConfig(ctx context.Context, cliConfig ChannelConfig, conf OrdererConfig) (*Orderer, error) {
	o := Orderer{Uri: conf.Host, caPath: conf.TlsPath}
	if !conf.UseTLS {
		o.Opts = []grpc.DialOption{grpc.WithInsecure()}
	} else if o.caPath != "" {
		if conf.ClientKey != "" {
			//TODO 为了兼容老版本每个节点都要配置双端验证，以后版本只在channelConfig配置一份设置
			cliConfig.TlsMutual = conf.TlsMutual
			cliConfig.ClientCert = conf.ClientCert
			cliConfig.ClientKey = conf.ClientKey
		}
		if cliConfig.TlsMutual {
			cert, err := tls.LoadX509KeyPair(cliConfig.ClientCert, cliConfig.ClientKey)
			if err != nil {
				return nil, fmt.Errorf("failed to Load client keypair: %s\n", err.Error())
			}
			caPem, err := ioutil.ReadFile(conf.TlsPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert faild err:%s\n", err.Error())
			}
			certpool := x509.NewCertPool()
			certpool.AppendCertsFromPEM(caPem)
			c := &tls.Config{
				ServerName:   conf.DomainName,
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
				RootCAs:      certpool,
				//InsecureSkipVerify: true, // Client verifies server's cert if false, else skip.
			}
			o.Opts = append(o.Opts, grpc.WithTransportCredentials(credentials.NewTLS(c)))
		} else {
			creds, err := credentials.NewClientTLSFromFile(o.caPath, conf.DomainName)
			if err != nil {
				return nil, fmt.Errorf("cannot read orderer %s credentials err is: %v", o.Name, err)
			}
			o.Opts = append(o.Opts, grpc.WithTransportCredentials(creds))
		}
	}
	o.Opts = append(o.Opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(1) * time.Minute,
			Timeout:             time.Duration(20) * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize)))
	c, err := grpc.DialContext(ctx, o.Uri, o.Opts...)
	if err != nil {
		return nil, fmt.Errorf("connect host=%s failed, err:%s\n", o.Uri, err.Error())
	}
	o.client = orderer.NewAtomicBroadcastClient(c)

	return &o, nil
}
