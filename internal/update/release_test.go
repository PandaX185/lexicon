package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestDiscoverLatestIgnoresPrerelease(t *testing.T) {
	const body = `{
		"tag_name":"v0.3.0","draft":false,"prerelease":false,
		"assets":[
			{"name":"lexicon-0.3.0-linux-amd64.tar.gz","browser_download_url":"https://example.test/tar"},
			{"name":"checksums.sha256","browser_download_url":"https://example.test/sum"}
		]}`
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/PandaX185/lexicon/releases/latest" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	d := GitHubDiscoverer{APIURL: srv.URL, Client: srv.Client()}
	release, err := d.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.Version != "0.3.0" || release.Tag != "v0.3.0" {
		t.Fatalf("release = %+v", release)
	}
	if release.Assets["lexicon-0.3.0-linux-amd64.tar.gz"] != "https://example.test/tar" {
		t.Fatalf("assets = %+v", release.Assets)
	}
	if release.Assets["checksums.sha256"] != "https://example.test/sum" {
		t.Fatalf("assets = %+v", release.Assets)
	}
}

func TestDiscoverVersionRejectsInvalidTag(t *testing.T) {
	d := GitHubDiscoverer{APIURL: "https://api.github.com"}
	if _, err := d.Version(context.Background(), "latest"); err == nil {
		t.Fatal("invalid version was accepted")
	}
	if _, err := ParseVersion("v0.2.0"); err == nil {
		t.Fatal("version with leading v was accepted")
	}
}

func TestTarballName(t *testing.T) {
	name, err := TarballName("0.2.0", "amd64")
	if err != nil || name != "lexicon-0.2.0-linux-amd64.tar.gz" {
		t.Fatalf("name = %q, err = %v", name, err)
	}
	if _, err := TarballName("0.2.0", "386"); err == nil {
		t.Fatal("unsupported architecture was accepted")
	}
}

func TestDiscoverLatestRejectsDraftAndPrerelease(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "prerelease",
			body: `{"tag_name":"v0.3.0-rc1","draft":false,"prerelease":true,"assets":[]}`,
		},
		{
			name: "draft",
			body: `{"tag_name":"v0.3.0","draft":true,"prerelease":false,"assets":[]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/PandaX185/lexicon/releases/latest" {
					t.Errorf("path = %s", r.URL.Path)
				}
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			d := GitHubDiscoverer{APIURL: srv.URL, Client: srv.Client()}
			if _, err := d.Latest(context.Background()); err == nil {
				t.Fatal("draft/prerelease release was accepted")
			} else if !strings.Contains(err.Error(), "draft or prerelease") {
				t.Fatalf("error = %v, want a draft/prerelease error", err)
			}
		})
	}
}

func TestBinaryNames(t *testing.T) {
	want := []string{"lexicon", "lexicon-daemon", "lexicon-installer", "lexicon-popup"}
	if !slices.Equal(binaryNames, want) {
		t.Fatalf("binaryNames = %q, want %q", binaryNames, want)
	}
}
