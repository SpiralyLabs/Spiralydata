package main

import (
	"fmt"
	"image/color"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ============================================================================
// TOOLTIPS
// ============================================================================

// TooltipButton est un bouton avec un tooltip
type TooltipButton struct {
	widget.Button
	tooltip string
}

// NewTooltipButton crée un nouveau bouton avec tooltip
func NewTooltipButton(label string, tooltip string, tapped func()) *TooltipButton {
	btn := &TooltipButton{
		tooltip: tooltip,
	}
	btn.Text = label
	btn.OnTapped = tapped
	btn.ExtendBaseWidget(btn)
	return btn
}

// MouseIn affiche le tooltip quand la souris entre
func (b *TooltipButton) MouseIn(e *desktop.MouseEvent) {
	if b.tooltip != "" {
		showTooltip(b.tooltip, e.AbsolutePosition)
	}
}

// MouseOut cache le tooltip
func (b *TooltipButton) MouseOut() {
	hideTooltip()
}

// Tooltip popup global
var (
	tooltipPopup  *widget.PopUp
	tooltipMu     sync.Mutex
	tooltipCancel chan struct{}
)

func showTooltip(text string, pos fyne.Position) {
	tooltipMu.Lock()
	defer tooltipMu.Unlock()

	if tooltipCancel != nil {
		close(tooltipCancel)
	}
	tooltipCancel = make(chan struct{})

	go func(cancel chan struct{}) {
		select {
		case <-time.After(500 * time.Millisecond):
			if myWindow == nil {
				return
			}

			tooltipMu.Lock()
			defer tooltipMu.Unlock()

			bg := canvas.NewRectangle(color.RGBA{R: 50, G: 55, B: 65, A: 230})
			bg.CornerRadius = 4

			label := widget.NewLabel(text)
			label.TextStyle = fyne.TextStyle{Italic: true}

			content := container.NewStack(bg, container.NewPadded(label))

			tooltipPopup = widget.NewPopUp(content, myWindow.Canvas())
			tooltipPopup.Move(fyne.NewPos(pos.X+10, pos.Y+20))
			tooltipPopup.Show()
		case <-cancel:
			return
		}
	}(tooltipCancel)
}

func hideTooltip() {
	tooltipMu.Lock()
	defer tooltipMu.Unlock()

	if tooltipCancel != nil {
		close(tooltipCancel)
		tooltipCancel = nil
	}

	if tooltipPopup != nil {
		tooltipPopup.Hide()
		tooltipPopup = nil
	}
}

// ============================================================================
// STATUS BAR
// ============================================================================

// StatusBar représente une barre de statut en bas de la fenêtre
type StatusBar struct {
	container      *fyne.Container
	statusLabel    *widget.Label
	connectionIcon *canvas.Circle
	fileCountLabel *widget.Label
	transferLabel  *widget.Label
	themeButton    *widget.Button
	mu             sync.Mutex
}

// NewStatusBar crée une nouvelle barre de statut
func NewStatusBar() *StatusBar {
	sb := &StatusBar{}

	// Indicateur de connexion
	sb.connectionIcon = canvas.NewCircle(color.RGBA{R: 128, G: 128, B: 128, A: 255})
	sb.connectionIcon.StrokeWidth = 0
	sb.connectionIcon.Resize(fyne.NewSize(12, 12))

	// Labels
	sb.statusLabel = widget.NewLabel("Déconnecté")
	sb.statusLabel.TextStyle = fyne.TextStyle{Monospace: true}

	sb.fileCountLabel = widget.NewLabel("📁 0 fichiers")
	sb.fileCountLabel.TextStyle = fyne.TextStyle{Monospace: true}

	sb.transferLabel = widget.NewLabel("")
	sb.transferLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Bouton de thème
	sb.themeButton = widget.NewButton("🌙", func() {
		ToggleTheme()
		if GetCurrentTheme() == ThemeDark {
			sb.themeButton.SetText("🌙")
		} else {
			sb.themeButton.SetText("☀️")
		}
	})
	sb.themeButton.Importance = widget.LowImportance

	// Conteneur principal
	leftContent := container.NewHBox(
		container.NewCenter(sb.connectionIcon),
		sb.statusLabel,
		widget.NewSeparator(),
		sb.fileCountLabel,
	)

	rightContent := container.NewHBox(
		sb.transferLabel,
		widget.NewSeparator(),
		sb.themeButton,
	)

	sb.container = container.NewBorder(
		widget.NewSeparator(),
		nil, nil, nil,
		container.NewHBox(
			leftContent,
			layout.NewSpacer(),
			rightContent,
		),
	)

	return sb
}

// GetContainer retourne le conteneur de la barre de statut
func (sb *StatusBar) GetContainer() *fyne.Container {
	return sb.container
}

// SetConnected met à jour l'état de connexion
func (sb *StatusBar) SetConnected(connected bool, serverAddr string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if connected {
		sb.connectionIcon.FillColor = color.RGBA{R: 46, G: 204, B: 113, A: 255}
		sb.statusLabel.SetText(fmt.Sprintf("Connecté à %s", serverAddr))
	} else {
		sb.connectionIcon.FillColor = color.RGBA{R: 128, G: 128, B: 128, A: 255}
		sb.statusLabel.SetText("Déconnecté")
	}
	sb.connectionIcon.Refresh()
}

// SetFileCount met à jour le nombre de fichiers
func (sb *StatusBar) SetFileCount(count int) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.fileCountLabel.SetText(fmt.Sprintf("📁 %d fichiers", count))
}

// SetTransferStatus met à jour le statut de transfert
func (sb *StatusBar) SetTransferStatus(status string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.transferLabel.SetText(status)
}

// ============================================================================
// SEARCH BAR
// ============================================================================

// SearchBar est une barre de recherche avec filtre
type SearchBar struct {
	container   *fyne.Container
	entry       *widget.Entry
	clearBtn    *widget.Button
	onSearch    func(query string)
	placeholder string
}

// NewSearchBar crée une nouvelle barre de recherche
func NewSearchBar(placeholder string, onSearch func(query string)) *SearchBar {
	sb := &SearchBar{
		onSearch:    onSearch,
		placeholder: placeholder,
	}

	sb.entry = widget.NewEntry()
	sb.entry.SetPlaceHolder(placeholder)
	sb.entry.OnChanged = func(s string) {
		if sb.onSearch != nil {
			sb.onSearch(s)
		}
		if s != "" {
			sb.clearBtn.Show()
		} else {
			sb.clearBtn.Hide()
		}
	}

	sb.clearBtn = widget.NewButton("✕", func() {
		sb.entry.SetText("")
		if sb.onSearch != nil {
			sb.onSearch("")
		}
		sb.clearBtn.Hide()
	})
	sb.clearBtn.Importance = widget.LowImportance
	sb.clearBtn.Hide()

	searchIcon := widget.NewLabel("🔍")

	sb.container = container.NewBorder(
		nil, nil,
		searchIcon,
		sb.clearBtn,
		sb.entry,
	)

	return sb
}

// GetContainer retourne le conteneur
func (sb *SearchBar) GetContainer() *fyne.Container {
	return sb.container
}

// GetText retourne le texte actuel
func (sb *SearchBar) GetText() string {
	return sb.entry.Text
}

// Clear efface la recherche
func (sb *SearchBar) Clear() {
	sb.entry.SetText("")
}

// ============================================================================
// BREADCRUMB
// ============================================================================

// Breadcrumb représente un fil d'Ariane
type Breadcrumb struct {
	container *fyne.Container
	parts     []string
	onClick   func(index int)
}

// NewBreadcrumb crée un nouveau fil d'Ariane
func NewBreadcrumb(onClick func(index int)) *Breadcrumb {
	bc := &Breadcrumb{
		onClick: onClick,
		parts:   []string{},
	}
	bc.container = container.NewHBox()
	return bc
}

// SetPath définit le chemin du fil d'Ariane
func (bc *Breadcrumb) SetPath(parts []string) {
	bc.parts = parts
	bc.container.Objects = nil

	// Icône home
	homeBtn := widget.NewButton("🏠", func() {
		if bc.onClick != nil {
			bc.onClick(-1) // -1 pour la racine
		}
	})
	homeBtn.Importance = widget.LowImportance
	bc.container.Add(homeBtn)

	for i, part := range parts {
		// Séparateur
		sep := widget.NewLabel(" › ")
		bc.container.Add(sep)

		// Bouton pour la partie
		index := i
		partBtn := widget.NewButton(part, func() {
			if bc.onClick != nil {
				bc.onClick(index)
			}
		})
		partBtn.Importance = widget.LowImportance

		// Le dernier élément est en gras
		if i == len(parts)-1 {
			partBtn.Importance = widget.MediumImportance
		}

		bc.container.Add(partBtn)
	}

	bc.container.Refresh()
}

// GetContainer retourne le conteneur
func (bc *Breadcrumb) GetContainer() *fyne.Container {
	return bc.container
}

// ============================================================================
// STATISTICS CARD
// ============================================================================

// StatCard est une carte de statistique
type StatCard struct {
	container *fyne.Container
	titleLbl  *widget.Label
	valueLbl  *widget.Label
	iconLbl   *widget.Label
	bg        *canvas.Rectangle
}

// NewStatCard crée une nouvelle carte de statistique
func NewStatCard(title, icon string, value string) *StatCard {
	sc := &StatCard{}

	sc.iconLbl = widget.NewLabel(icon)
	sc.iconLbl.TextStyle = fyne.TextStyle{Bold: true}

	sc.titleLbl = widget.NewLabel(title)
	sc.titleLbl.TextStyle = fyne.TextStyle{Italic: true}

	sc.valueLbl = widget.NewLabel(value)
	sc.valueLbl.TextStyle = fyne.TextStyle{Bold: true}
	sc.valueLbl.Alignment = fyne.TextAlignCenter

	content := container.NewVBox(
		container.NewHBox(sc.iconLbl, sc.titleLbl),
		sc.valueLbl,
	)

	// Fond de la carte - adaptatif au thème
	sc.bg = canvas.NewRectangle(sc.getBackgroundColor())
	sc.bg.CornerRadius = 8

	sc.container = container.NewStack(sc.bg, container.NewPadded(content))

	return sc
}

// getBackgroundColor retourne la couleur de fond selon le thème
func (sc *StatCard) getBackgroundColor() color.Color {
	if GetCurrentTheme() == ThemeDark {
		return color.RGBA{R: 45, G: 50, B: 60, A: 255}
	}
	return color.RGBA{R: 220, G: 225, B: 235, A: 255}
}

// RefreshTheme met à jour les couleurs selon le thème actuel
func (sc *StatCard) RefreshTheme() {
	if sc.bg != nil {
		sc.bg.FillColor = sc.getBackgroundColor()
		sc.bg.Refresh()
	}
}

// SetValue met à jour la valeur
func (sc *StatCard) SetValue(value string) {
	sc.valueLbl.SetText(value)
}

// GetContainer retourne le conteneur
func (sc *StatCard) GetContainer() *fyne.Container {
	return sc.container
}

// ============================================================================
// PROGRESS INDICATOR
// ============================================================================

// ProgressIndicator est un indicateur de progression circulaire
type ProgressIndicator struct {
	container *fyne.Container
	label     *widget.Label
	bar       *widget.ProgressBar
	active    bool
	stopChan  chan struct{}
	mu        sync.Mutex
}

// NewProgressIndicator crée un nouvel indicateur
func NewProgressIndicator() *ProgressIndicator {
	pi := &ProgressIndicator{}

	pi.label = widget.NewLabel("")
	pi.bar = widget.NewProgressBar()
	pi.bar.Hide()

	pi.container = container.NewVBox(
		pi.label,
		pi.bar,
	)

	return pi
}

// Start démarre l'indicateur
func (pi *ProgressIndicator) Start(message string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if pi.active {
		return
	}

	pi.active = true
	pi.stopChan = make(chan struct{})
	pi.label.SetText(message)
	pi.bar.Show()

	go func() {
		chars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-pi.stopChan:
				return
			case <-ticker.C:
				pi.mu.Lock()
				if pi.active {
					pi.label.SetText(fmt.Sprintf("%s %s", chars[i%len(chars)], message))
				}
				pi.mu.Unlock()
				i++
			}
		}
	}()
}

// SetProgress définit la progression (0.0 à 1.0)
func (pi *ProgressIndicator) SetProgress(value float64) {
	pi.bar.SetValue(value)
}

// Stop arrête l'indicateur
func (pi *ProgressIndicator) Stop(finalMessage string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if !pi.active {
		return
	}

	pi.active = false
	if pi.stopChan != nil {
		close(pi.stopChan)
	}
	pi.label.SetText(finalMessage)
	pi.bar.Hide()
}

// GetContainer retourne le conteneur
func (pi *ProgressIndicator) GetContainer() *fyne.Container {
	return pi.container
}

// ============================================================================
// KEYBOARD SHORTCUTS
// ============================================================================

// ShortcutHandler gère les raccourcis clavier
type ShortcutHandler struct {
	shortcuts map[string]func()
}

// NewShortcutHandler crée un nouveau gestionnaire de raccourcis
func NewShortcutHandler() *ShortcutHandler {
	return &ShortcutHandler{
		shortcuts: make(map[string]func()),
	}
}

// Register enregistre un raccourci
func (sh *ShortcutHandler) Register(shortcut string, action func()) {
	sh.shortcuts[shortcut] = action
}

// SetupWindowShortcuts configure les raccourcis pour une fenêtre
func (sh *ShortcutHandler) SetupWindowShortcuts(win fyne.Window) {
	// Ctrl+S - Sync
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyS,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		if action, ok := sh.shortcuts["sync"]; ok {
			action()
		}
	})

	// Ctrl+R - Refresh
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyR,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		if action, ok := sh.shortcuts["refresh"]; ok {
			action()
		}
	})

	// Ctrl+E - Explorer
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyE,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		if action, ok := sh.shortcuts["explorer"]; ok {
			action()
		}
	})

	// Ctrl+T - Toggle Theme
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyT,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		ToggleTheme()
	})

	// Ctrl+L - Clear Logs
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyL,
		Modifier: fyne.KeyModifierControl,
	}, func(shortcut fyne.Shortcut) {
		if action, ok := sh.shortcuts["clearlogs"]; ok {
			action()
		}
	})

	// F11 - Fullscreen
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyF11,
	}, func(shortcut fyne.Shortcut) {
		win.SetFullScreen(!win.FullScreen())
	})

	// Escape - Exit fullscreen
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyEscape,
	}, func(shortcut fyne.Shortcut) {
		if win.FullScreen() {
			win.SetFullScreen(false)
		}
	})
}

// ============================================================================
// ACTIVITY LOG WITH COLORS
// ============================================================================

// UILogLevel représente le niveau de log pour l'affichage UI
type UILogLevel int

const (
	UILogInfo UILogLevel = iota
	UILogSuccess
	UILogWarning
	UILogError
)

// ColoredLog représente un log coloré
type ColoredLog struct {
	Time    time.Time
	Message string
	Level   UILogLevel
}

// LogViewer est un visualiseur de logs amélioré
type LogViewer struct {
	container   *fyne.Container
	list        *widget.List
	logs        []ColoredLog
	filteredIdx []int
	searchBar   *SearchBar
	mu          sync.Mutex
	maxLogs     int
}

// NewLogViewer crée un nouveau visualiseur de logs
func NewLogViewer(maxLogs int) *LogViewer {
	lv := &LogViewer{
		logs:        []ColoredLog{},
		filteredIdx: []int{},
		maxLogs:     maxLogs,
	}

	// Liste de logs
	lv.list = widget.NewList(
		func() int { return len(lv.filteredIdx) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel(""),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			lv.mu.Lock()
			defer lv.mu.Unlock()

			if id >= len(lv.filteredIdx) {
				return
			}

			logIdx := lv.filteredIdx[id]
			if logIdx >= len(lv.logs) {
				return
			}

			log := lv.logs[logIdx]
			hbox := obj.(*fyne.Container)
			timeLabel := hbox.Objects[0].(*widget.Label)
			msgLabel := hbox.Objects[1].(*widget.Label)

			timeLabel.SetText(log.Time.Format("15:04:05"))
			timeLabel.TextStyle = fyne.TextStyle{Monospace: true}

			msgLabel.SetText(log.Message)
			msgLabel.TextStyle = fyne.TextStyle{Monospace: true}
		},
	)

	// Barre de recherche
	lv.searchBar = NewSearchBar("Filtrer les logs...", func(query string) {
		lv.filter(query)
	})

	// Clear button
	clearBtn := widget.NewButton("🗑️ Vider", func() {
		lv.Clear()
	})
	clearBtn.Importance = widget.LowImportance

	header := container.NewBorder(
		nil, nil, nil,
		clearBtn,
		lv.searchBar.GetContainer(),
	)

	lv.container = container.NewBorder(
		header,
		nil, nil, nil,
		lv.list,
	)

	// Initialiser l'index filtré
	lv.filter("")

	return lv
}

// AddLog ajoute un log
func (lv *LogViewer) AddLog(message string, level UILogLevel) {
	lv.mu.Lock()
	defer lv.mu.Unlock()

	log := ColoredLog{
		Time:    time.Now(),
		Message: message,
		Level:   level,
	}

	lv.logs = append(lv.logs, log)

	// Limiter le nombre de logs
	if len(lv.logs) > lv.maxLogs {
		lv.logs = lv.logs[len(lv.logs)-lv.maxLogs:]
	}

	// Mettre à jour le filtre
	lv.filteredIdx = append(lv.filteredIdx, len(lv.logs)-1)

	lv.list.Refresh()
	lv.list.ScrollToBottom()
}

// filter filtre les logs
func (lv *LogViewer) filter(query string) {
	lv.mu.Lock()
	defer lv.mu.Unlock()

	lv.filteredIdx = []int{}
	query = stringToLower(query)

	for i, log := range lv.logs {
		if query == "" || containsIgnoreCase(log.Message, query) {
			lv.filteredIdx = append(lv.filteredIdx, i)
		}
	}

	lv.list.Refresh()
}

// Clear efface tous les logs
func (lv *LogViewer) Clear() {
	lv.mu.Lock()
	defer lv.mu.Unlock()

	lv.logs = []ColoredLog{}
	lv.filteredIdx = []int{}
	lv.list.Refresh()
}

// GetContainer retourne le conteneur
func (lv *LogViewer) GetContainer() *fyne.Container {
	return lv.container
}

// Helper functions
func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func containsIgnoreCase(s, substr string) bool {
	s = stringToLower(s)
	substr = stringToLower(substr)
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ============================================================================
// SYSTEM INFO
// ============================================================================

// GetSystemInfo retourne les informations système
func GetSystemInfo() map[string]string {
	info := make(map[string]string)
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpus"] = fmt.Sprintf("%d", runtime.NumCPU())
	info["goroutines"] = fmt.Sprintf("%d", runtime.NumGoroutine())

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info["memory_alloc"] = formatBytes(m.Alloc)
	info["memory_total"] = formatBytes(m.TotalAlloc)

	return info
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
