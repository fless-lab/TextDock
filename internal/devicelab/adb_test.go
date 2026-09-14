package devicelab

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDeviceClassificationAndConsoleEncoding(t *testing.T) {
	items := ParseDevices("* daemon started successfully\nList of devices attached\nemulator-5554\tdevice model:Pixel_Test\nemulator-5556\toffline\nserial123\tdevice model:Physical_Phone\nunauth\tunauthorized\n")
	if len(items) != 4 || !items[0].CanInject || items[1].CanInject || items[2].CanInject || items[3].CanInject {
		t.Fatalf("classification: %+v", items)
	}
	for _, serial := range []string{"", "serial123", "emulator-5554\nkill", "emulator-5554;id", "emulator-5555", "emulator-99999"} {
		if EmulatorSerial(serial) {
			t.Fatalf("unsafe serial accepted: %q", serial)
		}
	}
	if err := ValidateBody("code \"482193\"; $(ignored)\\literal\n@login.example #482193 🙂"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"", "\t", "hi\x00kill", "hi\rkill", strings.Repeat("x", 1025)} {
		if err := ValidateBody(body); !errors.Is(err, ErrInput) {
			t.Fatalf("unsupported body accepted: %q", body)
		}
	}
}

func TestMissingExecutableAndOutputBound(t *testing.T) {
	_, err := (Executor{Path: "/does-not-exist/textdock-adb"}).Run(context.Background(), "version")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing executable: %v", err)
	}
	var output cappedOutput
	input := []byte(strings.Repeat("x", 100000))
	if n, err := output.Write(input); n != len(input) || err != nil || len(output.data) != 65536 || !output.overflow {
		t.Fatal("subprocess output was not bounded and drained")
	}
}
