//go:build linux

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

const (
	mappingID        = "pty-e2e"
	testUserPassword = "serialserver-e2e-password"
)

func TestRelayThroughPseudoTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("serial relay e2e test is skipped in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))
	binDir := t.TempDir()

	serverBin := filepath.Join(binDir, "vsp-server")
	deviceAgentBin := filepath.Join(binDir, "device-agent")
	desktopGatewayBin := filepath.Join(binDir, "desktop-gateway")

	buildBinary(ctx, t, filepath.Join(repoRoot, "vsp-server"), serverBin, "./cmd")
	buildBinary(ctx, t, filepath.Join(repoRoot, "vsp-client"), deviceAgentBin, "./cmd/device-agent")
	buildBinary(ctx, t, filepath.Join(repoRoot, "vsp-client"), desktopGatewayBin, "./cmd/desktop-gateway")

	serverPort := freeTCPPort(t)
	serverHTTP := fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverProc := startProcess(t, ctx, "vsp-server", filepath.Join(repoRoot, "vsp-server"), []string{
		fmt.Sprintf("VSP_SERVER_PORT=%d", serverPort),
		"VSP_JWT_SECRET=serialserver-e2e-jwt-secret",
		"VSP_DB_PATH=" + filepath.Join(t.TempDir(), "vsp.db"),
	}, serverBin)
	waitForServer(ctx, t, serverHTTP, serverProc)

	client := &http.Client{Timeout: 3 * time.Second}
	username := fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	registerUser(t, client, serverHTTP, username)
	token := loginUser(t, client, serverHTTP, username)
	device := createDevice(t, client, serverHTTP, token)

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("open pseudo-terminal: %v", err)
	}
	serialPath := slave.Name()
	if err := slave.Close(); err != nil {
		t.Fatalf("close pseudo-terminal slave handle: %v", err)
	}
	t.Cleanup(func() {
		_ = master.Close()
	})

	deviceProc := startProcess(t, ctx, "device-agent", filepath.Join(repoRoot, "vsp-client"), nil,
		deviceAgentBin,
		"-server", serverAddr,
		"-key", device.DeviceKey,
		"-mapping", mappingID,
		"-name", "E2E PTY",
		"-port", serialPath,
		"-baud", "115200",
		"-reconnect=false",
	)
	waitForMappingOnline(ctx, t, client, serverHTTP, token, device.ID, deviceProc, serverProc)

	gatewayAddr := fmt.Sprintf("127.0.0.1:%d", freeTCPPort(t))
	gatewayProc := startProcess(t, ctx, "desktop-gateway", filepath.Join(repoRoot, "vsp-client"), nil,
		desktopGatewayBin,
		"-server", serverAddr,
		"-token", token,
		"-device-id", strconv.FormatUint(uint64(device.ID), 10),
		"-mapping", mappingID,
		"-listen", gatewayAddr,
	)
	tcpConn := waitForTCP(ctx, t, gatewayAddr, gatewayProc, deviceProc, serverProc)
	defer tcpConn.Close()

	tcpPayload := []byte("tcp-to-serial:e2e\n")
	writeWithDeadline(t, tcpConn, tcpPayload, "tcp client")
	readUntilContains(t, master, []byte("tcp-to-serial:e2e"), 10*time.Second, "pseudo-terminal master")

	serialReply := []byte("serial-to-tcp:e2e\n")
	writeWithDeadline(t, master, serialReply, "pseudo-terminal master")
	readUntilContains(t, tcpConn, []byte("serial-to-tcp:e2e"), 10*time.Second, "tcp client")

	if err := tcpConn.Close(); err != nil {
		t.Fatalf("close tcp client: %v", err)
	}
	deviceProc.stop(t, 5*time.Second)
	gatewayProc.stop(t, 5*time.Second)
	serverProc.stop(t, 5*time.Second)
}

type apiEnvelope[T any] struct {
	Data  T      `json:"data"`
	Error string `json:"error"`
}

type loginData struct {
	Token string `json:"token"`
}

type deviceData struct {
	ID        uint   `json:"id"`
	DeviceKey string `json:"device_key"`
}

type mappingState struct {
	Mapping struct {
		ID string `json:"id"`
	} `json:"mapping"`
	Online bool `json:"online"`
	Busy   bool `json:"busy"`
}

func registerUser(t *testing.T, client *http.Client, baseURL, username string) {
	t.Helper()
	_ = postJSON[map[string]any](t, client, baseURL+"/api/auth/register", "", http.StatusCreated, map[string]string{
		"username": username,
		"email":    username + "@example.invalid",
		"password": testUserPassword,
	})
}

func loginUser(t *testing.T, client *http.Client, baseURL, username string) string {
	t.Helper()
	data := postJSON[loginData](t, client, baseURL+"/api/auth/login", "", http.StatusOK, map[string]string{
		"username": username,
		"password": testUserPassword,
	})
	if data.Token == "" {
		t.Fatal("login response did not include a token")
	}
	return data.Token
}

func createDevice(t *testing.T, client *http.Client, baseURL, token string) deviceData {
	t.Helper()
	data := postJSON[deviceData](t, client, baseURL+"/api/devices", token, http.StatusCreated, map[string]string{
		"name": "e2e-device",
	})
	if data.ID == 0 || data.DeviceKey == "" {
		t.Fatalf("create device response missing id or device key: %+v", data)
	}
	return data
}

func postJSON[T any](t *testing.T, client *http.Client, endpoint, token string, wantStatus int, body any) T {
	t.Helper()

	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &encoded)
	if err != nil {
		t.Fatalf("create request %s: %v", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return doJSON[T](t, client, req, wantStatus)
}

func getJSON[T any](t *testing.T, client *http.Client, endpoint, token string, wantStatus int) T {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create request %s: %v", endpoint, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return doJSON[T](t, client, req, wantStatus)
}

func doJSON[T any](t *testing.T, client *http.Client, req *http.Request, wantStatus int) T {
	t.Helper()

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", req.Method, req.URL, resp.StatusCode, wantStatus, string(body))
	}

	var envelope apiEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response from %s: %v\n%s", req.URL, err, string(body))
	}
	if envelope.Error != "" {
		t.Fatalf("%s returned API error: %s", req.URL, envelope.Error)
	}
	return envelope.Data
}

func waitForServer(ctx context.Context, t *testing.T, baseURL string, proc *managedProcess) {
	t.Helper()
	client := &http.Client{Timeout: 500 * time.Millisecond}
	waitUntil(ctx, t, "vsp-server HTTP readiness", []*managedProcess{proc}, func() bool {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/api/stats", nil)
		if err != nil {
			t.Fatalf("create readiness request: %v", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode > 0
	})
}

func waitForMappingOnline(ctx context.Context, t *testing.T, client *http.Client, baseURL, token string, deviceID uint, procs ...*managedProcess) {
	t.Helper()
	endpoint := fmt.Sprintf("%s/api/devices/%d/mappings", baseURL, deviceID)
	waitUntil(ctx, t, "device mapping online", procs, func() bool {
		mappings := getJSON[[]mappingState](t, client, endpoint, token, http.StatusOK)
		for _, state := range mappings {
			if state.Mapping.ID == mappingID && state.Online && !state.Busy {
				return true
			}
		}
		return false
	})
}

func waitForTCP(ctx context.Context, t *testing.T, addr string, procs ...*managedProcess) net.Conn {
	t.Helper()
	var conn net.Conn
	waitUntil(ctx, t, "desktop gateway TCP listener", procs, func() bool {
		var err error
		conn, err = net.DialTimeout("tcp", addr, 200*time.Millisecond)
		return err == nil
	})
	return conn
}

func waitUntil(ctx context.Context, t *testing.T, label string, procs []*managedProcess, ok func() bool) {
	t.Helper()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		ensureRunning(t, procs...)
		if ok() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s: %v", label, ctx.Err())
		case <-ticker.C:
		}
	}
}

func buildBinary(ctx context.Context, t *testing.T, dir, out, pkg string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, pkg)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s in %s: %v\n%s", pkg, dir, err, string(output))
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func writeWithDeadline(t *testing.T, writer io.Writer, data []byte, label string) {
	t.Helper()
	if deadlineWriter, ok := writer.(interface{ SetWriteDeadline(time.Time) error }); ok {
		_ = deadlineWriter.SetWriteDeadline(time.Now().Add(5 * time.Second))
		defer deadlineWriter.SetWriteDeadline(time.Time{})
	}
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("write to %s: %v", label, err)
	}
	if n != len(data) {
		t.Fatalf("write to %s wrote %d bytes, want %d", label, n, len(data))
	}
}

func readUntilContains(t *testing.T, reader io.Reader, want []byte, timeout time.Duration, label string) []byte {
	t.Helper()
	deadline := time.Now().Add(timeout)
	if deadlineReader, ok := reader.(interface{ SetReadDeadline(time.Time) error }); ok {
		_ = deadlineReader.SetReadDeadline(deadline)
		defer deadlineReader.SetReadDeadline(time.Time{})
	}

	var got bytes.Buffer
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			got.Write(buf[:n])
			if bytes.Contains(got.Bytes(), want) {
				return got.Bytes()
			}
		}
		if err != nil {
			if isTimeout(err) || time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %q on %s; got %q", string(want), label, got.String())
			}
			t.Fatalf("read from %s: %v; got %q", label, err, got.String())
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q on %s; got %q", string(want), label, got.String())
		}
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.Is(err, os.ErrDeadlineExceeded) ||
		(errors.As(err, &netErr) && netErr.Timeout()) ||
		strings.Contains(err.Error(), "i/o timeout")
}

type managedProcess struct {
	name   string
	cancel context.CancelFunc
	cmd    *exec.Cmd
	logs   *safeBuffer
	done   chan struct{}

	mu  sync.Mutex
	err error
}

func startProcess(t *testing.T, parent context.Context, name, dir string, env []string, args ...string) *managedProcess {
	t.Helper()
	if len(args) == 0 {
		t.Fatalf("start %s: missing command", name)
	}

	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	logs := &safeBuffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start %s: %v", name, err)
	}

	proc := &managedProcess{
		name:   name,
		cancel: cancel,
		cmd:    cmd,
		logs:   logs,
		done:   make(chan struct{}),
	}
	go func() {
		err := cmd.Wait()
		proc.mu.Lock()
		proc.err = err
		proc.mu.Unlock()
		close(proc.done)
	}()

	t.Cleanup(func() {
		proc.stop(t, 3*time.Second)
		if t.Failed() {
			t.Logf("%s logs:\n%s", proc.name, proc.logs.String())
		}
	})

	return proc
}

func ensureRunning(t *testing.T, procs ...*managedProcess) {
	t.Helper()
	for _, proc := range procs {
		if proc == nil {
			continue
		}
		if exited, err := proc.exited(); exited {
			t.Fatalf("%s exited early: %v\nlogs:\n%s", proc.name, err, proc.logs.String())
		}
	}
}

func (p *managedProcess) exited() (bool, error) {
	select {
	case <-p.done:
		p.mu.Lock()
		defer p.mu.Unlock()
		return true, p.err
	default:
		return false, nil
	}
}

func (p *managedProcess) stop(t *testing.T, timeout time.Duration) {
	t.Helper()
	if exited, _ := p.exited(); exited {
		return
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Signal(os.Interrupt)
	}
	select {
	case <-p.done:
		return
	case <-time.After(timeout):
	}

	p.cancel()
	select {
	case <-p.done:
		return
	case <-time.After(2 * time.Second):
	}

	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not exit after interrupt and kill\nlogs:\n%s", p.name, p.logs.String())
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
