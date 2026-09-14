// Test-only command fixture. Never linked into the TextDock executable.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	args := os.Args[1:]
	switch {
	case len(args) == 1 && args[0] == "version": fmt.Println("Android Debug Bridge version 1.0.41\nVersion textdock-test-fixture")
	case len(args) == 2 && args[0] == "devices": fmt.Println("List of devices attached\nemulator-5554\tdevice product:sdk model:Test_Emulator transport_id:1\nemulator-5556\toffline\nphysical-test\tdevice model:Physical_Test_Phone")
	case len(args) >= 2 && args[0] == "-s" && args[1] == "emulator-5554":
		switch {
		case len(args) == 5 && args[2] == "shell": fmt.Println("1")
		case len(args) == 5 && args[3] == "avd": fmt.Println("TextDock_Test\nOK")
		case len(args) == 6 && args[2] == "emu" && args[3] == "sms" && args[4] == "pdu":
			if strings.ContainsAny(args[5], "\r\n\x00") { fmt.Fprintln(os.Stderr, "unsafe console delimiter"); os.Exit(1) }
			if strings.Contains(args[5], "00720065006a006500630074002d0074006800690073002d006d006500730073006100670065") { fmt.Println("KO: fixture rejected message") } else { fmt.Println("OK") }
		default: os.Exit(1)
		}
	default: os.Exit(1)
	}
}
