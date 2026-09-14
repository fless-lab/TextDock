package message

import "testing"

func TestNativeOTPFormats(t *testing.T) {
	web, err := FormatOTP(OTPFormat{Format: "webotp", Code: "482193", Domain: "login.example.test"})
	if err != nil || web != "Your verification code is 482193.\n\n@login.example.test #482193" {
		t.Fatalf("web format: %q %v", web, err)
	}
	android, err := FormatOTP(OTPFormat{Format: "android", Code: "482193", AppHash: "FA+9qCX9VSu"})
	if err != nil || len(android) > 140 {
		t.Fatalf("Android format: %q %v", android, err)
	}
	for _, domain := range []string{"https://example.test", "example.test:18257", "example.test/path", "evil.test\n@example.test", "-bad.test"} {
		if _, err := FormatOTP(OTPFormat{Format: "webotp", Code: "482193", Domain: domain}); err == nil {
			t.Fatalf("invalid hostname accepted: %q", domain)
		}
	}
	if _, err := FormatOTP(OTPFormat{Format: "android", Code: "abcdef", AppHash: "short"}); err == nil {
		t.Fatal("invalid Retriever format accepted")
	}
}
