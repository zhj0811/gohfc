package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"testing"

	gohfc "github.com/zhj0811/gohfc/pkg"
	"github.com/stretchr/testify/require"
)

const (
	tlsPath        = "${GOHFC_PATH}/test/fixtures/crypto-config/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt"
	org2tlsPath    = "${GOHFC_PATH}/test/fixtures/crypto-config/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/server.crt"
	orderertlsPath = "${GOHFC_PATH}/test/fixtures/crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt"
	mspPath        = "${GOHFC_PATH}/test/fixtures/crypto-config/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
	clientPath     = "${GOHFC_PATH}/test/fixtures/client_sdk.yaml"
	channeltx      = "${GOHFC_PATH}/test/fixtures/channel-artifacts/mychannel.tx"
	ccout          = "${GOHFC_PATH}/test/fixtures/example.out"
	codetgz        = "${GOHFC_PATH}/test/fixtures/chaincode.tar.gz"
	abstore        = "${GOHFC_PATH}/test/fixtures/abstore.tar.gz"
	org1Anchortx   = "${GOHFC_PATH}/test/fixtures/channel-artifacts/Org1MSPanchors.tx"
)

func TestEventProvider_ListenEventFullBlock(t *testing.T) {
	eventConf := newEventConfig(t)
	ep, err := gohfc.NewEventClient(context.Background(), &eventConf)
	require.NoError(t, err)
	ch, err := ep.ListenEventFullBlock("mychannel", 2)
	require.NoError(t, err)

	go func() {
		for {
			block := <-ch
			t.Logf("receive Block %#v", block)
		}
	}()
	select {}
}

func TestEventProvider_ListenEventFilterBlock(t *testing.T) {
	eventConf := newEventConfig(t)
	ep, err := gohfc.NewEventClient(context.Background(), &eventConf)
	require.NoError(t, err)
	ch, err := ep.ListenEventFilterBlock("mychannel", 2)
	require.NoError(t, err)
	sig := make(chan os.Signal)
	signal.Notify(sig)
	go func() {
		for {
			block := <-ch
			if block.Error != nil {
				t.Logf("receive Block Error: %s", block.Error.Error())
			} else {
				t.Logf("receive BlockNumber %d", block.BlockHeight)
			}
			t.Logf("receive Block %#v", block)
		}
	}()

	select {
	case <-sig:
		t.Log("exit success")
	}
}

func preEnv(t *testing.T) {
	if gohfcPath := os.Getenv("GOHFC_PATH"); len(gohfcPath) == 0 {
		err := os.Setenv("GOHFC_PATH", "../../../")
		require.NoError(t, err)
	}
	return
}

func newEventConfig(t *testing.T) gohfc.EventConfig {
	preEnv(t)
	tlsBytes, err := ioutil.ReadFile(gohfc.Subst(tlsPath))
	require.NoError(t, err)
	peerConf := gohfc.PeerByteConfig{
		Url:        "localhost:7051",
		DomainName: "peer0.org1.example.com",
		TlsBytes:   tlsBytes,
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
	return gohfc.EventConfig{
		CryptoConfig: gohfc.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		PeerByteConfig: peerConf,
		UserByteConfig: userConf,
	}

}

func findCertAndKeyFile(msppath string) (string, string, error) {
	findCert := func(path string) (string, error) {
		list, err := ioutil.ReadDir(path)
		if err != nil {
			return "", err
		}
		var file os.FileInfo
		for _, item := range list {
			if !item.IsDir() {
				if file == nil {
					file = item
				} else if item.ModTime().After(file.ModTime()) {
					file = item
				}
			}
		}
		if file == nil {
			return "", fmt.Errorf("have't file in the %s", path)
		}
		return filepath.Join(path, file.Name()), nil
	}
	prikey, err := findCert(filepath.Join(msppath, "keystore"))
	if err != nil {
		return "", "", err
	}
	cert, err := findCert(filepath.Join(msppath, "signcerts"))
	if err != nil {
		return "", "", err
	}
	return cert, prikey, nil
}

func TestEventClient_ListenEventFullBlock(t *testing.T) {
	sdk, err := gohfc.New(gohfc.Subst(clientPath))
	require.Nil(t, err, "init sdk failed")
	t.Log("Init sdk success...")

	ch, err := sdk.ListenEventFullBlock("mychannel", 2)
	require.NoError(t, err)

	go func() {
		for {
			block := <-ch
			t.Logf("receive Block %#v", block)
		}
	}()
	select {}
}

func TestEventClient_ListenEventFilterBlock(t *testing.T) {
	sdk, err := gohfc.New(gohfc.Subst(clientPath))
	require.Nil(t, err, "init sdk failed")
	t.Log("Init sdk success...")

	ch, err := sdk.ListenEventFilterBlock("mychannel", 2)
	require.NoError(t, err)

	go func() {
		for {
			block := <-ch
			if block.Error != nil {
				t.Logf("receive Block Error: %s", block.Error.Error())
			} else {
				t.Logf("receive BlockNumber %d", block.BlockHeight)
			}
			t.Logf("receive Block %#v", block)
		}
	}()
	select {}
}
