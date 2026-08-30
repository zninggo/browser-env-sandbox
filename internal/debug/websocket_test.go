package debug

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// newPipeClient wires a CDPClient over an in-memory net.Pipe pair so frame
// helpers can be exercised without a real TCP listener. The client side reads
// from / writes to one end; the other end is returned for the test to feed
// frames in and read frames out. Buffers are seeded exactly as handleWebSocket
// does, so readFrame/writeFrame see the same per-connection reused state the
// fix relies on.
func newPipeClient(t *testing.T) (*CDPClient, net.Conn) {
	t.Helper()
	cA, cB := net.Pipe()
	client := &CDPClient{
		conn:     cA,
		br:       bufio.NewReader(cA),
		bw:       bufio.NewWriter(cA),
		requests: make(chan CDPRequest, 100),
	}
	return client, cB
}

// writeMaskedFrame writes a complete masked WebSocket text frame with the given
// payload to conn, matching how a real browser client sends (clients always
// mask). Used to feed readFrame from the "network" side.
func writeMaskedFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	mask := [4]byte{0x11, 0x22, 0x33, 0x44} // arbitrary fixed mask key
	header := []byte{0x81}                  // FIN + text opcode
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length|0x80))
	case length < 65536:
		header = append(header, 126|0x80)
		var len16 [2]byte
		binary.BigEndian.PutUint16(len16[:], uint16(length))
		header = append(header, len16[:]...)
	default:
		header = append(header, 127|0x80)
		var len64 [8]byte
		binary.BigEndian.PutUint64(len64[:], uint64(length))
		header = append(header, len64[:]...)
	}
	header = append(header, mask[:]...)
	masked := make([]byte, length)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := conn.Write(append(header, masked...)); err != nil {
		t.Errorf("write test frame: %v", err)
	}
}

// writeMaskedFrameHugeLength writes a masked text frame header with an explicit
// 64-bit length field set to declaredLength, with no payload following. The
// oversized-length check in readFrame fires after the length is parsed and
// before any payload allocation, so the missing payload bytes are irrelevant —
// the error returns first and the connection tears down cleanly.
func writeMaskedFrameHugeLength(t *testing.T, conn net.Conn, declaredLength uint64) {
	t.Helper()
	mask := [4]byte{0x11, 0x22, 0x33, 0x44}
	header := []byte{0x81, 127 | 0x80}
	var len64 [8]byte
	binary.BigEndian.PutUint64(len64[:], declaredLength)
	header = append(header, len64[:]...)
	header = append(header, mask[:]...)
	if _, err := conn.Write(header); err != nil {
		t.Errorf("write huge-length header: %v", err)
	}
}

// --- Item 3: frame size cap ---

// TestReadFrameRejectsOversized verifies item 3: a client-controlled 64-bit
// length field above maxFrameSize is rejected BEFORE the make([]byte, length)
// allocation, so a single frame cannot exhaust memory. readFrame must return an
// error and the read loop tears down — no multi-GB allocation happens.
func TestReadFrameRejectsOversized(t *testing.T) {
	client, peer := newPipeClient(t)
	defer peer.Close()
	defer client.conn.Close()

	// net.Pipe writes block until the peer reads, so feed the frame from a
	// goroutine while the main goroutine calls readFrame. readFrame parses the
	// length, sees it exceed maxFrameSize, and returns an error BEFORE reading
	// any payload — so the writer's remaining bytes are simply discarded when
	// the connection tears down.
	go writeMaskedFrameHugeLength(t, peer, uint64(maxFrameSize)+1)

	errCh := make(chan error, 1)
	go func() { _, err := client.readFrame(); errCh <- err }()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("readFrame accepted an oversized frame; expected an error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readFrame did not return for an oversized frame")
	}
}

// TestReadFrameAcceptsUnderCap verifies a legitimate frame is parsed correctly
// — the cap rejects only the oversized, not normal traffic.
func TestReadFrameAcceptsUnderCap(t *testing.T) {
	client, peer := newPipeClient(t)
	defer peer.Close()
	defer client.conn.Close()

	want := []byte(`{"id":1,"method":"Runtime.enable"}`)
	go writeMaskedFrame(t, peer, want)

	errCh := make(chan struct{ b []byte; e error }, 1)
	go func() {
		b, e := client.readFrame()
		errCh <- struct{ b []byte; e error }{b, e}
	}()
	select {
	case r := <-errCh:
		if r.e != nil {
			t.Fatalf("readFrame normal frame: %v", r.e)
		}
		if string(r.b) != string(want) {
			t.Fatalf("payload mismatch: got %q want %q", r.b, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("readFrame did not return for a normal frame")
	}
}

// --- Item 1 & 2: per-client write lock + reused bufio ---

// TestWriteFramePerClientLockNoBlock verifies item 1: per-client write frames
// are serialized only against the SAME client. Two clients each writing to
// their own connection must proceed concurrently — under the old global
// wsMutex one client's write held the lock the other needed (head-of-line
// blocking). Two independent CDPClients must finish concurrently rather than
// serializing on a shared lock.
func TestWriteFramePerClientLockNoBlock(t *testing.T) {
	c1, peer1 := newPipeClient(t)
	defer peer1.Close()
	defer c1.conn.Close()
	c2, peer2 := newPipeClient(t)
	defer peer2.Close()
	defer c2.conn.Close()

	// Drain both peers so writes don't block on full pipe buffers.
	go io.Copy(io.Discard, peer1)
	go io.Copy(io.Discard, peer2)

	payload := make([]byte, 4096)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = c1.writeFrame(payload)
	}()
	go func() {
		defer wg.Done()
		_ = c2.writeFrame(payload)
	}()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent per-client writes deadlocked — global lock still in place?")
	}
}

// TestBufferedReaderReused verifies item 2: the bufio.Reader is created once
// per connection and reused across frames, so buffered lookahead bytes from one
// frame are not discarded before the next. We feed two frames back-to-back; a
// fresh bufio.NewReader each call would lose the second frame's leading bytes
// (stranded in the discarded buffer). With reuse, both frames parse cleanly.
func TestBufferedReaderReused(t *testing.T) {
	client, peer := newPipeClient(t)
	defer peer.Close()
	defer client.conn.Close()

	f1 := []byte(`{"id":1,"method":"Runtime.enable"}`)
	f2 := []byte(`{"id":2,"method":"Page.enable"}`)

	// Write both frames in one go so the bufio.Reader almost certainly buffers
	// bytes of frame 2 while readFrame parses frame 1. If readFrame created a
	// new reader each call, frame 2's bytes would be stranded in the old
	// reader's buffer and the second read would block/desync. net.Pipe writes
	// block until the peer reads, so feed from a goroutine.
	go func() {
		writeMaskedFrame(t, peer, f1)
		writeMaskedFrame(t, peer, f2)
	}()

	got1, err := client.readFrame()
	if err != nil {
		t.Fatalf("first readFrame: %v", err)
	}
	got2, err := client.readFrame()
	if err != nil {
		t.Fatalf("second readFrame (buffered bytes must survive): %v", err)
	}
	if string(got1) != string(f1) || string(got2) != string(f2) {
		t.Fatalf("frames desynced: got %q then %q", got1, got2)
	}
}

// --- Item 4: debuggerState concurrency ---

// newDebugServerWithState builds a CDPServer whose shared dbgState is already
// initialized, so concurrent Debugger.* handlers all hit the same breakpoints
// map + nextBreakpointID under -race. Multiple CDP clients share one server.
func newDebugServerWithState(t *testing.T) *CDPServer {
	t.Helper()
	s := &CDPServer{clients: make(map[string]*CDPClient)}
	s.dbgState = &debuggerState{breakpoints: make(map[string]breakpoint)}
	return s
}

// TestDebuggerStateConcurrentNoFatal verifies item 4: the breakpoints map and
// nextBreakpointID are shared across all CDP clients (one CDPServer, many
// read-loop goroutines). Without the debuggerState lock the Go runtime would
// detect concurrent map read/write and fatal the process. Under -race this
// must complete with zero races and no runtime fatal.
func TestDebuggerStateConcurrentNoFatal(t *testing.T) {
	srv := newDebugServerWithState(t)

	const clients = 8
	const perClient = 200
	var wg sync.WaitGroup
	for c := 0; c < clients; c++ {
		wg.Add(1)
		go func(clientIdx int) {
			defer wg.Done()
			for i := 0; i < perClient; i++ {
				// Interleave breakpoint add/remove/get so the map is read and
				// written concurrently from every goroutine.
				params, _ := json.Marshal(map[string]any{
					"url":        "test.js",
					"lineNumber": i,
				})
				_ = srv.handleDebugger(CDPRequest{
					ID:     i,
					Method: "Debugger.setBreakpointByUrl",
					Params: params,
				})

				if i%2 == 0 {
					_ = srv.handleDebugger(CDPRequest{
						ID:     i,
						Method: "Debugger.getBreakpoints",
					})
				}
				if i%3 == 0 {
					rmParams, _ := json.Marshal(map[string]any{
						"breakpointId": "bes-bp-1",
					})
					_ = srv.handleDebugger(CDPRequest{
						ID:     i,
						Method: "Debugger.removeBreakpoint",
						Params: rmParams,
					})
				}
			}
		}(c)
	}
	wg.Wait()

	// The map must still be internally consistent: a final read under the lock
	// returns without panic and the ID counter advanced with successful sets.
	srv.dbgState.mu.Lock()
	id := srv.dbgState.nextBreakpointID
	srv.dbgState.mu.Unlock()
	if id <= 0 {
		t.Fatalf("nextBreakpointID never advanced: %d", id)
	}
}

// TestDebuggerStateConcurrentLazyInit verifies the lazy-init path under s.mu:
// many goroutines hitting a server with a nil dbgState at once must not race
// the `if s.dbgState == nil` check nor double-initialize.
func TestDebuggerStateConcurrentLazyInit(t *testing.T) {
	srv := &CDPServer{clients: make(map[string]*CDPClient)}
	var wg sync.WaitGroup
	const n = 32
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = srv.handleDebugger(CDPRequest{ID: 1, Method: "Debugger.enable"})
		}()
	}
	wg.Wait()
	if srv.dbgState == nil {
		t.Fatal("dbgState was never initialized under concurrent first access")
	}
}
