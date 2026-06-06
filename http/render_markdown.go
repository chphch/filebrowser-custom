package fbhttp

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/russross/blackfriday/v2"

	"github.com/filebrowser/filebrowser/v2/files"
)

const markdownHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root { --fg:#24292f; --bg:#fff; --muted:#57606a; --border:#d0d7de; --code-bg:#f6f8fa; --link:#0969da; }
html,body { margin:0; padding:0; background:#fafafa; color:var(--fg);
  font:16px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,"Apple SD Gothic Neo","Malgun Gothic",sans-serif; }
main { max-width:920px; margin:0 auto; padding:32px 48px; background:var(--bg); box-shadow:0 1px 4px rgba(0,0,0,.06); }
@media (max-width:767px) { main { padding:16px; } }
h1,h2,h3,h4 { margin-top:1.5em; padding-bottom:.3em; border-bottom:1px solid var(--border); }
h1 { font-size:1.8em; } h2 { font-size:1.4em; } h3 { font-size:1.15em; border-bottom:0; }
a { color:var(--link); text-decoration:none; } a:hover { text-decoration:underline; }
code { background:var(--code-bg); padding:.18em .35em; border-radius:4px;
  font:.92em ui-monospace,SFMono-Regular,Menlo,monospace; }
pre { background:var(--code-bg); padding:14px; border-radius:6px; overflow:auto; }
pre code { background:transparent; padding:0; }
blockquote { margin:1em 0; padding:0 1em; color:var(--muted); border-left:4px solid var(--border); }
table { border-collapse:collapse; margin:1em 0; }
th,td { border:1px solid var(--border); padding:6px 13px; }
th { background:var(--code-bg); }
hr { border:0; border-top:1px solid var(--border); margin:2em 0; }
img { max-width:100%%; }
</style>
</head>
<body>
<main>
%s
</main>
</body>
</html>`

// isMarkdownExt reports whether the file extension is a markdown variant.
func isMarkdownExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// renderMarkdownToHTML reads the file, renders the markdown to HTML, and
// writes a self-contained HTML page with inline CSS. No scripts are emitted,
// so the global `script-src 'none'` CSP does not interfere.
func renderMarkdownToHTML(w http.ResponseWriter, _ *http.Request, file *files.FileInfo) (int, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	raw, err := io.ReadAll(fd)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	body := blackfriday.Run(raw,
		blackfriday.WithExtensions(blackfriday.CommonExtensions))

	// Best-effort title from first H1.
	title := file.Name
	for _, line := range strings.SplitN(string(raw), "\n", 50) {
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Content-Security-Policy", `script-src 'none';`)
	w.Header().Set("Cache-Control", "private, no-store")
	_, err = fmt.Fprintf(w, markdownHTMLTemplate, html.EscapeString(title), body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
