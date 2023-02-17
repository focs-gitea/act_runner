package artifactcache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	tempExt = ".tmp"
)

type Storage struct {
	rootDir string
}

func NewStorage(rootDir string) (*Storage, error) {
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, err
	}
	return &Storage{
		rootDir: rootDir,
	}, nil
}

func (s *Storage) Exist(id int64) (bool, error) {
	name := s.filename(id)
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) Write(id int64, offset int64, reader io.Reader) error {
	temp := s.filename(id) + tempExt
	if err := os.MkdirAll(filepath.Dir(temp), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(temp, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		return err
	}
	return nil
}

func (s *Storage) Commit(id int64) error {
	name := s.filename(id)
	temp := name + tempExt

	return os.Rename(temp, name)
}

func (s *Storage) Serve(w http.ResponseWriter, r *http.Request, id int64) {
	name := s.filename(id)
	http.ServeFile(w, r, name)
}

func (s *Storage) Remove(id int64) {
	name := s.filename(id)
	temp := name + tempExt
	_ = os.Remove(name)
	_ = os.Remove(temp)
}

func (s *Storage) filename(id int64) string {
	return filepath.Join(s.rootDir, fmt.Sprint(id))
}
