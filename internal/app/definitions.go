package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/evanjhopkins/RunBinder/internal/domain"
	"github.com/evanjhopkins/RunBinder/internal/taskconfig"
)

type Definitions struct{}

func (d *Definitions) Load(path string) (domain.Task, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return domain.Task{}, err
	}
	cfg, definition, hash, err := d.parse(abs)
	if err != nil {
		return domain.Task{}, err
	}
	workingDir := filepath.Dir(abs)
	if cfg.WorkingDir != "" {
		workingDir = cfg.WorkingDir
		if !filepath.IsAbs(workingDir) {
			workingDir = filepath.Join(filepath.Dir(abs), workingDir)
		}
		workingDir, err = filepath.Abs(workingDir)
		if err != nil {
			return domain.Task{}, err
		}
	}
	info, err := os.Stat(workingDir)
	if err != nil {
		return domain.Task{}, fmt.Errorf("working directory: %w", err)
	}
	if !info.IsDir() {
		return domain.Task{}, errors.New("working_dir is not a directory")
	}
	return domain.Task{
		Namespace: cfg.Namespace, Definition: definition, Hash: hash,
		WorkingDir: workingDir, SourcePath: abs,
	}, nil
}

func (d *Definitions) Hash(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	_, _, hash, err := d.parse(abs)
	return hash, err
}

func (d *Definitions) Namespace(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	cfg, err := taskconfig.Parse(data)
	if err != nil {
		return "", err
	}
	return cfg.Namespace, nil
}

func (d *Definitions) Write(path string, cfg taskconfig.Config) error {
	definition, _, err := cfg.Canonical()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(definition), 0o644)
}

func (d *Definitions) parse(path string) (taskconfig.Config, string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return taskconfig.Config{}, "", "", fmt.Errorf("read task definition: %w", err)
	}
	cfg, err := taskconfig.Parse(data)
	if err != nil {
		return taskconfig.Config{}, "", "", err
	}
	definition, hash, err := cfg.Canonical()
	if err != nil {
		return taskconfig.Config{}, "", "", err
	}
	return cfg, definition, hash, nil
}
