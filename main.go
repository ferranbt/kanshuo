package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ferranbt/kanshuo/internal"
	"github.com/ferranbt/kanshuo/internal/anki"
	"github.com/ferranbt/kanshuo/internal/testutil"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
)

const (
	defaultAnkiDeck = "kanshuo"
)

func main() {
	var (
		archivePath string
		videoPath   string
		traditional bool
	)

	rootCmd := &cobra.Command{
		Use:   "kanshuo",
		Short: "Kanshuo - Chinese Subtitle Learning Tool",
		Long:  `A tool for learning Chinese through subtitles with real-time annotations and Anki integration.`,
	}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start subtitle server",
		Long:  `Starts an HTTP server on port 8080 to serve pre-processed subtitles to the Chrome extension.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startServer(archivePath)
		},
	}

	processCmd := &cobra.Command{
		Use:   "process",
		Short: "Process video and generate annotated subtitles",
		Long:  `Downloads subtitles, extracts frames, performs OCR, and generates annotations for a video.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return processVideo(archivePath, videoPath, traditional)
		},
	}

	ankiCmd := &cobra.Command{
		Use:   "anki",
		Short: "Parent command for anki functions",
		Long:  ``,
	}

	ankiDeckCmd := &cobra.Command{
		Use:   "list",
		Short: "List words in the Anki deck",
		Long:  `List all words stored in the Anki deck.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := anki.NewClient(testutil.NewTestLogger())
			words, err := c.ListWords(defaultAnkiDeck)
			if err != nil {
				return err
			}
			for _, word := range words {
				fmt.Println(word)
			}
			return nil
		},
	}

	processCmd.Flags().StringVarP(&archivePath, "archive", "a", "data", "Path to archive directory")
	processCmd.Flags().StringVarP(&videoPath, "video", "v", "", "Path to video file (required)")
	processCmd.Flags().BoolVarP(&traditional, "traditional", "t", false, "Use traditional Chinese characters")
	processCmd.MarkFlagRequired("video")

	serverCmd.Flags().StringVarP(&archivePath, "archive", "a", "data", "Path to archive directory")

	ankiCmd.AddCommand(ankiDeckCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(processCmd)
	rootCmd.AddCommand(ankiCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type Server struct {
	anki        *anki.Client
	logger      *slog.Logger
	archivePath string
}

// errorLoggingMiddleware logs all 5xx errors
func (s *Server) errorLoggingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			// Check if response status is 5xx
			if c.Response().Status >= 500 && c.Response().Status < 600 {
				s.logger.Error("Internal server error",
					"method", c.Request().Method,
					"path", c.Request().URL.Path,
					"status", c.Response().Status,
					"error", err,
				)
			}

			return err
		}
	}
}

func startServer(archivePath string) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ankiClient := anki.NewClient(logger)

	srv := &Server{
		anki:        ankiClient,
		logger:      logger,
		archivePath: archivePath,
	}

	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(srv.errorLoggingMiddleware())

	e.GET("/subtitles", srv.handleGetSubtitles)
	e.POST("/save", srv.handleSaveWord)

	e.Logger.Fatal(e.Start(":8080"))

	return nil
}

// Handle GET request for subtitles by video ID
func (s *Server) handleGetSubtitles(c echo.Context) error {
	videoID := c.QueryParam("id")
	if videoID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Missing 'id' parameter",
		})
	}

	subs, ok, err := internal.LoadSubtitles(s.archivePath, videoID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Errorf("Failed to read subtitles: %v", err).Error(),
		})
	}
	if !ok {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"available": false,
		})
	}

	s.logger.Info("Served subtitles", "id", videoID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"available": true,
		"subtitles": subs,
	})
}

func (s *Server) handleSaveWord(c echo.Context) error {
	var wordData struct {
		Text                string `json:"text"`
		Pinyin              string `json:"pinyin"`
		Meaning             string `json:"meaning"`
		Pos                 string `json:"pos"`
		Sentence            string `json:"sentence"`
		SentencePinyin      string `json:"sentencePinyin"`
		SentenceTranslation string `json:"sentenceTranslation"`
	}

	if err := c.Bind(&wordData); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Bad request",
		})
	}

	s.logger.Info("Saving word to Anki",
		"word", wordData.Text,
		"sentence", wordData.Sentence)

	word := anki.Word{
		Text:                wordData.Text,
		Pinyin:              wordData.Pinyin,
		Meaning:             wordData.Meaning,
		Pos:                 wordData.Pos,
		Sentence:            wordData.Sentence,
		SentencePinyin:      wordData.SentencePinyin,
		SentenceTranslation: wordData.SentenceTranslation,
	}

	if err := s.anki.SaveWord(defaultAnkiDeck, word); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func processVideo(archivePath, videoPath string, traditional bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doneCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		defer close(doneCh)
		if err := internal.Process(ctx, archivePath, videoPath, traditional); err != nil {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("Received interrupt signal, shutting down gracefully...")
		cancel()
		<-doneCh
		return fmt.Errorf("interrupted by user")
	case err := <-errCh:
		cancel()
		<-doneCh
		return fmt.Errorf("processing failed: %w", err)
	case <-doneCh:
		return nil
	}
}
