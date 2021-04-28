/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/stretchr/testify/assert"
	"github.com/zhj0811/gohfc/pkg/testpb"
	"google.golang.org/grpc"
)

var (
	listener        net.Listener
	server          *grpc.Server
	fakeEchoService *EchoServiceServer
)

type EchoServiceServer struct {
	EchoStub       func(context.Context, *testpb.Message) (*testpb.Message, error)
	EchoStreamStub func(testpb.EchoService_EchoStreamServer) error
}

func (fake *EchoServiceServer) Echo(arg1 context.Context, arg2 *testpb.Message) (*testpb.Message, error) {
	return nil, nil
}
func (fake *EchoServiceServer) EchoStream(arg1 testpb.EchoService_EchoStreamServer) error {
	return nil
}

//构建一个GRPC server
func fakeGRPCServer() {
	var err error

	fakeEchoService = &EchoServiceServer{}
	fakeEchoService.EchoStub = func(ctx context.Context, msg *testpb.Message) (*testpb.Message, error) {
		msg.Sequence++
		return msg, nil
	}
	fakeEchoService.EchoStreamStub = func(stream testpb.EchoService_EchoStreamServer) error {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		msg.Sequence++
		return stream.Send(msg)
	}

	listener, err = net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	server = grpc.NewServer()
	testpb.RegisterEchoServiceServer(server, fakeEchoService)

	server.Serve(listener)
}

//测试构建peer
func TestNewPeerFromConfig(t *testing.T) {
	type args struct {
		cliConfig   ChannelConfig
		conf        PeerConfig
		cryptoSuite CryptoSuite
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test withoutTLS",
			args: args{
				conf: PeerConfig{Host: ":9999", UseTLS: false},
			},
			want: ":9999",
		},
		//{
		//	name: "test with TLS",
		//	args : args{
		//		conf: PeerConfig{
		//			Host:       ":9999",
		//			UseTLS:     true,
		//			TlsPath:    "/home/vagrant/one_pc_test/deploy/e2ecli/crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt",
		//		},
		//	},
		//	want: ":9999",
		//},
	}
	go fakeGRPCServer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPeerFromConfig(context.Background(), tt.args.cliConfig, tt.args.conf, tt.args.cryptoSuite)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPeerFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got.Uri)
		})
	}
}

//测试peer endorser
func TestPeer_Endorse(t *testing.T) {
	var proposalResponse = &peer.ProposalResponse{Version: 9527}
	type fields struct {
		Name   string
		client peer.EndorserClient
	}
	type args struct {
		resp chan *PeerResponse
		prop *peer.SignedProposal
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *PeerResponse
	}{
		{
			name:   "test all right",
			fields: fields{Name: "peer0", client: getMockEndorserClient(proposalResponse, nil)},
			args: args{
				resp: make(chan *PeerResponse),
				prop: nil,
			},
			want: &PeerResponse{
				Response: proposalResponse,
				Err:      nil,
				Name:     "peer0",
			},
		},
		{
			name:   "test error",
			fields: fields{Name: "peer1", client: getMockEndorserClient(proposalResponse, errors.New("test error"))},
			args: args{
				resp: make(chan *PeerResponse),
				prop: nil,
			},
			want: &PeerResponse{
				Response: nil,
				Err:      errors.New("test error"),
				Name:     "peer1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Peer{
				Name:   tt.fields.Name,
				client: tt.fields.client,
			}
			go p.Endorse(tt.args.resp, tt.args.prop)
			assert.Equal(t, tt.want, <-tt.args.resp)
		})
	}
}
