package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	selfupdate "github.com/creativeprojects/go-selfupdate/update"
)

func TestInstallLatest(t *testing.T) {
	newBinary := []byte("new bgst binary")
	hash := sha256.Sum256(newBinary)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/latest":
			fmt.Fprintf(response, `{"tag_name":"v0.2.0","html_url":"%s/release","assets":[{"name":"bgst-linux-amd64","browser_download_url":"%s/binary"},{"name":"checksums.txt","browser_download_url":"%s/checksums"}]}`, server.URL, server.URL, server.URL)
		case "/binary":
			_, _ = response.Write(newBinary)
		case "/checksums":
			fmt.Fprintf(response, "%x  bgst-linux-amd64\n", hash)
		default:
			http.NotFound(response, request)
		}
	}))
	t.Cleanup(server.Close)

	target := filepath.Join(t.TempDir(), "bgst")
	if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	service := New()
	service.latestURL = server.URL + "/latest"
	service.targetPath = target
	service.goos = "linux"
	service.goarch = "amd64"

	result, err := service.InstallLatest(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || result.Latest != "v0.2.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(newBinary) {
		t.Fatalf("updated binary = %q", contents)
	}
}

func TestInstallLatestAlreadyCurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.com/release","assets":[]}`))
	}))
	t.Cleanup(server.Close)

	service := New()
	service.latestURL = server.URL
	service.apply = func(_ io.Reader, _ selfupdate.Options) error {
		t.Fatal("apply should not run when already current")
		return nil
	}
	result, err := service.InstallLatest(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated {
		t.Fatal("already current release should not update")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left       string
		right      string
		want       int
		comparable bool
	}{
		{left: "v0.1.0", right: "v0.2.0", want: -1, comparable: true},
		{left: "v1.2.3", right: "1.2.3", want: 0, comparable: true},
		{left: "v2.0.0", right: "v1.9.9", want: 1, comparable: true},
		{left: "dev", right: "v1.0.0", comparable: false},
	}
	for _, test := range tests {
		got, comparable := compareVersions(test.left, test.right)
		if got != test.want || comparable != test.comparable {
			t.Fatalf("compareVersions(%q, %q) = %d, %v", test.left, test.right, got, comparable)
		}
	}
}
