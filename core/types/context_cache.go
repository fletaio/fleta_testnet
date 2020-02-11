package types

import (
	"github.com/fletaio/fleta_testnet/common"
	"github.com/fletaio/fleta_testnet/common/hash"
)

type contextCache struct {
	ctx            *Context
	AccountMap     map[common.Address]Account
	AccountNameMap map[string]common.Address
	AccountDataMap map[string][]byte
	ProcessDataMap map[string][]byte
	UTXOMap        map[uint64]*UTXO
	TimeSlotMap    map[uint32]map[string]bool
}

// NewContextCache is used for generating genesis state
func newContextCache(ctx *Context) *contextCache {
	return &contextCache{
		ctx:            ctx,
		AccountMap:     map[common.Address]Account{},
		AccountNameMap: map[string]common.Address{},
		AccountDataMap: map[string][]byte{},
		ProcessDataMap: map[string][]byte{},
		UTXOMap:        map[uint64]*UTXO{},
		TimeSlotMap:    map[uint32]map[string]bool{},
	}
}

// ChainID returns the id of the chain
func (cc *contextCache) ChainID() uint8 {
	return cc.ctx.ChainID()
}

// Name returns the name of the chain
func (cc *contextCache) Name() string {
	return cc.ctx.Name()
}

// Version returns the version of the chain
func (cc *contextCache) Version() uint16 {
	return cc.ctx.Version()
}

// TargetHeight returns contextCached target height when context generation
func (cc *contextCache) TargetHeight() uint32 {
	return cc.ctx.TargetHeight()
}

// LastStatus returns the recored target height, prev hash
func (cc *contextCache) LastStatus() (uint32, hash.Hash256) {
	return cc.ctx.LastStatus()
}

// LastHash returns the recorded prev hash when context generation
func (cc *contextCache) LastHash() hash.Hash256 {
	return cc.ctx.LastHash()
}

// LastTimestamp returns the last timestamp of the chain
func (cc *contextCache) LastTimestamp() uint64 {
	return cc.ctx.LastTimestamp()
}

// Account returns the account instance of the address
func (cc *contextCache) Account(addr common.Address) (Account, error) {
	if acc, has := cc.AccountMap[addr]; has {
		return acc, nil
	} else {
		if acc, err := cc.ctx.loader.Account(addr); err != nil {
			return nil, err
		} else {
			cc.AccountMap[addr] = acc
			return acc, nil
		}
	}
}

// AddressByName returns the account address of the name
func (cc *contextCache) AddressByName(Name string) (common.Address, error) {
	if addr, has := cc.AccountNameMap[Name]; has {
		return addr, nil
	} else {
		if addr, err := cc.ctx.loader.AddressByName(Name); err != nil {
			return common.Address{}, err
		} else {
			cc.AccountNameMap[Name] = addr
			return addr, nil
		}
	}
}

// HasAccount checks that the account of the address is exist or not
func (cc *contextCache) HasAccount(addr common.Address) (bool, error) {
	if _, has := cc.AccountMap[addr]; has {
		return true, nil
	} else {
		return cc.ctx.loader.HasAccount(addr)
	}
}

// HasAccountName checks that the account of the name is exist or not
func (cc *contextCache) HasAccountName(Name string) (bool, error) {
	if _, has := cc.AccountNameMap[Name]; has {
		return true, nil
	} else {
		return cc.ctx.loader.HasAccountName(Name)
	}
}

// AccountData returns the account data
func (cc *contextCache) AccountData(addr common.Address, pid uint8, name []byte) []byte {
	key := string(addr[:]) + string(pid) + string(name)
	if value, has := cc.AccountDataMap[key]; has {
		return value
	} else {
		value := cc.ctx.loader.AccountData(addr, pid, name)
		cc.AccountDataMap[key] = value
		return value
	}
}

// HasUTXO checks that the utxo of the id is exist or not
func (cc *contextCache) HasUTXO(id uint64) (bool, error) {
	if _, has := cc.UTXOMap[id]; has {
		return true, nil
	} else {
		return false, nil
	}
}

// UTXO returns the UTXO
func (cc *contextCache) UTXO(id uint64) (*UTXO, error) {
	if utxo, has := cc.UTXOMap[id]; has {
		return utxo, nil
	} else {
		if utxo, err := cc.ctx.loader.UTXO(id); err != nil {
			return nil, err
		} else {
			cc.UTXOMap[id] = utxo
			return utxo, nil
		}
	}
}

// ProcessData returns the process data
func (cc *contextCache) ProcessData(pid uint8, name []byte) []byte {
	key := string(pid) + string(name)
	if value, has := cc.ProcessDataMap[key]; has {
		return value
	} else {
		value := cc.ctx.loader.ProcessData(pid, name)
		cc.ProcessDataMap[key] = value
		return value
	}
}

// HasTimeSlot returns timeslot is exist or not
func (cc *contextCache) HasTimeSlot(slot uint32, key string) bool {
	if mp, has := cc.TimeSlotMap[slot]; has {
		if _, has := mp[key]; has {
			return true
		}
	}
	if has := cc.ctx.loader.HasTimeSlot(slot, key); has {
		mp, has := cc.TimeSlotMap[slot]
		if !has {
			mp = map[string]bool{}
			cc.TimeSlotMap[slot] = mp
		}
		mp[key] = true
		return true
	}
	return false
}
