package main

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/fletaio/fleta_testnet/common/amount"

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
	"github.com/fletaio/fleta_testnet/process/formulator"
	"github.com/fletaio/fleta_testnet/process/gateway"
	"github.com/fletaio/fleta_testnet/process/payment"
	"github.com/fletaio/fleta_testnet/process/vault"
	"github.com/fletaio/fleta_testnet/service/p2p"
	"github.com/fletaio/testnet_explorer/explorerservice"
)

// Config is a configuration for the cmd
type Config struct {
	SeedNodeMap  map[string]string
	NodeKeyHex   string
	ObserverKeys []string
	Port         int
	APIPort      int
	WebPort      int
	StoreRoot    string
	CreateMode   bool
	CustomText   string
}

func main() {
	var cfg Config
	if err := config.LoadFile("./config.toml", &cfg); err != nil {
		panic(err)
	}
	if len(cfg.StoreRoot) == 0 {
		cfg.StoreRoot = "./ndata"
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

	ObserverKeys := []common.PublicHash{}
	for _, k := range cfg.ObserverKeys {
		pubhash, err := common.ParsePublicHash(k)
		if err != nil {
			panic(err)
		}
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
	Symbol := "FLETA"
	Usage := "Mainnet"
	Version := uint16(0x0001)

	back, err := backend.Create("buntdb", cfg.StoreRoot+"/context")
	if err != nil {
		panic(err)
	}
	cdb, err := pile.Open(cfg.StoreRoot + "/chain")
	if err != nil {
		panic(err)
	}
	cdb.SetSyncMode(true)
	st, err := chain.NewStore(back, cdb, ChainID, Symbol, Usage, Version)
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
	app := app.NewFletaApp()
	cn := chain.NewChain(cs, app, st)
	cn.MustAddProcess(admin.NewAdmin(1))
	cn.MustAddProcess(vault.NewVault(2))
	cn.MustAddProcess(formulator.NewFormulator(3))
	cn.MustAddProcess(gateway.NewGateway(4))
	cn.MustAddProcess(payment.NewPayment(5))
	ws := NewWatcher()
	if cfg.CreateMode {
		cn.MustAddService(ws)
	}
	e, err := explorerservice.NewBlockExplorer("_explorer", cs, cfg.WebPort)
	if err != nil {
		panic(err)
	}
	cn.MustAddService(e)
	if err := cn.Init(); err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("chain", cn)
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

	nd := p2p.NewNode(ndkey, SeedNodeMap, cn, cfg.StoreRoot+"/peer")
	if err := nd.Init(); err != nil {
		panic(err)
	}
	cm.RemoveAll()
	cm.Add("node", nd)

	if cfg.CreateMode {
		waitMap := map[common.Address]*chan struct{}{}
		go func() {
			switch cfg.NodeKeyHex {
			case "f07f3de26238cb57776556c67368665a53a969efeddf582028ae0c2344261feb":
				Addrs = Addrs[:12000]
			case "2e56f231189d41397a844232250276992691b9102aa53efd3a315ec2abf76094":
				Addrs = Addrs[12000:24000]
			case "74a4bb065b9553e18c5f6aab54bcb07db58f2950b09d3be024e20318512d97bb":
				Addrs = Addrs[24000:36000]
			case "f7b6a6291165b7d4cea6d16b911b6f2ba024aac6f160b230b9a04de876f3b045":
				Addrs = Addrs[36000:48000]
			case "4bc61ab268197c465d471d67618a3bf385651a93ed5011b8db08f5dfdca43c1d":
				Addrs = Addrs[48000:60000]
			case "bfe9f217f31f52a8e3e975c415d297ff201a4c4abfbcb921eb9013b0c21397f4":
				Addrs = Addrs[60000:72000]
			default:
				Addrs = []common.Address{}
			}

			for _, Addr := range Addrs {
				waitMap[Addr] = ws.addAddress(Addr)
			}

			for _, v := range Addrs {
				go func(Addr common.Address) {
					Seq := st.Seq(Addr)
					key, _ := key.NewMemoryKeyFromString("fd1167aad31c104c9fceb5b8a4ffd3e20a272af82176352d3b6ac236d02bafd4")
					log.Println(Addr.String(), "Start Transaction", Seq)

					for i := 0; i < 1; i++ {
						Seq++
						tx := &vault.Transfer{
							Timestamp_: uint64(time.Now().UnixNano()),
							Seq_:       Seq,
							From_:      Addr,
							To:         Addr,
							Amount:     amount.NewCoinAmount(0, 1),
						}
						sig, err := key.Sign(chain.HashTransaction(ChainID, tx))
						if err != nil {
							panic(err)
						}
						if err := nd.AddTx(tx, []common.Signature{sig}); err != nil {
							panic(err)
						}
						time.Sleep(100 * time.Millisecond)
					}

					pCh := waitMap[Addr]

					if pCh == nil {
						log.Println(Addr)
					}

					ticker := time.NewTicker(9000 * time.Millisecond)
					for {
						select {
						case <-ticker.C:
							Seq++
							tx := &vault.Transfer{
								Timestamp_: uint64(time.Now().UnixNano()),
								Seq_:       Seq,
								From_:      Addr,
								To:         Addr,
								Amount:     amount.NewCoinAmount(0, 1),
							}
							sig, err := key.Sign(chain.HashTransaction(ChainID, tx))
							if err != nil {
								panic(err)
							}
							if err := nd.AddTx(tx, []common.Signature{sig}); err != nil {
							}
						case <-*pCh:
							NextSeq := st.Seq(Addr) + 1
							tx := &vault.Transfer{
								Timestamp_: uint64(time.Now().UnixNano()),
								Seq_:       NextSeq,
								From_:      Addr,
								To:         Addr,
								Amount:     amount.NewCoinAmount(0, 1),
							}
							sig, err := key.Sign(chain.HashTransaction(ChainID, tx))
							if err != nil {
								panic(err)
							}
							if err := nd.AddTx(tx, []common.Signature{sig}); err != nil {
							}
						}
					}
				}(v)
			}
		}()

		go func() {
			for {
				b := <-ws.blockCh
				for _, t := range b.Transactions {
					if tx, is := t.(*vault.Transfer); is {
						pCh, has := waitMap[tx.From()]
						if has {
							(*pCh) <- struct{}{}
						}
					}
				}
			}
		}()
	}

	go nd.Run(":" + strconv.Itoa(cfg.Port))

	cm.Wait()
}

// Watcher provides json rpc and web service for the chain
type Watcher struct {
	sync.Mutex
	types.ServiceBase
	waitMap map[common.Address]*chan struct{}
	blockCh chan *types.Block
}

// NewWatcher returns a Watcher
func NewWatcher() *Watcher {
	s := &Watcher{
		waitMap: map[common.Address]*chan struct{}{},
		blockCh: make(chan *types.Block, 1000),
	}
	return s
}

// Name returns the name of the service
func (s *Watcher) Name() string {
	return "fleta.watcher"
}

// Init called when initialize service
func (s *Watcher) Init(pm types.ProcessManager, cn types.Provider) error {
	return nil
}

// OnLoadChain called when the chain loaded
func (s *Watcher) OnLoadChain(loader types.Loader) error {
	return nil
}

func (s *Watcher) addAddress(addr common.Address) *chan struct{} {
	ch := make(chan struct{})
	s.waitMap[addr] = &ch
	return &ch
}

// OnBlockConnected called when a block is connected to the chain
func (s *Watcher) OnBlockConnected(b *types.Block, events []types.Event, loader types.Loader) {
	s.blockCh <- b
}
