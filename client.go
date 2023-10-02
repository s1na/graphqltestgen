// File to a great extent copied from https://github.com/lightclient/rpctestgen/blob/8230021ff769691e2cb79b80fd309607512a3d4f/client.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/urfave/cli/v2"
)

const (
	NETWORKPORT = 30303
	HOST        = "127.0.0.1"
	PORT        = 9546
)

// gethClient is a wrapper around a go-ethereum instance on a separate thread.
type gethClient struct {
	flags   *cli.Context
	cmd     *exec.Cmd
	path    string
	workdir string
}

// newGethClient instantiates a new GethClient.
//
// The client's data directory is set to a temporary location and it
// initializes with the genesis and the provided blocks.
func newGethClient(ctx context.Context, cctx *cli.Context, path string, verbose bool) (*gethClient, error) {
	tmp, err := os.MkdirTemp("", "graphqltestgen-*")
	if err != nil {
		return nil, err
	}

	var (
		datadir = fmt.Sprintf("--datadir=%s", tmp)
		gcmode  = "--gcmode=archive"
	)

	// Run geth init.
	options := []string{datadir, gcmode, "init", cctx.String("genesis")}
	err = runCmd(ctx, path, verbose, options...)
	if err != nil {
		return nil, err
	}

	// Run geth import.
	options = []string{datadir, gcmode, "import", cctx.String("chain")}
	err = runCmd(ctx, path, verbose, options...)
	if err != nil {
		return nil, err
	}

	return &gethClient{path: path, workdir: tmp, flags: cctx}, nil
}

// Start starts geth, but does not wait for the command to exit.
func (g *gethClient) Start(ctx context.Context, verbose bool) error {
	fmt.Println("starting client")
	var (
		options = []string{
			fmt.Sprintf("--datadir=%s", g.workdir),
			fmt.Sprintf("--verbosity=%d", g.flags.Int("logLevel")),
			fmt.Sprintf("--port=%d", NETWORKPORT),
			"--gcmode=archive",
			"--nodiscover",
			"--http",
			"--graphql",
			fmt.Sprintf("--http.addr=%s", HOST),
			fmt.Sprintf("--http.port=%d", PORT),
		}
	)
	g.cmd = exec.CommandContext(
		ctx,
		g.path,
		options...,
	)
	if verbose {
		g.cmd.Stdout = os.Stdout
		g.cmd.Stderr = os.Stderr
	}
	if err := g.cmd.Start(); err != nil {
		return err
	}
	return nil
}

// GraphQLAddr returns the address where the client is servering its endpoint.
func (g *gethClient) GraphQLAddr() string {
	return fmt.Sprintf("http://%s:%d/graphql", HOST, PORT)
}

// Close closes the client.
func (g *gethClient) Close() error {
	g.cmd.Process.Kill()
	g.cmd.Wait()
	return os.RemoveAll(g.workdir)
}

// runCmd runs a command and outputs the command's stdout and stderr to the
// caller's stdout and stderr if verbose is set.
func runCmd(ctx context.Context, path string, verbose bool, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
