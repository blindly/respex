package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var releasesAPI = "https://api.github.com/repos/blindly/respex/releases"

var updateHTTPClient = &http.Client{Timeout: 30 * time.Second}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

func fetchURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "respex/"+version)
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 100<<20))
}

func fetchRelease(ctx context.Context, requested string) (*releaseInfo, error) {
	url := releasesAPI + "/latest"
	if requested != "" {
		url = releasesAPI + "/tags/" + requested
	}
	body, err := fetchURL(ctx, url)
	if err != nil {
		return nil, err
	}
	var release releaseInfo
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}
	if release.Tag == "" {
		return nil, errors.New("release response has no tag")
	}
	return &release, nil
}

func releaseDownloads(release *releaseInfo) (releaseAsset, releaseAsset, error) {
	name := fmt.Sprintf("respex_%s_%s_%s", strings.TrimPrefix(release.Tag, "v"), runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	var binary, checksums releaseAsset
	for _, asset := range release.Assets {
		switch asset.Name {
		case name:
			binary = asset
		case "checksums.txt":
			checksums = asset
		}
	}
	if binary.URL == "" || checksums.URL == "" {
		return releaseAsset{}, releaseAsset{}, fmt.Errorf("release %s has no verified binary for %s/%s", release.Tag, runtime.GOOS, runtime.GOARCH)
	}
	return binary, checksums, nil
}

func expectedChecksum(content []byte, name string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return fields[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

func installUpdate(content []byte) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(executable), ".respex-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	backup := executable + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("prepare executable replacement: %w", err)
	}
	if err := os.Rename(tmpPath, executable); err != nil {
		_ = os.Rename(backup, executable)
		return fmt.Errorf("replace executable: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}

func runUpdate(args []string, out, errOut io.Writer) int {
	check := false
	jsonOutput := false
	requested := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--check":
			check = true
		case "--json":
			jsonOutput = true
		case "--version":
			if i+1 >= len(args) {
				return fail(errOut, errors.New("usage: respex update [--check] [--json] [--version tag]"))
			}
			i++
			requested = args[i]
		default:
			return fail(errOut, errors.New("usage: respex update [--check] [--json] [--version tag]"))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	release, err := fetchRelease(ctx, requested)
	if err != nil {
		return fail(errOut, err)
	}
	if release.Tag == version {
		if jsonOutput {
			if err := json.NewEncoder(out).Encode(map[string]any{"current": version, "latest": release.Tag, "update_available": false}); err != nil {
				return fail(errOut, err)
			}
		} else {
			fmt.Fprintf(out, "respex %s is already current\n", version)
		}
		return 0
	}
	if check {
		if jsonOutput {
			if err := json.NewEncoder(out).Encode(map[string]any{"current": version, "latest": release.Tag, "update_available": true}); err != nil {
				return fail(errOut, err)
			}
		} else {
			fmt.Fprintf(out, "update available: %s → %s\n", version, release.Tag)
		}
		return 0
	}
	binary, checksums, err := releaseDownloads(release)
	if err != nil {
		return fail(errOut, err)
	}
	checksumContent, err := fetchURL(ctx, checksums.URL)
	if err != nil {
		return fail(errOut, err)
	}
	expected, err := expectedChecksum(checksumContent, binary.Name)
	if err != nil {
		return fail(errOut, err)
	}
	content, err := fetchURL(ctx, binary.URL)
	if err != nil {
		return fail(errOut, err)
	}
	actual := sha256.Sum256(content)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return fail(errOut, errors.New("download checksum mismatch; executable was not changed"))
	}
	if err := installUpdate(content); err != nil {
		return fail(errOut, err)
	}
	if jsonOutput {
		if err := json.NewEncoder(out).Encode(map[string]any{"previous": version, "current": release.Tag, "updated": true}); err != nil {
			return fail(errOut, err)
		}
	} else {
		fmt.Fprintf(out, "updated respex %s → %s\n", version, release.Tag)
	}
	return 0
}
