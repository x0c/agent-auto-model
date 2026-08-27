package codex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/x0c/agent-auto-model/internal/config"
	"github.com/x0c/agent-auto-model/internal/runtime/codex/rewrite"
	"github.com/x0c/agent-auto-model/internal/runtime/codex/ws"
)

func runProxiedTUI(home, real string, args []string, locked bool) error {
	cfg := config.LoadEffective(home)
	st := rewrite.NewState(config.ModelsFor(cfg, config.RuntimeCodex), locked)

	dir := os.TempDir()
	sock := filepath.Join(dir, fmt.Sprintf("agent-auto-model-codex-%d.sock", os.Getpid()))
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return fmt.Errorf("启动 Codex 代理失败：%w", err)
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(sock)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	up := exec.CommandContext(ctx, real, "app-server", "--stdio")
	upIn, err := up.StdinPipe()
	if err != nil {
		return err
	}
	upOut, err := up.StdoutPipe()
	if err != nil {
		return err
	}
	up.Stderr = os.Stderr
	if err := up.Start(); err != nil {
		return fmt.Errorf("启动 Codex app-server 失败：%w", err)
	}
	defer func() {
		cancel()
		_ = up.Wait()
	}()

	var upMu sync.Mutex
	go func() {
		_ = serveOne(ctx, ln, home, st, upIn, upOut, &upMu)
	}()

	tuiArgs := append([]string{"--remote", "unix://" + sock}, args...)
	tui := exec.Command(real, tuiArgs...)
	tui.Stdin = os.Stdin
	tui.Stdout = os.Stdout
	tui.Stderr = os.Stderr
	tui.Env = os.Environ()
	err = tui.Run()
	cancel()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		return err
	}
	return nil
}

func serveOne(ctx context.Context, ln net.Listener, home string, st *rewrite.State, upIn io.WriteCloser, upOut io.Reader, upMu *sync.Mutex) error {
	type acceptRes struct {
		c   net.Conn
		err error
	}
	ch := make(chan acceptRes, 1)
	go func() {
		c, err := ln.Accept()
		ch <- acceptRes{c, err}
	}()
	var conn net.Conn
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return res.err
		}
		conn = res.c
	}
	defer conn.Close()
	wsc, err := ws.Handshake(conn)
	if err != nil {
		return err
	}
	defer wsc.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		// 不用 bufio.Scanner（有单行上限，超限会静默断流，TUI 卡死在 MCP 启动）；
		// ReadString 支持任意长度行，0.150.x 的 MCP 工具清单等大响应可达 MB 级。
		br := bufio.NewReaderSize(upOut, 128*1024)
		for {
			line, err := br.ReadString('\n')
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				if len(trimmed) > 1<<20 {
					fmt.Fprintf(os.Stderr, "[agent-auto-model] 上行长响应 %d 字节\n", len(trimmed))
				}
				rewrite.ObserveOutgoing([]byte(trimmed), st)
				if werr := wsc.WriteText(trimmed); werr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "[agent-auto-model] 读上游输出失败：%v\n", err)
				}
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			text, err := wsc.ReadText()
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(os.Stderr, "[agent-auto-model] 读 TUI 连接失败：%v\n", err)
				}
				return
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			out, d := rewrite.RewriteIncoming([]byte(text), st)
			logDecision(home, d)
			upMu.Lock()
			_, _ = upIn.Write(out)
			_, _ = upIn.Write([]byte("\n"))
			upMu.Unlock()
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
	case <-done:
	}
	return nil
}
