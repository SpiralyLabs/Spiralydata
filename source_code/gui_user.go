package main

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

var (
	shouldAutoConnect = false
)

func showUserSetup(win fyne.Window) {
	// Charger la config existante
	config, _ := LoadConfig()
	
	// Labels et entries
	serverLabel := widget.NewLabel("🌐 IP du serveur")
	serverLabel.Alignment = fyne.TextAlignLeading
	serverEntry := widget.NewEntry()
	serverEntry.SetPlaceHolder("ex: 192.168.1.100")
	if config.ServerIP != "" {
		serverEntry.SetText(config.ServerIP)
	}
	
	portLabel := widget.NewLabel("🔌 Port")
	portLabel.Alignment = fyne.TextAlignLeading
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("ex: 1234")
	if config.ServerPort != "" {
		portEntry.SetText(config.ServerPort)
	}
	
	idLabel := widget.NewLabel("🔑 ID du host")
	idLabel.Alignment = fyne.TextAlignLeading
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("ex: 123456")
	if config.HostID != "" {
		idEntry.SetText(config.HostID)
	}
	
	// Sélection du dossier de synchronisation
	syncDirLabel := widget.NewLabel("📁 Dossier de synchronisation")
	syncDirLabel.Alignment = fyne.TextAlignLeading
	
	syncDirEntry := widget.NewEntry()
	syncDirEntry.SetPlaceHolder("Sélectionnez un dossier...")
	
	// Dossier par défaut
	defaultDir := filepath.Join(getExecutableDir(), "Spiralydata")
	if config.SyncDirectory != "" {
		syncDirEntry.SetText(config.SyncDirectory)
	} else {
		syncDirEntry.SetText(defaultDir)
	}
	
	browseDirBtn := widget.NewButton("Parcourir", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				syncDirEntry.SetText(uri.Path())
			}
		}, win)
	})
	
	dirContainer := container.NewBorder(nil, nil, nil, browseDirBtn, syncDirEntry)
	
	// Checkbox pour sauvegarder la config
	saveCheck := widget.NewCheck("💾 Sauvegarder la configuration", nil)
	saveCheck.SetChecked(config.SaveConfig)
	
	// Checkbox pour connexion automatique
	autoConnectCheck := widget.NewCheck("→ Se connecter automatiquement au démarrage", nil)
	autoConnectCheck.SetChecked(config.AutoConnect)
	
	// Désactiver autoConnect si saveConfig n'est pas cochée
	if !saveCheck.Checked {
		autoConnectCheck.Disable()
	}
	
	saveCheck.OnChanged = func(checked bool) {
		if checked {
			autoConnectCheck.Enable()
		} else {
			autoConnectCheck.SetChecked(false)
			autoConnectCheck.Disable()
		}
	}
	
	// Conteneur du formulaire
	formContent := container.NewVBox(
		serverLabel,
		serverEntry,
		widget.NewSeparator(),
		portLabel,
		portEntry,
		widget.NewSeparator(),
		idLabel,
		idEntry,
		widget.NewSeparator(),
		syncDirLabel,
		dirContainer,
		widget.NewSeparator(),
		saveCheck,
		autoConnectCheck,
	)
	
	// Boutons
	connectBtn := widget.NewButton("Se connecter", func() {
		serverIP := serverEntry.Text
		port := portEntry.Text
		hostID := idEntry.Text
		syncDir := syncDirEntry.Text
		
		if serverIP == "" || port == "" || hostID == "" {
			addLog("❌ IP, port et ID requis")
			return
		}
		
		if syncDir == "" {
			addLog("❌ Dossier de synchronisation requis")
			return
		}
		
		serverAddr := serverIP + ":" + port
		
		// Sauvegarder la config si demandé
		if saveCheck.Checked {
			newConfig := &AppConfig{
				ServerIP:       serverIP,
				ServerPort:     port,
				HostID:         hostID,
				SyncDirectory:  syncDir,
				SaveConfig:     true,
				AutoConnect:    autoConnectCheck.Checked,
			}
			
			if err := SaveConfig(newConfig); err != nil {
				addLog(fmt.Sprintf("⚠️ Erreur sauvegarde config: %v", err))
			} else {
				addLog("💾 Configuration sauvegardée")
			}
		}
		
		showUserConnecting(win, serverAddr, hostID, syncDir)
	})
	connectBtn.Importance = widget.HighImportance
	
	backBtn := widget.NewButton("Retour", func() {
		win.SetContent(container.NewHSplit(
			createMainMenu(win),
			container.NewBorder(
				widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				nil, nil, nil,
				container.NewVScroll(logWidget),
			),
		))
	})
	
	buttonsContainer := container.NewVBox(
		connectBtn,
		backBtn,
	)
	
	content := container.NewVBox(
		widget.NewLabelWithStyle("🔌 Connexion au Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		formContent,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(buttonsContainer)),
		layout.NewSpacer(),
	)
	
	split := container.NewHSplit(
		content,
		container.NewBorder(
			widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil,
			container.NewVScroll(logWidget),
		),
	)
	split.Offset = 0.5
	
	win.SetContent(split)
}

// Fonction pour tenter une connexion automatique au démarrage
func tryAutoConnect(win fyne.Window) bool {
	config, err := LoadConfig()
	if err != nil || !config.AutoConnect || !config.SaveConfig {
		return false
	}
	
	if config.ServerIP == "" || config.ServerPort == "" || config.HostID == "" {
		return false
	}
	
	addLog("⚡ Connexion automatique...")
	serverAddr := config.ServerIP + ":" + config.ServerPort
	
	// Utiliser le dossier de sync configuré ou le défaut
	syncDir := config.SyncDirectory
	if syncDir == "" {
		syncDir = filepath.Join(getExecutableDir(), "Spiralydata")
	}
	
	showUserConnecting(win, serverAddr, config.HostID, syncDir)
	return true
}