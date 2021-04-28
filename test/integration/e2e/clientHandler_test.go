package e2e

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	gohfc "github.com/zhj0811/gohfc/pkg"
	"github.com/stretchr/testify/require"
)

//TestFabricClientHandler 测试wischain创建通道、加入通道、安装实例化智能合约功能
func TestFabricClientHandler(t *testing.T) {
	config := newClientHandlerConfig(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, err := gohfc.NewFabricClientHandler(ctx, config)
	require.NoError(t, err)

	env, err := ioutil.ReadFile(gohfc.Subst(channeltx))
	require.NoError(t, err)
	err = client.CreateUpdateChannel(env, "mychannel")
	require.NoError(t, err)
	t.Log("create channel success")

	err = client.JoinChannel("mychannel")
	require.Nil(t, err, " JoinChannel failed")
	t.Logf("JoinChannel success")

	anchorEnv, err := ioutil.ReadFile(gohfc.Subst(org1Anchortx))
	require.Nil(t, err, " read org1Anchortx failed")
	err = client.CreateUpdateChannel(anchorEnv, "mychannel")
	require.NoError(t, err)
	t.Log("update anchor peer success")

	//intall cc with cc.out
	ccPack, err := ioutil.ReadFile(gohfc.Subst(ccout))
	require.Nil(t, err, "read cc.out failed")
	err = client.InstallCCByPack(ccPack)
	require.Nil(t, err, "install cc.out failed")
	t.Log("install cc.out success")

	t.Log("Entering instantiate")
	instantiateRequest := &gohfc.ChainCode{
		ChannelId: "mychannel",
		Name:      "mycc",
		Version:   "1.0",
		Type:      gohfc.ChaincodeSpec_GOLANG,
		Args:      []string{"init", "a", "100", "b", "200"},
	}
	err = client.InstantiateChainCode(instantiateRequest, "deploy", policy)
	require.Nil(t, err, "InstantiateChainCode failed")
	t.Log("InstantiateChainCode success")

	time.Sleep(time.Second * 3)
	installRequest := &gohfc.InstallRequest{
		//ChannelId:        "mychannel",
		ChainCodeName:    "mycc",
		ChainCodeVersion: "2.0",
		ChainCodeType:    gohfc.ChaincodeSpec_GOLANG,
		Namespace:        "github.com/PeerFintech/chaincode",
		//SrcPath:          path.Join(os.Getenv("GOHFC_PATH"), "test/fixtures/chaincode"),
	}
	fReader, err := os.Open(gohfc.Subst(codetgz))
	defer fReader.Close()
	require.Nil(t, err, "open code.tar.gz failed")
	err = client.InstallCCByCodeTar(fReader, installRequest)
	require.Nil(t, err, "install code.tar.gz 2.0 failed")
	t.Log("install code.tar.gz 2.0 success")
	time.Sleep(time.Second * 3)
	t.Log("Entering upgrade chaincode")
	instantiateRequest = &gohfc.ChainCode{
		ChannelId: "mychannel",
		Name:      "mycc",
		Version:   "2.0",
		Type:      gohfc.ChaincodeSpec_GOLANG,
		Args:      []string{"init", "a", "100", "b", "200"},
	}
	err = client.InstantiateChainCode(instantiateRequest, "upgrade", policy)
	require.Nil(t, err, "upgrade ChainCode failed")
	t.Log("upgrade ChainCode success")
}

func newClientHandlerConfig(t *testing.T) gohfc.ClientHandlerConfig {
	preEnv(t)
	tlsBytes, err := ioutil.ReadFile(gohfc.Subst(tlsPath))
	require.NoError(t, err)
	peerConf := gohfc.PeerByteConfig{
		Url:        "localhost:7051",
		DomainName: "peer0.org1.example.com",
		TlsBytes:   tlsBytes,
		UseTLS:     true,
	}

	orderertlsBytes, err := ioutil.ReadFile(gohfc.Subst(orderertlsPath))
	require.NoError(t, err)
	orConf := gohfc.OrdererByteConfig{
		Url:        "localhost:7050",
		DomainName: "orderer.example.com",
		TlsBytes:   orderertlsBytes,
		UseTLS:     true,
	}

	cert, prikey, err := findCertAndKeyFile(gohfc.Subst(mspPath))
	require.NoError(t, err)
	certBytes, err := ioutil.ReadFile(cert)
	require.NoError(t, err)
	keyBytes, err := ioutil.ReadFile(prikey)
	require.NoError(t, err)
	userConf := gohfc.UserByteConfig{
		CertBytes: certBytes,
		KeyBytes:  keyBytes,
		MspID:     "Org1MSP",
	}
	return gohfc.ClientHandlerConfig{
		//CryptoConfig: gohfc.CryptoConfig{
		//	Family:    "ecdsa",
		//	Algorithm: "P256-SHA256",
		//	Hash:      "SHA2-256",
		//},
		PeerByteConfig:    &peerConf,
		OrdererByteConfig: &orConf,
		UserByteConfig:    &userConf,
	}

}
