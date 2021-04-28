/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"context"

	"github.com/hyperledger/fabric-protos-go/peer"
	"google.golang.org/grpc"
)

//getMockEndorserClient 返回一个构造的endorser client
func getMockEndorserClient(response *peer.ProposalResponse, err error) peer.EndorserClient {
	return &mockEndorserClient{
		response: response,
		err:      err,
	}
}

type mockEndorserClient struct {
	response *peer.ProposalResponse
	err      error
}

func (m *mockEndorserClient) ProcessProposal(ctx context.Context, in *peer.SignedProposal, opts ...grpc.CallOption) (*peer.ProposalResponse, error) {
	return m.response, m.err
}

//getMockBroadcastClient 返回一个构造的broadcast client
//func getMockBroadcastClient(err error)
