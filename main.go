package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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
var cacheDirBase string
var chartsCacheDir string

func init() {
	if env, ok := os.LookupEnv("HELMGITHUB_DEBUG_LOG"); ok {
		t, err := os.OpenFile(env, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Panic(err)
		}
		log.SetOutput(t)
	}
	token, err := loadGithubToken()
	if err != nil {
		log.Panic(err)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client = github.NewClient(tc)
	cacheDirBase = getCacheDirBase()
	chartsCacheDir = getChartCacheDir()
}

func main() {
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
			cacheFile, err := openCacheFile(uri)
			if err != nil {
				log.Panic(err)
			}
			defer cacheFile.Close()
			ok := validateDigest(getArchiveDigest(uri), cacheFile.Name())
			if !ok {
				resp, err := fetchArchive(ctx, uri)
				if err != nil {
					_ = os.Remove(cacheFile.Name())
					log.Panic(err)
				}
				defer resp.Close()
				_, err = io.Copy(io.MultiWriter(os.Stdout, cacheFile), resp)
				if err != nil {
					_ = os.Remove(cacheFile.Name())
					log.Panic(err)
				}
			} else {
				_, err := io.Copy(os.Stdout, cacheFile)
				if err != nil {
					log.Panic(err)
				}
			}
		}
	}
}

func getArchiveDigest(uri string) string {
	_, r := parseOwnerRepository(uri)
	bytes, err := os.ReadFile(path.Join(cacheDirBase, r+"-index.yaml"))
	if err != nil {
		return ""
	}
	idx := helm.IndexFile{}
	if err := yaml.Unmarshal(bytes, &idx); err != nil {
		return ""
	}
	for _, versions := range idx.Entries {
		for _, version := range versions {
			for _, u := range version.URLs {
				if strings.HasSuffix(u, uri) {
					return version.Digest
				}
			}
		}
	}
	return ""
}

func validateDigest(digest string, fileName string) bool {
	df, err := helm.DigestFile(fileName)
	if err != nil {
		log.Panic(err)
	}
	return digest == df
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

func getChartCacheDir() string {
	dir := getCacheDirBase()
	dir = path.Join(dir, "github", "chart")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		log.Panic(err)
	}
	return dir
}

func getCacheDirBase() string {
	var dir string
	if env, ok := os.LookupEnv("HELM_REPOSITORY_CACHE"); ok {
		dir = env
	} else {
		ucd, err := os.UserCacheDir()
		if err != nil {
			log.Panic(err)
		}
		dir = ucd
	}
	return dir
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

func openCacheFile(uri string) (*os.File, error) {
	artifactName := parseArtifactName(uri)
	chartPath := path.Join(chartsCacheDir, artifactName+".tgz")
	_, err := os.Stat(chartPath)
	if err != nil {
		create, err := os.Create(chartPath)
		if err != nil {
			return nil, err
		}
		return create, nil
	}
	open, err := os.Open(chartPath)
	if err != nil {
		return nil, err
	}
	return open, nil
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
	return nil, fmt.Errorf("asset '%s' not found", uri)
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
