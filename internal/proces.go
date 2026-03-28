package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "embed"

	gt "github.com/bas24/googletranslatefree"
	"github.com/ferranbt/kanshuo/internal/ocr"
	"github.com/ferranbt/kanshuo/internal/testutil"
	"github.com/jcramb/cedict"
	"github.com/liuzl/gocc"
	"github.com/lrstanley/go-ytdlp"
	"github.com/yanyiwu/gojieba"
)

type VideoInfo struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Extractor string  `json:"extractor"`
	Duration  float64 `json:"duration"` // in seconds
}

const (
	extractorYoutube  = "youtube"
	extractorBilibili = "bilibili"
)

func Process(ctx context.Context, archivePath string, targetVideo string, traditional bool) error {
	fmt.Printf("Processing video: %s, using traditional %v\n", targetVideo, traditional)

	dl1 := ytdlp.New().
		DumpJSON().
		SkipDownload().
		NoPlaylist().
		CookiesFromBrowser("chrome")

	result, err := dl1.Run(context.TODO(), targetVideo)
	if err != nil {
		panic(err)
	}

	var info VideoInfo
	for _, entry := range result.OutputLogs {
		if err := json.Unmarshal([]byte(entry.Line), &info); err == nil {
			break
		}
	}

	// create the folder for the video if not exists already
	archiveTargetPath := filepath.Join(archivePath, info.ID)
	if err := os.MkdirAll(archiveTargetPath, 0755); err != nil {
		log.Fatal(err)
	}

	var (
		videoPath          = filepath.Join(archiveTargetPath, "video.mp4")
		streamPath         = filepath.Join(archiveTargetPath, "temp_frames")
		outputSubsPath     = filepath.Join(archiveTargetPath, "subs.json")
		annotationSubsPath = filepath.Join(archiveTargetPath, "subs_ann.json")
	)

	fmt.Println("-- video info --")
	fmt.Println(info.ID)

	if !fileExists(videoPath) {
		dl := ytdlp.New().
			FormatSort("res,ext:mp4:m4a").
			Format("bestvideo[height<=480][ext=mp4]/bestvideo[height<=480]/best[height<=480]").
			CookiesFromBrowser("chrome").
			NoPlaylist().
			NoOverwrites().
			Continue().
			ProgressFunc(100*time.Millisecond, func(prog ytdlp.ProgressUpdate) {
				fmt.Printf(
					"%s @ %s [eta: %s] :: %s\n",
					prog.Status,
					prog.PercentString(),
					prog.ETA(),
					prog.Filename,
				)
			}).
			Output(videoPath)

		if _, err := dl.Run(ctx, targetVideo); err != nil {
			panic(err)
		}
	}

	err = streamAndExtract(ctx, videoPath, streamPath, int(info.Duration), func(i, total int) {
		fmt.Printf("EXTRACT FRAMES (%d|%d)\n", i, total)
	})
	if err != nil {
		log.Fatal(err)
	}

	err = extractSubtitlesFromFrames(ctx, streamPath, outputSubsPath, traditional, func(i, total int) {
		fmt.Printf("OCR (%d|%d)\n", i, total)
	})
	if err != nil {
		log.Fatal(err)
	}

	err = annotateSubtitle(ctx, outputSubsPath, annotationSubsPath, func(i, total int) {
		fmt.Printf("ANNOTATE (%d|%d)\n", i, total)
	})
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

type frameEntry struct {
	Path            string         `json:"path"`
	Result          *ocr.OCRRegion `json:"result"`
	TimeToProcessMs int64          `json:"time"`
	Position        float64        `json:"position"`
}

func (f *frameEntry) Equal(ff *frameEntry) bool {
	return f.Path == ff.Path && f.Position == ff.Position
}

func LoadSubtitles(artifactsPath string, id string) ([]*Subtitle, bool, error) {
	subsPath := filepath.Join(artifactsPath, id, "subs_ann.json")
	_, err := os.Stat(subsPath)
	if err != nil {
		if err == os.ErrNotExist {
			return nil, false, nil
		} else {
			return nil, false, err
		}
	}

	data, err := os.ReadFile(subsPath)
	if err != nil {
		return nil, false, err
	}

	var subtitles []*Subtitle
	if err := json.Unmarshal(data, &subtitles); err != nil {
		return nil, false, err
	}

	return subtitles, true, nil
}

type Subtitle struct {
	Text string `json:"text"`

	StartFrame    string  `json:"-"`
	StartPosition float64 `json:"start_position"`

	EndFrame    string  `json:"-"`
	EndPosition float64 `json:"end_position"`

	Confidence float64 `json:"confidence"`

	Simplified string `json:"simplified"`
	Pinyin     string `json:"pinyin"`

	Annotation *Annotation `json:"annotation,omitempty"`
}

type Annotation struct {
	Translation string

	WAnalysis []*WordAnalysis
}

func (a *Annotation) Text() string {
	m := []string{}

	for _, w := range a.WAnalysis {
		m = append(m, w.Simplified)
	}
	return strings.Join(m, "")
}

type WordAnalysis struct {
	Simplified string
	Pinyin     string
	Tag        string
	Meaning    []string
}

func streamAndExtract(ctx context.Context, videoPath string, output string, duration int, onProgress func(frame, total int)) error {
	totalFrames := int(duration)
	if dirExists(output) {
		return nil
	}

	tmpOutput := output + "_aux"
	if err := os.MkdirAll(tmpOutput, 0755); err != nil {
		return fmt.Errorf("create frames dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-progress", "pipe:1",
		"-an",
		"-vf", "fps=1",
		"-vsync", "0",
		tmpOutput+"/%08d.jpg",
	)

	// capture stderr so we can return ffmpeg errors
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line, ok := strings.CutPrefix(line, "frame="); ok {
			if frameNum, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
				onProgress(frameNum, totalFrames)
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg error: %w\n%s", err, stderrBuf.String())
	}

	if err := moveDir(tmpOutput, output); err != nil {
		return err
	}
	return nil
}

// containsChinese checks if a string contains at least one Chinese character
func containsChinese(text string) bool {
	for _, r := range text {
		// Chinese Unicode ranges:
		// 4E00-9FFF: CJK Unified Ideographs (common Chinese characters)
		// 3400-4DBF: CJK Unified Ideographs Extension A
		// F900-FAFF: CJK Compatibility Ideographs
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0xF900 && r <= 0xFAFF) {
			return true
		}
	}
	return false
}

func filter[T any](slice []T, predicate func(T) bool) []T {
	result := make([]T, 0)
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

func frameNameToSeconds(name string) (float64, error) {
	// Strip extension and path, keep only the number
	base := filepath.Base(name)
	numStr := strings.TrimSuffix(base, filepath.Ext(base))

	frameNum, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid frame name %q: %w", name, err)
	}

	// fps=1 means 1 frame per second, frames are 1-indexed
	return float64(frameNum - 1), nil
}

func mergeFramesToSubtitles(frames []*frameEntry) []*Subtitle {
	var subtitles []*Subtitle

	if len(frames) == 0 {
		return subtitles
	}

	current := Subtitle{
		Text:          frames[0].Result.Text,
		StartPosition: frames[0].Position,
		EndPosition:   frames[0].Position,
	}

	for _, frame := range frames[1:] {
		if frame.Result.Text == current.Text {
			// Same text, extend the current subtitle
			current.EndPosition = frame.Position
			current.EndFrame = frame.Path
		} else {
			// Text changed, save current and start a new one
			saved := current                      // copy
			subtitles = append(subtitles, &saved) // pointer to the copy

			current = Subtitle{
				Text:          frame.Result.Text,
				StartPosition: frame.Position,
				StartFrame:    frame.Path,
				EndPosition:   frame.Position,
				EndFrame:      frame.Path,
				Confidence:    frame.Result.Confidence,
			}
		}
	}

	subtitles = append(subtitles, &current)
	return subtitles
}

const minOCRConfidence = 0.1

func performOCR(ctx context.Context, logger *slog.Logger, framesDir string, traditional bool, progress func(current, total int)) ([]*frameEntry, error) {
	easyOCR, err := ocr.NewEasyOCR(logger, traditional)
	if err != nil {
		return nil, err
	}
	defer easyOCR.Stop()

	entries, err := os.ReadDir(framesDir)
	if err != nil {
		log.Fatal(err)
	}

	totalEntries := len(entries)
	var current atomic.Int32

	var frameEntries []*frameEntry
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".jpg") {
			continue
		}

		framePosition, err := frameNameToSeconds(e.Name())
		if err != nil {
			return nil, err
		}

		frameEntries = append(frameEntries, &frameEntry{
			Path:     filepath.Join(framesDir, e.Name()),
			Position: framePosition,
		})
	}

	ocrTempFile := filepath.Join(framesDir, "_ocr_frames.json")
	if _, err := os.Stat(ocrTempFile); err == nil {
		// read the saved file, validate and load any result that we have stored there
		data, err := os.ReadFile(ocrTempFile)
		if err != nil {
			return nil, err
		}
		var newEntries []*frameEntry
		if err := json.Unmarshal(data, &newEntries); err != nil {
			return nil, err
		}
		if len(newEntries) != len(frameEntries) {
			return nil, fmt.Errorf("incorrect length %d %d", len(newEntries), len(frameEntries))
		}
		for i, entry := range frameEntries {
			newEntry := newEntries[i]
			if !entry.Equal(newEntry) {
				return nil, fmt.Errorf("one entry not correct")
			}
			if newEntry.Result != nil {
				entry.Result = newEntry.Result
				entry.TimeToProcessMs = newEntry.TimeToProcessMs
			}
		}
	}

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	fmt.Println(len(entries))
	for _, entry := range frameEntries {
		if entry.Result != nil {
			continue
		}

		// stop spawning new goroutines if context is cancelled
		select {
		case <-ctx.Done():
			goto wait
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(entry *frameEntry) {
			defer wg.Done()
			defer func() {
				newVal := current.Add(1)
				if newVal%10 == 0 || int(newVal) == totalEntries {
					progress(int(newVal), totalEntries)
				}
				<-sem
			}()

			cropped, err := testutil.CropBottomQuarter(entry.Path)
			if err != nil {
				panic(err)
			}
			defer os.Remove(cropped)

			result, err := easyOCR.OCR(cropped)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Error("failed to process ocr", "err", err)
			}

			// only process them if the ocr regions contain chinese
			regions := result.Regions
			regions = filter(regions, func(o ocr.OCRRegion) bool {
				return containsChinese(o.Text)
			})

			if len(regions) == 0 {
				// put a placeholder value such that if we restart we do not compute it again
				// even if it is here because it has confidence 0 it will not be returned
				entry.Result = &ocr.OCRRegion{}
				return
			}

			best := regions[0]
			for _, r := range regions[1:] {
				if r.Confidence > best.Confidence {
					best = r
				}
			}

			entry.Result = &best
			entry.TimeToProcessMs = result.TimeToProcessMs
		}(entry)
	}
wait:
	wg.Wait()

	logger.Info("Saving ocr temporal file", "path", ocrTempFile)
	tempFileData, err := json.Marshal(frameEntries)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(ocrTempFile, tempFileData, 0755); err != nil {
		return nil, err
	}

	// we stopped mid way, we wait to write the temporal file into memory
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// filter and keep only frames that have confidence higher than 0.1
	framesWithSubs := filter(frameEntries, func(i *frameEntry) bool {
		return i.Result.Confidence > minOCRConfidence
	})

	return framesWithSubs, nil
}

func extractSubtitlesFromFrames(ctx context.Context, framesDir string, outputSubs string, traditional bool, progress func(i int, total int)) error {
	if fileExists(outputSubs) {
		return nil
	}

	logger := testutil.NewTestLogger()
	framesWithSubs, err := performOCR(ctx, logger, framesDir, traditional, progress)
	if err != nil {
		return err
	}
	subs := mergeFramesToSubtitles(framesWithSubs)

	s2t, err := gocc.New("t2s")
	if err != nil {
		return err
	}

	d := cedict.New()

	for _, sub := range subs {
		out, err := s2t.Convert(sub.Text)
		if err != nil {
			return err
		}

		sub.Pinyin = convertToPinyinTones(d.HanziToPinyin(out))
		sub.Simplified = out
	}

	subsRaw, err := json.Marshal(subs)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputSubs, subsRaw, 0722); err != nil {
		return err
	}

	return nil
}

func annotateSubtitle(ctx context.Context, subsPath string, annotationPath string, progress func(i, total int)) error {
	fmt.Println("Annotating subtitles")

	if !fileExists(annotationPath) {
		if err := copyFile(subsPath, annotationPath); err != nil {
			return err
		}
	}

	data, err := os.ReadFile(annotationPath)
	if err != nil {
		return err
	}

	var subs []*Subtitle
	if err := json.Unmarshal(data, &subs); err != nil {
		return err
	}

	allComputed := true
	for _, s := range subs {
		if s.Annotation == nil {
			allComputed = false
			break
		}
	}
	if allComputed {
		// early exit if there is nothing else to compute
		return nil
	}

	d := cedict.New()

	j := gojieba.NewJieba()
	defer j.Free()

	var computeErr error

LOOP:
	for indx, s := range subs {
		select {
		case <-ctx.Done():
			break LOOP
		default:
		}

		if s.Annotation != nil {
			continue
		}

		ann, err := annotateText(d, j, s)
		if err != nil {
			// we break instead of directly returning because we still
			// want to save a snapshot of the processing
			computeErr = err
			break
		}
		s.Annotation = ann

		progress(indx, len(subs))
	}

	dataRaw, err := json.Marshal(subs)
	if err != nil {
		return err
	}
	if err := os.WriteFile(annotationPath, dataRaw, 0755); err != nil {
		return err
	}

	return computeErr
}

func annotateText(d *cedict.Dict, j *gojieba.Jieba, sub *Subtitle) (*Annotation, error) {
	result, err := gt.Translate(sub.Simplified, "zh-CN", "en")
	if err != nil {
		return nil, err
	}

	ann := &Annotation{
		Translation: result,
		WAnalysis:   []*WordAnalysis{},
	}

	words := j.Tag(sub.Simplified)
	for _, w := range words {
		if strings.TrimSpace(w) == "" {
			continue
		}

		tag := parseTag(w)

		pinyin := d.HanziToPinyin(tag.Word)
		pinyin = convertToPinyinTones(pinyin)

		meanings := extractMeanings(d, tag)

		ann.WAnalysis = append(ann.WAnalysis, &WordAnalysis{
			Simplified: tag.Word,
			Tag:        tag.Tag,
			Pinyin:     pinyin,
			Meaning:    meanings,
		})
	}

	return ann, nil
}

func convertToPinyinTones(pinyin string) (result string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Failed to convert pinyin: %s", pinyin)
			// cedict.PinyinTones panicked (likely unexpected input), return original pinyin
			result = pinyin
		}
	}()

	pinyin = strings.ReplaceAll(pinyin, "u:", "u")
	pinyin = strings.ReplaceAll(pinyin, "4zu", "zu")
	result = cedict.PinyinTones(pinyin)
	return result
}

type WordTag struct {
	Word string
	Tag  string
}

func parseTag(s string) WordTag {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return WordTag{Word: s}
	}
	return WordTag{Word: parts[0], Tag: parts[1]}
}

func extractMeanings(d *cedict.Dict, word WordTag) []string {
	entries := extractMeaningsExact(d, word)
	if len(entries) > 0 {
		return entries
	}

	// fallback: if word not found, look up each character individually
	// handles compositional phrases like 太多, 脸上 that jieba doesn't split
	runes := []rune(word.Word)
	if len(runes) <= 1 {
		return nil
	}

	for _, r := range runes {
		charEntries := extractMeaningsExact(d, WordTag{Word: string(r), Tag: ""})
		entries = append(entries, charEntries...)
	}

	return entries
}

func extractMeaningsExact(d *cedict.Dict, word WordTag) []string {
	entries := []string{}

	// CC-CEDICT can have multiple entries for the same hanzi (e.g. 也 as "surname Ye" and as "also").
	// The library has no direct "get all entries by hanzi" method, so we work around it:
	// convert the hanzi to pinyin first, fetch all entries for that pinyin (which may include
	// other hanzi with the same pronunciation), then filter down to only the ones matching
	// our specific hanzi.
	for _, m := range d.GetByPinyin(d.HanziToPinyin(word.Word)) {
		if m.Simplified != word.Word {
			continue
		}

		// CC-CEDICT convention: uppercase pinyin (e.g. "Ye3") indicates a proper noun or surname.
		isProperNoun := len(m.Pinyin) > 0 && m.Pinyin[0] >= 'A' && m.Pinyin[0] <= 'Z'

		if isProperNoun {
			if word.Tag == "nr" {
				// Jieba also identified this as a proper noun — high confidence match, return immediately.
				return m.Meanings
			}
			// Jieba disagrees — this is likely a homograph where the surname reading
			// is not the intended one (e.g. 也 as "Ye" vs "also"). Skip it.
			continue
		}

		entries = append(entries, m.Meanings...)

	}

	return entries
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func moveDir(src, dst string) error {
	return os.Rename(src, dst)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
