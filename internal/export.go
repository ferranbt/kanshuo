package internal

import (
	"fmt"
	"html"
	"os"
	"strings"
)

func ExportSubtitlesPrintable(subtitlesPath string, outputPath string) error {
	subtitles, err := readSubtitles(subtitlesPath)
	if err != nil {
		return err
	}

	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  body { font-family: serif; margin: 2cm; font-size: 14px; line-height: 1.6; }
  .entry { display: inline-block; margin-bottom: 10px }
  .sep { color: #ccc; margin: 0 10px; }
</style>
</head>
<body>
<p>
`)

	for i, s := range subtitles {
		if i > 0 {
			sb.WriteString(`<span class="sep">·</span>`)
		}
		fmt.Fprintf(&sb, `<span class="entry">%s</span>`, html.EscapeString(s.Simplified))
	}

	sb.WriteString(`
</p>
</body>
</html>`)

	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}
