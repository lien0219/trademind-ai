package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Provider stores objects on the local filesystem under a single root directory.
type Provider struct {
	root       string
	publicBase string
}

// New resolves root to an absolute path and returns a local Provider.
func New(root, publicBase string) (*Provider, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("local storage: %w", err)
	}
	pub := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	return &Provider{root: abs, publicBase: pub}, nil
}

func (p *Provider) resolve(objectKey string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("local storage: nil provider")
	}
	rel := strings.Trim(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	if rel == "" {
		return "", fmt.Errorf("local storage: empty object key")
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("local storage: invalid object key")
	}
	full := filepath.Join(p.root, filepath.FromSlash(rel))
	absRoot, err := filepath.Abs(p.root)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	sep := string(os.PathSeparator)
	if !strings.HasPrefix(absFull, absRoot+sep) && absFull != absRoot {
		return "", fmt.Errorf("local storage: path outside root")
	}
	return absFull, nil
}

// Put writes reader to root/objectKey (creates parent directories).
func (p *Provider) Put(ctx context.Context, objectKey string, r io.Reader, size int64, _ string) error {
	if size < 0 {
		return fmt.Errorf("local storage: invalid size")
	}
	path, err := p.resolve(objectKey)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("local storage mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("local storage create: %w", err)
	}
	defer f.Close()
	var errw error
	if size > 0 {
		_, errw = io.Copy(f, io.LimitReader(r, size))
	} else {
		_, errw = io.Copy(f, r)
	}
	if errw != nil {
		_ = os.Remove(path)
		return fmt.Errorf("local storage write: %w", errw)
	}
	return nil
}

// GetURL joins public_base with the object key (must be HTTPS/HTTP URL prefix from settings).
func (p *Provider) GetURL(_ context.Context, objectKey string) (string, error) {
	key := strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	if p.publicBase == "" {
		return "", fmt.Errorf("local storage: public_base is not configured")
	}
	return p.publicBase + "/" + key, nil
}

// Get opens the object for reading; the caller must close the returned reader.
func (p *Provider) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	path, err := p.resolve(objectKey)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local storage: file not found")
		}
		return nil, fmt.Errorf("local storage: open: %w", err)
	}
	return f, nil
}

// Delete removes the object file.
func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	path, err := p.resolve(objectKey)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("local storage delete: %w", err)
	}
	return nil
}
