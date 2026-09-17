// Command release validates version tags and packages native desktop builds.
// It has no GUI dependencies and never creates tags or publishes releases.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$`)
var targets = []string{"windows-amd64", "windows-arm64", "macos-amd64", "macos-arm64", "linux-amd64", "linux-arm64"}

func parseTag(tag string) (version string, prerelease bool, err error) {
	if !versionTag.MatchString(tag) {
		return "", false, fmt.Errorf("expected vMAJOR.MINOR.PATCH or a prerelease tag, got %q", tag)
	}
	version, suffix, prerelease := strings.Cut(strings.TrimPrefix(tag, "v"), "-")
	if prerelease {
		for _, id := range strings.Split(suffix, ".") {
			if len(id) > 1 && id[0] == '0' && strings.Trim(id, "0123456789") == "" {
				return "", false, fmt.Errorf("numeric prerelease identifiers cannot have leading zeros")
			}
		}
	}
	return version, prerelease, nil
}

func assetName(tag, target string) (string, error) {
	if _, _, err := parseTag(tag); err != nil {
		return "", err
	}
	for _, supported := range targets {
		if target == supported {
			ext := ".zip"
			if strings.HasPrefix(target, "linux-") {
				ext = ".tar.gz"
			}
			return "Go_DCC_Ex_Driver-" + tag + "-" + target + ext, nil
		}
	}
	return "", fmt.Errorf("unsupported release target %q", target)
}

// Check the actual executable header so an emulated compiler cannot silently
// label an x64 binary as ARM64 (or vice versa).
func verifyBinary(path, target string) error {
	arm := strings.HasSuffix(target, "-arm64")
	switch {
	case strings.HasPrefix(target, "windows-"):
		f, err := pe.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		want := uint16(pe.IMAGE_FILE_MACHINE_AMD64)
		if arm {
			want = pe.IMAGE_FILE_MACHINE_ARM64
		}
		if f.Machine != want {
			return fmt.Errorf("PE architecture does not match %s", target)
		}
	case strings.HasPrefix(target, "linux-"):
		f, err := elf.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		want := elf.EM_X86_64
		if arm {
			want = elf.EM_AARCH64
		}
		if f.Machine != want {
			return fmt.Errorf("ELF architecture does not match %s", target)
		}
	case strings.HasPrefix(target, "macos-"):
		f, err := macho.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		want := macho.CpuAmd64
		if arm {
			want = macho.CpuArm64
		}
		if f.Cpu != want {
			return fmt.Errorf("Mach-O architecture does not match %s", target)
		}
	default:
		return fmt.Errorf("unknown target %q", target)
	}
	return nil
}

type entry struct {
	name string
	data []byte
	mode os.FileMode
}

func entries(tag, target string, binary, readme []byte) ([]entry, error) {
	if _, err := assetName(tag, target); err != nil {
		return nil, err
	}
	version, _, _ := parseTag(tag)
	files := []entry{{"README.md", readme, 0644}}
	name := "dccex-driver"
	if strings.HasPrefix(target, "windows-") {
		name += ".exe"
	}
	if strings.HasPrefix(target, "macos-") {
		name = "Go_DCC_Ex_Driver.app/Contents/MacOS/dccex-driver"
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.github.Dylan-M.Go_DCC_Ex_Driver</string>
<key>CFBundleName</key><string>Go_DCC_Ex_Driver</string>
<key>CFBundleDisplayName</key><string>DCC-EX Native Throttle</string>
<key>CFBundleExecutable</key><string>dccex-driver</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleShortVersionString</key><string>%s</string>
<key>CFBundleVersion</key><string>%s</string>
<key>NSHighResolutionCapable</key><true/>
</dict></plist>
`, version, version)
		files = append(files, entry{"Go_DCC_Ex_Driver.app/Contents/Info.plist", []byte(plist), 0644})
	}
	return append(files, entry{name, binary, 0755}), nil
}

func archive(w io.Writer, files []entry, linux bool) error {
	stamp := time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	if linux {
		gz := gzip.NewWriter(w)
		tw := tar.NewWriter(gz)
		for _, f := range files {
			if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: int64(f.mode), Size: int64(len(f.data)), ModTime: stamp}); err != nil {
				return err
			}
			if _, err := tw.Write(f.data); err != nil {
				return err
			}
		}
		if err := tw.Close(); err != nil {
			return err
		}
		return gz.Close()
	}
	zw := zip.NewWriter(w)
	for _, f := range files {
		header := &zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: stamp}
		header.SetMode(f.mode)
		out, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := out.Write(f.data); err != nil {
			return err
		}
	}
	return zw.Close()
}

func packageBinary(tag, target, binaryPath, output string) error {
	name, err := assetName(tag, target)
	if err != nil {
		return err
	}
	if err := verifyBinary(binaryPath, target); err != nil {
		return err
	}
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		return err
	}
	files, err := entries(tag, target, binary, readme)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(output, name))
	if err != nil {
		return err
	}
	err = archive(f, files, strings.HasPrefix(target, "linux-"))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func checksums(tag, dir string) error {
	var lines strings.Builder
	for _, target := range targets {
		name, err := assetName(tag, target)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if len(data) == 0 {
			return fmt.Errorf("empty release asset %s", name)
		}
		fmt.Fprintf(&lines, "%x  %s\n", sha256.Sum256(data), name)
	}
	return os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(lines.String()), 0644)
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "validate" {
		version, pre, err := parseTag(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("version=%s\nprerelease=%t\n", version, pre)
		return nil
	}
	if len(args) == 5 && args[0] == "package" {
		return packageBinary(args[1], args[2], args[3], args[4])
	}
	if len(args) == 3 && args[0] == "checksums" {
		return checksums(args[1], args[2])
	}
	return fmt.Errorf("usage: release validate TAG | package TAG TARGET BINARY OUTDIR | checksums TAG DIR")
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
