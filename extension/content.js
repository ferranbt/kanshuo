// Content script - detects YouTube and Bilibili videos and triggers subtitle loading

// Prevent multiple initializations
if (window.kanshuoInitialized) {
  console.log('[Kanshuo] Already initialized, skipping');
} else {
  window.kanshuoInitialized = true;
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

// Extract Bilibili video ID from URL
function getBilibiliVideoID() {
  const url = window.location.href;

  // Check if we're on Bilibili
  if (!url.includes('bilibili.com/video/')) {
    return null;
  }

  // Extract BV ID from URL (e.g., BV1NUfCB1E7n)
  const match = url.match(/\/video\/(BV[a-zA-Z0-9]+)/);
  return match ? match[1] : null;
}

// Get video ID from either platform
function getVideoID() {
  return getYouTubeVideoID() || getBilibiliVideoID();
}

// Determine platform
function getPlatform() {
  if (window.location.href.includes('youtube.com')) return 'youtube';
  if (window.location.href.includes('bilibili.com')) return 'bilibili';
  return null;
}

// Monitor for URL changes (both YouTube and Bilibili are SPAs)
let currentVideoID = null;
let currentPlatform = null;
let hasSubtitles = false;
let subtitlesEnabled = false;

function getState() {
  return {
    platform: currentPlatform,
    videoID: currentVideoID,
    hasSubtitles: hasSubtitles,
    subtitlesEnabled: subtitlesEnabled
  };
}

function updatePopup() {
  chrome.runtime.sendMessage({
    type: 'STATE_UPDATE',
    state: getState()
  });
  updateIcon();
}

function updateIcon() {
  let iconState = 'none';

  if (currentPlatform && currentVideoID) {
    if (subtitlesEnabled) {
      iconState = 'enabled';
    } else if (hasSubtitles) {
      iconState = 'ready';
    } else {
      iconState = 'available';
    }
  }

  chrome.runtime.sendMessage({
    type: 'UPDATE_ICON',
    state: iconState
  });
}

function checkForNewVideo() {
  const videoID = getVideoID();
  const platform = getPlatform();

  if (videoID && videoID !== currentVideoID) {
    console.log(`[Kanshuo] New ${platform} video detected:`, videoID);
    currentVideoID = videoID;
    currentPlatform = platform;
    hasSubtitles = false;
    subtitlesEnabled = false;

    updatePopup();

    // Request subtitles from background script
    chrome.runtime.sendMessage({
      type: 'LOAD_SUBTITLES',
      videoID: videoID,
      platform: platform
    });
  } else if (!videoID && currentVideoID) {
    // Left video page
    console.log('[Kanshuo] Left video page');
    currentVideoID = null;
    currentPlatform = null;
    hasSubtitles = false;
    subtitlesEnabled = false;

    updatePopup();
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

// Listen for messages
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message.type === 'SUBTITLES_LOADED') {
    console.log('[Kanshuo] Received subtitles:', message.subtitles.length, 'entries');
    hasSubtitles = true;
    updatePopup();

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
    hasSubtitles = false;
    updatePopup();
  }

  if (message.type === 'GET_STATE') {
    sendResponse(getState());
    return true;
  }

  if (message.type === 'TOGGLE_SUBTITLES') {
    subtitlesEnabled = message.enabled;
    console.log('[Kanshuo] Subtitles', subtitlesEnabled ? 'enabled' : 'disabled');

    // Dispatch event for overlay.js to show/hide
    const event = new CustomEvent('kanshuo-toggle-subtitles', {
      detail: { enabled: subtitlesEnabled }
    });
    document.dispatchEvent(event);

    updatePopup();
  }
});

} // End of initialization guard
