/* package URL handles Helm-DM URLs

Helm uses three kinds of URLs:

	- Fully qualified (long) names: https://example.com/foo/bar-1.2.3.tgz
	- Short names: helm:example.com/foo/bar#1.2.3
	- Local names: file:///foo/bar

This package provides utilities for working with this type of URL.
*/
package url

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
	SchemeHelm  = "helm"
	SchemeFile  = "file"
)

// TarNameRegex parses the name component of a URI and breaks it into a name and version.
//
// This borrows liberally from github.com/Masterminds/semver.
const TarNameRegex = `([0-9A-Za-z\-_/]+)-(v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?)(.tgz)?`

var tnregexp *regexp.Regexp

func init() {
	tnregexp = regexp.MustCompile("^" + TarNameRegex + "$")
}

type URL struct {
	// The scheme of the URL. Typically one of http, https, helm, or file.
	Scheme string
	// The host information, if applicable.
	Host string
	// The bucket name
	Bucket string
	// The chart name
	Name string
	// The version or version range.
	Version string

	// If this is a local chart, the path to the chart.
	LocalRef string

	isLocal bool

	original string
}

func Parse(path string) (*URL, error) {

	// Check for absolute or relative path.
	if path[0] == '.' || path[0] == '/' {
		return &URL{
			LocalRef: path,
			isLocal:  true,
			original: path,
		}, nil
	}

	// TODO: Do we want to support file:///foo/bar.tgz?
	if strings.HasPrefix(path, SchemeFile+":") {
		path := strings.TrimPrefix(path, SchemeFile+":")
		return &URL{
			LocalRef: filepath.Clean(path),
			isLocal:  true,
			original: path,
		}, nil
	}

	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	// Short name
	if u.Scheme == SchemeHelm {
		parts := strings.SplitN(u.Opaque, "/", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("both bucket and chart name are required in %s: %s", path, u.Path)
		}
		// Need to parse opaque data into bucket and chart.
		return &URL{
			Scheme:   u.Scheme,
			Host:     parts[0],
			Bucket:   parts[1],
			Name:     parts[2],
			Version:  u.Fragment,
			original: path,
		}, nil
	}

	// Long name
	parts := strings.SplitN(u.Path, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("both bucket and chart name are required in %s", path)
	}

	name, version, err := parseTarName(parts[2])
	if err != nil {
		return nil, err
	}

	return &URL{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Bucket:   parts[1],
		Name:     name,
		Version:  version,
		original: path,
	}, nil
}

// IsLocal returns true if this is a local path.
func (u *URL) IsLocal() bool {
	return u.isLocal
}

// Local returns a local version of the path.
//
// This will return an error if the URL does not reference a local chart.
func (u *URL) Local() (string, error) {
	return u.LocalRef, nil
}

var ErrLocal = errors.New("cannot use local URL as remote")
var ErrRemote = errors.New("cannot use remote URL as local")

// Short returns a short form URL.
//
// This will return an error if the URL references a local chart.
func (u *URL) Short() (string, error) {
	if u.IsLocal() {
		return "", ErrLocal
	}
	fname := fmt.Sprintf("%s/%s/%s", u.Host, u.Bucket, u.Name)
	return (&url.URL{
		Scheme:   SchemeHelm,
		Opaque:   fname,
		Fragment: u.Version,
	}).String(), nil
}

// Long returns a long-form URL.
//
// If secure is true, this will return an HTTPS URL, otherwise HTTP.
//
// This will return an error if the URL references a local chart.
func (u *URL) Long(secure bool) (string, error) {
	if u.IsLocal() {
		return "", ErrLocal
	}

	scheme := SchemeHTTPS
	if !secure {
		scheme = SchemeHTTP
	}
	fname := fmt.Sprintf("%s/%s-%s.tgz", u.Bucket, u.Name, u.Version)

	return (&url.URL{
		Scheme: scheme,
		Host:   u.Host,
		Path:   fname,
	}).String(), nil

}

// parseTarName parses a long-form tarfile name.
func parseTarName(name string) (string, string, error) {
	if strings.HasSuffix(name, ".tgz") {
		name = strings.TrimSuffix(name, ".tgz")
	}
	v := tnregexp.FindStringSubmatch(name)
	if v == nil {
		return name, "", fmt.Errorf("invalid name %s", name)
	}
	return v[1], v[2], nil
}
