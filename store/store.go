// Package store handles migration files on disk and the application-level
// view of "what has been applied" by collating the driver's history table
// with the files in the output directory.
package store

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// File represents one migration file on disk.
type File struct {
	Path      string
	Version   int64
	Name      string
	Direction string // "up" | "down"
	Format    string // "sql" | "json" | "jsonc" | "js"
	Checksum  string // sha256 of file body (excluding header checksum line)
	Header    Header
	Body      string
}

// Header is the parsed metadata block at the top of a generated migration file.
type Header struct {
	LibraryVersion string
	Version        int64
	Name           string
	Direction      string
	Generated      string
	Checksum       string // checksum recorded in the header itself
}

var fileNameRe = regexp.MustCompile(`^(\d{4,})_(.+?)\.(up|down)\.(sql|json|jsonc|js)$`)

// ListFiles scans dir and returns all migration files sorted by (version, direction).
func ListFiles(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list files: %w", err)
	}
	var files []File
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := fileNameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			continue
		}
		f := File{
			Path:      filepath.Join(dir, e.Name()),
			Version:   v,
			Name:      m[2],
			Direction: m[3],
			Format:    m[4],
		}
		if err := loadFile(&f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Version != files[j].Version {
			return files[i].Version < files[j].Version
		}
		return files[i].Direction < files[j].Direction
	})
	return files, nil
}

func loadFile(f *File) error {
	raw, err := os.ReadFile(f.Path)
	if err != nil {
		return fmt.Errorf("read %s: %w", f.Path, err)
	}
	hdr, body := splitHeaderBody(string(raw))
	f.Body = body
	f.Header = parseHeader(hdr)
	f.Checksum = ChecksumBody(body)
	return nil
}

func splitHeaderBody(content string) (string, string) {
	scan := bufio.NewScanner(strings.NewReader(content))
	scan.Buffer(make([]byte, 0, 1<<20), 1<<22)
	var headerLines []string
	var bodyLines []string
	inHeader := true
	for scan.Scan() {
		l := scan.Text()
		if inHeader {
			if strings.HasPrefix(l, "--") || strings.HasPrefix(l, "//") || l == "" {
				headerLines = append(headerLines, l)
				continue
			}
			inHeader = false
		}
		bodyLines = append(bodyLines, l)
	}
	return strings.Join(headerLines, "\n"), strings.Join(bodyLines, "\n")
}

func parseHeader(h string) Header {
	out := Header{}
	for _, l := range strings.Split(h, "\n") {
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(strings.TrimPrefix(l, "--"), "//")
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "go-migrate-safe v"):
			out.LibraryVersion = strings.TrimPrefix(l, "go-migrate-safe v")
		case strings.HasPrefix(l, "Version:"):
			n, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(l, "Version:")), 10, 64)
			if err == nil {
				out.Version = n
			}
		case strings.HasPrefix(l, "Name:"):
			out.Name = strings.TrimSpace(strings.TrimPrefix(l, "Name:"))
		case strings.HasPrefix(l, "Direction:"):
			out.Direction = strings.TrimSpace(strings.TrimPrefix(l, "Direction:"))
		case strings.HasPrefix(l, "Generated:"):
			out.Generated = strings.TrimSpace(strings.TrimPrefix(l, "Generated:"))
		case strings.HasPrefix(l, "Checksum:"):
			c := strings.TrimSpace(strings.TrimPrefix(l, "Checksum:"))
			c = strings.TrimPrefix(c, "sha256:")
			out.Checksum = c
		}
	}
	return out
}

// ChecksumBody returns the SHA-256 hex digest of the body bytes.
// This must match the body the codegen package signs.
func ChecksumBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// NextVersion returns one greater than the highest existing file version,
// starting at 1 if dir is empty / does not exist.
func NextVersion(dir string) (int64, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return 0, err
	}
	var max int64
	for _, f := range files {
		if f.Version > max {
			max = f.Version
		}
	}
	return max + 1, nil
}

// ExistingVersions returns the sorted unique versions present in dir.
func ExistingVersions(dir string) ([]int64, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	seen := map[int64]struct{}{}
	for _, f := range files {
		seen[f.Version] = struct{}{}
	}
	out := make([]int64, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// PendingFiles returns up-direction files whose version is greater than the
// highest applied version in history.
func PendingFiles(dir string, history []migrate.MigrationRecord) ([]File, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	var maxApplied int64
	for _, r := range history {
		if r.Direction == "up" && r.Status == "applied" && r.Version > maxApplied {
			maxApplied = r.Version
		}
	}
	var out []File
	for _, f := range files {
		if f.Direction == "up" && f.Version > maxApplied {
			out = append(out, f)
		}
	}
	return out, nil
}

// FindFile returns the file matching version+direction or an error.
func FindFile(dir string, version int64, direction string) (*File, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if f.Version == version && f.Direction == direction {
			ff := f
			return &ff, nil
		}
	}
	return nil, fmt.Errorf("migration file v%04d.%s not found in %s", version, direction, dir)
}

// Status describes a migration's state.
type Status struct {
	Version  int64
	Name     string
	UpFile   string
	DownFile string
	Applied  bool
	AppliedAt string
}

// BuildStatus correlates filesystem files with the driver's history.
func BuildStatus(_ context.Context, dir string, hist []migrate.MigrationRecord) ([]Status, error) {
	files, err := ListFiles(dir)
	if err != nil {
		return nil, err
	}
	byVersion := map[int64]*Status{}
	for _, f := range files {
		st := byVersion[f.Version]
		if st == nil {
			st = &Status{Version: f.Version, Name: f.Name}
			byVersion[f.Version] = st
		}
		if f.Direction == "up" {
			st.UpFile = f.Path
		} else {
			st.DownFile = f.Path
		}
	}
	for _, r := range hist {
		if r.Direction == "up" && r.Status == "applied" {
			st := byVersion[r.Version]
			if st == nil {
				st = &Status{Version: r.Version, Name: r.Name}
				byVersion[r.Version] = st
			}
			st.Applied = true
			st.AppliedAt = r.AppliedAt
		}
	}
	out := make([]Status, 0, len(byVersion))
	for _, s := range byVersion {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
