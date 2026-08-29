package fpupdate

import (
	"encoding/json"
	"fmt"
	"time"
)

type networkNode struct {
	Name                     string            `json:"name"`
	ParentNames              []string          `json:"parentNames"`
	PossibleValues           []json.RawMessage `json:"possibleValues"`
	ConditionalProbabilities json.RawMessage   `json:"conditionalProbabilities"`
}

type bayesianNetwork struct {
	Nodes []networkNode `json:"nodes"`
}

func parseNetworkToBesJSON(networkRaw []byte, npmVersion string) ([]byte, error) {
	var network bayesianNetwork
	if err := json.Unmarshal(networkRaw, &network); err != nil {
		return nil, fmt.Errorf("unmarshal network: %w", err)
	}

	nodes := make(map[string]*networkNode)
	for i := range network.Nodes {
		nodes[network.Nodes[i].Name] = &network.Nodes[i]
	}

	gpus := extractGPUs(nodes["videoCard"])
	screens := extractScreens(nodes["screen"])
	fonts := extractFonts(nodes["fonts"])
	hwConc := extractInts(nodes["hardwareConcurrency"])
	devMem := extractDeviceMemory(nodes["deviceMemory"])
	chromeUAs := extractChromeUAs(nodes["userAgent"])

	output := map[string]any{
		"version":        "1.0",
		"source":         "apify/fingerprint-suite",
		"source_version": npmVersion,
		"source_url":     "https://www.npmjs.com/package/fingerprint-generator",
		"generated_at":   time.Now().UTC().Format(time.RFC3339),
		"stats": map[string]int{
			"gpus":                 len(gpus),
			"screens":              len(screens),
			"fonts":                len(fonts),
			"chrome_uas":           len(chromeUAs),
			"hardware_concurrency": len(hwConc),
			"device_memory":        len(devMem),
		},
		"gpus":                 gpus,
		"screens":              screens,
		"fonts":                fonts,
		"chrome_uas":           chromeUAs,
		"hardware_concurrency": hwConc,
		"device_memory":        devMem,
	}

	return json.Marshal(output)
}

func unwrapStringified(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	if s, ok := v.(string); ok && len(s) > 13 && s[:13] == "*STRINGIFIED*" {
		var inner any
		if json.Unmarshal([]byte(s[13:]), &inner) == nil {
			return inner
		}
	}
	return v
}

func extractGPUs(node *networkNode) []map[string]string {
	if node == nil {
		return nil
	}
	var result []map[string]string
	for _, raw := range node.PossibleValues {
		v := unwrapStringified(raw)
		if m, ok := v.(map[string]any); ok {
			renderer, _ := m["renderer"].(string)
			vendor, _ := m["vendor"].(string)
			if renderer != "" && vendor != "" {
				result = append(result, map[string]string{
					"vendor":   vendor,
					"renderer": renderer,
				})
			}
		}
	}
	return result
}

func extractScreens(node *networkNode) []map[string]any {
	if node == nil {
		return nil
	}
	var result []map[string]any
	for _, raw := range node.PossibleValues {
		v := unwrapStringified(raw)
		if m, ok := v.(map[string]any); ok {
			s := map[string]any{
				"width":              safeInt(m, "width"),
				"height":             safeInt(m, "height"),
				"avail_width":        safeIntOr(m, "availWidth", safeInt(m, "width")),
				"avail_height":       safeIntOr(m, "availHeight", safeInt(m, "height")),
				"color_depth":        safeIntOr(m, "colorDepth", 24),
				"device_pixel_ratio": safeFloat(m, "devicePixelRatio", 1.0),
				"inner_width":        safeIntOr(m, "innerWidth", safeInt(m, "width")),
				"inner_height":       safeIntOr(m, "innerHeight", safeInt(m, "height")),
				"outer_width":        safeIntOr(m, "outerWidth", safeInt(m, "width")),
				"outer_height":       safeIntOr(m, "outerHeight", safeInt(m, "height")),
			}
			result = append(result, s)
		}
	}
	return result
}

func extractFonts(node *networkNode) [][]string {
	if node == nil {
		return nil
	}
	var result [][]string
	for _, raw := range node.PossibleValues {
		v := unwrapStringified(raw)
		if arr, ok := v.([]any); ok {
			var fonts []string
			for _, f := range arr {
				if s, ok := f.(string); ok {
					fonts = append(fonts, s)
				}
			}
			if len(fonts) > 0 {
				result = append(result, fonts)
			}
		} else if m, ok := v.(map[string]any); ok {
			if arr, ok := m["fonts"].([]any); ok {
				var fonts []string
				for _, f := range arr {
					if s, ok := f.(string); ok {
						fonts = append(fonts, s)
					}
				}
				if len(fonts) > 0 {
					result = append(result, fonts)
				}
			}
		}
	}
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func extractInts(node *networkNode) []int {
	if node == nil {
		return nil
	}
	var result []int
	for _, raw := range node.PossibleValues {
		v := unwrapStringified(raw)
		switch n := v.(type) {
		case float64:
			if n >= 1 {
				result = append(result, int(n))
			}
		case string:
			var i int
			fmt.Sscanf(n, "%d", &i)
			if i >= 1 {
				result = append(result, i)
			}
		}
	}
	if len(result) == 0 {
		result = []int{8, 12, 16, 4}
	}
	return result
}

func extractDeviceMemory(node *networkNode) []int {
	if node == nil {
		return nil
	}
	var result []int
	for _, raw := range node.PossibleValues {
		v := unwrapStringified(raw)
		switch n := v.(type) {
		case float64:
			if n >= 1 {
				result = append(result, int(n))
			}
		case string:
			var i int
			fmt.Sscanf(n, "%d", &i)
			if i >= 1 {
				result = append(result, i)
			}
		}
	}
	if len(result) == 0 {
		result = []int{4, 8, 16, 32}
	}
	return result
}

func extractChromeUAs(node *networkNode) []string {
	if node == nil {
		return nil
	}
	var result []string
	for _, raw := range node.PossibleValues {
		var v any
		json.Unmarshal(raw, &v)
		if s, ok := v.(string); ok {
			if containsStr(s, "Chrome/") && !containsStr(s, "Edg/") && !containsStr(s, "OPR/") && !containsStr(s, "Brave") {
				result = append(result, s)
			}
		}
	}
	return result
}

func safeInt(m map[string]any, key string) int {
	v, _ := m[key].(float64)
	return int(v)
}

func safeIntOr(m map[string]any, key string, def int) int {
	v, ok := m[key].(float64)
	if !ok || v == 0 {
		return def
	}
	return int(v)
}

func safeFloat(m map[string]any, key string, def float64) float64 {
	v, ok := m[key].(float64)
	if !ok {
		return def
	}
	return v
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
