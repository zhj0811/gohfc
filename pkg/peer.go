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

	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// Peer expose API's to communicate with peer
type Peer struct {
	Name        string
	OrgName     string
	Uri         string
	MspId       string
	Opts        []grpc.DialOption
	caPath      string
	tlsCertHash []byte
	client      peer.EndorserClient
}

// PeerResponse is response from peer transaction request
type PeerResponse struct {
	Response *peer.ProposalResponse
	Err      error
	Name     string
}

// Endorse sends single transaction to single peer.
func (p *Peer) Endorse(resp chan *PeerResponse, prop *peer.SignedProposal) {
	proposalResp, err := p.client.ProcessProposal(context.Background(), prop)
	if err != nil {
		resp <- &PeerResponse{Response: nil, Name: p.Name, Err: err}
		return
	}
	resp <- &PeerResponse{Response: proposalResp, Name: p.Name, Err: nil}
}

// Endorse sends single transaction to single peer.
func (p *Peer) SubmitProposal(signedProposal *peer.SignedProposal) error {
	proposalResponse, err := p.client.ProcessProposal(context.Background(), signedProposal)
	if err != nil {
		return errors.WithMessage(err, "failed to endorse ProcessProposal")
	}
	return validateProposalResponse(proposalResponse)
}

func validateProposalResponse(proposalResponse *peer.ProposalResponse) error {
	if proposalResponse == nil {
		return errors.New("received nil proposal response")
	}

	if proposalResponse.Response == nil {
		return errors.New("received proposal response with nil response")
	}

	if proposalResponse.Response.Status != int32(cb.Status_SUCCESS) {
		return errors.Errorf("submit proposal failed with status: %d - %s", proposalResponse.Response.Status, proposalResponse.Response.Message)
	}
	return nil
}

// NewPeerFromConfig creates new peer from provided config
func NewPeerFromConfig(ctx context.Context, cliConfig ChannelConfig, conf PeerConfig, cryptoSuite CryptoSuite) (*Peer, error) {
	p := Peer{Uri: conf.Host, caPath: conf.TlsPath}
	if !conf.UseTLS {
		p.Opts = []grpc.DialOption{grpc.WithInsecure()}
	} else if p.caPath != "" {
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
			if cryptoSuite != nil {
				p.tlsCertHash = cryptoSuite.Hash(cert.Certificate[0])
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
			p.Opts = append(p.Opts, grpc.WithTransportCredentials(credentials.NewTLS(c)))
		} else {
			creds, err := credentials.NewClientTLSFromFile(p.caPath, conf.DomainName)
			if err != nil {
				return nil, fmt.Errorf("cannot read peer %s credentials err is: %v", p.Name, err)
			}
			p.Opts = append(p.Opts, grpc.WithTransportCredentials(creds))
		}
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
