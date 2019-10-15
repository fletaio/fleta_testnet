package p2p

import (
	"time"

	"github.com/fletaio/fleta/common"
)

func (nd *Node) sendMessage(Priority int, Target common.PublicHash, m interface{}) {
	nd.sendQueues[Priority].Push(&SendMessageItem{
		Target:  Target,
		Message: m,
	})
}

func (nd *Node) sendMessagePacket(Priority int, Target common.PublicHash, raw []byte, Height uint32) {
	nd.sendQueues[Priority].Push(&SendMessageItem{
		Target: Target,
		Packet: raw,
		Height: Height,
	})
}

func (nd *Node) broadcastMessage(Priority int, m interface{}) {
	nd.sendQueues[Priority].Push(&SendMessageItem{
		Message: m,
	})
}

func (nd *Node) limitCastMessage(Priority int, m interface{}) {
	nd.sendQueues[Priority].Push(&SendMessageItem{
		Message: m,
		Limit:   3,
	})
}

func (nd *Node) exceptLimitCastMessage(Priority int, Target common.PublicHash, m interface{}) {
	nd.sendQueues[Priority].Push(&SendMessageItem{
		Target:  Target,
		Message: m,
		Limit:   3,
	})
}

func (nd *Node) sendStatusTo(TargetPubHash common.PublicHash) error {
	if TargetPubHash == nd.myPublicHash {
		return nil
	}

	cp := nd.cn.Provider()
	height, lastHash, _ := cp.LastStatus()
	nm := &StatusMessage{
		Version:  cp.Version(),
		Height:   height,
		LastHash: lastHash,
	}
	nd.sendMessage(0, TargetPubHash, nm)
	return nil
}

func (nd *Node) broadcastStatus() error {
	cp := nd.cn.Provider()
	height, lastHash, _ := cp.LastStatus()
	nm := &StatusMessage{
		Version:  cp.Version(),
		Height:   height,
		LastHash: lastHash,
	}
	nd.ms.BroadcastMessage(nm)
	return nil
}

func (nd *Node) sendRequestBlockTo(TargetPubHash common.PublicHash, Height uint32, Count uint8) error {
	if TargetPubHash == nd.myPublicHash {
		return nil
	}

	nm := &RequestMessage{
		Height: Height,
		Count:  Count,
	}
	nd.sendMessage(0, TargetPubHash, nm)
	for i := uint32(0); i < uint32(Count); i++ {
		nd.requestTimer.Add(Height+i, 10*time.Second, string(TargetPubHash[:]))
	}
	return nil
}
