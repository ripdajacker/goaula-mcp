package main

// Gallery commands: list albums with media URLs, and download media to files.

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	aula "github.com/ripdajacker/goaula-mcp"
)

// cmdGallery lists gallery albums with their media (URLs).
func cmdGallery(args []string) int {
	var albumLimit, mediaLimit, days int
	var noMedia bool
	_, jsonOut, ctx := flagSet(args, "gallery", func(fs *flag.FlagSet) {
		fs.IntVar(&albumLimit, "albums", 12, "maximum number of albums")
		fs.IntVar(&mediaLimit, "media", 30, "maximum number of media per album")
		fs.IntVar(&days, "days", 0, "only albums created within the past N days (0 = no filter)")
		fs.BoolVar(&noMedia, "no-media", false, "list all albums without their media (one API call instead of one per album)")
	})

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var pastDays *int
	if days > 0 {
		pastDays = &days
	}

	var albums []aula.GalleryAlbum
	if noMedia {
		albums, err = client.Albums(albumLimit, nil, pastDays, nil)
	} else {
		albums, err = client.GalleryAlbumMedia(albumLimit, mediaLimit, nil, pastDays, nil)
	}
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, albums, func(b *strings.Builder) {
		if noMedia {
			for _, a := range albums {
				fmt.Fprintf(b, "%s (album %d): %s\n", a.Title, a.ID, tsOrDefault(a.CreationDate, "—"))
			}
			if len(albums) == 0 {
				b.WriteString("No gallery albums.\n")
			}
			b.WriteString("Listing of a single album: goaula gallery-download --album <album ID> (or a data tool: goaula gallery --albums N)\n")
			return
		}
		for _, a := range albums {
			fmt.Fprintf(b, "%s (album %d, %d media): %s\n", a.Title, a.ID, len(a.Media), tsOrDefault(a.CreationDate, "—"))
			for _, m := range a.Media {
				fileURL := m.FullSizeURL()
				if fileURL == "" {
					fmt.Fprintf(b, "  media %d (%s): no URL available\n", m.ID, m.MediaType)
					continue
				}
				fmt.Fprintf(b, "  media %d (%s): %s\n", m.ID, m.MediaType, stripQS(fileURL))
			}
		}
		if len(albums) == 0 {
			b.WriteString("No gallery albums.\n")
		}
		b.WriteString("Download: goaula gallery-download --album <album ID> [--dir PATH]\n")
	})
	return exitOK
}

// stripQS makes the human URLs readable: the signed query is huge noise.
func stripQS(mediaURL string) string {
	if u, err := url.Parse(mediaURL); err == nil {
		return u.Host + u.Path
	}
	return mediaURL
}

// cmdGalleryDownload downloads media to files.
func cmdGalleryDownload(args []string) int {
	fs := flag.NewFlagSet("gallery-download", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output machine-readable JSON")
	var urls []string
	fs.Func("url", "single media URL to download (repeatable)", func(v string) error {
		urls = append(urls, v)
		return nil
	})
	albumID := fs.Int64("album", 0, "download all media of this album")
	mediaLimit := fs.Int("limit", 100, "maximum number of media when --album is used")
	dir := fs.String("dir", "", "destination directory (default: ~/Downloads/aula-media)")
	targetPath := fs.String("out", "", "destination file when downloading a single --url")
	if err := fs.Parse(args); err != nil {
		os.Exit(exitUsage)
	}
	if *albumID == 0 && len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "goaula: nothing to download: pass --album N (IDs via 'goaula gallery') and/or one or more --url")
		return exitUsage
	}
	if *targetPath != "" && (len(urls) != 1 || *albumID != 0) {
		fmt.Fprintln(os.Stderr, "goaula: --out applies to a single --url download only")
		return exitUsage
	}

	client, err := sessionClient(context{})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var paths []string
	skipped := 0
	save := func(mediaURL, path string) {
		fpath, serr := saveMedia(client, mediaURL, path)
		if serr != nil {
			fmt.Fprintf(os.Stderr, "goaula: %v\n", serr)
			os.Exit(exitFail)
		}
		paths = append(paths, fpath)
	}

	if *albumID != 0 {
		// Fresh URLs: the media is fetched right before downloading.
		media, merr := client.MediaForAlbum(*albumID, *mediaLimit)
		if merr != nil {
			failSession(merr)
			return exitFail
		}
		target, derr := resolveMediaDir(*dir)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "goaula: %v\n", derr)
			return exitFail
		}
		for i := range media {
			mediaURL := media[i].FullSizeURL()
			if mediaURL == "" {
				skipped++
				continue
			}
			save(mediaURL, filepath.Join(target, albumMediaName(mediaURL, media[i].ID)))
		}
	}
	for _, u := range urls {
		name := mediaFileName(u, "aula-media")
		path, derr := resolveMediaDir(*dir)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "goaula: %v\n", derr)
			return exitFail
		}
		p := filepath.Join(path, name)
		if *targetPath != "" {
			p = *targetPath
		}
		save(u, p)
	}

	if len(paths) == 0 && skipped == 0 {
		fmt.Fprintln(os.Stderr, "goaula: nothing was downloaded (no media with a URL?)")
		return exitFail
	}
	emit(*jsonOut, map[string]any{"saved": paths, "skippedNoUrl": skipped}, func(b *strings.Builder) {
		for _, p := range paths {
			fmt.Fprintln(b, p)
		}
		if skipped > 0 {
			fmt.Fprintf(b, "(skipped %d media without a URL)\n", skipped)
		}
	})
	return exitOK
}

// resolveMediaDir resolves the effective download directory, falling back to
// the default when dir is empty.
func resolveMediaDir(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	return defaultMediaDir()
}

// albumMediaName: "<mediaID><lowercased extension from the URL>".
func albumMediaName(mediaURL string, id int64) string {
	name := strconv.FormatInt(id, 10)
	if u, err := url.Parse(mediaURL); err == nil && filepath.Ext(u.Path) != "" {
		name += strings.ToLower(filepath.Ext(u.Path))
	}
	return name
}

// mediaFileName derives a file name from the URL's path segment.
func mediaFileName(mediaURL, fallback string) string {
	if u, err := url.Parse(mediaURL); err == nil {
		if base := filepath.Base(u.Path); base != "." && base != "/" {
			return base
		}
	}
	return fallback
}

// saveMedia downloads a media URL and writes it to path (a "-N" suffix
// resolves collisions — never overwrite).
func saveMedia(client *aula.Client, mediaURL, path string) (string, error) {
	data, err := client.DownloadMedia(mediaURL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	for i := 1; ; i++ {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			break
		}
		ext := filepath.Ext(path)
		path = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(path, ext), i, ext)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "saved", path)
	return path, nil
}

// defaultMediaDir is the fallback download directory: ~/Downloads/aula-media.
func defaultMediaDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine the download directory: %w", err)
	}
	return filepath.Join(home, "Downloads", "aula-media"), nil
}
