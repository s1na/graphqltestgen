package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
)

func main() {
	gspec, err := readGenesis()
	if err != nil {
		utils.Fatalf("invalid genesis file: %v", err)
	}

	nodeCfg := &node.Config{}
	stack, err := node.New(nodeCfg)
	if err != nil {
		utils.Fatalf("could not create node: %v", err)
	}
	defer stack.Close()

	chainDb, err := stack.OpenDatabaseWithFreezer("chaindata", 0, 0, "chaindata", "", false)
	if err != nil {
		utils.Fatalf("could not open database: %v", err)
	}
	defer chainDb.Close()

	ethone := ethash.New(ethash.Config{}, nil, false)
	engine := beacon.New(ethone)
	chain, err := core.NewBlockChain(chainDb, &core.CacheConfig{TrieDirtyDisabled: true}, gspec, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		utils.Fatalf("Can't create BlockChain: %v", err)
	}
	utils.ImportChain(chain, "chain.rlp")

	var (
		key, _  = crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
		address = crypto.PubkeyToAddress(key.PublicKey)
		dest    = common.HexToAddress("0x6295ee1b4f6dd65047762f924ecd367c17eabf8f")
		//coinbase = common.HexToAddress("0x8888f1f195afa192cfee860698584c030f4c9db1")
		signer = types.NewLondonSigner(gspec.Config.ChainID)
		dad    = common.HexToAddress("0x0000000000000000000000000000000000000dad")
		head   = chain.GetBlock(chain.CurrentBlock().Hash(), chain.CurrentBlock().Number.Uint64())
	)

	blocks, _ := core.GenerateChain(gspec.Config, head, engine, chainDb, 1, func(i int, b *core.BlockGen) {
		b.SetPoS()
		tx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
			Nonce:     b.TxNonce(address),
			Gas:       50000,
			GasTipCap: big.NewInt(params.GWei),
			GasFeeCap: new(big.Int).Mul(big.NewInt(3), big.NewInt(params.GWei)),
			To:        &dest,
			Data:      common.FromHex("0x12a7b914"),
			AccessList: types.AccessList{
				{
					Address:     dest,
					StorageKeys: []common.Hash{common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")},
				},
			},
		}), signer, key)
		b.AddTx(tx)
		b.AddWithdrawal(&types.Withdrawal{
			Index:     5,
			Validator: 10,
			Address:   dad,
			Amount:    new(big.Int).Mul(big.NewInt(10), big.NewInt(params.GWei)).Uint64(),
		})
	})

	fmt.Printf("everything finished. Head at: %v\n", chain.CurrentBlock().Number.Uint64())
	ser, err := json.Marshal(blocks[0].Header())
	if err != nil {
		utils.Fatalf("error: %v\n", err)
	} else {
		fmt.Printf("head block is: %v\n", string(ser))
	}
	num, err := chain.InsertChain(blocks)
	if err != nil {
		utils.Fatalf("error: %v\n", err)
	}
	fmt.Printf("inserted %v blocks\n", num)

	head = chain.GetBlock(chain.CurrentBlock().Hash(), chain.CurrentBlock().Number.Uint64())
	ser, err = json.Marshal(head.Header())
	fmt.Printf("head block is: %v\n", string(ser))

	// Now export to newchain.rlp
	if err := utils.ExportChain(chain, "newchain.rlp"); err != nil {
		utils.Fatalf("error exporting chain: %v\n", err)
	}
}

func readGenesis() (*core.Genesis, error) {
	filePath := "genesis.json"
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	genesis := new(core.Genesis)
	if err := json.NewDecoder(file).Decode(genesis); err != nil {
		return nil, err
	}
	return genesis, nil
}
