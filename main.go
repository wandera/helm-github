package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/wandera/helm-github/helm"
	"golang.org/x/oauth2"
	"sigs.k8s.io/yaml"
)

const (
	protocol      = "github"
	githubHost    = "github.com"
	indexFilename = "index.yaml"
)

var client *github.Client

func init() {
	token, err := loadGithubToken()
	if err != nil {
		log.Panic(err)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client = github.NewClient(tc)
}

func main() {
	if env, ok := os.LookupEnv("HELMGITHUB_DEBUG_LOG"); ok {
		t, err := os.OpenFile(env, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Panic(err)
		}
		defer t.Close()
		log.SetOutput(t)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if len(os.Args) == 5 {
		uri := strings.TrimPrefix(os.Args[4], protocol+"://")
		if strings.HasSuffix(uri, indexFilename) {
			file, err := fetchIndexFile(ctx, uri)
			if err != nil {
				log.Panic(err)
			}
			file = switchProtocolToGithub(file)
			file.SortEntries()
			bytes, err := yaml.Marshal(&file)
			if err != nil {
				log.Panic(err)
			}
			fmt.Println(string(bytes))
		} else {
			rc, err := fetchArchive(ctx, uri)
			if err != nil {
				log.Panic(err)
			}
			defer rc.Close()
			_, err = io.Copy(os.Stdout, rc)
			if err != nil {
				log.Panic(err)
			}
		}
	}
}

func loadGithubToken() (string, error) {
	if env, ok := os.LookupEnv("GITHUB_TOKEN"); ok {
		return env, nil
	}
	if env, ok := os.LookupEnv("GIT_ASKPASS"); ok {
		f, err := os.Open(env)
		if err != nil {
			return "", err
		}
		bytes, err := io.ReadAll(f)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	return "", fmt.Errorf("github token not found")
}

func getIndexBranch() string {
	if env, ok := os.LookupEnv("HELMGITHUB_INDEX_BRANCH"); ok {
		return env
	}
	return "gh-pages"
}

func fetchIndexFile(ctx context.Context, uri string) (helm.IndexFile, error) {
	owner, repository := parseOwnerRepository(uri)
	contents, _, _, err := client.Repositories.GetContents(ctx, owner, repository, indexFilename, &github.RepositoryContentGetOptions{Ref: getIndexBranch()})
	if err != nil {
		return helm.IndexFile{}, err
	}
	decoded, err := contents.GetContent()
	if err != nil {
		return helm.IndexFile{}, err
	}
	file := helm.IndexFile{}
	err = yaml.UnmarshalStrict([]byte(decoded), &file)
	if err != nil {
		return helm.IndexFile{}, err
	}
	return file, nil
}

func fetchArchive(ctx context.Context, uri string) (io.ReadCloser, error) {
	owner, repository := parseOwnerRepository(uri)
	tag := parseArtifactName(uri)
	release, _, err := client.Repositories.GetReleaseByTag(ctx, owner, repository, tag)
	if err != nil {
		return nil, err
	}
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.GetBrowserDownloadURL(), uri) {
			rc, _, err := client.Repositories.DownloadReleaseAsset(ctx, owner, repository, asset.GetID(), http.DefaultClient)
			if err != nil {
				return nil, err
			}
			return rc, nil
		}
	}
	return nil, nil
}

func switchProtocolToGithub(file helm.IndexFile) helm.IndexFile {
	for name, versions := range file.Entries {
		file.Entries[name] = mapFunc(versions, func(chart helm.ChartVersion) helm.ChartVersion {
			chart.URLs = mapFunc(chart.URLs, mapGithubURL)
			return chart
		})
	}
	return file
}

func parseOwnerRepository(uri string) (string, string) {
	uri = strings.TrimPrefix(uri, githubHost)
	uri = strings.TrimLeft(uri, "/")
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", ""
	}
	seps := strings.Split(parsed.Path, "/")
	if len(seps) >= 2 {
		return seps[0], seps[1]
	}
	return "", ""
}

func parseArtifactName(uri string) string {
	return uri[strings.LastIndex(uri, "/")+1 : strings.LastIndex(uri, ".")]
}

func mapGithubURL(urlString string) string {
	parse, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}
	if (parse.Scheme == "https" || parse.Scheme == "http") && parse.Host == githubHost {
		parse.Scheme = protocol
		return parse.String()
	}
	return urlString
}

func mapFunc[T any, R any](in []T, f func(T) R) []R {
	ret := make([]R, len(in))
	for i, t := range in {
		ret[i] = f(t)
	}
	return ret
}
