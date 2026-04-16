// Background service worker - fetches pre-processed subtitles from server

const SERVER_URL = 'http://localhost:8080';

console.log('[Kanshuo Background] 🚀 Service worker started');

// Update extension icon based on state
function updateIcon(tabId, state) {
  // States: 'none' (gray), 'available' (yellow), 'ready' (green), 'enabled' (blue)
  const colors = {
    none: '#9e9e9e',      // Gray - not on video page
    available: '#ffc107', // Yellow - on video page, no subs
    ready: '#4caf50',     // Green - subs available
    enabled: '#2196f3'    // Blue - subs enabled
  };

  const color = colors[state] || colors.none;

  chrome.action.setBadgeBackgroundColor({ color: color, tabId: tabId });
  chrome.action.setBadgeText({ text: state === 'enabled' ? '●' : '', tabId: tabId });
}

// Listen for messages from content scripts
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  console.log('[Kanshuo Background] Received message:', message.type, 'from tab', sender.tab?.id);

  if (message.type === 'LOAD_SUBTITLES') {
    // Fetch subtitles for the video ID
    const videoID = message.videoID;
    console.log('[Kanshuo Background] Loading subtitles for video:', videoID);

    loadSubtitles(videoID, sender.tab.id);
  }

  if (message.type === 'SAVE_WORD') {
    // Handle save word request
    console.log('[Kanshuo Background] Saving word:', message.data);
    saveWord(message.data, sendResponse);
    return true; // Keep channel open for async response
  }

  if (message.type === 'UPDATE_ICON') {
    // Update icon based on state
    updateIcon(sender.tab.id, message.state);
  }
});

// Fetch subtitles from server
async function loadSubtitles(videoID, tabId) {
  try {
    console.log('[Kanshuo Background] Fetching subtitles from server...');

    const response = await fetch(`${SERVER_URL}/videos/${videoID}/subs`);

    if (!response.ok) {
      console.error('[Kanshuo Background] ❌ Failed to fetch subtitles:', response.status);
      return;
    }

    const data = await response.json();

    if (!data.available) {
      console.log('[Kanshuo Background] ℹ️ No subtitles available for video:', videoID);
      // Notify content script
      chrome.tabs.sendMessage(tabId, {
        type: 'SUBTITLES_NOT_AVAILABLE',
        videoID: videoID
      }).catch(err => {
        console.warn('[Kanshuo Background] Could not send message to tab:', err);
      });
      return;
    }

    console.log('[Kanshuo Background] ✅ Loaded', data.subtitles.length, 'subtitles');

    // Send subtitles to content script
    chrome.tabs.sendMessage(tabId, {
      type: 'SUBTITLES_LOADED',
      videoID: videoID,
      subtitles: data.subtitles
    }).catch(err => {
      console.warn('[Kanshuo Background] Could not send subtitles to tab:', err);
    });
  } catch (error) {
    console.warn('[Kanshuo Background] ⚠️ Could not fetch subtitles:', error.message);
  }
}

// Save word to server
async function saveWord(wordData, sendResponse) {
  console.log('[Kanshuo Background] Sending word to server:', wordData);
  try {
    const response = await fetch(`${SERVER_URL}/save`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(wordData)
    });

    const data = await response.json();

    if (response.ok) {
      console.log('[Kanshuo Background] ✅ Word saved successfully');
      sendResponse({ success: true });
    } else {
      const errorMsg = data.error || `Server error: ${response.status}`;
      console.error('[Kanshuo Background] ❌ Server returned error:', errorMsg);
      sendResponse({ success: false, error: errorMsg });
    }
  } catch (error) {
    console.error('[Kanshuo Background] ⚠️ Could not save word:', error.message);
    sendResponse({ success: false, error: error.message });
  }
}
