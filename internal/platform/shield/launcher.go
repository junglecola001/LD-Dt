package shield

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// LaunchWindow opens an isolated native Microsoft Edge app window (--app) for
// Cloudflare verification or Linux.do login so requests never pass through Wails
// WebResourceRequested header injection or WebView2 visibility races. Falls back
// to fallbackOpen (default system browser) when Edge is unavailable.
func LaunchWindow(targetURL, dataDir string, fallbackOpen func(string) error) error {
	parsed, err := validateShieldURL(targetURL)
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if edgePath := findEdgeExecutable(); edgePath != "" {
			if _, startErr := startEdgeAppWindow(edgePath, parsed.String(), dataDir, 0); startErr == nil {
				return nil
			}
		}
	}

	if fallbackOpen == nil {
		return errors.New("未找到可用浏览器打开验证窗口")
	}
	return fallbackOpen(parsed.String())
}

func validateShieldURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("仅允许通过 HTTPS 打开验证或登录页面")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "linux.do" && !strings.HasSuffix(host, ".linux.do") {
		return nil, errors.New("验证窗口仅允许访问 linux.do 域名")
	}
	return parsed, nil
}

func findEdgeExecutable() string {
	candidates := make([]string, 0, 4)
	if p := os.Getenv("ProgramFiles(x86)"); p != "" {
		candidates = append(candidates, filepath.Join(p, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	if p := os.Getenv("ProgramFiles"); p != "" {
		candidates = append(candidates, filepath.Join(p, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		candidates = append(candidates, filepath.Join(p, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	for _, bin := range []string{"msedge.exe", "msedge"} {
		if found, err := exec.LookPath(bin); err == nil && found != "" {
			return found
		}
	}
	return ""
}

func allocateLoopbackPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, errors.New("failed to allocate loopback port")
	}
	return addr.Port, nil
}

func edgeBridgeProfileDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		return ""
	}
	return filepath.Join(dataDir, "EdgeBridge")
}

func startEdgeAppWindow(edgePath, targetURL, dataDir string, cdpPort int) (*exec.Cmd, error) {
	args := []string{
		"--app=" + targetURL,
		"--window-size=600,760",
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
	}
	if cdpPort > 0 {
		args = append(args,
			fmt.Sprintf("--remote-debugging-port=%d", cdpPort),
			fmt.Sprintf("--remote-allow-origins=http://127.0.0.1:%d", cdpPort),
		)
	}
	if profileDir := edgeBridgeProfileDir(dataDir); profileDir != "" {
		if err := os.MkdirAll(profileDir, 0o700); err == nil {
			args = append(args, "--user-data-dir="+profileDir)
		}
	}

	cmd := exec.Command(edgePath, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return cmd, nil
}

func readPersistedPort(dataDir string) int {
	profileDir := edgeBridgeProfileDir(dataDir)
	if profileDir == "" {
		return 0
	}
	raw, err := os.ReadFile(filepath.Join(profileDir, "ld-cdp-port.txt"))
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func writePersistedPort(dataDir string, port int) {
	profileDir := edgeBridgeProfileDir(dataDir)
	if profileDir == "" || port <= 0 {
		return
	}
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(profileDir, "ld-cdp-port.txt"), []byte(strconv.Itoa(port)), 0o600)
}
