package hub

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type HubInfo struct {
	Pid        int    `json:"pid"`
	HttpsPort  int    `json:"httpsPort"`
	HttpPort   int    `json:"httpPort"`
	LsVersion  string `json:"lsVersion"`
	CsrfToken  string `json:"csrfToken"`
	ConfigPath string `json:"-"`
}

var httpPortRegex = regexp.MustCompile(`Language server listening on random port at (\d+) for HTTP`)

// DiscoverHub locates and reads the active Antigravity hub configuration.
// If overridePath is non-empty, it reads that path directly.
// Otherwise, it dynamically discovers the live Antigravity language_server process
// and its listening HTTP port. If process discovery fails, it falls back to
// scanning ~/.gemini/antigravity/daemon/hub_*.json, pruning stale records.
func DiscoverHub(overridePath string) (*HubInfo, error) {
	if overridePath != "" {
		return readAndParseHub(overridePath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}
	daemonDir := filepath.Join(home, ".gemini/antigravity/daemon")

	// 1. Try dynamic process discovery from running language_server
	if hub, err := discoverFromProcess(home); err == nil && hub != nil {
		// Sync the live hub info into the daemon dir so other tools/tests find it
		_ = saveHubInfo(daemonDir, hub)
		cleanStaleDaemonFiles(daemonDir, hub.Pid)
		return hub, nil
	}

	// 2. Fall back to scanning ~/.gemini/antigravity/daemon/hub_*.json
	hub, err := discoverFromDaemonDir(daemonDir)
	if err == nil && hub != nil {
		return hub, nil
	}

	return nil, fmt.Errorf("no active Antigravity hub found (make sure Antigravity is running): %w", err)
}

// discoverFromProcess scans /proc for running language_server instances.
func discoverFromProcess(home string) (*HubInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("cannot read /proc: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		args := strings.Split(string(cmdlineBytes), "\x00")
		if len(args) == 0 {
			continue
		}

		isHub := false
		csrfToken := ""
		lsVersion := "2.28.2"

		for i, arg := range args {
			if strings.Contains(arg, "language_server") {
				isHub = true
			}
			if arg == "--csrf_token" && i+1 < len(args) {
				csrfToken = args[i+1]
			} else if strings.HasPrefix(arg, "--csrf_token=") {
				csrfToken = strings.TrimPrefix(arg, "--csrf_token=")
			}
			if arg == "--override_ide_version" && i+1 < len(args) {
				lsVersion = args[i+1]
			} else if strings.HasPrefix(arg, "--override_ide_version=") {
				lsVersion = strings.TrimPrefix(arg, "--override_ide_version=")
			}
		}

		if !isHub || csrfToken == "" {
			continue
		}

		// Found candidate language_server process! Now find its HTTP listening port.
		port := findHttpPort(pid, home, csrfToken)
		if port > 0 {
			return &HubInfo{
				Pid:       pid,
				HttpPort:  port,
				CsrfToken: csrfToken,
				LsVersion: lsVersion,
			}, nil
		}
	}

	return nil, fmt.Errorf("no running language_server process with valid HTTP port found")
}

// findHttpPort tries log parsing first, then listening socket inspection from /proc/<pid>/fd.
func findHttpPort(pid int, home string, csrfToken string) int {
	// A. Check language_server.log
	logPaths := []string{
		filepath.Join(home, ".config/Antigravity/logs/language_server.log"),
		filepath.Join(home, ".gemini/antigravity/logs/language_server.log"),
	}

	for _, logPath := range logPaths {
		if port := parsePortFromLog(logPath); port > 0 {
			if probeWebSocket(port, csrfToken) {
				return port
			}
		}
	}

	// B. Inspect process listening sockets via /proc/<pid>/fd and /proc/net/tcp
	ports := getProcessListeningPorts(pid)
	for _, port := range ports {
		if probeWebSocket(port, csrfToken) {
			return port
		}
	}

	return 0
}

func parsePortFromLog(logPath string) int {
	file, err := os.Open(logPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	// Read lines to find the last occurrence of the listening port
	scanner := bufio.NewScanner(file)
	lastPort := 0
	for scanner.Scan() {
		line := scanner.Text()
		if m := httpPortRegex.FindStringSubmatch(line); len(m) == 2 {
			if p, err := strconv.Atoi(m[1]); err == nil && p > 0 {
				lastPort = p
			}
		}
	}
	return lastPort
}

// getProcessListeningPorts retrieves local listening TCP ports for a given PID.
func getProcessListeningPorts(pid int) []int {
	fdDir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return nil
	}

	socketInodes := make(map[uint64]bool)
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
			inodeStr := link[8 : len(link)-1]
			if inode, err := strconv.ParseUint(inodeStr, 10, 64); err == nil {
				socketInodes[inode] = true
			}
		}
	}

	if len(socketInodes) == 0 {
		return nil
	}

	var ports []int
	for _, tcpPath := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(tcpPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 {
				continue
			}
			// State 0A is TCP_LISTEN
			if fields[3] != "0A" {
				continue
			}
			inode, err := strconv.ParseUint(fields[9], 10, 64)
			if err != nil || !socketInodes[inode] {
				continue
			}
			// Local address: HEX_IP:HEX_PORT
			parts := strings.Split(fields[1], ":")
			if len(parts) == 2 {
				if port64, err := strconv.ParseInt(parts[1], 16, 32); err == nil && port64 > 0 {
					ports = append(ports, int(port64))
				}
			}
		}
		f.Close()
	}

	return ports
}

// probeWebSocket checks if an HTTP port is indeed the Antigravity websocket RPC server.
func probeWebSocket(port int, csrfToken string) bool {
	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/connect-websocket", port), nil)
	if err != nil {
		return false
	}
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// The endpoint returns 200 OK or 400 Bad Request (since upgrade headers weren't provided)
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest
}

// discoverFromDaemonDir scans ~/.gemini/antigravity/daemon/hub_*.json.
func discoverFromDaemonDir(daemonDir string) (*HubInfo, error) {
	files, err := filepath.Glob(filepath.Join(daemonDir, "hub_*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan hub files in %s: %w", daemonDir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no hub json files found in %s", daemonDir)
	}

	var bestHub *HubInfo
	var newestTime time.Time

	for _, file := range files {
		hub, err := readAndParseHub(file)
		if err != nil {
			_ = os.Remove(file)
			continue
		}

		// Verify process is alive
		if hub.Pid > 0 && !isPidAlive(hub.Pid) {
			_ = os.Remove(file)
			continue
		}

		// Verify TCP port is reachable
		if !probeTCP(hub.HttpPort) {
			_ = os.Remove(file)
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			bestHub = hub
		}
	}

	if bestHub == nil {
		return nil, fmt.Errorf("no live Antigravity hub found in %s (all files were stale or offline)", daemonDir)
	}

	return bestHub, nil
}

func isPidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, Signal(0) checks if the process exists without sending a signal.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func probeTCP(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func readAndParseHub(filePath string) (*HubInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var hub HubInfo
	if err := json.Unmarshal(data, &hub); err != nil {
		return nil, fmt.Errorf("failed to parse JSON in %s: %w", filePath, err)
	}
	hub.ConfigPath = filePath
	if hub.HttpPort == 0 || hub.CsrfToken == "" {
		return nil, fmt.Errorf("invalid HubInfo in %s (missing httpPort or csrfToken)", filePath)
	}
	return &hub, nil
}

func saveHubInfo(daemonDir string, hub *HubInfo) error {
	if err := os.MkdirAll(daemonDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(hub, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(daemonDir, fmt.Sprintf("hub_%d.json", hub.Pid))
	hub.ConfigPath = dest
	return os.WriteFile(dest, data, 0644)
}

func cleanStaleDaemonFiles(daemonDir string, currentPid int) {
	files, err := filepath.Glob(filepath.Join(daemonDir, "hub_*.json"))
	if err != nil {
		return
	}
	for _, file := range files {
		base := filepath.Base(file)
		var pid int
		if _, err := fmt.Sscanf(base, "hub_%d.json", &pid); err == nil {
			if pid != currentPid && (!isPidAlive(pid) || !probeFileHub(file)) {
				_ = os.Remove(file)
			}
		}
	}
}

func probeFileHub(file string) bool {
	hub, err := readAndParseHub(file)
	if err != nil {
		return false
	}
	return probeTCP(hub.HttpPort)
}
