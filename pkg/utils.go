/*
Copyright: PeerFintech. All Rights Reserved.
*/

package gohfc

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"
)

func getChainCodeObj(args []string, transientMap map[string][]byte, channelName, chaincodeName string) (*ChainCode, error) {
	if len(channelName) == 0 {
		channelName = handler.client.Channel.ChannelId
	}
	if len(chaincodeName) == 0 {
		chaincodeName = handler.client.Channel.ChaincodeName
	}
	if channelName == "" || chaincodeName == "" {
		return nil, fmt.Errorf("channelName or chaincodeName is empty")
	}

	chaincode := ChainCode{
		ChannelId:    channelName,
		Type:         ChaincodeSpec_GOLANG,
		Name:         chaincodeName,
		Args:         args,
		TransientMap: transientMap,
	}

	return &chaincode, nil
}

//解析背书策略
func parsePolicy() error {
	policyOrgs := handler.client.Channel.Orgs
	policyRule := handler.client.Channel.Rule
	if len(policyOrgs) == 0 || policyRule == "" {
		for _, v := range handler.client.Peers {
			peerNames = append(peerNames, v.Name)
		}
	}

	for ordname := range handler.client.Orderers {
		orderNames = append(orderNames, ordname)
	}

	for _, v := range handler.client.EventPeers {
		eventPeer = v.Name
		break
	}

	if len(policyOrgs) > 0 && policyRule != "" {
		for _, v := range handler.client.Peers {
			if containsStr(policyOrgs, v.OrgName) {
				orgPeerMap[v.OrgName] = append(orgPeerMap[v.OrgName], v.Name)
				if policyRule == "or" {
					orRulePeerNames = append(orRulePeerNames, v.Name)
				}
			}
		}
		return nil
	}

	return fmt.Errorf("No peers are configured")
}

func getSendOrderName() string {
	return orderNames[generateRangeNum(0, len(orderNames))]
}

func getSendPeerName() []string {
	if len(orRulePeerNames) > 0 {
		return []string{orRulePeerNames[generateRangeNum(0, len(orRulePeerNames))]}
	}
	if len(peerNames) > 0 {
		return peerNames
	}
	var sendNameList []string
	policyRule := handler.client.Channel.Rule
	if policyRule == "and" {
		for _, peerNames := range orgPeerMap {
			sendNameList = append(sendNameList, peerNames[generateRangeNum(0, len(peerNames))])
			continue
		}
	}

	return sendNameList
}

func generateRangeNum(min, max int) int {
	rand.Seed(time.Now().Unix())
	randNum := rand.Intn(max-min) + min
	return randNum
}

func containsStr(strList []string, str string) bool {
	for _, v := range strList {
		if v == str {
			return true
		}
	}
	return false
}

// isNum 判断字符串内容是否为数字
func isNum(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// isDomainName 判断地址是否为域名
func isDomainName(s string) bool {
	if s == "localhost" {
		return false
	}

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			continue
		default:
			if !isNum(string(s[i])) {
				return true
			}
		}
	}

	return false
}

// isIPAdress 判断IP地址是否正确
func isIPAdress(s string) bool {
	if s == "localhost" {
		return true
	}

	ip := net.ParseIP(s)
	return ip != nil
}
