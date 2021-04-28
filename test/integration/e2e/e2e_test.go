package e2e

import (
	"os"
	"path"
	"testing"
	"time"

	gohfc "github.com/zhj0811/gohfc/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policy    = "OR('Org1MSP.member', 'Org2MSP.member')"
	andpolicy = "AND('Org1MSP.member', 'Org2MSP.member')"
)

// TestE2E setup and run
func TestE2E(t *testing.T) {
	gohfcPath := os.Getenv("GOHFC_PATH")
	clientSDKFile := path.Join(gohfcPath, "./test/fixtures/client_sdk.yaml")
	sdk, err := gohfc.New(clientSDKFile)
	require.Nil(t, err, "init sdk failed")
	t.Log("Init sdk success...")

	t.Run("SetUpAndRun", func(t *testing.T) {
		time.Sleep(time.Second * 10)
		channeltxFile := path.Join(gohfcPath, "./test/fixtures/channel-artifacts/mychannel.tx")
		err = sdk.CreateUpdateChannel(channeltxFile, "mychannel")
		require.Nil(t, err, "create channel failed")
		t.Log("create channel success")

		//join channel
		time.Sleep(time.Second * 5)
		res, err := sdk.JoinChannel("mychannel", "admin@org1", "peer01")
		require.Nil(t, err, "org1 JoinChannel failed")
		t.Logf("org1 JoinChannel response:%#v", res)
		time.Sleep(time.Second * 3)
		res, err = sdk.JoinChannel("mychannel", "admin@org2", "peer02")
		require.Nil(t, err, "org2 JoinChannel failed")
		t.Logf("org2 JoinChannel response:%#v", res)

		//UpdateAnchorPeer
		time.Sleep(time.Second * 3)
		org1anchor := path.Join(gohfcPath, "./test/fixtures/channel-artifacts/Org1MSPanchors.tx")
		if err := sdk.UpdateAnchorPeer("admin@org1", org1anchor, "mychannel"); err != nil {
			t.Fatalf("org1 anchor update failed: %s", err.Error())
		}
		t.Log("org1 anchor update success")
		//
		time.Sleep(time.Second * 3)
		org2anchor := path.Join(gohfcPath, "./test/fixtures/channel-artifacts/Org2MSPanchors.tx")
		if err := sdk.UpdateAnchorPeer("admin@org2", org2anchor, "mychannel"); err != nil {
			t.Fatalf("org2 anchor update failed: %s", err.Error())
		}
		t.Log("org2 anchor update success")

		// install chaincode
		time.Sleep(time.Second * 10)
		installRequest := &gohfc.InstallRequest{
			ChannelId:        "mychannel",
			ChainCodeName:    "mycc",
			ChainCodeVersion: "1.0",
			ChainCodeType:    gohfc.ChaincodeSpec_GOLANG,
			Namespace:        "github.com/zhj0811/gohfc/test/fixtures/chaincode",
			SrcPath:          path.Join(os.Getenv("GOHFC_PATH"), "test/fixtures/chaincode"),
		}
		t.Logf("InstallChaincode installRequest.SrcPath:%s", installRequest.SrcPath)
		res, err = sdk.InstallChainCode(installRequest, "admin@org1", "peer01")
		require.Nil(t, err, "org2 InstallChaincode failed")
		t.Logf("org1 InstallChaincode response:%#v", res)
		time.Sleep(time.Second * 3)
		res, err = sdk.InstallChainCode(installRequest, "admin@org2", "peer02")
		require.Nil(t, err, "org2 InstallChaincode failed")
		t.Logf("org2 InstallChaincode response:%#v", res)

		time.Sleep(time.Second * 10)
		t.Log("Entering instantiate")
		instantiateRequest := &gohfc.ChainCode{
			ChannelId: "mychannel",
			Name:      "mycc",
			Version:   "1.0",
			Type:      gohfc.ChaincodeSpec_GOLANG,
			Args:      []string{"init", "a", "100", "b", "200"},
		}
		instantiateRes, err := sdk.InstantiateChainCode(instantiateRequest, policy)
		require.Nil(t, err, "InstantiateChainCode failed")
		t.Logf("InstantiateChainCode response:%#v", instantiateRes)

		time.Sleep(time.Second * 100)
		t.Log("Entering Queryfunc...")
		queryRes, err := sdk.Query([]string{"query", "a"}, nil, "mychannel", "")
		assert.Nil(t, err, "query failed")
		assert.Equal(t, "100", string(queryRes[0].Response.Response.Payload))

		time.Sleep(time.Second * 5)
		t.Log("Entering Invoke...")
		invokeRes, err := sdk.Invoke([]string{"invoke", "a", "b", "20"}, nil, "mychannel", "")
		assert.Nil(t, err, "invoke failed")
		t.Logf("Invoke response:%#v", invokeRes)

		time.Sleep(time.Second * 5)
		t.Log("Entering Queryfunc...")
		queryRes, err = sdk.Query([]string{"query", "a"}, nil, "mychannel", "")
		assert.Nil(t, err, "query failed")
		assert.Equal(t, "80", string(queryRes[0].Response.Response.Payload))

		block, err := sdk.GetBlockByTxID(invokeRes.TxID, "mychannel")
		assert.Nil(t, err)
		assert.Equal(t, 1, block.TxNum)
		assert.Equal(t, uint64(2), block.LastConfigBlock)
		assert.Equal(t, uint8(0), block.Transactions[0].ValidationCode)
		assert.Equal(t, "VALID", block.Transactions[0].ValidationCodeName)
		assert.NotNil(t, block.TxHash)
		assert.NotNil(t, block.BlockNum)
		assert.NotNil(t, block.BlockHash)
		assert.NotNil(t, block.PreBlockHash)
	})

	t.Run("test query block", func(t *testing.T) {
		t.Log("Entering GetBlockHeight...")
		blockHeight, err := sdk.GetBlockHeight("mychannel")
		assert.Nil(t, err)
		assert.Equal(t, uint64(5), blockHeight)

		t.Log("Entering GetBlockHeightByEventPeer...")
		blockHeight, err = sdk.GetBlockHeightByEventPeer("mychannel")
		assert.Nil(t, err)
		assert.Equal(t, uint64(5), blockHeight)

		t.Log("Entering GetBlockByNumber...")
		block, err := sdk.GetBlockByNumber(4, "mychannel")
		assert.Nil(t, err)
		assert.Equal(t, 1, block.TxNum)
		assert.Equal(t, uint64(2), block.LastConfigBlock)
		assert.Equal(t, uint8(0), block.Transactions[0].ValidationCode)
		assert.Equal(t, "VALID", block.Transactions[0].ValidationCodeName)
		assert.NotNil(t, block.TxHash)
		assert.NotNil(t, block.BlockNum)
		assert.NotNil(t, block.BlockHash)
		assert.NotNil(t, block.PreBlockHash)
	})
}

//TestRunWithoutSet 使用上述TestE2E生成的fabric环境，直接测试invoke&query func
func TestRunWithoutSet(t *testing.T) {
	t.Run("Run ", func(t *testing.T) {
		gohfcPath := os.Getenv("GOHFC_PATH")
		clientSDKFile := path.Join(gohfcPath, "./test/fixtures/client_sdk.yaml")
		sdk, err := gohfc.New(clientSDKFile)
		require.Nil(t, err, "init sdk failed")
		t.Log("Init sdk success...")

		time.Sleep(time.Second * 5)
		t.Log("Entering Queryfunc...")
		queryRes, err := sdk.Query([]string{"query", "a"}, nil, "mychannel", "")
		assert.Nil(t, err, "query failed")
		t.Logf("Query response:%s", string(queryRes[0].Response.Response.Payload))

		time.Sleep(time.Second * 10)
		t.Log("Entering Invoke...")
		invokeRes, err := sdk.Invoke([]string{"invoke", "b", "a", "10"}, nil, "mychannel", "")
		assert.Nil(t, err, "invoke failed")
		t.Logf("Invoke response:%v", invokeRes)

		time.Sleep(time.Second * 5)
		t.Log("Entering Queryfunc...")
		queryRes, err = sdk.Query([]string{"query", "a"}, nil, "mychannel", "")
		assert.Nil(t, err, "query failed")
		t.Logf("Query response:%s", string(queryRes[0].Response.Response.Payload))
	})
}
