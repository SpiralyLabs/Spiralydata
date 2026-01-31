package main

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// showUserConnecting affiche l'interface de connexion en cours
// Gère l'animation et la tentative de connexion au serveur
func showUserConnecting(win fyne.Window, serverAddr, hostID, syncDir string) {
	addLog(fmt.Sprintf("Connexion à %s...", serverAddr))
	addLog(fmt.Sprintf("Dossier de sync: %s", syncDir))

	// Créer le dossier de synchronisation si nécessaire
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		addLog(fmt.Sprintf("Impossible de créer le dossier: %v", err))
		showUserSetup(win)
		return
	}
	addLog(fmt.Sprintf("Dossier créé: %s", syncDir))

	// Texte d'information
	infoText := fmt.Sprintf(
		"CONNEXION EN COURS\n\n"+
			"Serveur: %s\n"+
			"ID: %s\n"+
			"Dossier: %s\n\n"+
			"Statut: Connexion...",
		serverAddr, hostID, syncDir,
	)

	info := widget.NewLabel(infoText)
	info.Wrapping = fyne.TextWrapWord

	// Animation de chargement
	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	loadingIndex := 0
	loadingLabel := widget.NewLabel("⠋ Connexion en cours...")
	statusLabel := widget.NewLabel("Statut: Connexion en cours...")

	stopAnimation := false
	connectionSuccess := false
	var client *Client

	// Boutons de contrôle (sans émojis sur boutons avec texte)
	syncBtn := widget.NewButton("SYNC AUTO", nil)
	syncBtn.Importance = widget.DangerImportance
	syncBtn.Hide()

	explorerBtn := widget.NewButton("EXPLORATEUR", nil)
	explorerBtn.Importance = widget.MediumImportance
	explorerBtn.Disable()
	explorerBtn.Hide()

	pullBtn := widget.NewButton("RECEVOIR", nil)
	pullBtn.Importance = widget.MediumImportance
	pullBtn.Disable()
	pullBtn.Hide()

	pushBtn := widget.NewButton("ENVOYER", nil)
	pushBtn.Importance = widget.MediumImportance
	pushBtn.Disable()
	pushBtn.Hide()

	clearBtn := widget.NewButton("VIDER LOCAL", nil)
	clearBtn.Importance = widget.MediumImportance
	clearBtn.Disable()
	clearBtn.Hide()

	// Handler pour le bouton sync
	syncBtn.OnTapped = func() {
		if client != nil {
			client.ToggleAutoSync()
			if client.autoSync {
				syncBtn.SetText("SYNC AUTO ACTIF")
				syncBtn.Importance = widget.SuccessImportance
				statusLabel.SetText("Statut: Synchronisation Automatique Active")

				explorerBtn.Disable()
				pullBtn.Disable()
				pushBtn.Disable()
				clearBtn.Disable()
			} else {
				syncBtn.SetText("SYNC AUTO")
				syncBtn.Importance = widget.DangerImportance
				statusLabel.SetText("Statut: Mode Manuel")

				explorerBtn.Enable()
				pullBtn.Enable()
				pushBtn.Enable()
				clearBtn.Enable()
			}
			syncBtn.Refresh()
			statusLabel.Refresh()
		}
	}

	// Handler pour l'explorateur
	explorerBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			explorer := NewFileExplorer(client, win, func() {
				showUserConnected(win, serverAddr, hostID, syncDir, client, &stopAnimation, &connectionSuccess, loadingLabel, statusLabel, info)
			})
			explorer.Show()
		}
	}

	// Handler pour recevoir
	pullBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			pullBtn.Disable()
			pullBtn.SetText("Reception...")
			go func() {
				client.PullAllFromServer()
				time.Sleep(100 * time.Millisecond)
				pullBtn.SetText("RECEVOIR")
				pullBtn.Enable()
				pullBtn.Refresh()
			}()
		}
	}

	// Handler pour envoyer
	pushBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			pushBtn.Disable()
			pushBtn.SetText("Envoi...")
			go func() {
				client.PushLocalChanges()
				time.Sleep(100 * time.Millisecond)
				pushBtn.SetText("ENVOYER")
				pushBtn.Enable()
				pushBtn.Refresh()
			}()
		}
	}

	// Handler pour vider local
	clearBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			clearBtn.Disable()
			clearBtn.SetText("Suppression...")
			go func() {
				client.ClearLocalFiles()
				time.Sleep(100 * time.Millisecond)
				clearBtn.SetText("VIDER LOCAL")
				clearBtn.Enable()
				clearBtn.Refresh()
			}()
		}
	}

	// Conteneur de synchronisation
	syncContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Mode de Synchronisation", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(syncBtn),
			),
		),
	)
	syncContainer.Hide()

	// Boutons de configuration (créés ici pour être disponibles dès le début)
	syncConfigBtn := widget.NewButton("Config Sync", func() {
		ShowSyncConfigDialog(win, GetSyncConfig(), func(config *SyncConfig) {
			SetSyncConfig(config)
			SaveSyncConfigToFile(config)
			addLog("Configuration sync sauvegardée")
		})
	})
	syncConfigBtn.Importance = widget.LowImportance

	conflictBtn := widget.NewButton("Conflits (0)", func() {
		ShowConflictListDialog(win, GetConflictManager())
	})
	conflictBtn.Importance = widget.LowImportance

	queueBtn := widget.NewButton("File transfert", func() {
		ShowTransferQueueDialog(win, GetTransferQueue())
	})
	queueBtn.Importance = widget.LowImportance

	// Bouton télécharger une backup
	backupBtn := widget.NewButton("Telecharger une backup", func() {
		if client == nil {
			addLog("Non connecte au serveur")
			return
		}
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			go func() {
				client.DownloadBackup(uri.Path())
			}()
		}, win)
	})
	backupBtn.Importance = widget.LowImportance

	// Mise à jour du compteur de conflits en arrière-plan
	go func() {
		for !stopAnimation {
			time.Sleep(2 * time.Second)
			count := GetConflictManager().ConflictCount()
			if count > 0 {
				conflictBtn.SetText(fmt.Sprintf("Conflits (%d)", count))
				conflictBtn.Importance = widget.DangerImportance
			} else {
				conflictBtn.SetText("Conflits (0)")
				conflictBtn.Importance = widget.LowImportance
			}
			conflictBtn.Refresh()
		}
	}()

	// Conteneur des contrôles manuels
	manualControlsContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Contrôles Manuels", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(explorerBtn),
			),
		),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(pullBtn),
			),
		),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(pushBtn),
			),
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Actions", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(clearBtn),
			),
		),
		container.NewCenter(
			container.NewHBox(
				syncConfigBtn,
				conflictBtn,
				queueBtn,
				backupBtn,
			),
		),
	)
	manualControlsContainer.Hide()

	// Bouton déconnexion - retourne à la page User Setup (pas fermer l'app!)
	disconnectBtn := widget.NewButton("Se deconnecter", func() {
		defer func() {
			if r := recover(); r != nil {
				addLog(fmt.Sprintf("Erreur lors de la deconnexion: %v", r))
			}
			// Toujours revenir au menu même en cas d'erreur
			statusBar.SetConnected(false, "")
			showUserSetup(win)
		}()
		
		addLog("Deconnexion...")
		stopAnimation = true
		if client != nil {
			client.shouldExit = true
			if client.ws != nil {
				client.ws.Close()
			}
			client.cleanup()
			client = nil
		}
	})
	disconnectBtn.Importance = widget.DangerImportance

	// Contenu principal
	content := container.NewVBox(
		widget.NewLabelWithStyle("Informations de Connexion", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		loadingLabel,
		statusLabel,
		syncContainer,
		manualControlsContainer,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(disconnectBtn)),
	)

	split := container.NewHSplit(
		container.NewVScroll(content),
		createLogPanel(),
	)
	split.Offset = 0.5

	win.SetContent(container.NewBorder(
		nil,
		statusBar.GetContainer(),
		nil, nil,
		split,
	))

	// Animation de chargement
	go func() {
		for !stopAnimation && !connectionSuccess {
			time.Sleep(100 * time.Millisecond)
			if !stopAnimation && !connectionSuccess {
				char := loadingChars[loadingIndex%len(loadingChars)]
				loadingLabel.SetText(fmt.Sprintf("%s Connexion en cours...", char))
				loadingLabel.Refresh()
				loadingIndex++
			}
		}
	}()

	// Connexion au serveur
	go func() {
		addLog(fmt.Sprintf("Connexion au serveur %s avec l'ID %s", serverAddr, hostID))
		addLog(fmt.Sprintf("Utilisation du dossier: %s", syncDir))

		go StartClientGUI(serverAddr, hostID, syncDir, &stopAnimation, &connectionSuccess, loadingLabel, statusLabel, info, &client)

		time.Sleep(2 * time.Second)
		if connectionSuccess {
			syncBtn.Show()
			explorerBtn.Show()
			pullBtn.Show()
			pushBtn.Show()
			clearBtn.Show()

			explorerBtn.Enable()
			pullBtn.Enable()
			pushBtn.Enable()
			clearBtn.Enable()

			syncContainer.Show()
			manualControlsContainer.Show()
			content.Refresh()

			addLog("Interface de contrôle prête")
		}
	}()
}

// showUserConnected affiche l'interface utilisateur connecté
// Avec tous les contrôles de synchronisation
func showUserConnected(win fyne.Window, serverAddr, hostID, syncDir string, client *Client, stopAnimation *bool, connectionSuccess *bool, loadingLabel, statusLabel, info *widget.Label) {
	// Boutons sans émojis
	syncBtn := widget.NewButton("SYNC AUTO", nil)
	syncBtn.Importance = widget.DangerImportance

	explorerBtn := widget.NewButton("EXPLORATEUR", nil)
	explorerBtn.Importance = widget.MediumImportance

	pullBtn := widget.NewButton("RECEVOIR", nil)
	pullBtn.Importance = widget.MediumImportance

	pushBtn := widget.NewButton("ENVOYER", nil)
	pushBtn.Importance = widget.MediumImportance

	clearBtn := widget.NewButton("VIDER LOCAL", nil)
	clearBtn.Importance = widget.MediumImportance

	if client.autoSync {
		syncBtn.SetText("SYNC AUTO ACTIF")
		syncBtn.Importance = widget.SuccessImportance
		explorerBtn.Disable()
		pullBtn.Disable()
		pushBtn.Disable()
		clearBtn.Disable()
	}

	syncBtn.OnTapped = func() {
		if client != nil {
			client.ToggleAutoSync()
			if client.autoSync {
				syncBtn.SetText("SYNC AUTO ACTIF")
				syncBtn.Importance = widget.SuccessImportance
				statusLabel.SetText("Statut: Synchronisation Automatique Active")

				explorerBtn.Disable()
				pullBtn.Disable()
				pushBtn.Disable()
				clearBtn.Disable()
			} else {
				syncBtn.SetText("SYNC AUTO")
				syncBtn.Importance = widget.DangerImportance
				statusLabel.SetText("Statut: Mode Manuel")

				explorerBtn.Enable()
				pullBtn.Enable()
				pushBtn.Enable()
				clearBtn.Enable()
			}
			syncBtn.Refresh()
			statusLabel.Refresh()
		}
	}

	explorerBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			explorer := NewFileExplorer(client, win, func() {
				showUserConnected(win, serverAddr, hostID, syncDir, client, stopAnimation, connectionSuccess, loadingLabel, statusLabel, info)
			})
			explorer.Show()
		}
	}

	pullBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			pullBtn.Disable()
			pullBtn.SetText("Reception...")
			go func() {
				client.PullAllFromServer()
				time.Sleep(100 * time.Millisecond)
				pullBtn.SetText("RECEVOIR")
				pullBtn.Enable()
				pullBtn.Refresh()
			}()
		}
	}

	pushBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			pushBtn.Disable()
			pushBtn.SetText("Envoi...")
			go func() {
				client.PushLocalChanges()
				time.Sleep(100 * time.Millisecond)
				pushBtn.SetText("ENVOYER")
				pushBtn.Enable()
				pushBtn.Refresh()
			}()
		}
	}

	clearBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			clearBtn.Disable()
			clearBtn.SetText("Suppression...")
			go func() {
				client.ClearLocalFiles()
				time.Sleep(100 * time.Millisecond)
				clearBtn.SetText("VIDER LOCAL")
				clearBtn.Enable()
				clearBtn.Refresh()
			}()
		}
	}

	syncContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Mode de Synchronisation", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(syncBtn),
			),
		),
	)

	// Bouton configuration sync (sans émoji)
	syncConfigBtn := widget.NewButton("Config Sync", func() {
		ShowSyncConfigDialog(win, GetSyncConfig(), func(config *SyncConfig) {
			SetSyncConfig(config)
			SaveSyncConfigToFile(config)
			addLog("Configuration sync sauvegardée")
		})
	})
	syncConfigBtn.Importance = widget.LowImportance

	// Bouton conflits
	conflictBtn := widget.NewButton("Conflits (0)", func() {
		ShowConflictListDialog(win, GetConflictManager())
	})
	conflictBtn.Importance = widget.LowImportance

	// Mettre à jour le compteur de conflits
	go func() {
		for {
			time.Sleep(2 * time.Second)
			count := GetConflictManager().ConflictCount()
			if count > 0 {
				conflictBtn.SetText(fmt.Sprintf("Conflits (%d)", count))
				conflictBtn.Importance = widget.DangerImportance
			} else {
				conflictBtn.SetText("Conflits (0)")
				conflictBtn.Importance = widget.LowImportance
			}
			conflictBtn.Refresh()
		}
	}()

	// Bouton file de transfert - affiche les actions en attente
	queueBtn := widget.NewButton("File transfert", func() {
		ShowTransferQueueDialog(win, GetTransferQueue())
	})
	queueBtn.Importance = widget.LowImportance

	// Bouton télécharger une backup
	backupBtn := widget.NewButton("Telecharger une backup", func() {
		if client == nil {
			addLog("Non connecte au serveur")
			return
		}
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			go func() {
				client.DownloadBackup(uri.Path())
			}()
		}, win)
	})
	backupBtn.Importance = widget.LowImportance

	manualControlsContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Controles Manuels", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(explorerBtn),
			),
		),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(pullBtn),
			),
		),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(pushBtn),
			),
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Actions", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(clearBtn),
			),
		),
		container.NewCenter(
			container.NewHBox(
				syncConfigBtn,
				conflictBtn,
				queueBtn,
				backupBtn,
			),
		),
	)

	// Bouton déconnexion qui retourne à showUserSetup
	disconnectBtn := widget.NewButton("Se deconnecter", func() {
		defer func() {
			if r := recover(); r != nil {
				addLog(fmt.Sprintf("Erreur lors de la deconnexion: %v", r))
			}
			// Toujours revenir au menu même en cas d'erreur
			statusBar.SetConnected(false, "")
			showUserSetup(win)
		}()
		
		addLog("Deconnexion...")
		*stopAnimation = true
		if client != nil {
			client.shouldExit = true
			if client.ws != nil {
				client.ws.Close()
			}
			client.cleanup()
		}
	})
	disconnectBtn.Importance = widget.DangerImportance

	content := container.NewVBox(
		widget.NewLabelWithStyle("Informations de Connexion", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		loadingLabel,
		statusLabel,
		syncContainer,
		manualControlsContainer,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(disconnectBtn)),
	)

	split := container.NewHSplit(
		container.NewVScroll(content),
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
