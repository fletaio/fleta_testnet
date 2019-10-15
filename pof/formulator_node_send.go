package pof

import (
	"time"

	"github.com/fletaio/fleta/common"
	"github.com/fletaio/fleta/service/p2p"
)

func (fr *FormulatorNode) sendMessage(Priority int, Target common.PublicHash, m interface{}) {
	fr.sendQueues[Priority].Push(&p2p.SendMessageItem{
		Target:  Target,
		Message: m,
	})
}

func (fr *FormulatorNode) sendMessagePacket(Priority int, Target common.PublicHash, bs []byte) {
	fr.sendQueues[Priority].Push(&p2p.SendMessageItem{
		Target: Target,
		Packet: bs,
	})
}

func (fr *FormulatorNode) broadcastMessagePacket(Priority int, bs []byte) {
	fr.sendQueues[Priority].Push(&p2p.SendMessageItem{
		Packet: bs,
	})
}

func (fr *FormulatorNode) limitCastMessage(Priority int, m interface{}) {
	fr.sendQueues[Priority].Push(&p2p.SendMessageItem{
		Message: m,
		Limit:   3,
	})
}

func (fr *FormulatorNode) exceptLimitCastMessage(Priority int, Target common.PublicHash, m interface{}) {
	fr.sendQueues[Priority].Push(&p2p.SendMessageItem{
		Target:  Target,
		Message: m,
		Limit:   3,
	})
}

func (fr *FormulatorNode) broadcastStatus() error {
	cp := fr.cs.cn.Provider()
	height, lastHash, _ := cp.LastStatus()
	nm := &p2p.StatusMessage{
		Version:  cp.Version(),
		Height:   height,
		LastHash: lastHash,
	}
	bs := p2p.MessageToPacket(nm)
	fr.ms.BroadcastPacket(bs)
	fr.broadcastMessagePacket(0, bs)
	return nil
}

func (fr *FormulatorNode) sendRequestBlockTo(TargetID string, Height uint32, Count uint8) error {
	nm := &p2p.RequestMessage{
		Height: Height,
		Count:  Count,
	}
	fr.ms.SendTo(TargetID, nm)
	for i := uint32(0); i < uint32(Count); i++ {
		fr.requestTimer.Add(Height+i, 2*time.Second, TargetID)
	}
	return nil
}

func (fr *FormulatorNode) sendRequestBlockToNode(TargetPubHash common.PublicHash, Height uint32, Count uint8) error {
	if TargetPubHash == fr.myPublicHash {
		return nil
	}

	nm := &p2p.RequestMessage{
		Height: Height,
		Count:  Count,
	}
	fr.sendMessage(0, TargetPubHash, nm)
	for i := uint32(0); i < uint32(Count); i++ {
		fr.requestNodeTimer.Add(Height+i, 10*time.Second, string(TargetPubHash[:]))
	}
	return nil
}
