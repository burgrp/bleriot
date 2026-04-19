#!/usr/bin/env python3
"""Convert registers.md to a print-ready PDF via WeasyPrint (CSS paged media)."""

import re
import sys
from pathlib import Path
import markdown
from weasyprint import HTML, CSS
from weasyprint.text.fonts import FontConfiguration

SRC = Path(__file__).parent / "registers.md"
PDF_OUT = Path(__file__).parent / "registers.pdf"

md_text = SRC.read_text()

# ── Build TOC from headings ────────────────────────────────────────────────
def slug(text):
    text = re.sub(r"[`*_\[\]()]", "", text)   # strip markdown inline
    text = text.lower()
    text = re.sub(r"[^\w\s-]", "", text)
    text = re.sub(r"[\s]+", "-", text.strip())
    return text

toc_entries = []
for m in re.finditer(r"^(#{1,3})\s+(.+)$", md_text, re.MULTILINE):
    level = len(m.group(1))
    title = m.group(2).strip()
    # strip inline markdown from title for display
    display = re.sub(r"[`*_]", "", title)
    toc_entries.append((level, display, slug(title)))

toc_html_parts = ['<nav id="toc"><h2>Contents</h2><ol class="toc-list">']
for level, display, anchor in toc_entries:
    cls = f"toc-h{level}"
    toc_html_parts.append(
        f'<li class="{cls}"><a href="#{anchor}">{display}</a></li>'
    )
toc_html_parts.append("</ol></nav>")
toc_html = "\n".join(toc_html_parts)

# ── Convert markdown body, adding id= anchors to headings ─────────────────
# Pre-process: inject {#anchor} for the toc extension
lines = []
for line in md_text.splitlines():
    m = re.match(r"^(#{1,3})\s+(.+)$", line)
    if m:
        anchor = slug(m.group(2).strip())
        lines.append(f'{m.group(0)} {{#{anchor}}}')
    else:
        lines.append(line)
md_anchored = "\n".join(lines)

body_html = markdown.markdown(
    md_anchored,
    extensions=["tables", "fenced_code", "toc", "attr_list"],
    extension_configs={"toc": {"permalink": False}},
)

CSS_TEXT = """
/* ── Fonts ── */
@font-face {
    font-family: 'Source Code Pro';
    src: local('Courier New');
}

:root {
    --accent: #1a3a5c;
    --accent-light: #edf2f8;
    --border: #c8d4de;
    --text: #1a1a1a;
    --code-bg: #f4f6f8;
}

* { box-sizing: border-box; margin: 0; padding: 0; }

body {
    font-family: DejaVu Serif, Georgia, serif;
    font-size: 9.5pt;
    line-height: 1.52;
    color: var(--text);
}

/* ── Page layout ── */
@page {
    size: A4;
    margin: 22mm 22mm 25mm 22mm;

    @bottom-center {
        content: counter(page);
        font-family: DejaVu Sans, Arial, sans-serif;
        font-size: 8.5pt;
        color: #666;
    }
    @bottom-left {
        content: "PAN211x Register Reference";
        font-family: DejaVu Sans, Arial, sans-serif;
        font-size: 8pt;
        color: #999;
    }
    @top-right {
        content: string(section-title);
        font-family: DejaVu Sans, Arial, sans-serif;
        font-size: 8pt;
        color: #999;
        font-style: italic;
    }
}

@page toc-page {
    @bottom-center { content: none; }
    @bottom-left   { content: none; }
    @top-right     { content: none; }
}

@page :first {
    @bottom-center { content: none; }
    @bottom-left   { content: none; }
    @top-right     { content: none; }
}

/* ── TOC ── */
#toc {
    page: toc-page;
    page-break-after: always;
}

#toc h2 {
    font-size: 22pt;
    font-family: DejaVu Sans, Arial, sans-serif;
    color: var(--accent);
    border-bottom: 2.5pt solid var(--accent);
    padding-bottom: 7pt;
    margin-bottom: 16pt;
}

ol.toc-list {
    list-style: none;
    padding: 0;
}

.toc-h1 { margin-top: 7pt; font-weight: bold; font-size: 9.5pt; }
.toc-h2 { margin-top: 4pt; margin-left: 10pt; font-size: 9pt; }
.toc-h3 { margin-left: 22pt; font-size: 8.5pt; color: #444; }

.toc-h1 a, .toc-h2 a, .toc-h3 a {
    color: inherit;
    text-decoration: none;
    display: flex;
    justify-content: space-between;
    gap: 4pt;
}

.toc-h1 a::after,
.toc-h2 a::after,
.toc-h3 a::after {
    content: target-counter(attr(href), page);
    font-family: DejaVu Sans Mono, Courier New, monospace;
    font-size: 8.5pt;
    color: #666;
    font-weight: normal;
    white-space: nowrap;
    flex-shrink: 0;
}

/* ── Content ── */
#content { }

/* Running header for section title */
h2 { string-set: section-title content(); }

/* ── Headings ── */
h1 {
    font-family: DejaVu Sans, Arial, sans-serif;
    font-size: 20pt;
    color: var(--accent);
    border-bottom: 3pt solid var(--accent);
    padding-bottom: 8pt;
    margin-top: 0;
    margin-bottom: 16pt;
    page-break-before: avoid;
}

h2 {
    font-family: DejaVu Sans, Arial, sans-serif;
    font-size: 14pt;
    color: var(--accent);
    border-bottom: 1pt solid var(--border);
    padding-bottom: 4pt;
    margin-top: 20pt;
    margin-bottom: 9pt;
    page-break-after: avoid;
}

h3 {
    font-family: DejaVu Sans, Arial, sans-serif;
    font-size: 10.5pt;
    color: #1e4070;
    margin-top: 14pt;
    margin-bottom: 5pt;
    page-break-after: avoid;
}

h4 {
    font-family: DejaVu Sans, Arial, sans-serif;
    font-size: 9.5pt;
    color: #333;
    margin-top: 9pt;
    margin-bottom: 4pt;
    page-break-after: avoid;
}

p { margin-bottom: 6pt; orphans: 3; widows: 3; }

/* ── Tables ── */
table {
    border-collapse: collapse;
    width: 100%;
    font-size: 8pt;
    font-family: DejaVu Sans Mono, Courier New, monospace;
    margin-bottom: 9pt;
    page-break-inside: avoid;
}

thead tr { background: var(--accent); color: white; }

thead th {
    padding: 4pt 6pt;
    text-align: left;
    font-weight: bold;
    font-family: DejaVu Sans, Arial, sans-serif;
    font-size: 8pt;
    letter-spacing: 0.02em;
}

tbody tr:nth-child(even) { background: var(--accent-light); }
tbody tr:nth-child(odd)  { background: white; }

tbody td {
    padding: 2.5pt 6pt;
    border-bottom: 0.5pt solid var(--border);
    vertical-align: top;
}

/* ── Code ── */
code {
    font-family: DejaVu Sans Mono, Courier New, monospace;
    font-size: 7.5pt;
    background: var(--code-bg);
    padding: 1pt 3pt;
    border-radius: 2pt;
}

pre {
    background: var(--code-bg);
    border: 1pt solid var(--border);
    border-left: 4pt solid var(--accent);
    padding: 7pt 9pt;
    font-family: DejaVu Sans Mono, Courier New, monospace;
    font-size: 7.5pt;
    line-height: 1.38;
    white-space: pre-wrap;
    word-wrap: break-word;
    margin-bottom: 9pt;
    page-break-inside: avoid;
}

pre code { background: none; border: none; padding: 0; }

/* ── Blockquotes ── */
blockquote {
    border-left: 4pt solid #e09020;
    background: #fffbf0;
    padding: 5pt 9pt;
    margin: 7pt 0;
    font-size: 8.5pt;
    page-break-inside: avoid;
}

blockquote p { margin: 0; }

/* ── Lists ── */
ul, ol { padding-left: 14pt; margin-bottom: 6pt; }
li { margin-bottom: 1.5pt; }

/* ── Rules ── */
hr { border: none; border-top: 1pt solid var(--border); margin: 12pt 0; }

strong { font-weight: bold; }
em     { font-style: italic; }
"""

HTML_CONTENT = f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>PAN211x Register Reference</title>
</head>
<body>
{toc_html}
<div id="content">
{body_html}
</div>
</body>
</html>
"""

font_config = FontConfiguration()
html = HTML(string=HTML_CONTENT, base_url=str(Path(__file__).parent))
css  = CSS(string=CSS_TEXT, font_config=font_config)

html.write_pdf(str(PDF_OUT), stylesheets=[css], font_config=font_config)
print(f"PDF written: {PDF_OUT}")
