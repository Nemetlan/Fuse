package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Universal Regex: Matches any string ending in a numeric extension like .001, .part01, .chunk.002, etc.
var chunkPattern = regexp.MustCompile(`^(.*?)(?:\.part|\.chunkpart|\.rclone_chunk|\.chunk|\.)?(\d+)$`)

const bufferSize = 4 * 1024 * 1024

type chunkFile struct {
	path string
	num  int
	size int64
}

// Custom theme for minimal, ultra-dark UI
type blipTheme struct{}

func (b *blipTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0D, G: 0x0E, B: 0x12, A: 0xFF} // Charcoal background
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x1F, G: 0x22, B: 0x2E, A: 0xFF}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x2A, G: 0x2E, B: 0x3D, A: 0xFF}
	default:
		return theme.DefaultTheme().Color(name, theme.VariantDark)
	}
}

func (b *blipTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }
func (b *blipTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (b *blipTheme) Size(n fyne.ThemeSizeName) float32  { return theme.DefaultTheme().Size(n) }

type mergeApp struct {
	win          fyne.Window
	progress     binding.Float
	progressBar  *widget.ProgressBar
	deleteSource *widget.Check

	// Interactive Dropzone UI
	dropBoxRect *canvas.Rectangle
	dropIcon    *widget.Icon
	dropTitle   *canvas.Text
	dropSub     *canvas.Text
	statusLabel *widget.Label

	mu   sync.Mutex
	busy bool
}

func main() {
	a := app.NewWithID("io.fuse.filemerger")
	a.Settings().SetTheme(&blipTheme{})

	w := a.NewWindow("Fuse")
	w.Resize(fyne.NewSize(420, 460))
	w.SetFixedSize(true)

	ma := &mergeApp{
		win:      w,
		progress: binding.NewFloat(),
	}

	ma.buildUI()

	// Native Drag-and-Drop Event Hooks
	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		ma.animateDropzone(false)
		if len(uris) == 0 {
			return
		}
		path := uris[0].Path()
		if path == "" {
			ma.fail(fmt.Errorf("could not resolve file path"))
			return
		}
		ma.startMerge(path)
	})

	w.ShowAndRun()
}

func (ma *mergeApp) buildUI() {
	// --- Minimalist Header ---
	title := canvas.NewText("Fuse", color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Universal file reassembly & chunk merger", color.NRGBA{R: 0x6E, G: 0x76, B: 0x8E, A: 0xFF})
	subtitle.TextSize = 11
	subtitle.Alignment = fyne.TextAlignCenter

	header := container.NewVBox(
		container.NewPadded(title),
		subtitle,
	)

	// --- Animated Drop Zone (Smaller Footprint) ---
	ma.dropBoxRect = canvas.NewRectangle(color.NRGBA{R: 0x14, G: 0x17, B: 0x22, A: 0xFF})
	ma.dropBoxRect.StrokeColor = color.NRGBA{R: 0x2A, G: 0x2F, B: 0x42, A: 0xFF}
	ma.dropBoxRect.StrokeWidth = 1.5
	ma.dropBoxRect.CornerRadius = 12

	ma.dropIcon = widget.NewIcon(theme.UploadIcon())

	ma.dropTitle = canvas.NewText("Drag & drop any chunk file", color.NRGBA{R: 0xEA, G: 0xEE, B: 0xF6, A: 0xFF})
	ma.dropTitle.TextStyle = fyne.TextStyle{Bold: true}
	ma.dropTitle.TextSize = 13
	ma.dropTitle.Alignment = fyne.TextAlignCenter

	ma.dropSub = canvas.NewText("Supports .001, .part1, .chunk001, etc.", color.NRGBA{R: 0x6E, G: 0x76, B: 0x8E, A: 0xFF})
	ma.dropSub.TextSize = 10
	ma.dropSub.Alignment = fyne.TextAlignCenter

	browseBtn := widget.NewButton("Browse Files", func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, ma.win)
				return
			}
			if r == nil {
				return
			}
			path := r.URI().Path()
			_ = r.Close()
			ma.startMerge(path)
		}, ma.win)
		fd.Show()
	})
	browseBtn.Importance = widget.MediumImportance

	dropContentBox := container.NewVBox(
		container.NewCenter(ma.dropIcon),
		ma.dropTitle,
		ma.dropSub,
		container.NewPadded(container.NewCenter(browseBtn)),
	)

	// Stack inside a padded wrapper to restrict rectangle height
	dropCardInner := container.NewStack(
		ma.dropBoxRect,
		container.NewCenter(dropContentBox),
	)

	dropCardContainer := container.NewPadded(
		container.NewPadded(dropCardInner),
	)

	// --- Status & Options Footer ---
	ma.statusLabel = widget.NewLabel("Ready to fuse files")
	ma.statusLabel.Alignment = fyne.TextAlignCenter

	ma.progressBar = widget.NewProgressBarWithData(ma.progress)
	ma.progressBar.Hide()

	ma.deleteSource = widget.NewCheck("Delete chunk files after merging", nil)
	ma.deleteSource.SetChecked(true)

	footer := container.NewVBox(
		ma.progressBar,
		ma.statusLabel,
		widget.NewSeparator(),
		container.NewCenter(ma.deleteSource),
	)

	mainContent := container.NewBorder(
		header,
		footer,
		nil,
		nil,
		dropCardContainer,
	)

	ma.win.SetContent(container.NewPadded(mainContent))
}

// --- Animation & State Visual Helpers ---

func (ma *mergeApp) animateDropzone(active bool) {
	go func() {
		steps := 8
		for i := 1; i <= steps; i++ {
			time.Sleep(15 * time.Millisecond)
			fyne.Do(func() {
				if active {
					ma.dropBoxRect.StrokeColor = color.NRGBA{
						R: uint8(0x2A + (0x3B-0x2A)*i/steps),
						G: uint8(0x2F + (0x82-0x2F)*i/steps),
						B: uint8(0x42 + (0xF6-0x42)*i/steps),
						A: 0xFF,
					}
				} else {
					ma.dropBoxRect.StrokeColor = color.NRGBA{
						R: uint8(0x3B - (0x3B-0x2A)*i/steps),
						G: uint8(0x82 - (0x82-0x2F)*i/steps),
						B: uint8(0xF6 - (0xF6-0x42)*i/steps),
						A: 0xFF,
					}
				}
				ma.dropBoxRect.Refresh()
			})
		}
	}()
}

func (ma *mergeApp) pulseSuccess() {
	go func() {
		fyne.Do(func() {
			ma.dropBoxRect.StrokeColor = color.NRGBA{R: 0x10, G: 0xB9, B: 0x81, A: 0xFF}
			ma.dropBoxRect.Refresh()
		})
		time.Sleep(1200 * time.Millisecond)
		ma.animateDropzone(false)
	}()
}

func (ma *mergeApp) setProgress(v float64) {
	fyne.Do(func() {
		_ = ma.progress.Set(v)
	})
}

func (ma *mergeApp) setStatus(msg string) {
	fyne.Do(func() {
		ma.statusLabel.SetText(msg)
	})
}

func (ma *mergeApp) showProgress() {
	fyne.Do(func() {
		_ = ma.progress.Set(0)
		ma.progressBar.Show()
		ma.animateDropzone(true)
	})
}

func (ma *mergeApp) hideProgress() {
	fyne.Do(func() {
		ma.progressBar.Hide()
	})
}

func (ma *mergeApp) fail(err error) {
	ma.hideProgress()
	ma.animateDropzone(false)
	ma.setStatus("Process failed")
	fyne.Do(func() {
		dialog.ShowError(err, ma.win)
	})
}

// --- Helper for truncating long filenames ---

func truncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Keep front and back portion with '...' in between
	partLen := (maxLen - 3) / 2
	return s[:partLen] + "..." + s[len(s)-partLen:]
}

// --- Merge Execution Logic ---

func (ma *mergeApp) startMerge(path string) {
	ma.mu.Lock()
	if ma.busy {
		ma.mu.Unlock()
		fyne.Do(func() {
			dialog.ShowInformation("In Progress", "Fuse is currently reassembling another file.", ma.win)
		})
		return
	}
	ma.busy = true
	ma.mu.Unlock()

	ma.showProgress()
	go ma.doMerge(path)
}

func (ma *mergeApp) doMerge(path string) {
	defer func() {
		ma.mu.Lock()
		ma.busy = false
		ma.mu.Unlock()
	}()

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		ma.fail(fmt.Errorf("invalid selection: please choose a single chunk file"))
		return
	}

	dir := filepath.Dir(path)
	name := filepath.Base(path)

	m := chunkPattern.FindStringSubmatch(name)
	if m == nil {
		ma.fail(fmt.Errorf("%q does not end with a recognized numeric chunk sequence", name))
		return
	}

	baseName := strings.TrimRight(m[1], ".")
	if baseName == "" {
		baseName = "merged_output"
	}

	// Truncate the name to max 28 characters to prevent layout breaking
	displayName := truncateMiddle(baseName, 28)

	fyne.Do(func() {
		ma.dropTitle.Text = displayName
		ma.dropTitle.Refresh()
	})

	entries, err := os.ReadDir(dir)
	if err != nil {
		ma.fail(fmt.Errorf("unable to read directory: %w", err))
		return
	}

	var chunks []chunkFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		mm := chunkPattern.FindStringSubmatch(e.Name())
		if mm == nil {
			continue
		}

		currentBase := strings.TrimRight(mm[1], ".")
		if currentBase != baseName {
			continue
		}

		num, err := strconv.Atoi(mm[2])
		if err != nil {
			continue
		}

		fi, err := e.Info()
		if err != nil {
			continue
		}

		chunks = append(chunks, chunkFile{
			path: filepath.Join(dir, e.Name()),
			num:  num,
			size: fi.Size(),
		})
	}

	if len(chunks) == 0 {
		ma.fail(fmt.Errorf("no matching chunk sequences found for target file"))
		return
	}

	sort.Slice(chunks, func(i, j int) bool { return chunks[i].num < chunks[j].num })

	var totalSize int64
	for _, c := range chunks {
		totalSize += c.size
	}

	ma.setStatus(fmt.Sprintf("Fusing %d chunks (%s)...", len(chunks), formatBytes(totalSize)))

	outPath := filepath.Join(dir, baseName)
	tmpPath := outPath + ".merging.tmp"

	if mergeErr := ma.streamMerge(chunks, tmpPath, totalSize); mergeErr != nil {
		_ = os.Remove(tmpPath)
		ma.fail(mergeErr)
		return
	}

	if renameErr := os.Rename(tmpPath, outPath); renameErr != nil {
		ma.fail(fmt.Errorf("failed to finalize output file: %w", renameErr))
		return
	}

	ma.setProgress(1)

	if ma.deleteSource.Checked {
		ma.setStatus("Cleaning up source chunk files...")
		for _, c := range chunks {
			_ = os.Remove(c.path)
		}
	}

	ma.hideProgress()
	ma.pulseSuccess()
	ma.setStatus("Files fused successfully!")

	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "Fuse Complete",
		Content: fmt.Sprintf("Successfully restored %s", displayName),
	})
}

func (ma *mergeApp) streamMerge(chunks []chunkFile, tmpPath string, totalSize int64) error {
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	writer := bufio.NewWriterSize(outFile, bufferSize)
	buf := make([]byte, bufferSize)

	var written int64
	for _, c := range chunks {
		in, openErr := os.Open(c.path)
		if openErr != nil {
			outFile.Close()
			return openErr
		}

		n, copyErr := io.CopyBuffer(writer, in, buf)
		in.Close()

		if copyErr != nil {
			outFile.Close()
			return copyErr
		}

		written += n
		if totalSize > 0 {
			ma.setProgress(float64(written) / float64(totalSize))
		}
	}

	if err := writer.Flush(); err != nil {
		outFile.Close()
		return err
	}

	return outFile.Close()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}