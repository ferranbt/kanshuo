package anki

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Duckduckgot/gtts"
	"github.com/atselvan/ankiconnect"
)

type Client struct {
	logger *slog.Logger
	client *ankiconnect.Client
}

func NewClient(logger *slog.Logger) *Client {
	return &Client{
		logger: logger,
		client: ankiconnect.NewClient(),
	}
}

type Word struct {
	Text                string
	Pinyin              string
	Meaning             string
	Pos                 string
	Sentence            string // Example sentence context (hanzi)
	SentencePinyin      string // Example sentence pinyin
	SentenceTranslation string // Example sentence translation
}

func (c *Client) generateAudio(text string) ([]byte, error) {
	speech := gtts.Speech{
		Folder:   "",
		Language: "zh-CN", // Chinese (Simplified, Mandarin)
	}

	data, err := speech.SpeakB(text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate audio: %v", err)
	}

	return data, nil
}

// SaveWord saves a Chinese word to the specified Anki deck with audio pronunciation
func (c *Client) SaveWord(deckName string, word Word) error {
	decks, err := c.client.Decks.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get Anki decks (is Anki running with AnkiConnect enabled?): %v", err)
	}

	deckExists := false
	for _, deck := range *decks {
		if deck == deckName {
			deckExists = true
			break
		}
	}

	if !deckExists {
		if err := c.client.Decks.Create(deckName); err != nil {
			return fmt.Errorf("failed to create Anki deck: %v", err)
		}
	}

	// Generate audio for the Chinese text
	audioData, audioErr := c.generateAudio(word.Text)
	if audioErr != nil {
		c.logger.Error("audio could not be generated for word", "err", err)
		audioData = nil
	}

	// Generate audio for the sentence context if provided
	var sentenceAudioData []byte
	if word.Sentence != "" {
		var err error
		sentenceAudioData, err = c.generateAudio(word.Sentence)
		if err != nil {
			c.logger.Error("audio could not be generated for sentence", "err", err)
			sentenceAudioData = nil
		}
	}

	// Create a note with Chinese word information
	// Front: Chinese word + Pinyin
	// Back: Meaning + Part of Speech + Example Sentence (if provided)
	front := fmt.Sprintf("%s<br><i>%s</i>", word.Text, word.Pinyin)
	back := fmt.Sprintf("%s<br><small>(%s)</small>", word.Meaning, word.Pos)

	if word.Sentence != "" {
		back += fmt.Sprintf("<br><br><b>Example:</b><br>%s", word.Sentence)
		if word.SentencePinyin != "" {
			back += fmt.Sprintf("<br><i>%s</i>", word.SentencePinyin)
		}
		if word.SentenceTranslation != "" {
			back += fmt.Sprintf("<br><small>%s</small>", word.SentenceTranslation)
		}
	}

	note := ankiconnect.Note{
		DeckName:  deckName,
		ModelName: "Basic",
		Fields: ankiconnect.Fields{
			"Front": front,
			"Back":  back,
		},
		Tags: []string{"kanshuo", "chinese"},
	}

	// Add audio if we generated it successfully
	if audioData != nil || sentenceAudioData != nil {
		note.Audio = []ankiconnect.Audio{}

		if audioData != nil {
			audioFilename := fmt.Sprintf("kanshuo_%s.mp3", word.Text)
			note.Audio = append(note.Audio, ankiconnect.Audio{
				Data:     base64.StdEncoding.EncodeToString(audioData),
				Filename: audioFilename,
				Fields:   []string{"Front"},
			})
		}

		if sentenceAudioData != nil {
			safeSentence := word.Sentence
			if len(safeSentence) > 20 {
				safeSentence = safeSentence[:20]
			}
			sentenceFilename := fmt.Sprintf("kanshuo_sentence_%s.mp3", safeSentence)
			note.Audio = append(note.Audio, ankiconnect.Audio{
				Data:     base64.StdEncoding.EncodeToString(sentenceAudioData),
				Filename: sentenceFilename,
				Fields:   []string{"Back"},
			})
		}
	}

	if err := c.client.Notes.Add(note); err != nil {
		errMsg := fmt.Sprintf("%v", err)

		if strings.Contains(errMsg, "duplicate") {
			return fmt.Errorf("note already exists in Anki (duplicate)")
		}
		return fmt.Errorf("failed to add note to Anki: %v", err)
	}

	return nil
}
