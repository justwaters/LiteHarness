package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
)

// hasColor reports whether stdout supports color (a TTY).
var hasColor = isTTY()

func isTTY() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// c wraps text in ANSI color if color is enabled.
func c(code, text string) string {
	if !hasColor {
		return text
	}
	return code + text + ColorReset
}

// PrintModel streams model text to stdout.
func PrintModel(w io.Writer, text string) {
	fmt.Print(c(ColorGreen, text))
}

// PrintToolTrace prints a dimmed tool trace line.
func PrintToolTrace(format string, args ...interface{}) {
	fmt.Printf(c(ColorDim, "[tool] ")+format+"\n", args...)
}

// PrintToolResult prints a tool result in cyan.
func PrintToolResult(format string, args ...interface{}) {
	fmt.Printf(c(ColorCyan, "[result] ")+format+"\n", args...)
}

// PrintUser prints user input in dimmed blue.
func PrintUser(text string) {
	fmt.Println(c(ColorDim+ColorBlue, "> "+text))
}

// PrintError prints an error in red.
func PrintError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, c(ColorRed, "Error: ")+format+"\n", args...)
}

// Spinner shows a simple spinning indicator while thinking.
type Spinner struct {
	running bool
	stop    chan struct{}
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠶", "⠵", "⠷", "⠏"}

// StartSpinner begins spinning on stdout.
func StartSpinner() *Spinner {
	s := &Spinner{running: true, stop: make(chan struct{})}
	go s.spin()
	return s
}

func (s *Spinner) spin() {
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Print("\r" + stringsRepeat(" ", 20) + "\r")
			return
		default:
			if hasColor {
				fmt.Printf("\r%s %s", c(ColorYellow, spinnerChars[i%len(spinnerChars)]), "thinking...")
			} else {
				fmt.Printf("\r%s thinking...", spinnerChars[i%len(spinnerChars)])
			}
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	if s.running {
		s.running = false
		close(s.stop)
	}
}

func stringsRepeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// ClearLine clears the current terminal line.
func ClearLine() {
	fmt.Print("\r\033[K")
}
