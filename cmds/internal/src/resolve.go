package src

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var shaRef = regexp.MustCompile(`^[0-9a-fA-F]{12,40}$`)

const acceptedSpecForms = "URL | host/org/repo | Go module@version | npm:<pkg>[@ver] | arch:<pkg>"

type source struct {
	kind     string
	url      string
	host     string
	repoPath string
	ref      string
	cacheRef string
	pinned   bool
	module   string
	version  string
}

func (a app) resolve(spec string) (source, error) {
	if spec == "" {
		return source{}, errors.New("empty source spec")
	}
	if after, ok := strings.CutPrefix(spec, "npm:"); ok {
		return a.resolveNPM(after)
	}
	if after, ok := strings.CutPrefix(spec, "arch:"); ok {
		pkg := after
		if pkg == "" {
			return source{}, errors.New("arch spec needs a package name")
		}
		return gitSource("https://gitlab.archlinux.org/archlinux/packaging/packages/"+pkg+".git", "default")
	}
	if hasSlashRef(spec) {
		return source{}, fmt.Errorf("git ref in %q contains /; slash refs are not supported", spec)
	}
	if looksLikeGoModule(spec) {
		module, version := splitRef(spec)
		return source{kind: "go", module: module, version: version}, nil
	}
	return gitSourceFromSpec(spec)
}

func looksLikeGoModule(spec string) bool {
	module, version := splitRef(spec)
	return module != spec && version != "default" && strings.Contains(module, ".") && !knownGitHost(strings.Split(module, "/")[0])
}

func knownGitHost(host string) bool {
	return host == "github.com" || host == "gitlab.com" || host == "gitlab.archlinux.org" || host == "codeberg.org"
}

func (a app) resolveNPM(spec string) (source, error) {
	name, version := splitNPM(spec)
	if name == "" {
		return source{}, errors.New("npm spec needs a package name")
	}
	meta, err := fetchNPM(name, version)
	if err != nil {
		return source{}, err
	}
	if meta.Repository.URL == "" {
		return source{}, fmt.Errorf("npm:%s has no repository.url", name)
	}
	return gitSource(cleanGitURL(meta.Repository.URL), "default")
}

type npmMeta struct {
	DistTags map[string]string `json:"dist-tags"`
	Versions map[string]struct {
		Repository struct {
			URL string `json:"url"`
		} `json:"repository"`
	} `json:"versions"`
	Repository struct {
		URL string `json:"url"`
	} `json:"repository"`
}

func fetchNPM(name, version string) (npmMeta, error) {
	escaped := strings.ReplaceAll(url.PathEscape(name), "%2F", "%2f")
	endpoint := "https://registry.npmjs.org/" + escaped
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return npmMeta{}, fmt.Errorf("fetch npm metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return npmMeta{}, fmt.Errorf("fetch npm metadata: %s", resp.Status)
	}
	var meta npmMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return npmMeta{}, fmt.Errorf("decode npm metadata: %w", err)
	}
	if version == "default" {
		version = meta.DistTags["latest"]
	}
	if version != "" && version != "default" {
		v, ok := meta.Versions[version]
		if !ok {
			return npmMeta{}, fmt.Errorf("npm:%s has no version %s", name, version)
		}
		meta.Repository = v.Repository
	}
	return meta, nil
}

func splitNPM(spec string) (string, string) {
	if strings.HasPrefix(spec, "@") {
		parts := strings.Split(spec, "/")
		if len(parts) < 2 {
			return spec, "default"
		}
		pkg := parts[0] + "/" + parts[1]
		if at := strings.LastIndex(pkg, "@"); at > 0 {
			return pkg[:at], pkg[at+1:]
		}
		if len(parts) > 2 {
			rest := strings.Join(parts[2:], "/")
			if after, ok := strings.CutPrefix(rest, "@"); ok {
				return pkg, after
			}
		}
		return pkg, "default"
	}
	return splitRef(spec)
}

func cleanGitURL(raw string) string {
	raw = strings.TrimPrefix(raw, "git+")
	raw = strings.TrimPrefix(raw, "git://")
	if !strings.Contains(raw, "://") && !strings.HasPrefix(raw, "git@") {
		raw = "https://" + raw
	}
	if strings.HasPrefix(raw, "github.com/") {
		raw = "https://" + raw
	}
	return strings.TrimSuffix(raw, "#readme")
}

func gitSourceFromSpec(spec string) (source, error) {
	base, ref := splitRef(spec)
	if bareName(base) {
		return source{}, invalidSourceSpec(spec)
	}
	src, err := gitSource(base, ref)
	if err != nil {
		return source{}, fmt.Errorf("%w; accepted forms: %s", err, acceptedSpecForms)
	}
	return src, nil
}

func gitSource(raw, ref string) (source, error) {
	if ref == "" {
		ref = "default"
	}
	host, repoPath, cloneURL, err := parseGit(raw)
	if err != nil {
		return source{}, err
	}
	cacheRef := ref
	pinned := ref != "default"
	if shaRef.MatchString(ref) {
		cacheRef = strings.ToLower(ref[:12])
	}
	if err := validateRef(cacheRef); err != nil {
		return source{}, err
	}
	return source{
		kind:     "git",
		url:      cloneURL,
		host:     host,
		repoPath: repoPath,
		ref:      ref,
		cacheRef: cacheRef,
		pinned:   pinned,
	}, nil
}

func parseGit(raw string) (string, string, string, error) {
	raw = cleanGitURL(raw)
	if after, ok := strings.CutPrefix(raw, "git@"); ok {
		rest := after
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("invalid git SSH URL %q", raw)
		}
		repoPath, err := cleanRepoPath(strings.TrimSuffix(parts[1], ".git"))
		if err != nil {
			return "", "", "", fmt.Errorf("invalid git repo path %q: %w", parts[1], err)
		}
		return parts[0], repoPath, raw, nil
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid git URL %q: %w", raw, err)
		}
		repoPath, pathErr := cleanURLRepoPath(u.Path)
		if pathErr != nil {
			return "", "", "", fmt.Errorf("invalid git URL path %q: %w", u.Path, pathErr)
		}
		if u.Host == "" {
			return "", "", "", fmt.Errorf("invalid git URL %q", raw)
		}
		cloneURL := raw
		if !strings.HasSuffix(cloneURL, ".git") {
			cloneURL += ".git"
		}
		return u.Host, repoPath, cloneURL, nil
	}
	parts := strings.Split(raw, "/")
	if len(parts) < 3 || !strings.Contains(parts[0], ".") {
		return "", "", "", fmt.Errorf("git shorthand needs host/org/repo, got %q", raw)
	}
	repoPath, err := cleanRepoPath(strings.TrimSuffix(strings.Join(parts[1:], "/"), ".git"))
	if err != nil {
		return "", "", "", fmt.Errorf("invalid git shorthand path %q: %w", strings.Join(parts[1:], "/"), err)
	}
	return parts[0], repoPath, "https://" + parts[0] + "/" + repoPath + ".git", nil
}

func invalidSourceSpec(spec string) error {
	return fmt.Errorf("invalid source spec %q; accepted forms: %s", spec, acceptedSpecForms)
}

func cleanURLRepoPath(raw string) (string, error) {
	return cleanRepoPath(strings.TrimPrefix(strings.TrimSuffix(raw, ".git"), "/"))
}

func cleanRepoPath(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("empty path")
	}
	if path.IsAbs(raw) {
		return "", errors.New("absolute path")
	}
	for segment := range strings.SplitSeq(raw, "/") {
		if segment == "" {
			return "", errors.New("empty path segment")
		}
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("invalid path segment %q", segment)
		}
	}
	return raw, nil
}

func validateRef(ref string) error {
	if ref == "" {
		return errors.New("empty ref")
	}
	if strings.Contains(ref, "/") {
		return fmt.Errorf("ref %q contains /; slash refs are not supported", ref)
	}
	return nil
}

func hasSlashRef(spec string) bool {
	at := strings.LastIndex(spec, "@")
	if at <= 0 || at < strings.LastIndex(spec, ":") {
		return false
	}
	return strings.Contains(spec[at+1:], "/")
}

func splitRef(spec string) (string, string) {
	if ref := refPart(spec); ref != "" {
		return strings.TrimSuffix(spec, ref), strings.TrimPrefix(ref, "@")
	}
	return spec, "default"
}

func refPart(spec string) string {
	at := strings.LastIndex(spec, "@")
	if at <= 0 {
		return ""
	}
	lastSlash := strings.LastIndex(spec, "/")
	lastColon := strings.LastIndex(spec, ":")
	if at < lastSlash || at < lastColon {
		return ""
	}
	return spec[at:]
}

func (s source) dest(root string) (string, error) {
	if _, err := cleanRepoPath(s.repoPath); err != nil {
		return "", fmt.Errorf("invalid repo path %q: %w", s.repoPath, err)
	}
	if err := validateRef(s.cacheRef); err != nil {
		return "", err
	}
	repo := path.Base(s.repoPath)
	dir := strings.TrimSuffix(repo, ".git") + "@" + s.cacheRef
	parent := path.Dir(s.repoPath)
	var dest string
	if parent == "." {
		dest = filepath.Join(root, s.host, dir)
	} else {
		dest = filepath.Join(root, s.host, filepath.FromSlash(parent), dir)
	}
	return cleanDestUnderRoot(root, dest)
}

func cleanDestUnderRoot(root, dest string) (string, error) {
	cleanRoot := filepath.Clean(root)
	cleanDest := filepath.Clean(dest)
	rel, err := filepath.Rel(cleanRoot, cleanDest)
	if err != nil {
		return "", fmt.Errorf("verify cache dest: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("cache dest %q escapes cache root %q", cleanDest, cleanRoot)
	}
	return cleanDest, nil
}
