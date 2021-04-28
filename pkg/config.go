/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hyperledger/fabric/cmd/common"
	"gopkg.in/yaml.v2"
)

// ClientConfig holds config data for crypto, peers and orderers
type ClientConfig struct {
	CryptoConfig  `yaml:"crypto"`
	Orderers      map[string]OrdererConfig `yaml:"orderers"`
	Peers         map[string]PeerConfig    `yaml:"peers"`
	EventPeers    map[string]PeerConfig    `yaml:"eventPeers"`
	Users         map[string]UserConfig    `yaml:"users"`
	ChannelConfig `yaml:"channel"`
	Discovery     *DiscoveryConfig `yaml:"discovery"`
}

type UserConfig struct {
	MspConfigPath string `yaml:"mspConfigPath"`
	MspID         string `yaml:"mspID"`
}

type ChannelConfig struct {
	ChannelId        string `yaml:"channelId"`
	ChaincodeName    string `yaml:"chaincodeName"`
	ChaincodeVersion string `yaml:"chaincodeVersion"`
	ChaincodePolicy  `yaml:"chaincodePolicy"`
	TlsMutual        bool   `yaml:"tlsMutual"`
	ClientCert       string `yaml:"clientCert"`
	ClientKey        string `yaml:"clientKey"`
}

type ChaincodePolicy struct {
	Orgs []string `yaml:"orgs"`
	Rule string   `yaml:"rule"`
}

// CAConfig holds config for Fabric CA
type CAConfig struct {
	CryptoConfig      `yaml:"crypto"`
	Uri               string `yaml:"url"`
	SkipTLSValidation bool   `yaml:"skipTLSValidation"`
	MspId             string `yaml:"mspId"`
}

// Config holds config values for fabric and fabric-ca cryptography
type CryptoConfig struct {
	Family    string `yaml:"family"`
	Algorithm string `yaml:"algorithm"`
	Hash      string `yaml:"hash"`
}

// PeerConfig hold config values for Peer. ULR is in address:port notation
type PeerConfig struct {
	Host       string `yaml:"host"`
	OrgName    string `yaml:"orgName"`
	UseTLS     bool   `yaml:"useTLS"`
	TlsPath    string `yaml:"tlsPath"`
	DomainName string `yaml:"domainName"`
	TlsMutual  bool   `yaml:"tlsMutual"`
	ClientCert string `yaml:"clientCert"`
	ClientKey  string `yaml:"clientKey"`
}

// OrdererConfig hold config values for Orderer. ULR is in address:port notation
type OrdererConfig struct {
	Host       string `yaml:"host"`
	UseTLS     bool   `yaml:"useTLS"`
	TlsPath    string `yaml:"tlsPath"`
	DomainName string `yaml:"domainName"`
	TlsMutual  bool   `yaml:"tlsMutual"`
	ClientCert string `yaml:"clientCert"`
	ClientKey  string `yaml:"clientKey"`
}

// DiscoveryConfig discovery配置，用于传统的读取配置文件
type DiscoveryConfig struct {
	Host       string        `yaml:"host"`
	DomainName string        `yaml:"domainName"`
	Config     common.Config `yaml:"config"`
}

// SimpleDiscoveryConfig discovery配置，用于接收传递过来的配置信息
type SimpleDiscoveryConfig struct {
	Host         string
	DomainName   string
	TLSConfig    DiscoveryTLSConfig
	SignerConfig DiscoverySignerConfig
}

type DiscoveryTLSConfig struct {
	Cert       []byte
	Key        []byte
	PeerCACert []byte
	Timeout    time.Duration
}

type DiscoverySignerConfig struct {
	MSPID    string
	Identity []byte
	Key      []byte
}

// newFabricClientConfig create config from provided yaml file in path
func newClientConfig(path string) (*ClientConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := new(ClientConfig)
	err = yaml.Unmarshal([]byte(data), config)
	if err != nil {
		return nil, err
	}
	formatClientConfig(config)
	return config, nil
}

func formatClientConfig(config *ClientConfig) {
	for key, orderer := range config.Orderers {
		orderer.TlsPath = Subst(orderer.TlsPath)
		orderer.ClientCert = Subst(orderer.ClientCert)
		orderer.ClientKey = Subst(orderer.ClientKey)
		config.Orderers[key] = orderer
	}

	for key, peer := range config.Peers {
		peer.TlsPath = Subst(peer.TlsPath)
		peer.ClientCert = Subst(peer.ClientCert)
		peer.ClientKey = Subst(peer.ClientKey)
		config.Peers[key] = peer
	}

	for key, peer := range config.EventPeers {
		peer.TlsPath = Subst(peer.TlsPath)
		peer.ClientCert = Subst(peer.ClientCert)
		peer.ClientKey = Subst(peer.ClientKey)
		config.EventPeers[key] = peer
	}

	for key, user := range config.Users {
		user.MspConfigPath = Subst(user.MspConfigPath)
		config.Users[key] = user
	}
	config.ChannelConfig.ClientKey = Subst(config.ChannelConfig.ClientKey)
	config.ChannelConfig.ClientCert = Subst(config.ChannelConfig.ClientCert)

	config.Discovery.Config.SignerConfig.IdentityPath = Subst(config.Discovery.Config.SignerConfig.IdentityPath)
	config.Discovery.Config.SignerConfig.KeyPath = Subst(config.Discovery.Config.SignerConfig.KeyPath)
	config.Discovery.Config.TLSConfig.KeyPath = Subst(config.Discovery.Config.TLSConfig.KeyPath)
	config.Discovery.Config.TLSConfig.PeerCACertPath = Subst(config.Discovery.Config.TLSConfig.PeerCACertPath)
	config.Discovery.Config.TLSConfig.CertPath = Subst(config.Discovery.Config.TLSConfig.CertPath)
	return
}

// Subst replaces instances of '${VARNAME}' (eg ${GOPATH}) with the variable.
// Variables names that are not set by the SDK are replaced with the environment variable.
func Subst(path string) string {
	const (
		sepPrefix = "${"
		sepSuffix = "}"
	)

	splits := strings.Split(path, sepPrefix)

	var buffer bytes.Buffer

	// first split precedes the first sepPrefix so should always be written
	buffer.WriteString(splits[0]) // nolint: gas

	for _, s := range splits[1:] {
		subst, rest := substVar(s, sepPrefix, sepSuffix)
		buffer.WriteString(subst) // nolint: gas
		buffer.WriteString(rest)  // nolint: gas
	}

	return buffer.String()
}

// substVar searches for an instance of a variables name and replaces them with their value.
// The first return value is substituted portion of the string or noMatch if no replacement occurred.
// The second return value is the unconsumed portion of s.
func substVar(s string, noMatch string, sep string) (string, string) {
	endPos := strings.Index(s, sep)
	if endPos == -1 {
		return noMatch, s
	}

	v, ok := os.LookupEnv(s[:endPos])
	if !ok {
		return noMatch, s
	}

	return v, s[endPos+1:]
}

// NewCAConfig create new Fabric CA config from provided yaml file in path
func NewCAConfig(path string) (*CAConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := new(CAConfig)
	err = yaml.Unmarshal([]byte(data), config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
