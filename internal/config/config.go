package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"
)

// File contains non-secret runtime defaults. Environment and CLI flags override
// these values; the server token remains an environment-only secret.
type File struct {
	Listen     string `json:"listen"`
	DB         string `json:"db"`
	PublicURL  string `json:"public_url"`
	Retention  string `json:"retention"`
	OTPPattern string `json:"otp_pattern"`
}

func Load(args []string) (File, error) {
	var out File
	path := os.Getenv("TEXTDOCK_CONFIG")
	for i, arg := range args {
		if arg == "--config" || arg == "-config" {
			if i+1 < len(args) {
				path = args[i+1]
			}
		}
		if len(arg) > 9 && arg[:9] == "--config=" {
			path = arg[9:]
		}
	}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 64<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, err
	}
	if d.Decode(new(any)) != io.EOF {
		return out, errors.New("config must contain one JSON object")
	}
	return out, nil
}
