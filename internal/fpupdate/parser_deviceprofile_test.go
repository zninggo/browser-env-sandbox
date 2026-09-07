package fpupdate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestParseNetworkToDeviceProfileJSON_PreservesJointDistribution verifies the
// parser converts a Bayesian network into the joint device-profile format and,
// critically, preserves the UA-conditioned correlation rather than flattening
// it into independent lists (the bug the legacy parser had).
func TestParseNetworkToDeviceProfileJSON_PreservesJointDistribution(t *testing.T) {
	winUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	macUA := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

	network := bayesianNetwork{Nodes: []networkNode{
		{
			Name:            "userAgent",
			PossibleValues:  []json.RawMessage{json.RawMessage(`"` + winUA + `"`), json.RawMessage(`"` + macUA + `"`)},
			ConditionalProbabilities: json.RawMessage(`{"` + winUA + `":0.6,"` + macUA + `":0.4}`),
		},
		{
			Name:                     "videoCard",
			ConditionalProbabilities: json.RawMessage(`{"deeper":{"` + winUA + `":{"*STRINGIFIED*{\"renderer\":\"ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Direct3D11 vs_5_0 ps_5_0)\",\"vendor\":\"Google Inc. (NVIDIA)\"}":1.0},"` + macUA + `":{"*STRINGIFIED*{\"renderer\":\"ANGLE (Apple, ANGLE Metal Renderer: Apple M1, Unspecified Version)\",\"vendor\":\"Google Inc. (Apple)\"}":1.0}}}`),
		},
		{
			Name:                     "platform",
			ConditionalProbabilities: json.RawMessage(`{"deeper":{"` + winUA + `":{"Win32":1.0},"` + macUA + `":{"MacIntel":1.0}}}`),
		},
		{
			Name:                     "hardwareConcurrency",
			ConditionalProbabilities: json.RawMessage(`{"deeper":{"` + winUA + `":{"*STRINGIFIED*8":1.0},"` + macUA + `":{"*STRINGIFIED*10":1.0}}}`),
		},
	}}

	raw, err := json.Marshal(network)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := BuildDeviceProfileJSON(raw, "2.1.88", 0)
	if err != nil {
		t.Fatalf("BuildDeviceProfileJSON: %v", err)
	}

	var got struct {
		UserAgents []string `json:"user_agents"`
		UAPriors   []float64 `json:"ua_priors"`
		VideoCard  map[string][]struct {
			Renderer string `json:"renderer"`
			Vendor   string `json:"vendor"`
		} `json:"video_card"`
		Platform map[string][]struct {
			V string `json:"v"`
		} `json:"platform"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(got.UserAgents) != 2 || len(got.UAPriors) != 2 {
		t.Fatalf("expected 2 user_agents/priors, got %d/%d", len(got.UserAgents), len(got.UAPriors))
	}
	if got.UAPriors[0] != 0.6 || got.UAPriors[1] != 0.4 {
		t.Fatalf("priors = %v, want [0.6 0.4]", got.UAPriors)
	}

	// Correlation preserved: the Windows UA must map to a Direct3D11 GPU and
	// the macOS UA to a Metal GPU.
	winGPU := got.VideoCard[winUA]
	if len(winGPU) == 0 || !strings.Contains(winGPU[0].Renderer, "Direct3D11") {
		t.Fatalf("win UA videoCard = %+v, want Direct3D11 renderer", winGPU)
	}
	macGPU := got.VideoCard[macUA]
	if len(macGPU) == 0 || !strings.Contains(macGPU[0].Renderer, "Metal") {
		t.Fatalf("mac UA videoCard = %+v, want Metal renderer", macGPU)
	}

	// Platform correlation preserved.
	if got.Platform[winUA][0].V != "Win32" || got.Platform[macUA][0].V != "MacIntel" {
		t.Fatalf("platform conditionals = %+v, want Win32 / MacIntel", got.Platform)
	}
}

func TestRepresentativeUAs_CoversCombinations(t *testing.T) {
	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/147 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/148 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/147 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 14; K) Chrome/147 Mobile Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7) Version/26.3 Safari/604.1", // iOS → dropped
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/147 Edg/147 Safari/537.36", // Edge → dropped
	}
	keep := representativeUAs(uas, 3)
	if len(keep) != 3 {
		t.Fatalf("representativeUAs kept %d, want 3", len(keep))
	}
	// iOS and Edge must never be selected (outside supported families).
	for _, ua := range keep {
		if strings.Contains(ua, "iPhone") || strings.Contains(ua, "Edg/") {
			t.Fatalf("unsupported UA %q selected", ua)
		}
	}
}