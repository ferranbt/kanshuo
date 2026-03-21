// Overlay system for displaying semantic subtitle data based on video time

class SubtitleOverlay {
  constructor() {
    this.overlayElement = null;
    this.currentData = null;
    this.videoElement = null;
    this.modalElement = null;
    this.backdropElement = null;
    this.subtitles = []; // Array of subtitle entries with timing
    this.currentSubtitleIndex = -1;
    this.checkInterval = null;
    this.currentSentence = null; // Store the current displayed sentence (hanzi)
    this.currentSentencePinyin = null; // Store the current sentence pinyin
    this.currentSentenceTranslation = null; // Store the current sentence translation
  }

  init() {
    console.log('[Kanshuo Overlay] Initializing overlay system');
    this.findVideoElement();
    this.listenForSubtitles();
  }

  findVideoElement() {
    // Find the video element on the page
    const videos = document.querySelectorAll('video');
    console.log('[Kanshuo Overlay] Looking for video elements, found:', videos.length);
    if (videos.length > 0) {
      this.videoElement = videos[0]; // Use first video
      console.log('[Kanshuo Overlay] Found video element:', this.videoElement);
      return true;
    }
    return false;
  }

  createOverlay() {
    if (this.overlayElement) return; // Already created

    // Create overlay container
    this.overlayElement = document.createElement('div');
    this.overlayElement.id = 'kanshuo-overlay';
    this.overlayElement.style.cssText = `
      position: absolute;
      top: 20px;
      left: 50%;
      transform: translateX(-50%);
      pointer-events: none;
      z-index: 9999999;
      display: none;
      text-align: center;
      width: 90%;
      max-width: 1200px;
    `;

    // Find the video container (YouTube specific)
    const videoContainer = this.videoElement.closest('.html5-video-container') || this.videoElement.parentElement;

    // Ensure the container has relative positioning
    const containerPosition = window.getComputedStyle(videoContainer).position;
    if (containerPosition === 'static') {
      videoContainer.style.position = 'relative';
    }

    videoContainer.appendChild(this.overlayElement);
    console.log('[Kanshuo Overlay] Overlay created and appended to video container');

    // Create modal for word details
    this.createModal(document.body);
  }

  createModal(parent) {
    this.modalElement = document.createElement('div');
    this.modalElement.id = 'kanshuo-modal';
    this.modalElement.style.cssText = `
      position: fixed;
      top: 20px;
      right: 20px;
      background: rgba(0, 0, 0, 0.95);
      border: 2px solid #00ff00;
      border-radius: 8px;
      padding: 16px;
      z-index: 10000000;
      display: none;
      width: 280px;
      max-width: 90vw;
      box-shadow: 0 8px 32px rgba(0, 0, 0, 0.8);
      font-family: 'Microsoft YaHei', 'PingFang SC', sans-serif;
    `;

    // Prevent clicks on modal from bubbling to video player
    this.modalElement.addEventListener('click', (e) => {
      e.stopPropagation();
    });

    parent.appendChild(this.modalElement);

    // Create backdrop
    this.backdropElement = document.createElement('div');
    this.backdropElement.style.cssText = `
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(0, 0, 0, 0.7);
      z-index: 9999999;
      display: none;
    `;
    parent.appendChild(this.backdropElement);

    // Close modal when clicking backdrop
    this.backdropElement.addEventListener('click', () => this.closeModal());
  }

  listenForSubtitles() {
    // Listen for subtitles loaded event from content script
    document.addEventListener('kanshuo-subtitles-loaded', (event) => {
      console.log('[Kanshuo Overlay] 📩 Received subtitles');
      this.loadSubtitles(event.detail.subtitles);
    });
  }

  loadSubtitles(subtitles) {
    this.subtitles = subtitles;
    this.currentSubtitleIndex = -1;
    console.log('[Kanshuo Overlay] Loaded', subtitles.length, 'subtitle entries');

    // Find video element if not already found
    if (!this.videoElement) {
      if (!this.findVideoElement()) {
        console.warn('[Kanshuo Overlay] ⚠️ No video element found, will retry...');
        // Retry after a short delay with exponential backoff
        let retries = 0;
        const maxRetries = 10;
        const retryInterval = setInterval(() => {
          retries++;
          console.log('[Kanshuo Overlay] Retry', retries, 'of', maxRetries);
          if (this.findVideoElement()) {
            console.log('[Kanshuo Overlay] ✅ Found video element on retry', retries);
            clearInterval(retryInterval);
            this.loadSubtitles(subtitles);
          } else if (retries >= maxRetries) {
            console.error('[Kanshuo Overlay] ❌ Failed to find video element after', maxRetries, 'retries');
            clearInterval(retryInterval);
          }
        }, 500);
        return;
      }
    }

    // Create overlay if needed
    this.createOverlay();

    // Start checking for subtitle display
    this.startSubtitleCheck();
  }

  startSubtitleCheck() {
    // Clear any existing interval
    if (this.checkInterval) {
      clearInterval(this.checkInterval);
    }

    // Check every 100ms for subtitle timing
    this.checkInterval = setInterval(() => {
      this.checkCurrentSubtitle();
    }, 100);

    console.log('[Kanshuo Overlay] Started subtitle timing check');
  }

  checkCurrentSubtitle() {
    if (!this.videoElement || this.subtitles.length === 0) {
      return;
    }

    const currentTime = this.videoElement.currentTime;

    // Find subtitle that matches current time
    // Adding a small tolerance to catch subtitles better
    const tolerance = 0.2; // 200ms tolerance
    let matchingIndex = -1;

    for (let i = 0; i < this.subtitles.length; i++) {
      const sub = this.subtitles[i];

      // Handle case where start_position == end_position (show for at least 2 seconds)
      let effectiveEnd = sub.end_position;
      if (sub.start_position === sub.end_position) {
        effectiveEnd = sub.start_position + 2.0; // Show for 2 seconds minimum
      }

      // Check if current time is within subtitle range (with tolerance)
      if (currentTime >= (sub.start_position - tolerance) && currentTime <= (effectiveEnd + tolerance)) {
        matchingIndex = i;
        break;
      }
    }

    // Display subtitle if found and different from current
    if (matchingIndex !== -1 && matchingIndex !== this.currentSubtitleIndex) {
      const sub = this.subtitles[matchingIndex];
      console.log('[Kanshuo Overlay] ✅ Showing subtitle:', sub.text,
                  '| Time:', currentTime.toFixed(2),
                  '| Range:', sub.start_position, '-', sub.end_position,
                  '| Index:', matchingIndex);
      this.currentSubtitleIndex = matchingIndex;
      this.displaySubtitle(sub);
    } else if (matchingIndex === -1 && this.currentSubtitleIndex !== -1) {
      // Hide subtitle if no match and currently showing one
      console.log('[Kanshuo Overlay] ⏹️ Hiding subtitle at time', currentTime.toFixed(2));
      this.currentSubtitleIndex = -1;
      this.hideSubtitle();
    }
  }

  displaySubtitle(subtitle) {
    if (!this.overlayElement) return;

    console.log('[Kanshuo Overlay] 📺 Displaying subtitle:', subtitle.text);

    // Store current sentence for context when saving words
    this.currentSentence = subtitle.simplified || subtitle.text;
    this.currentSentencePinyin = subtitle.pinyin || ''; // Store pinyin too

    // Parse annotation from the raw data structure
    const annotation = this.parseAnnotation(subtitle.annotation);

    // Store translation if annotation exists
    this.currentSentenceTranslation = annotation ? annotation.translation : '';

    if (!annotation) {
      // No annotation, just display plain text
      this.overlayElement.innerHTML = `
        <div class="kanshuo-region" style="pointer-events: auto;">
          <div class="kanshuo-text" style="color: white; font-size: 28px;">
            ${subtitle.simplified || subtitle.text}
          </div>
        </div>
      `;
    } else {
      // Display with annotation
      const subtitleHTML = this.createSemanticHTML(subtitle.simplified || subtitle.text, annotation);
      this.overlayElement.innerHTML = subtitleHTML;
      this.attachWordClickHandlers();
    }

    this.overlayElement.style.display = 'block';
    console.log('[Kanshuo Overlay] ✅ Overlay display set to block, visibility should be visible now');
  }

  hideSubtitle() {
    if (this.overlayElement) {
      this.overlayElement.style.display = 'none';
    }
  }

  // Parse the annotation data from the server format
  parseAnnotation(annotationData) {
    if (!annotationData || !annotationData.WAnalysis) {
      return null;
    }

    // Convert WAnalysis array to words array
    const words = annotationData.WAnalysis
      .filter(w => w.Simplified && w.Simplified.trim() !== '') // Skip empty/whitespace
      .map(w => ({
        text: w.Simplified,
        pinyin: w.Pinyin || '',
        meaning: Array.isArray(w.Meaning) ? w.Meaning.join(', ') : (w.Meaning || ''),
        wordType: this.mapPosTag(w.Tag),
        grammar: ''
      }));

    return {
      originalText: annotationData.Translation || '',
      words: words,
      translation: annotationData.Translation || ''
    };
  }

  // Map POS tags to readable types
  mapPosTag(tag) {
    const tagMap = {
      'n': 'noun',
      'nr': 'noun',
      'v': 'verb',
      'a': 'adjective',
      'd': 'adverb',
      'r': 'pronoun',
      'p': 'preposition',
      'c': 'conjunction',
      'u': 'particle',
      'x': 'other'
    };
    return tagMap[tag] || tag || 'unknown';
  }

  attachWordClickHandlers() {
    const words = this.overlayElement.querySelectorAll('.kanshuo-word');
    words.forEach(word => {
      word.addEventListener('click', (e) => {
        e.stopPropagation();
        const wordData = {
          text: word.textContent.trim(),
          pos: word.getAttribute('data-pos'),
          tooltip: word.getAttribute('data-tooltip')
        };
        this.showModal(wordData);
      });
    });
  }

  showModal(wordData) {
    console.log('[Kanshuo Overlay] 📝 Showing modal for:', wordData);

    // Parse tooltip (format: "pinyin - meaning (grammar)")
    const parts = wordData.tooltip.split(' - ');
    const pinyin = parts[0] || '';
    const meaningPart = parts[1] || '';
    const [meaning, grammar] = meaningPart.split(' (');

    const modalHTML = `
      <div style="color: white;">
        <h2 style="font-size: 24px; margin: 0 0 8px 0; color: #00ff00;">
          ${wordData.text}
        </h2>
        <div style="font-size: 14px; margin-bottom: 6px; color: #aaa;">
          ${pinyin}
        </div>
        <div style="font-size: 13px; margin-bottom: 6px;">
          ${meaning}
        </div>
        <div style="font-size: 11px; margin-bottom: 12px; color: #999;">
          ${wordData.pos}${grammar ? ` • ${grammar.replace(')', '')}` : ''}
        </div>
        <button id="kanshuo-save-btn" style="
          background: #00ff00;
          color: black;
          border: none;
          padding: 8px 16px;
          font-size: 14px;
          font-weight: bold;
          border-radius: 4px;
          cursor: pointer;
          width: 100%;
        ">
          💾 Save
        </button>
        <button id="kanshuo-close-btn" style="
          background: transparent;
          color: #999;
          border: 1px solid #666;
          padding: 6px 12px;
          font-size: 12px;
          border-radius: 4px;
          cursor: pointer;
          width: 100%;
          margin-top: 6px;
        ">
          Close
        </button>
      </div>
    `;

    this.modalElement.innerHTML = modalHTML;
    this.modalElement.style.display = 'block';
    this.backdropElement.style.display = 'block';

    // Add event listeners with stop propagation
    document.getElementById('kanshuo-save-btn').addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      this.saveWord({
        text: wordData.text,
        pinyin,
        meaning,
        pos: wordData.pos,
        sentence: this.currentSentence, // Add sentence context (hanzi)
        sentencePinyin: this.currentSentencePinyin, // Add sentence pinyin
        sentenceTranslation: this.currentSentenceTranslation // Add sentence translation
      });
    });
    document.getElementById('kanshuo-close-btn').addEventListener('click', (e) => {
      e.stopPropagation();
      e.preventDefault();
      this.closeModal();
    });
  }

  closeModal() {
    if (this.modalElement) {
      this.modalElement.style.display = 'none';
    }
    if (this.backdropElement) {
      this.backdropElement.style.display = 'none';
    }
  }

  async saveWord(wordData) {
    console.log('[Kanshuo Overlay] 💾 Saving word with context:', wordData);

    // Send message to background script to make the request
    chrome.runtime.sendMessage({
      type: 'SAVE_WORD',
      data: wordData
    }, (response) => {
      const btn = document.getElementById('kanshuo-save-btn');
      if (response && response.success) {
        console.log('[Kanshuo Overlay] ✅ Word saved successfully');
        btn.textContent = '✅ Saved!';
        btn.style.background = '#4CAF50';
        setTimeout(() => this.closeModal(), 1000);
      } else {
        const errorMsg = response?.error || 'Unknown error';
        console.error('[Kanshuo Overlay] ❌ Failed to save word:', errorMsg);
        btn.textContent = '❌ Failed';
        btn.style.background = '#f44336';
      }
    });
  }

  getPosColor(pos) {
    // Color code by part of speech
    const posColors = {
      'noun': '#4A90E2',      // Blue
      'verb': '#7ED321',      // Green
      'adj': '#F5A623',       // Yellow
      'adjective': '#F5A623', // Yellow
      'adv': '#FF6B35',       // Orange
      'adverb': '#FF6B35',    // Orange
      'pronoun': '#BD10E0',   // Purple
      'prep': '#50E3C2',      // Cyan
      'preposition': '#50E3C2', // Cyan
      'conj': '#F8E71C',      // Bright yellow
      'conjunction': '#F8E71C', // Bright yellow
      'particle': '#D0D0D0',  // Light gray
    };

    return posColors[pos.toLowerCase()] || '#FFFFFF'; // Default white
  }

  createSemanticHTML(text, annotation) {
    if (!annotation || !annotation.words || annotation.words.length === 0) {
      return `
        <div class="kanshuo-region">
          <div class="kanshuo-text" style="color: white;">${text}</div>
        </div>
      `;
    }

    const wordsHTML = annotation.words.map(word => {
      const color = this.getPosColor(word.wordType);
      const meaning = word.meaning || 'no meaning';
      const pinyin = word.pinyin || '';
      const grammar = word.grammar || '';
      const tooltip = grammar ? `${pinyin} - ${meaning} (${grammar})` : `${pinyin} - ${meaning}`;

      return `<span class="kanshuo-word"
                   data-pos="${word.wordType}"
                   data-tooltip="${tooltip}"
                   style="color: ${color};">
                ${word.text}
              </span>`;
    }).join('');

    return `
      <style>
        #kanshuo-overlay {
          font-family: 'Microsoft YaHei', 'PingFang SC', sans-serif;
        }
        .kanshuo-region {
          background: rgba(0, 0, 0, 0.85);
          padding: 12px 20px;
          border-radius: 8px;
          text-align: center;
          display: inline-block;
          margin: 0 auto;
          pointer-events: auto;
        }
        .kanshuo-text {
          font-size: 28px;
          line-height: 1.6;
        }
        .kanshuo-translation {
          font-size: 18px;
          color: #aaaaaa;
          margin-top: 8px;
          font-style: italic;
        }
        .kanshuo-word {
          cursor: pointer;
          padding: 4px 6px;
          border-radius: 4px;
          transition: all 0.2s;
          position: relative;
          display: inline-block;
          font-weight: 600;
        }
        .kanshuo-word:hover {
          background-color: rgba(255, 255, 255, 0.2);
          transform: translateY(-2px);
        }
        .kanshuo-word:hover::after {
          content: attr(data-pos) ": " attr(data-tooltip);
          position: absolute;
          background: rgba(0, 0, 0, 0.95);
          color: #00ff00;
          padding: 10px 14px;
          border-radius: 6px;
          font-size: 14px;
          white-space: normal;
          word-wrap: break-word;
          max-width: 300px;
          max-height: 150px;
          overflow-y: auto;
          top: 100%;
          left: 50%;
          transform: translateX(-50%);
          margin-top: 8px;
          z-index: 1000000;
          border: 2px solid #00ff00;
          font-weight: normal;
          box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
          line-height: 1.4;
        }
        .kanshuo-word:hover::before {
          content: '';
          position: absolute;
          top: 100%;
          left: 50%;
          transform: translateX(-50%);
          border: 6px solid transparent;
          border-bottom-color: #00ff00;
          margin-top: 2px;
        }
      </style>
      <div class="kanshuo-region">
        <div class="kanshuo-text">${wordsHTML}</div>
        ${annotation.translation ? `<div class="kanshuo-translation">${annotation.translation}</div>` : ''}
      </div>
    `;
  }
}

// Global overlay instance
let globalOverlay = null;

// Function to get or create overlay instance
function getOverlay() {
  if (!globalOverlay) {
    console.log('[Kanshuo Overlay] Creating overlay instance');
    globalOverlay = new SubtitleOverlay();
    globalOverlay.init();
  }
  return globalOverlay;
}

// Ensure overlay is initialized when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    getOverlay();
  });
} else {
  getOverlay();
}
