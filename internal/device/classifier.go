package device

import "strings"

// DeviceClass represents the classification tier of a requesting device.
type DeviceClass string

const (
	MobileLow  DeviceClass = "mobile-low"
	MobileHigh DeviceClass = "mobile-high"
	Tablet     DeviceClass = "tablet"
	Desktop    DeviceClass = "desktop"
	Unknown    DeviceClass = "unknown"
)

// DeviceProfile holds the optimization parameters for a device class.
type DeviceProfile struct {
	Class           DeviceClass
	MaxWidth        int
	Quality         int
	PreferredFormat string
}

var profiles = map[DeviceClass]DeviceProfile{
	MobileLow:  {Class: MobileLow, MaxWidth: 480, Quality: 60, PreferredFormat: "webp"},
	MobileHigh: {Class: MobileHigh, MaxWidth: 768, Quality: 75, PreferredFormat: "webp"},
	Tablet:     {Class: Tablet, MaxWidth: 1024, Quality: 80, PreferredFormat: "webp"},
	Desktop:    {Class: Desktop, MaxWidth: 1920, Quality: 85, PreferredFormat: "webp"},
	Unknown:    {Class: Unknown, MaxWidth: 1024, Quality: 75, PreferredFormat: "jpeg"},
}

// Classify parses the User-Agent string and returns the appropriate DeviceProfile.
func Classify(userAgent string) DeviceProfile {
	ua := strings.ToLower(userAgent)

	if ua == "" {
		return profiles[Unknown]
	}

	// Bots and CLI tools → unknown
	if isBotOrCLI(ua) {
		return profiles[Unknown]
	}

	// Tablets: iPad, or Android with "tablet" keyword
	if strings.Contains(ua, "ipad") {
		return profiles[Tablet]
	}
	if strings.Contains(ua, "android") && strings.Contains(ua, "tablet") {
		return profiles[Tablet]
	}

	// Mobile devices
	if strings.Contains(ua, "iphone") {
		return profiles[MobileHigh]
	}
	if strings.Contains(ua, "android") && strings.Contains(ua, "mobile") {
		if isLowEndAndroid(ua) {
			return profiles[MobileLow]
		}
		return profiles[MobileHigh]
	}

	// Generic mobile indicators
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "phone") {
		return profiles[MobileLow]
	}

	// Standard browser UAs → desktop
	if isBrowser(ua) {
		return profiles[Desktop]
	}

	return profiles[Unknown]
}

// NegotiateFormat checks the Accept header for WebP support and returns the best format.
func NegotiateFormat(acceptHeader string, preferred string) string {
	if strings.Contains(acceptHeader, "image/webp") && preferred == "webp" {
		return "webp"
	}
	return "jpeg"
}

func isBotOrCLI(ua string) bool {
	botIndicators := []string{"bot", "crawl", "spider", "curl", "wget", "httpie", "postman"}
	for _, indicator := range botIndicators {
		if strings.Contains(ua, indicator) {
			return true
		}
	}
	return false
}

func isLowEndAndroid(ua string) bool {
	// Android versions 5.x and below are considered low-end
	lowVersions := []string{"android 4.", "android 5.", "android 3.", "android 2.", "android 1."}
	for _, v := range lowVersions {
		if strings.Contains(ua, v) {
			return true
		}
	}
	return false
}

func isBrowser(ua string) bool {
	browserIndicators := []string{"mozilla", "chrome", "safari", "firefox", "edge", "opera", "msie", "trident"}
	for _, indicator := range browserIndicators {
		if strings.Contains(ua, indicator) {
			return true
		}
	}
	return false
}
