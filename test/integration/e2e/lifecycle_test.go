package e2e

import (
	"context"
	"io/ioutil"
	"testing"
	"time"

	gohfc "github.com/zhj0811/gohfc/pkg"
	"github.com/stretchr/testify/require"
)

func TestLifecycle(t *testing.T) {
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

	time.Sleep(3 * time.Second)
	err = client.JoinChannel("mychannel")
	require.Nil(t, err, " JoinChannel failed")
	t.Logf("JoinChannel success")

	time.Sleep(3 * time.Second)
	anchorEnv, err := ioutil.ReadFile(gohfc.Subst(org1Anchortx))
	require.Nil(t, err, " read org1Anchortx failed")
	err = client.CreateUpdateChannel(anchorEnv, "mychannel")
	require.NoError(t, err)
	t.Log("update anchor peer success")

	//peer lifecycle install chaincode abstore.tar.gz
	_, err = client.LifecycleInstall(gohfc.Subst(abstore))
	require.Nil(t, err, "install abstore.tar.gz failed")
	t.Log("install abstore.tar.gz success")

	t.Log("Entering approve for my org")
	pkgID := "mycc_1:0ba4fbd04456b4ccf4096ba343ac77148e9131b1dd67abc5f3f5d37d02b52fdb"
	err = client.ApproveForMyOrg("mychannel", "mycc", "1.0", pkgID, "", 1, true)
	require.Nil(t, err, "approve for my org failed")
	t.Log("approve for my org success")

	time.Sleep(3 * time.Second)

	t.Log("Entering lifecycle commit")
	err = client.LifecycleCommit("mychannel", "mycc", "1.0", pkgID, "", 1, true)
	require.Nil(t, err, "lifecycle commit failed")
	t.Log("lifecycle commit success")

	time.Sleep(3 * time.Second)

	t.Log("Entering init invoke")
	args := []string{"init", "a", "100", "b", "200"}
	_, err = client.InvokeOrQuery(args, nil, "mychannel", "mycc", true, false)
	require.Nil(t, err, "init invoke failed")
	t.Log("init invoke success")

}
