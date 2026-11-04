package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

// Gallery tool: photo albums with media.

// downloadArgs matches aula_gallery_download's request arguments.
type downloadArgs struct {
	URL        *string `json:"url,omitempty"`
	AlbumID    *int64  `json:"albumId,omitempty"`
	MediaLimit *int    `json:"mediaLimit,omitempty"`
	TargetPath string  `json:"targetPath,omitempty"`
}

type downloadedFile struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

type downloadResult struct {
	Saved        []downloadedFile `json:"saved"`
	SkippedNoURL int              `json:"skippedNoUrl"`
	TotalBytes   int64            `json:"totalBytes"`
	Directory    string           `json:"directory,omitempty"`
	MetadataPath string           `json:"metadataPath,omitempty"`
}

func registerGalleryTools(server *mcp.Server, session *aulaSession, count func()) {
	type galleryArgs struct {
		AlbumLimit *int    `json:"albumLimit,omitempty"`
		MediaLimit *int    `json:"mediaLimit,omitempty"`
		Days       *int    `json:"days,omitempty"`
		ChildIDs   []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_gallery", "List gallery albums",
		"Lists Aula gallery albums with their media (photo URLs). Albums are sorted newest first.",
		`{
		  "type": "object",
		  "properties": {
		    "albumLimit": {
		      "type": "integer",
		      "description": "Maximum number of albums. Default: 12."
		    },
		    "mediaLimit": {
		      "type": "integer",
		      "description": "Maximum number of media items per album. Default: 30."
		    },
		    "days": {
		      "type": "integer",
		      "description": "If set, only albums created within the past N days are returned."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means the current context."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args galleryArgs) (any, error) {
			albumLimit := max(1, intArg(args.AlbumLimit, 12))
			mediaLimit := max(1, intArg(args.MediaLimit, 30))

			var retrievePastDays *int
			if args.Days != nil && *args.Days > 0 {
				retrievePastDays = args.Days
			}

			return client.GalleryAlbumMedia(albumLimit, mediaLimit, nil, retrievePastDays, args.ChildIDs)
		})

	// ── aula_gallery_download ──────────────────────────────────────────
	mcp.AddTool(server, &mcp.Tool{
		Name:  "aula_gallery_download",
		Title: "Download gallery media",
		Description: "Downloads gallery media bytes to files on the server machine. Pass a single media URL " +
			"(from aula_gallery) or an album ID to download all its media — album downloads also write a companion " +
			"Markdown (.md) file with the album metadata and the downloaded files. The media URLs are signed and " +
			"expire quickly — download shortly after listing them.",
		Annotations: &mcp.ToolAnnotations{Title: "Download gallery media"},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args downloadArgs) (*mcp.CallToolResult, any, error) {
		if args.AlbumID != nil && args.URL != nil {
			return fail("Pass either url or albumId, not both.")
		}
		if args.AlbumID == nil && args.URL == nil {
			return fail("Nothing to download: pass a media url (from aula_gallery) or an albumId.")
		}
		var result string
		_, err := session.call(func(client *aula.Client) (any, error) {
			r, derr := downloadGalleryMedia(client, args)
			result = r
			return r, derr
		})
		if err != nil {
			var invalidSession *aula.InvalidSessionError
			if errors.As(err, &invalidSession) {
				slog.Error("Session error while running tool", "tool", "aula_gallery_download", "error", err)
				return nil, nil, errors.New(invalidSession.Message)
			}
			slog.Error("Tool failed", "tool", "aula_gallery_download", "error", err)
			return nil, nil, fmt.Errorf("Tool 'aula_gallery_download' failed: %w", err)
		}
		return textResult(result)
	})
	count()
}

// downloadGalleryMedia downloads a single media URL or all media of an album
// to files and returns a report for the tool result.
func downloadGalleryMedia(client *aula.Client, args downloadArgs) (string, error) {
	if args.URL != nil {
		dir, name := singleDownloadTarget(*args.URL, args.TargetPath)
		data, err := client.DownloadMedia(*args.URL)
		if err != nil {
			return "", err
		}
		path, err := writeMediaFile(dir, name, data)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		out := downloadResult{
			Saved:      []downloadedFile{{Path: path, Bytes: info.Size()}},
			TotalBytes: info.Size(),
		}
		return reportDownloads(out), nil
	}

	// Album mode.
	album, err := client.AlbumWithMedia(*args.AlbumID, intArg(args.MediaLimit, 100))
	if err != nil {
		return "", err
	}
	if len(album.Media) == 0 {
		return fmt.Sprintf("Album %d contains no downloadable media.", *args.AlbumID), nil
	}
	dir := args.TargetPath
	if dir == "" {
		var derr error
		if dir, derr = defaultMediaDir(); derr != nil {
			return "", derr
		}
	}
	out := downloadResult{Directory: dir}
	var saved []aula.SavedGalleryFile
	for i := range album.Media {
		mediaURL := album.Media[i].FullSizeURL()
		if mediaURL == "" {
			out.SkippedNoURL++
			continue
		}
		data, err := client.DownloadMedia(mediaURL)
		if err != nil {
			return "", err
		}
		path, err := writeMediaFile(dir, albumMediaFileName(mediaURL, album.Media[i]), data)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		out.Saved = append(out.Saved, downloadedFile{Path: path, Bytes: info.Size()})
		saved = append(saved, aula.SavedGalleryFile{ID: album.Media[i].ID, Path: path})
		out.TotalBytes += info.Size()
	}

	// Metadata companion Markdown.
	if md := aula.GalleryAlbumMarkdown(album, saved); md != "" {
		name := aula.SanitizeFilename(album.Title)
		if name == "" {
			name = fmt.Sprintf("album-%d", album.ID)
		}
		mdPath, err := writeMediaFile(dir, name+".md", []byte(md))
		if err != nil {
			return "", err
		}
		out.MetadataPath = mdPath
	}
	return reportDownloads(out), nil
}

// reportDownloads renders the result summary as text for the tool result.
func reportDownloads(out downloadResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Downloaded %d file(s), %d bytes", len(out.Saved), out.TotalBytes))
	if out.SkippedNoURL > 0 {
		fmt.Fprintf(&b, " (skipped %d without a URL)", out.SkippedNoURL)
	}
	b.WriteString(":")
	for _, f := range out.Saved {
		fmt.Fprintf(&b, "\n%s (%d bytes)", f.Path, f.Bytes)
	}
	if out.MetadataPath != "" {
		fmt.Fprintf(&b, "\nMetadata: %s", out.MetadataPath)
	}
	return b.String()
}

// singleDownloadTarget resolves the destination directory and file name for
// a single-URL download. An explicit targetPath is always honored.
func singleDownloadTarget(mediaURL, targetPath string) (dir, name string) {
	if targetPath != "" {
		return filepath.Split(targetPath)
	}
	dir, _ = defaultMediaDir()
	return dir, mediaFileName(mediaURL, "aula-media")
}

// albumMediaFileName derives a per-media file name for album downloads:
// the media ID plus the URL's extension, e.g. "123456.jpg".
func albumMediaFileName(mediaURL string, media aula.AlbumMedia) string {
	name := fmt.Sprintf("%d", media.ID)
	if ext := mediaFileExt(mediaURL); ext != "" {
		name += ext
	}
	return name
}

// mediaFileName derives a file name for a media URL: the last path segment
// (the storage key's name), else the given fallback.
func mediaFileName(mediaURL, fallback string) string {
	if u, err := url.Parse(mediaURL); err == nil && filepath.Base(u.Path) != "." && filepath.Base(u.Path) != "/" {
		return filepath.Base(u.Path)
	}
	return fallback
}

// mediaFileExt returns the lowercased extension of the media URL's path.
func mediaFileExt(mediaURL string) string {
	u, err := url.Parse(mediaURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(filepath.Ext(u.Path))
}

// defaultMediaDirFn is the fallback directory resolver when targetPath is not
// given: ~/Downloads/aula-media. A variable to override in tests.
var defaultMediaDirFn = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine the download directory: %w", err)
	}
	return filepath.Join(home, "Downloads", "aula-media"), nil
}

func defaultMediaDir() (string, error) { return defaultMediaDirFn() }

// writeMediaFile writes the media bytes to dir/name, creating the directory
// if needed and never overwriting an existing file (a "-<n>" suffix is added
// instead). Returns the final path.
func writeMediaFile(dir, name string, data []byte) (string, error) {
	if name == "" || name == "." || name == "/" {
		name = "aula-media"
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := uniquePath(filepath.Join(dir, name))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	slog.Info("Gallery media saved", "path", path, "bytes", len(data))
	return path, nil
}

// uniquePath returns path, or the first path with a "-N" suffix before the
// extension that does not exist yet.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
