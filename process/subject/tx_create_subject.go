package subject

import (
	"bytes"
	"encoding/json"

	"github.com/fletaio/fleta_testnet/process/study"
	"github.com/fletaio/fleta_testnet/process/user"
	"github.com/fletaio/fleta_testnet/common"
	"github.com/fletaio/fleta_testnet/common/hash"
	"github.com/fletaio/fleta_testnet/core/types"
)

// CreateSubject is a CreateSubject
type CreateSubject struct {
	Timestamp_      uint64
	Seq_            uint64
	From_           common.Address
	UserID          string
	SubjectID       string
	ScreeningNumber string
	Initial         string
	PasswordHash    hash.Hash256
}

// Timestamp returns the timestamp of the transaction
func (tx *CreateSubject) Timestamp() uint64 {
	return tx.Timestamp_
}

// Seq returns the sequence of the transaction
func (tx *CreateSubject) Seq() uint64 {
	return tx.Seq_
}

// From returns the from address of the transaction
func (tx *CreateSubject) From() common.Address {
	return tx.From_
}

// Validate validates signatures of the transaction
func (tx *CreateSubject) Validate(p types.Process, loader types.LoaderWrapper, signers []common.PublicHash) error {
	sp := p.(*Subject)

	if len(tx.SubjectID) == 0 {
		return ErrInvalidSubjectID
	}
	if tx.Seq() <= loader.Seq(tx.From()) {
		return types.ErrInvalidSequence
	}
	if !sp.user.IsUserRole(loader, tx.From(), tx.UserID, []string{"CRC", "SUBI", "PI"}) {
		return user.ErrInvalidRole
	}
	if sp.HasSubject(loader, tx.From(), tx.SubjectID) {
		return ErrExistSubject
	}

	fromAcc, err := loader.Account(tx.From())
	if err != nil {
		return err
	}
	if _, is := fromAcc.(*study.SiteAccount); !is {
		return study.ErrNotSiteAccount
	}
	if err := fromAcc.Validate(loader, signers); err != nil {
		return err
	}
	return nil
}

// Execute updates the context by the transaction
func (tx *CreateSubject) Execute(p types.Process, ctw *types.ContextWrapper, index uint16) error {
	sp := p.(*Subject)

	sp.addSubject(ctw, tx.From(), tx.SubjectID)

	return nil
}

// MarshalJSON is a marshaler function
func (tx *CreateSubject) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(`{`)
	buffer.WriteString(`"timestamp":`)
	if bs, err := json.Marshal(tx.Timestamp_); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"seq":`)
	if bs, err := json.Marshal(tx.Seq_); err != nil {
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
	buffer.WriteString(`"user_id":`)
	if bs, err := json.Marshal(tx.UserID); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"subject_id":`)
	if bs, err := json.Marshal(tx.SubjectID); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"screening_number":`)
	if bs, err := json.Marshal(tx.ScreeningNumber); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"initial":`)
	if bs, err := json.Marshal(tx.Initial); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`,`)
	buffer.WriteString(`"password_hash":`)
	if bs, err := tx.PasswordHash.MarshalJSON(); err != nil {
		return nil, err
	} else {
		buffer.Write(bs)
	}
	buffer.WriteString(`}`)
	return buffer.Bytes(), nil
}
