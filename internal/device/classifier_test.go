package device

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name         string
		userAgent    string
		wantClass    DeviceClass
		wantMaxWidth int
		wantQuality  int
		wantFormat   string
	}{
		{
			name:         "iPhone Safari",
			userAgent:    "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantClass:    MobileHigh,
			wantMaxWidth: 768,
			wantQuality:  75,
			wantFormat:   "webp",
		},
		{
			name:         "old Android 5.0",
			userAgent:    "Mozilla/5.0 (Linux; Android 5.0; SM-G900F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/45.0.2454.94 Mobile Safari/537.36",
			wantClass:    MobileLow,
			wantMaxWidth: 480,
			wantQuality:  60,
			wantFormat:   "webp",
		},
		{
			name:         "iPad",
			userAgent:    "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			wantClass:    Tablet,
			wantMaxWidth: 1024,
			wantQuality:  80,
			wantFormat:   "webp",
		},
		{
			name:         "Chrome on Windows desktop",
			userAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			wantClass:    Desktop,
			wantMaxWidth: 1920,
			wantQuality:  85,
			wantFormat:   "webp",
		},
		{
			name:         "curl",
			userAgent:    "curl/8.1.2",
			wantClass:    Unknown,
			wantMaxWidth: 1024,
			wantQuality:  75,
			wantFormat:   "jpeg",
		},
		{
			name:         "empty string",
			userAgent:    "",
			wantClass:    Unknown,
			wantMaxWidth: 1024,
			wantQuality:  75,
			wantFormat:   "jpeg",
		},
		{
			name:         "Android tablet",
			userAgent:    "Mozilla/5.0 (Linux; Android 13; Pixel C) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Tablet",
			wantClass:    Tablet,
			wantMaxWidth: 1024,
			wantQuality:  80,
			wantFormat:   "webp",
		},
		{
			name:         "modern Android phone",
			userAgent:    "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			wantClass:    MobileHigh,
			wantMaxWidth: 768,
			wantQuality:  75,
			wantFormat:   "webp",
		},
		{
			name:         "Googlebot",
			userAgent:    "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			wantClass:    Unknown,
			wantMaxWidth: 1024,
			wantQuality:  75,
			wantFormat:   "jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := Classify(tt.userAgent)
			if profile.Class != tt.wantClass {
				t.Errorf("Class = %q, want %q", profile.Class, tt.wantClass)
			}
			if profile.MaxWidth != tt.wantMaxWidth {
				t.Errorf("MaxWidth = %d, want %d", profile.MaxWidth, tt.wantMaxWidth)
			}
			if profile.Quality != tt.wantQuality {
				t.Errorf("Quality = %d, want %d", profile.Quality, tt.wantQuality)
			}
			if profile.PreferredFormat != tt.wantFormat {
				t.Errorf("PreferredFormat = %q, want %q", profile.PreferredFormat, tt.wantFormat)
			}
		})
	}
}

func TestNegotiateFormat(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		preferred    string
		want         string
	}{
		{
			name:         "browser accepts webp and preferred is webp",
			acceptHeader: "image/avif,image/webp,image/apng,image/*,*/*;q=0.8",
			preferred:    "webp",
			want:         "webp",
		},
		{
			name:         "browser does not accept webp",
			acceptHeader: "image/jpeg, image/png, */*",
			preferred:    "webp",
			want:         "jpeg",
		},
		{
			name:         "preferred is jpeg even if webp accepted",
			acceptHeader: "image/webp,image/apng,*/*",
			preferred:    "jpeg",
			want:         "jpeg",
		},
		{
			name:         "empty accept header",
			acceptHeader: "",
			preferred:    "webp",
			want:         "jpeg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NegotiateFormat(tt.acceptHeader, tt.preferred)
			if got != tt.want {
				t.Errorf("NegotiateFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}
