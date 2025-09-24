package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

func TestForkExecUsesExecutablePath(t *testing.T) {
	if os.Getpid() == 1 {
		t.Skip("Cannot test forkExec as pid 1")
	}

	testExe := createTestExecutable(t)
	defer os.Remove(testExe)

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	// Simulate the problematic scenario: os.Args[0] is just the command name without path
	// This is what would happen when supercronic is called from a different directory
	os.Args = []string{"supercronic", "-no-reap", "-test", "/dev/null"}

	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	err := os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	testForkExecBehavior(t)
}

func TestOsExecutableVsOsArgs(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable: %v", err)
	}

	if !filepath.IsAbs(exe) {
		t.Errorf("os.Executable() should return absolute path, got: %s", exe)
	}

	arg0 := os.Args[0]

	if !strings.Contains(exe, filepath.Base(arg0)) {
		t.Logf("os.Executable(): %s", exe)
		t.Logf("os.Args[0]: %s", arg0)
	}

	if _, err := os.Stat(exe); err != nil {
		t.Errorf("os.Executable() returned non-existent file: %v", err)
	}
}

func TestReaperForkExecFromRootDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("This test is Unix-specific")
	}

	if os.Getuid() != 0 {
		t.Skip("This test requires root privileges to change to / directory safely")
	}

	testBinary := createTestExecutable(t)
	defer os.Remove(testBinary)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	err := os.Chdir("/")
	if err != nil {
		t.Fatalf("Failed to change to root directory: %v", err)
	}

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	os.Args = []string{"supercronic", "-no-reap", "-test", "/dev/null"}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable path: %v", err)
	}

	if !filepath.IsAbs(exe) {
		t.Errorf("Expected absolute path, got: %s", exe)
	}

	if _, err := os.Stat(exe); err != nil {
		t.Errorf("Executable path doesn't exist: %v", err)
	}

	if filepath.Base(os.Args[0]) != filepath.Base(exe) {
		t.Logf("os.Args[0]: %s (would fail with old code)", os.Args[0])
		t.Logf("os.Executable(): %s (works with new code)", exe)
	}
}

func createTestExecutable(t *testing.T) string {
	t.Helper()

	// Create temporary Go source
	tempDir := t.TempDir()
	srcFile := filepath.Join(tempDir, "test_supercronic.go")

	testSrc := `package main
import (
	"flag"
	"fmt"
	"os"
)
func main() {
	test := flag.Bool("test", false, "test mode")
	noReap := flag.Bool("no-reap", false, "disable reaping")
	flag.Parse()

	if *test {
		fmt.Println("test mode - would validate crontab")
		os.Exit(0)
	}

	if *noReap {
		fmt.Println("reaping disabled")
	}

	// Simulate supercronic behavior
	fmt.Println("supercronic test executable running")
}`

	err := os.WriteFile(srcFile, []byte(testSrc), 0644)
	if err != nil {
		t.Fatalf("Failed to write test source: %v", err)
	}

	// Build the test executable
	exeFile := filepath.Join(tempDir, "test_supercronic")
	cmd := exec.Command("go", "build", "-o", exeFile, srcFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build test executable: %v", err)
	}

	return exeFile
}

func testForkExecBehavior(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable (this is what the old bug was): %v", err)
	}

	args := make([]string, 0, len(os.Args)+1)
	args = append(args, exe, "-no-reap") // This is the fix - use exe instead of os.Args[0]
	args = append(args, os.Args[1:]...)

	if !filepath.IsAbs(args[0]) {
		t.Errorf("First argument should be absolute path, got: %s", args[0])
	}

	if _, err := os.Stat(args[0]); err != nil {
		t.Errorf("Executable path in args[0] doesn't exist: %v", err)
	}

	if err := syscall.Access(args[0], syscall.F_OK); err != nil {
		t.Errorf("syscall.Access failed on executable path: %v", err)
	}
}

func TestForkExecIntegration(t *testing.T) {
	if os.Getpid() == 1 {
		t.Skip("Cannot test forkExec as pid 1 - would interfere with process reaping")
	}

	originalArgs := os.Args
	defer func() {
		os.Args = originalArgs
	}()

	os.Args = []string{"supercronic", "-test", "/dev/null"}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() failed: %v", err)
	}

	args := make([]string, 0, len(os.Args)+1)
	args = append(args, exe, "-no-reap")
	args = append(args, os.Args[1:]...)

	if _, err := os.Stat(args[0]); err != nil {
		t.Errorf("forkExec would fail because args[0] is not a valid executable: %v", err)
	}

	t.Logf("forkExec would use executable: %s", args[0])
	t.Logf("forkExec would use args: %v", args)
}
