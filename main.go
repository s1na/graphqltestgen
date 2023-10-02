package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "graphqltestgen",
		Usage: "Extend a blockchain for graphql testing",
		Commands: []*cli.Command{
			{
				Name:  "head",
				Usage: "validate chain and print the current header",
				Action: func(c *cli.Context) error {
					head(c)
					return nil
				},
			},
			{
				Name:  "fill",
				Usage: "fill response for a graphql query",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "bin",
						Aliases: []string{"b"},
						Value:   "geth",
						Usage:   "path to geth binary",
					},
					&cli.StringFlag{
						Name:    "request",
						Aliases: []string{"r"},
						Value:   "request.gql",
						Usage:   "path to request file",
					},
					&cli.StringFlag{
						Name:    "response",
						Aliases: []string{"s"},
						Value:   "response.gql",
						Usage:   "path to response file",
					},
					&cli.IntFlag{
						Name:    "verbosity",
						Aliases: []string{"v"},
						Value:   3,
						Usage:   "verbosity of geth",
					},
				},
				Action: func(c *cli.Context) error {
					return fill(c)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "genesis",
				Aliases: []string{"g"},
				Value:   "genesis.json",
				Usage:   "path to genesis file",
			},
			&cli.StringFlag{
				Name:    "chain",
				Aliases: []string{"c"},
				Value:   "chain.rlp",
				Usage:   "path to chain file",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "newchain.rlp",
				Usage:   "path to output file",
			},
		},
		Action: func(c *cli.Context) error {
			run(c)
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		utils.Fatalf("error: %v\n", err)
	}
}

type stack struct {
	node  *node.Node
	db    ethdb.Database
	chain *core.BlockChain
}

func (n *stack) Stop() {
	n.node.Close()
	n.db.Close()
}

func run(ctx *cli.Context) {
	stack, err := importChain(ctx.String("genesis"), ctx.String("chain"))
	if err != nil {
		utils.Fatalf("error importing chain: %v\n", err)
	}
	defer stack.Stop()

	chain := stack.chain
	blocks, err := generateCancunBlocks(chain, stack.db)
	if err != nil {
		utils.Fatalf("error generating blocks: %v\n", err)
	}

	num, err := chain.InsertChain(blocks)
	if err != nil {
		utils.Fatalf("error: %v\n", err)
	}
	head := chain.CurrentHeader()
	ser, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		utils.Fatalf("error: %v\n", err)
	}
	fmt.Printf("Inserted %d blocks. New head is:\n%s\n", num, ser)

	// Write to file.
	if err := utils.ExportChain(chain, ctx.String("output")); err != nil {
		utils.Fatalf("error exporting chain: %v\n", err)
	}
}

func head(ctx *cli.Context) {
	stack, err := importChain(ctx.String("genesis"), ctx.String("chain"))
	if err != nil {
		utils.Fatalf("error importing chain: %v\n", err)
	}
	defer stack.Stop()
	head := stack.chain.CurrentHeader()
	if head == nil {
		utils.Fatalf("error: head is nil\n")
	}
	ser, err := json.MarshalIndent(head, "", "  ")
	if err != nil {
		utils.Fatalf("error: %v\n", err)
	}
	fmt.Printf("%s\n", ser)
}

func importChain(genesisPath, chainPath string) (*stack, error) {
	node, err := node.New(&node.Config{})
	if err != nil {
		return nil, fmt.Errorf("could not create node: %v", err)
	}
	chainDb, err := node.OpenDatabaseWithFreezer("chaindata", 0, 0, "chaindata", "", false)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %v", err)
	}

	engine := beacon.New(ethash.NewFaker())
	gspec, err := readGenesis(genesisPath)
	if err != nil {
		return nil, fmt.Errorf("invalid genesis file: %v", err)
	}

	chain, err := core.NewBlockChain(chainDb, &core.CacheConfig{TrieDirtyDisabled: true}, gspec, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create blockchain: %v", err)
	}
	if err := utils.ImportChain(chain, chainPath); err != nil {
		return nil, fmt.Errorf("could not import chain: %v", err)
	}
	return &stack{node: node, db: chainDb, chain: chain}, nil
}

func generateCancunBlocks(chain *core.BlockChain, db ethdb.Database) ([]*types.Block, error) {
	var (
		config  = chain.Config()
		key, _  = crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
		address = crypto.PubkeyToAddress(key.PublicKey)
		dest    = common.HexToAddress("0x6295ee1b4f6dd65047762f924ecd367c17eabf8f")
		//coinbase = common.HexToAddress("0x8888f1f195afa192cfee860698584c030f4c9db1")
		signer = types.NewCancunSigner(config.ChainID)
		//dad    = common.HexToAddress("0x0000000000000000000000000000000000000dad")
		head = chain.GetBlock(chain.CurrentBlock().Hash(), chain.CurrentBlock().Number.Uint64())
	)
	blocks, _ := core.GenerateChain(config, head, chain.Engine(), db, 1, func(i int, b *core.BlockGen) {
		b.SetPoS()
		tx, err := types.SignTx(types.NewTx(&types.BlobTx{
			Nonce:      b.TxNonce(address),
			Gas:        50000,
			GasTipCap:  uint256.NewInt(params.GWei),
			GasFeeCap:  new(uint256.Int).Mul(uint256.NewInt(3), uint256.NewInt(params.GWei)),
			To:         dest,
			Data:       common.FromHex("0x12a7b914"),
			BlobFeeCap: uint256.NewInt(params.GWei),
			BlobHashes: []common.Hash{{1}, {1, 2}},
		}), signer, key)
		if err != nil {
			utils.Fatalf("error: %v\n", err)
		}
		b.SetBlobGas(2 * params.BlobTxBlobGasPerBlob)
		b.AddTx(tx)
	})
	return blocks, nil
}

func fill(ctx *cli.Context) error {
	req, err := os.ReadFile(ctx.String("request"))
	if err != nil {
		return err
	}
	client, err := newGethClient(context.Background(), ctx, ctx.String("bin"), true)
	if err != nil {
		return err
	}
	if err := client.Start(context.Background(), true); err != nil {
		return err
	}
	defer func() {
		if err := client.Close(); err != nil {
			utils.Fatalf("failed to stop client: %s", err.Error())
		}
	}()
	time.Sleep(5 * time.Second)
	response, err := sendGraphQLRequest(client.GraphQLAddr(), req)
	if err != nil {
		return err
	}
	if err := os.WriteFile(ctx.String("response"), response, 0644); err != nil {
		return err
	}
	fmt.Printf("Wrote response to %s\n", ctx.String("response"))
	return nil
}

func readGenesis(path string) (*core.Genesis, error) {
	file, err := os.Open(path)
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

func sendGraphQLRequest(endpoint string, query []byte) ([]byte, error) {
	type request struct {
		Query string `json:"query"`
	}
	r := request{Query: string(query)}
	var err error
	query, err = json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return sendRequest("POST", endpoint, query)
}

func sendRequest(method, endpoint string, query []byte) ([]byte, error) {
	// Create a new request using http
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
