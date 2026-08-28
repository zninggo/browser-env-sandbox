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

// envShimJS returns the combined environment enhancement JavaScript.
// Injected post-context to add APIs that can't be set via v8go ObjectTemplate.
func envShimJS() string {
	return shimPart1 + "\n" + shimPart2 + "\n" + shimPart3 + "\n" + shimPart4 + "\n" + shimPart5
}

// envShimParts returns each shim file as a separate string.
// Running them individually prevents a single syntax error from
// blocking all environment enhancements.
func envShimParts() []string {
	return []string{shimPart1, shimPart2, shimPart3, shimPart4, shimPart5}
}
