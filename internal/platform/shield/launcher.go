package shield

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			if startEdgeAppWindow(edgePath, parsed.String(), dataDir) == nil {
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

func startEdgeAppWindow(edgePath, targetURL, dataDir string) error {
	args := []string{
		"--app=" + targetURL,
		"--window-size=600,760",
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
	}
	if strings.TrimSpace(dataDir) != "" {
		profileDir := filepath.Join(dataDir, "ShieldBrowser")
		if err := os.MkdirAll(profileDir, 0o700); err == nil {
			args = append(args, "--user-data-dir="+profileDir)
		}
	}

	cmd := exec.Command(edgePath, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
