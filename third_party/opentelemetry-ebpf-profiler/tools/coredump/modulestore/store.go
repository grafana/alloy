// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// The modulestore package implements `Store`, a storage for large binary files (modules).
// For more information, please refer to the documentation on the `Store` type.

package modulestore // import "go.opentelemetry.io/ebpf-profiler/tools/coredump/modulestore"

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/readatbuf"
	zstpak "go.opentelemetry.io/ebpf-profiler/tools/zstpak/lib"
)

const (
	// localTempSuffix specifies the prefix appended to files in the local storage while they are
	// still being written to.
	localTempPrefix = "tmp."
	// s3KeyPrefix defines the prefix prepended to all S3 keys.
	s3KeyPrefix = "module-store/"
	// s3ResultsPerPage defines how many results to request per page when listing objects.
	s3ResultsPerPage = 1000
	// s3MaxPages defines the maximum number of pages to ever retrieve when listing objects.
	s3MaxPages = 16
	// zstpakChunkSize determines the chunk size to use when compressing files.
	zstpakChunkSize = 64 * 1024
)

// Store is a compressed storage for large binary files (modules). Upon inserting a new file, the
// caller receives a unique ID to identify the file by. This ID can then later be used to retrieve
// the module again. Files are transparently compressed upon insertion and lazily decompressed
// during reading. Modules can be pushed to a remote backing storage in the form of an S3 bucket.
// Modules present remotely but not locally are automatically downloaded when needed.
//
// It is safe to create multiple `Store` instances for the same local directory and remote bucket
// at the same time, also when created within multiple different applications.
type Store struct {
	s3client       *s3.Client
	httpClient     *http.Client
	publicReadURL  string
	bucket         string
	localCachePath string
}

// New creates a new module storage. The modules present in the local cache are inspected and a
// full index of the modules in the remote S3 bucket is retrieved and cached as well.
func New(s3client *s3.Client, publicReadURL, s3Bucket, localCachePath string) (*Store, error) {
	if err := os.MkdirAll(localCachePath, 0o750); err != nil {
		return nil, err
	}
	tr := &http.Transport{
		MaxIdleConns:       2,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	return &Store{
		s3client:       s3client,
		httpClient:     &http.Client{Transport: tr},
		publicReadURL:  publicReadURL,
		bucket:         s3Bucket,
		localCachePath: localCachePath,
	}, nil
}

// InsertModuleLocally places a file into the local cache, returning an ID to refer it by in the
// future. The file is **not** uploaded to the remote storage automatically. If the file was already
// previously present in the local store, the function returns the ID of the existing file.
func (store *Store) InsertModuleLocally(localPath string) (id ID, isNew bool, err error) {
	var in *os.File
	in, err = os.Open(localPath)
	if err != nil {
		return ID{}, false, fmt.Errorf("failed to open local file: %w", err)
	}

	id, err = calculateModuleID(in)
	if err != nil {
		return ID{}, false, err
	}
	_, err = in.Seek(0, io.SeekStart)
	if err != nil {
		return ID{}, false, errors.New("failed to seek file back to start")
	}

	present, err := store.IsPresentLocally(id)
	if err != nil {
		return ID{}, false, fmt.Errorf("failed to check whether the module exists locally: %w", err)
	}
	if present {
		return id, false, nil
	}

	// We first write the file with a suffix marking it as temporary, to prevent half-written
	// files to persist in the local cache on crashes.
	storePath := store.makeLocalPath(id)
	out, err := os.CreateTemp(store.localCachePath, localTempPrefix)
	if err != nil {
		return ID{}, false, fmt.Errorf("failed to create file in local cache: %w", err)
	}
	defer out.Close()

	if err = zstpak.CompressInto(in, out, zstpakChunkSize); err != nil {
		_ = os.Remove(storePath)
		return ID{}, false, fmt.Errorf("failed to compress file: %w", err)
	}

	if err = commitTempFile(out, storePath); err != nil {
		return ID{}, false, err
	}

	return id, true, nil
}

// OpenReadAt opens a file in the store for random-access reading.
func (store *Store) OpenReadAt(id ID) (*ModuleReader, error) {
	localPath, err := store.ensurePresentLocally(id)
	if err != nil {
		return nil, err
	}

	file, err := zstpak.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}

	reader := &ModuleReader{
		ReaderAt:          file,
		Closer:            file,
		preferredReadSize: uint(file.ChunkSize()),
		size:              uint(file.UncompressedSize()),
	}

	return reader, nil
}

// OpenBufferedReadAt is a buffered version of `OpenReadAt`.
func (store *Store) OpenBufferedReadAt(id ID, cacheSizeBytes uint) (
	*ModuleReader, error) {
	reader, err := store.OpenReadAt(id)
	if err != nil {
		return nil, err
	}

	numChunks := cacheSizeBytes / reader.preferredReadSize

	if numChunks == 0 {
		// Cache size too small for a full page: continue without buffering.
		return reader, nil
	}

	buffered, err := readatbuf.New(reader.ReaderAt, reader.preferredReadSize, numChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to add buffering to the reader: %w", err)
	}

	reader.ReaderAt = buffered
	return reader, nil
}

// UploadModule uploads a module from the local storage to the remote. If the module is already
// present, no operation is performed.
func (store *Store) UploadModule(id ID) error {
	present, err := store.IsPresentRemotely(id)
	if err != nil {
		return fmt.Errorf("failed to check whether the module exists on remote: %w", err)
	}
	if present {
		return nil
	}
	present, err = store.IsPresentLocally(id)
	if err != nil {
		return fmt.Errorf("failed to check whether the module exists locally: %w", err)
	}
	if !present {
		return fmt.Errorf("the given module `%x` isn't present locally", id)
	}

	localPath := store.makeLocalPath(id)
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}

	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to hash content of %q: %v", localPath, err)
	}
	contentSHA256 := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	moduleKey := makeS3Key(id)
	contentType := "application/octet-stream"
	contentDisposition := "attachment"

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to set position in file %q: %v", localPath, err)
	}

	_, err = store.s3client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:             &store.bucket,
		Key:                &moduleKey,
		Body:               file,
		ContentType:        &contentType,
		ContentDisposition: &contentDisposition,
		ChecksumSHA256:     &contentSHA256,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// RemoveLocalModule removes a module from the local cache. No-op if not present.
func (store *Store) RemoveLocalModule(id ID) error {
	present, err := store.IsPresentLocally(id)
	if err != nil {
		return fmt.Errorf("failed to check whether the module exists locally: %w", err)
	}
	if !present {
		return nil
	}

	if err := os.Remove(store.makeLocalPath(id)); err != nil {
		return fmt.Errorf("failed to delete local file: %w", err)
	}

	return nil
}

// RemoveRemoteModule removes a module from the remote storage. No-op if not present.
func (store *Store) RemoveRemoteModule(id ID) error {
	moduleKey := makeS3Key(id)
	_, err := store.s3client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &store.bucket,
		Key:    &moduleKey,
	})
	if err != nil {
		if isErrNoSuchKey(err) {
			return nil
		}
		return fmt.Errorf("failed to delete file from remote: %w", err)
	}

	return nil
}

// UnpackModuleToPath extracts a module from the store to the given local path.
func (store *Store) UnpackModuleToPath(id ID, outPath string) error {
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	return store.UnpackModule(id, out)
}

// UnpackModule extracts a module from the store, writing it to the given writer.
func (store *Store) UnpackModule(id ID, out io.Writer) error {
	reader, err := store.OpenReadAt(id)
	if err != nil {
		return fmt.Errorf("failed to open module: %w", err)
	}
	defer reader.Close()

	// Create a sparse file if output is an empty file.
	file, ok := out.(*os.File)
	if ok {
		pos, err := file.Seek(0, io.SeekCurrent)
		if err == nil && pos == 0 {
			err = unix.Ftruncate(int(file.Fd()), int64(reader.Size()))
			if err != nil {
				file = nil
			}
		} else {
			file = nil
		}
	}

	chunk := make([]byte, reader.PreferredReadSize())
	offset := 0
	for {
		n, err := reader.ReadAt(chunk, int64(offset))
		if err != nil {
			if err == io.EOF {
				chunk = chunk[:n]
			} else {
				return fmt.Errorf("failed to read module: %w", err)
			}
		}
		if n == 0 {
			break
		}

		// Optimized sparse file path.
		if file != nil && libpf.SliceAllEqual(chunk, 0) {
			_, err = file.Seek(int64(len(chunk)), io.SeekCurrent)
			if err != nil {
				return fmt.Errorf("failed to seek: %v", err)
			}

			offset += n
			continue
		}

		_, err = out.Write(chunk)
		if err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}

		offset += n
	}

	return nil
}

// ExportModule exports a compressed module from the store, writing it to the given writer.
func (store *Store) ExportModule(id ID, tw *tar.Writer) error {
	localPath, err := store.ensurePresentLocally(id)
	if err != nil {
		return err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:    localPath,
		Mode:    0644,
		Size:    int64(fi.Size()),
		ModTime: fi.ModTime(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	return err
}

// IsPresentRemotely checks whether a module is present in the remote data-store.
func (store *Store) IsPresentRemotely(id ID) (bool, error) {
	moduleKey := makeS3Key(id)
	_, err := store.s3client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &store.bucket,
		Key:    &moduleKey,
	})

	if err != nil {
		if isErrNoSuchKey(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to query module existence: %w", err)
	}

	return true, nil
}

// IsPresentLocally checks whether a module is present in the local cache.
func (store *Store) IsPresentLocally(id ID) (bool, error) {
	_, err := os.Stat(store.makeLocalPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat local file: %w", err)
	}
	return true, nil
}

// ListRemoteModules creates a map of all modules present in the remote storage and their date
// of last change.
func (store *Store) ListRemoteModules() (map[ID]time.Time, error) {
	objectList, err := getS3ObjectList(
		store.s3client, store.bucket, s3KeyPrefix, s3ResultsPerPage*s3MaxPages)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve object list: %w", err)
	}

	modules := map[ID]time.Time{}
	for _, object := range objectList {
		if object.Key == nil || object.LastModified == nil {
			return nil, errors.New("s3 object lacks required field")
		}

		idText := strings.TrimPrefix(*object.Key, s3KeyPrefix)
		id, err := IDFromString(idText)
		if err != nil {
			return nil, fmt.Errorf("failed to parse hash in S3 filename: %w", err)
		}
		modules[id] = *object.LastModified
	}

	return modules, nil
}

// ListLocalModules creates a set of all modules present in the local cache.
func (store *Store) ListLocalModules() (libpf.Set[ID], error) {
	modules := libpf.Set[ID]{}

	moduleVisitor := func(id ID) error {
		modules[id] = libpf.Void{}
		return nil
	}
	unkVisitor := func(string) error {
		return nil
	}
	if err := store.visitLocalModules(moduleVisitor, unkVisitor); err != nil {
		return nil, err
	}

	return modules, nil
}

// RemoveLocalTempFiles removes all lingering temporary files that were never fully committed.
//
// If multiple instances of `Store` exist for the same cache directory, this may with uncommitted
// writes of the other instance.
func (store *Store) RemoveLocalTempFiles() error {
	moduleVisitor := func(ID) error {
		return nil
	}
	unkVisitor := func(unkPath string) error {
		if !strings.HasPrefix(path.Base(unkPath), localTempPrefix) {
			log.Warnf("`%s` file in local cache is neither a temp file nor a module", unkPath)
			return nil
		}
		if err := os.Remove(unkPath); err != nil {
			return fmt.Errorf("failed to remove file: %w", err)
		}
		return nil
	}
	return store.visitLocalModules(moduleVisitor, unkVisitor)
}

// ensurePresentLocally makes sure a module is present locally, downloading it from the remote
// storage if required. On success, it returns the path to the compressed file in the local storage.
func (store *Store) ensurePresentLocally(id ID) (string, error) {
	localPath := store.makeLocalPath(id)
	present, err := store.IsPresentLocally(id)
	if err != nil {
		return "", err
	}
	if present {
		return localPath, nil
	}

	moduleKey := makeS3Key(id)
	resp, err := http.Get(store.publicReadURL + moduleKey)
	if err != nil {
		return "", fmt.Errorf("failed to request file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errorResponse, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("store returned %d %s", resp.StatusCode, errorResponse)
	}

	// Download the file to a temporary location to prevent half-complete modules on crashes.
	file, err := os.CreateTemp(store.localCachePath, localTempPrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()
	if _, err = io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to receive file: %w", err)
	}

	if err = commitTempFile(file, localPath); err != nil {
		return "", err
	}

	return localPath, nil
}

// makeLocalPath creates the local cache path for the given ID.
func (store *Store) makeLocalPath(id ID) string {
	return fmt.Sprintf("%s/%s", store.localCachePath, id.String())
}

// visitLocalModules visits all files in the local cache path. `moduleVisitor` is called for each
// file recognized as a valid module ID, `unkVisitor` is called with the full path of all other
// files in the path.
func (store *Store) visitLocalModules(moduleVisitor func(ID) error,
	unkVisitor func(string) error) error {
	files, err := os.ReadDir(store.localCachePath)
	if err != nil {
		return fmt.Errorf("failed to read files in local cache: %w", err)
	}

	for _, file := range files {
		id, err := IDFromString(file.Name())
		if err == nil {
			err = moduleVisitor(id)
		} else {
			err = unkVisitor(path.Join(store.localCachePath, file.Name()))
		}
		if err != nil {
			return nil
		}
	}

	return nil
}

// makeS3Key creates the S3 key for the given module.
func makeS3Key(id ID) string {
	return s3KeyPrefix + id.String()
}

// commitTempFile makes sure that the given file is flushed to disk, then moves it to its final
// destination.
func commitTempFile(temp *os.File, finalPath string) error {
	if err := syscall.Fsync(int(temp.Fd())); err != nil {
		return fmt.Errorf("failed to flush file to disk: %w", err)
	}
	if err := os.Rename(temp.Name(), finalPath); err != nil {
		return fmt.Errorf("failed to move file to final location: %w", err)
	}

	return nil
}

// isErrNoSuchKey checks whether the given AWS error indicates that the given key does not exist.
func isErrNoSuchKey(err error) bool {
	// The documentation says that the API is supposed to return `NoSuchKey` if an object doesn't
	// exist. However, in reality, the Go client instead simply inspects the HTTP status code and
	// turns the 404 into "NotFound", without exposing the actual error code sent by the API.
	//
	// This unfortunately prevents us from telling a 404 from a non-existent key from a 404 caused
	// by a non-existent bucket. We thus have to just assume it's `NoSuchKey`, since non-existent
	// bucket should rarely happen in practice.
	//
	// For forward compatibility (in case this ever gets fixed), we also check for `NoSuchKey`.

	var noSuchKey *s3types.NoSuchKey
	var notFound *s3types.NotFound
	return errors.As(err, &noSuchKey) || errors.As(err, &notFound)
}
