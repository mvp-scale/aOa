package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the aOa daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE:  runDaemonStop,
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)

	// Check if already running
	client := socket.NewClient(sockPath)
	if client.Ping() {
		fmt.Println("⚡ daemon already running")
		return nil
	}

	// Create fully wired app (bbolt, learner, enricher, session reader)
	a, err := app.New(app.Config{ProjectRoot: root})
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}

	if err := a.Start(); err != nil {
		return err
	}

	fmt.Printf("⚡ aOa daemon started at %s\n", sockPath)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n⚡ shutting down...")
	return a.Stop()
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		fmt.Println("⚡ daemon is not running")
		return nil
	}

	if err := client.Shutdown(); err != nil {
		return err
	}

	fmt.Println("⚡ daemon stopped")
	return nil
}
