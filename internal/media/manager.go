package media

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
	"github.com/TiaraBasori/PaperValet/internal/interfaces"
	"github.com/TiaraBasori/PaperValet/pkg/logger"
)

// Manager handles media upload/download operations.
type Manager struct {
	api         *tg.Client
	peer        interfaces.PeerResolver
	downloadDir string
	logger      interfaces.Logger
}

// NewManager creates a media manager.
func NewManager(api *tg.Client, resolver interfaces.PeerResolver, downloadDir string) *Manager {
	if downloadDir == "" {
		downloadDir = "downloads"
	}
	os.MkdirAll(downloadDir, 0o755)
	return &Manager{
		api:         api,
		peer:        resolver,
		downloadDir: downloadDir,
		logger:      logger.NamedLogger("media"),
	}
}

// DownloadMedia downloads media from a message to local file.
func (m *Manager) DownloadMedia(ctx context.Context, msg *interfaces.MessageEvent) (string, error) {
	if msg.Media == nil {
		return "", fmt.Errorf("no media in message")
	}

	// Generate filename
	ext := m.guessExtension(msg.Media)
	filename := fmt.Sprintf("%d_%d%s", msg.ChatID, msg.Message.ID, ext)
	path := filepath.Join(m.downloadDir, filename)

	// Get input peer
	p, err := m.peer.ResolveFromChatID(ctx, msg.ChatID)
	if err != nil {
		return "", err
	}

	// Download using gotd message API
	sender := message.NewSender(m.api)
	media := msg.Media

	var f *os.File
	f, err = os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Use the high-level API
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		photo := m.Photo
		if p, ok := photo.(*tg.Photo); ok {
			err = sender.DownloadMedia(ctx, p, p, f)
		}
	case *tg.MessageMediaDocument:
		doc := m.Document
		if d, ok := doc.(*tg.Document); ok {
			err = sender.DownloadMedia(ctx, p, d, f)
		}
	default:
		err = fmt.Errorf("unsupported media type: %T", media)
	}

	if err != nil {
		os.Remove(path)
		return "", err
	}

	m.logger.Info("downloaded media", "path", path, "msg_id", msg.Message.ID)
	return path, nil
}

// UploadFile uploads a local file and returns InputMedia for sending.
func (m *Manager) UploadFile(ctx context.Context, chatID int64, path string) (tg.InputMediaClass, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p, err := m.peer.ResolveFromChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	sender := message.NewSender(m.api)

	// Get file info
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Upload as document
	uploaded, err := sender.UploadFile(ctx, p, f, message.UploadFileOptions{
		FileName: filepath.Base(path),
		MIMEType: mime.TypeByExtension(filepath.Ext(path)),
		Progress: nil,
	})
	if err != nil {
		return nil, err
	}

	m.logger.Info("uploaded file", "path", path, "size", info.Size())
	return uploaded, nil
}

// SendFile sends a local file to a chat.
func (m *Manager) SendFile(ctx context.Context, chatID int64, path string, caption string, replyTo int) error {
	media, err := m.UploadFile(ctx, chatID, path)
	if err != nil {
		return err
	}

	p, err := m.peer.ResolveFromChatID(ctx, chatID)
	if err != nil {
		return err
	}

	sender := message.NewSender(m.api)
	_, err = sender.Media(ctx, p, media).Caption(caption).Send()
	if err != nil {
		return err
	}

	return nil
}

// SendPhoto sends a photo from local path.
func (m *Manager) SendPhoto(ctx context.Context, chatID int64, path string, caption string) error {
	return m.SendFile(ctx, chatID, path, caption, 0)
}

// SendDocument sends a document from local path.
func (m *Manager) SendDocument(ctx context.Context, chatID int64, path string, caption string) error {
	return m.SendFile(ctx, chatID, path, caption, 0)
}

// guessExtension guesses file extension from media.
func (m *Manager) guessExtension(media tg.MessageMediaClass) string {
	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		return ".jpg"
	case *tg.MessageMediaDocument:
		if m.Document != nil {
			if doc, ok := m.Document.(*tg.Document); ok {
				for _, attr := range doc.Attributes {
					if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
						return filepath.Ext(fn.FileName)
					}
				}
				if doc.MimeType != "" {
					exts, _ := mime.ExtensionsByType(doc.MimeType)
					if len(exts) > 0 {
						return exts[0]
					}
				}
			}
		}
		return ".bin"
	default:
		return ".bin"
	}
}

// MediaInfo holds extracted media metadata.
type MediaInfo struct {
	Type       string // photo, document, video, audio, voice, sticker, etc.
	MIMEType   string
	FileName   string
	FileSize   int64
	Width      int
	Height     int
	Duration   int
	IsAnimated bool
}

// ExtractInfo extracts metadata from message media.
func ExtractInfo(media tg.MessageMediaClass) *MediaInfo {
	info := &MediaInfo{Type: "unknown"}

	switch m := media.(type) {
	case *tg.MessageMediaPhoto:
		info.Type = "photo"
		if p, ok := m.Photo.(*tg.Photo); ok {
			info.FileSize = int64(p.Sizes[len(p.Sizes)-1].GetSize())
			info.Width = p.Sizes[len(p.Sizes)-1].GetWidth()
			info.Height = p.Sizes[len(p.Sizes)-1].GetHeight()
		}
	case *tg.MessageMediaDocument:
		info.Type = "document"
		if d, ok := m.Document.(*tg.Document); ok {
			info.MIMEType = d.MimeType
			info.FileSize = d.Size
			for _, attr := range d.Attributes {
				switch a := attr.(type) {
				case *tg.DocumentAttributeFilename:
					info.FileName = a.FileName
				case *tg.DocumentAttributeVideo:
					info.Type = "video"
					info.Width = a.W
					info.Height = a.H
					info.Duration = a.Duration
				case *tg.DocumentAttributeAudio:
					info.Type = "audio"
					info.Duration = a.Duration
					if a.Voice {
						info.Type = "voice"
					}
				case *tg.DocumentAttributeSticker:
					info.Type = "sticker"
					info.IsAnimated = false
				case *tg.DocumentAttributeAnimated:
					info.IsAnimated = true
				}
			}
		}
	case *tg.MessageMediaWebPage:
		info.Type = "webpage"
	}

	return info
}

// FormatSize formats bytes to human readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CleanupOldDownloads removes files older than maxAge.
func (m *Manager) CleanupOldDownloads(maxAge time.Duration) error {
	entries, err := os.ReadDir(m.downloadDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(m.downloadDir, entry.Name())
			os.Remove(path)
			m.logger.Debug("cleaned old download", "file", entry.Name())
		}
	}
	return nil
}

// GetDownloadDir returns the download directory.
func (m *Manager) GetDownloadDir() string {
	return m.downloadDir
}

// SetDownloadDir changes the download directory.
func (m *Manager) SetDownloadDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	m.downloadDir = dir
	return nil
}

// --- Helper for command context ---

// DownloadAndReply downloads media from message and sends back as file.
func (m *Manager) DownloadAndReply(ctx *interfaces.CommandContext) error {
	if ctx.Message.Media == nil {
		return ctx.Reply("No media in message")
	}

	path, err := m.DownloadMedia(ctx.Context(), ctx.Message)
	if err != nil {
		return ctx.Reply(fmt.Sprintf("Download failed: %v", err))
	}

	info := ExtractInfo(ctx.Message.Media)
	return ctx.Reply(fmt.Sprintf("Downloaded: %s (%s, %s)",
		filepath.Base(path), info.Type, FormatSize(info.FileSize)))
}