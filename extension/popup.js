// Popup script - manages the extension popup UI

let currentState = {
  platform: null,
  videoID: null,
  hasSubtitles: false,
  subtitlesEnabled: false,
  showTranslation: true
};

// Update the popup UI based on current state
function updateUI() {
  const content = document.getElementById('content');

  if (!currentState.platform || !currentState.videoID) {
    // Not on a supported video page
    content.innerHTML = `
      <div class="status">
        <div class="status-indicator gray"></div>
        <span>Not on a video page</span>
      </div>
      <div class="message">
        Navigate to a YouTube or Bilibili video to see subtitles
      </div>
    `;
    return;
  }

  // On a supported video page
  let statusColor, statusText;

  if (currentState.subtitlesEnabled) {
    statusColor = 'blue';
    statusText = 'Subtitles enabled';
  } else if (currentState.hasSubtitles) {
    statusColor = 'green';
    statusText = 'Subtitles available';
  } else {
    statusColor = 'yellow';
    statusText = 'No subtitles found';
  }

  content.innerHTML = `
    <div class="status">
      <div class="status-indicator ${statusColor}"></div>
      <span>${statusText}</span>
    </div>
    <div class="video-info">
      <div><strong>${currentState.platform === 'youtube' ? 'YouTube' : 'Bilibili'}</strong></div>
      <div class="video-id">${currentState.videoID}</div>
    </div>
    <div id="controls"></div>
  `;

  const controls = document.getElementById('controls');

  if (currentState.hasSubtitles) {
    // Has subtitles - show enable/disable button
    if (currentState.subtitlesEnabled) {
      controls.innerHTML = `
        <button class="btn-disable" id="toggleBtn">Disable Subtitles</button>
        <label style="display: flex; align-items: center; gap: 8px; padding: 8px; cursor: pointer;">
          <input type="checkbox" id="translationCheckbox" ${currentState.showTranslation ? 'checked' : ''}>
          <span style="font-size: 13px;">Show English translation</span>
        </label>
      `;
      document.getElementById('translationCheckbox').addEventListener('change', toggleTranslation);
    } else {
      controls.innerHTML = `
        <button class="btn-enable" id="toggleBtn">Enable Subtitles</button>
      `;
    }

    document.getElementById('toggleBtn').addEventListener('click', toggleSubtitles);
  } else {
    // No subtitles - show download button (disabled for now)
    controls.innerHTML = `
      <button class="btn-download" id="downloadBtn" disabled>Download Subtitles</button>
      <div style="font-size: 12px; color: #999; text-align: center; margin-top: 8px;">
        Coming soon
      </div>
    `;
  }
}

// Toggle subtitles on/off
function toggleSubtitles() {
  const newState = !currentState.subtitlesEnabled;

  // Send message to content script
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs[0]) {
      chrome.tabs.sendMessage(tabs[0].id, {
        type: 'TOGGLE_SUBTITLES',
        enabled: newState
      });

      currentState.subtitlesEnabled = newState;
      updateUI();
    }
  });
}

// Toggle translation visibility
function toggleTranslation(event) {
  const showTranslation = event.target.checked;

  // Send message to content script
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs[0]) {
      chrome.tabs.sendMessage(tabs[0].id, {
        type: 'TOGGLE_TRANSLATION',
        showTranslation: showTranslation
      });

      currentState.showTranslation = showTranslation;
    }
  });
}

// Request current state from content script
function requestState() {
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    if (tabs[0]) {
      chrome.tabs.sendMessage(tabs[0].id, {
        type: 'GET_STATE'
      }, (response) => {
        if (response) {
          currentState = response;
          updateUI();
        }
      });
    }
  });
}

// Listen for state updates from content script
chrome.runtime.onMessage.addListener((message) => {
  if (message.type === 'STATE_UPDATE') {
    currentState = message.state;
    updateUI();
  }
});

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  requestState();
});
