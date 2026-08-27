package fpengine

import (
	"fmt"

	"github.com/zninggo/bes/pkg/api"
)

// WebGPUFingerprint holds WebGPU adapter info for fingerprint generation.
type WebGPUFingerprint struct {
	Vendor   string `json:"vendor"`   // e.g. "nvidia"
	Architecture string `json:"architecture"` // e.g. "ampere"
	Device   string `json:"device"`   // e.g. "NVIDIA GeForce RTX 4060"
	Description string `json:"description"`
}

// GenerateWebGPU generates a WebGPU fingerprint consistent with the GPU profile.
func GenerateWebGPU(gpu api.GPUProfile) WebGPUFingerprint {
	// Parse GPU vendor from WebGL renderer string
	vendor := "unknown"
	arch := "unknown"
	device := gpu.Renderer

	if contains(gpu.Vendor, "NVIDIA") {
		vendor = "nvidia"
		arch = guessNvidiaArch(gpu.Renderer)
	} else if contains(gpu.Vendor, "Intel") {
		vendor = "intel"
		arch = "gen12"
	} else if contains(gpu.Vendor, "AMD") {
		vendor = "amd"
		arch = "rdna2"
	} else if contains(gpu.Vendor, "Apple") {
		vendor = "apple"
		arch = "apple-gpu"
	}

	return WebGPUFingerprint{
		Vendor:       vendor,
		Architecture: arch,
		Device:       device,
		Description:  fmt.Sprintf("%s %s", vendor, arch),
	}
}

func guessNvidiaArch(renderer string) string {
	if contains(renderer, "4060") || contains(renderer, "4070") || contains(renderer, "4080") || contains(renderer, "4090") {
		return "ada-lovelace"
	}
	if contains(renderer, "3060") || contains(renderer, "3070") || contains(renderer, "3080") || contains(renderer, "3090") {
		return "ampere"
	}
	if contains(renderer, "2060") || contains(renderer, "2070") || contains(renderer, "2080") {
		return "turing"
	}
	return "unknown"
}

// ClientHints generates the complete set of Sec-CH-UA client hints
// for a given browser profile.
type ClientHints struct {
	SecCHUA               string `json:"sec_ch_ua"`                  // "Chromium";v="131", "Google Chrome";v="131", "Not.A/Brand";v="24"
	SecCHUAMobile         string `json:"sec_ch_ua_mobile"`           // "?0"
	SecCHUAPlatform       string `json:"sec_ch_ua_platform"`         // "Windows"
	SecCHUAFullVersionList string `json:"sec_ch_ua_full_version_list"` // full version with build numbers
	SecCHUAPlatformVersion string `json:"sec_ch_ua_platform_version"`  // "15.0.0"
	SecCHUAArch           string `json:"sec_ch_ua_arch"`             // "x86"
	SecCHUABitness        string `json:"sec_ch_ua_bitness"`          // "64"
	SecCHUAModel          string `json:"sec_ch_ua_model"`            // ""
	SecCHUAWOW64          string `json:"sec_ch_ua_wow64"`            // "?0"
}

// GenerateClientHints generates complete client hints for a browser + OS profile.
func GenerateClientHints(bp api.BrowserProfile, osp api.OSProfile) ClientHints {
	// Determine platform string
	platform := "Windows"
	platformVersion := "15.0.0"
	arch := "x86"
	bitness := "64"

	switch osp.Name {
	case "macos":
		platform = "macOS"
		platformVersion = "14.5.0"
		arch = "arm"
	case "linux":
		platform = "Linux"
		platformVersion = "6.6.0"
		arch = "x86"
	case "android":
		platform = "Android"
		platformVersion = "13.0.0"
		arch = "arm"
	}

	secCHUA := fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not.A/Brand";v="24"`, bp.Version, bp.Version)

	// Full version list includes build numbers (e.g. 131.0.6778.87)
	fullVersion := bp.Version + ".0.6778.87"
	secCHUAFullVersionList := fmt.Sprintf(`"Chromium";v="%s", "Google Chrome";v="%s", "Not.A/Brand";v="24.0.0.0"`, fullVersion, fullVersion)

	return ClientHints{
		SecCHUA:                secCHUA,
		SecCHUAMobile:          "?0",
		SecCHUAPlatform:        platform,
		SecCHUAFullVersionList: secCHUAFullVersionList,
		SecCHUAPlatformVersion: platformVersion,
		SecCHUAArch:            arch,
		SecCHUABitness:         bitness,
		SecCHUAModel:           "",
		SecCHUAWOW64:           "?0",
	}
}

// AsHeaders converts client hints to HTTP header format.
func (ch ClientHints) AsHeaders() map[string]string {
	return map[string]string{
		"sec-ch-ua":                 ch.SecCHUA,
		"sec-ch-ua-mobile":          ch.SecCHUAMobile,
		"sec-ch-ua-platform":        ch.SecCHUAPlatform,
		"sec-ch-ua-full-version-list": ch.SecCHUAFullVersionList,
		"sec-ch-ua-platform-version": ch.SecCHUAPlatformVersion,
		"sec-ch-ua-arch":            ch.SecCHUAArch,
		"sec-ch-ua-bitness":         ch.SecCHUABitness,
		"sec-ch-ua-model":           ch.SecCHUAModel,
		"sec-ch-ua-wow64":           ch.SecCHUAWOW64,
	}
}
