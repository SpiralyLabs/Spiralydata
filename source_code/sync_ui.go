package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ShowSyncConfigDialog affiche le dialogue de configuration de synchronisation
func ShowSyncConfigDialog(window fyne.Window, config *SyncConfig, onSave func(*SyncConfig)) {
	// Mode de synchronisation
	modeLabel := widget.NewLabelWithStyle("Mode de synchronisation", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	modeOptions := []string{
		"Bidirectionnel",
		"Host → User",
		"User → Host",
		"Miroir",
		"Fusion",
		"Sur demande",
	}
	
	modeSelect := widget.NewSelect(modeOptions, nil)
	modeSelect.SetSelectedIndex(int(config.Mode))
	
	modeDescriptions := map[int]string{
		0: "📡 Les fichiers sont synchronisés dans les deux sens",
		1: "⬇️ Seul le serveur envoie des fichiers aux clients",
		2: "⬆️ Seuls les clients envoient des fichiers au serveur",
		3: "🪞 Le client devient une copie exacte du serveur",
		4: "🔀 Les fichiers sont fusionnés, jamais supprimés",
		5: "🖐️ Synchronisation uniquement sur demande manuelle",
	}
	
	modeDesc := widget.NewLabel(modeDescriptions[int(config.Mode)])
	modeDesc.Wrapping = fyne.TextWrapWord
	
	modeSelect.OnChanged = func(s string) {
		for i, opt := range modeOptions {
			if opt == s {
				modeDesc.SetText(modeDescriptions[i])
				break
			}
		}
	}
	
	modePanel := container.NewVBox(
		modeLabel,
		modeSelect,
		modeDesc,
		widget.NewSeparator(),
	)
	
	// Stratégie de conflits
	conflictLabel := widget.NewLabelWithStyle("Résolution des conflits", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	conflictOptions := []string{
		"Demander à l'utilisateur",
		"Garder le plus récent",
		"Garder la version locale",
		"Garder la version distante",
		"Garder les deux versions",
		"Fusion automatique",
	}
	
	conflictSelect := widget.NewSelect(conflictOptions, nil)
	conflictSelect.SetSelectedIndex(int(config.ConflictStrategy))
	
	conflictPanel := container.NewVBox(
		conflictLabel,
		conflictSelect,
		widget.NewSeparator(),
	)
	
	// Compression
	compressionLabel := widget.NewLabelWithStyle("Compression", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	compressionCheck := widget.NewCheck("Activer la compression", nil)
	compressionCheck.SetChecked(config.CompressionEnabled)
	
	compressionLevelLabel := widget.NewLabel(fmt.Sprintf("Niveau: %d", config.CompressionLevel))
	compressionSlider := widget.NewSlider(1, 9)
	compressionSlider.Value = float64(config.CompressionLevel)
	compressionSlider.OnChanged = func(v float64) {
		compressionLevelLabel.SetText(fmt.Sprintf("Niveau: %d", int(v)))
	}
	
	compressionPanel := container.NewVBox(
		compressionLabel,
		compressionCheck,
		container.NewBorder(nil, nil, widget.NewLabel("Compression:"), compressionLevelLabel, compressionSlider),
		widget.NewSeparator(),
	)
	
	// Limite de bande passante
	bandwidthLabel := widget.NewLabelWithStyle("Limite de bande passante", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	bandwidthOptions := []string{
		"Illimité",
		"100 KB/s",
		"500 KB/s",
		"1 MB/s",
		"5 MB/s",
		"10 MB/s",
	}
	bandwidthValues := []int64{0, 100*1024, 500*1024, 1024*1024, 5*1024*1024, 10*1024*1024}
	
	bandwidthSelect := widget.NewSelect(bandwidthOptions, nil)
	// Trouver l'option correspondante
	for i, v := range bandwidthValues {
		if v == config.BandwidthLimit {
			bandwidthSelect.SetSelectedIndex(i)
			break
		}
	}
	if bandwidthSelect.Selected == "" {
		bandwidthSelect.SetSelectedIndex(0)
	}
	
	bandwidthPanel := container.NewVBox(
		bandwidthLabel,
		bandwidthSelect,
		widget.NewSeparator(),
	)
	
	// Retry
	retryLabel := widget.NewLabelWithStyle("Tentatives de retry", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	retryEntry := widget.NewEntry()
	retryEntry.SetText(strconv.Itoa(config.RetryCount))
	retryEntry.Validator = func(s string) error {
		_, err := strconv.Atoi(s)
		return err
	}
	
	retryDelayEntry := widget.NewEntry()
	retryDelayEntry.SetText(strconv.Itoa(int(config.RetryDelay.Seconds())))
	
	retryPanel := container.NewVBox(
		retryLabel,
		container.NewGridWithColumns(2,
			widget.NewLabel("Nombre de tentatives:"),
			retryEntry,
			widget.NewLabel("Délai entre tentatives (sec):"),
			retryDelayEntry,
		),
		widget.NewSeparator(),
	)
	
	// Planification
	scheduleLabel := widget.NewLabelWithStyle("Synchronisation planifiée", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	
	scheduleCheck := widget.NewCheck("Activer la planification", nil)
	scheduleCheck.SetChecked(config.ScheduleEnabled)
	
	intervalOptions := []string{"1 minute", "5 minutes", "15 minutes", "30 minutes", "1 heure", "6 heures", "12 heures", "24 heures"}
	intervalValues := []time.Duration{
		time.Minute, 5*time.Minute, 15*time.Minute, 30*time.Minute,
		time.Hour, 6*time.Hour, 12*time.Hour, 24*time.Hour,
	}
	
	intervalSelect := widget.NewSelect(intervalOptions, nil)
	for i, v := range intervalValues {
		if v == config.ScheduleInterval {
			intervalSelect.SetSelectedIndex(i)
			break
		}
	}
	if intervalSelect.Selected == "" {
		intervalSelect.SetSelectedIndex(1) // 5 minutes par défaut
	}
	
	schedulePanel := container.NewVBox(
		scheduleLabel,
		scheduleCheck,
		container.NewBorder(nil, nil, widget.NewLabel("Intervalle:"), nil, intervalSelect),
		widget.NewSeparator(),
	)
	
	// Extensions prioritaires
	priorityLabel := widget.NewLabelWithStyle("Extensions prioritaires", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	priorityDesc := widget.NewLabel("Les fichiers avec ces extensions seront synchronisés en premier")
	priorityDesc.TextStyle = fyne.TextStyle{Italic: true}
	
	priorityEntry := widget.NewEntry()
	priorityEntry.SetText(joinExtensions(config.PriorityExtensions))
	priorityEntry.SetPlaceHolder(".txt, .md, .json, .go")
	
	priorityPanel := container.NewVBox(
		priorityLabel,
		priorityDesc,
		priorityEntry,
	)
	
	// Contenu scrollable
	content := container.NewVBox(
		modePanel,
		conflictPanel,
		compressionPanel,
		bandwidthPanel,
		retryPanel,
		schedulePanel,
		priorityPanel,
	)
	
	scrollContent := container.NewVScroll(content)
	scrollContent.SetMinSize(fyne.NewSize(450, 500))
	
	// Boutons
	saveBtn := widget.NewButtonWithIcon("Sauvegarder", theme.DocumentSaveIcon(), func() {
		// Récupérer les valeurs
		for i, opt := range modeOptions {
			if opt == modeSelect.Selected {
				config.Mode = SyncMode(i)
				break
			}
		}
		
		for i, opt := range conflictOptions {
			if opt == conflictSelect.Selected {
				config.ConflictStrategy = ConflictStrategy(i)
				break
			}
		}
		
		config.CompressionEnabled = compressionCheck.Checked
		config.CompressionLevel = int(compressionSlider.Value)
		
		for i, opt := range bandwidthOptions {
			if opt == bandwidthSelect.Selected {
				config.BandwidthLimit = bandwidthValues[i]
				break
			}
		}
		
		if retry, err := strconv.Atoi(retryEntry.Text); err == nil {
			config.RetryCount = retry
		}
		
		if delay, err := strconv.Atoi(retryDelayEntry.Text); err == nil {
			config.RetryDelay = time.Duration(delay) * time.Second
		}
		
		config.ScheduleEnabled = scheduleCheck.Checked
		
		for i, opt := range intervalOptions {
			if opt == intervalSelect.Selected {
				config.ScheduleInterval = intervalValues[i]
				break
			}
		}
		
		config.PriorityExtensions = parseExtensions(priorityEntry.Text)
		
		if onSave != nil {
			onSave(config)
		}
		
		addLog("✅ Configuration de synchronisation sauvegardée")
	})
	saveBtn.Importance = widget.HighImportance
	
	dialogContent := container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), saveBtn),
		nil,
		nil,
		scrollContent,
	)
	
	dialog.ShowCustom("⚙️ Configuration de Synchronisation", "Fermer", dialogContent, window)
}

// ShowConflictDialog affiche le dialogue de résolution de conflit
func ShowConflictDialog(window fyne.Window, conflict *Conflict, onResolve func(ConflictStrategy)) {
	titleLabel := widget.NewLabelWithStyle("⚠️ Conflit détecté", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	
	pathLabel := widget.NewLabel(fmt.Sprintf("Fichier: %s", conflict.Path))
	pathLabel.Wrapping = fyne.TextWrapWord
	
	// Infos version locale
	localInfo := widget.NewLabel(fmt.Sprintf(
		"📁 Version locale:\n"+
		"   Taille: %s\n"+
		"   Modifié: %s\n"+
		"   Hash: %s...",
		FormatFileSize(conflict.LocalVersion.Size),
		conflict.LocalVersion.ModTime.Format("02/01/2006 15:04:05"),
		conflict.LocalVersion.Hash[:16],
	))
	
	// Infos version distante
	remoteInfo := widget.NewLabel(fmt.Sprintf(
		"☁️ Version distante:\n"+
		"   Taille: %s\n"+
		"   Modifié: %s\n"+
		"   Hash: %s...",
		FormatFileSize(conflict.RemoteVersion.Size),
		conflict.RemoteVersion.ModTime.Format("02/01/2006 15:04:05"),
		conflict.RemoteVersion.Hash[:16],
	))
	
	infoPanel := container.NewGridWithColumns(2, localInfo, remoteInfo)
	
	// Boutons de résolution
	keepLocalBtn := widget.NewButton("Garder local", func() {
		onResolve(ConflictKeepLocal)
	})
	
	keepRemoteBtn := widget.NewButton("Garder distant", func() {
		onResolve(ConflictKeepRemote)
	})
	
	keepNewestBtn := widget.NewButton("Plus récent", func() {
		onResolve(ConflictKeepNewest)
	})
	keepNewestBtn.Importance = widget.HighImportance
	
	keepBothBtn := widget.NewButton("Garder les deux", func() {
		onResolve(ConflictKeepBoth)
	})
	
	var mergeBtn *widget.Button
	if IsTextFile(conflict.Path) {
		mergeBtn = widget.NewButton("Fusionner", func() {
			onResolve(ConflictAutoMerge)
		})
	}
	
	buttonsRow1 := container.NewGridWithColumns(2, keepLocalBtn, keepRemoteBtn)
	buttonsRow2 := container.NewGridWithColumns(2, keepNewestBtn, keepBothBtn)
	
	buttons := container.NewVBox(buttonsRow1, buttonsRow2)
	if mergeBtn != nil {
		buttons.Add(container.NewCenter(mergeBtn))
	}
	
	content := container.NewVBox(
		titleLabel,
		widget.NewSeparator(),
		pathLabel,
		widget.NewSeparator(),
		infoPanel,
		widget.NewSeparator(),
		widget.NewLabel("Choisissez une action:"),
		buttons,
	)
	
	dialog.ShowCustom("Résolution de conflit", "Annuler", content, window)
}

// ShowConflictListDialog affiche la liste des conflits
func ShowConflictListDialog(window fyne.Window, cm *ConflictManager) {
	conflicts := cm.GetConflicts()
	
	if len(conflicts) == 0 {
		dialog.ShowInformation("Conflits", "Aucun conflit en attente de résolution.", window)
		return
	}
	
	titleLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("⚠️ %d conflit(s) à résoudre", len(conflicts)),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	
	// Liste des conflits
	list := widget.NewList(
		func() int { return len(conflicts) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.WarningIcon()),
				widget.NewLabel("Fichier en conflit"),
				layout.NewSpacer(),
				widget.NewButton("Résoudre", nil),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			conflict := conflicts[id]
			box := item.(*fyne.Container)
			
			label := box.Objects[1].(*widget.Label)
			label.SetText(conflict.Path)
			
			btn := box.Objects[3].(*widget.Button)
			btn.OnTapped = func() {
				ShowConflictDialog(window, conflict, func(strategy ConflictStrategy) {
					cm.ResolveConflict(conflict.ID, strategy, nil)
					// Rafraîchir la liste
					ShowConflictListDialog(window, cm)
				})
			}
		},
	)
	list.Resize(fyne.NewSize(500, 300))
	
	// Boutons d'action groupée
	resolveAllNewest := widget.NewButton("Tous: Plus récent", func() {
		count := cm.ResolveAll(ConflictKeepNewest)
		dialog.ShowInformation("Résolu", fmt.Sprintf("%d conflit(s) résolu(s)", count), window)
	})
	
	resolveAllLocal := widget.NewButton("Tous: Local", func() {
		count := cm.ResolveAll(ConflictKeepLocal)
		dialog.ShowInformation("Résolu", fmt.Sprintf("%d conflit(s) résolu(s)", count), window)
	})
	
	resolveAllRemote := widget.NewButton("Tous: Distant", func() {
		count := cm.ResolveAll(ConflictKeepRemote)
		dialog.ShowInformation("Résolu", fmt.Sprintf("%d conflit(s) résolu(s)", count), window)
	})
	
	batchButtons := container.NewHBox(
		resolveAllNewest,
		resolveAllLocal,
		resolveAllRemote,
	)
	
	content := container.NewBorder(
		titleLabel,
		batchButtons,
		nil,
		nil,
		list,
	)
	
	dialog.ShowCustom("Gestion des conflits", "Fermer", content, window)
}

// ShowTransferQueueDialog affiche les actions en attente d'envoi au serveur
func ShowTransferQueueDialog(window fyne.Window, queue *TransferQueue) {
	pendingActions := GetPendingActions()
	actions := pendingActions.GetAll()
	
	titleLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("Actions en attente (%d)", len(actions)),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)
	
	infoLabel := widget.NewLabel("Ces actions seront envoyées au serveur lors du prochain 'Envoyer'")
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}
	infoLabel.Wrapping = fyne.TextWrapWord
	
	// Conteneur pour la liste des actions
	actionsContainer := container.NewVBox()
	
	// Fonction pour rafraîchir la liste
	var refreshList func()
	refreshList = func() {
		actionsContainer.RemoveAll()
		currentActions := pendingActions.GetAll()
		titleLabel.SetText(fmt.Sprintf("Actions en attente (%d)", len(currentActions)))
		
		if len(currentActions) == 0 {
			emptyLabel := widget.NewLabel("Aucune action en attente.\n\nLes modifications locales apparaîtront ici\naprès création, modification ou suppression de fichiers.")
			emptyLabel.Alignment = fyne.TextAlignCenter
			emptyLabel.Wrapping = fyne.TextWrapWord
			actionsContainer.Add(container.NewPadded(emptyLabel))
		} else {
			for _, action := range currentActions {
				actionCopy := action
				
				// Icône selon le type d'action et de fichier
				typeIcon := "📄"
				if action.IsDir {
					typeIcon = "📁"
				}
				
				// Ligne d'information
				actionText := fmt.Sprintf("%s %s  %s", action.GetIcon(), typeIcon, action.Path)
				actionLabel := widget.NewLabel(actionText)
				actionLabel.Wrapping = fyne.TextWrapWord
				
				// Taille (seulement pour les fichiers)
				sizeText := ""
				if !action.IsDir && action.Type != ActionDelete {
					sizeText = FormatFileSize(action.Size)
				}
				sizeLabel := widget.NewLabel(sizeText)
				
				// Description de l'action
				descLabel := widget.NewLabel(action.GetDescription())
				descLabel.TextStyle = fyne.TextStyle{Bold: true}
				
				// Bouton supprimer
				removeBtn := widget.NewButton("×", func() {
					pendingActions.Remove(actionCopy.Path)
					refreshList()
				})
				removeBtn.Importance = widget.DangerImportance
				
				// Ligne complète
				row := container.NewBorder(
					nil, nil,
					container.NewHBox(descLabel, sizeLabel),
					removeBtn,
					actionLabel,
				)
				
				actionsContainer.Add(container.NewPadded(row))
				actionsContainer.Add(widget.NewSeparator())
			}
		}
		actionsContainer.Refresh()
	}
	
	refreshList()
	
	// Zone scrollable pour les actions
	actionsScroll := container.NewVScroll(actionsContainer)
	actionsScroll.SetMinSize(fyne.NewSize(600, 400))
	
	// Boutons de contrôle
	clearBtn := widget.NewButton("Vider toutes les actions", func() {
		pendingActions.Clear()
		refreshList()
		addLog("Actions en attente vidées")
	})
	clearBtn.Importance = widget.DangerImportance
	
	refreshBtn := widget.NewButton("Actualiser", func() {
		refreshList()
	})
	refreshBtn.Importance = widget.LowImportance
	
	controlBar := container.NewHBox(
		layout.NewSpacer(),
		refreshBtn,
		clearBtn,
	)
	
	content := container.NewBorder(
		container.NewVBox(titleLabel, infoLabel, widget.NewSeparator(), controlBar, widget.NewSeparator()),
		nil,
		nil,
		nil,
		actionsScroll,
	)
	
	// Dialog plus grand et spacieux
	dialog.ShowCustom("File Transfert - Actions en attente", "Fermer", content, window)
}

// ShowSyncSummary affiche un résumé de la configuration de sync
func ShowSyncSummary(config *SyncConfig) string {
	summary := fmt.Sprintf("Mode: %s", config.GetModeName())
	
	if config.CompressionEnabled {
		summary += fmt.Sprintf(" | Compression: %d", config.CompressionLevel)
	}
	
	if config.BandwidthLimit > 0 {
		summary += fmt.Sprintf(" | Limite: %s/s", FormatFileSize(config.BandwidthLimit))
	}
	
	if config.ScheduleEnabled {
		summary += " | Planifié"
	}
	
	return summary
}

// Utilitaires

func joinExtensions(exts []string) string {
	result := ""
	for i, ext := range exts {
		if i > 0 {
			result += ", "
		}
		result += ext
	}
	return result
}

func parseExtensions(s string) []string {
	parts := []string{}
	for _, part := range splitTrim(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			if !strings.HasPrefix(part, ".") {
				part = "." + part
			}
			parts = append(parts, part)
		}
	}
	return parts
}

func splitTrim(s, sep string) []string {
	parts := []string{}
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// Intégration avec le reste de l'application

// CreateSyncConfigButton crée un bouton pour ouvrir la config sync
func CreateSyncConfigButton(window fyne.Window) *widget.Button {
	return widget.NewButtonWithIcon("⚙️ Config Sync", theme.SettingsIcon(), func() {
		ShowSyncConfigDialog(window, GetSyncConfig(), func(config *SyncConfig) {
			SetSyncConfig(config)
			// Sauvegarder dans le fichier config
			SaveSyncConfigToFile(config)
		})
	})
}

// CreateConflictButton crée un bouton pour voir les conflits
func CreateConflictButton(window fyne.Window) *widget.Button {
	btn := widget.NewButtonWithIcon("⚠️ Conflits", theme.WarningIcon(), func() {
		ShowConflictListDialog(window, GetConflictManager())
	})
	
	// Mettre à jour l'apparence selon les conflits
	go func() {
		for {
			time.Sleep(time.Second)
			if GetConflictManager().HasConflicts() {
				btn.Importance = widget.DangerImportance
			} else {
				btn.Importance = widget.MediumImportance
			}
			btn.Refresh()
		}
	}()
	
	return btn
}

// CreateQueueButton crée un bouton pour voir la file de transfert
func CreateQueueButton(window fyne.Window) *widget.Button {
	return widget.NewButtonWithIcon("📤 File", theme.UploadIcon(), func() {
		ShowTransferQueueDialog(window, GetTransferQueue())
	})
}
