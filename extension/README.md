# Kanshuo Chrome Extension

## Installation

1. Open Chrome and go to `chrome://extensions/`
2. Enable "Developer mode" (toggle in top right)
3. Click "Load unpacked"
4. Select this `extension/` folder
5. The extension should now be loaded

## Usage

1. Start the Go server: `go run main.go`
2. Open any website with video (YouTube, Netflix, etc.)
3. Play the video
4. Check the Go server terminal for events
5. Screenshots will be saved to `screenshots/` folder

## Debugging

- Check extension logs: Right-click extension icon → "Inspect service worker"
- Check content script logs: Open DevTools on any page → Console tab (look for `[Kanshuo]` messages)
- Check server: Visit http://localhost:8080/status
