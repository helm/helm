/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// GenericClient provides low-level OCI operations parameterised by artifact type
// rather than hardcoded to one artifact. Its diagnostics are not: index selection
// reports what it rejected in the vocabulary of Helm charts.
type GenericClient struct {
	debug              bool
	enableCache        bool
	credentialsFile    string
	username           string
	password           string
	out                io.Writer
	authorizer         *auth.Client
	registryAuthorizer RemoteClient
	credentialsStore   credentials.Store
	httpClient         *http.Client
	plainHTTP          bool
}

// GenericPullOptions configures a generic pull operation
type GenericPullOptions struct {
	// MediaTypes to include in the pull (empty means all)
	AllowedMediaTypes []string
	// Skip descriptors with these media types
	SkipMediaTypes []string
	// Custom PreCopy function for filtering
	PreCopy func(context.Context, ocispec.Descriptor) error
	// ArtifactType to select from OCI Image Index (empty means no filtering).
	// When pulling from an Image Index containing multiple manifests,
	// this field is used to select the manifest with matching artifactType.
	ArtifactType string
	// Selectors disambiguate when more than one manifest in an Image Index matches
	// ArtifactType. Two keys are honoured and the rest are ignored:
	// org.opencontainers.image.title and org.opencontainers.image.version. Either
	// annotation may be absent, and the missing one is then read out of the
	// candidate's own config blob.
	//
	// Both choose between candidates rather than validate one: a reference that
	// resolves to a single chart yields that chart whatever was asked for, matching
	// what a reference to a plain manifest already does, and the version is consulted
	// only once a name has left more than one candidate standing. Selectors are
	// consulted for a single candidate in one case only, when entries were dropped
	// and being the last one standing is therefore not a fact about the index.
	Selectors map[string]string
	// ParseIdentity reads a name and version out of an artifact's config blob. It is
	// what lets selection fall back when an index descriptor carries no annotations,
	// and it is the only place an artifact's own format is known here. Without it,
	// selection matches on descriptor annotations alone.
	ParseIdentity func(config []byte) (name, version string, err error)
}

// GenericPullResult contains the result of a generic pull operation
type GenericPullResult struct {
	Manifest    ocispec.Descriptor
	Descriptors []ocispec.Descriptor
	MemoryStore *memory.Store
	Ref         string
}

// NewGenericClient creates a new generic OCI client from an existing Client
func NewGenericClient(client *Client) *GenericClient {
	return &GenericClient{
		debug:              client.debug,
		enableCache:        client.enableCache,
		credentialsFile:    client.credentialsFile,
		username:           client.username,
		password:           client.password,
		out:                client.out,
		authorizer:         client.authorizer,
		registryAuthorizer: client.registryAuthorizer,
		credentialsStore:   client.credentialsStore,
		httpClient:         client.httpClient,
		plainHTTP:          client.plainHTTP,
	}
}

// maxIndexDepth is how many indexes deep the search follows, counting from the one
// the reference resolves to. A multi-arch image listed beside a chart is depth one,
// which is what tooling produces today, and an index aggregating such indexes is
// depth two. Three leaves a level beyond anything a publisher assembles. Past that
// the cost is one sequential request per level against a chain the registry is free
// to make up as it is walked.
const maxIndexDepth = 3

// nestedIndex is an index listed inside another one, carried with the depth it was
// found at, since that is what the limit above is applied to.
type nestedIndex struct {
	desc  ocispec.Descriptor
	depth int
}

// dockerManifestListMediaType is what Docker Hub and older buildx write where the
// OCI spec writes an image index. It lists manifests the same way, so an entry
// carrying it holds artifacts the same way too.
const dockerManifestListMediaType = "application/vnd.docker.distribution.manifest.list.v2+json"

// dockerManifestMediaType is the same story one level down: a manifest copied
// between registries by tooling that writes the Docker schema describes its config
// and layers exactly as an image manifest does.
const dockerManifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"

// listsManifests reports whether an entry holds other entries rather than an
// artifact. Such an entry is never a candidate, since an artifact is identified by
// the config blob its manifest carries and a list has none, and it is searched
// rather than dropped, since it can hold the artifact that was asked for.
func listsManifests(mediaType string) bool {
	return mediaType == ocispec.MediaTypeImageIndex || mediaType == dockerManifestListMediaType
}

// maxEntryBytes bounds what the pass below reads out of an entry it was not asked
// for. The size is the index's own claim about content the registry serves, and a
// manifest that large is not a manifest, so an entry declaring more is left unread
// instead of streamed. The figure is what oras allows a manifest to be. Reading a
// candidate's own identity is bounded by oras instead, which caps what it allocates
// ahead of the content and verifies size and digest against it.
const maxEntryBytes = 4 * 1024 * 1024

// entryBody is what an entry turns out to hold once read, which is what decides how
// it is treated: a config makes it an artifact and a list of manifests makes it a
// container of artifacts. An entry declared as a manifest that holds no config is
// read as neither; one declared as a list that holds a config is too, while a list
// that holds nothing is a list, and searching it finds nothing to find.
type entryBody struct {
	Config    ocispec.Descriptor   `json:"config"`
	Manifests []ocispec.Descriptor `json:"manifests"`
}

// resolveFromIndex selects one manifest from an OCI Image Index by artifactType.
// When nothing matches on artifactType it retries over the entries that declare
// none, matching their config mediaType instead, which is how indexes written
// before artifactType existed are still resolvable. More than one match is
// narrowed by the selectors; a choice they cannot settle is an error rather than
// a guess.
func resolveFromIndex(ctx context.Context, fetcher content.Fetcher, indexDesc ocispec.Descriptor, artifactType string, selectors map[string]string, parse func(config []byte) (name, version string, err error)) (ocispec.Descriptor, error) {
	// Fetch the index manifest
	indexData, err := content.FetchAll(ctx, fetcher, indexDesc)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to fetch image index: %w", err)
	}

	var index ocispec.Index
	if err := json.Unmarshal(indexData, &index); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("unable to parse image index: %w", err)
	}

	// First pass: entries that declare the artifact type. A declared type is taken
	// at face value even on a descriptor that also carries a platform, since tooling
	// that stamps a platform on every index entry would otherwise hide the artifact.
	var candidates []ocispec.Descriptor
	var availableTypes []string
	var unmatched []ocispec.Descriptor
	var nested []nestedIndex
	var skipped []error
	for _, manifest := range index.Manifests {
		// Set aside to be searched, per listsManifests: taken as a candidate the pull
		// fails on a manifest it never stored, dropped it takes the chart it holds with
		// it, and the chart beside it answers instead.
		if listsManifests(manifest.MediaType) {
			nested = append(nested, nestedIndex{desc: manifest, depth: 1})
			continue
		}
		if manifest.ArtifactType == artifactType {
			candidates = append(candidates, manifest)
			continue
		}
		if manifest.ArtifactType != "" {
			availableTypes = append(availableTypes, manifest.ArtifactType)
		}
		unmatched = append(unmatched, manifest)
	}

	wantName := selectors[ocispec.AnnotationTitle]
	wantVersion := selectors[ocispec.AnnotationVersion]

	// Second pass: the indexes an entry points at, and every entry whose declaration
	// did not match, read for the config mediaType its manifest carries. An index
	// written before artifactType existed declares nothing, and a builder that sets
	// the field wrongly declares something else; both leave the config as the only
	// place the artifact says what it is, so both are read here rather than only the
	// first. It runs when the first pass found nothing, and also when no single entry
	// it found announces the whole requested name and version: narrowing the
	// candidates before either is checked is how a mixed index answers with the wrong
	// chart. The cost is one fetch per entry that is not already a candidate, and it
	// is not confined to failures: an index builder may set artifactType without
	// copying the manifest annotations onto the descriptor, and then the declared
	// entries announce nothing, so pulls that go on to succeed pay it as well.
	configCache := map[string]ocispec.Descriptor{}
	// Counted rather than taken from the length of the queue: an index may list the
	// same manifest twice and the repeats are not read again, so the queue says how
	// many entries were queued and this says how many were looked at.
	read := 0
	if len(candidates) == 0 || !announcesChart(candidates, wantName, wantVersion) {
		// An index an entry points at is searched the same way the top one is, so a
		// chart keeps its place wherever the publisher nested it. Entries already seen
		// are not read twice, which is also what stops an index that lists itself.
		seen := map[string]bool{indexDesc.Digest.String(): true}
		for i := 0; i < len(nested); i++ {
			entry := nested[i]
			if seen[entry.desc.Digest.String()] {
				continue
			}
			seen[entry.desc.Digest.String()] = true
			if entry.depth > maxIndexDepth {
				skipped = append(skipped, fmt.Errorf(
					"%s: nested more than %d indexes deep and was not searched", entry.desc.Digest, maxIndexDepth))
				continue
			}
			if entry.desc.Size > maxEntryBytes {
				skipped = append(skipped, fmt.Errorf(
					"%s: entry declares %d bytes, more than a manifest is, and was not read", entry.desc.Digest, entry.desc.Size))
				continue
			}
			indexData, err := content.FetchAll(ctx, fetcher, entry.desc)
			if err != nil {
				skipped = append(skipped, fmt.Errorf("%s: %w", entry.desc.Digest, err))
				continue
			}
			var sub entryBody
			if err := json.Unmarshal(indexData, &sub); err != nil {
				skipped = append(skipped, fmt.Errorf("%s: %w", entry.desc.Digest, err))
				continue
			}
			if len(sub.Manifests) == 0 && sub.Config.MediaType != "" {
				skipped = append(skipped, fmt.Errorf(
					"%s: entry declares a list of manifests and holds a config instead, and was searched as neither", entry.desc.Digest))
				continue
			}
			for _, manifest := range sub.Manifests {
				switch {
				case listsManifests(manifest.MediaType):
					nested = append(nested, nestedIndex{desc: manifest, depth: entry.depth + 1})
				case manifest.ArtifactType == artifactType:
					candidates = append(candidates, manifest)
				default:
					if manifest.ArtifactType != "" {
						availableTypes = append(availableTypes, manifest.ArtifactType)
					}
					unmatched = append(unmatched, manifest)
				}
			}
		}
		for _, candidate := range unmatched {
			if seen[candidate.Digest.String()] {
				continue
			}
			seen[candidate.Digest.String()] = true
			if candidate.Size > maxEntryBytes {
				skipped = append(skipped, fmt.Errorf(
					"%s: entry declares %d bytes, more than a manifest is, and was not read", candidate.Digest, candidate.Size))
				continue
			}
			manifestData, err := content.FetchAll(ctx, fetcher, candidate)
			if err != nil {
				skipped = append(skipped, fmt.Errorf("%s: %w", candidate.Digest, err))
				continue
			}
			var body entryBody
			if err := json.Unmarshal(manifestData, &body); err != nil {
				skipped = append(skipped, fmt.Errorf("%s: %w", candidate.Digest, err))
				continue
			}
			if body.Config.MediaType == "" {
				skipped = append(skipped, fmt.Errorf(
					"%s: entry declares a manifest and holds no config, and was read as neither", candidate.Digest))
				continue
			}
			read++
			if body.Config.MediaType == artifactType {
				candidates = append(candidates, candidate)
				configCache[candidate.Digest.String()] = body.Config
			}
		}
	}

	// An index may legally list the same manifest twice. Left in, the repeats would
	// be counted as separate candidates and one chart would be called ambiguous with
	// itself, so they are collapsed before anything downstream counts them.
	candidates = dedupeByDigest(candidates)

	// Every answer below rests on nothing else in the index matching. Absence is a
	// fact about the index only while every entry could be read; once entries have
	// been dropped it is a fact about the pass instead. So an incomplete pass may
	// not hand back a candidate that merely survived, and its negative answers have
	// to admit what they could not see. Reading an identity can fail too, and those
	// failures join the same record, which is why it is consulted where it is used
	// rather than captured once.

	// A multi-arch image repeats one artifactType per platform; listing it once is
	// the whole content of the message.
	slices.Sort(availableTypes)
	availableTypes = slices.Compact(availableTypes)

	switch len(candidates) {
	case 0:
		if len(skipped) > 0 {
			return ocispec.Descriptor{}, fmt.Errorf(
				"no manifest with artifactType %q found in image index; available types: %v; %d entries could not be read: %v",
				artifactType, availableTypes, len(skipped), skipped)
		}
		readNote := fmt.Sprintf("the manifests of %d other entries were read and none declares a chart config", read)
		if read == 1 {
			readNote = "the manifest of the one other entry was read and it declares no chart config"
		}
		return ocispec.Descriptor{}, fmt.Errorf(
			"no manifest with artifactType %q found in image index; available types: %v; %s",
			artifactType, availableTypes, readNote)
	case 1:
		// One candidate is taken without checking the name against it. Requiring a
		// match would break publishing a chart under a repository named differently
		// from the chart, which is legal and common; there is also nothing to choose
		// between, so a wrong name here means the reference itself was wrong.
		//
		// That reasoning holds only while the candidate set is complete. Once entries
		// have been dropped, "nothing to choose between" describes what could be read
		// rather than what the index holds, and the survivor has to earn the answer.
		// With nothing skipped the set is complete and this is the only chart in the
		// index, so it answers whatever name was asked for. Only an incomplete pass
		// makes that reasoning unsound, and only then is the identity worth reading.
		if len(skipped) > 0 && wantName != "" {
			name, version, idErr := descriptorIdentity(ctx, fetcher, candidates[0], configCache, parse)
			switch {
			case idErr != nil:
				return ocispec.Descriptor{}, fmt.Errorf(
					"the identity of the only remaining chart in the image index could not be read: %w; %d other entries could not be read either: %v",
					idErr, len(skipped), skipped)
			case name != wantName || (wantVersion != "" && version != wantVersion):
				return ocispec.Descriptor{}, fmt.Errorf(
					"the only readable chart in the image index is %q version %q, and the reference asks for %q version %q; %d entries could not be read, so it cannot be told whether the requested one is among them: %v",
					name, version, wantName, wantVersion, len(skipped), skipped)
			}
		}
		return candidates[0], nil
	}

	// More than one chart in the index: the name is required to disambiguate and
	// the version breaks a remaining tie. Every error below lists the candidates,
	// because the caller cannot see the index and has no other way to find out
	// what it would have to pick between.
	resolved := make([]chartCandidate, 0, len(candidates))
	for _, d := range candidates {
		name, version, idErr := descriptorIdentity(ctx, fetcher, d, configCache, parse)
		if idErr != nil {
			// An identity that could not be read matches nothing below, so losing it
			// silently would turn "this chart is not here" into a claim about the
			// index that only describes the read.
			skipped = append(skipped, idErr)
		}
		resolved = append(resolved, chartCandidate{desc: d, name: name, version: version, idErr: idErr})
	}

	if wantName == "" {
		return ocispec.Descriptor{}, fmt.Errorf(
			"image index holds %d charts and no chart name was given to disambiguate; candidates: %s",
			len(resolved), describeCandidates(resolved))
	}

	var named []chartCandidate
	for _, cand := range resolved {
		if cand.name == wantName {
			named = append(named, cand)
		}
	}
	switch len(named) {
	case 0:
		if len(skipped) > 0 {
			return ocispec.Descriptor{}, fmt.Errorf(
				"no chart named %q among the %d found in the image index; %d entries could not be read: %v; candidates: %s",
				wantName, len(resolved), len(skipped), skipped, describeCandidates(resolved))
		}
		return ocispec.Descriptor{}, fmt.Errorf(
			"image index holds %d charts and none is named %q; candidates: %s",
			len(resolved), wantName, describeCandidates(resolved))
	case 1:
		if len(skipped) > 0 && wantVersion != "" && named[0].version != wantVersion {
			if named[0].idErr != nil {
				return ocispec.Descriptor{}, fmt.Errorf(
					"the version of the only chart named %q could not be read: %w; %d entries could not be read: %v",
					wantName, named[0].idErr, len(skipped), skipped)
			}
			return ocispec.Descriptor{}, fmt.Errorf(
				"the only readable chart named %q is version %q, not %q; %d entries could not be read: %v",
				wantName, named[0].version, wantVersion, len(skipped), skipped)
		}
		return named[0].desc, nil
	}

	// Several charts share the requested name: break the tie by version.
	if wantVersion != "" {
		var versioned []chartCandidate
		for _, cand := range named {
			if cand.version == wantVersion {
				versioned = append(versioned, cand)
			}
		}
		switch len(versioned) {
		case 1:
			return versioned[0].desc, nil
		case 0:
			if len(skipped) > 0 {
				return ocispec.Descriptor{}, fmt.Errorf(
					"cannot choose between the %d charts named %q, and none is version %q; %d entries could not be read: %v; candidates: %s",
					len(named), wantName, wantVersion, len(skipped), skipped, describeCandidates(named))
			}
			return ocispec.Descriptor{}, fmt.Errorf(
				"cannot choose between the %d charts named %q, and none is version %q; candidates: %s",
				len(named), wantName, wantVersion, describeCandidates(named))
		default:
			return ocispec.Descriptor{}, fmt.Errorf(
				"image index holds %d entries for %s:%s; they can only be told apart by digest: %s",
				len(versioned), wantName, wantVersion, describeDigests(versioned))
		}
	}

	return ocispec.Descriptor{}, fmt.Errorf(
		"image index is ambiguous: %d charts named %q; specify a version; candidates: %s",
		len(named), wantName, describeCandidates(named))
}

// dedupeByDigest keeps the first entry for each digest, preserving index order.
func dedupeByDigest(descs []ocispec.Descriptor) []ocispec.Descriptor {
	seen := make(map[string]bool, len(descs))
	out := make([]ocispec.Descriptor, 0, len(descs))
	for _, d := range descs {
		key := d.Digest.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out
}

// describeDigests renders candidates as digests, for the one case where their
// name and version are identical and nothing else tells them apart.
func describeDigests(candidates []chartCandidate) string {
	parts := make([]string, 0, len(candidates))
	for _, c := range candidates {
		parts = append(parts, c.desc.Digest.String())
	}
	return strings.Join(parts, ", ")
}

// announcesChart reports whether any single descriptor in the index announces the
// whole requested identity. Asking per attribute instead would let one candidate supply
// the name and another the version, and the pair they form belongs to neither.
// It only reads what the index already states: an identity that is knowable but
// not announced answers false, which costs the second pass a fetch it did not
// strictly need and never costs a wrong answer.
func announcesChart(descs []ocispec.Descriptor, name, version string) bool {
	return slices.ContainsFunc(descs, func(d ocispec.Descriptor) bool {
		if name != "" && d.Annotations[ocispec.AnnotationTitle] != name {
			return false
		}
		return version == "" || d.Annotations[ocispec.AnnotationVersion] == version
	})
}

// chartCandidate pairs an index descriptor with the chart identity used to
// disambiguate it.
type chartCandidate struct {
	desc    ocispec.Descriptor
	name    string
	version string
	// idErr is set when the identity could not be read. Kept on the candidate so a
	// message about it cannot claim a name or version that was never fetched.
	idErr error
}

// descriptorIdentity returns a candidate's name and version, preferring the
// descriptor annotations and asking parse to read whichever of the two they omit
// out of the config blob. Both are needed: a name alone cannot break a tie between
// two candidates sharing it. Without a parser only the annotations are available.
func descriptorIdentity(ctx context.Context, fetcher content.Fetcher, desc ocispec.Descriptor, configCache map[string]ocispec.Descriptor, parse func(config []byte) (name, version string, err error)) (name, version string, err error) {
	name = desc.Annotations[ocispec.AnnotationTitle]
	version = desc.Annotations[ocispec.AnnotationVersion]
	if (name != "" && version != "") || parse == nil {
		return name, version, nil
	}

	// Resolve the config descriptor, reusing the one captured during the fallback
	// pass when present so the manifest is not fetched a second time.
	config, ok := configCache[desc.Digest.String()]
	if !ok {
		manifestData, ferr := content.FetchAll(ctx, fetcher, desc)
		if ferr != nil {
			return name, version, ferr
		}
		var manifest ocispec.Manifest
		if uerr := json.Unmarshal(manifestData, &manifest); uerr != nil {
			return name, version, fmt.Errorf("%s: %w", desc.Digest, uerr)
		}
		config = manifest.Config
	}
	configData, ferr := content.FetchAll(ctx, fetcher, config)
	if ferr != nil {
		return name, version, ferr
	}
	parsedName, parsedVersion, perr := parse(configData)
	if perr != nil {
		return name, version, fmt.Errorf("%s: %w", desc.Digest, perr)
	}
	if name == "" {
		name = parsedName
	}
	if version == "" {
		version = parsedVersion
	}
	return name, version, nil
}

// describeCandidates renders chart candidates as name:version, falling back to
// the digest, for disambiguation error messages. A candidate whose identity could
// not be read is marked as such: rendering it like one that simply carries no
// version would state as absent what was never looked at.
func describeCandidates(candidates []chartCandidate) string {
	parts := make([]string, 0, len(candidates))
	for _, c := range candidates {
		switch {
		case c.idErr != nil && c.name == "":
			parts = append(parts, c.desc.Digest.String()+" (identity unreadable)")
		case c.idErr != nil:
			parts = append(parts, c.name+" (version unreadable)")
		case c.name == "":
			parts = append(parts, c.desc.Digest.String())
		case c.version == "":
			parts = append(parts, c.name)
		default:
			parts = append(parts, c.name+":"+c.version)
		}
	}
	return strings.Join(parts, ", ")
}

// PullGeneric performs an OCI pull parameterised by artifact type.
func (c *GenericClient) PullGeneric(ref string, options GenericPullOptions) (*GenericPullResult, error) {
	parsedRef, err := newReference(ref)
	if err != nil {
		return nil, err
	}

	memoryStore := memory.New()
	var descriptors []ocispec.Descriptor

	// Set up a repository with authentication and configuration
	repository, err := remote.NewRepository(parsedRef.String())
	if err != nil {
		return nil, err
	}
	repository.PlainHTTP = c.plainHTTP
	repository.Client = c.authorizer

	ctx := context.Background()

	// Prepare allowed media types for filtering
	var allowedMediaTypes []string
	if len(options.AllowedMediaTypes) > 0 {
		allowedMediaTypes = make([]string, len(options.AllowedMediaTypes))
		copy(allowedMediaTypes, options.AllowedMediaTypes)
		sort.Strings(allowedMediaTypes)
	}

	var mu sync.Mutex
	copyOptions := oras.CopyOptions{
		CopyGraphOptions: oras.CopyGraphOptions{
			PreCopy: func(ctx context.Context, desc ocispec.Descriptor) error {
				// Apply a custom PreCopy function if provided
				if options.PreCopy != nil {
					if err := options.PreCopy(ctx, desc); err != nil {
						return err
					}
				}

				mediaType := desc.MediaType

				// Skip media types if specified
				if slices.Contains(options.SkipMediaTypes, mediaType) {
					return oras.SkipNode
				}

				// Filter by allowed media types if specified
				if len(allowedMediaTypes) > 0 {
					if i := sort.SearchStrings(allowedMediaTypes, mediaType); i >= len(allowedMediaTypes) || allowedMediaTypes[i] != mediaType {
						return oras.SkipNode
					}
				}

				mu.Lock()
				descriptors = append(descriptors, desc)
				mu.Unlock()
				return nil
			},
		},
	}

	// Select inside the copy rather than before it. oras resolves the root itself
	// and hands it to MapRoot through a proxy that still serves the body it already
	// read, so an index is recognised and narrowed without a request of our own.
	if options.ArtifactType != "" {
		copyOptions.MapRoot = func(ctx context.Context, src content.ReadOnlyStorage, root ocispec.Descriptor) (ocispec.Descriptor, error) {
			if root.MediaType != ocispec.MediaTypeImageIndex {
				return root, nil
			}
			selected, err := resolveFromIndex(ctx, src, root, options.ArtifactType, options.Selectors, options.ParseIdentity)
			if err != nil {
				return ocispec.Descriptor{}, err
			}
			// The copy about to start filters on the same media types the caller
			// allowed, and a root it will not store is dropped without an error of its
			// own: the pull then fails on a blob nothing ever wrote, naming neither the
			// entry nor why. Selection knows both, so it says so here instead.
			if len(allowedMediaTypes) > 0 {
				if i := sort.SearchStrings(allowedMediaTypes, selected.MediaType); i >= len(allowedMediaTypes) || allowedMediaTypes[i] != selected.MediaType {
					return ocispec.Descriptor{}, fmt.Errorf(
						"the chart selected from the image index is a %s, which this pull does not accept: %s",
						selected.MediaType, selected.Digest)
				}
			}
			return selected, nil
		}
	}

	manifest, err := oras.Copy(ctx, repository, parsedRef.String(), memoryStore, "", copyOptions)
	if err != nil {
		// oras wraps a MapRoot failure as its own copy error. The selection message
		// underneath is the part naming something the caller can act on.
		var copyErr *oras.CopyError
		if errors.As(err, &copyErr) && copyErr.Op == "MapRoot" {
			return nil, copyErr.Err
		}
		return nil, err
	}

	return &GenericPullResult{
		Manifest:    manifest,
		Descriptors: descriptors,
		MemoryStore: memoryStore,
		Ref:         parsedRef.String(),
	}, nil
}

// GetDescriptorData retrieves the data for a specific descriptor
func (c *GenericClient) GetDescriptorData(store *memory.Store, desc ocispec.Descriptor) ([]byte, error) {
	return content.FetchAll(context.Background(), store, desc)
}
