package main

import (
	"fmt"
	"image/color"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	logWidget      *widget.Entry
	logScroll      *container.Scroll
	logs           []string
	maxLogs        = 100
	myApp          fyne.App
	logMutex       sync.Mutex
	logTicker      *time.Ticker
	logNeedsUpdate bool
)

func main() {
	StartGUI()
}

type darkTheme struct{}

func (d darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 30, G: 33, B: 41, A: 255}
	case theme.ColorNameButton:
		return color.RGBA{R: 52, G: 58, B: 70, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	case theme.ColorNameSuccess:
		return color.RGBA{R: 46, G: 204, B: 113, A: 255}
	case theme.ColorNameError:
		return color.RGBA{R: 231, G: 76, B: 60, A: 255}
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (d darkTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (d darkTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (d darkTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func addLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)
	
	logMutex.Lock()
	logs = append(logs, logEntry)
	if len(logs) > maxLogs {
		logs = logs[1:]
	}
	logNeedsUpdate = true
	logMutex.Unlock()
}

func startLogUpdater() {
	logTicker = time.NewTicker(200 * time.Millisecond)
	
	go func() {
		for range logTicker.C {
			logMutex.Lock()
			needsUpdate := logNeedsUpdate
			logNeedsUpdate = false
			logMutex.Unlock()
			
			if needsUpdate && logWidget != nil {
				logMutex.Lock()
				text := strings.Join(logs, "\n")
				logMutex.Unlock()
				
				logWidget.SetText(text)
				if logScroll != nil {
					logScroll.ScrollToBottom()
				}
			}
		}
	}()
}

func getPublicIP() string {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "Unknown"
	}
	defer resp.Body.Close()
	
	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Unknown"
	}
	
	return string(ip)
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "Unknown"
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

func StartGUI() {
	myApp = app.NewWithID("com.spiraly.sync")
	myApp.Settings().SetTheme(&darkTheme{})
	
	startLogUpdater()
	
	myWindow := myApp.NewWindow("Spiralydata")
	myWindow.Resize(fyne.NewSize(1200, 700))
	
	logWidget = widget.NewEntry()
	logWidget.SetText("🚀 Bienvenue dans Spiralydata\n")
	logWidget.MultiLine = true
	logWidget.Wrapping = fyne.TextWrapWord
	logWidget.Disable()
	
	// Style monospace pour les logs
	logWidget.TextStyle = fyne.TextStyle{Monospace: true}
	
	logScroll = container.NewVScroll(logWidget)
	logScroll.SetMinSize(fyne.NewSize(400, 600))
	
	logContainer := container.NewBorder(
		widget.NewLabelWithStyle("📋 Logs", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		logScroll,
	)
	
	// Tentative de connexion automatique
	if !tryAutoConnect(myWindow) {
		content := createMainMenu(myWindow)
		
		split := container.NewHSplit(
			content,
			logContainer,
		)
		split.Offset = 0.5
		
		myWindow.SetContent(split)
	} else {
		addLog("🔄 Connexion automatique en cours...")
	}
	
	myWindow.ShowAndRun()
}

func createMainMenu(win fyne.Window) fyne.CanvasObject {
	title := canvas.NewText("SPIRALYDATA", color.White)
	title.TextSize = 28
	title.Alignment = fyne.TextAlignCenter
	title.TextStyle = fyne.TextStyle{Bold: true}
	
	subtitle := widget.NewLabel("📄 Synchronisation de fichiers intelligente")
	subtitle.Alignment = fyne.TextAlignCenter
	
	hostBtn := widget.NewButton("Mode Hôte (Host)", func() {
		showHostSetup(win)
	})
	hostBtn.Importance = widget.HighImportance
	
	userBtn := widget.NewButton("Mode Utilisateur (User)", func() {
		showUserSetup(win)
	})
	userBtn.Importance = widget.HighImportance
	
	quitBtn := widget.NewButton("Quitter", func() {
		myApp.Quit()
	})
	
	buttonsContainer := container.NewVBox(
		hostBtn,
		userBtn,
		layout.NewSpacer(),
		quitBtn,
	)
	
	return container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(title),
		container.NewCenter(subtitle),
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(
			container.NewVBox(
				buttonsContainer,
			),
		)),
		layout.NewSpacer(),
	)
}

func showHostSetup(win fyne.Window) {
	portLabel := widget.NewLabel("🌐 Port")
	portLabel.Alignment = fyne.TextAlignLeading
	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("ex: 1234")
	
	idLabel := widget.NewLabel("🔑 ID du serveur (6 chiffres)")
	idLabel.Alignment = fyne.TextAlignLeading
	idEntry := widget.NewEntry()
	idEntry.SetPlaceHolder("ex: 123456")
	
	formContent := container.NewVBox(
		portLabel,
		portEntry,
		widget.NewSeparator(),
		idLabel,
		idEntry,
	)
	
	startBtn := widget.NewButton("Démarrer le serveur", func() {
		port := portEntry.Text
		hostID := idEntry.Text
		
		if len(hostID) != 6 {
			addLog("❌ L'ID doit contenir 6 caractères")
			return
		}
		
		if port == "" {
			addLog("❌ Le port est requis")
			return
		}
		
		showHostRunning(win, port, hostID)
	})
	startBtn.Importance = widget.HighImportance
	
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
		startBtn,
		backBtn,
	)
	
	content := container.NewVBox(
		widget.NewLabelWithStyle("⚙️ Configuration du Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
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

func showHostRunning(win fyne.Window, port, hostID string) {
	addLog("🚀 Démarrage du serveur...")
	
	localIP := getLocalIP()
	publicIP := "⏳ Chargement..."
	
	infoText := fmt.Sprintf(
		"🖥️ SERVEUR EN DÉMARRAGE\n\n"+
			"🔑 ID: %s\n"+
			"🌐 Port: %s\n"+
			"🏠 IP Locale: %s\n"+
			"🌍 IP Publique: %s\n"+
			"📍 Adresse: %s:%s\n\n"+
			"👥 Clients connectés: 0",
		hostID, port, localIP, publicIP, publicIP, port,
	)
	
	info := widget.NewLabel(infoText)
	info.Wrapping = fyne.TextWrapWord
	
	loadingChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	loadingIndex := 0
	stopLoading := false
	loadingLabel := widget.NewLabel("⠋ Démarrage du serveur...")
	
	var currentServer *Server
	
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
		loadingLabel.SetText("✅ Serveur actif")
		loadingLabel.Refresh()
	}()
	
	stopBtn := widget.NewButton("Arrêter le serveur", func() {
		addLog("🛑 Arrêt du serveur...")
		stopLoading = true
		
		if currentServer != nil {
			currentServer.Stop()
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
	stopBtn.Importance = widget.DangerImportance
	
	content := container.NewVBox(
		widget.NewLabelWithStyle("ℹ️ Informations du Serveur", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		info,
		widget.NewSeparator(),
		loadingLabel,
		layout.NewSpacer(),
		container.NewCenter(container.NewPadded(stopBtn)),
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
		pubIP := getPublicIP()
		time.Sleep(1 * time.Second)
		stopLoading = true
		
		updatedInfo := fmt.Sprintf(
			"✅ SERVEUR ACTIF\n\n"+
				"🔑 ID: %s\n"+
				"🌐 Port: %s\n"+
				"🏠 IP Locale: %s\n"+
				"🌍 IP Publique: %s\n"+
				"📍 Adresse: %s:%s\n\n"+
				"💡 Partagez l'IP publique et l'ID\n"+
				"avec les utilisateurs.",
			hostID, port, localIP, pubIP, pubIP, port,
		)
		info.SetText(updatedInfo)
		info.Refresh()
		
		currentServer = NewServer(hostID)
		addLog(fmt.Sprintf("🌐 Port: %s", port))
		addLog(fmt.Sprintf("🔑 ID: %s", hostID))
		addLog(fmt.Sprintf("🏠 IP Locale: %s", localIP))
		addLog(fmt.Sprintf("🌍 IP Publique: %s", pubIP))
		currentServer.Start(port)
	}()
}