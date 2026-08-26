// fakejava stands in for java.exe in tests: it records its arguments and exits.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	wd, _ := os.Getwd()
	os.WriteFile("java-args.txt", []byte(strings.Join(os.Args[1:], "\n")+"\n"), 0o644)
	fmt.Printf("fake java ran in %s with %d args\n", wd, len(os.Args)-1)
	if code := os.Getenv("LEMV_FAKE_EXIT"); code != "" {
		n, _ := strconv.Atoi(code)
		os.Exit(n) // lets the launcher's exit-code handling be tested
	}
	time.Sleep(4 * time.Second) // pretend the game is running for a moment
}
