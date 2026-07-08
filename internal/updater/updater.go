package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate/update"
)

const maxAssetSize = 128 << 20

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type release struct {
	TagName string         `json:"tag_name"`
	URL     string         `json:"html_url"`
	Assets  []releaseAsset `json:"assets"`
}

type Result struct {
	Current    string
	Latest     string
	ReleaseURL string
	Updated    bool
}

type applyFunc func(io.Reader, selfupdate.Options) error

type Service struct {
	client     *http.Client
	latestURL  string
	goos       string
	goarch     string
	targetPath string
	apply      applyFunc
}

func New() Service {
	return Service{
		client:    &http.Client{Timeout: 2 * time.Minute},
		latestURL: "https://api.github.com/repos/davis7dotsh/bgst/releases/latest",
		goos:      runtime.GOOS,
		goarch:    runtime.GOARCH,
		apply:     selfupdate.Apply,
	}
}

func (s Service) InstallLatest(ctx context.Context, current string) (Result, error) {
	latest, err := s.fetchRelease(ctx)
	if err != nil {
		return Result{}, err
	}
	current = normalizeVersion(current)
	result := Result{Current: current, Latest: latest.TagName, ReleaseURL: latest.URL}

	if relation, comparable := compareVersions(current, latest.TagName); comparable && relation >= 0 {
		return result, nil
	}

	assetName, err := s.assetName()
	if err != nil {
		return Result{}, err
	}
	binaryURL, ok := findAsset(latest.Assets, assetName)
	if !ok {
		return Result{}, fmt.Errorf("release %s has no %s asset", latest.TagName, assetName)
	}
	checksumsURL, ok := findAsset(latest.Assets, "checksums.txt")
	if !ok {
		return Result{}, fmt.Errorf("release %s has no checksums.txt asset", latest.TagName)
	}

	checksums, err := s.download(ctx, checksumsURL)
	if err != nil {
		return Result{}, fmt.Errorf("download checksums: %w", err)
	}
	checksum, err := checksumFor(checksums, assetName)
	if err != nil {
		return Result{}, err
	}
	binary, err := s.download(ctx, binaryURL)
	if err != nil {
		return Result{}, fmt.Errorf("download %s: %w", assetName, err)
	}

	options := selfupdate.Options{TargetPath: s.targetPath, TargetMode: 0o755, Checksum: checksum}
	if err := s.apply(bytes.NewReader(binary), options); err != nil {
		if rollbackErr := selfupdate.RollbackError(err); rollbackErr != nil {
			return Result{}, fmt.Errorf("replace executable: %w (rollback also failed: %v)", err, rollbackErr)
		}
		return Result{}, fmt.Errorf("replace executable: %w", err)
	}
	result.Updated = true
	return result, nil
}

func (s Service) fetchRelease(ctx context.Context) (release, error) {
	body, err := s.download(ctx, s.latestURL)
	if err != nil {
		return release{}, fmt.Errorf("load latest release: %w", err)
	}
	var latest release
	if err := json.Unmarshal(body, &latest); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if latest.TagName == "" {
		return release{}, errors.New("latest release did not include a version")
	}
	return latest, nil
}

func (s Service) download(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "bgst-updater")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", request.URL.Host, response.Status)
	}
	if response.ContentLength > maxAssetSize {
		return nil, errors.New("download is unexpectedly large")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxAssetSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAssetSize {
		return nil, errors.New("download is unexpectedly large")
	}
	return body, nil
}

func (s Service) assetName() (string, error) {
	if (s.goos != "linux" && s.goos != "darwin" && s.goos != "windows") || (s.goarch != "amd64" && s.goarch != "arm64") {
		return "", fmt.Errorf("updates are not published for %s/%s", s.goos, s.goarch)
	}
	name := "bgst-" + s.goos + "-" + s.goarch
	if s.goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

func findAsset(assets []releaseAsset, name string) (string, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset.URL, true
		}
	}
	return "", false
}

func checksumFor(contents []byte, name string) ([]byte, error) {
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		checksum, err := hex.DecodeString(fields[0])
		if err != nil || len(checksum) != sha256.Size {
			return nil, fmt.Errorf("invalid SHA-256 checksum for %s", name)
		}
		return checksum, nil
	}
	return nil, fmt.Errorf("checksums.txt has no entry for %s", name)
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "dev"
	}
	return value
}

func compareVersions(left, right string) (int, bool) {
	parse := func(value string) ([3]int, bool) {
		value = strings.TrimPrefix(value, "v")
		value = strings.SplitN(value, "+", 2)[0]
		if strings.Contains(value, "-") {
			return [3]int{}, false
		}
		parts := strings.Split(value, ".")
		if len(parts) != 3 {
			return [3]int{}, false
		}
		var parsed [3]int
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil || number < 0 {
				return [3]int{}, false
			}
			parsed[index] = number
		}
		return parsed, true
	}

	leftVersion, leftOK := parse(left)
	rightVersion, rightOK := parse(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	for index := range leftVersion {
		if leftVersion[index] < rightVersion[index] {
			return -1, true
		}
		if leftVersion[index] > rightVersion[index] {
			return 1, true
		}
	}
	return 0, true
}
