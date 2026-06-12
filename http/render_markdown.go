package fbhttp

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
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
pre.mermaid { background:transparent; padding:0; text-align:center; }
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
%s
</body>
</html>`

// mermaidScript is appended to the page only when at least one mermaid code
// block is present. It loads mermaid as an ES module from a CDN and renders
// every <pre class="mermaid"> block on load. Requires the render endpoint's
// relaxed CSP (see renderMarkdownToHTML).
const mermaidScript = `<script type="module">
import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
mermaid.initialize({ startOnLoad: true, securityLevel: 'loose' });
</script>`

// mermaidBlockRe matches the <pre><code class="language-mermaid">…</code></pre>
// that blackfriday emits for a ```mermaid fenced block. The inner text stays
// HTML-escaped — the browser decodes it back via textContent when mermaid reads
// the node, so the diagram source is recovered intact.
var mermaidBlockRe = regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)

// isMarkdownExt reports whether the file extension is a markdown variant.
func isMarkdownExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// renderMarkdownToHTML reads the file, renders the markdown to HTML, and
// writes a self-contained HTML page with inline CSS. When the document
// contains mermaid code blocks, a mermaid ES-module loader is injected and the
// Content-Security-Policy is relaxed just enough to run it; otherwise no
// scripts are emitted.
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

	// Convert mermaid fenced code into the <pre class="mermaid"> form mermaid.js
	// renders, and decide whether the page needs the mermaid loader.
	htmlBody := mermaidBlockRe.ReplaceAllStringFunc(string(body), func(m string) string {
		sub := mermaidBlockRe.FindStringSubmatch(m)
		return `<pre class="mermaid">` + sub[1] + `</pre>`
	})
	hasMermaid := strings.Contains(htmlBody, `<pre class="mermaid">`)

	// Override both the global middleware CSP (default-src 'self') and the old
	// script-src 'none' with a single authoritative policy. When mermaid is
	// present, relax it just enough to load and run the CDN module.
	script := ""
	csp := `default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline';`
	if hasMermaid {
		script = mermaidScript
		csp = `default-src 'self'; img-src 'self' data:; ` +
			`font-src 'self' data: https://cdn.jsdelivr.net; ` +
			`style-src 'self' 'unsafe-inline'; ` +
			`script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; ` +
			`connect-src 'self' https://cdn.jsdelivr.net;`
	}

	// Best-effort title from first H1.
	title := file.Name
	for _, line := range strings.SplitN(string(raw), "\n", 50) {
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Cache-Control", "private, no-store")
	_, err = fmt.Fprintf(w, markdownHTMLTemplate, html.EscapeString(title), htmlBody, script)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return 0, nil
}
