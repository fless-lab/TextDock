// Package devicelab controls explicitly selected Android emulators. It does not
// invoke a host shell or send SMS through a physical device's SIM.
package devicelab

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
)

var ErrDisabled = errors.New("device lab is disabled; set TEXTDOCK_ADB_ENABLED=true")
var ErrUnavailable = errors.New("ADB is unavailable")
var ErrTarget = errors.New("select an online Android emulator that has completed boot")
var ErrInput = errors.New("invalid emulator SMS request")
var ErrConflict = errors.New("injection key was already used with different data or a deleted message")
var serialPattern = regexp.MustCompile(`^emulator-([0-9]{4,5})$`)

type Config struct {
	Enabled bool
	Path    string
	Timeout time.Duration
}

func FromEnv() (Config, error) {
	c := Config{Path: os.Getenv("TEXTDOCK_ADB_PATH"), Timeout: 8 * time.Second}
	if c.Path == "" {
		c.Path = "adb"
	}
	if value := os.Getenv("TEXTDOCK_ADB_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return c, errors.New("TEXTDOCK_ADB_ENABLED must be a boolean")
		}
		c.Enabled = enabled
	}
	if value := os.Getenv("TEXTDOCK_ADB_TIMEOUT"); value != "" {
		d, err := time.ParseDuration(value)
		if err != nil || d < time.Second || d > 30*time.Second {
			return c, errors.New("TEXTDOCK_ADB_TIMEOUT must be 1s–30s")
		}
		c.Timeout = d
	}
	return c, nil
}

type Runner interface {
	Run(context.Context, ...string) (string, error)
}
type Executor struct {
	Path    string
	Timeout time.Duration
}
type cappedOutput struct {
	data     []byte
	overflow bool
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 65536 - len(b.data)
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func (e Executor) Run(ctx context.Context, args ...string) (string, error) {
	path, err := exec.LookPath(e.Path)
	if err != nil {
		return "", fmt.Errorf("%w: install Android Platform Tools or set TEXTDOCK_ADB_PATH", ErrUnavailable)
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	command.WaitDelay = 2 * time.Second
	var output cappedOutput
	command.Stdout = &output
	command.Stderr = &output
	err = command.Run()
	text := strings.TrimSpace(string(output.data))
	if ctx.Err() != nil {
		return text, ctx.Err()
	}
	if output.overflow {
		return text, errors.New("ADB output exceeded 64 KiB")
	}
	return text, err
}

type Device struct {
	Serial    string `json:"serial"`
	State     string `json:"state"`
	Model     string `json:"model"`
	Kind      string `json:"kind"`
	CanInject bool   `json:"can_inject"`
	Reason    string `json:"reason"`
}
type Status struct {
	Enabled   bool     `json:"enabled"`
	Available bool     `json:"available"`
	Version   string   `json:"version"`
	Error     string   `json:"error,omitempty"`
	Devices   []Device `json:"devices"`
}

func EmulatorSerial(serial string) bool {
	match := serialPattern.FindStringSubmatch(serial)
	if match == nil {
		return false
	}
	port, _ := strconv.Atoi(match[1])
	return port >= 1024 && port <= 65534 && port%2 == 0
}
func ParseDevices(text string) []Device {
	out := make([]Device, 0)
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "adb ") {
			continue
		}
		state := fields[1]
		if state != "device" && state != "offline" && state != "unauthorized" && state != "recovery" && state != "sideload" && state != "no" {
			continue
		}
		d := Device{Serial: fields[0], State: state, Kind: "physical", Reason: "SMS injection is available for emulators only"}
		for _, f := range fields[2:] {
			if model, ok := strings.CutPrefix(f, "model:"); ok {
				d.Model = strings.ReplaceAll(model, "_", " ")
			}
		}
		if EmulatorSerial(d.Serial) {
			d.Kind = "emulator"
			d.CanInject = state == "device"
			d.Reason = ""
			if !d.CanInject {
				d.Reason = "Emulator is " + state
			}
		}
		out = append(out, d)
	}
	return out
}

// ValidateBody bounds the input before PDU encoding. API input normalizes CRLF
// to LF; other control characters are rejected rather than silently changed.
func ValidateBody(body string) error {
	if len(body) > 1024 || strings.TrimSpace(body) == "" {
		return fmt.Errorf("%w: body must contain 1–1024 UTF-8 bytes", ErrInput)
	}
	for _, r := range body {
		if unicode.IsControl(r) && r != '\n' {
			return fmt.Errorf("%w: body contains an unsupported control character", ErrInput)
		}
	}
	return nil
}

// ConsoleText uses documented UTF-16 escapes. Passing raw supplementary UTF-8
// characters through sms send can truncate them to one UCS-2 unit in emulator
// versions whose text converter predates surrogate pairs. Explicit pairs avoid
// that conversion while staying on the current modem's supported send path.
func ConsoleText(body string) (string, error) {
	if err := ValidateBody(body); err != nil {
		return "", err
	}
	var out strings.Builder
	for _, r := range body {
		switch {
		case r == '\\':
			out.WriteString(`\\`)
		case r == '\n':
			out.WriteString(`\n`)
		case r < 128:
			out.WriteRune(r)
		case r <= 0xffff:
			fmt.Fprintf(&out, `\u%04x`, r)
		default:
			high, low := utf16.EncodeRune(r)
			fmt.Fprintf(&out, `\u%04x\u%04x`, high, low)
		}
	}
	return out.String(), nil
}
