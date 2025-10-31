// Package ui exposes utilities to get a Handler for the Grafana Alloy UI.
package ui

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/common/server"
)

// RegisterRoutes registers routes to the provided mux.Router for serving the
// Grafana Alloy UI. The UI will be served relative to pathPrefix. If no
// pathPrefix is specified, the UI will be served at root.
//
// By default, the UI is retrieved from the ./internal/web/ui/dist directory
// relative to working directory, assuming that Alloy is run from the repo
// root. However, if the builtinassets Go tag is present, the built UI will be
// embedded into the binary; run go generate -tags builtinassets for this
// package to generate the assets to embed.
//
// RegisterRoutes catches all requests from pathPrefix and so should only be
// invoked after all other routes have been registered.
//
// RegisterRoutes is not intended for public use and will only work properly
// when called from github.com/grafana/alloy.
func RegisterRoutes(pathPrefix string, router *mux.Router) {
	if !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix = pathPrefix + "/"
	}

	renderer := &templateRenderer{
		pathPrefix: strings.TrimSuffix(pathPrefix, "/"),
		inner:      Assets(),

		contentCache:     make(map[string]string),
		contentCacheTime: make(map[string]time.Time),
	}

	// Redirect root without trailing slash to version with slash
	router.HandleFunc(strings.TrimSuffix(pathPrefix, "/"), func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, pathPrefix, http.StatusFound)
	})

	// Serve all static files from dist, with SPA fallback to index.html
	router.PathPrefix(pathPrefix).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip the prefix to get the file path
		filePath := strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(pathPrefix, "/"))
		if filePath == "" {
			filePath = "/"
		}

		// Try to serve the requested file
		f, err := renderer.Open(filePath)
		if err == nil {
			// File exists, serve it
			defer f.Close()
			r.URL.Path = filePath
			server.StaticFileServer(renderer).ServeHTTP(w, r)
			return
		}

		// File not found, serve index.html for SPA routing
		r.URL.Path = "/"
		server.StaticFileServer(renderer).ServeHTTP(w, r)
	})
}

// templateRenderer wraps around an inner fs.FS and will use html/template to
// render files it serves. Files will be cached after rendering to save on CPU
// time between repeated requests.
//
// The templateRenderer is used to inject runtime variables into the statically
// built UI, such as the base URL path where the UI is exposed.
type templateRenderer struct {
	pathPrefix string
	inner      http.FileSystem

	cacheMut         sync.RWMutex
	contentCache     map[string]string
	contentCacheTime map[string]time.Time
}

var _ http.FileSystem = (*templateRenderer)(nil)

func (tr *templateRenderer) Open(name string) (http.File, error) {
	// First, open the inner file.
	f, err := tr.inner.Open(name)
	if err != nil {
		return nil, err
	}
	// Get the modification time of the file. This will be used to determine if
	// our cache is stale.
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	// Return the underlying file if we got a directory. Otherwise, we're going
	// to create our own synthetic file.
	//
	// When we create a synthetic file, we close the original file, f, on
	// return. Otherwise, we leave f open on return so the caller can read and
	// close it.
	if fi.IsDir() {
		return f, nil
	}
	defer f.Close()

	// Return the existing cache entry if one exists.
	if ent, ok := tr.getCacheEntry(name, fi, true); ok {
		return ent, nil
	}

	tr.cacheMut.Lock()
	defer tr.cacheMut.Unlock()

	// Check to see if another goroutine happened to cache the file while we were
	// waiting for the lock.
	if ent, ok := tr.getCacheEntry(name, fi, false); ok {
		return ent, nil
	}

	rawBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading file for template processing: %w", err)
	}
	tmpl, err := template.New(name).Delims("{{!", "!}}").Parse(string(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	// Render the file as an html/template.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ PublicURL string }{PublicURL: tr.pathPrefix}); err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}

	tr.contentCache[name] = buf.String()
	tr.contentCacheTime[name] = fi.ModTime()

	return &readerFile{
		Reader: bytes.NewReader(buf.Bytes()),
		fi: &infoWithSize{
			FileInfo: fi,
			size:     int64(buf.Len()),
		},
	}, nil
}

func (tr *templateRenderer) getCacheEntry(name string, fi fs.FileInfo, lock bool) (f http.File, ok bool) {
	if lock {
		tr.cacheMut.RLock()
		defer tr.cacheMut.RUnlock()
	}

	content, ok := tr.contentCache[name]
	if !ok {
		return nil, false
	}

	// Before returning, make sure that fi isn't newer than our cache time.
	if fi.ModTime().After(tr.contentCacheTime[name]) {
		// The file has changed since we cached it. It needs to be re-cached. This
		// is only common to happen during development, but would rarely happen in
		// production where the files are static.
		return nil, false
	}

	return &readerFile{
		Reader: strings.NewReader(content),
		fi: &infoWithSize{
			FileInfo: fi,
			size:     int64(len(content)),
		},
	}, true
}

type readerFile struct {
	io.Reader
	fi fs.FileInfo
}

var _ http.File = (*readerFile)(nil)

func (rf *readerFile) Stat() (fs.FileInfo, error) { return rf.fi, nil }

func (rf *readerFile) Close() error {
	// no-op: nothing to close
	return nil
}

// http.Filesystem expects that io.Seeker and Readdir is implemented for all
// http.File implementations.
//
// These don't need to do anything; http also contains an adapter for fs.FS to
// http.FileSystem (http.FS) where these two methods can be a no-op.

func (rf *readerFile) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("Seek not implemented")
}

func (rf *readerFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, fmt.Errorf("Readdir not implemented")
}

type infoWithSize struct {
	fs.FileInfo
	size int64
}

func (iws *infoWithSize) Size() int64 { return iws.size }
