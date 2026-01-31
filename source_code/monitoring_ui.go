package main

import (
	"fmt"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ============================================================================
// MONITORING UI
// ============================================================================

// ShowMonitoringDialog affiche le dialogue de monitoring complet
func ShowMonitoringDialog(window fyne.Window) {
	tabs := container.NewAppTabs(
		container.NewTabItem("📋 Logs", createLogsTab(window)),
		container.NewTabItem("📊 Système", createSystemMetricsTab(window)),
		container.NewTabItem("🌐 Réseau", createNetworkTab(window)),
		container.NewTabItem("👥 Clients", createClientsTab(window)),
		container.NewTabItem("💾 Backup", createBackupTab(window)),
		container.NewTabItem("💬 Chat", createChatTab(window)),
	)
	
	tabs.SetTabLocation(container.TabLocationTop)
	
	dlg := dialog.NewCustom("📊 Monitoring & Outils", "Fermer", tabs, window)
	dlg.Resize(fyne.NewSize(750, 600))
	dlg.Show()
}

// ============================================================================
// LOGS TAB
// ============================================================================

func createLogsTab(window fyne.Window) fyne.CanvasObject {
	logger := GetAdvancedLogger()
	
	// Filtres
	levelSelect := widget.NewSelect([]string{"Tous", "DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}, nil)
	levelSelect.SetSelected("Tous")
	
	categories := append([]string{"Toutes"}, logger.GetCategories()...)
	categorySelect := widget.NewSelect(categories, nil)
	categorySelect.SetSelected("Toutes")
	
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("🔍 Rechercher...")
	
	// Liste des logs
	var currentEntries []*LogEntry
	
	logsList := widget.NewList(
		func() int { return len(currentEntries) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Log entry...")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(currentEntries) {
				return
			}
			entry := currentEntries[len(currentEntries)-1-id]
			item.(*widget.Label).SetText(entry.Format())
		},
	)
	
	refreshLogs := func() {
		filter := &LogFilter{Limit: 500}
		
		if levelSelect.Selected != "Tous" {
			level := ParseLogLevel(levelSelect.Selected)
			filter.MinLevel = &level
		}
		
		if categorySelect.Selected != "Toutes" {
			filter.Categories = []string{categorySelect.Selected}
		}
		
		if searchEntry.Text != "" {
			filter.Keyword = searchEntry.Text
		}
		
		currentEntries = logger.GetEntries(filter)
		logsList.Refresh()
	}
	
	refreshLogs()
	
	levelSelect.OnChanged = func(string) { refreshLogs() }
	categorySelect.OnChanged = func(string) { refreshLogs() }
	searchEntry.OnChanged = func(string) { refreshLogs() }
	
	// Boutons
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), refreshLogs)
	
	exportBtn := widget.NewButton("Export", func() {
		items := []string{"JSON", "CSV", "TXT"}
		selectWidget := widget.NewSelect(items, nil)
		
		dialog.ShowCustomConfirm("Format d'export", "Exporter", "Annuler", selectWidget, func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			
			path := fmt.Sprintf("logs_%s", time.Now().Format("20060102_150405"))
			var err error
			
			switch selectWidget.Selected {
			case "JSON":
				err = logger.ExportToJSON(path+".json", nil)
			case "CSV":
				err = logger.ExportToCSV(path+".csv", nil)
			case "TXT":
				err = logger.ExportToTXT(path+".txt", nil)
			}
			
			if err != nil {
				dialog.ShowError(err, window)
			} else {
				dialog.ShowInformation("Export", "Logs exportés avec succès", window)
			}
		}, window)
	})
	
	clearBtn := widget.NewButton("Effacer", func() {
		dialog.ShowConfirm("Confirmation", "Effacer tous les logs?", func(ok bool) {
			if ok {
				logger.Clear()
				refreshLogs()
			}
		}, window)
	})
	clearBtn.Importance = widget.DangerImportance
	
	// Stats
	stats := logger.GetStatistics()
	statsLabel := widget.NewLabel(fmt.Sprintf("Total: %d logs", stats["total"]))
	
	return container.NewBorder(
		container.NewVBox(
			container.NewHBox(
				widget.NewLabel("Niveau:"), levelSelect,
				widget.NewLabel("Catégorie:"), categorySelect,
				searchEntry,
				refreshBtn,
			),
		),
		container.NewHBox(exportBtn, layout.NewSpacer(), statsLabel, clearBtn),
		nil, nil,
		logsList,
	)
}

// ============================================================================
// SYSTEM METRICS TAB
// ============================================================================

func createSystemMetricsTab(window fyne.Window) fyne.CanvasObject {
	// Labels
	cpuLabel := widget.NewLabel("--")
	memAllocLabel := widget.NewLabel("--")
	memSysLabel := widget.NewLabel("--")
	memHeapLabel := widget.NewLabel("--")
	gcLabel := widget.NewLabel("--")
	goroutinesLabel := widget.NewLabel("--")
	
	// Cache
	dirCacheLabel := widget.NewLabel("--")
	hashCacheLabel := widget.NewLabel("--")
	bufferPoolLabel := widget.NewLabel("--")
	
	updateMetrics := func() {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		
		cpuLabel.SetText(fmt.Sprintf("%d cores", runtime.NumCPU()))
		memAllocLabel.SetText(FormatFileSize(int64(memStats.Alloc)))
		memSysLabel.SetText(FormatFileSize(int64(memStats.Sys)))
		memHeapLabel.SetText(FormatFileSize(int64(memStats.HeapAlloc)))
		gcLabel.SetText(fmt.Sprintf("%d", memStats.NumGC))
		goroutinesLabel.SetText(fmt.Sprintf("%d", runtime.NumGoroutine()))
		
		dirCacheLabel.SetText(fmt.Sprintf("%d entrées", GetDirCache().Size()))
		hashCacheLabel.SetText(fmt.Sprintf("%d entrées", GetHashCache().Size()))
		
		bufStats := GetBufferPool().GetStats()
		bufferPoolLabel.SetText(fmt.Sprintf("Gets: %d, Puts: %d", bufStats.Gets, bufStats.Puts))
	}
	
	// Auto-refresh
	stopUpdate := make(chan bool)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopUpdate:
				return
			case <-ticker.C:
				updateMetrics()
			}
		}
	}()
	
	updateMetrics()
	
	// Boutons
	forceGCBtn := widget.NewButton("Forcer GC", func() {
		runtime.GC()
		updateMetrics()
		addLog("🔄 Garbage collection forcé")
	})
	
	clearCacheBtn := widget.NewButton("Vider caches", func() {
		GetDirCache().InvalidateAll()
		GetHashCache().Clear()
		updateMetrics()
		addLog("🗑️ Caches vidés")
	})
	
	return container.NewVBox(
		widget.NewLabelWithStyle("🖥️ Système", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("CPU:"), cpuLabel,
			widget.NewLabel("Mémoire allouée:"), memAllocLabel,
			widget.NewLabel("Mémoire système:"), memSysLabel,
			widget.NewLabel("Heap:"), memHeapLabel,
			widget.NewLabel("GC runs:"), gcLabel,
			widget.NewLabel("Goroutines:"), goroutinesLabel,
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("📦 Cache", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Dir cache:"), dirCacheLabel,
			widget.NewLabel("Hash cache:"), hashCacheLabel,
			widget.NewLabel("Buffer pool:"), bufferPoolLabel,
		),
		widget.NewSeparator(),
		container.NewHBox(layout.NewSpacer(), forceGCBtn, clearCacheBtn),
	)
}

// ============================================================================
// NETWORK TAB
// ============================================================================

func createNetworkTab(window fyne.Window) fyne.CanvasObject {
	connMgr := GetConnectionManager()
	
	stateLabel := widget.NewLabel("--")
	latencyLabel := widget.NewLabel("--")
	sentLabel := widget.NewLabel("--")
	receivedLabel := widget.NewLabel("--")
	qualityLabel := widget.NewLabel("--")
	uptimeLabel := widget.NewLabel("--")
	
	updateNetwork := func() {
		stats := connMgr.GetStats()
		
		stateLabel.SetText(stats.State.Icon() + " " + stats.State.String())
		
		if stats.Latency > 0 {
			latencyLabel.SetText(stats.Latency.String())
		} else {
			latencyLabel.SetText("N/A")
		}
		
		sentLabel.SetText(FormatFileSize(stats.TotalSent))
		receivedLabel.SetText(FormatFileSize(stats.TotalReceived))
		qualityLabel.SetText(connMgr.GetConnectionQuality().String())
		
		if stats.Uptime > 0 {
			uptimeLabel.SetText(stats.Uptime.Round(time.Second).String())
		} else {
			uptimeLabel.SetText("--")
		}
	}
	
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			updateNetwork()
		}
	}()
	
	updateNetwork()
	
	// Config réseau
	config := GetNetworkConfig()
	
	autoReconnectCheck := widget.NewCheck("Reconnexion automatique", nil)
	autoReconnectCheck.SetChecked(config.AutoReconnect)
	autoReconnectCheck.OnChanged = func(v bool) {
		config.AutoReconnect = v
	}
	
	keepAliveCheck := widget.NewCheck("Keep-alive activé", nil)
	keepAliveCheck.SetChecked(config.KeepAliveEnabled)
	keepAliveCheck.OnChanged = func(v bool) {
		config.KeepAliveEnabled = v
	}
	
	return container.NewVBox(
		widget.NewLabelWithStyle("🌐 État de la connexion", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("État:"), stateLabel,
			widget.NewLabel("Latence:"), latencyLabel,
			widget.NewLabel("Qualité:"), qualityLabel,
			widget.NewLabel("Uptime:"), uptimeLabel,
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("📊 Transferts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2,
			widget.NewLabel("Envoyé:"), sentLabel,
			widget.NewLabel("Reçu:"), receivedLabel,
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("⚙️ Configuration", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		autoReconnectCheck,
		keepAliveCheck,
	)
}

// ============================================================================
// CLIENTS TAB
// ============================================================================

func createClientsTab(window fyne.Window) fyne.CanvasObject {
	clientMgr := GetClientManager()
	
	clients := clientMgr.GetClients()
	
	clientsList := widget.NewList(
		func() int { return len(clients) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("🟢"),
				widget.NewLabel("Client Name"),
				layout.NewSpacer(),
				widget.NewLabel("IP"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(clients) {
				return
			}
			client := clients[id]
			box := item.(*fyne.Container)
			box.Objects[0].(*widget.Label).SetText(client.Status.Icon())
			box.Objects[1].(*widget.Label).SetText(client.Name)
			box.Objects[3].(*widget.Label).SetText(client.IP)
		},
	)
	
	refreshClients := func() {
		clients = clientMgr.GetClients()
		clientsList.Refresh()
	}
	
	// Boutons
	refreshBtn := widget.NewButtonWithIcon("Rafraîchir", theme.ViewRefreshIcon(), refreshClients)
	
	kickBtn := widget.NewButton("Déconnecter", func() {
		if len(clients) == 0 {
			return
		}
		
		var names []string
		for _, c := range clients {
			names = append(names, c.Name)
		}
		
		selectWidget := widget.NewSelect(names, nil)
		
		dialog.ShowCustomConfirm("Déconnecter client", "Déconnecter", "Annuler", selectWidget, func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			
			for _, c := range clients {
				if c.Name == selectWidget.Selected {
					clientMgr.RemoveClient(c.ID)
					refreshClients()
					break
				}
			}
		}, window)
	})
	
	banBtn := widget.NewButton("Bannir", func() {
		if len(clients) == 0 {
			return
		}
		
		var names []string
		for _, c := range clients {
			names = append(names, c.Name)
		}
		
		selectWidget := widget.NewSelect(names, nil)
		reasonEntry := widget.NewEntry()
		reasonEntry.SetPlaceHolder("Raison du ban")
		
		content := container.NewVBox(
			widget.NewLabel("Client:"),
			selectWidget,
			widget.NewLabel("Raison:"),
			reasonEntry,
		)
		
		dialog.ShowCustomConfirm("Bannir client", "Bannir", "Annuler", content, func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			
			for _, c := range clients {
				if c.Name == selectWidget.Selected {
					clientMgr.BanClient(c.ID, reasonEntry.Text, 24*time.Hour)
					refreshClients()
					break
				}
			}
		}, window)
	})
	banBtn.Importance = widget.DangerImportance
	
	countLabel := widget.NewLabel(fmt.Sprintf("%d client(s) connecté(s)", len(clients)))
	
	return container.NewBorder(
		container.NewHBox(
			widget.NewLabelWithStyle("👥 Clients connectés", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			countLabel,
		),
		container.NewHBox(refreshBtn, kickBtn, banBtn),
		nil, nil,
		clientsList,
	)
}

// ============================================================================
// BACKUP TAB
// ============================================================================

func createBackupTab(window fyne.Window) fyne.CanvasObject {
	backupMgr := GetBackupManager()
	
	backups := backupMgr.GetBackups()
	
	backupsList := widget.NewList(
		func() int { return len(backups) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("Backup"),
				layout.NewSpacer(),
				widget.NewLabel("Size"),
				widget.NewLabel("Date"),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(backups) {
				return
			}
			backup := backups[len(backups)-1-id]
			box := item.(*fyne.Container)
			box.Objects[0].(*widget.Label).SetText(backup.ID)
			box.Objects[2].(*widget.Label).SetText(FormatFileSize(backup.Size))
			box.Objects[3].(*widget.Label).SetText(backup.CreatedAt.Format("02/01 15:04"))
		},
	)
	
	refreshBackups := func() {
		backups = backupMgr.GetBackups()
		backupsList.Refresh()
	}
	
	createBackupBtn := widget.NewButtonWithIcon("Creer backup", theme.ContentAddIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			
			go func() {
				_, err := backupMgr.CreateBackup(uri.Path(), BackupFull, "Backup manuel")
				if err != nil {
					dialog.ShowError(err, window)
				} else {
					refreshBackups()
					dialog.ShowInformation("Succes", "Backup cree", window)
				}
			}()
		}, window)
	})
	createBackupBtn.Importance = widget.HighImportance
	
	restoreBtn := widget.NewButton("Restaurer", func() {
		if len(backups) == 0 {
			dialog.ShowInformation("Info", "Aucun backup disponible", window)
			return
		}
		
		var options []string
		for _, b := range backups {
			options = append(options, b.ID)
		}
		
		selectWidget := widget.NewSelect(options, nil)
		
		dialog.ShowCustomConfirm("Restaurer", "Selectionner", "Annuler", selectWidget, func(ok bool) {
			if !ok || selectWidget.Selected == "" {
				return
			}
			
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if err != nil || uri == nil {
					return
				}
				
				if err := backupMgr.RestoreBackup(selectWidget.Selected, uri.Path()); err != nil {
					dialog.ShowError(err, window)
				} else {
					dialog.ShowInformation("Succes", "Backup restaure", window)
				}
			}, window)
		}, window)
	})
	
	return container.NewVBox(
		widget.NewLabelWithStyle("Sauvegardes", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		backupsList,
		container.NewHBox(createBackupBtn, restoreBtn),
	)
}

// ============================================================================
// CHAT TAB
// ============================================================================

func createChatTab(window fyne.Window) fyne.CanvasObject {
	chatMgr := GetChatManager()
	
	messages := chatMgr.GetMessages(50)
	
	chatList := widget.NewList(
		func() int { return len(messages) },
		func() fyne.CanvasObject {
			return widget.NewLabel("Message...")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			if id >= len(messages) {
				return
			}
			msg := messages[id]
			item.(*widget.Label).SetText(fmt.Sprintf("[%s] %s: %s",
				msg.Timestamp.Format("15:04"),
				msg.FromName,
				msg.Content,
			))
		},
	)
	
	refreshChat := func() {
		messages = chatMgr.GetMessages(50)
		chatList.Refresh()
	}
	
	// Input
	messageEntry := widget.NewEntry()
	messageEntry.SetPlaceHolder("Tapez votre message...")
	
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), func() {
		if messageEntry.Text == "" {
			return
		}
		
		chatMgr.SendMessage("host", "Host", messageEntry.Text, MessageText)
		messageEntry.SetText("")
		refreshChat()
	})
	sendBtn.Importance = widget.HighImportance
	
	messageEntry.OnSubmitted = func(s string) {
		sendBtn.OnTapped()
	}
	
	// Auto-refresh
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			refreshChat()
		}
	}()
	
	return container.NewBorder(
		widget.NewLabelWithStyle("💬 Chat", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, sendBtn, messageEntry),
		nil, nil,
		chatList,
	)
}

// ============================================================================
// BOUTON PRINCIPAL
// ============================================================================

// CreateMonitoringButton crée le bouton de monitoring
func CreateMonitoringButton(window fyne.Window) *widget.Button {
	return widget.NewButtonWithIcon("📊 Monitoring", theme.InfoIcon(), func() {
		ShowMonitoringDialog(window)
	})
}
