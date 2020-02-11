package vault

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/fletaio/fleta_testnet/encoding"

	"github.com/fletaio/fleta_testnet/common"
	"github.com/fletaio/fleta_testnet/common/amount"
	"github.com/fletaio/fleta_testnet/core/types"
)

func init() {
	encoding.Register(Transfer{}, func(enc *encoding.Encoder, rv reflect.Value) error {
		item := rv.Interface().(Transfer)
		if err := enc.EncodeUint64(item.Timestamp_); err != nil {
			return err
		}
		if err := enc.EncodeBytes(item.From_[:]); err != nil {
			return err
		}
		if err := enc.EncodeBytes(item.To[:]); err != nil {
			return err
		}
		if err := enc.EncodeBytes(item.Amount.Bytes()); err != nil {
			return err
		}
		return nil
	}, func(dec *encoding.Decoder, rv reflect.Value) error {
		item := &Transfer{}
		if ts, err := dec.DecodeUint64(); err != nil {
			return err
		} else {
			item.Timestamp_ = ts
		}
		if bs, err := dec.DecodeBytes(); err != nil {
			return err
		} else {
			copy(item.From_[:], bs)
		}
		if bs, err := dec.DecodeBytes(); err != nil {
			return err
		} else {
			copy(item.To[:], bs)
		}
		if bs, err := dec.DecodeBytes(); err != nil {
			return err
		} else {
			item.Amount = amount.NewAmountFromBytes(bs)
		}
		rv.Set(reflect.ValueOf(item).Elem())
		return nil
	})
}

// Transfer is a Transfer
type Transfer struct {
	Timestamp_ uint64
	From_      common.Address
	To         common.Address
	Amount     *amount.Amount
}

// Timestamp returns the timestamp of the transaction
func (tx *Transfer) Timestamp() uint64 {
	return tx.Timestamp_
}

// From returns the from address of the transaction
func (tx *Transfer) From() common.Address {
	return tx.From_
}

// Fee returns the fee of the transaction
func (tx *Transfer) Fee(p types.Process, loader types.LoaderWrapper) *amount.Amount {
	sp := p.(*Vault)
	return sp.GetDefaultFee(loader)
}

// Validate validates signatures of the transaction
func (tx *Transfer) Validate(p types.Process, loader types.LoaderWrapper, signers []common.PublicHash) error {
	sp := p.(*Vault)

	if tx.Amount.Less(amount.COIN.DivC(10)) {
		return types.ErrDustAmount
	}

	if has, err := loader.HasAccount(tx.To); err != nil {
		return err
	} else if !has {
		return types.ErrNotExistAccount
	}

	fromAcc, err := loader.Account(tx.From())
	if err != nil {
		return err
	}
	if err := fromAcc.Validate(loader, signers); err != nil {
		return err
	}

	if err := sp.CheckFeePayableWith(p, loader, tx, tx.Amount); err != nil {
		return err
	}
	return nil
}

// Execute updates the context by the transaction
func (tx *Transfer) Execute(p types.Process, ctw *types.ContextWrapper, index uint16) error {
	sp := p.(*Vault)

	return sp.WithFee(p, ctw, tx, func() error {
		if err := sp.SubBalance(ctw, tx.From(), tx.Amount); err != nil {
			return err
		}
		if err := sp.AddBalance(ctw, tx.To, tx.Amount); err != nil {
			return err
		}
		return nil
	})
}

// MarshalJSON is a marshaler function
func (tx *Transfer) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(`{`)
	buffer.WriteString(`"timestamp":`)
	if bs, err := json.Marshal(tx.Timestamp_); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"from":`)
	if bs, err := tx.From_.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"to":`)
	if bs, err := tx.To.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"amount":`)
	if bs, err := tx.Amount.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`}`)
	return buffer.Bytes(), nil
}
