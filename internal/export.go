package internal

import (
	"fmt"
	"html"
	"strings"
)

func RenderPage(id string, subtitles []*Subtitle) (string, error) {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  body { font-family: serif; margin: 2cm; font-size: 16px; line-height: 1.6; }
  .entry { display: inline-block; margin-bottom: 18px; text-align: center; }
  .sep { color: #ccc; margin: 0 10px; }
  .pinyin { display: none; font-size: 0.75em; color: #888; }
  #toggleControls { position: fixed; top: 16px; right: 16px; font-family: sans-serif; font-size: 13px; padding: 4px 10px; cursor: pointer; border: 1px solid #ccc; border-radius: 4px; background: #f5f5f5; }
  #controlsPanel { position: fixed; top: 46px; right: 16px; display: none; flex-direction: column; gap: 10px; background: #f5f5f5; padding: 10px 14px; border-radius: 6px; font-family: sans-serif; font-size: 13px; border: 1px solid #ddd; }
  #controlsPanel label { display: flex; align-items: center; gap: 8px; cursor: pointer; }
  #controlsPanel input[type=range] { width: 80px; cursor: pointer; }
  #title { margin-bottom: 10px; }
</style>
</head>
<body>

<button id="toggleControls" onclick="var p = document.getElementById('controlsPanel'); p.style.display = p.style.display === 'flex' ? 'none' : 'flex'">⚙︎ Controls</button>

<div id="controlsPanel">
  <label>
    Font: <span id="sizeLabel">16</span>px
    <input type="range" id="fontSize" min="8" max="48" value="16"
      oninput="document.getElementById('sizeLabel').textContent = this.value; document.querySelectorAll('.simplified').forEach(el => el.style.fontSize = this.value + 'px')">
  </label>
  <label>
    Letter gap: <span id="gapLabel">0</span>px
    <input type="range" id="letterGap" min="0" max="20" value="0"
      oninput="document.getElementById('gapLabel').textContent = this.value; document.querySelectorAll('.simplified').forEach(el => el.style.letterSpacing = this.value + 'px')">
  </label>
  <label>
    Line gap: <span id="lineGapLabel">14</span>px
    <input type="range" id="lineGap" min="0" max="60" value="18"
      oninput="document.getElementById('lineGapLabel').textContent = this.value; document.querySelectorAll('.entry').forEach(el => el.style.marginBottom = this.value + 'px')">
  </label>
  <label>
    <input type="checkbox" id="pinyinToggle" onchange="togglePinyin(this.checked)">
    Show pinyin
  </label>
</div>
<div id="title">` + id + `</div>
`)

	for i, s := range subtitles {
		if i > 0 {
			sb.WriteString(`<span class="sep">·</span>`)
		}
		fmt.Fprintf(&sb,
			`<span class="entry"><span class="simplified">%s</span><span class="pinyin">%s</span></span>`,
			html.EscapeString(s.Simplified),
			html.EscapeString(s.Pinyin),
		)
	}

	sb.WriteString(`
</p>

<script>
  function togglePinyin(show) {
    document.querySelectorAll('.pinyin').forEach(el => el.style.display = show ? 'block' : 'none');
  }
</script>
</body>
</html>`)

	return sb.String(), nil
}
