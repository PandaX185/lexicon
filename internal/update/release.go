package update

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	GitHubAPI     = "https://api.github.com"
	ReleaseBase   = "https://github.com/PandaX185/lexicon/releases/download"
	Repository    = "PandaX185/lexicon"
	DictionaryTag = "dictionary-3.0"

	EnSQLiteSHA256 = "556973a4033d85d7ba7a778430064638129eed9f9e998ff7894b0eaf22778780"
	ManifestSHA256 = "7f962840a2aea9d4e03258207441ab4110face5392ef6ac6bfb7ba37fc3f37a0"
)

var binaryNames = []string{"lexicon", "lexicon-daemon", "lexicon-installer", "lexicon-popup"}

// Release is the verified metadata needed to download one release.
type Release struct {
	Tag     string
	Version string
	Assets  map[string]string // asset name -> browser download URL
}

// Discoverer finds release metadata. Tests use a local fake.
type Discoverer interface {
	Latest(context.Context) (Release, error)
	Version(context.Context, string) (Release, error)
}

// GitHubDiscoverer reads release metadata from the GitHub HTTPS API.
type GitHubDiscoverer struct {
	APIURL string
	Client HTTPClient
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

func (g GitHubDiscoverer) Latest(ctx context.Context) (Release, error) {
	return g.discover(ctx, g.base()+"/repos/"+Repository+"/releases/latest")
}

func (g GitHubDiscoverer) Version(ctx context.Context, version string) (Release, error) {
	if _, err := ParseVersion(version); err != nil {
		return Release{}, err
	}
	return g.discover(ctx, g.base()+"/repos/"+Repository+"/releases/tags/v"+version)
}

func (g GitHubDiscoverer) base() string {
	if g.APIURL != "" {
		return strings.TrimRight(g.APIURL, "/")
	}
	return GitHubAPI
}

func (g GitHubDiscoverer) discover(ctx context.Context, rawURL string) (Release, error) {
	data, err := fetch(ctx, g.Client, rawURL, maxChecksumsBytes)
	if err != nil {
		return Release{}, err
	}
	var payload githubRelease
	if err := json.Unmarshal(data, &payload); err != nil {
		return Release{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return Release{}, fmt.Errorf("release %s is a draft or prerelease", payload.TagName)
	}
	version, err := ParseVersion(strings.TrimPrefix(payload.TagName, "v"))
	if err != nil {
		return Release{}, fmt.Errorf("release tag %q: %w", payload.TagName, err)
	}
	assets := make(map[string]string, len(payload.Assets))
	for _, asset := range payload.Assets {
		assets[asset.Name] = asset.BrowserDownloadURL
	}
	return Release{Tag: "v" + version, Version: version, Assets: assets}, nil
}

// ParseVersion accepts only a strict X.Y.Z stable version.
func ParseVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid version %q: want X.Y.Z", version)
	}
	for _, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || strconv.Itoa(number) != part {
			return "", fmt.Errorf("invalid version %q: want X.Y.Z", version)
		}
	}
	return version, nil
}

// TarballName returns the release asset for a version and Go architecture.
func TarballName(version, arch string) (string, error) {
	switch arch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q", arch)
	}
	return "lexicon-" + version + "-linux-" + arch + ".tar.gz", nil
}
