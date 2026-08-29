package bes

import (
	"net"
	"os"
	"testing"
	"time"
)

// probe 仅为探测 bes-server 是否在监听,不做语义校验。
func probe(addr string) (bool, error) {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

// TestCallSmoke 走真实 bes-server 验证 Call 成功路径。
// 前置:bes-server 已在 BES_TEST_ADDR(默认 localhost:18080)监听且无 auth。
//   BES_TEST_ADDR=127.0.0.1:18080 go test -v ./sdk/go/ -run TestCallSmoke
//
// 覆盖契约:SDK 发送 {"function":...,"args":[...]}(见 bes.go:77),
// bridge callRequest.Function(json tag "function")匹配,
// /call 返回 200 且 callResponse.Result 为函数返回值。
func TestCallSmoke(t *testing.T) {
	addr := os.Getenv("BES_TEST_ADDR")
	if addr == "" {
		addr = "localhost:18080"
	}
	if ok, _ := probe(addr); !ok {
		t.Skipf("bes-server not reachable at %s; start it first (bes-server --port 18080)", addr)
	}

	s := New(addr)
	if err := s.CreateSession(SessionOptions{}); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	defer s.Close()

	// 定义一个全局函数供 Call 调用。
	if _, err := s.Eval(`function add(a,b){return String(Number(a)+Number(b))}`); err != nil {
		t.Fatalf("Eval define add failed: %v", err)
	}

	got, err := s.Call("add", "1", "2")
	if err != nil {
		t.Fatalf("Call failed: %v (contract broken? SDK must send 'function' not 'function_name')", err)
	}
	const want = "3"
	if got != want {
		t.Fatalf("Call result = %q, want %q", got, want)
	}
	t.Logf("Call(add,1,2) = %q ✓", got)
}
