//go:build linux && cgo && promtail_journal_enabled

// Package sdjournal is a minimal wrapper over libsystemd's sd-journal API.
//
// We don't link libsystemd at build time. Instead openLib resolves each
// function at runtime with dlopen/dlsym, so the default Alloy binary has no
// hard dependency on libsystemd. It is only needed when loki.source.journal
// runs.
//
// dlsym returns an untyped void*, which Go can't call directly, so for
// each function we define a tiny C wrapper (named j_<func>) that casts the
// void* back to the real signature and calls it.
//
// To wrap a new sd_journal_<x> function, copy its real signature from
// systemd/sd-journal.h and, following the existing examples in the cgo preamble
// below, add:
//  1. a typedef for that signature,
//  2. a _Static_assert(... "drift") that checks our typedef still matches the
//     header (this is the safety net: a mismatch fails the build),
//  3. a j_<x> wrapper that casts the void* and calls the function.
//
// Then, on the Go side:
//  4. add a field for it to the lib struct and dlsym it in openLib,
//  5. add the Go method that calls C.j_<x>.
//
// The _Static_assert checks only run when this package is compiled against the
// systemd headers, which CI does via the promtail_journal_enabled build.
package sdjournal

/*
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdlib.h>
#include <stdint.h>
#include <systemd/sd-journal.h>

typedef int (*j_open_fn)(sd_journal **, int);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_open), j_open_fn), "sd_journal_open signature drift");
static int j_open(void *f, sd_journal **ret, int flags) {
	j_open_fn fn = f;
	return fn(ret, flags);
}

typedef int (*j_open_directory_fn)(sd_journal **, const char *, int);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_open_directory), j_open_directory_fn), "sd_journal_open_directory signature drift");
static int j_open_directory(void *f, sd_journal **ret, const char *path, int flags) {
	j_open_directory_fn fn = f;
	return fn(ret, path, flags);
}

typedef int (*j_next_fn)(sd_journal *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_next), j_next_fn), "sd_journal_next signature drift");
static int j_next(void *f, sd_journal *j) {
	j_next_fn fn = f;
	return fn(j);
}

typedef int (*j_previous_fn)(sd_journal *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_previous), j_previous_fn), "sd_journal_previous signature drift");
static int j_previous(void *f, sd_journal *j) {
	j_previous_fn fn = f;
	return fn(j);
}

typedef void (*j_restart_data_fn)(sd_journal *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_restart_data), j_restart_data_fn), "sd_journal_restart_data signature drift");
static void j_restart_data(void *f, sd_journal *j) {
	j_restart_data_fn fn = f;
	fn(j);
}

typedef int (*j_enumerate_data_fn)(sd_journal *, const void **, size_t *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_enumerate_data), j_enumerate_data_fn), "sd_journal_enumerate_data signature drift");
static int j_enumerate_data(void *f, sd_journal *j, const void **data, size_t *length) {
	j_enumerate_data_fn fn = f;
	return fn(j, data, length);
}

typedef void (*j_close_fn)(sd_journal *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_close), j_close_fn), "sd_journal_close signature drift");
static void j_close(void *f, sd_journal *j) {
	j_close_fn fn = f;
	fn(j);
}

typedef int (*j_wait_fn)(sd_journal *, uint64_t);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_wait), j_wait_fn), "sd_journal_wait signature drift");
static int j_wait(void *f, sd_journal *j, uint64_t timeout_usec) {
	j_wait_fn fn = f;
	return fn(j, timeout_usec);
}

typedef int (*j_test_cursor_fn)(sd_journal *, const char *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_test_cursor), j_test_cursor_fn), "sd_journal_test_cursor signature drift");
static int j_test_cursor(void *f, sd_journal *j, const char *cursor) {
	j_test_cursor_fn fn = f;
	return fn(j, cursor);
}

typedef int (*j_get_cursor_fn)(sd_journal *, char **);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_get_cursor), j_get_cursor_fn), "sd_journal_get_cursor signature drift");
static int j_get_cursor(void *f, sd_journal *j, char **cursor) {
	j_get_cursor_fn fn = f;
	return fn(j, cursor);
}

typedef int (*j_seek_cursor_fn)(sd_journal *, const char *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_seek_cursor), j_seek_cursor_fn), "sd_journal_seek_cursor signature drift");
static int j_seek_cursor(void *f, sd_journal *j, const char *cursor) {
	j_seek_cursor_fn fn = f;
	return fn(j, cursor);
}

typedef int (*j_get_realtime_usec_fn)(sd_journal *, uint64_t *);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_get_realtime_usec), j_get_realtime_usec_fn), "sd_journal_get_realtime_usec signature drift");
static int j_get_realtime_usec(void *f, sd_journal *j, uint64_t *usec) {
	j_get_realtime_usec_fn fn = f;
	return fn(j, usec);
}

typedef int (*j_add_match_fn)(sd_journal *, const void *, size_t);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_add_match), j_add_match_fn), "sd_journal_add_match signature drift");
static int j_add_match(void *f, sd_journal *j, const void *data, size_t size) {
	j_add_match_fn fn = f;
	return fn(j, data, size);
}

typedef int (*j_seek_realtime_usec_fn)(sd_journal *, uint64_t);
_Static_assert(__builtin_types_compatible_p(typeof(&sd_journal_seek_realtime_usec), j_seek_realtime_usec_fn), "sd_journal_seek_realtime_usec signature drift");
static int j_seek_realtime_usec(void *f, sd_journal *j, uint64_t usec) {
	j_seek_realtime_usec_fn fn = f;
	return fn(j, usec);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	// sd_journal_open flag. See sd-journal.h SD_JOURNAL_LOCAL_ONLY.
	sdJournalLocalOnly = 1

	// sdJournalNop: the journal did not change since the last invocation.
	sdJournalNop = int(C.SD_JOURNAL_NOP)
	// sdJournalAppend: new entries were appended to the end of the journal.
	sdJournalAppend = int(C.SD_JOURNAL_APPEND)
	// sdJournalInvalidate: journal files were added or removed (rotation, vacuum).
	sdJournalInvalidate = int(C.SD_JOURNAL_INVALIDATE)
)

// Well-known systemd journal field names.
const (
	// FieldMessage is the human-readable log line.
	FieldMessage = "MESSAGE"
	// FieldPriority is the syslog priority level (0-7).
	FieldPriority = "PRIORITY"
	// FieldSystemdUnit is the systemd unit that emitted the entry.
	FieldSystemdUnit = "_SYSTEMD_UNIT"
)

// lib holds the dlopen handle and the resolved sd-journal symbols. Once loaded
// these are immutable. It is purely a symbol table.
type lib struct {
	// handle is the dlopen handle for libsystemd.so.0 itself.
	handle unsafe.Pointer
	// open opens a journal, returning journal handle.
	open unsafe.Pointer
	// openDirectory opens the journal stored in a specific directory.
	openDirectory unsafe.Pointer
	// close releases the handle.
	close unsafe.Pointer
	// wait blocks until the journal changes.
	wait unsafe.Pointer
	// next advances the read pointer to the following entry.
	next unsafe.Pointer
	// previous moves the read pointer back to the preceding entry.
	previous unsafe.Pointer
	// restartData resets field enumeration back to the entry's first field.
	restartData unsafe.Pointer
	// enumerateData returns the next field of the current entry.
	enumerateData unsafe.Pointer
	// getCursor returns an opaque cursor string for the current entry.
	getCursor unsafe.Pointer
	// seekCursor moves the read pointer to the entry identified by a cursor.
	seekCursor unsafe.Pointer
	// seekRealtimeUsec moves the read pointer to the given wall-clock time.
	seekRealtimeUsec unsafe.Pointer
	// testCursor reports whether the current entry matches the given cursor.
	testCursor unsafe.Pointer
	// getRealtimeUsec returns the wall-clock receive time of the current entry,
	// in microseconds since the Unix epoch.
	getRealtimeUsec unsafe.Pointer
	// addMatch adds a field filter, only matching entries are returned.
	addMatch unsafe.Pointer
}

var (
	journalLib *lib
	mut        sync.Mutex
)

func openLib() (*lib, error) {
	mut.Lock()
	defer mut.Unlock()
	if journalLib != nil {
		return journalLib, nil
	}

	name := C.CString("libsystemd.so.0")
	// Safe: freeing a C string we allocated above
	defer C.free(unsafe.Pointer(name)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block

	handle := C.dlopen(name, C.RTLD_NOW)
	if handle == nil {
		return nil, fmt.Errorf("failed to load libsystemd.so.0: %s", C.GoString(C.dlerror()))
	}

	l := &lib{handle: handle}
	var err error

	defer func() {
		if err != nil {
			_ = C.dlclose(handle)
		}
	}()

	l.open, err = dlsym(handle, "sd_journal_open")
	if err != nil {
		return nil, err
	}
	l.openDirectory, err = dlsym(handle, "sd_journal_open_directory")
	if err != nil {
		return nil, err
	}
	l.close, err = dlsym(handle, "sd_journal_close")
	if err != nil {
		return nil, err
	}
	l.next, err = dlsym(handle, "sd_journal_next")
	if err != nil {
		return nil, err
	}
	l.previous, err = dlsym(handle, "sd_journal_previous")
	if err != nil {
		return nil, err
	}
	l.restartData, err = dlsym(handle, "sd_journal_restart_data")
	if err != nil {
		return nil, err
	}
	l.enumerateData, err = dlsym(handle, "sd_journal_enumerate_data")
	if err != nil {
		return nil, err
	}
	l.wait, err = dlsym(handle, "sd_journal_wait")
	if err != nil {
		return nil, err
	}
	l.getCursor, err = dlsym(handle, "sd_journal_get_cursor")
	if err != nil {
		return nil, err
	}
	l.seekCursor, err = dlsym(handle, "sd_journal_seek_cursor")
	if err != nil {
		return nil, err
	}
	l.testCursor, err = dlsym(handle, "sd_journal_test_cursor")
	if err != nil {
		return nil, err
	}
	l.getRealtimeUsec, err = dlsym(handle, "sd_journal_get_realtime_usec")
	if err != nil {
		return nil, err
	}
	l.addMatch, err = dlsym(handle, "sd_journal_add_match")
	if err != nil {
		return nil, err
	}
	l.seekRealtimeUsec, err = dlsym(handle, "sd_journal_seek_realtime_usec")
	if err != nil {
		return nil, err
	}

	journalLib = l
	return journalLib, nil
}

func dlsym(handle unsafe.Pointer, name string) (unsafe.Pointer, error) {
	cname := C.CString(name)
	// Safe: freeing a C string we allocated above
	defer C.free(unsafe.Pointer(cname)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block

	sym := C.dlsym(handle, cname)
	if sym == nil {
		return nil, fmt.Errorf("missing libsystemd symbol %q: %s", name, C.GoString(C.dlerror()))
	}
	return sym, nil
}

// Journal is an open handle to the systemd journal. It owns the underlying
// sd_journal object, and is the only type callers interact with: the C API is
// never exposed. (The libsystemd symbol table is loaded once per process and
// cached globally.)
type Journal struct {
	lib     *lib
	journal *C.sd_journal

	fields []Field
}

type Options struct {
	Path    string
	Cursor  string
	MaxAge  time.Duration
	Matches []string
}

// New loads libsystemd, opens the journal (the local journal, or the directory
// in opts.Path), applies any field matches, and positions the read pointer for
// the first call to Next.
func New(opts Options) (*Journal, error) {
	l, err := openLib()
	if err != nil {
		return nil, err
	}

	var journal *C.sd_journal
	if opts.Path != "" {
		p := C.CString(opts.Path)
		// Safe: freeing a C string we allocated above
		defer C.free(unsafe.Pointer(p)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block
		if ret := C.j_open_directory(l.openDirectory, &journal, p, 0); ret < 0 {
			return nil, fmt.Errorf("sd_journal_open_directory failed: %w", syscall.Errno(-ret))
		}
	} else {
		if ret := C.j_open(l.open, &journal, sdJournalLocalOnly); ret < 0 {
			return nil, fmt.Errorf("sd_journal_open failed: %w", syscall.Errno(-ret))
		}
	}

	j := &Journal{lib: l, journal: journal}

	// Matches must be added before positioning so seeks only land on matching
	// entries.
	for _, m := range opts.Matches {
		if err := j.addMatch(m); err != nil {
			j.Close()
			return nil, err
		}
	}

	if err := j.seekToStart(opts.Cursor, opts.MaxAge); err != nil {
		j.Close()
		return nil, err
	}

	return j, nil
}

type Field struct {
	Name  string
	Value string
}

var ErrNoData = errors.New("no data")

// Next returns a slice with fields from the next entry in the journal and the cursor associated with that entry.
// The returned slice is borrowed and invalidated after the next call to Next.
// If there are no more entries ErrNoData is returned.
func (j *Journal) Next() ([]Field, string, error) {
	ret := C.j_next(j.lib.next, j.journal)
	if ret < 0 {
		return nil, "", fmt.Errorf("sd_journal_next failed: %w", syscall.Errno(-ret))
	}

	// No more data.
	if ret == 0 {
		return nil, "", ErrNoData
	}

	var c *C.char
	if ret := C.j_get_cursor(j.lib.getCursor, j.journal, &c); ret < 0 {
		return nil, "", fmt.Errorf("sd_journal_get_cursor failed: %w", syscall.Errno(-ret))
	}
	// Safe: freeing a C string libsystemd allocated for us
	defer C.free(unsafe.Pointer(c)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block
	cursor := C.GoString(c)

	j.fields = j.fields[:0]
	C.j_restart_data(j.lib.restartData, j.journal)
	var (
		data   unsafe.Pointer
		length C.size_t
	)
	for {
		ret := C.j_enumerate_data(j.lib.enumerateData, j.journal, &data, &length)
		if ret < 0 {
			return nil, cursor, fmt.Errorf("sd_journal_enumerate_data failed: %w", syscall.Errno(-ret))
		}
		if ret == 0 {
			// no more fields
			return j.fields, cursor, nil
		}

		kv := C.GoStringN((*C.char)(data), C.int(length))
		if i := strings.Index(kv, "="); i >= 0 {
			j.fields = append(j.fields, Field{Name: kv[:i], Value: kv[i+1:]})
		} else {
			return nil, "", errors.New("failed to parse field")
		}
	}
}

// Wait blocks until the journal changes, so a subsequent Next can return newly
// arrived entries. Call it after Next reports ErrNoData.
func (j *Journal) Wait(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		const waitTime = C.uint64_t(100 * 1000) // 100ms
		ret := C.j_wait(j.lib.wait, j.journal, waitTime)
		switch int(ret) {
		case sdJournalNop:
			// No new entries so wait again.
			continue
		case sdJournalAppend, sdJournalInvalidate:
			return nil
		default:
			return fmt.Errorf("sd_journal_wait failed: %w", syscall.Errno(-ret))
		}
	}
}

// Realtime returns the wall-clock time the current entry was received by the
// journal.
func (j *Journal) Realtime() (time.Time, error) {
	var usec C.uint64_t
	if ret := C.j_get_realtime_usec(j.lib.getRealtimeUsec, j.journal, &usec); ret < 0 {
		return time.Time{}, fmt.Errorf("sd_journal_get_realtime_usec failed: %w", syscall.Errno(-ret))
	}
	return time.UnixMicro(int64(usec)), nil
}

// Close releases the journal handle.
func (j *Journal) Close() {
	C.j_close(j.lib.close, j.journal)
}

// seekToStart positions the read pointer for the first Next call. It resumes
// from cursor when possible, but falls back to now-maxAge when there is no
// cursor, the cursor's entry has been rotated away to an older position, or the
// resume point is older than maxAge.
func (j *Journal) seekToStart(cursor string, maxAge time.Duration) error {
	var cutoff time.Time
	if maxAge > 0 {
		cutoff = time.Now().Add(-maxAge)
	}

	if cursor == "" {
		if cutoff.IsZero() {
			return nil // start from the oldest entry
		}
		return j.seekRealtime(cutoff)
	}

	c := C.CString(cursor)
	// Safe: freeing a C string we allocated above
	defer C.free(unsafe.Pointer(c)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block

	if ret := C.j_seek_cursor(j.lib.seekCursor, j.journal, c); ret < 0 {
		return fmt.Errorf("sd_journal_seek_cursor failed: %w", syscall.Errno(-ret))
	}

	switch ret := C.j_next(j.lib.next, j.journal); {
	case ret < 0:
		// Couldn't read from the cursor. Fall back to cutoff or don't seek at all.
		if cutoff.IsZero() {
			return nil
		}
		return j.seekRealtime(cutoff)
	case ret == 0:
		// The cursor is at or past the newest entry; nothing newer to read yet.
		return nil
	}

	// If the entry we landed on is older than the cutoff or we failed
	// to get realtime we start from cutoff.
	if !cutoff.IsZero() {
		if ts, err := j.Realtime(); err != nil || ts.Before(cutoff) {
			return j.seekRealtime(cutoff)
		}
	}

	// If we're on the cursor's own entry, leave the pointer so the first Next advances past it.
	// if the cursor was rotated away the entry is unread, so step back to avoid skipping it.
	if C.j_test_cursor(j.lib.testCursor, j.journal, c) <= 0 {
		if ret := C.j_previous(j.lib.previous, j.journal); ret < 0 {
			return fmt.Errorf("sd_journal_previous failed: %w", syscall.Errno(-ret))
		}
	}
	return nil
}

// seekRealtime positions the journal so the next call to Next returns the first
// entry received at or after t.
func (j *Journal) seekRealtime(t time.Time) error {
	if ret := C.j_seek_realtime_usec(j.lib.seekRealtimeUsec, j.journal, C.uint64_t(t.UnixMicro())); ret < 0 {
		return fmt.Errorf("sd_journal_seek_realtime_usec failed: %w", syscall.Errno(-ret))
	}
	return nil
}

// addMatch adds a field filter so that subsequent reads only return
// entries with that field value.
func (j *Journal) addMatch(match string) error {
	m := C.CString(match)
	// Safe: freeing a C string we allocated above
	defer C.free(unsafe.Pointer(m)) // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block

	// Safe: add_match copies the C string we allocated above
	if ret := C.j_add_match(j.lib.addMatch, j.journal, unsafe.Pointer(m), C.size_t(len(match))); ret < 0 { // #nosec G103 nosemgrep: go.lang.security.audit.unsafe.use-of-unsafe-block
		return fmt.Errorf("sd_journal_add_match failed: %w", syscall.Errno(-ret))
	}
	return nil
}
