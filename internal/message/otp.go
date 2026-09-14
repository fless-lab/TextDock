package message

import (
	"errors"
	"regexp"
	"strings"
)

type OTPFormat struct {
	Format  string `json:"format"`
	Code    string `json:"code"`
	Domain  string `json:"domain,omitempty"`
	AppHash string `json:"app_hash,omitempty"`
}

var otpCode = regexp.MustCompile(`^[A-Za-z0-9]{4,10}$`)
var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var appHash = regexp.MustCompile(`^[A-Za-z0-9+/]{11}$`)

func FormatOTP(in OTPFormat) (string, error) {
	if !otpCode.MatchString(in.Code) || !strings.ContainsAny(in.Code, "0123456789") {
		return "", errors.New("code must be 4–10 alphanumeric characters including a digit")
	}
	switch in.Format {
	case "webotp":
		domain := strings.ToLower(in.Domain)
		if len(domain) == 0 || len(domain) > 253 {
			return "", errors.New("provide the application's hostname without scheme, path or port")
		}
		for _, label := range strings.Split(domain, ".") {
			if !dnsLabel.MatchString(label) {
				return "", errors.New("domain must be an ASCII hostname without scheme, path or port")
			}
		}
		return "Your verification code is " + in.Code + ".\n\n@" + domain + " #" + in.Code, nil
	case "android":
		if !appHash.MatchString(in.AppHash) {
			return "", errors.New("Android SMS Retriever requires the correct 11-character app hash")
		}
		return "<#> Your verification code is " + in.Code + "\n" + in.AppHash, nil
	default:
		return "", errors.New("format must be webotp or android")
	}
}
