package main

import (
	"fmt"
	"github.com/restic/restic/internal/debug"
	"github.com/restic/restic/internal/servehttp"
	"github.com/spf13/cobra"
	"net"
	"net/http"
)

var cmdServe = &cobra.Command{
	Use:   "serve [flags]",
	Short: "Start HTTP server",
	Long: `
The "serve" command starts a simple HTTP to view created snapshots. This is a
read-only server, existing repository cannot be modified.

EXIT STATUS
===========

Exit status is 0 if the command was successful, and non-zero if there was any error.
`,
	DisableAutoGenTag: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServe(serveOptions, globalOptions, args)
	},
}

// ServeOptions collects all options for the serve command.
type ServeOptions struct {
	Port int
}

var serveOptions ServeOptions

func init() {
	cmdRoot.AddCommand(cmdServe)

	serveFlags := cmdMount.Flags()
	serveFlags.IntVar(&serveOptions.Port, "port", 0, "specify port for starting HTTP server")
}

func runServe(opts ServeOptions, gopts GlobalOptions, args []string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", opts.Port))
	if err != nil {
		return err
	}
	defer listener.Close()

	debug.Log("start serve")
	defer debug.Log("finish serve")

	repo, err := OpenRepository(gopts)
	if err != nil {
		return err
	}

	if !gopts.NoLock {
		lock, err := lockRepo(gopts.ctx, repo)
		defer unlockRepo(lock)
		if err != nil {
			return err
		}
	}

	err = repo.LoadIndex(gopts.ctx)
	if err != nil {
		return err
	}

	//cfg := fuse.Config{
	//	OwnerIsRoot:      opts.OwnerRoot,
	//	Hosts:            opts.Hosts,
	//	Tags:             opts.Tags,
	//	Paths:            opts.Paths,
	//	SnapshotTemplate: opts.SnapshotTemplate,
	//}
	//root := fuse.NewRoot(repo, cfg)

	Printf("Now serving the repository at http://%s\n", listener.Addr())
	Printf("When finished, quit with Ctrl-c here.\n")

	rootFS, err := servehttp.NewRoot(gopts.ctx, repo, servehttp.Config{})
	if err != nil {
		return err
	}
	httpFS := http.FS(rootFS)
	handler := http.FileServer(httpFS)

	debug.Log("serving server at http://%s", listener.Addr())
	err = http.Serve(listener, handler)
	if err != nil {
		return err
	}

	return nil
}
