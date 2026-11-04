package aula

import (
	"strings"
	"testing"
)

func TestGalleryAlbumMarkdown(t *testing.T) {
	album := &GalleryAlbum{
		ID:           3756587,
		Title:        "Torsdag d.16 okt 2025",
		CreationDate: "2025-10-16T12:21:00+02:00",
		Creator:      &AlbumCreator{Name: "Lone Jensen", InstitutionName: "Rønbækskolen"},
		Description:  "Ude dag med **sange** | og musik",
		Media: []AlbumMedia{
			{
				ID:         170819819,
				MediaType:  "image",
				Title:      "Højtaler",
				UploadedAt: "2025-10-16T12:18:00+02:00",
				Tags:       []Tag{{Name: "Enya"}, {Name: "Alice"}},
			},
			{ID: 170819818, MediaType: "video"},
		},
	}
	md := GalleryAlbumMarkdown(album, []SavedGalleryFile{
		{ID: 170819819, Path: "/somewhere/170819819.jpeg"},
	})
	for _, want := range []string{
		"# Torsdag d.16 okt 2025",
		"- **Album ID:** 3756587",
		"- **Created by:** Lone Jensen — Rønbækskolen",
		"\\*\\*sange\\*\\* \\| og musik",
		"| [170819819.jpeg](170819819.jpeg) | 170819819 | image | Højtaler | — | Enya, Alice |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	// Media without a saved file must not claim a link.
	if strings.Contains(md, "170819818.jpeg") {
		t.Errorf("unsaved media linked:\n%s", md)
	}
}

func TestGalleryAlbumMarkdownNoAlbum(t *testing.T) {
	if md := GalleryAlbumMarkdown(nil, nil); md != "" {
		t.Errorf("nil album should render empty, got %q", md)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Torsdag d.16 okt 2025", "Torsdag-d.16-okt-2025"},
		{"Julemarked 2024 🌲", "Julemarked-2024"},
		{"a/b\\c:d*e", "abcde"},
		{"/../etc/passwd", "etcpasswd"},
		{"   ", ""},
		{"", ""},
		{"Allerede_dato_ok", "Allerede_dato_ok"},
	}
	for _, c := range cases {
		if got := SanitizeFilename(c.in); got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
