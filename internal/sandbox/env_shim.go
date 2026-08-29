package sandbox

import _ "embed"

//go:embed env_shim_part1.js
var shimPart1 string

//go:embed env_shim_part2.js
var shimPart2 string

//go:embed env_shim_part3.js
var shimPart3 string

//go:embed env_shim_part4.js
var shimPart4 string

//go:embed env_shim_part5.js
var shimPart5 string

// envShimParts returns each shim file as a separate string.
// Running them individually prevents a single syntax error from
// blocking all environment enhancements.
func envShimParts() []string {
	return []string{shimPart1, shimPart2, shimPart3, shimPart4, shimPart5}
}
