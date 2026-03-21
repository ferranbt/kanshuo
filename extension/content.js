// Content script - detects YouTube videos and triggers subtitle loading

console.log('[Kanshuo] Content script loaded');

// Extract YouTube video ID from URL
function getYouTubeVideoID() {
  const url = window.location.href;

  // Check if we're on YouTube
  if (!url.includes('youtube.com/watch')) {
    return null;
  }

  // Extract video ID from URL parameter
  const urlParams = new URLSearchParams(window.location.search);
  return urlParams.get('v');
}

// Monitor for URL changes (YouTube is a SPA)
let currentVideoID = null;

function checkForNewVideo() {
  const videoID = getYouTubeVideoID();

  if (videoID && videoID !== currentVideoID) {
    console.log('[Kanshuo] New YouTube video detected:', videoID);
    currentVideoID = videoID;

    // Request subtitles from background script
    chrome.runtime.sendMessage({
      type: 'LOAD_SUBTITLES',
      videoID: videoID
    });
  } else if (!videoID && currentVideoID) {
    // Left YouTube video page
    console.log('[Kanshuo] Left YouTube video page');
    currentVideoID = null;
  }
}

// Check immediately
checkForNewVideo();

// Monitor URL changes (YouTube uses pushState for navigation)
let lastUrl = location.href;
new MutationObserver(() => {
  const url = location.href;
  if (url !== lastUrl) {
    lastUrl = url;
    checkForNewVideo();
  }
}).observe(document, { subtree: true, childList: true });

// Also check periodically as backup
setInterval(checkForNewVideo, 2000);

// Listen for subtitle data from background script
chrome.runtime.onMessage.addListener((message) => {
  if (message.type === 'SUBTITLES_LOADED') {
    console.log('[Kanshuo] Received subtitles:', message.subtitles.length, 'entries');

    // Dispatch event for overlay.js to handle
    const event = new CustomEvent('kanshuo-subtitles-loaded', {
      detail: {
        videoID: message.videoID,
        subtitles: message.subtitles
      }
    });
    document.dispatchEvent(event);
  }

  if (message.type === 'SUBTITLES_NOT_AVAILABLE') {
    console.log('[Kanshuo] No subtitles available for video:', message.videoID);
  }
});
