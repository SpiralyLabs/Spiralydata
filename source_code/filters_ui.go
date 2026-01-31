package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ExtensionTag représente un tag d'extension avec bouton de suppression
type ExtensionTag struct {
	widget.BaseWidget
	extension string
	onRemove  func(string)
	container *fyne.Container
}

// NewExtensionTag crée un nouveau tag d'extension
func NewExtensionTag(ext string, onRemove func(string)) *ExtensionTag {
	tag := &ExtensionTag{
		extension: ext,
		onRemove:  onRemove,
	}
	tag.ExtendBaseWidget(tag)
	return tag
}

// CreateRenderer crée le renderer pour le tag
func (t *ExtensionTag) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.RGBA{R: 60, G: 120, B: 200, A: 255})
	bg.CornerRadius = 4

	label := widget.NewLabel(t.extension)
	label.TextStyle = fyne.TextStyle{Monospace: true}

	removeBtn := widget.NewButton("×", func() {
		if t.onRemove != nil {
			t.onRemove(t.extension)
		}
	})
	removeBtn.Importance = widget.LowImportance

	t.container = container.NewHBox(label, removeBtn)
	content := container.NewStack(bg, container.NewPadded(t.container))

	return widget.NewSimpleRenderer(content)
}

// FilterConfigUI gère l'interface des filtres
type FilterConfigUI struct {
	config        *FilterConfig
	win           fyne.Window
	tagsContainer *fyne.Container
	modeToggle    *widget.Button
	summaryLabel  *widget.Label
	onUpdate      func()
}

// NewFilterConfigUI crée une nouvelle interface de filtres
func NewFilterConfigUI(config *FilterConfig, win fyne.Window, onUpdate func()) *FilterConfigUI {
	return &FilterConfigUI{
		config:   config,
		win:      win,
		onUpdate: onUpdate,
	}
}

// CreateExtensionFilterPanel crée le panneau de filtrage par extension
// Sans le checkbox "Activer le filtrage" en haut
func (ui *FilterConfigUI) CreateExtensionFilterPanel() *fyne.Container {
	// Titre sans émoji
	titleLabel := widget.NewLabelWithStyle("Filtrage par Extension", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Label du mode
	modeLabel := widget.NewLabel("Extensions à exclure:")

	// Bouton toggle mode (sans émoji)
	ui.modeToggle = widget.NewButton("Inverser (Whitelist)", func() {
		currentMode := ui.config.Filters.Extension.GetMode()
		if currentMode == FilterModeBlacklist {
			ui.config.Filters.Extension.SetMode(FilterModeWhitelist)
			ui.modeToggle.SetText("Inverser (Blacklist)")
			modeLabel.SetText("Extensions autorisées:")
		} else {
			ui.config.Filters.Extension.SetMode(FilterModeBlacklist)
			ui.modeToggle.SetText("Inverser (Whitelist)")
			modeLabel.SetText("Extensions à exclure:")
		}
		if ui.onUpdate != nil {
			ui.onUpdate()
		}
	})
	ui.modeToggle.Importance = widget.LowImportance

	// Initialiser le texte du mode
	if ui.config.Filters.Extension.GetMode() == FilterModeWhitelist {
		ui.modeToggle.SetText("Inverser (Blacklist)")
		modeLabel.SetText("Extensions autorisées:")
	}

	// Conteneur pour les tags
	ui.tagsContainer = container.NewGridWrap(fyne.NewSize(80, 35))
	ui.refreshTags()

	tagsScroll := container.NewHScroll(ui.tagsContainer)
	tagsScroll.SetMinSize(fyne.NewSize(400, 45))

	// Bouton ajouter (sans émoji)
	addBtn := widget.NewButton("Ajouter", func() {
		ui.showAddExtensionDialog()
	})
	addBtn.Importance = widget.HighImportance

	// Bouton suggestions (sans émoji)
	suggestBtn := widget.NewButton("Suggestions", func() {
		ui.showSuggestionsDialog()
	})
	suggestBtn.Importance = widget.MediumImportance

	// Bouton tout effacer (emoji seul OK)
	clearBtn := widget.NewButton("🗑️", func() {
		dialog.ShowConfirm("Confirmer", "Supprimer toutes les extensions?", func(ok bool) {
			if ok {
				ui.config.Filters.Extension.Clear()
				ui.refreshTags()
				if ui.onUpdate != nil {
					ui.onUpdate()
				}
			}
		}, ui.win)
	})
	clearBtn.Importance = widget.DangerImportance

	// Résumé
	ui.summaryLabel = widget.NewLabel("")
	ui.updateSummary()

	// Layout
	modeRow := container.NewHBox(
		modeLabel,
		layout.NewSpacer(),
		ui.modeToggle,
	)

	buttonsRow := container.NewHBox(
		addBtn,
		suggestBtn,
		layout.NewSpacer(),
		clearBtn,
	)

	return container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		modeRow,
		tagsScroll,
		buttonsRow,
		widget.NewSeparator(),
		ui.summaryLabel,
	)
}

// refreshTags rafraîchit l'affichage des tags
func (ui *FilterConfigUI) refreshTags() {
	ui.tagsContainer.Objects = nil

	extensions := ui.config.Filters.Extension.GetExtensions()
	for _, ext := range extensions {
		extCopy := ext
		tag := ui.createTagWidget(extCopy)
		ui.tagsContainer.Add(tag)
	}

	ui.tagsContainer.Refresh()
	ui.updateSummary()
}

// createTagWidget crée un widget tag pour une extension
func (ui *FilterConfigUI) createTagWidget(ext string) fyne.CanvasObject {
	// Fond coloré
	var bgColor color.Color
	if ui.config.Filters.Extension.GetMode() == FilterModeBlacklist {
		bgColor = color.RGBA{R: 180, G: 60, B: 60, A: 255} // Rouge pour exclusion
	} else {
		bgColor = color.RGBA{R: 60, G: 140, B: 60, A: 255} // Vert pour inclusion
	}

	bg := canvas.NewRectangle(bgColor)
	bg.CornerRadius = 4

	label := canvas.NewText(ext, color.White)
	label.TextSize = 12
	label.TextStyle = fyne.TextStyle{Monospace: true}

	removeBtn := widget.NewButton("×", func() {
		ui.config.Filters.Extension.RemoveExtension(ext)
		ui.refreshTags()
		if ui.onUpdate != nil {
			ui.onUpdate()
		}
		addLog(fmt.Sprintf("Extension supprimée: %s", ext))
	})
	removeBtn.Importance = widget.LowImportance

	content := container.NewHBox(
		container.NewPadded(label),
		removeBtn,
	)

	return container.NewStack(bg, content)
}

// showAddExtensionDialog affiche le dialogue d'ajout d'extension
func (ui *FilterConfigUI) showAddExtensionDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("ex: .jpg ou jpg")

	errorLabel := widget.NewLabel("")
	errorLabel.TextStyle = fyne.TextStyle{Italic: true}

	content := container.NewVBox(
		widget.NewLabel("Entrez l'extension à ajouter:"),
		entry,
		errorLabel,
	)

	dialog.ShowCustomConfirm("Ajouter une extension", "Ajouter", "Annuler", content, func(ok bool) {
		if ok {
			ext := entry.Text
			if ext == "" {
				return
			}

			err := ui.config.Filters.Extension.AddExtension(ext)
			if err != nil {
				dialog.ShowError(err, ui.win)
				return
			}

			ui.refreshTags()
			if ui.onUpdate != nil {
				ui.onUpdate()
			}
			addLog(fmt.Sprintf("Extension ajoutée: %s", ext))
		}
	}, ui.win)

	// Validation en temps réel
	entry.OnChanged = func(s string) {
		if s == "" {
			errorLabel.SetText("")
			return
		}
		if !ValidateExtension(s) {
			errorLabel.SetText("Format invalide")
		} else {
			normalized, _ := NormalizeExtension(s)
			errorLabel.SetText(fmt.Sprintf("Sera ajouté comme: %s", normalized))
		}
	}
}

// showSuggestionsDialog affiche le dialogue de suggestions
func (ui *FilterConfigUI) showSuggestionsDialog() {
	var checkboxes []*widget.Check
	categoryBoxes := make(map[string]*widget.Check)

	content := container.NewVBox()

	for category, extensions := range SuggestedExtensions {
		catCopy := category
		extList := strings.Join(extensions, ", ")

		check := widget.NewCheck(fmt.Sprintf("%s (%s)", category, extList), func(checked bool) {})
		categoryBoxes[catCopy] = check
		checkboxes = append(checkboxes, check)
		content.Add(check)
	}

	// Ajouter une option pour les dossiers communs
	content.Add(widget.NewSeparator())
	content.Add(widget.NewLabel("Dossiers à exclure:"))

	folderChecks := make(map[string]*widget.Check)
	for _, folder := range CommonExcludedFolders[:7] {
		folderCopy := folder
		check := widget.NewCheck(folder, func(checked bool) {})
		folderChecks[folderCopy] = check
		content.Add(check)
	}

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(350, 300))

	dialog.ShowCustomConfirm("Suggestions", "Appliquer", "Annuler", scroll, func(ok bool) {
		if ok {
			for category, check := range categoryBoxes {
				if check.Checked {
					ui.config.Filters.Extension.AddSuggestedCategory(category)
					addLog(fmt.Sprintf("Catégorie ajoutée: %s", category))
				}
			}

			for folder, check := range folderChecks {
				if check.Checked {
					ui.config.Filters.Path.AddExcludedFolder(folder)
					addLog(fmt.Sprintf("Dossier exclu: %s", folder))
				}
			}

			ui.refreshTags()
			if ui.onUpdate != nil {
				ui.onUpdate()
			}
		}
	}, ui.win)
}

// updateSummary met à jour le résumé
func (ui *FilterConfigUI) updateSummary() {
	if ui.summaryLabel == nil {
		return
	}

	extCount := len(ui.config.Filters.Extension.GetExtensions())
	mode := "Blacklist"
	if ui.config.Filters.Extension.GetMode() == FilterModeWhitelist {
		mode = "Whitelist"
	}

	ui.summaryLabel.SetText(fmt.Sprintf("%d extension(s) - Mode: %s", extCount, mode))
}

// updateUI met à jour l'interface
func (ui *FilterConfigUI) updateUI() {
	ui.refreshTags()
	ui.updateSummary()
}

// CreateSizeFilterPanel crée le panneau de filtrage par taille
func (ui *FilterConfigUI) CreateSizeFilterPanel() *fyne.Container {
	titleLabel := widget.NewLabelWithStyle("Filtrage par Taille", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Taille min
	minLabel := widget.NewLabel("Taille minimum:")
	minEntry := widget.NewEntry()
	minEntry.SetPlaceHolder("0")
	if ui.config.Filters.Size.MinSize > 0 {
		minEntry.SetText(strconv.FormatInt(ui.config.Filters.Size.MinSize, 10))
	}

	// Container avec taille fixe pour le champ
	minEntryContainer := container.NewGridWrap(fyne.NewSize(120, 36), minEntry)

	minUnitSelect := widget.NewSelect([]string{"Octets", "Ko", "Mo", "Go"}, nil)
	minUnitSelect.SetSelected("Octets")

	// Taille max
	maxLabel := widget.NewLabel("Taille maximum:")
	maxEntry := widget.NewEntry()
	maxEntry.SetPlaceHolder("0 (illimité)")
	if ui.config.Filters.Size.MaxSize > 0 {
		maxEntry.SetText(strconv.FormatInt(ui.config.Filters.Size.MaxSize, 10))
	}

	// Container avec taille fixe pour le champ
	maxEntryContainer := container.NewGridWrap(fyne.NewSize(120, 36), maxEntry)

	maxUnitSelect := widget.NewSelect([]string{"Octets", "Ko", "Mo", "Go"}, nil)
	maxUnitSelect.SetSelected("Octets")

	// Callback pour mettre à jour
	updateSize := func() {
		minVal, _ := strconv.ParseInt(minEntry.Text, 10, 64)
		maxVal, _ := strconv.ParseInt(maxEntry.Text, 10, 64)

		// Convertir selon l'unité
		minMultiplier := getMultiplier(minUnitSelect.Selected)
		maxMultiplier := getMultiplier(maxUnitSelect.Selected)

		ui.config.Filters.Size.MinSize = minVal * minMultiplier
		ui.config.Filters.Size.MaxSize = maxVal * maxMultiplier

		if ui.onUpdate != nil {
			ui.onUpdate()
		}
	}

	minEntry.OnChanged = func(s string) { updateSize() }
	maxEntry.OnChanged = func(s string) { updateSize() }
	minUnitSelect.OnChanged = func(s string) { updateSize() }
	maxUnitSelect.OnChanged = func(s string) { updateSize() }

	return container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		container.NewHBox(minLabel, minEntryContainer, minUnitSelect),
		container.NewHBox(maxLabel, maxEntryContainer, maxUnitSelect),
	)
}

// getMultiplier retourne le multiplicateur pour l'unité
func getMultiplier(unit string) int64 {
	switch unit {
	case "Ko":
		return 1024
	case "Mo":
		return 1024 * 1024
	case "Go":
		return 1024 * 1024 * 1024
	default:
		return 1
	}
}

// CreatePathFilterPanel crée le panneau de filtrage par chemin
func (ui *FilterConfigUI) CreatePathFilterPanel() *fyne.Container {
	titleLabel := widget.NewLabelWithStyle("Filtrage par Chemin", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Exclure fichiers cachés
	hiddenCheck := widget.NewCheck("Exclure les fichiers cachés", func(checked bool) {
		ui.config.Filters.Path.ExcludeHidden = checked
		if ui.onUpdate != nil {
			ui.onUpdate()
		}
	})
	hiddenCheck.SetChecked(ui.config.Filters.Path.ExcludeHidden)

	// Liste des dossiers exclus
	foldersLabel := widget.NewLabel("Dossiers exclus:")

	// Déclarer la variable avant pour pouvoir l'utiliser dans le callback
	var foldersList *widget.List
	foldersList = widget.NewList(
		func() int {
			return len(ui.config.Filters.Path.ExcludedFolders)
		},
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewLabel("Dossier"), widget.NewButton("×", nil))
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			box := item.(*fyne.Container)
			label := box.Objects[0].(*widget.Label)
			btn := box.Objects[1].(*widget.Button)

			if id < len(ui.config.Filters.Path.ExcludedFolders) {
				folder := ui.config.Filters.Path.ExcludedFolders[id]
				label.SetText(folder)
				btn.OnTapped = func() {
					ui.config.Filters.Path.RemoveExcludedFolder(folder)
					foldersList.Refresh()
					if ui.onUpdate != nil {
						ui.onUpdate()
					}
				}
			}
		},
	)

	// Wrapper avec scroll et taille minimum
	foldersScroll := container.NewVScroll(foldersList)
	foldersScroll.SetMinSize(fyne.NewSize(200, 100))

	// Bouton ajouter dossier
	addFolderBtn := widget.NewButton("Ajouter dossier", func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("ex: node_modules")

		dialog.ShowCustomConfirm("Ajouter un dossier à exclure", "Ajouter", "Annuler",
			container.NewVBox(
				widget.NewLabel("Nom du dossier:"),
				entry,
			), func(ok bool) {
				if ok && entry.Text != "" {
					ui.config.Filters.Path.AddExcludedFolder(entry.Text)
					foldersList.Refresh()
					if ui.onUpdate != nil {
						ui.onUpdate()
					}
				}
			}, ui.win)
	})

	return container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		hiddenCheck,
		foldersLabel,
		foldersScroll,
		addFolderBtn,
	)
}

// CreateFullFilterPanel crée le panneau complet avec tous les filtres
// Avec boutons Appliquer, Réinitialiser, Exporter, Importer
func (ui *FilterConfigUI) CreateFullFilterPanel() *fyne.Container {
	extPanel := ui.CreateExtensionFilterPanel()
	sizePanel := ui.CreateSizeFilterPanel()
	pathPanel := ui.CreatePathFilterPanel()

	// Checkbox pour sauvegarder lors de l'application
	saveOnApplyCheck := widget.NewCheck("Enregistrer la configuration", nil)
	saveOnApplyCheck.SetChecked(false)

	// Bouton Appliquer
	applyBtn := widget.NewButton("Appliquer", func() {
		// Activer le filtrage si des extensions sont définies
		if len(ui.config.Filters.Extension.GetExtensions()) > 0 {
			ui.config.Filters.Extension.SetEnabled(true)
		}

		if saveOnApplyCheck.Checked {
			if err := SaveFiltersToConfig(ui.config); err != nil {
				dialog.ShowError(err, ui.win)
			} else {
				addLog("Filtres sauvegardés")
			}
		}

		if ui.onUpdate != nil {
			ui.onUpdate()
		}
		addLog("Filtres appliqués")
	})
	applyBtn.Importance = widget.HighImportance

	// Bouton Réinitialiser
	resetBtn := widget.NewButton("Réinitialiser", func() {
		dialog.ShowConfirm("Confirmer", "Réinitialiser tous les filtres?", func(ok bool) {
			if ok {
				ui.config.Filters = NewFilterConfig().Filters
				ui.config.Filters.Extension.SetEnabled(false)
				ui.refreshTags()
				if ui.onUpdate != nil {
					ui.onUpdate()
				}
				addLog("Filtres réinitialisés")
			}
		}, ui.win)
	})
	resetBtn.Importance = widget.DangerImportance

	// Bouton Exporter
	exportBtn := widget.NewButton("Exporter", func() {
		ui.exportFilters()
	})
	exportBtn.Importance = widget.MediumImportance

	// Bouton Importer
	importBtn := widget.NewButton("Importer", func() {
		ui.importFilters()
	})
	importBtn.Importance = widget.MediumImportance

	// Layout des boutons d'action
	actionsRow := container.NewHBox(
		applyBtn,
		resetBtn,
		layout.NewSpacer(),
		exportBtn,
		importBtn,
	)

	return container.NewVBox(
		container.NewPadded(extPanel),
		container.NewPadded(sizePanel),
		container.NewPadded(pathPanel),
		widget.NewSeparator(),
		saveOnApplyCheck,
		container.NewPadded(actionsRow),
	)
}

// FilterExportData structure pour l'export des filtres
type FilterExportData struct {
	Extensions      []string `json:"extensions"`
	ExtensionMode   int      `json:"extension_mode"`
	MinSize         int64    `json:"min_size"`
	MaxSize         int64    `json:"max_size"`
	ExcludeHidden   bool     `json:"exclude_hidden"`
	ExcludedFolders []string `json:"excluded_folders"`
}

// exportFilters exporte les filtres vers un fichier JSON
func (ui *FilterConfigUI) exportFilters() {
	exportData := FilterExportData{
		Extensions:      ui.config.Filters.Extension.GetExtensions(),
		ExtensionMode:   int(ui.config.Filters.Extension.GetMode()),
		MinSize:         ui.config.Filters.Size.MinSize,
		MaxSize:         ui.config.Filters.Size.MaxSize,
		ExcludeHidden:   ui.config.Filters.Path.ExcludeHidden,
		ExcludedFolders: ui.config.Filters.Path.ExcludedFolders,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}

	// Sauvegarder dans un fichier
	exportPath := "spiraly_filters.json"
	err = os.WriteFile(exportPath, data, 0644)
	if err != nil {
		dialog.ShowError(err, ui.win)
		return
	}

	dialog.ShowInformation("Export réussi", fmt.Sprintf("Filtres exportés vers: %s", exportPath), ui.win)
	addLog(fmt.Sprintf("Filtres exportés vers %s", exportPath))
}

// importFilters importe les filtres depuis un fichier JSON
func (ui *FilterConfigUI) importFilters() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ui.win)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()

		var exportData FilterExportData
		decoder := json.NewDecoder(reader)
		if err := decoder.Decode(&exportData); err != nil {
			dialog.ShowError(fmt.Errorf("Fichier invalide: %v", err), ui.win)
			return
		}

		// Appliquer les données importées
		ui.config.Filters.Extension.Clear()
		for _, ext := range exportData.Extensions {
			ui.config.Filters.Extension.AddExtension(ext)
		}
		ui.config.Filters.Extension.SetMode(FilterMode(exportData.ExtensionMode))
		ui.config.Filters.Size.MinSize = exportData.MinSize
		ui.config.Filters.Size.MaxSize = exportData.MaxSize
		ui.config.Filters.Path.ExcludeHidden = exportData.ExcludeHidden
		ui.config.Filters.Path.ExcludedFolders = exportData.ExcludedFolders

		ui.refreshTags()
		if ui.onUpdate != nil {
			ui.onUpdate()
		}

		dialog.ShowInformation("Import réussi", "Filtres importés avec succès", ui.win)
		addLog("Filtres importés")
	}, ui.win)
}

// ShowFilterDialog affiche le dialogue de configuration des filtres
func ShowFilterDialog(config *FilterConfig, win fyne.Window, onUpdate func()) {
	ui := NewFilterConfigUI(config, win, onUpdate)
	content := ui.CreateFullFilterPanel()

	scroll := container.NewVScroll(content)
	scroll.SetMinSize(fyne.NewSize(500, 500))

	dialog.ShowCustom("Configuration des Filtres", "Fermer", scroll, win)
}

// CreateFilterIndicator crée un indicateur de filtres actifs pour l'UI
func CreateFilterIndicator(config *FilterConfig) *fyne.Container {
	summary := config.GetSummary()
	label := widget.NewLabel(summary)
	label.TextStyle = fyne.TextStyle{Italic: true}

	return container.NewHBox(label)
}
