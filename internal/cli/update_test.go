package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestUpdateCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v9.9.9","assets":[]}`)
	}))
	defer server.Close()
	oldAPI, oldVersion := releasesAPI, version
	releasesAPI, version = server.URL, "v1.0.0"
	t.Cleanup(func() { releasesAPI, version = oldAPI, oldVersion })
	var out, errOut bytes.Buffer
	if code := runUpdate([]string{"--check"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "v1.0.0 → v9.9.9") {
		t.Fatalf("update check = %d, %s | %s", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := runUpdate([]string{"--check", "--json"}, &out, &errOut); code != 0 || !json.Valid(out.Bytes()) {
		t.Fatalf("JSON update check = %d, %s | %s", code, out.String(), errOut.String())
	}
}

func TestReleaseDownloadsAndChecksum(t *testing.T) {
	name := fmt.Sprintf("respex_1.2.3_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	release := &releaseInfo{Tag: "v1.2.3", Assets: []releaseAsset{{Name: name, URL: "binary"}, {Name: "checksums.txt", URL: "checksums"}}}
	binary, checksums, err := releaseDownloads(release)
	if err != nil || binary.Name != name || checksums.Name != "checksums.txt" {
		t.Fatalf("release downloads = %+v, %+v, %v", binary, checksums, err)
	}
	checksum, err := expectedChecksum([]byte("abc123  "+name+"\n"), name)
	if err != nil || checksum != "abc123" {
		t.Fatalf("checksum = %q, %v", checksum, err)
	}
}
