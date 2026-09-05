package netlayer

import (
	"strconv"
	"strings"

	utls "github.com/refraction-networking/utls"
)

// This file is the data-driven "UA → Chrome version → JA3/JA4/HTTP2" mapping.
//
// The invariant the whole netlayer package depends on: the TLS ClientHello
// (JA3/JA4), the HTTP/2 frame fingerprint, and the User-Agent must all resolve
// to the SAME Chrome version. Nothing here is random — every field is a table
// lookup whose provenance is traceable to the sources named next to it.

// h2Fingerprint is the akamai HTTP/2 fingerprint: the SETTINGS frame entries
// (in Chrome's exact order), the connection-level WINDOW_UPDATE increment, and
// the HEADERS PRIORITY weight. Together they produce akamai's signature
//
//	1:65536;2:0;4:6291456;6:262144|15663105|0|m,a,s,p
//
// Captured from real Chromium 151 via tls.peet.ws. These values have been
// stable across Chrome 57→current (Chrome has not changed its HTTP/2 preface
// in that window), so a single default profile covers every Chrome in our
// ladder; it is kept as a struct rather than bare constants so a future Chrome
// change is a one-line table edit, not a hunt through the framing code.
type h2Fingerprint struct {
	// headerTableSize is SETTINGS_HEADER_TABLE_SIZE (0x1).
	headerTableSize uint32
	// initialWindowSize is SETTINGS_INITIAL_WINDOW_SIZE (0x4), 6 MiB.
	initialWindowSize uint32
	// maxHeaderListSize is SETTINGS_MAX_HEADER_LIST_SIZE (0x6).
	maxHeaderListSize uint32
	// connWindowUpdate is the WINDOW_UPDATE increment on stream 0.
	connWindowUpdate uint32
	// priorityWeight is the zero-indexed HEADERS priority weight (255 → 256).
	priorityWeight uint8
}

// chromeH2Default is Chrome's canonical HTTP/2 preface fingerprint.
var chromeH2Default = h2Fingerprint{
	headerTableSize:   65536,
	initialWindowSize: 6291456,
	maxHeaderListSize: 262144,
	connWindowUpdate:  15663105,
	priorityWeight:    255,
}

// pqSigAlgMinChromeVersion is the first Chrome major that prepends the
// post-quantum signature schemes (ML-DSA-65/87, SLH-DSA) to
// signature_algorithms. It is a separate axis from the ClientHello preset:
// those sig schemes are defined in tls_spec.go and injected post-build in
// dialUTLS, because utls v1.8.2 (whose Chrome presets stop at 133) predates
// Chrome 140's PQ rollout.
const pqSigAlgMinChromeVersion = 140

// chromeFingerprintProfile binds one Chrome release line to its TLS (JA3/JA4)
// and HTTP/2 fingerprints.
type chromeFingerprintProfile struct {
	// minMajor is the first Chrome major that shipped this ClientHello.
	minMajor int
	// hello is the utls ClientHelloID whose ClientHelloSpec reproduces the
	// exact ClientHello Chrome sent at that release: cipher-suite order,
	// extension order, supported curves, key shares, signature algorithms,
	// and ALPN. This spec fully determines JA3 (MD5 over suites/extensions/
	// curves/points) and JA4 (hash over the same material plus ALPN and the
	// signature-algorithm set), so choosing the right preset is what makes the
	// JA3/JA4 match the UA — no random generation anywhere.
	hello utls.ClientHelloID
	// pqSigAlgs is set by chromeProfileFor (not the table) for Chrome 140+.
	pqSigAlgs bool
	// h2 is the HTTP/2 frame fingerprint for this release line.
	h2 h2Fingerprint
}

// chromeFingerprintLadder is the fingerprint mapping table. It is an ordered
// slice keyed by the lowest Chrome major to introduce a given ClientHello;
// Chrome only changes its ClientHello at these specific releases, so every
// version between two entries reuses the nearest lower entry (the "rollback"
// convention of JA3/JA4 fingerprint databases such as ja3er / tlsfingerprint).
//
// Provenance: each entry's ClientHelloSpec is defined in
// refraction-networking/utls v1.8.2, u_parrots.go, in the `case HelloChrome_NNN`
// blocks. The version→preset pairing below restates those cases against the
// public Chrome release history (which release first used each ClientHello):
//
//	58  TLS 1.2 max, pre-TLS1.3 cipher order          → HelloChrome_58
//	62  (identical spec to 58 in utls)                → HelloChrome_62
//	70  first Default TLS 1.3 ClientHello            → HelloChrome_70
//	72  cipher/extension reorder                      → HelloChrome_72
//	83  ALPS + new cert-compression codepoints        → HelloChrome_83
//	87  GREASE position in cipher list changes        → HelloChrome_87
//	96  cipher reorder                                → HelloChrome_96
//	100 dropped TLS 1.0/1.1 in SupportedVersions      → HelloChrome_100
//	102 signature-algorith/extension shuffle          → HelloChrome_102
//	106 GREASE extension shuffling (HelloChrome_106_Shuffle) → 106
//	115 X25519Kyber768 post-quantum key share         → HelloChrome_115_PQ
//	120 Kyber768 + GREASE EncryptedClientHello        → HelloChrome_120_PQ
//	131 X25519MLKEM768 (Kyber→ML-KEM switch)          → HelloChrome_131
//	133 new ALPS codepoint                            → HelloChrome_133
var chromeFingerprintLadder = []chromeFingerprintProfile{
	{minMajor: 58, hello: utls.HelloChrome_58, h2: chromeH2Default},           // 58–61
	{minMajor: 62, hello: utls.HelloChrome_62, h2: chromeH2Default},           // 62–69
	{minMajor: 70, hello: utls.HelloChrome_70, h2: chromeH2Default},           // 70–71
	{minMajor: 72, hello: utls.HelloChrome_72, h2: chromeH2Default},           // 72–82
	{minMajor: 83, hello: utls.HelloChrome_83, h2: chromeH2Default},           // 83–86
	{minMajor: 87, hello: utls.HelloChrome_87, h2: chromeH2Default},           // 87–95
	{minMajor: 96, hello: utls.HelloChrome_96, h2: chromeH2Default},           // 96–99
	{minMajor: 100, hello: utls.HelloChrome_100, h2: chromeH2Default},         // 100–101
	{minMajor: 102, hello: utls.HelloChrome_102, h2: chromeH2Default},         // 102–105
	{minMajor: 106, hello: utls.HelloChrome_106_Shuffle, h2: chromeH2Default}, // 106–114
	{minMajor: 115, hello: utls.HelloChrome_115_PQ, h2: chromeH2Default},      // 115–119
	{minMajor: 120, hello: utls.HelloChrome_120_PQ, h2: chromeH2Default},      // 120–130
	{minMajor: 131, hello: utls.HelloChrome_131, h2: chromeH2Default},         // 131–132
	{minMajor: 133, hello: utls.HelloChrome_133, h2: chromeH2Default},         // 133+ (PQ sig algs join at 140)
}

// chromeProfileFor resolves a Chrome major version to its fingerprint profile
// using the rollback rule above. Versions below the first entry clamp up to it
// rather than fail; versions above the last entry inherit the newest preset
// (Chrome's ClientHello has been stable at 133 apart from the PQ sig algs,
// which pqSigAlgs tracks independently at 140+).
func chromeProfileFor(major int) chromeFingerprintProfile {
	first := chromeFingerprintLadder[0].minMajor
	if major < first {
		major = first
	}
	p := chromeFingerprintLadder[0]
	for i := range chromeFingerprintLadder {
		if major >= chromeFingerprintLadder[i].minMajor {
			p = chromeFingerprintLadder[i]
		} else {
			break
		}
	}
	p.pqSigAlgs = major >= pqSigAlgMinChromeVersion
	return p
}

// chromeVersionFromUA extracts the Chrome major version from a User-Agent
// string. Chrome desktop/mobile UAs all embed "Chrome/<major>.<minor>...";
// only the major is needed because the fingerprint ladder keys on majors.
// Returns (0, false) when no Chrome version token is present (e.g. a Firefox
// or Safari UA — those browsers have their own fingerprints, out of scope here).
func chromeVersionFromUA(ua string) (int, bool) {
	const marker = "Chrome/"
	idx := strings.Index(ua, marker)
	if idx < 0 {
		return 0, false
	}
	rest := ua[idx+len(marker):]
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		rest = rest[:dot]
	}
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}

// chromeFingerprintForUA resolves a User-Agent string to its fingerprint
// profile — the "UA → Chrome version → JA3/JA4/HTTP2 fingerprint" binding the
// acceptance criteria require. It is the single lookup entry point: whoever
// holds a UA gets back the exact ClientHello preset, PQ-sig-alg flag, and
// HTTP/2 frames that UA's Chrome version would send.
func chromeFingerprintForUA(ua string) (chromeFingerprintProfile, bool) {
	major, ok := chromeVersionFromUA(ua)
	if !ok {
		return chromeFingerprintProfile{}, false
	}
	return chromeProfileFor(major), true
}
