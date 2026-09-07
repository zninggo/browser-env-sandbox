package fpupdate

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// This file converts the apify/fingerprint-suite Bayesian network (network.json)
// into a joint "device profile" distribution that the fpengine consumes as
// data/device_profiles.json.
//
// network.json models a Bayesian network:
//
//	userAgent (root) → { videoCard, screen, fonts, hardwareConcurrency,
//	                     deviceMemory, platform, vendor, maxTouchPoints, ... }
//
// i.e. every fingerprint dimension is *conditioned on the User-Agent*. That
// conditioning is exactly the device-level joint distribution the engine needs:
// a macOS UA implies a Metal renderer, a Retina screen, Apple fonts and 8-10
// cores — because those are the observed frequencies of real macOS browsers.
//
// The legacy parser (parser.go) flattened these conditional tables into
// independent global lists, discarding the correlation. This file preserves it:
// it emits user_agents + ua_priors, and per-dimension UA-conditioned weighted
// categoricals, so the engine can sample a whole coherent device in one step.

// dpNodeNames are the child nodes we preserve for joint sampling. timezone /
// geo is not part of the network (fingerprint-generator injects locale
// separately), so it is handled by the existing KB timezone/language pairing.
var dpNodeNames = []string{
	"videoCard",
	"screen",
	"fonts",
	"hardwareConcurrency",
	"deviceMemory",
	"platform",
	"vendor",
	"maxTouchPoints",
}

// BuildDeviceProfileJSON is the exported entry point for generating the
// device-profile data file. maxUAs>0 limits the output to a representative
// subset (used for the embedded fallback).
func BuildDeviceProfileJSON(networkRaw []byte, npmVersion string, maxUAs int) ([]byte, error) {
	return parseNetworkToDeviceProfileJSON(networkRaw, npmVersion, maxUAs)
}

// parseNetworkToDeviceProfileJSON converts raw network.json into the device
// profile JSON. When maxUAs > 0, only up to maxUAs representative User-Agents
// (one per OS × browser combination, round-robin) are emitted — used to build
// the small embedded fallback.
func parseNetworkToDeviceProfileJSON(networkRaw []byte, npmVersion string, maxUAs int) ([]byte, error) {
	var network bayesianNetwork
	if err := json.Unmarshal(networkRaw, &network); err != nil {
		return nil, fmt.Errorf("unmarshal network: %w", err)
	}
	nodes := make(map[string]*networkNode, len(network.Nodes))
	for i := range network.Nodes {
		nodes[network.Nodes[i].Name] = &network.Nodes[i]
	}
	uaNode := nodes["userAgent"]
	if uaNode == nil {
		return nil, fmt.Errorf("network has no userAgent node")
	}

	// Root UA priors: conditionalProbabilities maps UA string → prior weight.
	uaStrings := make([]string, 0, len(uaNode.PossibleValues))
	uaPriors := make([]float64, 0, len(uaNode.PossibleValues))
	for _, raw := range uaNode.PossibleValues {
		var ua string
		if err := json.Unmarshal(raw, &ua); err != nil || ua == "" {
			continue
		}
		uaStrings = append(uaStrings, ua)
	}
	for _, ua := range uaStrings {
		uaPriors = append(uaPriors, uaNodeWeight(uaNode, ua))
	}

	// Subset selection (embedded fallback): keep a representative set.
	keep := representativeUAs(uaStrings, maxUAs)

	// Deduplicate font lists into a shared pool.
	fontPool, fontCond := buildFontPool(nodes["fonts"], keep)

	out := map[string]any{
		"version":         "1.0",
		"source":          "apify/fingerprint-suite",
		"source_version":  npmVersion,
		"source_url":      "https://www.npmjs.com/package/fingerprint-generator",
		"generated_at":    time.Now().UTC().Format(time.RFC3339),
		"user_agents":     keep,
		"ua_priors":       subsetPriors(uaStrings, uaPriors, keep),
		"video_card":      buildDPVideoCard(nodes["videoCard"], keep),
		"screen":          buildDPScreen(nodes["screen"], keep),
		"hardware_concurrency": buildDPInts(nodes["hardwareConcurrency"], keep),
		"device_memory":        buildDPInts(nodes["deviceMemory"], keep),
		"max_touch_points":     buildDPInts(nodes["maxTouchPoints"], keep),
		"platform":             buildDPStrings(nodes["platform"], keep),
		"vendor":               buildDPStrings(nodes["vendor"], keep),
		"fonts": map[string]any{
			"pool": fontPool,
			"cond": fontCond,
		},
	}
	return json.Marshal(out)
}

// uaNodeWeight returns the prior weight for a UA (root node's conditional
// probabilities), defaulting to 0 when absent.
func uaNodeWeight(uaNode *networkNode, ua string) float64 {
	cp := uaNode.ConditionalProbabilities
	if len(cp) == 0 {
		return 0
	}
	var prior map[string]float64
	if err := json.Unmarshal(cp, &prior); err != nil {
		return 0
	}
	return prior[ua]
}

// childDist returns the UA-conditioned distribution {valueKey: prob} for a
// child node, or nil.
func childDist(node *networkNode, ua string) map[string]float64 {
	if node == nil || len(node.ConditionalProbabilities) == 0 {
		return nil
	}
	var wrapper struct {
		Deeper map[string]map[string]float64 `json:"deeper"`
	}
	if err := json.Unmarshal(node.ConditionalProbabilities, &wrapper); err != nil {
		return nil
	}
	return wrapper.Deeper[ua]
}

// representativeUAs returns the UAs to keep: all of them when maxUAs<=0,
// otherwise a coverage-maximizing subset (round-robin across OS×browser).
func representativeUAs(uaStrings []string, maxUAs int) []string {
	if maxUAs <= 0 || len(uaStrings) <= maxUAs {
		return uaStrings
	}
	// Bucket by (browser, os), keeping only the engine-supported combos.
	type combo struct{ browser, os string }
	buckets := map[combo][]string{}
	var order []combo
	for _, ua := range uaStrings {
		b, o := uaBrowserClass(ua), uaOSClass(ua)
		if !(b == "chrome" || b == "firefox" || b == "safari") {
			continue
		}
		if !(o == "windows" || o == "macos" || o == "linux" || o == "android") {
			continue
		}
		c := combo{b, o}
		if _, ok := buckets[c]; !ok {
			order = append(order, c)
		}
		buckets[c] = append(buckets[c], ua)
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].browser != order[j].browser {
			return order[i].browser < order[j].browser
		}
		return order[i].os < order[j].os
	})

	var keep []string
	for len(keep) < maxUAs {
		progressed := false
		for _, c := range order {
			if len(buckets[c]) == 0 {
				continue
			}
			keep = append(keep, buckets[c][0])
			buckets[c] = buckets[c][1:]
			progressed = true
			if len(keep) >= maxUAs {
				break
			}
		}
		if !progressed {
			break
		}
	}
	return keep
}

func subsetPriors(all []string, priors []float64, keep []string) []float64 {
	idx := map[string]float64{}
	for i, ua := range all {
		if i < len(priors) {
			idx[ua] = priors[i]
		}
	}
	out := make([]float64, len(keep))
	for i, ua := range keep {
		out[i] = idx[ua]
	}
	return out
}

func buildDPVideoCard(node *networkNode, keep []string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, ua := range keep {
		dist := childDist(node, ua)
		if dist == nil {
			continue
		}
		var entries []map[string]any
		for val, p := range dist {
			m := unwrapMap(val)
			if m == nil {
				continue
			}
			renderer, _ := m["renderer"].(string)
			vendor, _ := m["vendor"].(string)
			if renderer == "" && vendor == "" {
				continue
			}
			entries = append(entries, map[string]any{"renderer": renderer, "vendor": vendor, "p": p})
		}
		out[ua] = entries
	}
	return out
}

func buildDPScreen(node *networkNode, keep []string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, ua := range keep {
		dist := childDist(node, ua)
		if dist == nil {
			continue
		}
		var entries []map[string]any
		for val, p := range dist {
			m := unwrapMap(val)
			if m == nil {
				continue
			}
			w := safeInt(m, "width")
			if w == 0 {
				continue
			}
			entries = append(entries, map[string]any{
				"width":              w,
				"height":             safeInt(m, "height"),
				"avail_width":        safeIntOr(m, "availWidth", w),
				"avail_height":       safeIntOr(m, "availHeight", safeInt(m, "height")),
				"color_depth":        safeIntOr(m, "colorDepth", 24),
				"pixel_depth":        safeIntOr(m, "pixelDepth", safeIntOr(m, "colorDepth", 24)),
				"device_pixel_ratio": safeFloat(m, "devicePixelRatio", 1.0),
				"outer_width":        safeIntOr(m, "outerWidth", w),
				"outer_height":       safeIntOr(m, "outerHeight", safeInt(m, "height")),
				"p":                  p,
			})
		}
		out[ua] = entries
	}
	return out
}

func buildDPInts(node *networkNode, keep []string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, ua := range keep {
		dist := childDist(node, ua)
		if dist == nil {
			continue
		}
		var entries []map[string]any
		for val, p := range dist {
			n := unwrapInt(val)
			if n <= 0 {
				continue
			}
			entries = append(entries, map[string]any{"v": n, "p": p})
		}
		if len(entries) > 0 {
			out[ua] = entries
		}
	}
	return out
}

func buildDPStrings(node *networkNode, keep []string) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for _, ua := range keep {
		dist := childDist(node, ua)
		if dist == nil {
			continue
		}
		var entries []map[string]any
		for val, p := range dist {
			s := unwrapString(val)
			if s == "" {
				continue
			}
			entries = append(entries, map[string]any{"v": s, "p": p})
		}
		if len(entries) > 0 {
			out[ua] = entries
		}
	}
	return out
}

// buildFontPool deduplicates font lists into a shared pool and emits the
// UA-conditioned weighted index table.
func buildFontPool(node *networkNode, keep []string) ([][]string, map[string][]map[string]any) {
	pool := [][]string{}
	index := map[string]int{}
	cond := map[string][]map[string]any{}

	for _, ua := range keep {
		dist := childDist(node, ua)
		if dist == nil {
			continue
		}
		var entries []map[string]any
		for val, p := range dist {
			fonts := unwrapStringSlice(val)
			if len(fonts) == 0 {
				continue
			}
			key := fontKey(fonts)
			i, ok := index[key]
			if !ok {
				i = len(pool)
				pool = append(pool, fonts)
				index[key] = i
			}
			entries = append(entries, map[string]any{"i": i, "p": p})
		}
		if len(entries) > 0 {
			cond[ua] = entries
		}
	}
	return pool, cond
}

// unwrapMap parses a (possibly *STRINGIFIED*-prefixed) map value.
func unwrapMap(v string) map[string]any {
	u := unwrapStringifiedJSON(v)
	m, _ := u.(map[string]any)
	return m
}

func unwrapInt(v string) int {
	u := unwrapStringifiedJSON(v)
	switch n := u.(type) {
	case float64:
		return int(n)
	case string:
		var i int
		fmt.Sscanf(n, "%d", &i)
		return i
	}
	return 0
}

func unwrapString(v string) string {
	u := unwrapStringifiedJSON(v)
	if s, ok := u.(string); ok {
		return s
	}
	return ""
}

func unwrapStringSlice(v string) []string {
	u := unwrapStringifiedJSON(v)
	arr, ok := u.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// unwrapStringifiedJSON strips a leading "*STRINGIFIED*" prefix and parses the
// remainder as JSON. Returns the raw string (as a string) when unparseable.
func unwrapStringifiedJSON(s string) any {
	if len(s) > 13 && s[:13] == "*STRINGIFIED*" {
		var inner any
		if json.Unmarshal([]byte(s[13:]), &inner) == nil {
			return inner
		}
	}
	return s
}

func fontKey(fonts []string) string {
	b, _ := json.Marshal(fonts)
	return string(b)
}

// uaBrowserClass / uaOSClass mirror fpengine's UA classification so the
// representative subset is chosen over the same OS × browser space the engine
// samples from.
func uaBrowserClass(ua string) string {
	switch {
	case containsStrFold(ua, "firefox/"):
		return "firefox"
	case containsStrFold(ua, "chrome/"):
		return "chrome"
	case containsStrFold(ua, "version/") && containsStrFold(ua, "safari"):
		return "safari"
	case containsStrFold(ua, "edg/"):
		return "edge"
	default:
		return "other"
	}
}

func uaOSClass(ua string) string {
	switch {
	case containsStrFold(ua, "android"):
		return "android"
	case containsStrFold(ua, "iphone") || containsStrFold(ua, "ipad"):
		return "ios"
	case containsStrFold(ua, "mac os x") || containsStrFold(ua, "macintosh"):
		return "macos"
	case containsStrFold(ua, "windows nt") || containsStrFold(ua, "windows"):
		return "windows"
	case containsStrFold(ua, "cros"):
		return "chromeos"
	case containsStrFold(ua, "linux"):
		return "linux"
	default:
		return "other"
	}
}

func containsStrFold(s, sub string) bool {
	// ASCII-only case-fold is sufficient for UA tokens.
	return indexOfFold(s, sub) >= 0
}

func indexOfFold(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	if len(s) < len(sub) {
		return -1
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}