/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	cb "github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	lb "github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/common/policydsl"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

type ChainCodeType int32

const (
	ChaincodeSpec_UNDEFINED ChainCodeType = 0
	ChaincodeSpec_GOLANG    ChainCodeType = 1
	ChaincodeSpec_NODE      ChainCodeType = 2
	ChaincodeSpec_CAR       ChainCodeType = 3
	ChaincodeSpec_JAVA      ChainCodeType = 4

	lifecycleName                = "_lifecycle"
	approveFuncName              = "ApproveChaincodeDefinitionForMyOrg"
	commitFuncName               = "CommitChaincodeDefinition"
	checkCommitReadinessFuncName = "CheckCommitReadiness"
)

// ChainCode the fields necessary to execute operation over chaincode.
type ChainCode struct {
	ChannelId    string
	Name         string
	Version      string
	Type         ChainCodeType
	Args         []string
	ArgBytes     []byte
	TransientMap map[string][]byte
	rawArgs      [][]byte
	isInit       bool
}

func (c *ChainCode) toChainCodeArgs() [][]byte {
	if len(c.rawArgs) > 0 {
		return c.rawArgs
	}
	args := make([][]byte, len(c.Args))
	for i, arg := range c.Args {
		args[i] = []byte(arg)
	}
	if len(c.ArgBytes) > 0 {
		args = append(args, c.ArgBytes)
	}
	return args
}

// InstallRequest holds fields needed to install chaincode
type InstallRequest struct {
	ChannelId        string
	ChainCodeName    string
	ChainCodeVersion string
	ChainCodeType    ChainCodeType
	Namespace        string
	SrcPath          string
	Libraries        []ChaincodeLibrary
}

type CollectionConfig struct {
	Name               string
	RequiredPeersCount int32
	MaximumPeersCount  int32
	Organizations      []string
}

type ChaincodeLibrary struct {
	Namespace string
	SrcPath   string
}

// ChainCodesResponse is the result of queering installed and instantiated chaincodes
type ChainCodesResponse struct {
	PeerName   string
	Error      error
	ChainCodes []*pb.ChaincodeInfo
}

// createInstallProposal read chaincode from provided source and namespace, pack it and generate install proposal
// transaction. Transaction is not send from this func
func createInstallProposal(identity Identity, req *InstallRequest, crypto CryptoSuite) (*transactionProposal, error) {

	var packageBytes []byte
	var err error

	switch req.ChainCodeType {
	case ChaincodeSpec_GOLANG:
		packageBytes, err = packGolangCC(req.Namespace, req.SrcPath, req.Libraries)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedChaincodeType
	}
	//now := time.Now()

	return createInstallTransaction(identity, req, crypto, packageBytes)
}

func createInstallTransaction(identity Identity, req *InstallRequest, crypto CryptoSuite, packageBytes []byte) (*transactionProposal, error) {
	depSpec, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{Name: req.ChainCodeName, Path: req.Namespace, Version: req.ChainCodeVersion},
			Type:        pb.ChaincodeSpec_Type(req.ChainCodeType),
		},
		CodePackage: packageBytes,
		//EffectiveDate: &timestamp.Timestamp{Seconds: int64(now.Second()), Nanos: int32(now.Nanosecond())},
	})
	if err != nil {
		return nil, err
	}
	spec, err := chainCodeInvocationSpec(ChainCode{Type: req.ChainCodeType,
		Name:     LSCC,
		Args:     []string{"install"},
		ArgBytes: depSpec,
	})

	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		return nil, err
	}
	ccHdrExt := &pb.ChaincodeHeaderExtension{ChaincodeId: &pb.ChaincodeID{Name: LSCC}}

	channelHeaderBytes, err := channelHeader(cb.HeaderType_ENDORSER_TRANSACTION, txId, req.ChannelId, 0, ccHdrExt)
	if err != nil {
		return nil, err
	}

	ccPropPayloadBytes, err := proto.Marshal(&pb.ChaincodeProposalPayload{
		Input:        spec,
		TransientMap: nil,
	})
	if err != nil {
		return nil, err
	}

	sigHeader, err := signatureHeader(creator, txId)
	if err != nil {
		return nil, err
	}
	header := header(sigHeader, channelHeaderBytes)

	hdrBytes, err := proto.Marshal(header)
	if err != nil {
		return nil, err
	}
	proposal, err := proposal(hdrBytes, ccPropPayloadBytes)
	if err != nil {
		return nil, err
	}
	return &transactionProposal{proposal: proposal, transactionId: txId.TransactionId}, nil

}

// createInstantiateProposal creates instantiate proposal transaction for already installed chaincode.
// transaction is not send from this func
func createInstantiateProposal(identity Identity, req *ChainCode, operation, policy string, collectionConfig []byte, crypto CryptoSuite) (*transactionProposal, error) {
	if operation != deploy && operation != upgrade {
		return nil, errors.New("instantiate operation accept only 'deploy' and 'upgrade' operations")
	}

	depSpec, err := proto.Marshal(&pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{Name: req.Name, Version: req.Version},
			Type:        pb.ChaincodeSpec_Type(req.Type),
			Input:       &pb.ChaincodeInput{Args: req.toChainCodeArgs()},
		},
	})
	if err != nil {
		return nil, err
	}

	policyEnv := &cb.SignaturePolicyEnvelope{}
	if policy == "" {
		policyEnv = policydsl.SignedByMspMember(identity.MspId)
	} else {
		policyEnv, err = policydsl.FromString(policy)
		if err != nil {
			return nil, errors.WithMessage(err, "create signaturePolicyEnv failed")
		}
	}

	marshPolicy, err := proto.Marshal(policyEnv)
	if err != nil {
		return nil, errors.WithMessage(err, "marshal signaturePolicyEnv failed")
	}

	args := [][]byte{
		[]byte(operation),
		[]byte(req.ChannelId),
		depSpec,
		marshPolicy,
		[]byte("escc"),
		[]byte("vscc"),
	}
	if len(collectionConfig) > 0 {
		args = append(args, collectionConfig)
	}

	spec, err := chainCodeInvocationSpec(ChainCode{
		Type:    req.Type,
		Name:    LSCC,
		rawArgs: args,
	})

	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		return nil, err
	}
	headerExtension := &pb.ChaincodeHeaderExtension{ChaincodeId: &pb.ChaincodeID{Name: LSCC}}

	channelHeaderBytes, err := channelHeader(cb.HeaderType_ENDORSER_TRANSACTION, txId, req.ChannelId, 0, headerExtension)
	if err != nil {
		return nil, err
	}
	payloadBytes, err := proto.Marshal(&pb.ChaincodeProposalPayload{Input: spec, TransientMap: req.TransientMap})
	if err != nil {
		return nil, err
	}
	signatureHeader, err := signatureHeader(creator, txId)
	if err != nil {
		return nil, err
	}
	headerBytes, err := proto.Marshal(header(signatureHeader, channelHeaderBytes))
	if err != nil {
		return nil, err
	}

	proposal, err := proposal(headerBytes, payloadBytes)
	if err != nil {
		return nil, err
	}
	return &transactionProposal{proposal: proposal, transactionId: txId.TransactionId}, nil

}

// packGolangCC read provided src expecting Golang source code, repackage it in provided namespace, and compress it
func packGolangCC(namespace, source string, libs []ChaincodeLibrary) ([]byte, error) {

	twBuf := new(bytes.Buffer)
	tw := tar.NewWriter(twBuf)

	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)

	concatLibs := append(libs, ChaincodeLibrary{SrcPath: source, Namespace: namespace})

	for _, s := range concatLibs {
		_, err := os.Stat(s.SrcPath)
		if err != nil {
			return nil, err
		}
		baseDir := path.Join("/src", s.Namespace)
		err = filepath.Walk(s.SrcPath,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}

				header.Mode = 0100000
				if baseDir != "" {
					header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, s.SrcPath))
				}
				if header.Name == baseDir {
					return nil
				}

				if err := tw.WriteHeader(header); err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()
				_, err = io.Copy(tw, file)

				return err
			})
		if err != nil {
			tw.Close()
			return nil, err
		}
	}
	_, err := zw.Write(twBuf.Bytes())
	if err != nil {
		return nil, err
	}
	tw.Close()
	zw.Close()
	return gzBuf.Bytes(), nil
}

func createLifecycleInstallProposal(identity Identity, pkgBytes []byte) ([]byte, error) {
	creatorBytes, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, err
	}

	installChaincodeArgs := &lb.InstallChaincodeArgs{
		ChaincodeInstallPackage: pkgBytes,
	}

	installChaincodeArgsBytes, err := proto.Marshal(installChaincodeArgs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal InstallChaincodeArgs")
	}

	ccInput := &pb.ChaincodeInput{Args: [][]byte{[]byte("InstallChaincode"), installChaincodeArgsBytes}}

	cis := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{Name: lifecycleName},
			Input:       ccInput,
		},
	}

	proposal, _, err := protoutil.CreateProposalFromCIS(cb.HeaderType_ENDORSER_TRANSACTION, "", cis, creatorBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create proposal for ChaincodeInvocationSpec")
	}

	if proposal == nil {
		return nil, errors.New("proposal cannot be nil")
	}
	proposalBytes, err := proto.Marshal(proposal)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling proposal")
	}
	return proposalBytes, nil
}

func createApproveProposal(identity Identity, channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) (proposalBytes []byte, txID string, err error) {
	policyBytes, err := createPolicyBytes(signaturePolicy, "")
	if err != nil {
		return nil, "", errors.WithMessage(err, "create policy failed")
	}
	ccsrc := &lb.ChaincodeSource{
		Type: &lb.ChaincodeSource_LocalPackage{
			LocalPackage: &lb.ChaincodeSource_Local{
				PackageId: ccID,
			},
		},
	}

	args := &lb.ApproveChaincodeDefinitionForMyOrgArgs{
		Name:                ccName,
		Version:             ccVersion,
		Sequence:            sequence,
		ValidationParameter: policyBytes,
		InitRequired:        initReqired,
		Source:              ccsrc,
	}

	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, "", err
	}
	ccInput := &pb.ChaincodeInput{Args: [][]byte{[]byte(approveFuncName), argsBytes}}

	cis := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{Name: lifecycleName},
			Input:       ccInput,
		},
	}

	creatorBytes, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, "", err
	}

	proposal, txID, err := protoutil.CreateChaincodeProposal(cb.HeaderType_ENDORSER_TRANSACTION, channelID, cis, creatorBytes)
	if err != nil {
		return nil, "", errors.WithMessage(err, "failed to create ChaincodeInvocationSpec proposal")
	}

	proposalBytes, err = proto.Marshal(proposal)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling proposal")
	}

	return proposalBytes, txID, nil
}

func createCommitProposal(identity Identity, channelID, ccName, ccVersion, ccID, signaturePolicy string, sequence int64, initReqired bool) (proposalBytes []byte, txID string, err error) {
	policyBytes, err := createPolicyBytes(signaturePolicy, "")
	if err != nil {
		return nil, "", errors.WithMessage(err, "create policy failed")
	}
	args := &lb.CommitChaincodeDefinitionArgs{
		Name:     ccName,
		Version:  ccVersion,
		Sequence: sequence,
		//EndorsementPlugin:   c.Input.EndorsementPlugin,
		//ValidationPlugin:    c.Input.ValidationPlugin,
		ValidationParameter: policyBytes,
		InitRequired:        initReqired,
		//Collections:         CollectionConfigPackage,
	}

	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, "", err
	}
	ccInput := &pb.ChaincodeInput{Args: [][]byte{[]byte(commitFuncName), argsBytes}}

	cis := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{Name: lifecycleName},
			Input:       ccInput,
		},
	}

	creatorBytes, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, "", errors.WithMessage(err, "failed to serialize identity")
	}

	proposal, txID, err := protoutil.CreateChaincodeProposalWithTxIDAndTransient(cb.HeaderType_ENDORSER_TRANSACTION, channelID, cis, creatorBytes, "", nil)
	if err != nil {
		return nil, "", errors.WithMessage(err, "failed to create ChaincodeInvocationSpec proposal")
	}
	proposalBytes, err = proto.Marshal(proposal)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling proposal")
	}

	return proposalBytes, txID, nil
}

func createInvokeQueryProposal(identity Identity, cc ChainCode) (proposalBytes []byte, txID string, err error) {
	invocation := &pb.ChaincodeInvocationSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			Type:        pb.ChaincodeSpec_Type(cc.Type),
			ChaincodeId: &pb.ChaincodeID{Name: cc.Name},
			Input: &pb.ChaincodeInput{
				Args:   cc.toChainCodeArgs(),
				IsInit: cc.isInit,
			},
		},
	}

	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, "", errors.WithMessage(err, "error serializing identity")
	}

	prop, txID, err := protoutil.CreateChaincodeProposalWithTxIDAndTransient(cb.HeaderType_ENDORSER_TRANSACTION, cc.ChannelId, invocation, creator, txID, cc.TransientMap)
	if err != nil {
		return nil, "", errors.WithMessage(err, "error creating proposal")
	}
	proposalBytes, err = proto.Marshal(prop)
	if err != nil {
		return nil, "", errors.Wrap(err, "error marshaling proposal")
	}

	return proposalBytes, txID, nil
}
