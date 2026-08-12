package command

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maknihamdi/fluxexp/internal/ui"

	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	var address string

	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Serve the local read-only exploration portal",
		RunE: func(cmd *cobra.Command, _ []string) error {
			srv := &http.Server{
				Addr:              address,
				Handler:           ui.Handler(ui.NewService()),
				ReadHeaderTimeout: 5 * time.Second,
			}

			fmt.Fprintf(cmd.OutOrStdout(), "fluxexp ui → http://%s  (Ctrl-C to stop)\n", address)

			errc := make(chan error, 1)
			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					errc <- err
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
			select {
			case err := <-errc:
				return fmt.Errorf("serving ui: %w", err)
			case <-stop:
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				return srv.Shutdown(ctx)
			}
		},
	}

	cmd.Flags().StringVar(&address, "address", "127.0.0.1:8765", "address to bind the portal (loopback by default)")
	return cmd
}
