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

// envShimJS returns the combined environment enhancement JavaScript.
// Injected post-context to add APIs that can't be set via v8go ObjectTemplate.
func envShimJS() string {
	return shimPart1 + "\n" + shimPart2 + "\n" + shimPart3 + "\n" + shimPart4
}
