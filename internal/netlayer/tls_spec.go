package netlayer

import (
	"fmt"
	utls "github.com/refraction-networking/utls"
)

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

// buildChromeClientHelloSpec creates a ClientHelloSpec based on Chrome 133
// (which uses the new ALPS codepoint 17613) but with the post-quantum
// signature algorithms that modern Chrome (140+) includes.
//
// This is necessary because utls v1.8.2's HelloChrome_133 spec predates
// Chrome's addition of PQ signature schemes (0x0904-0x0906). Without them,
// the JA4 fingerprint's signature-algorithm hash differs from real Chrome.
func buildChromeClientHelloSpec() (utls.ClientHelloSpec, error) {
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_133)
	if err != nil {
		return utls.ClientHelloSpec{}, err
	}

	// Find the SignatureAlgorithmsExtension and prepend PQ schemes.
	found := false
	for i, ext := range spec.Extensions {
		fmt.Printf("[TLS_SPEC] ext[%d] type=%T\n", i, ext)
		if sigExt, ok := ext.(*utls.SignatureAlgorithmsExtension); ok {
			fmt.Printf("[TLS_SPEC] FOUND SignatureAlgorithmsExtension, current algs=%v\n", sigExt.SupportedSignatureAlgorithms)
			// Chrome 151 order: PQ schemes first, then classic schemes.
			sigExt.SupportedSignatureAlgorithms = append(
				[]utls.SignatureScheme{
					sigSchemeMLDSA65SHA256,
					sigSchemeMLDSA87SHA512,
					sigSchemeSLHDSA192sSHA256,
				},
				sigExt.SupportedSignatureAlgorithms...,
			)
			spec.Extensions[i] = sigExt
			found = true
			fmt.Printf("[TLS_SPEC] AFTER modify, algs=%v\n", sigExt.SupportedSignatureAlgorithms)
			break
		}
	}
	if !found {
		fmt.Println("[TLS_SPEC] WARNING: SignatureAlgorithmsExtension not found in spec!")
	}

	return spec, nil
}
