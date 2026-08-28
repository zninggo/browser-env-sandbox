package netlayer

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	utls "github.com/refraction-networking/utls"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// Chrome HTTP/2 fingerprint constants.
// Captured from real Chromium 151 via tls.peet.ws
// (akamai_fingerprint: 1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p,
//  hash 52d84b11737d980aef856699f885ca86).
const (
	// chromeH2HeaderTableSize is SETTINGS_HEADER_TABLE_SIZE (0x1).
	chromeH2HeaderTableSize = 65536
	// chromeH2InitialWindowSize is SETTINGS_INITIAL_WINDOW_SIZE (0x4), 6 MB.
	chromeH2InitialWindowSize = 6291456
	// chromeH2MaxHeaderListSize is SETTINGS_MAX_HEADER_LIST_SIZE (0x6).
	chromeH2MaxHeaderListSize = 262144
	// chromeH2ConnWindowUpdate is the WINDOW_UPDATE increment sent on stream 0
	// (connection-level flow control). Chrome sends 15663105.
	chromeH2ConnWindowUpdate = 15663105
	// chromeH2PriorityWeight is the zero-indexed weight for the HEADERS frame
	// priority field. Real Chrome uses weight=256, stored as 255 in the frame
	// (spec: "add one to obtain a weight between 1 and 256").
	chromeH2PriorityWeight = 255
)

// requestH2 sends an HTTP request over a manually-constructed HTTP/2
// connection with Chrome-precise frame fingerprinting.
//
// Unlike http2.Transport (which hardcodes SETTINGS values, pseudo-header
// order, and omits PRIORITY), this function controls every byte:
//   - SETTINGS: 4 entries in Chrome's exact order (HEADER_TABLE_SIZE,
//     ENABLE_PUSH=0, INITIAL_WINDOW_SIZE=6291456, MAX_HEADER_LIST_SIZE)
//   - WINDOW_UPDATE: connection-level increment 15663105
//   - HEADERS: Priority flag (exclusive, weight=256), pseudo-headers in
//     Chrome's order (:method, :authority, :scheme, :path)
//
// The TLS layer uses utls (Chrome ClientHello) via dialUTLS.
func (c *UTLSClient) requestH2(ctx context.Context, method, reqURL string, headers map[string]string, body []byte) (*Response, error) {
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// 1. Dial TLS with utls (Chrome ClientHello, ALPN negotiates h2).
	conn, err := c.dialUTLS(ctx, host, port)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 2. Verify ALPN negotiated h2.
	tlsConn, ok := conn.(*utls.UConn)
	if !ok {
		return nil, fmt.Errorf("dialUTLS returned non-utls connection: %T", conn)
	}
	cs := tlsConn.ConnectionState()
	if cs.NegotiatedProtocol != "h2" {
		return nil, fmt.Errorf("ALPN did not negotiate h2 (got %q)", cs.NegotiatedProtocol)
	}

	// 3. Set deadline.
	if c.timeout > 0 {
		conn.SetDeadline(time.Now().Add(c.timeout))
	}

	// 4. Coalesce all outgoing frames (preface + SETTINGS + WINDOW_UPDATE +
	// HEADERS) into a single buffer, then write them in one conn.Write() call.
	// Chrome sends these in 1-2 TLS records; Go's Framer writes each frame as
	// a separate TLS record by default, which server-side fingerprinting can detect.
	var outBuf bytes.Buffer

	// 4a. Client preface (magic string).
	outBuf.WriteString(http2.ClientPreface)

	// 4b. Create framer that writes to our buffer (not the connection directly).
	framer := http2.NewFramer(&outBuf, conn)

	// 4c. Send SETTINGS (Chrome's 4 settings, in Chrome's exact order).
	if err := framer.WriteSettings(
		http2.Setting{ID: http2.SettingHeaderTableSize, Val: chromeH2HeaderTableSize},
		http2.Setting{ID: http2.SettingEnablePush, Val: 0},
		http2.Setting{ID: http2.SettingInitialWindowSize, Val: chromeH2InitialWindowSize},
		http2.Setting{ID: http2.SettingMaxHeaderListSize, Val: chromeH2MaxHeaderListSize},
	); err != nil {
		return nil, fmt.Errorf("write settings: %w", err)
	}

	// 4d. Send WINDOW_UPDATE (connection-level, stream 0).
	if err := framer.WriteWindowUpdate(0, chromeH2ConnWindowUpdate); err != nil {
		return nil, fmt.Errorf("write window update: %w", err)
	}

	// 4e. Encode headers with hpack (pseudo-headers first in Chrome order).
	path := parsedURL.Path
	if path == "" {
		path = "/"
	}
	if parsedURL.RawQuery != "" {
		path += "?" + parsedURL.RawQuery
	}

	var hdrBuf bytes.Buffer
	enc := hpack.NewEncoder(&hdrBuf)
	// Pseudo-headers in Chrome order: :method, :authority, :scheme, :path.
	enc.WriteField(hpack.HeaderField{Name: ":method", Value: strings.ToUpper(method)})
	enc.WriteField(hpack.HeaderField{Name: ":authority", Value: parsedURL.Host})
	enc.WriteField(hpack.HeaderField{Name: ":scheme", Value: parsedURL.Scheme})
	enc.WriteField(hpack.HeaderField{Name: ":path", Value: path})

	// Regular headers in Chrome's canonical order.
	for _, h := range orderHeadersChrome(headers) {
		enc.WriteField(hpack.HeaderField{Name: strings.ToLower(h[0]), Value: h[1]})
	}

	// 4f. Send HEADERS frame (stream 1, with Priority, EndStream if no body).
	endStream := len(body) == 0
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: hdrBuf.Bytes(),
		EndStream:     endStream,
		EndHeaders:    true,
		Priority: http2.PriorityParam{
			StreamDep: 0,
			Exclusive: true,
			Weight:    chromeH2PriorityWeight,
		},
	}); err != nil {
		return nil, fmt.Errorf("write headers: %w", err)
	}

	// 4g. Send DATA frame if request body exists (also to buffer).
	if len(body) > 0 {
		if err := framer.WriteData(1, true, body); err != nil {
			return nil, fmt.Errorf("write data: %w", err)
		}
	}

	// 5. Flush all coalesced frames to the TLS connection in a single Write,
	// so they go into one TLS record (matching Chrome's behavior).
	if _, err := conn.Write(outBuf.Bytes()); err != nil {
		return nil, fmt.Errorf("flush coalesced frames: %w", err)
	}

	// 6. Switch framer to read from the connection for response frames.
	// Create a new framer that reads from the connection.
	readFramer := http2.NewFramer(conn, conn)
	return readH2Response(readFramer)
}

// readH2Response reads HTTP/2 response frames from the framer until stream 1
// reaches EndStream. It handles server SETTINGS (sends ACK), PING (sends ACK),
// HEADERS/CONTINUATION (decodes response headers), and DATA (accumulates body).
func readH2Response(framer *http2.Framer) (*Response, error) {
	respHeaders := make(map[string]string)
	cookies := make(map[string]string)
	var setCookies []string
	var respBody bytes.Buffer
	var status int
	streamEnded := false

	// hpack decoder for response headers (max table size matches Chrome's 65536).
	hdec := hpack.NewDecoder(chromeH2HeaderTableSize, nil)

	// Accumulator for multi-frame header blocks (HEADERS + CONTINUATION).
	var hdrBlock bytes.Buffer
	hdrBlockActive := false

	for !streamEnded {
		frame, err := framer.ReadFrame()
		if err != nil {
			if err == io.EOF {
				break
			}
			if respBody.Len() > 0 || len(respHeaders) > 0 {
				break // return partial data
			}
			return nil, fmt.Errorf("read frame: %w", err)
		}

		switch f := frame.(type) {
		case *http2.SettingsFrame:
			if !f.IsAck() {
				if err := framer.WriteSettingsAck(); err != nil {
					return nil, fmt.Errorf("write settings ack: %w", err)
				}
			}

		case *http2.WindowUpdateFrame:
			// Flow control update from server; no action for single-request.

		case *http2.HeadersFrame:
			if f.StreamID == 1 {
				hdrBlock.Write(f.HeaderBlockFragment())
				if f.HeadersEnded() {
					if err := decodeH2Headers(hdrBlock.Bytes(), hdec, &respHeaders, &cookies, &setCookies, &status); err != nil {
						return nil, err
					}
					hdrBlock.Reset()
				} else {
					hdrBlockActive = true
				}
			}

		case *http2.ContinuationFrame:
			if f.StreamID == 1 && hdrBlockActive {
				hdrBlock.Write(f.HeaderBlockFragment())
				if f.HeadersEnded() {
					if err := decodeH2Headers(hdrBlock.Bytes(), hdec, &respHeaders, &cookies, &setCookies, &status); err != nil {
						return nil, err
					}
					hdrBlock.Reset()
					hdrBlockActive = false
				}
			}

		case *http2.DataFrame:
			if f.StreamID == 1 {
				respBody.Write(f.Data())
				if f.StreamEnded() {
					streamEnded = true
				}
			}

		case *http2.PingFrame:
			if !f.IsAck() {
				if err := framer.WritePing(true, f.Data); err != nil {
					return nil, fmt.Errorf("write ping ack: %w", err)
				}
			}

		case *http2.RSTStreamFrame:
			if f.StreamID == 1 {
				return nil, fmt.Errorf("RST_STREAM stream 1 (code %d)", f.ErrCode)
			}

		case *http2.GoAwayFrame:
			return nil, fmt.Errorf("GOAWAY (last stream %d, code %d)", f.LastStreamID, f.ErrCode)
		}
	}

	bodyStr := decompressH2Body(respHeaders["Content-Encoding"], respBody.Bytes())

	return &Response{
		Status:     status,
		Headers:    respHeaders,
		Body:       bodyStr,
		Cookies:    cookies,
		SetCookies: setCookies,
	}, nil
}

// decodeH2Headers decodes an hpack header block and populates response headers,
// cookies, and status code. The :status pseudo-header sets the status.
func decodeH2Headers(block []byte, hdec *hpack.Decoder, headers *map[string]string, cookies *map[string]string, setCookies *[]string, status *int) error {
	fields, err := hdec.DecodeFull(block)
	if err != nil {
		return fmt.Errorf("hpack decode: %w", err)
	}
	for _, hf := range fields {
		if hf.Name == ":status" {
			fmt.Sscanf(hf.Value, "%d", status)
			continue
		}
		if strings.HasPrefix(hf.Name, ":") {
			continue
		}
		(*headers)[http2CanonicalHeaderKey(hf.Name)] = hf.Value
		if strings.EqualFold(hf.Name, "set-cookie") {
			*setCookies = append(*setCookies, hf.Value)
			parts := strings.SplitN(hf.Value, ";", 2)
			if len(parts) > 0 {
				eqIdx := strings.Index(parts[0], "=")
				if eqIdx > 0 {
					(*cookies)[strings.TrimSpace(parts[0][:eqIdx])] = strings.TrimSpace(parts[0][eqIdx+1:])
				}
			}
		}
	}
	return nil
}

// decompressH2Body decompresses the response body based on Content-Encoding.
func decompressH2Body(encoding string, data []byte) string {
	switch strings.ToLower(encoding) {
	case "gzip":
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return string(data)
		}
		defer gr.Close()
		dec, err := io.ReadAll(gr)
		if err != nil {
			return string(data)
		}
		return string(dec)
	case "br":
		dec, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
		if err != nil {
			return string(data)
		}
		return string(dec)
	default:
		return string(data)
	}
}

// chromeHeaderOrder defines the canonical header order that Chrome sends.
// Headers not in this list are appended after, sorted alphabetically.
var chromeHeaderOrder = []string{
	"upgrade-insecure-requests",
	"user-agent",
	"accept",
	"sec-fetch-site",
	"sec-fetch-mode",
	"sec-fetch-user",
	"sec-fetch-dest",
	"sec-ch-ua",
	"sec-ch-ua-mobile",
	"sec-ch-ua-platform",
	"accept-encoding",
	"accept-language",
	"priority",
	"content-type",
	"origin",
	"referer",
	"cookie",
	"authorization",
}

// orderHeadersChrome arranges headers in Chrome's canonical order, applying
// sensible defaults for common headers if not provided by the caller.
func orderHeadersChrome(headers map[string]string) [][2]string {
	provided := make(map[string]string)
	for k, v := range headers {
		provided[strings.ToLower(k)] = v
	}

	// Defaults matching Chrome 131.
	if _, ok := provided["user-agent"]; !ok {
		provided["user-agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	if _, ok := provided["accept"]; !ok {
		provided["accept"] = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
	}
	if _, ok := provided["accept-language"]; !ok {
		provided["accept-language"] = "zh-CN,zh;q=0.9"
	}
	if _, ok := provided["accept-encoding"]; !ok {
		provided["accept-encoding"] = "gzip, deflate, br, zstd"
	}

	var result [][2]string
	seen := make(map[string]bool)
	for _, name := range chromeHeaderOrder {
		if val, ok := provided[name]; ok {
			result = append(result, [2]string{name, val})
			seen[name] = true
		}
	}

	// Remaining headers (not in canonical list), sorted alphabetically.
	var remaining []string
	for k := range provided {
		if !seen[k] {
			remaining = append(remaining, k)
		}
	}
	sort.Strings(remaining)
	for _, name := range remaining {
		result = append(result, [2]string{name, provided[name]})
	}

	return result
}

// http2CanonicalHeaderKey converts a lowercased HTTP/2 header name to
// net/http's canonical form (e.g. "content-type" → "Content-Type").
func http2CanonicalHeaderKey(name string) string {
	// net/http.CanonicalHeaderKey does exactly this.
	var buf bytes.Buffer
	upper := true
	for _, r := range name {
		if upper && r >= 'a' && r <= 'z' {
			r -= 32
		}
		upper = r == '-'
		buf.WriteRune(r)
	}
	return buf.String()
}
