package netlayer

import utls "github.com/refraction-networking/utls"

// Chrome post-quantum signature schemes added in Chrome ~140+.
// These appear BEFORE the classic algorithms in the signature_algorithms
// extension, and are required to match the JA4 fingerprint of modern Chrome.
// Captured from real Chromium 151 via tls.peet.ws.
const (
	// sigSchemeMLDSA65SHA256 is ML-DSA-65 with SHA-256 (post-quantum).
	sigSchemeMLDSA65SHA256 utls.SignatureScheme = 0x0904
	// sigSchemeMLDSA87SHA512 is ML-DSA-87 with SHA-512 (post-quantum).
	sigSchemeMLDSA87SHA512 utls.SignatureScheme = 0x0905
	// sigSchemeSLHDSA192sSHA256 is SLH-DSA-SHA2-192s with SHA-256 (post-quantum).
	sigSchemeSLHDSA192sSHA256 utls.SignatureScheme = 0x0906
)
