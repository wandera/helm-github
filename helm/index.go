package helm

import (
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
)

// IndexFile represents the index file in a chart repository.
type IndexFile struct {
	ServerInfo  map[string]interface{}   `json:"serverInfo,omitempty"`
	APIVersion  string                   `json:"apiVersion"`
	Generated   time.Time                `json:"generated"`
	Entries     map[string]ChartVersions `json:"entries"`
	PublicKeys  []string                 `json:"publicKeys,omitempty"`
	Annotations map[string]string        `json:"annotations,omitempty"`
}

func (i IndexFile) SortEntries() {
	for _, versions := range i.Entries {
		sort.Sort(sort.Reverse(versions))
	}
}

type ChartVersion struct {
	Metadata
	URLs    []string  `json:"urls"`
	Created time.Time `json:"created,omitempty"`
	Removed bool      `json:"removed,omitempty"`
	Digest  string    `json:"digest,omitempty"`
}

type ChartVersions []ChartVersion

func (c ChartVersions) Len() int { return len(c) }

func (c ChartVersions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func (c ChartVersions) Less(a, b int) bool {
	i, err := semver.NewVersion(c[a].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(c[b].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}
