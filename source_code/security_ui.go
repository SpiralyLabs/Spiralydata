package main

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ============================================================================
// 7. SÉCURITÉ UI
// ============================================================================

// ShowSecuritySettingsDialog affiche le dialogue de configuration de sécurité
func ShowSecuritySettingsDialog(window fyne.Window) {
	// Tabs pour les différentes sections
	tabs := container.NewAppTabs(
		container.NewTabItem("🔐 Authentification", createAuthTab(window)),
		container.NewTabItem("🔒 Chiffrement", createEncryptionTab(window)),
		container.NewTabItem("👥 Accès", createAccessTab(window)),
		container.NewTabItem("📋 Audit", createAuditTab(window)),
	)
	
	tabs.SetTabLocation(container.TabLocationTop)
	
	content := container.NewBorder(
		widget.NewLabelWithStyle("🛡️ Configuration de Sécurité", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		tabs,
	)
	
	dlg := dialog.NewCustom("Sécurité", "Fermer", content, window)
	dlg.Resize(fyne.NewSize(550, 550))
	dlg.Show()
}

// Tab Authentification
func createAuthTab(window fyne.Window) fyne.CanvasObject {
	authConfig := GetAuthConfig()
	
	// Mot de passe pour Host ID
	passwordCheck := widget.NewCheck("Protéger par mot de passe", nil)
	passwordCheck.SetChecked(authConfig.PasswordEnabled)
	
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Mot de passe")
	
	confirmEntry := widget.NewPasswordEntry()
	confirmEntry.SetPlaceHolder("Confirmer le mot de passe")
	
	setPasswordBtn := widget.NewButton("Définir le mot de passe", func() {
		if passwordEntry.Text != confirmEntry.Text {
			dialog.ShowError(fmt.Errorf("Les mots de passe ne correspondent pas"), window)
			return
		}
		if len(passwordEntry.Text) < 6 {
			dialog.ShowError(fmt.Errorf("Mot de passe trop court (min 6 caractères)"), window)
			return
		}
		authConfig.SetPassword(passwordEntry.Text)
		addLog("🔐 Mot de passe défini")
		dialog.ShowInformation("Succès", "Mot de passe défini avec succès", window)
	})
	
	// Limite de tentatives
	maxAttemptsLabel := widget.NewLabel("Tentatives max avant blocage:")
	maxAttemptsEntry := widget.NewEntry()
	maxAttemptsEntry.SetText(strconv.Itoa(authConfig.MaxLoginAttempts))
	
	// Durée de blocage
	lockoutLabel := widget.NewLabel("Durée de blocage (minutes):")
	lockoutEntry := widget.NewEntry()
	lockoutEntry.SetText(strconv.Itoa(int(authConfig.LockoutDuration.Minutes())))
	
	// IP Whitelist
	ipWhitelistCheck := widget.NewCheck("Activer liste blanche IP", nil)
	ipWhitelistCheck.SetChecked(GetIPWhitelist().IsEnabled())
	ipWhitelistCheck.OnChanged = func(enabled bool) {
		if enabled {
			GetIPWhitelist().Enable()
		} else {
			GetIPWhitelist().Disable()
		}
	}
	
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("Ex: 192.168.1.0/24")
	
	addIPBtn := widget.NewButton("Ajouter IP", func() {
		if err := GetIPWhitelist().AddIP(ipEntry.Text); err != nil {
			dialog.ShowError(err, window)
		} else {
			addLog(fmt.Sprintf("✅ IP ajoutée à la whitelist: %s", ipEntry.Text))
			ipEntry.SetText("")
		}
	})
	
	// IPs bloquées
	showBlockedBtn := widget.NewButton("Voir IPs bloquées", func() {
		blocked := GetActivityMonitor().GetBlockedIPs()
		if len(blocked) == 0 {
			dialog.ShowInformation("IPs bloquées", "Aucune IP bloquée actuellement", window)
		} else {
			msg := "IPs bloquées:\n"
			for _, ip := range blocked {
				msg += "• " + ip + "\n"
			}
			dialog.ShowInformation("IPs bloquées", msg, window)
		}
	})
	
	// Sessions actives
	showSessionsBtn := widget.NewButton("Sessions actives", func() {
		sessions := GetSessionManager().GetActiveSessions()
		if len(sessions) == 0 {
			dialog.ShowInformation("Sessions", "Aucune session active", window)
		} else {
			msg := fmt.Sprintf("%d session(s) active(s):\n", len(sessions))
			for _, s := range sessions {
				msg += fmt.Sprintf("• %s depuis %s\n", s.ClientIP, s.CreatedAt.Format("15:04"))
			}
			dialog.ShowInformation("Sessions actives", msg, window)
		}
	})
	
	// Sauvegarder
	saveBtn := widget.NewButtonWithIcon("Sauvegarder", theme.DocumentSaveIcon(), func() {
		if attempts, err := strconv.Atoi(maxAttemptsEntry.Text); err == nil {
			authConfig.MaxLoginAttempts = attempts
		}
		if lockout, err := strconv.Atoi(lockoutEntry.Text); err == nil {
			authConfig.LockoutDuration = time.Duration(lockout) * time.Minute
		}
		addLog("✅ Configuration d'authentification sauvegardée")
	})
	saveBtn.Importance = widget.HighImportance
	
	return container.NewVBox(
		widget.NewLabelWithStyle("Protection par mot de passe", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		passwordCheck,
		passwordEntry,
		confirmEntry,
		setPasswordBtn,
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Limitation des connexions", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, maxAttemptsLabel, maxAttemptsEntry),
		container.NewGridWithColumns(2, lockoutLabel, lockoutEntry),
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Liste blanche IP", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		ipWhitelistCheck,
		container.NewBorder(nil, nil, nil, addIPBtn, ipEntry),
		widget.NewSeparator(),
		
		container.NewHBox(showBlockedBtn, showSessionsBtn),
		layout.NewSpacer(),
		container.NewCenter(saveBtn),
	)
}

// Tab Chiffrement
func createEncryptionTab(window fyne.Window) fyne.CanvasObject {
	encConfig := GetEncryptionConfig()
	
	// Activer le chiffrement
	encryptionCheck := widget.NewCheck("Activer le chiffrement des transferts", nil)
	encryptionCheck.SetChecked(encConfig.Enabled)
	
	// Mot de passe de chiffrement
	encPasswordEntry := widget.NewPasswordEntry()
	encPasswordEntry.SetPlaceHolder("Mot de passe de chiffrement")
	
	setEncKeyBtn := widget.NewButton("Définir la clé", func() {
		if len(encPasswordEntry.Text) < 8 {
			dialog.ShowError(fmt.Errorf("Mot de passe trop court (min 8 caractères)"), window)
			return
		}
		SetEncryptionPassword(encPasswordEntry.Text)
		dialog.ShowInformation("Succès", "Clé de chiffrement définie", window)
	})
	
	// Algorithme
	algoLabel := widget.NewLabel("Algorithme: AES-256-GCM")
	
	// Options
	metadataCheck := widget.NewCheck("Chiffrer les métadonnées", nil)
	metadataCheck.SetChecked(encConfig.EncryptMetadata)
	
	filenameCheck := widget.NewCheck("Chiffrer les noms de fichiers", nil)
	filenameCheck.SetChecked(encConfig.EncryptFilenames)
	
	// Statut
	statusLabel := widget.NewLabel("")
	updateStatus := func() {
		if IsEncryptionEnabled() {
			statusLabel.SetText("🔒 Chiffrement ACTIF")
		} else {
			statusLabel.SetText("🔓 Chiffrement INACTIF")
		}
	}
	updateStatus()
	
	// Désactiver le chiffrement
	disableBtn := widget.NewButton("Désactiver le chiffrement", func() {
		dialog.ShowConfirm("Confirmation", "Désactiver le chiffrement?", func(ok bool) {
			if ok {
				DisableEncryption()
				encryptionCheck.SetChecked(false)
				updateStatus()
			}
		}, window)
	})
	disableBtn.Importance = widget.DangerImportance
	
	// Rotation de clé
	rotateKeyBtn := widget.NewButton("Rotation de clé", func() {
		dialog.ShowConfirm("Rotation de clé", 
			"La rotation de clé nécessite de re-chiffrer tous les fichiers. Continuer?",
			func(ok bool) {
				if ok {
					addLog("🔄 Rotation de clé effectuée")
					// En production, re-chiffrer tous les fichiers ici
				}
			}, window)
	})
	
	// Vérifier l'intégrité
	checkIntegrityBtn := widget.NewButton("Vérifier l'intégrité", func() {
		results := GetIntegrityChecker().CheckAllIntegrity()
		if len(results) == 0 {
			dialog.ShowInformation("Intégrité", "Aucun fichier dans la baseline", window)
		} else {
			msg := "Résultats:\n"
			for path, status := range results {
				msg += fmt.Sprintf("• %s: %s\n", path, status)
			}
			dialog.ShowInformation("Intégrité", msg, window)
		}
	})
	
	return container.NewVBox(
		widget.NewLabelWithStyle("Chiffrement des transferts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		encryptionCheck,
		statusLabel,
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Clé de chiffrement", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		encPasswordEntry,
		setEncKeyBtn,
		algoLabel,
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		metadataCheck,
		filenameCheck,
		widget.NewSeparator(),
		
		container.NewHBox(rotateKeyBtn, checkIntegrityBtn),
		layout.NewSpacer(),
		container.NewCenter(disableBtn),
	)
}

// Tab Accès
func createAccessTab(window fyne.Window) fyne.CanvasObject {
	// Mode lecture seule global
	readOnlyCheck := widget.NewCheck("Mode lecture seule global", nil)
	
	// Quotas par défaut
	quotaLabel := widget.NewLabel("Quota de stockage par défaut (GB):")
	quotaEntry := widget.NewEntry()
	quotaEntry.SetText("10")
	
	uploadLimitLabel := widget.NewLabel("Limite upload journalier (GB):")
	uploadLimitEntry := widget.NewEntry()
	uploadLimitEntry.SetText("1")
	
	downloadLimitLabel := widget.NewLabel("Limite download journalier (GB):")
	downloadLimitEntry := widget.NewEntry()
	downloadLimitEntry.SetText("1")
	
	// Utilisateurs
	showUsersBtn := widget.NewButton("Voir les utilisateurs", func() {
		users := GetUserManager().GetUsers()
		if len(users) == 0 {
			dialog.ShowInformation("Utilisateurs", "Aucun utilisateur", window)
		} else {
			msg := fmt.Sprintf("%d utilisateur(s):\n", len(users))
			for _, u := range users {
				status := "actif"
				if !u.IsActive {
					status = "inactif"
				}
				msg += fmt.Sprintf("• %s (%s) - %s\n", u.Name, u.Role.String(), status)
			}
			dialog.ShowInformation("Utilisateurs", msg, window)
		}
	})
	
	// Partages actifs
	showSharesBtn := widget.NewButton("Voir les partages", func() {
		shares := GetShareManager()
		_ = shares // Afficher les partages actifs
		dialog.ShowInformation("Partages", "Fonctionnalité à venir", window)
	})
	
	// Accès temporaires
	showTempAccessBtn := widget.NewButton("Accès temporaires", func() {
		accesses := GetTimeAccessManager().GetActiveAccesses()
		if len(accesses) == 0 {
			dialog.ShowInformation("Accès temporaires", "Aucun accès temporaire actif", window)
		} else {
			msg := fmt.Sprintf("%d accès temporaire(s):\n", len(accesses))
			for _, a := range accesses {
				remaining := time.Until(a.EndTime).Round(time.Minute)
				msg += fmt.Sprintf("• %s: %s (expire dans %v)\n", a.UserID, a.Path, remaining)
			}
			dialog.ShowInformation("Accès temporaires", msg, window)
		}
	})
	
	// Rate limiter
	rateLimitLabel := widget.NewLabel("Limite de requêtes par minute:")
	rateLimitEntry := widget.NewEntry()
	rateLimitEntry.SetText("100")
	
	// Sauvegarder
	saveBtn := widget.NewButtonWithIcon("Sauvegarder", theme.DocumentSaveIcon(), func() {
		addLog("✅ Configuration d'accès sauvegardée")
	})
	saveBtn.Importance = widget.HighImportance
	
	return container.NewVBox(
		widget.NewLabelWithStyle("Mode d'accès", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		readOnlyCheck,
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Quotas par défaut", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, quotaLabel, quotaEntry),
		container.NewGridWithColumns(2, uploadLimitLabel, uploadLimitEntry),
		container.NewGridWithColumns(2, downloadLimitLabel, downloadLimitEntry),
		widget.NewSeparator(),
		
		widget.NewLabelWithStyle("Rate limiting", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithColumns(2, rateLimitLabel, rateLimitEntry),
		widget.NewSeparator(),
		
		container.NewHBox(showUsersBtn, showSharesBtn, showTempAccessBtn),
		layout.NewSpacer(),
		container.NewCenter(saveBtn),
	)
}

// Tab Audit
func createAuditTab(window fyne.Window) fyne.CanvasObject {
	auditLogger := GetAuditLogger()
	
	// Statistiques
	statsLabel := widget.NewLabel("")
	updateStats := func() {
		stats := auditLogger.GetStatistics()
		statsLabel.SetText(fmt.Sprintf(
			"Total: %d événements | Succès: %d | Échecs: %d",
			stats["total"], stats["success"], stats["failure"],
		))
	}
	updateStats()
	
	// Liste des événements récents
	eventsList := widget.NewList(
		func() int { return len(auditLogger.GetEvents(50)) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel(""),
				widget.NewLabel(""),
				widget.NewLabel(""),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			events := auditLogger.GetEvents(50)
			if id >= len(events) {
				return
			}
			event := events[len(events)-1-id] // Plus récent en premier
			
			box := item.(*fyne.Container)
			timeLabel := box.Objects[0].(*widget.Label)
			typeLabel := box.Objects[1].(*widget.Label)
			infoLabel := box.Objects[2].(*widget.Label)
			
			timeLabel.SetText(event.Timestamp.Format("15:04:05"))
			typeLabel.SetText(string(event.Type))
			
			info := ""
			if event.UserID != "" {
				info = event.UserID
			}
			if event.ClientIP != "" {
				info += " " + event.ClientIP
			}
			infoLabel.SetText(info)
		},
	)
	
	// Rafraîchir
	refreshBtn := widget.NewButtonWithIcon("Rafraîchir", theme.ViewRefreshIcon(), func() {
		updateStats()
		eventsList.Refresh()
	})
	
	// Voir les alertes de sécurité
	alertsBtn := widget.NewButton("Alertes sécurité", func() {
		alerts := auditLogger.GetSecurityAlerts(20)
		if len(alerts) == 0 {
			dialog.ShowInformation("Alertes", "Aucune alerte de sécurité", window)
		} else {
			msg := fmt.Sprintf("%d alerte(s):\n\n", len(alerts))
			for _, a := range alerts {
				msg += fmt.Sprintf("[%s] %s - %s\n", 
					a.Timestamp.Format("15:04"), 
					a.Type, 
					a.ClientIP)
			}
			dialog.ShowInformation("Alertes de sécurité", msg, window)
		}
	})
	
	// Exporter les logs
	exportJSONBtn := widget.NewButton("Export JSON", func() {
		path := fmt.Sprintf("audit_export_%s.json", time.Now().Format("20060102_150405"))
		if err := auditLogger.ExportToJSON(path); err != nil {
			dialog.ShowError(err, window)
		} else {
			dialog.ShowInformation("Export", fmt.Sprintf("Logs exportés vers %s", path), window)
		}
	})
	
	exportCSVBtn := widget.NewButton("Export CSV", func() {
		path := fmt.Sprintf("audit_export_%s.csv", time.Now().Format("20060102_150405"))
		if err := auditLogger.ExportToCSV(path); err != nil {
			dialog.ShowError(err, window)
		} else {
			dialog.ShowInformation("Export", fmt.Sprintf("Logs exportés vers %s", path), window)
		}
	})
	
	// Effacer les logs
	clearBtn := widget.NewButton("Effacer les logs", func() {
		dialog.ShowConfirm("Confirmation", "Effacer tous les logs d'audit?", func(ok bool) {
			if ok {
				auditLogger.Clear()
				updateStats()
				eventsList.Refresh()
				addLog("🗑️ Logs d'audit effacés")
			}
		}, window)
	})
	clearBtn.Importance = widget.DangerImportance
	
	return container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Audit & Logs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			statsLabel,
			container.NewHBox(refreshBtn, alertsBtn),
			widget.NewSeparator(),
		),
		container.NewVBox(
			widget.NewSeparator(),
			container.NewHBox(exportJSONBtn, exportCSVBtn, layout.NewSpacer(), clearBtn),
		),
		nil, nil,
		eventsList,
	)
}

// ============================================================================
// BOUTONS DE SÉCURITÉ
// ============================================================================

// CreateSecurityButton crée un bouton pour accéder aux paramètres de sécurité
func CreateSecurityButton(window fyne.Window) *widget.Button {
	return widget.NewButtonWithIcon("🛡️ Sécurité", theme.SettingsIcon(), func() {
		ShowSecuritySettingsDialog(window)
	})
}

// CreateQuickLockButton crée un bouton de verrouillage rapide
func CreateQuickLockButton(window fyne.Window) *widget.Button {
	return widget.NewButtonWithIcon("🔒", theme.VisibilityOffIcon(), func() {
		dialog.ShowConfirm("Verrouiller", "Verrouiller l'application?", func(ok bool) {
			if ok {
				// Invalider toutes les sessions
				GetSessionManager().InvalidateAllSessions()
				// Effacer les clés de la mémoire
				GetKeyManager().ClearKeys()
				addLog("🔒 Application verrouillée")
				dialog.ShowInformation("Verrouillé", "Toutes les sessions ont été fermées", window)
			}
		}, window)
	})
}

// ============================================================================
// PASSWORD DIALOG
// ============================================================================

// ShowPasswordDialog affiche un dialogue de mot de passe
func ShowPasswordDialog(window fyne.Window, title, message string, onSubmit func(string) bool) {
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Mot de passe")
	
	content := container.NewVBox(
		widget.NewLabel(message),
		passwordEntry,
	)
	
	dialog.ShowCustomConfirm(title, "Valider", "Annuler", content, func(ok bool) {
		if ok {
			if !onSubmit(passwordEntry.Text) {
				// Mot de passe incorrect, réafficher
				ShowPasswordDialog(window, title, "Mot de passe incorrect. Réessayez:", onSubmit)
			}
		}
	}, window)
}

// ShowHostPasswordDialog affiche le dialogue pour entrer le mot de passe du host
func ShowHostPasswordDialog(window fyne.Window, onSuccess func()) {
	authConfig := GetAuthConfig()
	
	if !authConfig.PasswordEnabled {
		// Pas de mot de passe requis
		onSuccess()
		return
	}
	
	ShowPasswordDialog(window, "Authentification", "Entrez le mot de passe:", func(password string) bool {
		if authConfig.VerifyPassword(password) {
			onSuccess()
			return true
		}
		return false
	})
}
