package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type FileTreeItem struct {
	Path     string
	Name     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	Children []*FileTreeItem
	Parent   *FileTreeItem
}

type FileExplorer struct {
	client           *Client
	win              fyne.Window
	allItems         map[string]*FileTreeItem
	currentDir       *FileTreeItem
	rootDir          *FileTreeItem
	selectedItems    map[string]bool
	mu               sync.Mutex
	loadingLabel     *widget.Label
	contentContainer *fyne.Container
	backCallback     func()
	treeLoaded       bool
	settings         *ExplorerSettings
	previewPanel     *PreviewPanel
	showingPreview   bool
	searchQuery      string // Requête de recherche actuelle
}

func NewFileExplorer(client *Client, win fyne.Window, backCallback func()) *FileExplorer {
	return &FileExplorer{
		client:        client,
		win:           win,
		allItems:      make(map[string]*FileTreeItem),
		selectedItems: make(map[string]bool),
		backCallback:  backCallback,
		treeLoaded:    false,
		settings:      NewExplorerSettings(),
	}
}

func (fe *FileExplorer) Show() {
	// Vérification de sécurité
	if fe.client == nil {
		addLog("❌ Erreur: client est nil")
		if fe.backCallback != nil {
			fe.backCallback()
		}
		return
	}
	
	if fe.treeLoaded {
		fe.showDirectoryUI()
		return
	}
	
	fe.loadingLabel = widget.NewLabel("📄 Chargement de la structure des fichiers...")
	fe.loadingLabel.Alignment = fyne.TextAlignCenter
	
	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	loadingIndex := 0
	stopLoading := false
	
	progressLabel := widget.NewLabel("Fichiers scannés : 0")
	progressLabel.Alignment = fyne.TextAlignCenter
	
	cancelBtn := widget.NewButton("Annuler", func() {
		stopLoading = true
		fe.client.explorerActive = false
		fe.backCallback()
	})
	
	loadingContent := container.NewVBox(
		layout.NewSpacer(),
		fe.loadingLabel,
		progressLabel,
		layout.NewSpacer(),
		container.NewCenter(cancelBtn),
		layout.NewSpacer(),
	)
	
	fe.contentContainer = container.NewVBox(loadingContent)
	
	// Vérification de logWidget
	var logPanel fyne.CanvasObject
	if logWidget != nil {
		logPanel = container.NewBorder(
			widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil,
			container.NewVScroll(logWidget),
		)
	} else {
		logPanel = widget.NewLabel("Logs non disponibles")
	}
	
	split := container.NewHSplit(
		fe.contentContainer,
		logPanel,
	)
	split.Offset = 0.5
	
	fe.win.SetContent(split)
	
	go func() {
		for !stopLoading {
			time.Sleep(100 * time.Millisecond)
			if !stopLoading {
				char := loadingChars[loadingIndex%len(loadingChars)]
				fe.loadingLabel.SetText(fmt.Sprintf("%s Chargement de la structure des fichiers...", char))
				fe.loadingLabel.Refresh()
				loadingIndex++
			}
		}
	}()
	
	go fe.requestAndBuildTree(&stopLoading, progressLabel)
}

func (fe *FileExplorer) requestAndBuildTree(stopLoading *bool, progressLabel *widget.Label) {
	addLog("📂 Scan complet de la structure des fichiers...")

	// Vérification de sécurité
	if fe.client == nil {
		addLog("❌ Erreur: client est nil dans requestAndBuildTree")
		*stopLoading = true
		if fe.backCallback != nil {
			fe.backCallback()
		}
		return
	}

	// Créer le channel AVANT d'activer explorerActive
	fe.client.treeItemsChan = make(chan FileTreeItemMessage, 500)
	
	// Petit délai pour s'assurer que le channel est prêt
	time.Sleep(100 * time.Millisecond)
	
	// Maintenant activer le flag
	fe.client.explorerActive = true

	reqMsg := map[string]string{
		"type":   "request_file_tree",
		"origin": "client",
	}

	addLog("📤 Envoi de la requête file_tree...")
	err := fe.client.WriteJSONSafe(reqMsg)

	if err != nil {
		addLog(fmt.Sprintf("❌ Erreur lors de la demande: %v", err))
		*stopLoading = true
		fe.client.explorerActive = false
		// Utiliser le thread principal pour le callback
		fe.win.Canvas().Content().Refresh()
		fe.backCallback()
		return
	}
	
	addLog("✅ Requête envoyée, attente des données...")

	filesReceived := 0
	timeout := time.After(60 * time.Second)
	complete := false

	for !complete && !*stopLoading {
		select {
		case <-timeout:
			addLog(fmt.Sprintf("⏱️ Timeout - %d éléments reçus", filesReceived))
			*stopLoading = true
			fe.client.explorerActive = false
			if filesReceived > 0 {
				addLog("🔨 Construction avec les éléments reçus...")
				fe.buildTreeStructure()
				// Appeler showDirectoryUI sur le thread principal
				fe.win.Canvas().Refresh(fe.win.Canvas().Content())
				time.Sleep(50 * time.Millisecond)
				fe.safeShowDirectoryUI()
			} else {
				fe.backCallback()
			}
			return

		case treeItem, ok := <-fe.client.treeItemsChan:
			if !ok {
				addLog("❌ Channel fermé")
				*stopLoading = true
				fe.client.explorerActive = false
				fe.backCallback()
				return
			}
			
			if treeItem.Type == "file_tree_item" {
				item := &FileTreeItem{
					Path:     treeItem.Path,
					Name:     treeItem.Name,
					IsDir:    treeItem.IsDir,
					Children: []*FileTreeItem{},
				}
				fe.allItems[item.Path] = item
				filesReceived++

				if filesReceived%10 == 0 || filesReceived < 10 {
					progressLabel.SetText(fmt.Sprintf("Fichiers scannés : %d", filesReceived))
					progressLabel.Refresh()
				}
			} else if treeItem.Type == "file_tree_complete" {
				*stopLoading = true
				complete = true
				fe.client.explorerActive = false
				addLog(fmt.Sprintf("✅ Structure complète chargée: %d éléments", filesReceived))
				fe.buildTreeStructure()
				// Appeler showDirectoryUI sur le thread principal avec un petit délai
				time.Sleep(50 * time.Millisecond)
				fe.safeShowDirectoryUI()
				return
			}
		}
	}
}

// safeShowDirectoryUI appelle showDirectoryUI de manière sécurisée
func (fe *FileExplorer) safeShowDirectoryUI() {
	// Récupérer les panics potentiels
	defer func() {
		if r := recover(); r != nil {
			addLog(fmt.Sprintf("❌ Erreur UI récupérée: %v", r))
		}
	}()
	
	// Petit délai pour laisser le temps au thread UI
	time.Sleep(100 * time.Millisecond)
	
	fe.showDirectoryUI()
}

func (fe *FileExplorer) buildTreeStructure() {
	addLog("🔨 Construction de l'arborescence...")

	fe.rootDir = &FileTreeItem{
		Path:     "",
		Name:     "Spiralydata",
		IsDir:    true,
		Children: []*FileTreeItem{},
	}
	fe.allItems[""] = fe.rootDir

	// S'assurer que tous les items ont Children initialisé
	for _, item := range fe.allItems {
		if item.Children == nil {
			item.Children = []*FileTreeItem{}
		}
	}

	for path, item := range fe.allItems {
		if path == "" {
			continue
		}

		parentPath := filepath.Dir(path)
		parentPath = filepath.ToSlash(parentPath)

		if parentPath == "." {
			parentPath = ""
		}

		if parent, exists := fe.allItems[parentPath]; exists {
			item.Parent = parent
			if parent.Children == nil {
				parent.Children = []*FileTreeItem{}
			}
			parent.Children = append(parent.Children, item)
		} else {
			// Si le parent n'existe pas, ajouter à la racine
			item.Parent = fe.rootDir
			fe.rootDir.Children = append(fe.rootDir.Children, item)
		}
	}

	for _, item := range fe.allItems {
		if item.IsDir && len(item.Children) > 0 {
			sort.Slice(item.Children, func(i, j int) bool {
				if item.Children[i].IsDir != item.Children[j].IsDir {
					return item.Children[i].IsDir
				}
				return item.Children[i].Name < item.Children[j].Name
			})
		}
	}

	fe.currentDir = fe.rootDir
	fe.treeLoaded = true
	addLog(fmt.Sprintf("✅ Arborescence construite: %d éléments, %d dans la racine",
		len(fe.allItems), len(fe.rootDir.Children)))
}

func (fe *FileExplorer) showDirectoryUI() {
	defer func() {
		if r := recover(); r != nil {
			addLog(fmt.Sprintf("Erreur dans explorateur: %v", r))
		}
	}()

	// Vérifications de sécurité
	if fe.currentDir == nil {
		if fe.rootDir != nil {
			fe.currentDir = fe.rootDir
		} else {
			fe.backCallback()
			return
		}
	}

	if fe.settings == nil {
		fe.settings = NewExplorerSettings()
	}

	currentPath := fe.currentDir.Path
	if currentPath == "" {
		currentPath = "/"
	}

	// S'assurer que Children n'est pas nil
	if fe.currentDir.Children == nil {
		fe.currentDir.Children = []*FileTreeItem{}
	}

	// Breadcrumb navigation
	breadcrumb := NewBreadcrumb(func(index int) {
		if index == -1 {
			if fe.rootDir != nil {
				fe.currentDir = fe.rootDir
			}
		} else {
			parts := fe.getPathParts()
			if index < len(parts) {
				targetPath := ""
				for i := 0; i <= index; i++ {
					if targetPath == "" {
						targetPath = parts[i]
					} else {
						targetPath = targetPath + "/" + parts[i]
					}
				}
				if item, exists := fe.allItems[targetPath]; exists {
					fe.currentDir = item
				}
			}
		}
		fe.showDirectoryUI()
	})
	breadcrumb.SetPath(fe.getPathParts())

	// Barre de recherche avec bouton
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Rechercher...")
	if fe.searchQuery != "" {
		searchEntry.SetText(fe.searchQuery)
	}

	searchBtn := widget.NewButton("Rechercher", func() {
		fe.searchQuery = searchEntry.Text
		fe.showDirectoryUI()
	})
	searchBtn.Importance = widget.MediumImportance

	clearSearchBtn := widget.NewButton("x", func() {
		fe.searchQuery = ""
		searchEntry.SetText("")
		fe.showDirectoryUI()
	})
	clearSearchBtn.Importance = widget.LowImportance

	searchContainer := container.NewBorder(nil, nil, nil, 
		container.NewHBox(searchBtn, clearSearchBtn),
		searchEntry,
	)

	// Contrôles de tri - créer SANS callback d'abord pour éviter la boucle
	sortMenu := widget.NewSelect([]string{"Nom", "Taille", "Date", "Type"}, nil)
	sortMenu.SetSelected(fe.settings.GetSortFieldName())
	// Ajouter le callback APRÈS SetSelected
	sortMenu.OnChanged = func(selected string) {
		switch selected {
		case "Nom":
			fe.settings.SortField = SortByName
		case "Taille":
			fe.settings.SortField = SortBySize
		case "Date":
			fe.settings.SortField = SortByDate
		case "Type":
			fe.settings.SortField = SortByType
		}
		fe.showDirectoryUI()
	}

	sortOrderBtn := widget.NewButton(fe.settings.GetSortOrderIcon(), func() {
		fe.settings.ToggleSortOrder()
		fe.showDirectoryUI()
	})
	sortOrderBtn.Importance = widget.LowImportance

	// Bouton favori
	favIcon := "☆"
	if fe.settings.IsFavorite(currentPath) {
		favIcon = "⭐"
	}
	favBtn := widget.NewButton(favIcon, func() {
		if fe.settings.IsFavorite(currentPath) {
			fe.settings.RemoveFavorite(currentPath)
		} else {
			fe.settings.AddFavorite(currentPath)
		}
		fe.showDirectoryUI()
	})
	favBtn.Importance = widget.LowImportance

	// Compteur
	fileCount := len(fe.currentDir.Children)
	countLabel := widget.NewLabel(fmt.Sprintf("📊 %d éléments", fileCount))

	// Boutons sélection
	selectAllBtn := widget.NewButton("Tout sélectionner", func() {
		fe.mu.Lock()
		for _, item := range fe.currentDir.Children {
			if item != nil {
				fe.selectedItems[item.Path] = true
			}
		}
		fe.mu.Unlock()
		fe.showDirectoryUI()
	})
	selectAllBtn.Importance = widget.LowImportance

	deselectAllBtn := widget.NewButton("Tout désélectionner", func() {
		fe.mu.Lock()
		for _, item := range fe.currentDir.Children {
			if item != nil {
				fe.selectedItems[item.Path] = false
			}
		}
		fe.mu.Unlock()
		fe.showDirectoryUI()
	})
	deselectAllBtn.Importance = widget.LowImportance

	// Bouton parent
	parentBtn := widget.NewButton("⬆️", func() {
		if fe.currentDir != nil && fe.currentDir.Parent != nil {
			fe.currentDir = fe.currentDir.Parent
			fe.showDirectoryUI()
		}
	})
	parentBtn.Importance = widget.MediumImportance
	if fe.currentDir.Parent == nil {
		parentBtn.Disable()
	}

	// Bouton suppression dossier (sans émoji)
	deleteBtn := widget.NewButton("Supprimer dossier", func() {
		fe.confirmDeleteDirectory()
	})
	deleteBtn.Importance = widget.DangerImportance

	// Bouton refresh
	refreshBtn := widget.NewButton("Actualiser", func() {
		fe.treeLoaded = false
		fe.Show()
	})
	refreshBtn.Importance = widget.LowImportance

	// Barre d'outils
	toolBar := container.NewHBox(
		parentBtn,
		refreshBtn,
		favBtn,
		widget.NewSeparator(),
		widget.NewLabel("Tri:"),
		sortMenu,
		sortOrderBtn,
	)
	if fe.currentDir.Parent != nil {
		toolBar.Add(widget.NewSeparator())
		toolBar.Add(deleteBtn)
	}

	// Barre de sélection
	selectionBar := container.NewHBox(
		countLabel,
		layout.NewSpacer(),
		selectAllBtn,
		deselectAllBtn,
	)

	// Trier les éléments
	sortedChildren := fe.settings.SortItems(fe.currentDir.Children)

	// Filtrer si recherche active
	var filteredChildren []*FileTreeItem
	if fe.searchQuery != "" {
		query := strings.ToLower(fe.searchQuery)
		for _, item := range sortedChildren {
			if item != nil && strings.Contains(strings.ToLower(item.Name), query) {
				filteredChildren = append(filteredChildren, item)
			}
		}
		sortedChildren = filteredChildren
	}

	// Contenu de l'arborescence
	treeContent := container.NewVBox()
	for _, item := range sortedChildren {
		if item == nil {
			continue
		}
		
		icon := getFileIcon(item.Name, item.IsDir)
		displayName := icon + " " + item.Name
		
		itemPath := item.Path
		itemIsDir := item.IsDir
		itemRef := item
		itemName := item.Name

		fe.mu.Lock()
		isSelected := fe.selectedItems[itemPath]
		fe.mu.Unlock()

		check := widget.NewCheck(displayName, func(checked bool) {
			fe.mu.Lock()
			fe.selectedItems[itemPath] = checked
			fe.mu.Unlock()
		})
		check.SetChecked(isSelected)

		row := container.NewHBox(check, layout.NewSpacer())

		if itemIsDir {
			openBtn := widget.NewButton("Ouvrir", func() {
				fe.currentDir = itemRef
				fe.showDirectoryUI()
			})
			openBtn.Importance = widget.LowImportance
			row.Add(openBtn)
		} else {
			// Bouton aperçu pour fichiers supportés
			if CanPreview(itemName) {
				previewBtn := widget.NewButton("Aperçu", func() {
					fe.showFilePreview(itemPath)
				})
				previewBtn.Importance = widget.LowImportance
				row.Add(previewBtn)
			}
		}

		treeContent.Add(row)
	}

	if len(sortedChildren) == 0 {
		emptyLabel := widget.NewLabel("📂 Ce dossier est vide")
		emptyLabel.Alignment = fyne.TextAlignCenter
		treeContent.Add(emptyLabel)
	}

	treeScroll := container.NewVScroll(treeContent)
	treeScroll.SetMinSize(fyne.NewSize(450, 350))

	// Boutons action
	downloadBtn := widget.NewButton("Télécharger", func() {
		fe.showDownloadOptions()
	})
	downloadBtn.Importance = widget.HighImportance

	backBtn := widget.NewButton("Retour", func() {
		fe.backCallback()
	})

	// Compter sélection
	selectedCount := 0
	fe.mu.Lock()
	for _, selected := range fe.selectedItems {
		if selected {
			selectedCount++
		}
	}
	fe.mu.Unlock()
	selectedLabel := widget.NewLabel(fmt.Sprintf("✓ %d sélectionnés", selectedCount))

	// Panneau favoris (sidebar gauche)
	var favoritesPanel fyne.CanvasObject
	if len(fe.settings.Favorites) > 0 {
		favList := container.NewVBox()
		favList.Add(widget.NewLabelWithStyle("⭐ Favoris", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
		for _, fav := range fe.settings.Favorites {
			favPath := fav
			favName := filepath.Base(fav)
			if favName == "" || favName == "/" || favName == "." {
				favName = "Racine"
			}
			btn := widget.NewButton(favName, func() {
				if item, exists := fe.allItems[favPath]; exists {
					fe.currentDir = item
					fe.showDirectoryUI()
				} else if favPath == "" {
					fe.currentDir = fe.rootDir
					fe.showDirectoryUI()
				}
			})
			btn.Importance = widget.LowImportance
			favList.Add(btn)
		}
		favoritesPanel = container.NewVBox(favList, widget.NewSeparator())
	}

	// Layout principal
	mainContent := container.NewBorder(
		container.NewVBox(
			breadcrumb.GetContainer(),
			widget.NewSeparator(),
			searchContainer,
			widget.NewSeparator(),
			toolBar,
			selectionBar,
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewHBox(
				selectedLabel,
				layout.NewSpacer(),
				downloadBtn,
				backBtn,
			),
		),
		favoritesPanel,
		nil,
		treeScroll,
	)

	// Split avec logs
	var logPanel fyne.CanvasObject
	if logWidget != nil {
		logPanel = container.NewBorder(
			widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil,
			container.NewVScroll(logWidget),
		)
	} else {
		logPanel = widget.NewLabel("Logs")
	}
	
	split := container.NewHSplit(
		container.NewPadded(mainContent),
		logPanel,
	)
	split.Offset = 0.5

	fe.win.SetContent(split)
	addLog("✅ Explorateur affiché")
}

// showFilePreview affiche la prévisualisation d'un fichier
func (fe *FileExplorer) showFilePreview(relativePath string) {
	addLog(fmt.Sprintf("👁️ Prévisualisation: %s", relativePath))

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "spiraly_preview_"+filepath.Base(relativePath))

	fe.client.downloadChan = make(chan FileChange, 10)
	fe.client.downloadActive = true

	reqMsg := map[string]interface{}{
		"type":   "download_request",
		"origin": "client",
		"items":  []string{relativePath},
	}

	if err := fe.client.WriteJSONSafe(reqMsg); err != nil {
		addLog(fmt.Sprintf("❌ Erreur demande preview: %v", err))
		fe.client.downloadActive = false
		return
	}

	go func() {
		defer func() {
			fe.client.downloadActive = false
			if r := recover(); r != nil {
				addLog(fmt.Sprintf("❌ Panic dans preview: %v", r))
			}
		}()

		timeout := time.After(30 * time.Second)
		select {
		case <-timeout:
			addLog("⏱️ Timeout prévisualisation")
			return
		case msg := <-fe.client.downloadChan:
			if msg.Content == "" {
				addLog("❌ Fichier vide reçu")
				return
			}

			data, err := base64.StdEncoding.DecodeString(msg.Content)
			if err != nil {
				addLog(fmt.Sprintf("❌ Erreur décodage: %v", err))
				return
			}

			if err := os.WriteFile(tempFile, data, 0644); err != nil {
				addLog(fmt.Sprintf("❌ Erreur écriture temp: %v", err))
				return
			}

			addLog("✅ Fichier téléchargé, affichage preview...")
			fe.showPreviewPanel(tempFile, relativePath)
		}
	}()
}

// showPreviewPanel affiche le panneau de prévisualisation
func (fe *FileExplorer) showPreviewPanel(localPath, originalPath string) {
	addLog(fmt.Sprintf("🖼️ Affichage preview: %s", originalPath))

	// Vérification de sécurité
	if fe.currentDir == nil {
		addLog("❌ currentDir est nil dans showPreviewPanel")
		return
	}

	previewPanel := NewPreviewPanel(fe.win, func() {
		os.Remove(localPath)
		fe.showDirectoryUI()
	})

	previewContent := previewPanel.ShowPreview(localPath)

	// Layout avec explorateur réduit et preview
	treeContent := container.NewVBox()
	if fe.currentDir.Children != nil {
		for _, item := range fe.currentDir.Children {
			if item == nil {
				continue
			}
			icon := getFileIcon(item.Name, item.IsDir)
			lbl := widget.NewLabel(icon + " " + item.Name)
			treeContent.Add(lbl)
		}
	}

	backToLogsBtn := widget.NewButton("Retour aux logs", func() {
		os.Remove(localPath)
		fe.showDirectoryUI()
	})

	dirName := fe.currentDir.Name
	if dirName == "" {
		dirName = "Racine"
	}

	explorerMini := container.NewBorder(
		widget.NewLabelWithStyle("📂 "+dirName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		backToLogsBtn,
		nil, nil,
		container.NewVScroll(treeContent),
	)

	split := container.NewHSplit(
		container.NewPadded(explorerMini),
		previewContent,
	)
	split.Offset = 0.3

	fe.win.SetContent(split)
}

func (fe *FileExplorer) confirmDeleteDirectory() {
	dirName := fe.currentDir.Name
	dirPath := fe.currentDir.Path
	
	warningText := fmt.Sprintf(
		"⚠️ ATTENTION\n\n"+
			"Vous êtes sur le point de SUPPRIMER le dossier :\n\n"+
			"📁 %s\n\n"+
			"Cette action supprimera le dossier et TOUT son contenu\n"+
			"DIRECTEMENT sur le serveur hôte.\n\n"+
			"Cette action est IRRÉVERSIBLE.\n\n"+
			"Voulez-vous vraiment continuer ?",
		dirName,
	)
	
	warningLabel := widget.NewLabel(warningText)
	warningLabel.Wrapping = fyne.TextWrapWord
	
	confirmDialog := dialog.NewCustomConfirm(
		"⚠️ Confirmation de suppression",
		"SUPPRIMER",
		"Annuler",
		warningLabel,
		func(confirmed bool) {
			if confirmed {
				fe.deleteDirectory(dirPath)
			}
		},
		fe.win,
	)
	
	confirmDialog.Show()
}

func (fe *FileExplorer) deleteDirectory(dirPath string) {
	addLog(fmt.Sprintf("🗑️ Suppression du dossier : %s", dirPath))

	// Envoyer la commande de suppression au serveur
	change := FileChange{
		FileName: dirPath,
		Op:       "remove",
		IsDir:    true,
		Origin:   "client",
	}

	err := fe.client.WriteJSONSafe(change)

	if err != nil {
		addLog(fmt.Sprintf("❌ Erreur suppression : %v", err))
		dialog.ShowError(fmt.Errorf("Impossible de supprimer le dossier"), fe.win)
		return
	}

	addLog("✅ Commande de suppression envoyée")

	time.Sleep(200 * time.Millisecond)

	// Supprimer le dossier de l'arborescence locale
	fe.removeDirectoryFromTree(dirPath)
	
	// Nettoyer les sélections
	fe.mu.Lock()
	fe.cleanupDeletedDirectorySelections(dirPath)
	fe.mu.Unlock()
	
	// Retourner au parent
	if fe.currentDir.Parent != nil {
		fe.currentDir = fe.currentDir.Parent
		fe.showDirectoryUI()
	}
}

func (fe *FileExplorer) removeDirectoryFromTree(dirPath string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	
	// Trouver l'item à supprimer
	item, exists := fe.allItems[dirPath]
	if !exists {
		return
	}
	
	// Retirer de la liste des enfants du parent
	if item.Parent != nil {
		newChildren := []*FileTreeItem{}
		for _, child := range item.Parent.Children {
			if child.Path != dirPath {
				newChildren = append(newChildren, child)
			}
		}
		item.Parent.Children = newChildren
	}
	
	// Supprimer récursivement tous les sous-éléments de allItems
	fe.removeItemAndChildren(item)
}

func (fe *FileExplorer) removeItemAndChildren(item *FileTreeItem) {
	// Supprimer tous les enfants d'abord
	for _, child := range item.Children {
		fe.removeItemAndChildren(child)
	}
	
	// Supprimer l'item lui-même
	delete(fe.allItems, item.Path)
}

func (fe *FileExplorer) cleanupDeletedDirectorySelections(dirPath string) {
	// Supprimer les sélections du dossier et de tous ses enfants
	toDelete := []string{}
	for path := range fe.selectedItems {
		// Si le path commence par dirPath ou est égal à dirPath
		if path == dirPath || (len(path) > len(dirPath) && len(dirPath) > 0 && path[:len(dirPath)+1] == dirPath+"/") {
			toDelete = append(toDelete, path)
		}
	}
	
	for _, path := range toDelete {
		delete(fe.selectedItems, path)
	}
}

func (fe *FileExplorer) addItemToList(parent *fyne.Container, item *FileTreeItem) {
	icon := "📄"
	if item.IsDir {
		icon = "📁"
	}
	
	displayName := icon + " " + item.Name
	
	// Vérifier si cet item est sélectionné
	fe.mu.Lock()
	isSelected := fe.selectedItems[item.Path]
	fe.mu.Unlock()
	
	// Créer le conteneur
	itemContainer := container.NewHBox()
	
	// Variables locales pour éviter les problèmes de closure
	itemPath := item.Path
	itemIsDir := item.IsDir
	itemRef := item
	
	// Créer la checkbox
	check := widget.NewCheck(displayName, func(checked bool) {
		fe.mu.Lock()
		fe.selectedItems[itemPath] = checked
		fe.mu.Unlock()
	})
	check.SetChecked(isSelected)
	
	if itemIsDir {
		openBtn := widget.NewButton("Ouvrir", func() {
			fe.currentDir = itemRef
			fe.showDirectoryUI()
		})
		openBtn.Importance = widget.LowImportance
		
		itemContainer.Add(check)
		itemContainer.Add(layout.NewSpacer())
		itemContainer.Add(openBtn)
	} else {
		itemContainer.Add(check)
	}
	
	parent.Add(itemContainer)
}

// addItemToListWithPreview ajoute un élément avec bouton de prévisualisation
func (fe *FileExplorer) addItemToListWithPreview(parent *fyne.Container, item *FileTreeItem) {
	if item == nil {
		return
	}

	// Icône basée sur le type de fichier
	icon := getFileIcon(item.Name, item.IsDir)

	displayName := icon + " " + item.Name

	// Vérifier si sélectionné
	fe.mu.Lock()
	isSelected := fe.selectedItems[item.Path]
	fe.mu.Unlock()

	// Variables locales pour éviter les problèmes de closure
	itemPath := item.Path
	itemIsDir := item.IsDir
	itemRef := item

	// Checkbox
	check := widget.NewCheck(displayName, func(checked bool) {
		fe.mu.Lock()
		fe.selectedItems[itemPath] = checked
		fe.mu.Unlock()
	})
	check.SetChecked(isSelected)

	// Conteneur
	itemContainer := container.NewHBox()
	itemContainer.Add(check)
	itemContainer.Add(layout.NewSpacer())

	if itemIsDir {
		// Bouton ouvrir pour les dossiers
		openBtn := widget.NewButton("Ouvrir", func() {
			fe.currentDir = itemRef
			fe.showDirectoryUI()
		})
		openBtn.Importance = widget.LowImportance
		itemContainer.Add(openBtn)
	}

	parent.Add(itemContainer)
}

func (fe *FileExplorer) showDownloadOptions() {
	selected := fe.getSelectedItems()
	
	if len(selected) == 0 {
		addLog("⚠️ Aucun fichier sélectionné")
		dialog.ShowInformation("Aucune sélection", "Veuillez sélectionner au moins un fichier ou dossier", fe.win)
		return
	}
	
	addLog(fmt.Sprintf("📦 %d éléments sélectionnés", len(selected)))
	
	content := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Télécharger %d élément(s) sélectionné(s)", len(selected))),
		widget.NewSeparator(),
		widget.NewLabel("Choisissez la destination :"),
	)
	
	dlg := dialog.NewCustom("Destination du téléchargement", "Annuler", content, fe.win)
	
	syncDirBtn := widget.NewButton("Dans le dossier de synchronisation", func() {
		dlg.Hide()
		fe.downloadToSyncDir(selected)
	})
	syncDirBtn.Importance = widget.HighImportance
	
	customDirBtn := widget.NewButton("Autre emplacement...", func() {
		dlg.Hide()
		fe.downloadToCustomDir(selected)
	})
	
	content.Add(syncDirBtn)
	content.Add(customDirBtn)
	
	dlg.Show()
}

func (fe *FileExplorer) getSelectedItems() []string {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	
	var selected []string
	
	for path, isSelected := range fe.selectedItems {
		if isSelected {
			// Vérifier que l'item existe toujours dans l'arborescence
			if _, exists := fe.allItems[path]; exists {
				selected = append(selected, path)
			}
		}
	}
	
	return selected
}

func (fe *FileExplorer) downloadToSyncDir(items []string) {
	addLog(fmt.Sprintf("⬇️ Téléchargement de %d éléments vers le dossier de sync...", len(items)))
	go fe.performDownload(items, fe.client.localDir)
	fe.backCallback()
}

func (fe *FileExplorer) downloadToCustomDir(items []string) {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		
		targetDir := uri.Path()
		addLog(fmt.Sprintf("⬇️ Téléchargement de %d éléments vers %s...", len(items), targetDir))
		go fe.performDownload(items, targetDir)
		fe.backCallback()
	}, fe.win)
}

func (fe *FileExplorer) performDownload(items []string, targetDir string) {
	addLog("Debut du telechargement...")

	// Ignorer le tracking pendant le téléchargement
	fe.client.skipTracking = true
	defer func() {
		fe.client.skipTracking = false
	}()

	expandedItems := fe.expandDirectories(items)

	addLog(fmt.Sprintf("%d fichiers/dossiers a telecharger", len(expandedItems)))

	fe.client.downloadActive = true
	fe.client.downloadChan = make(chan FileChange, 100)

	reqMsg := DownloadRequest{
		Type:  "download_request",
		Items: expandedItems,
	}

	err := fe.client.WriteJSONSafe(reqMsg)

	if err != nil {
		addLog(fmt.Sprintf("Erreur envoi requete: %v", err))
		fe.client.downloadActive = false
		return
	}

	downloaded := 0
	timeout := time.After(120 * time.Second)
	lastUpdate := time.Now()
	
	for downloaded < len(expandedItems) {
		select {
		case <-timeout:
			addLog(fmt.Sprintf("Timeout - %d/%d fichiers telecharges", downloaded, len(expandedItems)))
			fe.client.downloadActive = false
			return
			
		case msg := <-fe.client.downloadChan:
			if msg.Origin == "server" {
				fe.saveDownloadedFile(msg, targetDir)
				downloaded++
				
				if time.Since(lastUpdate) > 1*time.Second || downloaded == len(expandedItems) {
					addLog(fmt.Sprintf("Telecharges: %d/%d", downloaded, len(expandedItems)))
					lastUpdate = time.Now()
				}
			}
		}
	}
	
	fe.client.downloadActive = false
	addLog(fmt.Sprintf("Telechargement termine: %d fichiers", downloaded))
}

func (fe *FileExplorer) expandDirectories(items []string) []string {
	expanded := make(map[string]bool)
	
	for _, path := range items {
		fe.expandDirectoryRecursive(path, expanded)
	}
	
	var result []string
	for path := range expanded {
		result = append(result, path)
	}
	
	sort.Strings(result)
	
	return result
}

func (fe *FileExplorer) expandDirectoryRecursive(path string, expanded map[string]bool) {
	expanded[path] = true
	
	fe.mu.Lock()
	item, exists := fe.allItems[path]
	fe.mu.Unlock()
	
	if exists && item.IsDir {
		for _, child := range item.Children {
			fe.expandDirectoryRecursive(child.Path, expanded)
		}
	}
}

func (fe *FileExplorer) saveDownloadedFile(msg FileChange, targetDir string) {
	normalizedPath := filepath.FromSlash(msg.FileName)
	targetPath := filepath.Join(targetDir, normalizedPath)

	switch msg.Op {
	case "mkdir":
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			addLog(fmt.Sprintf("❌ Erreur création dossier %s: %v", msg.FileName, err))
		}

	case "create", "write":
		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			addLog(fmt.Sprintf("❌ Erreur création répertoire: %v", err))
			return
		}

		data, err := base64.StdEncoding.DecodeString(msg.Content)
		if err != nil {
			addLog(fmt.Sprintf("❌ Erreur décodage %s: %v", msg.FileName, err))
			return
		}

		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			addLog(fmt.Sprintf("❌ Erreur écriture %s: %v", msg.FileName, err))
		}
	}
}

// getPathParts retourne les parties du chemin actuel pour le breadcrumb
func (fe *FileExplorer) getPathParts() []string {
	if fe.currentDir == nil || fe.currentDir.Path == "" {
		return []string{}
	}

	parts := strings.Split(fe.currentDir.Path, "/")
	var result []string
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// filterItems filtre les éléments affichés
func (fe *FileExplorer) filterItems(query string) {
	if query != "" {
		addLog(fmt.Sprintf("🔍 Recherche: %s", query))
	}
}

// getFileIcon retourne une icône basée sur l'extension
func getFileIcon(name string, isDir bool) string {
	if isDir {
		return "📁"
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".txt", ".md", ".log":
		return "📄"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".h":
		return "💻"
	case ".html", ".css", ".xml", ".json":
		return "🌐"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg":
		return "🖼️"
	case ".mp3", ".wav", ".flac", ".aac":
		return "🎵"
	case ".mp4", ".avi", ".mkv", ".mov":
		return "🎬"
	case ".zip", ".tar", ".gz", ".rar":
		return "📦"
	case ".pdf":
		return "📕"
	case ".doc", ".docx":
		return "📘"
	case ".xls", ".xlsx":
		return "📊"
	default:
		return "📄"
	}
}