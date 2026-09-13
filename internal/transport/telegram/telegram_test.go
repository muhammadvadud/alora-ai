package telegram

import "testing"

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct{ in, want string }{
		{"**Matiz 2013**", "<b>Matiz 2013</b>"},
		{
			"[To'lov havolasi](https://dev-app.rhmt.uz/invoice/abc)",
			`<a href="https://dev-app.rhmt.uz/invoice/abc">To'lov havolasi</a>`,
		},
		{"1 < 2 & 3 > 0", "1 &lt; 2 &amp; 3 &gt; 0"}, // HTML belgilari himoyalanadi
		{"oddiy matn", "oddiy matn"},                 // o'zgarmaydi
		{"**narx** [link](https://x.uz/a)", `<b>narx</b> <a href="https://x.uz/a">link</a>`},
	}
	for _, tt := range tests {
		if got := markdownToHTML(tt.in); got != tt.want {
			t.Errorf("markdownToHTML(%q)\n  = %q\n want %q", tt.in, got, tt.want)
		}
	}
}
