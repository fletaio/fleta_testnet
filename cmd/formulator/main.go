package main

import (
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fletaio/fleta_testnet/core/txpool"
	"github.com/fletaio/fleta_testnet/encoding"

	"github.com/gorilla/websocket"

	"github.com/fletaio/fleta_testnet/common/hash"

	uuid "github.com/satori/go.uuid"

	"github.com/fletaio/fleta_testnet/cmd/app"
	"github.com/fletaio/fleta_testnet/cmd/closer"
	"github.com/fletaio/fleta_testnet/cmd/config"
	"github.com/fletaio/fleta_testnet/common"
	"github.com/fletaio/fleta_testnet/common/key"
	"github.com/fletaio/fleta_testnet/core/backend"
	_ "github.com/fletaio/fleta_testnet/core/backend/badger_driver"
	_ "github.com/fletaio/fleta_testnet/core/backend/buntdb_driver"
	"github.com/fletaio/fleta_testnet/core/chain"
	"github.com/fletaio/fleta_testnet/core/pile"
	"github.com/fletaio/fleta_testnet/core/types"
	"github.com/fletaio/fleta_testnet/pof"
	"github.com/fletaio/fleta_testnet/process/admin"
	"github.com/fletaio/fleta_testnet/process/query"
	"github.com/fletaio/fleta_testnet/process/study"
	"github.com/fletaio/fleta_testnet/process/subject"
	"github.com/fletaio/fleta_testnet/process/user"
	"github.com/fletaio/fleta_testnet/process/visit"
	"github.com/fletaio/fleta_testnet/service/apiserver"
)

// Config is a configuration for the cmd
type Config struct {
	SeedNodeMap    map[string]string
	ObserverKeyMap map[string]string
	GenKeyHex      string
	NodeKeyHex     string
	Formulator     string
	Port           int
	APIPort        int
	StoreRoot      string
	InsertMode     bool
	InsertTxCount  int
}

func main() {
	var cfg Config
	if err := config.LoadFile("./config.toml", &cfg); err != nil {
		panic(err)
	}
	cfg.NodeKeyHex = cfg.GenKeyHex //TEMP
	if len(cfg.StoreRoot) == 0 {
		cfg.StoreRoot = "./fdata"
	}

	var frkey key.Key
	if bs, err := hex.DecodeString(cfg.GenKeyHex); err != nil {
		panic(err)
	} else if Key, err := key.NewMemoryKeyFromBytes(bs); err != nil {
		panic(err)
	} else {
		frkey = Key
	}

	var ndkey key.Key
	if len(cfg.NodeKeyHex) > 0 {
		if bs, err := hex.DecodeString(cfg.NodeKeyHex); err != nil {
			panic(err)
		} else if Key, err := key.NewMemoryKeyFromBytes(bs); err != nil {
			panic(err)
		} else {
			ndkey = Key
		}
	} else {
		if bs, err := ioutil.ReadFile("./ndkey.key"); err != nil {
			k, err := key.NewMemoryKey()
			if err != nil {
				panic(err)
			}

			fs, err := os.Create("./ndkey.key")
			if err != nil {
				panic(err)
			}
			fs.Write(k.Bytes())
			fs.Close()
			ndkey = k
		} else {
			if Key, err := key.NewMemoryKeyFromBytes(bs); err != nil {
				panic(err)
			} else {
				ndkey = Key
			}
		}
	}

	NetAddressMap := map[common.PublicHash]string{}
	ObserverKeys := []common.PublicHash{}
	for k, netAddr := range cfg.ObserverKeyMap {
		pubhash, err := common.ParsePublicHash(k)
		if err != nil {
			panic(err)
		}
		NetAddressMap[pubhash] = "ws://" + netAddr
		ObserverKeys = append(ObserverKeys, pubhash)
	}
	SeedNodeMap := map[common.PublicHash]string{}
	for k, netAddr := range cfg.SeedNodeMap {
		pubhash, err := common.ParsePublicHash(k)
		if err != nil {
			panic(err)
		}
		SeedNodeMap[pubhash] = netAddr
	}
	SeedNodeMap[common.MustParsePublicHash("4YjmYcLVvBSmtjh4Z7frRZhWgdEAYTSABCoqqzhKEJa")] = "45.77.147.144:41000"
	SeedNodeMap[common.MustParsePublicHash("27n37VV3ebGWSNH5r9wX3ZhUwzxC2heY34UvXjizLDK")] = "108.61.82.48:41000"
	SeedNodeMap[common.MustParsePublicHash("4GzTnuP7Hky1Dye1AJMLzEXTX2a5kEka5h9AJVvZyTD")] = "107.191.43.224:41000"
	SeedNodeMap[common.MustParsePublicHash("4ew8HQEwwSqeepMDCnwN9PiYg1uvoeZXyudqdQZBCb3")] = "140.82.7.91:41000"
	SeedNodeMap[common.MustParsePublicHash("VbMwA5AwSfn93ks8HMv7vvSx4THuzfeefTWVoANEha")] = "149.28.57.20:41000"
	SeedNodeMap[common.MustParsePublicHash("8eDJ3h8DLW8RSovYUjxmcDi1QNvo7UW64MQxGZ9dnS")] = "45.76.2.218:41000"
	SeedNodeMap[common.MustParsePublicHash("3ZdKaqaCbGSQ5xmAphzVTeEF1eGzX6iU4LLGD2ox2g9")] = "45.63.10.124:41000"
	SeedNodeMap[common.MustParsePublicHash("3UHQyJwSSHHCw29fB5xiGk9W7GNf1DjGC284WhW6jpD")] = "149.28.229.121:41000"
	SeedNodeMap[common.MustParsePublicHash("v3GwqbQehcqNVYbRzDk3TDJ7yJ19DgwoamZnMJZuVg")] = "66.55.159.135:41000"
	SeedNodeMap[common.MustParsePublicHash("3HhrC3gPR951SjnxjnHpfhRSWH1iR3SbCSwtCHvTLuC")] = "208.167.233.43:41000"
	SeedNodeMap[common.MustParsePublicHash("4Ei1HSF3KtDfGrdzHCWfRf4NSTZ2oYCT1CNGFkjV1WB")] = "144.202.0.171:41000"
	SeedNodeMap[common.MustParsePublicHash("3u6v76WAknSq1j86Pfb6p31FsBAJztPdVmY1kkw4k66")] = "208.167.239.236:41000"
	SeedNodeMap[common.MustParsePublicHash("MP6nHXaNjZRXFfSffbRuMDhjsS8YFxEsrtrDAZ9bNW")] = "45.76.6.45:41000"
	SeedNodeMap[common.MustParsePublicHash("4FQ3TVTWQi7TPDerc8nZUBtHyPaNRccA44ushVRWCKW")] = "45.76.0.241:41000"
	SeedNodeMap[common.MustParsePublicHash("3Ue7mXou8FJouGUyn7MtmahGNgevHt7KssNB2E9wRgL")] = "45.77.100.83:41000"
	SeedNodeMap[common.MustParsePublicHash("MZtuTqpsdGLm9QXKaM68sTDwUCyitL7q4L75Vrpwbo")] = "207.148.18.155:41000"
	SeedNodeMap[common.MustParsePublicHash("2fJTp1KMwBqJRqpwGgH5kUCtfBjUBGYgd8oXEA8V9AY")] = "207.246.127.38:41000"
	SeedNodeMap[common.MustParsePublicHash("3yTFnJJqx3wCiK2Edk9f9JwdvdkC4DP4T1y8xYztMkf")] = "45.63.13.183:41000"

	cm := closer.NewManager()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		cm.CloseAll()
	}()
	defer cm.CloseAll()

	MaxBlocksPerFormulator := uint32(10)
	ChainID := uint8(0x01)
	Name := "FLETA Testnet"
	Version := uint16(0x0001)

	var back backend.StoreBackend
	var cdb *pile.DB
	if true {
		contextDB, err := backend.Create("buntdb", cfg.StoreRoot+"/context")
		if err != nil {
			panic(err)
		}
		chainDB, err := pile.Open(cfg.StoreRoot + "/chain")
		if err != nil {
			panic(err)
		}
		chainDB.SetSyncMode(true)
		back = contextDB
		cdb = chainDB
	}
	st, err := chain.NewStore(back, cdb, ChainID, Name, Version)
	if err != nil {
		panic(err)
	}
	cm.Add("store", st)

	if st.Height() > 0 {
		if _, err := cdb.GetData(st.Height(), 0); err != nil {
			panic(err)
		}
	}

	cs := pof.NewConsensus(MaxBlocksPerFormulator, ObserverKeys)
	app := app.NewECRFApp()
	cn := chain.NewChain(cs, app, st)
	cn.MustAddProcess(admin.NewAdmin(1))
	vp := study.NewStudy(2)
	cn.MustAddProcess(vp)
	cn.MustAddProcess(user.NewUser(3))
	cn.MustAddProcess(subject.NewSubject(4))
	cn.MustAddProcess(visit.NewVisit(5))
	cn.MustAddProcess(query.NewQuery(6))
	as := apiserver.NewAPIServer()
	cn.MustAddService(as)
	if err := cn.Init(); err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("chain", cn)

	if err := st.IterBlockAfterContext(func(b *types.Block) error {
		if cm.IsClosed() {
			return chain.ErrStoreClosed
		}
		if err := cn.ConnectBlock(b, nil); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == chain.ErrStoreClosed {
			return
		}
		panic(err)
	}

	if cfg.InsertMode {
		switch cfg.GenKeyHex {
		case "f9d8e80d688c8b79a0470eaf418d0b6d0adac0648af9481f6d58b69ecebeb82c":
			Addrs = Addrs[:10000]
		case "7b5c05c6a87f650900dafd05fcbdb184c8d5b5f81cb31fff49f9212b44848340":
			Addrs = Addrs[10000:20000]
		case "e85029d11cdfc8595331bec963977a410fdeca1c36dbd89e2ec7c2985a15ac78":
			Addrs = Addrs[20000:30000]
		case "e2ec6a295d63d9bf312367773efe0b164d55554a61880741b072c87cd66298ae":
			Addrs = Addrs[30000:40000]
		case "bb3f0d6b24dce5d5b4d539a814ba23ff429c1dfacde9a83e72cb4049a38ca113":
			Addrs = Addrs[40000:50000]
		case "f322fa429a627772b76249c96d9e4525eb7c7ab66fc8ff16e7f141c1ddd61b6b":
			Addrs = Addrs[50000:60000]
		case "a3bcc459e90b575d75a64aa7f8a0e45b610057d2132112f9d5876b358d95609b":
			Addrs = Addrs[60000:70000]
		case "0f72009df8bbbf78aed3adbadb31d89410e7a4d4d0b104421b72b5d2e5343577":
			Addrs = Addrs[70000:80000]
		case "a0c7e7ab4bb90e55c4e8d6fde2f7e9c18d9e1a9a8ba8cdf8e48caa2e6003f252":
			Addrs = Addrs[80000:90000]
		case "16e0381a755ea31b5567db0557d173fca57396f54ba734ade9f7a8e420e446b3":
			Addrs = Addrs[90000:100000]
		case "e5db5c29bdfb784f7235f86bfc9ac28e5e6e0507aaacc4b0e1d7db73ee20a1f5":
			Addrs = Addrs[10000:110000]
		case "ea060ebefabb620500080461d438748e967965c210991b8e1a7b7435f96585e1":
			Addrs = Addrs[110000:120000]
		default:
			Addrs = []common.Address{}
		}
		if cfg.InsertTxCount > len(Addrs) {
			cfg.InsertTxCount = len(Addrs)
		}
		Addrs = Addrs[:cfg.InsertTxCount]
	} else {
		Addrs = []common.Address{}
	}

	PoolItems := []*txpool.PoolItem{}
	if len(Addrs) > 0 {
		key, _ := key.NewMemoryKeyFromString("fd1167aad31c104c9fceb5b8a4ffd3e20a272af82176352d3b6ac236d02bafd4")
		signer := common.NewPublicHash(key.PublicKey())
		fc := encoding.Factory("transaction")
		for _, Addr := range Addrs {
			tx := &study.UpdateMetaUnsafe{
				Timestamp_: uint64(time.Now().UnixNano()),
				From_:      Addr,
				Forms: []*study.Form{
					&study.Form{
						ID:       "form-id",
						Name:     "form-name",
						Type:     "form-type",
						Priority: 1,
						Extra:    types.NewStringStringMap(),
						Groups:   []*study.Group{},
					},
				},
			}
			t, err := fc.TypeOf(tx)
			if err != nil {
				panic(err)
			}
			TxHash := chain.HashTransactionByType(ChainID, t, tx)
			sig, err := key.Sign(TxHash)
			if err != nil {
				panic(err)
			}
			item := &txpool.PoolItem{
				TxType:      t,
				TxHash:      TxHash,
				Transaction: tx,
				Signatures:  []common.Signature{sig},
				Signers:     []common.PublicHash{signer},
			}
			PoolItems = append(PoolItems, item)
		}
	}

	fr := pof.NewFormulatorNode(&pof.FormulatorConfig{
		Formulator:              common.MustParseAddress(cfg.Formulator),
		MaxTransactionsPerBlock: 7000,
		PoolItems:               PoolItems,
	}, frkey, ndkey, NetAddressMap, SeedNodeMap, cs, cfg.StoreRoot+"/peer")
	if err := fr.Init(); err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("formulator", fr)

	if true {
		s, err := as.JRPC("chain")
		if err != nil {
			panic(err)
		}
		s.Set("height", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			return cn.Provider().Height(), nil
		})
		s.Set("transaction", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			TXID, err := arg.String(0)
			if err != nil {
				return nil, apiserver.ErrInvalidArgument
			}
			Height, Index, err := types.ParseTransactionID(TXID)
			if err != nil {
				return nil, err
			}
			if Height > st.Height() {
				return nil, apiserver.ErrInvalidArgument
			}
			b, err := st.Block(Height)
			if err != nil {
				return nil, err
			}
			if int(Index) >= len(b.Transactions) {
				return nil, apiserver.ErrInvalidArgument
			}
			return b.Transactions[Index], nil
		})
		s.Set("summary", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			Height, err := arg.Uint32(0)
			if err != nil {
				return nil, apiserver.ErrInvalidArgument
			}
			if Height > st.Height() {
				return nil, apiserver.ErrInvalidArgument
			}

			Blocks := []*types.Block{}
			Txs := []types.Transaction{}
			var MaxTPS float64
			for h := uint32(1); h <= Height; h++ {
				b, err := st.Block(h)
				if err != nil {
					return nil, err
				}

				if len(Blocks) == 0 {
					MaxTPS = float64(len(b.Transactions) * 2)
				} else {
					TPS := float64(len(b.Transactions)) * float64(time.Second) / float64(b.Header.Timestamp-Blocks[len(Blocks)-1].Header.Timestamp)
					if MaxTPS < TPS {
						MaxTPS = TPS
					}
				}
				Blocks = append(Blocks, b)
				Txs = append(Txs, b.Transactions...)
			}

			TimeElapsed := Blocks[len(Blocks)-1].Header.Timestamp - Blocks[0].Header.Timestamp
			return &struct {
				TargetHeight uint32
				ChainHeight  uint32
				TxCount      int
				TimeElapsed  float64
				MaxTPS       float64
				MeanTPS      float64
			}{
				TargetHeight: Height,
				ChainHeight:  st.Height(),
				TxCount:      len(Txs),
				TimeElapsed:  float64(TimeElapsed) / float64(time.Second),
				MaxTPS:       MaxTPS,
				MeanTPS:      float64(len(Txs)) * float64(time.Second) / float64(TimeElapsed),
			}, nil
		})
		s.Set("summary0tx", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			Height, err := arg.Uint32(0)
			if err != nil {
				return nil, apiserver.ErrInvalidArgument
			}
			if Height > st.Height() {
				return nil, apiserver.ErrInvalidArgument
			}

			Blocks := []*types.Block{}
			Txs := []types.Transaction{}
			var MaxTPS float64
			var FirstTxTimestamp uint64
			for h := uint32(1); h <= Height; h++ {
				b, err := st.Block(h)
				if err != nil {
					return nil, err
				}

				if len(Blocks) == 0 {
					MaxTPS = float64(len(b.Transactions) * 2)
				} else {
					TPS := float64(len(b.Transactions)) * float64(time.Second) / float64(b.Header.Timestamp-Blocks[len(Blocks)-1].Header.Timestamp)
					if MaxTPS < TPS {
						MaxTPS = TPS
					}
				}
				if FirstTxTimestamp == 0 {
					if len(b.Transactions) > 0 {
						FirstTxTimestamp = b.Transactions[0].Timestamp()
					}
				}

				Blocks = append(Blocks, b)
				Txs = append(Txs, b.Transactions...)
			}

			TimeElapsed := Blocks[len(Blocks)-1].Header.Timestamp - FirstTxTimestamp
			return &struct {
				TargetHeight uint32
				ChainHeight  uint32
				TxCount      int
				TimeElapsed  float64
				MaxTPS       float64
				MeanTPS      float64
			}{
				TargetHeight: Height,
				ChainHeight:  st.Height(),
				TxCount:      len(Txs),
				TimeElapsed:  float64(TimeElapsed) / float64(time.Second),
				MaxTPS:       MaxTPS,
				MeanTPS:      float64(len(Txs)) * float64(time.Second) / float64(TimeElapsed),
			}, nil
		})
		s.Set("genCount", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			arg0, err := arg.String(0)
			if err != nil {
				return nil, err
			}
			addr, err := common.ParseAddress(arg0)
			if err != nil {
				return nil, err
			}

			var Count int
			Height := st.Height()
			for h := uint32(1); h <= Height; h++ {
				bh, err := st.Header(h)
				if err != nil {
					return nil, err
				}
				if bh.Generator == addr {
					Count++
				}
			}
			return Count, nil
		})
		s.Set("sendText", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			text, err := arg.String(0)
			if err != nil {
				return nil, err
			}

			id := uuid.NewV1().String()
			key, _ := key.NewMemoryKeyFromString("fd1167aad31c104c9fceb5b8a4ffd3e20a272af82176352d3b6ac236d02bafd4")
			tx := &study.TextData{
				Timestamp_: uint64(time.Now().UnixNano()),
				From_:      common.NewAddress(0, uint16(21000), 0),
				Seq_:       st.Seq(common.NewAddress(0, uint16(21000), 0)) + 1,
				ID:         id,
				TextData:   text,
			}
			sig, err := key.Sign(chain.HashTransaction(ChainID, tx))
			if err != nil {
				return nil, err
			}
			if err := fr.AddTx(tx, []common.Signature{sig}); err != nil {
				return nil, err
			}
			return id, nil
		})
		s.Set("getText", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			id, err := arg.String(0)
			if err != nil {
				return nil, err
			}
			return vp.TextData(cn.NewContext(), id), nil
		})
		s.Set("checkIntegrity", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 1 {
				return nil, apiserver.ErrInvalidArgument
			}
			Height, err := arg.Uint32(0)
			if err != nil {
				return nil, apiserver.ErrInvalidArgument
			}
			if Height > st.Height() {
				return nil, apiserver.ErrInvalidArgument
			}

			Blocks := []*types.Block{}
			Hashes := []string{}
			HashesPrev := []string{}
			GenHash, err := st.Hash(0)
			if err != nil {
				return nil, err
			}
			Hashes = append(Hashes, GenHash.String())
			LevelRoots := []string{}
			LevelRootsCalced := []string{}
			for h := uint32(1); h <= Height; h++ {
				b, err := st.Block(h)
				if err != nil {
					return nil, err
				}
				Blocks = append(Blocks, b)

				TxHashes := make([]hash.Hash256, len(b.Transactions)+1)
				TxHashes[0] = b.Header.PrevHash

				for i, tx := range b.Transactions {
					t := b.TransactionTypes[i]
					TxHash := chain.HashTransactionByType(st.ChainID(), t, tx)
					TxHashes[i+1] = TxHash
				}
				LevelRoot, err := chain.BuildLevelRoot(TxHashes)
				if err != nil {
					return nil, err
				}
				LevelRoots = append(LevelRoots, b.Header.LevelRootHash.String())
				LevelRootsCalced = append(LevelRootsCalced, LevelRoot.String())
				Hashes = append(Hashes, encoding.Hash(b.Header).String())
				HashesPrev = append(HashesPrev, b.Header.PrevHash.String())
			}

			return &struct {
				TargetHeight     uint32
				ChainHeight      uint32
				Hashes           []string
				HashesPrev       []string
				LevelRoots       []string
				LevelRootsCalced []string
			}{
				TargetHeight:     Height,
				ChainHeight:      st.Height(),
				Hashes:           Hashes,
				HashesPrev:       HashesPrev,
				LevelRoots:       LevelRoots,
				LevelRootsCalced: LevelRootsCalced,
			}, nil
		})
		s.Set("readTest", func(ID interface{}, arg *apiserver.Argument) (interface{}, error) {
			if arg.Len() != 2 {
				return nil, apiserver.ErrInvalidArgument
			}
			userCount, err := arg.Int(0)
			if err != nil {
				return nil, err
			}
			requestPerUser, err := arg.Int(1)
			if err != nil {
				return nil, err
			}

			var SuccessCount uint64
			var ErrorCount uint64
			start := time.Now()
			var wg sync.WaitGroup
			for i := 0; i < userCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					c, _, _ := websocket.DefaultDialer.Dial("ws://localhost:48000/api/endpoints/websocket", nil)
					defer c.Close()

					for q := 0; q < requestPerUser; q++ {
						if _, err := GetBalance(c, "5CyLcFhpyN"); err != nil {
							log.Println(err)
							atomic.AddUint64(&ErrorCount, 1)
						} else {
							atomic.AddUint64(&SuccessCount, 1)
						}
					}
				}()
			}
			wg.Wait()

			TimeElapsed := time.Now().Sub(start)
			return &struct {
				SuccessCount uint64
				ErrorCount   uint64
				TimeElapsed  float64
				TPS          float64
			}{
				SuccessCount: SuccessCount,
				ErrorCount:   ErrorCount,
				TimeElapsed:  float64(TimeElapsed) / float64(time.Second),
				TPS:          float64(SuccessCount+ErrorCount) * float64(time.Second) / float64(TimeElapsed),
			}, nil
		})
	}

	go fr.Run(":" + strconv.Itoa(cfg.Port))
	go as.Run(":" + strconv.Itoa(cfg.APIPort))

	cm.Wait()
}

func GetBalance(c *websocket.Conn, addr string) (string, error) {
	res, err := DoRequest(c, "vault.balance", []interface{}{addr})
	if err != nil {
		return "", err
	} else {
		bs, err := json.MarshalIndent(res, "", "\t")
		if err != nil {
			return "", err
		} else {
			return string(bs), nil
		}
	}
}
