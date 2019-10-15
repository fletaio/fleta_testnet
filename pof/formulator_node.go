package pof

import (
	"bytes"
	"log"
	"sync"
	"time"

	"github.com/fletaio/fleta/common/amount"
	"github.com/fletaio/fleta/process/vault"

	"github.com/bluele/gcache"

	"github.com/fletaio/fleta/common"
	"github.com/fletaio/fleta/common/hash"
	"github.com/fletaio/fleta/common/key"
	"github.com/fletaio/fleta/common/queue"
	"github.com/fletaio/fleta/common/rlog"
	"github.com/fletaio/fleta/core/chain"
	"github.com/fletaio/fleta/core/txpool"
	"github.com/fletaio/fleta/core/types"
	"github.com/fletaio/fleta/encoding"
	"github.com/fletaio/fleta/service/p2p"
)

type genItem struct {
	BlockGen *BlockGenMessage
	ObSign   *BlockObSignMessage
	Context  *types.Context
}

// FormulatorConfig defines configuration of the formulator
type FormulatorConfig struct {
	Formulator              common.Address
	MaxTransactionsPerBlock int
	Addrs                   []common.Address
}

// FormulatorNode procudes a block by the consensus
type FormulatorNode struct {
	sync.Mutex
	Config         *FormulatorConfig
	cs             *Consensus
	ms             *FormulatorNodeMesh
	nm             *p2p.NodeMesh
	key            key.Key
	ndkey          key.Key
	myPublicHash   common.PublicHash
	frPublicHash   common.PublicHash
	statusLock     sync.Mutex
	genLock        sync.Mutex
	lastGenItemMap map[uint32]*genItem
	lastReqMessage *BlockReqMessage
	lastGenHeight  uint32
	lastGenTime    int64
	statusMap      map[string]*p2p.Status
	obStatusMap    map[string]*p2p.Status
	requestTimer   *p2p.RequestTimer
	requestLock    sync.RWMutex
	blockQ         *queue.SortedQueue
	txpool         *txpool.TransactionPool
	txQ            *queue.ExpireQueue
	txWaitQ        *queue.LinkedQueue
	recvQueues     []*queue.Queue
	sendQueues     []*queue.Queue
	singleCache    gcache.Cache
	batchCache     gcache.Cache
	isRunning      bool
	closeLock      sync.RWMutex
	isClose        bool

	//TEMP
	Txs      []types.Transaction
	Sigs     []common.Signature
	TxHashes []hash.Hash256
}

// NewFormulatorNode returns a FormulatorNode
func NewFormulatorNode(Config *FormulatorConfig, key key.Key, ndkey key.Key, NetAddressMap map[common.PublicHash]string, SeedNodeMap map[common.PublicHash]string, cs *Consensus, peerStorePath string) *FormulatorNode {
	if Config.MaxTransactionsPerBlock == 0 {
		Config.MaxTransactionsPerBlock = 10000
	}
	fr := &FormulatorNode{
		Config:         Config,
		cs:             cs,
		key:            key,
		ndkey:          ndkey,
		myPublicHash:   common.NewPublicHash(ndkey.PublicKey()),
		frPublicHash:   common.NewPublicHash(key.PublicKey()),
		lastGenItemMap: map[uint32]*genItem{},
		statusMap:      map[string]*p2p.Status{},
		obStatusMap:    map[string]*p2p.Status{},
		requestTimer:   p2p.NewRequestTimer(nil),
		blockQ:         queue.NewSortedQueue(),
		txpool:         txpool.NewTransactionPool(),
		txQ:            queue.NewExpireQueue(),
		txWaitQ:        queue.NewLinkedQueue(),
		recvQueues: []*queue.Queue{
			queue.NewQueue(), //block
			queue.NewQueue(), //tx
			queue.NewQueue(), //peer
		},
		sendQueues: []*queue.Queue{
			queue.NewQueue(), //block
			queue.NewQueue(), //tx
			queue.NewQueue(), //peer
		},
		singleCache: gcache.New(500).LRU().Build(),
		batchCache:  gcache.New(500).LRU().Build(),
	}
	fr.ms = NewFormulatorNodeMesh(key, NetAddressMap, fr)
	fr.nm = p2p.NewNodeMesh(fr.cs.cn.Provider().ChainID(), ndkey, SeedNodeMap, fr, peerStorePath)
	fr.txQ.AddGroup(60 * time.Second)
	fr.txQ.AddGroup(600 * time.Second)
	fr.txQ.AddGroup(3600 * time.Second)
	fr.txQ.AddHandler(fr)
	rlog.SetRLogAddress("fr:" + Config.Formulator.String())

	fr.temp() // TEMP
	return fr
}

func (fr *FormulatorNode) temp() {
	fc := encoding.Factory("transaction")
	t, err := fc.TypeOf(&vault.Transfer{})
	if err != nil {
		panic(err)
	}
	key, _ := key.NewMemoryKeyFromString("fd1167aad31c104c9fceb5b8a4ffd3e20a272af82176352d3b6ac236d02bafd4")
	Txs := []types.Transaction{}
	Sigs := []common.Signature{}
	TxHashes := []hash.Hash256{}
	for _, Addr := range fr.Config.Addrs {
		tx := &vault.Transfer{
			Timestamp_: uint64(time.Now().UnixNano()),
			From_:      Addr,
			To:         Addr,
			Amount:     amount.NewCoinAmount(1, 0),
		}
		sig, err := key.Sign(chain.HashTransaction(fr.cs.cn.Provider().ChainID(), tx))
		if err != nil {
			panic(err)
		}
		TxHash := chain.HashTransactionByType(fr.cs.cn.Provider().ChainID(), t, tx)

		Txs = append(Txs, tx)
		Sigs = append(Sigs, sig)
		TxHashes = append(TxHashes, TxHash)
	}
	fr.Txs = Txs
	fr.Sigs = Sigs
	fr.TxHashes = TxHashes
}

// Close terminates the formulator
func (fr *FormulatorNode) Close() {
	fr.closeLock.Lock()
	defer fr.closeLock.Unlock()

	fr.Lock()
	defer fr.Unlock()

	fr.isClose = true
	fr.cs.cn.Close()
}

// Init initializes formulator
func (fr *FormulatorNode) Init() error {
	fc := encoding.Factory("message")
	fc.Register(types.DefineHashedType("pof.BlockReqMessage"), &BlockReqMessage{})
	fc.Register(types.DefineHashedType("pof.BlockGenMessage"), &BlockGenMessage{})
	fc.Register(types.DefineHashedType("pof.BlockObSignMessage"), &BlockObSignMessage{})
	fc.Register(types.DefineHashedType("p2p.StatusMessage"), &p2p.StatusMessage{})
	fc.Register(types.DefineHashedType("p2p.BlockMessage"), &p2p.BlockMessage{})
	fc.Register(types.DefineHashedType("p2p.RequestMessage"), &p2p.RequestMessage{})
	fc.Register(types.DefineHashedType("p2p.TransactionMessage"), &p2p.TransactionMessage{})
	fc.Register(types.DefineHashedType("p2p.PeerListMessage"), &p2p.PeerListMessage{})
	fc.Register(types.DefineHashedType("p2p.RequestPeerListMessage"), &p2p.RequestPeerListMessage{})
	return nil
}

// Run runs the formulator
func (fr *FormulatorNode) Run(BindAddress string) {
	fr.Lock()
	if fr.isRunning {
		fr.Unlock()
		return
	}
	fr.isRunning = true
	fr.Unlock()

	go fr.ms.Run()
	go fr.nm.Run(BindAddress)
	go fr.requestTimer.Run()

	/*
		WorkerCount := runtime.NumCPU() - 1
		if WorkerCount < 1 {
			WorkerCount = 1
		}
		for i := 0; i < WorkerCount; i++ {
			go func() {
				for !fr.isClose {
					Count := 0
					for !fr.isClose {
						v := fr.txWaitQ.Pop()
						if v == nil {
							break
						}
						item := v.(*p2p.TxMsgItem)
						if err := fr.addTx(item.TxHash, item.Message.TxType, item.Message.Tx, item.Message.Sigs); err != nil {
							if err != p2p.ErrInvalidUTXO && err != txpool.ErrExistTransaction && err != txpool.ErrTooFarSeq && err != txpool.ErrPastSeq {
								rlog.Println("TransactionError", chain.HashTransactionByType(fr.cs.cn.Provider().ChainID(), item.Message.TxType, item.Message.Tx).String(), err.Error())
								if len(item.PeerID) > 0 {
									fr.nm.RemovePeer(item.PeerID)
								}
							}
						}
						rlog.Println("TransactionAppended", chain.HashTransactionByType(fr.cs.cn.Provider().ChainID(), item.Message.TxType, item.Message.Tx).String())

						if len(item.PeerID) > 0 {
							var SenderPublicHash common.PublicHash
							copy(SenderPublicHash[:], []byte(item.PeerID))
							fr.exceptLimitCastMessage(1, SenderPublicHash, item.Message)
						} else {
							fr.limitCastMessage(1, item.Message)
						}

						Count++
						if Count > 500 {
							break
						}
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
	*/

	go func() {
		for !fr.isClose {
			hasMessage := false
			for !fr.isClose {
				for _, q := range fr.recvQueues {
					v := q.Pop()
					if v == nil {
						continue
					}
					hasMessage = true
					item := v.(*p2p.RecvMessageItem)
					m, err := p2p.PacketToMessage(item.Packet)
					if err != nil {
						log.Println("PacketToMessage", err)
						fr.nm.RemovePeer(item.PeerID)
						break
					}
					if err := fr.handlePeerMessage(item.PeerID, m); err != nil {
						log.Println("handlePeerMessage", err)
						fr.nm.RemovePeer(item.PeerID)
						break
					}
					break
				}
				if !hasMessage {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		for !fr.isClose {
			hasMessage := false
			for !fr.isClose {
				for _, q := range fr.sendQueues {
					v := q.Pop()
					if v == nil {
						continue
					}
					hasMessage = true
					item := v.(*p2p.SendMessageItem)
					var EmptyHash common.PublicHash
					if bytes.Equal(item.Target[:], EmptyHash[:]) {
						if item.Limit > 0 {
							fr.nm.ExceptCastLimit("", item.Packet, item.Limit)
						} else {
							fr.nm.BroadcastPacket(item.Packet)
						}
					} else {
						if item.Limit > 0 {
							fr.nm.ExceptCastLimit(string(item.Target[:]), item.Packet, item.Limit)
						} else {
							fr.nm.SendTo(item.Target, item.Packet)
						}
					}
					break
				}
				if !hasMessage {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		for !fr.isClose {
			hasMessage := false
			for !fr.isClose {
				for _, q := range fr.sendQueues {
					v := q.Pop()
					if v == nil {
						continue
					}
					hasMessage = true
					item := v.(*p2p.SendMessageItem)
					var EmptyHash common.PublicHash
					if bytes.Equal(item.Target[:], EmptyHash[:]) {
						if item.Limit > 0 {
							fr.nm.ExceptCastLimit("", item.Packet, item.Limit)
						} else {
							fr.nm.BroadcastPacket(item.Packet)
						}
					} else {
						if item.Limit > 0 {
							fr.nm.ExceptCastLimit(string(item.Target[:]), item.Packet, item.Limit)
						} else {
							fr.nm.SendTo(item.Target, item.Packet)
						}
					}
					break
				}
				if !hasMessage {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	go func() {
		for !fr.isClose {
			fr.tryRequestBlocks()
			fr.tryRequestNext()
			time.Sleep(500 * time.Millisecond)
		}
	}()

	for !fr.isClose {
		fr.Lock()
		hasItem := false
		TargetHeight := uint64(fr.cs.cn.Provider().Height() + 1)
		Count := 0
		item := fr.blockQ.PopUntil(TargetHeight)
		for item != nil {
			b := item.(*types.Block)
			gi, has := fr.lastGenItemMap[b.Header.Height]
			isConnected := false
			if has {
				if gi.BlockGen != nil && gi.Context != nil {
					if gi.BlockGen.Block.Header.Generator == b.Header.Generator {
						if err := fr.cs.ct.ConnectBlockWithContext(b, gi.Context); err != nil {
							log.Println("blockQ.ConnectBlockWithContext", err)
						} else {
							isConnected = true
						}
					}
				}
			}
			if !isConnected {
				if err := fr.cs.cn.ConnectBlock(b); err != nil {
					break
				}
			}
			fr.cleanPool(b)
			rlog.Println("Formulator", fr.Config.Formulator.String(), "BlockConnected", b.Header.Generator.String(), b.Header.Height, len(b.Transactions))
			if fr.lastReqMessage != nil {
				if b.Header.Height <= fr.lastReqMessage.TargetHeight+fr.cs.maxBlocksPerFormulator {
					if b.Header.Generator != fr.Config.Formulator {
						fr.lastReqMessage = nil
					}
				}
			}
			delete(fr.lastGenItemMap, b.Header.Height)
			TargetHeight++
			Count++
			if Count > 10 {
				break
			}
			item = fr.blockQ.PopUntil(TargetHeight)
			hasItem = true
		}
		fr.Unlock()

		if hasItem {
			fr.broadcastStatus()
			fr.tryRequestBlocks()
		}

		if hasItem {
			time.Sleep(50 * time.Millisecond)
		} else {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// AddTx adds tx to txpool that only have valid signatures
func (fr *FormulatorNode) AddTx(tx types.Transaction, sigs []common.Signature) error {
	fc := encoding.Factory("transaction")
	t, err := fc.TypeOf(tx)
	if err != nil {
		return err
	}
	TxHash := chain.HashTransactionByType(fr.cs.cn.Provider().ChainID(), t, tx)
	fr.txWaitQ.Push(TxHash, &p2p.TxMsgItem{
		TxHash: TxHash,
		Message: &p2p.TransactionMessage{
			TxType: t,
			Tx:     tx,
			Sigs:   sigs,
		},
	})
	return nil
}

func (fr *FormulatorNode) addTx(TxHash hash.Hash256, t uint16, tx types.Transaction, sigs []common.Signature) error {
	if fr.txpool.Size() > 65535 {
		return txpool.ErrTransactionPoolOverflowed
	}

	cp := fr.cs.cn.Provider()
	if fr.txpool.IsExist(TxHash) {
		return txpool.ErrExistTransaction
	}
	if atx, is := tx.(chain.AccountTransaction); is {
		seq := cp.Seq(atx.From())
		if atx.Seq() <= seq {
			return txpool.ErrPastSeq
		} else if atx.Seq() > seq+100 {
			return txpool.ErrTooFarSeq
		}
	}
	signers := make([]common.PublicHash, 0, len(sigs))
	for _, sig := range sigs {
		pubkey, err := common.RecoverPubkey(TxHash, sig)
		if err != nil {
			return err
		}
		signers = append(signers, common.NewPublicHash(pubkey))
	}
	pid := uint8(t >> 8)
	p, err := fr.cs.cn.Process(pid)
	if err != nil {
		return err
	}
	ctx := fr.cs.cn.NewContext()
	ctw := types.NewContextWrapper(pid, ctx)
	if err := tx.Validate(p, ctw, signers); err != nil {
		return err
	}
	if err := fr.txpool.Push(fr.cs.cn.Provider().ChainID(), t, TxHash, tx, sigs, signers); err != nil {
		return err
	}
	fr.txQ.Push(string(TxHash[:]), &p2p.TransactionMessage{
		TxType: t,
		Tx:     tx,
		Sigs:   sigs,
	})
	return nil
}

// OnTimerExpired called when rquest expired
func (fr *FormulatorNode) OnTimerExpired(height uint32, value string) {
	go fr.tryRequestBlocks()
}

// OnItemExpired is called when the item is expired
func (fr *FormulatorNode) OnItemExpired(Interval time.Duration, Key string, Item interface{}, IsLast bool) {
	msg := Item.(*p2p.TransactionMessage)
	fr.limitCastMessage(1, msg)
	if IsLast {
		var TxHash hash.Hash256
		copy(TxHash[:], []byte(Key))
		fr.txpool.Remove(TxHash, msg.Tx)
	}
}

func (fr *FormulatorNode) addBlock(b *types.Block) error {
	cp := fr.cs.cn.Provider()
	if b.Header.Height <= cp.Height() {
		h, err := cp.Hash(b.Header.Height)
		if err != nil {
			return err
		}
		if h != encoding.Hash(b.Header) {
			//TODO : critical error signal
			return chain.ErrFoundForkedBlock
		}
	} else {
		if item := fr.blockQ.FindOrInsert(b, uint64(b.Header.Height)); item != nil {
			old := item.(*types.Block)
			if encoding.Hash(old.Header) != encoding.Hash(b.Header) {
				//TODO : critical error signal
				return chain.ErrFoundForkedBlock
			}
		}
	}
	return nil
}

func (fr *FormulatorNode) cleanPool(b *types.Block) {
	for i, tx := range b.Transactions {
		t := b.TransactionTypes[i]
		TxHash := chain.HashTransactionByType(fr.cs.cn.Provider().ChainID(), t, tx)
		fr.txpool.Remove(TxHash, tx)
		fr.txQ.Remove(string(TxHash[:]))
	}
}
