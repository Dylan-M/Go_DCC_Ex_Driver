package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersionTags(t *testing.T) {
	for _, tag := range []string{"v0.0.0", "v1.2.3", "v12.34.56", "v1.2.3-rc.1", "v1.2.3-alpha-1", "v1.2.3-0"} {
		version, pre, err := parseTag(tag)
		if err != nil || !strings.HasPrefix(tag, "v"+version) || pre != strings.Contains(tag, "-") {
			t.Fatal(tag, version, pre, err)
		}
	}
	for _, tag := range []string{"1.2.3", "v1.2", "v01.2.3", "v1.02.3", "v1.2.03", "v1.2.3-01", "v1.2.3-rc.01", "v1.2.3-", "v1.2.3+build", "v1.2.3/../../x", "v1.2.3\nlatest=true"} {
		if _, _, err := parseTag(tag); err == nil {
			t.Fatal("invalid version accepted", tag)
		}
	}
	if _, err := assetName("v1.2.3", "android-386"); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestAndroidAlphaReleasePolicy(t *testing.T) {
	for tag, want := range map[string]bool{
		"v0.0.1-alpha.1": true, "v1.0.0-alpha": true,
		"v1.0.0-beta.1": false, "v1.0.0-rc.1": false, "v1.0.0": false,
		"v1.0.0-not-alpha.1": false, "v1.0.0-alphabet.1": false,
	} {
		if androidAlpha(tag) != want {
			t.Fatal("incorrect Android publication policy", tag)
		}
		if err := run([]string{"validate", tag}); err != nil {
			t.Fatal(err)
		}
	}
	tag := "v0.0.1-alpha.3"
	name, err := assetName(tag, "android-arm64")
	if err != nil || name != "Go_DCC_Ex_Driver-v0.0.1-alpha.3-android-arm64-debug.apk" {
		t.Fatal(name, err)
	}
	dir := t.TempDir()
	for _, target := range targets {
		file, err := assetName(tag, target)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(target), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if checksums(tag, dir) == nil {
		t.Fatal("alpha release accepted without Android APK")
	}
	apk := []byte("APK test fixture")
	if err := os.WriteFile(filepath.Join(dir, name), apk, 0600); err != nil {
		t.Fatal(err)
	}
	if err := checksums(tag, dir); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil || !strings.Contains(string(manifest), fmt.Sprintf("%x  %s\n", sha256.Sum256(apk), name)) || strings.Count(string(manifest), "\n") != 7 {
		t.Fatal("missing APK checksum", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if checksums(tag, dir) == nil {
		t.Fatal("empty Android APK accepted")
	}
}

func TestArchiveLayoutsAndPermissions(t *testing.T) {
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			files, err := entries("v1.2.3-rc.1", target, []byte("binary"), []byte("readme"))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			linux := strings.HasPrefix(target, "linux-")
			if err := archive(&buf, files, linux); err != nil {
				t.Fatal(err)
			}
			got := map[string][]byte{}
			modes := map[string]os.FileMode{}
			if linux {
				gz, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				defer gz.Close()
				tr := tar.NewReader(gz)
				for {
					header, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					data, err := io.ReadAll(tr)
					if err != nil {
						t.Fatal(err)
					}
					got[header.Name], modes[header.Name] = data, os.FileMode(header.Mode)
				}
			} else {
				zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range zr.File {
					r, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					data, err := io.ReadAll(r)
					r.Close()
					if err != nil {
						t.Fatal(err)
					}
					got[f.Name], modes[f.Name] = data, f.Mode()
				}
			}
			binaryName := "dccex-driver"
			if strings.HasPrefix(target, "windows-") {
				binaryName += ".exe"
			}
			if strings.HasPrefix(target, "macos-") {
				binaryName = "Go_DCC_Ex_Driver.app/Contents/MacOS/dccex-driver"
				plist := string(got["Go_DCC_Ex_Driver.app/Contents/Info.plist"])
				if !strings.Contains(plist, "<string>1.2.3</string>") || strings.Contains(plist, "rc.1") || !strings.Contains(plist, "<string>dccex-driver</string>") {
					t.Fatal("invalid bundle metadata")
				}
			}
			if string(got[binaryName]) != "binary" || modes[binaryName].Perm() != 0755 || string(got["README.md"]) != "readme" {
				t.Fatal("archive contents or executable mode wrong", got, modes)
			}
			if len(got) != len(files) {
				t.Fatal("missing or duplicate files")
			}
		})
	}
}

func TestExecutableArchitecture(t *testing.T) {
	name, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	if err := verifyBinary(name, platform+"-"+runtime.GOARCH); err != nil {
		t.Fatal(err)
	}
	other := "arm64"
	if runtime.GOARCH == "arm64" {
		other = "amd64"
	}
	if err := verifyBinary(name, platform+"-"+other); err == nil {
		t.Fatal("incorrect architecture accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad-binary")
	if err := os.WriteFile(bad, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if verifyBinary(bad, target) == nil {
			t.Fatal("invalid executable accepted", target)
		}
	}
}

func TestChecksumsRequireAllSixAssets(t *testing.T) {
	dir := t.TempDir()
	if checksums("v1.2.3", dir) == nil {
		t.Fatal("missing assets accepted")
	}
	var want strings.Builder
	for _, target := range targets {
		name, err := assetName("v1.2.3", target)
		if err != nil {
			t.Fatal(err)
		}
		data := []byte(target)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&want, "%x  %s\n", sha256.Sum256(data), name)
	}
	if err := checksums("v1.2.3", dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil || string(got) != want.String() {
		t.Fatal("manifest mismatch", err)
	}
	name, _ := assetName("v1.2.3", targets[0])
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if checksums("v1.2.3", dir) == nil {
		t.Fatal("empty asset accepted")
	}
}
