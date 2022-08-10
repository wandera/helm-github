package helm

type Metadata struct {
	Name         string            `json:"name,omitempty"`
	Home         string            `json:"home,omitempty"`
	Sources      []string          `json:"sources,omitempty"`
	Version      string            `json:"version,omitempty"`
	Description  string            `json:"description,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Maintainers  []*Maintainer     `json:"maintainers,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	APIVersion   string            `json:"apiVersion,omitempty"`
	Condition    string            `json:"condition,omitempty"`
	Tags         string            `json:"tags,omitempty"`
	AppVersion   string            `json:"appVersion,omitempty"`
	Deprecated   bool              `json:"deprecated,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	KubeVersion  string            `json:"kubeVersion,omitempty"`
	Dependencies []*Dependency     `json:"dependencies,omitempty"`
	Type         string            `json:"type,omitempty"`
}

type Maintainer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

type Dependency struct {
	Name         string        `json:"name"`
	Version      string        `json:"version,omitempty"`
	Repository   string        `json:"repository"`
	Condition    string        `json:"condition,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	Enabled      bool          `json:"enabled,omitempty"`
	ImportValues []interface{} `json:"import-values,omitempty"`
	Alias        string        `json:"alias,omitempty"`
}
