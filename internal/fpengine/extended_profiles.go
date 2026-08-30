// Package fpengine — browser-specific navigator adjustments.
//
// buildNavigator constructs a Chrome-flavored navigator by default. Real
// Firefox/Safari expose different vendor strings, omit the UA Client Hints
// (userAgentData) API entirely, and report different productSub values; an
// Android Chrome session must advertise a mobile UA-CH payload. Without these
// adjustments, browser=firefox/safari still produced a Chrome-smelling
// navigator (vendor "Google Inc.", userAgentData present) that any server-side
// cross-check would flag as inconsistent with the UA string.
//
// Each adjustment is applied as a patch over the base Chrome navigator so the
// shared fields (UA, platform, languages, hardwareConcurrency, deviceMemory)
// stay consistent with the rest of the fingerprint.
package fpengine

// NavigatorAdjustments patches a base (Chrome) navigator map to match a
// specific browser/OS. Fields left at their zero value are not applied.
type NavigatorAdjustments struct {
	// applyVendor is true when Vendor should be written (lets Firefox set
	// vendor to "" without clobbering Chrome/Safari's non-empty vendor).
	applyVendor bool
	// Vendor overrides navigator.vendor. Firefox → "" (empty), Safari →
	// "Apple Computer, Inc.". Chrome keeps "Google Inc." (applyVendor false).
	Vendor string
	// applyProductSub is true when ProductSub should be written.
	applyProductSub bool
	// ProductSub overrides navigator.productSub. Firefox → "20100101",
	// Safari/Chrome → "20030107".
	ProductSub string
	// applyProduct is true when Product should be written.
	applyProduct bool
	// Product overrides navigator.product. Firefox/Safari → "Gecko".
	Product string
	// DropUserAgentData removes the userAgentData key entirely. Firefox and
	// Safari do not implement the UA Client Hints API; its presence is a
	// Chrome-only signal.
	DropUserAgentData bool
	// MobileUAHints marks the UA-CH payload as mobile (Android Chrome).
	MobileUAHints bool
	// hasMaxTouchPoints is true when MaxTouchPoints was explicitly set.
	hasMaxTouchPoints bool
	// MaxTouchPoints overrides navigator.maxTouchPoints (Android → 5).
	MaxTouchPoints int
}

// firefoxAdjustments returns the navigator patches that make a fingerprint
// smell like Firefox: empty vendor, no UA-CH, Gecko productSub.
func firefoxAdjustments() NavigatorAdjustments {
	return NavigatorAdjustments{
		applyVendor:       true,
		Vendor:            "",
		applyProductSub:   true,
		ProductSub:        "20100101",
		applyProduct:      true,
		Product:           "Gecko",
		DropUserAgentData: true,
	}
}

// safariAdjustments returns the navigator patches that make a fingerprint
// smell like Safari: Apple vendor, no UA-CH.
func safariAdjustments() NavigatorAdjustments {
	return NavigatorAdjustments{
		applyVendor:       true,
		Vendor:            "Apple Computer, Inc.",
		applyProductSub:   true,
		ProductSub:        "20030107",
		applyProduct:      true,
		Product:           "Gecko",
		DropUserAgentData: true,
	}
}

// androidChromeAdjustments returns the navigator patches for an Android
// Chrome session: mobile UA-CH payload and touch points.
func androidChromeAdjustments() NavigatorAdjustments {
	return NavigatorAdjustments{
		MobileUAHints:     true,
		hasMaxTouchPoints: true,
		MaxTouchPoints:    5,
	}
}

// adjustmentsForBrowser selects the navigator adjustments for a browser/OS
// combination. Returns nil (no patching) for desktop Chrome, which is the
// default flavor buildNavigator already produces.
func adjustmentsForBrowser(browser, osName string) *NavigatorAdjustments {
	switch {
	case browser == "firefox":
		a := firefoxAdjustments()
		return &a
	case browser == "safari":
		a := safariAdjustments()
		return &a
	case osName == "android":
		// Android runs Chrome (mobile); Firefox/Safari handled above.
		a := androidChromeAdjustments()
		return &a
	default:
		return nil
	}
}

// applyNavigatorAdjustments patches a base navigator map in place. No-op when
// adj is nil (desktop Chrome keeps its default flavor).
func applyNavigatorAdjustments(nav map[string]any, adj *NavigatorAdjustments) {
	if adj == nil {
		return
	}
	if adj.applyVendor {
		nav["vendor"] = adj.Vendor
	}
	if adj.applyProductSub {
		nav["productSub"] = adj.ProductSub
	}
	if adj.applyProduct {
		nav["product"] = adj.Product
	}
	if adj.DropUserAgentData {
		delete(nav, "userAgentData")
	}
	if adj.MobileUAHints {
		if uad, ok := nav["userAgentData"].(map[string]any); ok {
			uad["mobile"] = true
			nav["userAgentData"] = uad
		}
	}
	if adj.hasMaxTouchPoints {
		nav["maxTouchPoints"] = adj.MaxTouchPoints
	}
}
