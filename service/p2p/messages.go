//go:generate msgp

package p2p

import (
	"reflect"

	"github.com/fletaio/fleta_testnet/common"
	"github.com/fletaio/fleta_testnet/common/hash"
	"github.com/fletaio/fleta_testnet/core/types"
	"github.com/fletaio/fleta_testnet/encoding"
)

// message types
var (
	StatusMessageType          = types.DefineHashedType("p2p.StatusMessage")
	RequestMessageType         = types.DefineHashedType("p2p.RequestMessage")
	BlockMessageType           = types.DefineHashedType("p2p.BlockMessage")
	TransactionMessageType     = types.DefineHashedType("p2p.TransactionMessage")
	PeerListMessageType        = types.DefineHashedType("p2p.PeerListMessage")
	RequestPeerListMessageType = types.DefineHashedType("p2p.RequestPeerListMessage")
)

func init() {
	fc := encoding.Factory("transaction")
	encoding.Register([]*TransactionMessage{}, func(enc *encoding.Encoder, rv reflect.Value) error {
		item := rv.Interface().([]*TransactionMessage)
		Len := len(item)
		if err := enc.EncodeArrayLen(Len); err != nil {
			return err
		}
		for i := 0; i < Len; i++ {
			v := item[i]
			bs, err := types.EncodeTransaction(v.ChainID, v.Type, v.Tx)
			if err != nil {
				return err
			}
			if err := enc.EncodeBytes(bs); err != nil {
				return err
			}
			if err := enc.EncodeArrayLen(len(v.Signatures)); err != nil {
				return err
			}
			for _, sig := range v.Signatures {
				if err := enc.EncodeBytes(sig[:]); err != nil {
					return err
				}
			}
		}
		return nil
	}, func(dec *encoding.Decoder, rv reflect.Value) error {
		TxLen, err := dec.DecodeArrayLen()
		if err != nil {
			return err
		}
		if TxLen >= 65535 {
			return types.ErrInvalidTransactionCount
		}
		item := make([]*TransactionMessage, 0, TxLen)
		for i := 0; i < TxLen; i++ {
			bs, err := dec.DecodeBytes()
			if err != nil {
				return err
			}
			ChainID, tx, t, err := types.DecodeTransaction(fc, bs)
			if err != nil {
				return err
			}
			SigLen, err := dec.DecodeArrayLen()
			if err != nil {
				return err
			}
			sigs := make([]common.Signature, 0, SigLen)
			for j := 0; j < SigLen; j++ {
				var sig common.Signature
				if bs, err := dec.DecodeBytes(); err != nil {
					return err
				} else {
					copy(sig[:], bs)
				}
				sigs = append(sigs, sig)
			}
			item = append(item, &TransactionMessage{
				ChainID:    ChainID,
				Type:       t,
				Tx:         tx,
				Signatures: sigs,
				raw:        bs,
			})
		}

		rv.Set(reflect.ValueOf(item))
		return nil
	})
}

// RequestMessage used to request a chain data to a peer
type RequestMessage struct {
	Height uint32
	Count  uint8
}

// StatusMessage used to provide the chain information to a peer
type StatusMessage struct {
	Version  uint16
	Height   uint32
	LastHash hash.Hash256
}

// BlockMessage used to send a chain block to a peer
type BlockMessage struct {
	Blocks []*types.Block
}

// TransactionMessage is a message for a transaction
type TransactionMessage struct {
	ChainID    uint8
	Type       uint16
	Tx         types.Transaction
	Signatures []common.Signature
	raw        []byte
}

func (msg *TransactionMessage) Raw() []byte {
	return msg.raw
}

// PeerListMessage is a message for a peer list
type PeerListMessage struct {
	Ips   []string
	Hashs []string
}

// RequestPeerListMessage is a request message for a peer list
type RequestPeerListMessage struct {
}
