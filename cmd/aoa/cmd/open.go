package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open web dashboard in browser",
	Long:  "Opens the aOa dashboard in your default browser. Requires the daemon to be running.",
	RunE:  runOpen,
}

func runOpen(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa daemon start")
	}

	// Read the HTTP port file
	httpPortPath := filepath.Join(root, ".aoa", "http.port")
	portData, err := os.ReadFile(httpPortPath)
	if err != nil {
		return fmt.Errorf("dashboard not available (no port file)\n  → restart daemon: aoa daemon stop && aoa daemon start")
	}

	port := strings.TrimSpace(string(portData))
	url := fmt.Sprintf("http://localhost:%s", port)

	// Try to open in browser
	var openErr error
	switch runtime.GOOS {
	case "linux":
		openErr = exec.Command("xdg-open", url).Start()
	case "darwin":
		openErr = exec.Command("open", url).Start()
	default:
		openErr = fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}

	if openErr != nil {
		fmt.Printf("⚡ dashboard: %s\n", url)
		fmt.Printf("  (could not open browser: %v)\n", openErr)
		return nil
	}

	fmt.Printf("⚡ opening %s\n", url)
	return nil
}
