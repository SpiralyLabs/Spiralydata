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

// showUserSetup affiche la page de configuration du mode Utilisateur
// Sans émojis sur les boutons avec texte, avec bouton paramètres
func showUserSetup(win fyne.Window) {
	config, _ := LoadConfig()
	if config == nil {
		config = &AppConfig{DarkTheme: true}
	}

	// Labels sans émojis
	serverLabel := widget.NewLabel("IP du serveur")
	serverLabel.Alignment = fyne.TextAlignLeading
	serverEntry := widget.NewEntry()
	serverEntry.SetPlaceHolder("ex: 192.168.1.100")
	if config.ServerIP != "" {
		serverEntry.SetText(config.ServerIP)
	}

	portLabel := widget.NewLabel("Port")
	portLabel.Alignment = fyne.TextAlignLeading
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("ex: 1234")
	if config.ServerPort != "" {
		portEntry.SetText(config.ServerPort)
	}

	// ID: minimum 6 caractères, pas de maximum
	idLabel := widget.NewLabel("ID du host (6 caractères minimum)")
	idLabel.Alignment = fyne.TextAlignLeading
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("ex: 123456")
	if config.HostID != "" {
		idEntry.SetText(config.HostID)
	}

	syncDirLabel := widget.NewLabel("Dossier de synchronisation")
	syncDirLabel.Alignment = fyne.TextAlignLeading

	syncDirEntry := widget.NewEntry()
	syncDirEntry.SetPlaceHolder("Sélectionnez un dossier...")

	defaultDir := filepath.Join(getExecutableDir(), "Spiralydata")
	if config.SyncDirectory != "" {
		syncDirEntry.SetText(config.SyncDirectory)
	} else {
		syncDirEntry.SetText(defaultDir)
	}

	// Bouton parcourir sans émoji
	browseDirBtn := widget.NewButton("Parcourir", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				syncDirEntry.SetText(uri.Path())
			}
		}, win)
	})

	// Container avec espacement entre l'entry et le bouton
	dirContainer := container.NewBorder(nil, nil, nil, 
		container.NewPadded(browseDirBtn), 
		syncDirEntry,
	)

	// Checkboxes sans émojis
	saveCheck := widget.NewCheck("Sauvegarder la configuration", nil)
	saveCheck.SetChecked(config.SaveConfig)

	autoConnectCheck := widget.NewCheck("Se connecter automatiquement au démarrage", nil)
	autoConnectCheck.SetChecked(config.AutoConnect)

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

	// Bouton connexion sans émoji
	connectBtn := widget.NewButton("Se connecter", func() {
		serverIP := serverEntry.Text
		port := portEntry.Text
		hostID := idEntry.Text
		syncDir := syncDirEntry.Text

		if serverIP == "" || port == "" || hostID == "" {
			addLog("IP, port et ID requis")
			return
		}

		// Validation: minimum 6 caractères pour l'ID
		if len(hostID) < 6 {
			addLog("L'ID doit contenir au moins 6 caractères")
			return
		}

		if syncDir == "" {
			addLog("Dossier de synchronisation requis")
			return
		}

		serverAddr := serverIP + ":" + port

		if saveCheck.Checked {
			newConfig := &AppConfig{
				ServerIP:      serverIP,
				ServerPort:    port,
				HostID:        hostID,
				SyncDirectory: syncDir,
				SaveConfig:    true,
				AutoConnect:   autoConnectCheck.Checked,
				DarkTheme:     GetCurrentTheme() == ThemeDark,
			}

			if err := SaveConfig(newConfig); err != nil {
				addLog(fmt.Sprintf("Erreur sauvegarde config: %v", err))
			} else {
				addLog("Configuration sauvegardée")
			}
		}

		showUserConnecting(win, serverAddr, hostID, syncDir)
	})
	connectBtn.Importance = widget.HighImportance

	// Bouton paramètres
	settingsBtn := widget.NewButton("Paramètres", func() {
		showSettingsWithBack(win, func() {
			showUserSetup(win)
		})
	})

	// Bouton retour sans émoji
	backBtn := widget.NewButton("Retour", func() {
		win.SetContent(container.NewBorder(
			nil,
			statusBar.GetContainer(),
			nil, nil,
			container.NewHSplit(
				createMainMenu(win),
				createLogPanel(),
			),
		))
	})

	buttonsContainer := container.NewVBox(
		connectBtn,
		settingsBtn,
		backBtn,
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Connexion au Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		formContent,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(buttonsContainer)),
		layout.NewSpacer(),
	)

	// Ajouter du padding autour du contenu
	paddedContent := container.NewPadded(content)

	split := container.NewHSplit(
		container.NewVScroll(paddedContent),
		createLogPanel(),
	)
	split.Offset = 0.5

	win.SetContent(container.NewBorder(
		nil,
		statusBar.GetContainer(),
		nil, nil,
		split,
	))
}

// tryAutoConnect tente une connexion automatique si configurée
// Retourne true si une connexion automatique est lancée
func tryAutoConnect(win fyne.Window) bool {
	config, err := LoadConfig()
	if err != nil || !config.AutoConnect || !config.SaveConfig {
		return false
	}

	if config.ServerIP == "" || config.ServerPort == "" || config.HostID == "" {
		return false
	}

	addLog("Connexion automatique...")
	serverAddr := config.ServerIP + ":" + config.ServerPort

	syncDir := config.SyncDirectory
	if syncDir == "" {
		syncDir = filepath.Join(getExecutableDir(), "Spiralydata")
	}

	showUserConnecting(win, serverAddr, config.HostID, syncDir)
	return true
}
