package main

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Variables globales pour la gestion de l'interface
var (
	logWidget       *widget.Entry      // Widget d'affichage des logs
	logScroll       *container.Scroll  // Conteneur scrollable pour les logs
	logs            []string           // Liste des messages de log
	maxLogs         = 100              // Nombre maximum de logs à conserver
	myApp           fyne.App           // Instance principale de l'application
	myWindow        fyne.Window        // Fenêtre principale
	logMutex        sync.Mutex         // Mutex pour l'accès concurrent aux logs
	logTicker       *time.Ticker       // Timer pour la mise à jour des logs
	logNeedsUpdate  bool               // Flag indiquant si les logs doivent être rafraîchis
	logBuffer       []string           // Buffer temporaire pour les logs
	logBufferMu     sync.Mutex         // Mutex pour le buffer de logs
	statusBar       *StatusBar         // Barre de statut
	shortcutHandler *ShortcutHandler   // Gestionnaire de raccourcis clavier
)

// Constantes pour les dimensions de fenêtre
const (
	MinWindowWidth  float32 = 800  // Largeur minimale de la fenêtre
	MinWindowHeight float32 = 500  // Hauteur minimale de la fenêtre
	DefaultWidth    float32 = 1100 // Largeur par défaut
	DefaultHeight   float32 = 650  // Hauteur par défaut
)

// main est le point d'entrée de l'application
func main() {
	StartGUI()
}

// addLog ajoute un message au buffer de logs avec timestamp
// Utilise un buffer pour éviter les freezes lors d'ajouts fréquents
func addLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)

	logBufferMu.Lock()
	logBuffer = append(logBuffer, logEntry)
	logBufferMu.Unlock()
}

// flushLogBuffer transfère les logs du buffer vers la liste principale
// Appelé périodiquement par startLogUpdater
func flushLogBuffer() {
	logBufferMu.Lock()
	if len(logBuffer) == 0 {
		logBufferMu.Unlock()
		return
	}
	buffer := logBuffer
	logBuffer = nil
	logBufferMu.Unlock()

	logMutex.Lock()
	logs = append(logs, buffer...)
	// Limiter le nombre de logs conservés
	if len(logs) > maxLogs {
		logs = logs[len(logs)-maxLogs:]
	}
	logNeedsUpdate = true
	logMutex.Unlock()
}

// startLogUpdater démarre le timer de mise à jour des logs
// Met à jour l'affichage toutes les 150ms si nécessaire
func startLogUpdater() {
	logTicker = time.NewTicker(150 * time.Millisecond)

	go func() {
		for range logTicker.C {
			flushLogBuffer()

			logMutex.Lock()
			needsUpdate := logNeedsUpdate
			logNeedsUpdate = false
			logMutex.Unlock()

			if needsUpdate && logWidget != nil {
				logMutex.Lock()
				text := strings.Join(logs, "\n")
				logMutex.Unlock()

				if myWindow != nil {
					myWindow.Canvas().Refresh(logWidget)
				}
				logWidget.SetText(text)
				if logScroll != nil {
					logScroll.ScrollToBottom()
				}
			}
		}
	}()
}

// getPublicIP récupère l'adresse IP publique via une API externe
// Timeout de 5 secondes pour éviter les blocages
func getPublicIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.ipify.org?format=text", nil)
	if err != nil {
		return "Inconnue"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "Inconnue"
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Inconnue"
	}

	return strings.TrimSpace(string(ip))
}

// getLocalIP récupère l'adresse IP locale de la machine
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// StartGUI initialise et démarre l'interface graphique principale
func StartGUI() {
	myApp = app.NewWithID("com.spiralydata.app")

	// Charger la configuration et appliquer le thème
	config, _ := LoadConfig()
	if config == nil {
		config = &AppConfig{DarkTheme: true, ShowStatusBar: true, LogsMaxCount: 100}
	}

	// Thème sombre par défaut
	if config.DarkTheme {
		SetTheme(ThemeDark)
	} else {
		SetTheme(ThemeLight)
	}

	myWindow = myApp.NewWindow("Spiralydata")

	// Configurer la taille de la fenêtre
	width := config.WindowWidth
	height := config.WindowHeight
	if width < MinWindowWidth {
		width = DefaultWidth
	}
	if height < MinWindowHeight {
		height = DefaultHeight
	}
	myWindow.Resize(fyne.NewSize(width, height))

	// Permettre le redimensionnement complet (horizontal ET vertical)
	myWindow.SetFixedSize(false)

	// Initialiser le widget de logs
	logWidget = widget.NewMultiLineEntry()
	logWidget.Wrapping = fyne.TextWrapWord
	logWidget.Disable()
	logScroll = container.NewVScroll(logWidget)
	logScroll.SetMinSize(fyne.NewSize(300, 200))

	// Initialiser la barre de statut
	statusBar = NewStatusBar()

	// Initialiser les raccourcis clavier
	shortcutHandler = NewShortcutHandler()
	shortcutHandler.SetupWindowShortcuts(myWindow)

	startLogUpdater()

	addLog("Application démarrée")

	// Fermeture propre de l'application
	myWindow.SetOnClosed(func() {
		saveWindowConfig()
		if logTicker != nil {
			logTicker.Stop()
		}
	})

	// Tentative de connexion automatique
	if !tryAutoConnect(myWindow) {
		// Afficher le menu principal
		split := container.NewHSplit(
			createMainMenu(myWindow),
			createLogPanel(),
		)
		split.Offset = 0.5

		mainContent := container.NewBorder(
			nil,
			statusBar.GetContainer(),
			nil, nil,
			split,
		)

		myWindow.SetContent(mainContent)
	} else {
		addLog("Connexion automatique en cours...")
	}

	myWindow.ShowAndRun()
}

// createMainMenu crée le menu principal de l'application
// Sans émojis sur les boutons avec texte, sans numéro de version
func createMainMenu(win fyne.Window) fyne.CanvasObject {
	// Titre stylisé
	title := canvas.NewText("SPIRALYDATA", color.White)
	title.TextSize = 32
	title.Alignment = fyne.TextAlignCenter
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := widget.NewLabel("Synchronisation de fichiers intelligente")
	subtitle.Alignment = fyne.TextAlignCenter

	// Boutons du menu principal (sans émojis)
	hostBtn := widget.NewButton("Mode Hôte (Host)", func() {
		showHostSetup(win)
	})
	hostBtn.Importance = widget.HighImportance

	userBtn := widget.NewButton("Mode Utilisateur (User)", func() {
		showUserSetup(win)
	})
	userBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButton("Paramètres", func() {
		showSettings(win)
	})
	settingsBtn.Importance = widget.MediumImportance

	quitBtn := widget.NewButton("Quitter", func() {
		saveWindowConfig()
		myApp.Quit()
	})
	quitBtn.Importance = widget.LowImportance

	// Info raccourcis
	shortcutsInfo := widget.NewLabel("Ctrl+T: Changer thème | F11: Plein écran")
	shortcutsInfo.Alignment = fyne.TextAlignCenter
	shortcutsInfo.TextStyle = fyne.TextStyle{Italic: true}

	buttonsContainer := container.NewVBox(
		hostBtn,
		userBtn,
		widget.NewSeparator(),
		settingsBtn,
		layout.NewSpacer(),
		quitBtn,
	)

	return container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(title),
		container.NewCenter(subtitle),
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(buttonsContainer)),
		layout.NewSpacer(),
		container.NewCenter(shortcutsInfo),
		layout.NewSpacer(),
	)
}

// createLogPanel crée le panneau de logs avec recherche, copie et effacement
func createLogPanel() *fyne.Container {
	titleLabel := widget.NewLabelWithStyle("Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	// Barre de recherche
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Rechercher...")
	searchEntry.OnChanged = func(query string) {
		filterLogs(query)
	}

	// Bouton effacer (emoji seul OK)
	clearBtn := widget.NewButton("🗑️", func() {
		logMutex.Lock()
		logs = []string{}
		logNeedsUpdate = true
		logMutex.Unlock()
		addLog("Logs effacés")
	})
	clearBtn.Importance = widget.LowImportance

	// Bouton copier (emoji seul OK)
	copyBtn := widget.NewButton("📋", func() {
		logMutex.Lock()
		text := strings.Join(logs, "\n")
		logMutex.Unlock()
		myWindow.Clipboard().SetContent(text)
		addLog("Logs copiés dans le presse-papiers")
	})
	copyBtn.Importance = widget.LowImportance

	header := container.NewBorder(
		nil, nil,
		titleLabel,
		container.NewHBox(clearBtn, copyBtn),
		searchEntry,
	)

	return container.NewBorder(
		header,
		nil, nil, nil,
		logScroll,
	)
}

// filterLogs filtre les logs affichés selon la requête de recherche
func filterLogs(query string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	if query == "" {
		logWidget.SetText(strings.Join(logs, "\n"))
		return
	}

	query = strings.ToLower(query)
	var filtered []string
	for _, log := range logs {
		if strings.Contains(strings.ToLower(log), query) {
			filtered = append(filtered, log)
		}
	}
	logWidget.SetText(strings.Join(filtered, "\n"))
}

// saveWindowConfig sauvegarde la configuration de la fenêtre
func saveWindowConfig() {
	config, _ := LoadConfig()
	if config == nil {
		config = &AppConfig{}
	}
	size := myWindow.Canvas().Size()
	config.WindowWidth = size.Width
	config.WindowHeight = size.Height
	config.DarkTheme = GetCurrentTheme() == ThemeDark
	SaveConfig(config)
}

// showSettings affiche la page des paramètres depuis le menu principal
func showSettings(win fyne.Window) {
	showSettingsWithBack(win, func() {
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
}

// showSettingsWithBack affiche les paramètres avec callback de retour personnalisé
func showSettingsWithBack(win fyne.Window, backFunc func()) {
	config, _ := LoadConfig()
	if config == nil {
		config = &AppConfig{DarkTheme: true, ShowStatusBar: true, LogsMaxCount: 100}
	}

	// Sélection du thème
	themeLabel := widget.NewLabel("Thème")
	themeSelect := widget.NewSelect([]string{"Sombre", "Clair"}, func(selected string) {
		if selected == "Sombre" {
			SetTheme(ThemeDark)
			config.DarkTheme = true
		} else {
			SetTheme(ThemeLight)
			config.DarkTheme = false
		}
		SaveConfig(config)
	})
	if GetCurrentTheme() == ThemeDark {
		themeSelect.SetSelected("Sombre")
	} else {
		themeSelect.SetSelected("Clair")
	}

	// Nombre max de logs
	logsLabel := widget.NewLabel("Nombre max de logs")
	logsEntry := widget.NewEntry()
	logsEntry.SetText(fmt.Sprintf("%d", maxLogs))
	logsEntry.OnChanged = func(s string) {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 && n <= 1000 {
			maxLogs = n
		}
	}

	// Réinitialiser
	resetBtn := widget.NewButton("Réinitialiser la configuration", func() {
		SaveConfig(&AppConfig{DarkTheme: true, ShowStatusBar: true, LogsMaxCount: 100})
		SetTheme(ThemeDark)
		themeSelect.SetSelected("Sombre")
		addLog("Configuration réinitialisée")
	})
	resetBtn.Importance = widget.DangerImportance

	formContent := container.NewVBox(
		themeLabel,
		themeSelect,
		widget.NewSeparator(),
		logsLabel,
		logsEntry,
		widget.NewSeparator(),
		resetBtn,
	)

	backBtn := widget.NewButton("Retour", backFunc)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Paramètres", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		formContent,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(backBtn)),
		layout.NewSpacer(),
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

// showHostSetup affiche la page de configuration du mode Hôte
func showHostSetup(win fyne.Window) {
	portLabel := widget.NewLabel("Port")
	portLabel.Alignment = fyne.TextAlignLeading
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("ex: 1234")

	// ID: minimum 6 caractères, pas de maximum
	idLabel := widget.NewLabel("ID du serveur (6 caractères minimum)")
	idLabel.Alignment = fyne.TextAlignLeading
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("ex: 123456")

	// Section filtres
	filterConfig := GetFilterConfig()
	filterSummary := widget.NewLabel(filterConfig.GetSummary())
	filterSummary.TextStyle = fyne.TextStyle{Italic: true}
	filterSummary.Wrapping = fyne.TextWrapWord

	filterBtn := widget.NewButton("Configurer les filtres", func() {
		ShowFilterDialog(filterConfig, win, func() {
			filterSummary.SetText(filterConfig.GetSummary())
		})
	})
	filterBtn.Importance = widget.MediumImportance

	filterSection := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Filtrage des fichiers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		filterSummary,
		filterBtn,
	)

	formContent := container.NewVBox(
		portLabel,
		portEntry,
		widget.NewSeparator(),
		idLabel,
		idEntry,
		filterSection,
	)

	startBtn := widget.NewButton("Démarrer le serveur", func() {
		port := portEntry.Text
		hostID := idEntry.Text

		// Validation: minimum 6 caractères
		if len(hostID) < 6 {
			addLog("L'ID doit contenir au moins 6 caractères")
			return
		}

		if port == "" {
			addLog("Le port est requis")
			return
		}

		showHostRunning(win, port, hostID)
	})
	startBtn.Importance = widget.HighImportance

	settingsBtn := widget.NewButton("Paramètres", func() {
		showSettingsWithBack(win, func() {
			showHostSetup(win)
		})
	})

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
		startBtn,
		settingsBtn,
		backBtn,
	)

	content := container.NewVBox(
		widget.NewLabelWithStyle("Configuration du Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		formContent,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(buttonsContainer)),
		layout.NewSpacer(),
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

// showHostRunning affiche l'interface du serveur en cours d'exécution
func showHostRunning(win fyne.Window, port, hostID string) {
	addLog("Démarrage du serveur...")

	localIP := getLocalIP()
	publicIP := "Chargement..."

	// Carte d'information du serveur
	serverInfoCard := NewStatCard("Serveur", "🖥️", "Démarrage...")

	infoText := fmt.Sprintf(
		"SERVEUR EN DÉMARRAGE\n\n"+
			"ID: %s\n"+
			"Port: %s\n"+
			"IP Locale: %s\n"+
			"IP Publique: %s\n"+
			"Adresse: %s:%s\n\n"+
			"Clients connectés: 0",
		hostID, port, localIP, publicIP, publicIP, port,
	)

	info := widget.NewLabel(infoText)
	info.Wrapping = fyne.TextWrapWord

	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	loadingIndex := 0
	stopLoading := false
	loadingLabel := widget.NewLabel("⠋ Démarrage du serveur...")

	var currentServer *Server

	// Animation de chargement
	go func() {
		for !stopLoading {
			time.Sleep(100 * time.Millisecond)
			if !stopLoading {
				char := loadingChars[loadingIndex%len(loadingChars)]
				loadingLabel.SetText(fmt.Sprintf("%s Démarrage du serveur...", char))
				loadingLabel.Refresh()
				loadingIndex++
			}
		}
		loadingLabel.SetText("Serveur actif")
		loadingLabel.Refresh()
	}()

	// Bouton déconnexion qui retourne à la page Host setup (pas fermer l'app)
	disconnectBtn := widget.NewButton("Arrêter le serveur", func() {
		addLog("Arrêt du serveur...")
		stopLoading = true
		if currentServer != nil {
			currentServer.Stop()
		}
		statusBar.SetConnected(false, "")
		showHostSetup(win)
	})
	disconnectBtn.Importance = widget.DangerImportance

	// Bouton filtres modifiable en direct
	filterConfig := GetFilterConfig()
	filterBtn := widget.NewButton("Filtres", func() {
		ShowFilterDialog(filterConfig, win, func() {
			addLog("Filtres mis a jour")
		})
	})
	filterBtn.Importance = widget.MediumImportance

	// Bouton sécurité (whitelist IP)
	securityBtn := widget.NewButton("Securite", func() {
		showHostSecurityDialog(win)
	})
	securityBtn.Importance = widget.MediumImportance

	// Bouton télécharger une backup
	backupBtn := widget.NewButton("Telecharger une backup", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			go func() {
				sourceDir := filepath.Join(getExecutableDir(), "Spiralydata")
				timestamp := time.Now().Format("2006-01-02_15-04-05")
				destDir := filepath.Join(uri.Path(), fmt.Sprintf("Backup_Spiralydata_%s", timestamp))
				addLog(fmt.Sprintf("Backup vers: %s", destDir))
				
				err := copyDirRecursive(sourceDir, destDir)
				if err != nil {
					addLog(fmt.Sprintf("Erreur backup: %v", err))
				} else {
					addLog("Backup terminee avec succes")
				}
			}()
		}, win)
	})
	backupBtn.Importance = widget.MediumImportance

	// Boutons d'actions Host
	actionsContainer := container.NewHBox(
		filterBtn,
		securityBtn,
		backupBtn,
	)

	content := container.NewVBox(
		serverInfoCard.GetContainer(),
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		loadingLabel,
		widget.NewSeparator(),
		container.NewCenter(actionsContainer),
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(disconnectBtn)),
	)

	// Ajouter du padding
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

	// Démarrage asynchrone du serveur
	go func() {
		pubIP := getPublicIP()
		time.Sleep(1 * time.Second)
		stopLoading = true

		// Mettre à jour la carte d'info
		serverInfoCard.SetValue("Actif")

		updatedInfo := fmt.Sprintf(
			"SERVEUR ACTIF\n\n"+
				"ID: %s\n"+
				"Port: %s\n"+
				"IP Locale: %s\n"+
				"IP Publique: %s\n"+
				"Adresse: %s:%s\n\n"+
				"Partagez l'IP publique et l'ID\n"+
				"avec les utilisateurs.",
			hostID, port, localIP, pubIP, pubIP, port,
		)
		info.SetText(updatedInfo)
		info.Refresh()

		statusBar.SetConnected(true, localIP+":"+port)

		currentServer = NewServer(hostID)
		addLog(fmt.Sprintf("Port: %s", port))
		addLog(fmt.Sprintf("ID: %s", hostID))
		addLog(fmt.Sprintf("IP Locale: %s", localIP))
		addLog(fmt.Sprintf("IP Publique: %s", pubIP))
		currentServer.Start(port)
	}()
}

// showHostSecurityDialog affiche le dialogue de sécurité pour le Host
func showHostSecurityDialog(win fyne.Window) {
	// Whitelist IP
	ipWhitelistCheck := widget.NewCheck("Activer la liste blanche IP", nil)
	ipWhitelist := GetIPWhitelist()
	ipWhitelistCheck.SetChecked(ipWhitelist.IsEnabled())
	ipWhitelistCheck.OnChanged = func(enabled bool) {
		if enabled {
			ipWhitelist.Enable()
			addLog("Whitelist IP activée")
		} else {
			ipWhitelist.Disable()
			addLog("Whitelist IP désactivée")
		}
	}

	// Conteneur pour afficher les IP (simple VBox au lieu de widget.List)
	ipListContainer := container.NewVBox()
	
	// Fonction pour rafraîchir la liste des IP
	var refreshIPList func()
	refreshIPList = func() {
		ipListContainer.RemoveAll()
		ips := ipWhitelist.GetIPs()
		
		if len(ips) == 0 {
			emptyLabel := widget.NewLabel("Aucune IP configurée")
			emptyLabel.TextStyle = fyne.TextStyle{Italic: true}
			ipListContainer.Add(emptyLabel)
		} else {
			for _, ip := range ips {
				currentIP := ip // Capture de la valeur pour la closure
				
				ipLabel := widget.NewLabel(currentIP)
				removeBtn := widget.NewButton("×", func() {
					ipWhitelist.RemoveIP(currentIP)
					refreshIPList()
					addLog(fmt.Sprintf("IP supprimée: %s", currentIP))
				})
				removeBtn.Importance = widget.DangerImportance
				
				row := container.NewBorder(nil, nil, nil, removeBtn, ipLabel)
				ipListContainer.Add(row)
			}
		}
		ipListContainer.Refresh()
	}
	
	refreshIPList()

	// Info-bulle
	infoLabel := widget.NewLabel("La whitelist IP permet de n'autoriser que certaines\nadresses IP à se connecter au serveur.")
	infoLabel.TextStyle = fyne.TextStyle{Italic: true}

	// Container pour la liste avec taille fixe (zone scrollable)
	ipListScroll := container.NewVScroll(ipListContainer)
	ipListScroll.SetMinSize(fyne.NewSize(400, 200))

	// Section du haut: titre, info, checkbox et liste
	topSection := container.NewVBox(
		widget.NewLabelWithStyle("Sécurité du Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		infoLabel,
		widget.NewSeparator(),
		ipWhitelistCheck,
		widget.NewSeparator(),
		widget.NewLabel("IPs autorisées:"),
		ipListScroll,
	)

	// Champ pour ajouter une IP (en dehors du scroll, toujours visible en bas)
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("Ex: 192.168.1.100 ou 192.168.1.0/24")

	addIPBtn := widget.NewButton("Ajouter", func() {
		ip := ipEntry.Text
		if ip == "" {
			return
		}
		if err := ipWhitelist.AddIP(ip); err != nil {
			dialog.ShowError(err, win)
		} else {
			addLog(fmt.Sprintf("IP ajoutée: %s", ip))
			ipEntry.SetText("")
			refreshIPList()
		}
	})
	addIPBtn.Importance = widget.HighImportance

	// Section du bas (fixe): label + champ + bouton
	addIPLabel := widget.NewLabel("Ajouter une IP:")
	bottomSection := container.NewVBox(
		widget.NewSeparator(),
		addIPLabel,
		container.NewBorder(nil, nil, nil, addIPBtn, ipEntry),
	)

	// Layout final: top + bottom
	content := container.NewBorder(
		topSection,
		bottomSection,
		nil, nil,
		nil,
	)

	dialog.ShowCustom("Sécurité", "Fermer", content, win)
}
