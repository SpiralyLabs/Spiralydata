package main

import (
	"fmt"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func showUserConnecting(win fyne.Window, serverAddr, hostID, syncDir string) {
	addLog(fmt.Sprintf("🔌 Connexion à %s...", serverAddr))
	addLog(fmt.Sprintf("📁 Dossier de sync: %s", syncDir))
	
	// IMPORTANT: Créer le dossier AVANT toute tentative de connexion
	if err := os.MkdirAll(syncDir, 0755); err != nil {
		addLog(fmt.Sprintf("❌ Impossible de créer le dossier: %v", err))
		// Retour au menu en cas d'erreur
		win.SetContent(container.NewHSplit(
			createMainMenu(win),
			container.NewBorder(
				widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				nil, nil, nil,
				container.NewVScroll(logWidget),
			),
		))
		return
	}
	addLog(fmt.Sprintf("✅ Dossier créé: %s", syncDir))
	
	infoText := fmt.Sprintf(
		"⏳ CONNEXION EN COURS\n\n"+
			"🌐 Serveur: %s\n"+
			"🔑 ID: %s\n"+
			"📁 Dossier: %s\n\n"+
			"📡 Statut: Connexion...",
		serverAddr, hostID, syncDir,
	)
	
	info := widget.NewLabel(infoText)
	info.Wrapping = fyne.TextWrapWord
	
	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	loadingIndex := 0
	loadingLabel := widget.NewLabel("⠋ Connexion en cours...")
	statusLabel := widget.NewLabel("📡 Statut: Connexion en cours...")
	
	stopAnimation := false
	connectionSuccess := false
	var client *Client
	
	syncBtn := widget.NewButton("SYNC AUTO", nil)
	syncBtn.Importance = widget.DangerImportance
	syncBtn.Hide()
	
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
	
	syncBtn.OnTapped = func() {
		if client != nil {
			client.ToggleAutoSync()
			if client.autoSync {
				syncBtn.SetText("🟢 SYNC AUTO ACTIF")
				syncBtn.Importance = widget.SuccessImportance
				statusLabel.SetText("📡 Statut: Synchronisation Automatique Active")
				
				pullBtn.Disable()
				pushBtn.Disable()
				clearBtn.Disable()
			} else {
				syncBtn.SetText("SYNC AUTO")
				syncBtn.Importance = widget.DangerImportance
				statusLabel.SetText("📡 Statut: Mode Manuel")
				
				pullBtn.Enable()
				pushBtn.Enable()
				clearBtn.Enable()
			}
			syncBtn.Refresh()
			statusLabel.Refresh()
		}
	}
	
	pullBtn.OnTapped = func() {
		if client != nil && !client.autoSync {
			pullBtn.Disable()
			pullBtn.SetText("⏳ Reception...")
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
			pushBtn.SetText("⏳ Envoi...")
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
			clearBtn.SetText("⏳ Suppression...")
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
		widget.NewLabelWithStyle("⚙️ Mode de Synchronisation", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(syncBtn),
			),
		),
	)
	syncContainer.Hide()
	
	manualControlsContainer := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("🎮 Contrôles Manuels", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
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
		widget.NewLabelWithStyle("⚡ Actions Avancées", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		container.NewCenter(
			container.NewMax(
				container.NewPadded(clearBtn),
			),
		),
	)
	manualControlsContainer.Hide()
	
	disconnectBtn := widget.NewButton("DÉCONNECTER", func() {
		addLog("👋 Déconnexion...")
		stopAnimation = true
		if client != nil {
			client.shouldExit = true
			if client.ws != nil {
				client.ws.Close()
			}
		}
		win.SetContent(container.NewHSplit(
			createMainMenu(win),
			container.NewBorder(
				widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				nil, nil, nil,
				container.NewVScroll(logWidget),
			),
		))
	})
	disconnectBtn.Importance = widget.DangerImportance
	
	content := container.NewVBox(
		widget.NewLabelWithStyle("ℹ️ Informations de Connexion", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
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
		content,
		container.NewBorder(
			widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			nil, nil, nil,
			container.NewVScroll(logWidget),
		),
	)
	split.Offset = 0.5
	
	win.SetContent(split)
	
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
	
	go func() {
		addLog(fmt.Sprintf("🔌 Connexion au serveur %s avec l'ID %s", serverAddr, hostID))
		addLog(fmt.Sprintf("📂 Utilisation du dossier: %s", syncDir))
		
		go StartClientGUI(serverAddr, hostID, syncDir, &stopAnimation, &connectionSuccess, loadingLabel, statusLabel, info, &client)
		
		time.Sleep(2 * time.Second)
		if connectionSuccess {
			syncBtn.Show()
			pullBtn.Show()
			pushBtn.Show()
			clearBtn.Show()
			
			pullBtn.Enable()
			pushBtn.Enable()
			clearBtn.Enable()
			
			syncContainer.Show()
			manualControlsContainer.Show()
			content.Refresh()
			
			addLog("🎮 Interface de contrôle prête")
		}
	}()
}