// Package storage - documentos adjuntos en disco (volumen Docker
// "attachments"). Implementa ports.FileStorage; para DigitalOcean Spaces se
// agrega otro adaptador con la misma interfaz, sin tocar la aplicación.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Local struct{ root string }

func NewLocal(root string) (*Local, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("carpeta de adjuntos %s: %w", root, err)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Local{root: abs}, nil
}

// path impide salir de la carpeta raíz (p. ej. "../../etc/passwd").
func (l *Local) path(key string) (string, error) {
	p := filepath.Join(l.root, filepath.FromSlash(key))
	if !strings.HasPrefix(p, l.root+string(os.PathSeparator)) {
		return "", errors.New("ruta de archivo inválida")
	}
	return p, nil
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader) (int64, error) {
	p, err := l.path(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return 0, err
	}
	tmp := p + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, os.Rename(tmp, p) // escritura atómica
}

func (l *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}
