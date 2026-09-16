package executor

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestValidateLocalWSURL(t *testing.T) {
	ok := []string{
		"ws://127.0.0.1:9222/devtools/page/abc",
		"ws://localhost:9222/devtools/browser/x",
		"ws://[::1]:9222/devtools/page/abc",
	}
	for _, u := range ok {
		if _, err := validateLocalWSURL(u); err != nil {
			t.Errorf("validateLocalWSURL(%q) = %v, want nil", u, err)
		}
	}
	bad := []string{
		"ws://203.0.113.10:9222/devtools/page/abc", // remote host
		"ws://chrome.example.com:9222/devtools",    // remote name
		"wss://127.0.0.1:9222/devtools/page/abc",   // non-ws scheme
		"http://127.0.0.1:9222/json/list",          // wrong scheme
		"::not a url::",
	}
	for _, u := range bad {
		if _, err := validateLocalWSURL(u); err == nil {
			t.Errorf("validateLocalWSURL(%q) = nil error, want rejection", u)
		}
	}
}

func TestValidateLocalHTTPURL(t *testing.T) {
	if _, err := validateLocalHTTPURL("http://127.0.0.1:9222/json/list"); err != nil {
		t.Errorf("local http url rejected: %v", err)
	}
	for _, u := range []string{
		"http://0.0.0.0:9222/json/list",          // not an explicit local host
		"http://127.0.0.1/json/list",             // no port
		"https://127.0.0.1:9222/json/list",       // wrong scheme
		"http://example.com:9222/json/list",      // remote
	} {
		if _, err := validateLocalHTTPURL(u); err == nil {
			t.Errorf("validateLocalHTTPURL(%q) = nil error, want rejection", u)
		}
	}
}

// TestCDPFrameRoundTrip exercises writeFrame's client masking over an
// in-memory pipe. net.Pipe is synchronous, so a reader goroutine must be
// draining the server end while writeFrame runs; the reader collects the raw
// bytes and the assertions decode them afterwards.
func TestCDPFrameRoundTrip(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	client := &cdpConn{raw: c1}

	payload := []byte(`{"id":1,"method":"Runtime.evaluate","params":{"expression":"1+1"}}`)

	type result struct {
		data []byte
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		// header (2) + mask (4) + body (len)
		buf := make([]byte, 2+4+len(payload))
		_, err := io.ReadFull(c2, buf)
		resCh <- result{buf, err}
	}()

	if err := client.writeFrame(wsOpText, payload); err != nil {
		t.Fatal(err)
	}

	res := <-resCh
	if res.err != nil {
		t.Fatal(res.err)
	}
	buf := res.data
	hdr, mask, body := buf[:2], buf[2:6], buf[6:]
	if hdr[1]&0x80 == 0 {
		t.Error("client frame is not masked")
	}
	if hdr[0]&0x0F != wsOpText {
		t.Errorf("opcode = %d, want text", hdr[0]&0x0F)
	}
	if hdr[0]&0x80 == 0 {
		t.Error("FIN bit not set")
	}
	length := uint64(hdr[1] & 0x7F)
	if length != uint64(len(payload)) {
		t.Fatalf("frame length = %d, want %d", length, len(payload))
	}
	for i := range body {
		body[i] ^= mask[i%4]
	}
	if string(body) != string(payload) {
		t.Error("unmasked body mismatch")
	}
}

// writeFrameUnmaskedForTest writes a server-style (unmasked) frame to the
// given conn end for readFrame to consume.
func (c *cdpConn) writeFrameUnmaskedForTest(op byte, payload []byte) error {
	var hdr [10]byte
	n := 0
	hdr[0] = 0x80 | op
	switch {
	case len(payload) < 126:
		hdr[1] = byte(len(payload))
		n = 2
	case len(payload) <= 0xFFFF:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		n = 4
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(len(payload)))
		n = 10
	}
	if _, err := c.raw.Write(hdr[:n]); err != nil {
		return err
	}
	_, err := c.raw.Write(payload)
	return err
}

// TestCDPFrameFragmentReassembly drives the read loop with a fragmented
// message and verifies dispatch reassembles it into one JSON response.
func TestCDPFrameFragmentReassembly(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	sender := &cdpConn{raw: c2}
	// Client conn reads from c1.
	client := &cdpConn{raw: c1, br: bufio.NewReader(c1), pending: make(map[int64]chan cdpReply)}
	go client.readLoop()

	// Register a pending call, then send the response fragmented.
	ch := make(chan cdpReply, 1)
	client.mu.Lock()
	client.pending[7] = ch
	client.mu.Unlock()

	full := []byte(`{"id":7,"result":{"value":"ok"}}`)
	half := len(full) / 2
	if err := sender.writeFrameUnmaskedForTest(wsOpText, full[:half]); err != nil {
		t.Fatal(err)
	}
	// Continuation frame carries the rest.
	if err := sender.writeContFrameForTest(full[half:]); err != nil {
		t.Fatal(err)
	}

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("fragmented call failed: %v", r.err)
		}
		if string(r.result) != `{"value":"ok"}` {
			t.Errorf("result = %s", r.result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no response for fragmented message")
	}
}

func (c *cdpConn) writeContFrameForTest(payload []byte) error {
	// FIN + continuation opcode: this is the FINAL fragment.
	hdr := []byte{0x80, byte(len(payload))}
	if _, err := c.raw.Write(hdr); err != nil {
		return err
	}
	_, err := c.raw.Write(payload)
	return err
}
