# GOHFC - Golang Hyperledger Fabric Client

这是使用最低要求以纯Golang编写的Hyperledger Fabric的SDK。这不是官方的SDK，也不遵循Hyperledger团队提供的官方SDK API准则。有关官方SDK的列表，请参考官方Hyperledger文档。

它被设计为易于使用且速度最快。目前，它的性能远远超过了官方的Go SDK。此版本将得到更新和支持，因此非常欢迎提出请求和报告任何问题。

推荐的Go版本 >=1.11

此SDK已针对Hyperledger Fabric 1.4.x进行了测试。

## 安装

```
go get -u github.com/zhj0811/gohfc

```

## 基本概念

Gohfc提供了一个高级客户端，用于处理block，ledger，chaincode，channel和event的高级API。

一般流程是这样的：

- 使用docker-compose或任何其他适合您的工具启动Fabric。运行Fabric不是gohfc的责任。
- 使用`gohfc.CreateUpdateChannel`，通过将channel配置发送给orderer来创建一个或多个频道。
- 使用`gohfc.JoinChannel`，加入一个或多个peer到一个或多个channel。
- 使用`gohfc.InstallChainCode`，在一个或多个peer节点中安装一个或多个链码。
- 使用`gohfc.InstantiateChainCode`，实例化一个或多个已经安装的chaincode。
- 使用查询查询chaincode：`gohfc.Query`。这是只读操作。不会对区块链或ledger进行任何更改。
- 使用调用chaincode：`gohfc.Invoke`。该操作可以更新区块链和ledger。
- 使用`gohfc.ListenForFullBlock`或`gohfc.ListenForFilteredBlock`收听事件
- 还有更多的方法来获取特定的block，discover功能等。

## 初始化

可以从yaml文件初始化客户端。

### 关于MSPID
Fabric中的每个对等方和订购者都必须具有一组加密材料，例如根CA证书，中间证书，吊销证书列表等等。这组证书与ID关联，此ID称为MSP（成员服务提供商）。在每个操作中，都必须提供MSPID，以便对等方和订购者知道要加载并用于验证请求的加密材料集。

通常，MSP定义组织和组织内部具有角色的实体。几个MSP合并组成一个联盟，因此多个组织（每个组织都有自己的一组证书）可以一起工作。

因此，当发送对Fabric的任何请求时，此请求必须由Ecert签名（用户证书保留在中`gohfc.Identity`），并且必须提供MSPID，以便Fabric通过ID加载MSP，并验证此请求来自组织成员并且该成员具有适当的访问权限。

config file:

```

---
crypto:
  family: ecdsa
  algorithm: P256-SHA256
  hash: SHA2-256
orderers:
  orderer0:
    host: orderer.example.com:7050
    domainName: orderer.example.com
    useTLS: true
    tlsPath: /path/to/tls/server.crt
peers:
  peer01:
    host: peer0.org1.example.com:7051
    domainName: peer0.org1.example.com
    orgName: org1
    useTLS: true
    tlsPath: /path/to/tls/server.crt
  peer02:
    host: peer0.org2.example.com:9051
    domainName: peer0.org2.example.com
    orgName: org2
    useTLS: true
    tlsPath: /path/to/tls/server.crt
eventPeers:
  peer0:
    host: peer0.org1.example.com:7051
    domainName: peer0.org1.example.com
    orgName: org1
    useTLS: true
    tlsPath: /path/to/tls/server.crt
users:
  _default:
    mspConfigPath: /path/to/users/msp
    mspID: Org1MSP
  admin@org1:
    mspConfigPath: /path/to/users/msp
    mspID: Org1MSP
  admin@org2:
    mspConfigPath: /path/to/users/msp
    mspID: Org2MSP
channel:
  channelId:           mychannel
  chaincodeName:       mycc
  chaincodeVersion:    1.0
  chaincodePolicy:
    orgs:
      - org1
      - org2
    rule: or
discovery:
  host: peer0.org1.example.com:7051
  domainName: peer0.org1.example.com
  config:
    version: 0
    tlsconfig:
      certpath: ""
      keypath: ""
      peercacertpath: /path/to/users/tls/ca.crt
      timeout: 0s
    signerconfig:
      mspid: Org1MSP
      identitypath: /path/to/usersmsp/signcerts/cert.pem
      keypath: /path/to/users/msp/keystore/_sk


```

从配置文件初始化：

```
c, err := gohfc.New("./client_sdk.yaml")
if err != nil {
    fmt.Printf("Error loading file: %v", err)
	os.Exit(1)
}

```

### 安装 chaincode

安装新的链码时，`gohfc.InstallRequest`必须提供一个类型为struct的结构：

```

installRequest := &gohfc.InstallRequest{
    ChannelId:        "mychannel",
    ChainCodeName:    "mycc",
    ChainCodeVersion: "1.0",
    ChainCodeType:    gohfc.ChaincodeSpec_GOLANG,
    Namespace:        "github.com/zhj0811/gohfc/test/fixtures/chaincode",
    SrcPath:          path.Join(os.Getenv("GOHFC_PATH"), "test/fixtures/chaincode"),
}

```

Fabric将支持用不同语言编写的链码，因此必须使用`ChainCodeType`--Gohfc指定语言类型。现在仅支持Go。其他代码语言将在以后添加。

`ChannelId` 是安装chaincode的通道名称。

`ChainCodeName` 是chaincode的名称。此名称将在以后的请求（查询，调用等）中使用，以指定必须执行的链码。一个频道可以有多个链码。名称在频道的上下文中必须是唯一的。

`ChainCodeVersion` 指定版本。

Gohfc设计为无需Go环境即可工作。因此，当用户尝试安装chaincode时必须提供`Namespace`,`SrcPath` and `Libraries`（可选）

`Namespace` 是Go命名空间，它将在Fabric运行时中"install"chaincode。比如 `github.com/some/code`

`SrcPath` 是源代码所在的绝对路径，为打包安装做准备。 

这种分离使gohfc可以在没有任何外部运行时依赖项的情况下运行。

`Libraries` 是链表打包中将包含的库的可选列表。遵循`Namespace`和`SrcPath`同样的逻辑。

## TODO
- 简单的相互TLS认证

- ... ...


### 可用的加密算法

| Family   | Algorithm   | Description                                      | 
|:--------:|:-----------:|--------------------------------------------------| 
| ecdsa    | P256-SHA256 | Elliptic curve is P256 and signature uses SHA256 |
| ecdsa    | P384-SHA384 | Elliptic curve is P384 and signature uses SHA384 |
| ecdsa    | P521-SHA512 | Elliptic curve is P521 and signature uses SHA512 |
| rsa      | ----        | RSA is not supported in Fabric                   |

### Hash

| Family    | 
|:----------| 
| SHA2-256  |
| SHA2-384  |
| SHA3-256  |
| SHA3-384  |
