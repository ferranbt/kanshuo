package python

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

type PythonService struct {
	cmd *exec.Cmd
}

func (p *PythonService) Close() error {
	if err := p.cmd.Process.Kill(); err != nil {
		return err
	}
	p.cmd.Wait()
	return nil
}

func New(logger *slog.Logger, name string, script string, args ...string) (*PythonService, error) {
	pythonPath := filepath.Join(os.TempDir(), name+"-kanshuo-temp-python.py")
	if err := os.WriteFile(pythonPath, []byte(script), 0755); err != nil {
		return nil, err
	}

	writer := &slogWriter{logger: logger, level: slog.LevelDebug}

	cmdArgs := []string{
		pythonPath,
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("python3", cmdArgs...)
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	svc := &PythonService{
		cmd: cmd,
	}
	return svc, nil
}

type slogWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Log(context.Background(), w.level, string(p))
	return len(p), nil
}
