package browser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// MobileDevice represents a mobile device profile with all emulation parameters
type MobileDevice struct {
	Name             string  `json:"name"`
	UserAgent        string  `json:"userAgent"`
	Viewport         Viewport `json:"viewport"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor"`
	IsMobile         bool    `json:"isMobile"`
	HasTouch         bool    `json:"hasTouch"`
	DefaultOrientation string `json:"defaultOrientation"`
}

// Viewport represents device viewport settings
type Viewport struct {
	Width             int  `json:"width"`
	Height            int  `json:"height"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor"`
	IsMobile          bool `json:"isMobile"`
	HasTouch          bool `json:"hasTouch"`
	IsLandscape       bool `json:"isLandscape"`
}

// NetworkProfile represents mobile network conditions
type NetworkProfile struct {
	Name              string  `json:"name"`
	DownloadThroughput float64 `json:"downloadThroughput"` // bytes/sec
	UploadThroughput   float64 `json:"uploadThroughput"`   // bytes/sec
	Latency           float64  `json:"latency"`            // milliseconds
	PacketLoss        float64  `json:"packetLoss"`         // percentage (0-100)
	Offline           bool     `json:"offline"`
}

// predefinedDevices contains popular mobile device profiles
var predefinedDevices = map[string]*MobileDevice{
	// Apple iOS Devices
	"iPhone 14 Pro Max": {
		Name:      "iPhone 14 Pro Max",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             430,
			Height:            932,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone 14 Pro": {
		Name:      "iPhone 14 Pro",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             393,
			Height:            852,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone 14": {
		Name:      "iPhone 14",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             390,
			Height:            844,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone 13": {
		Name:      "iPhone 13",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             390,
			Height:            844,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone 12": {
		Name:      "iPhone 12",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             390,
			Height:            844,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone SE (3rd generation)": {
		Name:      "iPhone SE (3rd generation)",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 15_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.4 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             375,
			Height:            667,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPhone 8": {
		Name:      "iPhone 8",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 13_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             375,
			Height:            667,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	// iPad Devices
	"iPad Pro (12.9-inch)": {
		Name:      "iPad Pro (12.9-inch)",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             1024,
			Height:            1366,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPad Pro (11-inch)": {
		Name:      "iPad Pro (11-inch)",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             834,
			Height:            1194,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPad Air": {
		Name:      "iPad Air",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 15_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             820,
			Height:            1180,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPad": {
		Name:      "iPad",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 15_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             810,
			Height:            1080,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"iPad Mini": {
		Name:      "iPad Mini",
		UserAgent: "Mozilla/5.0 (iPad; CPU OS 15_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1",
		Viewport: Viewport{
			Width:             744,
			Height:            1133,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	// Android Devices
	"Samsung Galaxy S23 Ultra": {
		Name:      "Samsung Galaxy S23 Ultra",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             384,
			Height:            824,
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3.5,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Samsung Galaxy S22": {
		Name:      "Samsung Galaxy S22",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; SM-S901B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             360,
			Height:            780,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Samsung Galaxy S21": {
		Name:      "Samsung Galaxy S21",
		UserAgent: "Mozilla/5.0 (Linux; Android 12; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             360,
			Height:            800,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Samsung Galaxy A54": {
		Name:      "Samsung Galaxy A54",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; SM-A546B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             384,
			Height:            854,
			DeviceScaleFactor: 2.75,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2.75,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Google Pixel 7 Pro": {
		Name:      "Google Pixel 7 Pro",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             412,
			Height:            892,
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3.5,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Google Pixel 7": {
		Name:      "Google Pixel 7",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             412,
			Height:            892,
			DeviceScaleFactor: 2.6,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2.6,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Google Pixel 6": {
		Name:      "Google Pixel 6",
		UserAgent: "Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             412,
			Height:            892,
			DeviceScaleFactor: 2.6,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2.6,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Google Pixel 5": {
		Name:      "Google Pixel 5",
		UserAgent: "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             393,
			Height:            851,
			DeviceScaleFactor: 2.75,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2.75,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	// Android Tablets
	"Samsung Galaxy Tab S9": {
		Name:      "Samsung Galaxy Tab S9",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		Viewport: Viewport{
			Width:             753,
			Height:            1205,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Samsung Galaxy Tab S8": {
		Name:      "Samsung Galaxy Tab S8",
		UserAgent: "Mozilla/5.0 (Linux; Android 12; SM-X700) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36",
		Viewport: Viewport{
			Width:             753,
			Height:            1205,
			DeviceScaleFactor: 2,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 2,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	// Other Popular Devices
	"OnePlus 11": {
		Name:      "OnePlus 11",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; PHB110) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             412,
			Height:            892,
			DeviceScaleFactor: 3.5,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3.5,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
	"Xiaomi Mi 13": {
		Name:      "Xiaomi Mi 13",
		UserAgent: "Mozilla/5.0 (Linux; Android 13; 2211133C) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36",
		Viewport: Viewport{
			Width:             390,
			Height:            844,
			DeviceScaleFactor: 3,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: 3,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	},
}

// predefinedNetworkProfiles contains common mobile network conditions
var predefinedNetworkProfiles = map[string]*NetworkProfile{
	"offline": {
		Name:              "Offline",
		DownloadThroughput: 0,
		UploadThroughput:   0,
		Latency:           0,
		PacketLoss:        100,
		Offline:           true,
	},
	"slow-3g": {
		Name:              "Slow 3G",
		DownloadThroughput: 50 * 1024,   // 50 KB/s
		UploadThroughput:   25 * 1024,   // 25 KB/s
		Latency:           400,          // 400ms
		PacketLoss:        2,            // 2%
		Offline:           false,
	},
	"fast-3g": {
		Name:              "Fast 3G",
		DownloadThroughput: 200 * 1024,  // 200 KB/s
		UploadThroughput:   100 * 1024,  // 100 KB/s
		Latency:           150,          // 150ms
		PacketLoss:        1,            // 1%
		Offline:           false,
	},
	"4g": {
		Name:              "4G",
		DownloadThroughput: 1.5 * 1024 * 1024, // 1.5 MB/s
		UploadThroughput:   750 * 1024,        // 750 KB/s
		Latency:           50,                 // 50ms
		PacketLoss:        0.5,                // 0.5%
		Offline:           false,
	},
	"5g": {
		Name:              "5G",
		DownloadThroughput: 10 * 1024 * 1024,  // 10 MB/s
		UploadThroughput:   5 * 1024 * 1024,   // 5 MB/s
		Latency:           20,                 // 20ms
		PacketLoss:        0.1,                // 0.1%
		Offline:           false,
	},
	"edge": {
		Name:              "EDGE",
		DownloadThroughput: 30 * 1024,   // 30 KB/s
		UploadThroughput:   15 * 1024,   // 15 KB/s
		Latency:           500,          // 500ms
		PacketLoss:        3,            // 3%
		Offline:           false,
	},
	"wifi": {
		Name:              "WiFi",
		DownloadThroughput: 30 * 1024 * 1024,  // 30 MB/s
		UploadThroughput:   15 * 1024 * 1024,  // 15 MB/s
		Latency:           5,                  // 5ms
		PacketLoss:        0,                  // 0%
		Offline:           false,
	},
}

// GetPredefinedDevice returns a predefined mobile device profile
func GetPredefinedDevice(name string) (*MobileDevice, error) {
	// Case-insensitive search
	for deviceName, device := range predefinedDevices {
		if strings.EqualFold(deviceName, name) {
			// Return a copy to prevent modification
			deviceCopy := *device
			return &deviceCopy, nil
		}
	}
	
	// Try partial match
	nameLower := strings.ToLower(name)
	for deviceName, device := range predefinedDevices {
		if strings.Contains(strings.ToLower(deviceName), nameLower) {
			// Return a copy to prevent modification
			deviceCopy := *device
			return &deviceCopy, nil
		}
	}
	
	return nil, fmt.Errorf("device '%s' not found", name)
}

// ListDevices returns a sorted list of all available device names
func ListDevices() []string {
	devices := make([]string, 0, len(predefinedDevices))
	for name := range predefinedDevices {
		devices = append(devices, name)
	}
	sort.Strings(devices)
	return devices
}

// GetNetworkProfile returns a predefined network profile
func GetNetworkProfile(name string) (*NetworkProfile, error) {
	// Case-insensitive search
	for profileName, profile := range predefinedNetworkProfiles {
		if strings.EqualFold(profileName, name) {
			// Return a copy to prevent modification
			profileCopy := *profile
			return &profileCopy, nil
		}
	}
	
	return nil, fmt.Errorf("network profile '%s' not found", name)
}

// ListNetworkProfiles returns a sorted list of all available network profile names
func ListNetworkProfiles() []string {
	profiles := make([]string, 0, len(predefinedNetworkProfiles))
	for name := range predefinedNetworkProfiles {
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)
	return profiles
}

// CreateCustomDevice creates a custom mobile device profile
func CreateCustomDevice(name string, width, height int, scaleFactor float64, userAgent string) *MobileDevice {
	if userAgent == "" {
		// Default mobile Chrome user agent
		userAgent = fmt.Sprintf("Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36 (Custom %dx%d)", width, height)
	}
	
	return &MobileDevice{
		Name:      name,
		UserAgent: userAgent,
		Viewport: Viewport{
			Width:             width,
			Height:            height,
			DeviceScaleFactor: scaleFactor,
			IsMobile:          true,
			HasTouch:          true,
		},
		DeviceScaleFactor: scaleFactor,
		IsMobile:         true,
		HasTouch:         true,
		DefaultOrientation: "portrait",
	}
}

// CreateCustomNetworkProfile creates a custom network profile
func CreateCustomNetworkProfile(name string, downloadMbps, uploadMbps, latencyMs, packetLossPercent float64) *NetworkProfile {
	return &NetworkProfile{
		Name:              name,
		DownloadThroughput: downloadMbps * 1024 * 1024 / 8, // Convert Mbps to bytes/sec
		UploadThroughput:   uploadMbps * 1024 * 1024 / 8,   // Convert Mbps to bytes/sec
		Latency:           latencyMs,
		PacketLoss:        packetLossPercent,
		Offline:           false,
	}
}

// SetOrientation updates the device viewport for landscape/portrait orientation
func (d *MobileDevice) SetOrientation(landscape bool) {
	if landscape {
		// Swap width and height for landscape
		d.Viewport.Width, d.Viewport.Height = d.Viewport.Height, d.Viewport.Width
		d.Viewport.IsLandscape = true
	} else {
		// Ensure portrait orientation
		if d.Viewport.Width > d.Viewport.Height {
			d.Viewport.Width, d.Viewport.Height = d.Viewport.Height, d.Viewport.Width
		}
		d.Viewport.IsLandscape = false
	}
}

// ToJSON converts the device profile to JSON
func (d *MobileDevice) ToJSON() (string, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON creates a device profile from JSON
func DeviceFromJSON(jsonStr string) (*MobileDevice, error) {
	var device MobileDevice
	if err := json.Unmarshal([]byte(jsonStr), &device); err != nil {
		return nil, err
	}
	return &device, nil
}