package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sumitasok/sa.automation.wallet/internal/uiserver"
)

// uiAssets is the static UI. It is embedded so `wallet ui` works from any
// working directory; --dev serves the same files off disk for a fast edit loop.
//
//go:embed ui/index.html ui/app.js ui/styles.css
var uiAssets embed.FS

// runUI serves the read-only wallet dashboard over the external config data
// directory. Nothing is written: the directory is the pack's canonical output
// location and this command only reads it.
func runUI(args []string) error {
	fs_ := flag.NewFlagSet("ui", flag.ExitOnError)
	configPath := fs_.String("config-path", "", "config root (default: $CONFIG_PATH or ~/automation-monorepo-config)")
	dataDir := fs_.String("data-dir", "", "wallet data directory (default: <config-path>/data/expense-domain/wallet)")
	addr := fs_.String("addr", "127.0.0.1:8787", "listen address; keep it on loopback — the data is personal")
	dev := fs_.Bool("dev", false, "serve ui/ from disk instead of the embedded copy")
	open := fs_.Bool("open", false, "open the dashboard in the default browser")
	quiet := fs_.Bool("quiet", false, "suppress per-request logging")
	fs_.Parse(args)

	resolved, err := resolveDataDir(*configPath, *dataDir)
	if err != nil {
		return err
	}

	static, err := uiFileSystem(*dev)
	if err != nil {
		return err
	}

	logf := log.Printf
	if *quiet {
		logf = func(string, ...any) {}
	}

	srv, err := uiserver.New(uiserver.Options{
		DataDir: resolved,
		Static:  static,
		Addr:    *addr,
		Log:     logf,
	})
	if err != nil {
		return err
	}

	url := "http://" + *addr
	if host, port, splitErr := net.SplitHostPort(*addr); splitErr == nil && (host == "" || host == "0.0.0.0") {
		url = "http://127.0.0.1:" + port
	}

	fmt.Printf("Wallet UI\n")
	fmt.Printf("  data dir : %s\n", resolved)
	fmt.Printf("  assets   : %s\n", assetSource(*dev))
	fmt.Printf("  url      : %s\n", url)
	fmt.Printf("  mode     : read-only (no writes to the data directory)\n\n")

	if *open {
		openBrowser(url)
	}
	return srv.ListenAndServe()
}

// resolveDataDir applies the precedence the rest of the pack uses: an explicit
// flag, then AUTO_DATA_DIR (workspace mode), then <config-path>/data/expense-domain/wallet.
func resolveDataDir(configPath, dataDir string) (string, error) {
	if dataDir != "" {
		return expandHome(dataDir)
	}
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		if auto := os.Getenv("AUTO_DATA_DIR"); auto != "" {
			return expandHome(filepath.Join(auto, "wallet"))
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configPath = filepath.Join(home, "automation-monorepo-config")
	}
	expanded, err := expandHome(configPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(expanded, "data", "expense-domain", "wallet"), nil
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), "/"))
	}
	return filepath.Abs(path)
}

// uiFileSystem returns the static assets rooted at the UI directory, so
// "/index.html" resolves without the "ui/" prefix.
func uiFileSystem(dev bool) (fs.FS, error) {
	if dev {
		if _, err := os.Stat(filepath.Join("ui", "index.html")); err != nil {
			return nil, fmt.Errorf("--dev needs ui/index.html relative to the working directory: %w", err)
		}
		return os.DirFS("ui"), nil
	}
	return fs.Sub(uiAssets, "ui")
}

func assetSource(dev bool) string {
	if dev {
		return "./ui (dev, reloaded from disk)"
	}
	return "embedded"
}

// openBrowser is best-effort: a failure to launch is not a reason to stop
// serving, since the URL is printed either way.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("could not open browser: %v", err)
	}
}
