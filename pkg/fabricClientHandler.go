package gohfc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	lb "github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/core/common/ccpackage"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

type ClientHandler struct {
	CryptoSuite
	*Peer
	*Orderer
	*Identity
	Endorsers []*Peer
}

type ClientHandlerConfig struct {
	*CryptoConfig
	*PeerByteConfig
	*OrdererByteConfig
	*UserByteConfig
	EndorsersConfig []*PeerByteConfig
}

type OrdererByteConfig NodeConfig

//CreateUpdateChannel 创建通道，更新锚节点
func (client *ClientHandler) CreateUpdateChannel(env []byte, channelId string) error {
	envelope := new(common.Envelope)
	if err := proto.Unmarshal(env, envelope); err != nil {
		return errors.WithMessage(err, "unmarshal channel.tx byte failed")
	}
	ou, err := buildAndSignChannelConfig(*client.Identity, envelope.GetPayload(), client.CryptoSuite, channelId)
	if err != nil {
		return errors.WithMessage(err, "buildAndSignChannelConfig failed")
	}
	replay, err := client.Orderer.Broadcast(ou)
	if err != nil {
		return err
	}
	if replay.GetStatus() != common.Status_SUCCESS {
		return errors.New("error creating new channel. See orderer logs for more details")
	}
	return nil
}

func (client *ClientHandler) JoinChannel(channelId string) error {
	block, err := client.Orderer.getGenesisBlock(*client.Identity, client.CryptoSuite, channelId)
	if err != nil {
		return errors.WithMessage(err, "getGenesisBlock failed")
	}
	blockBytes, err := proto.Marshal(block)
	if err != nil {
		return errors.WithMessage(err, "marshal channelGenesisBlock failed")
	}

	chainCode := ChainCode{Name: CSCC,
		Type:     ChaincodeSpec_GOLANG,
		Args:     []string{"JoinChain"},
		ArgBytes: blockBytes}

	invocationBytes, err := chainCodeInvocationSpec(chainCode)
	if err != nil {
		return err
	}
	creator, err := marshalProtoIdentity(*client.Identity)
	if err != nil {
		return err
	}
	txId, err := newTransactionId(creator, client.CryptoSuite)
	if err != nil {
		return err
	}
	ext := &pb.ChaincodeHeaderExtension{ChaincodeId: &pb.ChaincodeID{Name: CSCC}}
	channelHeaderBytes, err := channelHeader(common.HeaderType_ENDORSER_TRANSACTION, txId, "", 0, ext)
	if err != nil {
		return err
	}

	sigHeaderBytes, err := signatureHeader(creator, txId)
	if err != nil {
		return err
	}

	header := header(sigHeaderBytes, channelHeaderBytes)
	headerBytes, err := proto.Marshal(header)
	if err != nil {
		return err
	}
	chainCodePropPl := new(pb.ChaincodeProposalPayload)
	chainCodePropPl.Input = invocationBytes

	chainCodePropPlBytes, err := proto.Marshal(chainCodePropPl)
	if err != nil {
		return err
	}

	proposalBytes, err := proposal(headerBytes, chainCodePropPlBytes)
	if err != nil {
		return err
	}

	signedProp, err := signedProposal(proposalBytes, *client.Identity, client.CryptoSuite)
	if err != nil {
		return err
	}
	return client.Peer.SubmitProposal(signedProp)
}

//InstallCCByPack 根据由peer生成的安装包安装chaincode；现仅支持golang版
func (client *ClientHandler) InstallCCByPack(b []byte) error {
	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	if err != nil {
		return errors.WithMessage(err, "sw.NewDefaultSecurityLevelWithKeystore failed")
	}
	ccpack, err := ccprovider.GetCCPackage(b, cryptoProvider)
	if err != nil {
		return errors.WithMessage(err, "get CDSPackage failed")
	}

	//either CDS or Envelope
	o := ccpack.GetPackageObject()

	//try CDS first
	cds, ok := o.(*pb.ChaincodeDeploymentSpec)
	if !ok || cds == nil {
		//try Envelope next
		env, ok := o.(*common.Envelope)
		if !ok || env == nil {
			return errors.WithMessage(err, "error extracting valid chaincode package")
		}

		//this will check for a valid package Envelope
		_, sCDS, err := ccpackage.ExtractSignedCCDepSpec(env)
		if err != nil {
			return errors.WithMessage(err, "error extracting valid signed chaincode package")
		}

		//return errors.New("noting to do & ccpackage failed")

		//...and get the CDS at last
		cds, err = utils.UnmarshalChaincodeDeploymentSpec(sCDS.ChaincodeDeploymentSpec)
		if err != nil {
			return errors.WithMessage(err, "error extracting chaincode deployment spec")
		}
		//err = platformRegistry.ValidateDeploymentSpec(cds.ChaincodeSpec.Type.String(), cds.CodePackage)
		//if err != nil {
		//	return errors.WithMessage(err, "chaincode deployment spec validation failed")
		//}
	}

	creator, err := marshalProtoIdentity(*client.Identity)
	if err != nil {
		return err
	}
	prop, _, err := utils.CreateInstallProposalFromCDS(o, creator)
	if err != nil {
		return errors.WithMessage(err, "creating install proposal failed")
	}
	propBytes, err := proto.Marshal(prop)
	if err != nil {
		return errors.WithMessage(err, "getBytesProposal failed")
	}

	signedProp, err := signedProposal(propBytes, *client.Identity, client.CryptoSuite)
	if err != nil {
		return err
	}
	return client.Peer.SubmitProposal(signedProp)
}

//InstallCCByCodeTar 根据code.tar.gz包安装chaincode；现仅支持golang版
func (client *ClientHandler) InstallCCByCodeTar(fr *os.File, req *InstallRequest) error {
	packageBytes, err := modifyPacakge(fr, req.Namespace)
	if err != nil {
		return errors.WithMessage(err, "modify tgz package failed")
	}
	prop, err := createInstallTransaction(*client.Identity, req, client.CryptoSuite, packageBytes)
	if err != nil {
		return errors.WithMessage(err, "create install transaction proposal failed")
	}
	signedProp, err := signedProposal(prop.proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return errors.WithMessage(err, "create signed proposal failed")
	}

	return client.Peer.SubmitProposal(signedProp)
}

func modifyPacakge(fr *os.File, namespace string) ([]byte, error) {
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	twBuf := new(bytes.Buffer)
	tw := tar.NewWriter(twBuf)

	// gzip read
	gr, err := gzip.NewReader(fr)
	if err != nil {
		return nil, errors.WithMessage(err, "gzip newReader failed")
	}
	defer gr.Close()
	// tar read
	tr := tar.NewReader(gr)

	// 读取文件
	baseDir := path.Join("/src", namespace)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.WithMessage(err, "tar newReader next() failed")
		}
		//h.Mode = 0100000
		h.Name = filepath.Join(baseDir, h.Name)
		if err := tw.WriteHeader(h); err != nil {
			return nil, errors.WithMessage(err, "tar.NewWriter write header failed")
		}
		_, err = io.Copy(tw, tr)
		if err != nil {
			return nil, errors.WithMessage(err, "copy reader to writer failed")
		}
	}

	if _, err := zw.Write(twBuf.Bytes()); err != nil {
		return nil, errors.WithMessage(err, "gzip.Writer write failed")
	}
	tw.Close()
	zw.Close()

	return gzBuf.Bytes(), nil
}

func (client *ClientHandler) InstantiateChainCode(req *ChainCode, operation, policy string) error {
	prop, err := createInstantiateProposal(*client.Identity, req, operation, policy, nil, client.CryptoSuite)
	if err != nil {
		return err
	}

	proposal, err := signedProposal(prop.proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return err
	}

	transaction, err := createTransaction(prop.proposal, sendToPeers([]*Peer{client.Peer}, proposal))
	if err != nil {
		return err
	}

	signedTransaction, err := client.CryptoSuite.Sign(transaction, client.Identity.PrivateKey)
	if err != nil {
		return err
	}

	reply, err := client.Orderer.Broadcast(&common.Envelope{Payload: transaction, Signature: signedTransaction})
	if err != nil {
		return err
	}
	if reply.Status != common.Status_SUCCESS {
		return errors.Errorf("broadcastResponse status is %d with info %s", reply.Status, reply.Info)
	}
	return nil
}

func (client *ClientHandler) GetLastConfigBlock(channelID string) (*common.Block, error) {
	return client.Orderer.getLastConfigBlock(*client.Identity, client.CryptoSuite, channelID)
}

func (client *ClientHandler) LifecycleInstall(pkg string) (*lb.InstallChaincodeResult, error) {
	packageBytes, err := ioutil.ReadFile(pkg)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read chaincode package at '%s'", pkg)
	}
	proposal, err := createLifecycleInstallProposal(*client.Identity, packageBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "create install transaction proposal failed")
	}
	signedProp, err := signedProposal(proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return nil, errors.WithMessage(err, "create signed proposal failed")
	}

	proposalResponse, err := client.Peer.client.ProcessProposal(context.Background(), signedProp)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to endorse ProcessProposal")
	}
	err = validateProposalResponse(proposalResponse)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to ProposalResponse failed")
	}
	icr := &lb.InstallChaincodeResult{}
	err = proto.Unmarshal(proposalResponse.Response.Payload, icr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}
	return icr, nil
}

func (client *ClientHandler) ApproveForMyOrg(channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) error {
	proposal, _, err := createApproveProposal(*client.Identity, channelID, ccName, ccVersion, ccID, signaturePolicy, sequence, initReqired)
	if err != nil {
		return errors.WithMessage(err, "failed to create proposal")
	}

	signedProp, err := signedProposal(proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return errors.WithMessage(err, "failed to sign proposal")
	}

	propResponses := sendToPeers([]*Peer{client.Peer}, signedProp)
	transaction, err := createTransaction(proposal, propResponses)
	if err != nil {
		return errors.WithMessage(err, "failed to create transaction")
	}
	signedTransaction, err := client.CryptoSuite.Sign(transaction, client.Identity.PrivateKey)
	if err != nil {
		return errors.WithMessage(err, "failed to sign transaction")
	}
	_, err = client.Orderer.Broadcast(&common.Envelope{Payload: transaction, Signature: signedTransaction})
	if err != nil {
		return errors.WithMessage(err, "failed to broadcast transaction")
	}
	return nil
}

func (client *ClientHandler) LifecycleCommit(channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) error {
	proposal, _, err := createCommitProposal(*client.Identity, channelID, ccName, ccVersion, ccID, signaturePolicy, sequence, initReqired)
	if err != nil {
		return errors.WithMessage(err, "failed to create proposal")
	}

	signedProp, err := signedProposal(proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return errors.WithMessage(err, "failed to sign proposal")
	}

	propResponses := sendToPeers(append(client.Endorsers, client.Peer), signedProp)
	transaction, err := createTransaction(proposal, propResponses)
	if err != nil {
		return errors.WithMessage(err, "failed to create transaction")
	}
	signedTransaction, err := client.CryptoSuite.Sign(transaction, client.Identity.PrivateKey)
	if err != nil {
		return errors.WithMessage(err, "failed to sign transaction")
	}
	_, err = client.Orderer.Broadcast(&common.Envelope{Payload: transaction, Signature: signedTransaction})
	if err != nil {
		return errors.WithMessage(err, "failed to broadcast transaction")
	}
	return nil
}

func (client *ClientHandler) InvokeOrQuery(args []string, transientMap map[string][]byte, channelName, chaincodeName string, isInit, isQuery bool) (*pb.ProposalResponse, error) {
	chaincode := ChainCode{
		ChannelId:    channelName,
		Type:         ChaincodeSpec_GOLANG,
		Name:         chaincodeName,
		Args:         args,
		TransientMap: transientMap,
		isInit:       isInit,
	}
	proposal, _, err := createInvokeQueryProposal(*client.Identity, chaincode)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create proposal")
	}

	signedProp, err := signedProposal(proposal, *client.Identity, client.CryptoSuite)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to sign proposal")
	}

	propResponses := sendToPeers(append(client.Endorsers, client.Peer), signedProp)

	if isQuery {
		err := validateProposalResponse(propResponses[0].Response)
		if err != nil {
			return nil, errors.WithMessage(err, "validate proposal response failed")
		}
		return propResponses[0].Response, nil
	}

	transaction, err := createTransaction(proposal, propResponses)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create transaction")
	}
	signedTransaction, err := client.CryptoSuite.Sign(transaction, client.Identity.PrivateKey)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to sign transaction")
	}
	_, err = client.Orderer.Broadcast(&common.Envelope{Payload: transaction, Signature: signedTransaction})
	if err != nil {
		return nil, errors.WithMessage(err, "failed to broadcast transaction")
	}
	return propResponses[0].Response, nil
}
