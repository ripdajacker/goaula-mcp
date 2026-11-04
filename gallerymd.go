package aula

import (
	"fmt"
	"strings"
)

// SavedGalleryFile records a downloaded gallery media file: the Aula media ID
// and the path it was written to (absolute or relative; links in the Markdown
// use the base name).
type SavedGalleryFile struct {
	ID   int64
	Path string
}

// GalleryAlbumMarkdown renders an album's metadata as Markdown, for
// companion .md files next to downloaded gallery media. The files argument
// lists what was downloaded from the album (nil is fine too), and links them
// by base name.
func GalleryAlbumMarkdown(album *GalleryAlbum, files []SavedGalleryFile) string {
	var b strings.Builder
	if album == nil {
		return ""
	}

	fmt.Fprintf(&b, "# %s\n\n", firstNonEmptyStr(album.Title, fmt.Sprintf("Album %d", album.ID)))

	fmt.Fprintf(&b, "- **Album ID:** %d\n", album.ID)
	if album.CreationDate != "" {
		fmt.Fprintf(&b, "- **Created:** %s\n", album.CreationDate)
	}
	if album.Creator != nil {
		text := strings.TrimSpace(strings.Join([]string{
			firstNonEmptyStr(album.Creator.Name, ""),
			firstNonEmptyStr(album.Creator.InstitutionName, ""),
		}, " — "))
		if text != "" && text != "—" {
			fmt.Fprintf(&b, "- **Created by:** %s\n", text)
		}
	}
	if album.Description != "" {
		fmt.Fprintf(&b, "- **Description:** %s\n", mdInline(album.Description))
	}
	if len(album.SharedWithGroups) > 0 {
		var names []string
		for _, g := range album.SharedWithGroups {
			if g.Name != "" {
				names = append(names, g.Name)
			}
		}
		if len(names) > 0 {
			fmt.Fprintf(&b, "- **Shared with:** %s\n", strings.Join(names, ", "))
		}
	}

	fmt.Fprintf(&b, "- **Media:** %d item(s)\n\n", len(album.Media))

	if len(files) > 0 {
		b.WriteString("| File | Media ID | Type | Title | Description | Tags | Uploaded |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		for _, f := range files {
			m := findMedia(album, f.ID)
			if m == nil {
				continue
			}
			fmt.Fprintf(&b, "| [%s](%s) | %d | %s | %s | %s | %s | %s |\n",
				baseNameFor(f.Path), baseNameFor(f.Path),
				m.ID,
				mdCell(m.MediaType),
				mdCell(m.Title),
				cellSnippet(m.Description, 120),
				mdCell(strings.Join(tagNames(m.Tags), ", ")),
				mdCell(m.UploadedAt))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// findMedia returns the album media with the given ID, if present.
func findMedia(album *GalleryAlbum, id int64) *AlbumMedia {
	for i := range album.Media {
		if album.Media[i].ID == id {
			return &album.Media[i]
		}
	}
	return nil
}

// tagNames flattens media tags to their names.
func tagNames(tags []Tag) []string {
	names := make([]string, 0, len(tags))
	for _, t := range tags {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names
}

// baseNameFor is the Markdown link target of a saved file path.
func baseNameFor(path string) string {
	if path == "" {
		return ""
	}
	// Split on the OS separator when possible ("path/filepath" would be a
	// file dependency; downloads never carry separator characters in names).
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

// mdCell renders a Markdown table cell (pipes escaped, newlines collapsed).
func mdCell(text string) string {
	text = strings.Join(strings.Fields(mdInline(text)), " ")
	return text
}

// mdInline escapes Markdown-structural characters in plain text.
func mdInline(text string) string {
	replacer := strings.NewReplacer("|", `\|`, "`", "\\`", "*", `\*`, "_", `\_`)
	return replacer.Replace(text)
}

// cellSnippet truncates a description for a table cell.
func cellSnippet(text string, max int) string {
	text = mdCell(text)
	if max > 0 && len(text) > max {
		return text[:max] + "…"
	}
	if text == "" {
		return "—"
	}
	return text
}

// firstNonEmptyStr returns the first non-empty string.
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// SanitizeFilename reduces a name to a safe file base ("", slashes and other
// path-breaking characters removed, spaces to dashes).
func SanitizeFilename(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == ' ', r == '-', r == '_', r == '\t', r == '+', r == ',':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			// Anything else (including '/', separators and quotes) is dropped.
		}
	}
	out := strings.Trim(b.String(), "-")
	out = strings.TrimLeft(out, ".") // avoid odd file names like "..etcpasswd"
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return out
}
