# Documentation Technique - SpiralData

## Architecture

### Vue d'ensemble

SpiralData est une application Go utilisant Fyne pour l'interface graphique et WebSocket (gorilla/websocket) pour la communication réseau.

| Composant | Description |
|-----------|-------------|
| **Server** | Gère les connexions WebSocket, diffuse les changements |
| **Client** | Se connecte au serveur, synchronise les fichiers locaux |
| **GUI** | Interface Fyne avec thèmes, logs, et contrôles |

---

## Structure des fichiers

### Fichiers principaux

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `gui.go` | Point d'entrée, interface principale | `main()`, `StartGUI()`, `createMainMenu()` |
| `server.go` | Serveur WebSocket | `NewServer()`, `Start()`, `Stop()`, `handleWS()` |
| `client.go` | Client WebSocket | `StartClientGUI()`, `ToggleAutoSync()`, `applyChange()` |
| `client_operations.go` | Opérations fichiers côté client | `PullAllFromServer()`, `PushLocalChanges()`, `watchRecursive()` |
| `client_connect.go` | Interface connexion client | `showUserConnecting()`, `showUserConnected()` |

### Fichiers de synchronisation

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `sync_modes.go` | Modes de sync, file de transfert, pending actions | `NewTransferQueue()`, `NewPendingActionsManager()`, `CompressData()` |
| `sync_ui.go` | Dialogues de configuration sync | `ShowSyncConfigDialog()`, `ShowTransferQueueDialog()`, `ShowConflictDialog()` |
| `server_handlers.go` | Gestion fichiers côté serveur | `sendAllFilesAndDirs()`, `applyChange()`, `watchRecursive()` |

### Fichiers d'interface

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `file_explorer.go` | Explorateur de fichiers | `NewFileExplorer()`, `Show()`, `loadFileTree()`, `downloadSelected()` |
| `explorer_utils.go` | Utilitaires explorateur | `formatSize()`, `getFileIcon()`, `sortItems()` |
| `filters.go` | Système de filtrage | `NewFilterConfig()`, `ShouldFilterFile()` |
| `filters_ui.go` | Interface filtres | `ShowFilterDialog()` |
| `preview.go` | Prévisualisation fichiers | `PreviewManager`, `CanPreview()`, `GetPreview()` |
| `preview_ui.go` | Interface prévisualisation | `PreviewPanel`, `ShowPreview()` |

### Fichiers de configuration et sécurité

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `config.go` | Gestion configuration | `LoadConfig()`, `SaveConfig()`, `SaveSyncConfigToFile()` |
| `security.go` | Sécurité et whitelist | `IPWhitelist`, `AddIP()`, `IsAllowed()` |
| `security_ui.go` | Interface sécurité | Composants UI pour la sécurité |
| `access_control.go` | Contrôle d'accès | Gestion des permissions |
| `encryption.go` | Chiffrement | Fonctions de chiffrement des données |
| `audit.go` | Journalisation audit | Traçabilité des actions |

### Fichiers utilitaires

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `types.go` | Types de données partagés | `FileChange`, `AuthRequest`, `AuthResponse` |
| `utils.go` | Utilitaires divers | `FormatFileSize()`, `getExecutableDir()`, `copyDirRecursive()` |
| `themes.go` | Gestion des thèmes | `SetTheme()`, `ThemeDark`, `ThemeLight` |
| `ui_components.go` | Composants UI réutilisables | `StatusBar`, `StatCard`, `ShortcutHandler` |
| `logging.go` | Système de logs | Gestion avancée des logs |
| `network.go` | Utilitaires réseau | Fonctions réseau |

### Fichiers avancés

| Fichier | Rôle | Fonctions principales |
|---------|------|----------------------|
| `conflicts.go` | Gestion des conflits | `ConflictManager`, `DetectConflict()`, `ResolveConflict()` |
| `backup.go` | Sauvegarde manuelle | `copyDirRecursive()`, `copyFile()` |
| `performance.go` | Monitoring performance | Metriques et optimisation |
| `performance_ui.go` | Interface performance | Affichage des métriques |
| `monitoring_ui.go` | Interface monitoring | Surveillance système |
| `collaboration.go` | Fonctions collaboratives | Multi-utilisateurs |

---

## Structures de données

### FileChange
```go
type FileChange struct {
    FileName string  // Chemin relatif du fichier
    Op       string  // Opération: "create", "write", "remove", "mkdir"
    Content  string  // Contenu encodé en Base64
    Origin   string  // "client" ou "server"
    IsDir    bool    // True si c'est un dossier
}
```

### PendingAction
```go
type PendingAction struct {
    Type    ActionType  // ActionCreate, ActionModify, ActionDelete
    Path    string      // Chemin du fichier
    Size    int64       // Taille en bytes
    ModTime time.Time   // Date de modification
    IsDir   bool        // True si dossier
    AddedAt time.Time   // Date d'ajout à la queue
}
```

### TransferItem
```go
type TransferItem struct {
    Path       string    // Chemin du fichier
    Priority   int       // Priorité (plus bas = plus prioritaire)
    Size       int64     // Taille
    IsDir      bool      // Est un dossier
    Operation  string    // Type d'opération
    Content    string    // Contenu Base64
    Compressed bool      // Compressé ou non
    Retries    int       // Nombre de tentatives
    AddedAt    time.Time // Date d'ajout
}
```

### SyncConfig
```go
type SyncConfig struct {
    Mode               SyncMode        // Mode de synchronisation
    CompressionEnabled bool            // Compression activée
    CompressionLevel   int             // Niveau 1-9
    BandwidthLimit     int64           // Limite en bytes/sec
    RetryCount         int             // Nombre de retry
    RetryDelay         time.Duration   // Délai entre retry
    ScheduleEnabled    bool            // Planification activée
    ScheduleInterval   time.Duration   // Intervalle de sync
    ConflictStrategy   ConflictStrategy // Stratégie de conflit
}
```

---

## Flux de synchronisation

### Connexion Client

```
1. Client → Server : Connexion WebSocket
2. Client → Server : AuthRequest { Type: "auth_request", HostID: "..." }
3. Server → Client : AuthResponse { Type: "auth_success" } ou { Type: "auth_failed" }
4. Server → Client : Envoi de tous les fichiers existants
5. Client : scanInitial() - Scan du dossier local
6. Client : ScanAndDetectDifferences() - Détection des différences
7. Client : watchRecursive() - Surveillance des changements
```

### Synchronisation automatique (Sync Auto ON)

```
Modification locale détectée
    ↓
watchRecursive() → handleLocalEvent()
    ↓
Envoi immédiat au serveur (FileChange)
    ↓
Server → broadcast() à tous les autres clients
```

### Synchronisation manuelle (Sync Auto OFF)

```
Modification locale détectée
    ↓
watchRecursive() → TrackLocalChange()
    ↓
Ajout à PendingActions
    ↓
(Utilisateur clique "ENVOYER")
    ↓
PushLocalChanges() → Envoi au serveur
    ↓
GetPendingActions().Clear()
```

---

## API WebSocket

### Messages Client → Server

| Type | Description | Données |
|------|-------------|---------|
| `auth_request` | Authentification | `{ type, host_id }` |
| `request_all_files` | Demande tous les fichiers | `{ type }` |
| `request_file_tree` | Demande l'arborescence | `{ type }` |
| `download_request` | Téléchargement sélectif | `{ type, items: [] }` |
| `FileChange` | Modification fichier | `{ filename, op, content, origin, is_dir }` |

### Messages Server → Client

| Type | Description | Données |
|------|-------------|---------|
| `auth_success` | Authentification réussie | `{ type, message }` |
| `auth_failed` | Authentification échouée | `{ type, message }` |
| `file_tree_item` | Élément d'arborescence | `{ type, path, name, is_dir }` |
| `file_tree_complete` | Fin d'arborescence | `{ type }` |
| `FileChange` | Modification fichier | `{ filename, op, content, origin, is_dir }` |

---

## Filtres

### Types de filtres

| Filtre | Description | Exemple |
|--------|-------------|---------|
| Extension | Exclure par extension | `.tmp`, `.log`, `.bak` |
| Path | Exclure par chemin | `node_modules`, `.git`, `cache/` |
| Size | Exclure par taille | Min: 0, Max: 100MB |

### Vérification

```go
filterConfig := GetFilterConfig()
if filterConfig.ShouldFilterFile(path, size, false) {
    // Fichier filtré, ignorer
    return
}
```

---

## Gestion des conflits

### Stratégies disponibles

| Stratégie | Description |
|-----------|-------------|
| `ConflictAskUser` | Demander à l'utilisateur |
| `ConflictKeepNewest` | Garder le plus récent (par date) |
| `ConflictKeepLocal` | Toujours garder la version locale |
| `ConflictKeepRemote` | Toujours garder la version serveur |
| `ConflictKeepBoth` | Créer deux copies |
| `ConflictAutoMerge` | Fusion automatique (si possible) |

---

## Compression

### Activation

La compression gzip peut etre activee dans la configuration sync pour reduire la taille des transferts.

```go
compressed, err := CompressData(data, config.CompressionLevel)
encoded := base64.StdEncoding.EncodeToString(compressed)
```

### Decompression

```go
decoded, _ := base64.StdEncoding.DecodeString(encoded)
data, err := DecompressData(decoded)
```

---

## Backup

### Fonction de backup

Le bouton "Telecharger une backup" permet de copier tous les fichiers synchronises vers un dossier externe.

```go
// Copie recursive d'un dossier
func copyDirRecursive(src, dst string) error
func copyFile(src, dst string) error
```

### Utilisation
- Disponible dans les modes Host et User
- Ouvre un dialogue de selection de dossier
- Copie tous les fichiers du dossier de synchronisation

---

## Deconnexion

### Comportement

La deconnexion retourne au menu principal sans fermer l'application :
- Le client ferme la connexion WebSocket
- Les ressources sont liberees (cleanup)
- L'interface revient a showUserSetup() ou showHostSetup()

---

## Sécurité

### Whitelist IP

```go
whitelist := GetIPWhitelist()
whitelist.Enable()
whitelist.AddIP("192.168.1.100")
whitelist.AddIP("10.0.0.0/8")  // Plage CIDR

if !whitelist.IsAllowed(clientIP) {
    // Refuser la connexion
}
```

---

## Compilation

### Prérequis

- Go 1.21+
- Fyne v2
- gorilla/websocket
- fsnotify

### Commandes

```bash
# Installation des dépendances
go mod tidy

# Compilation simple
go build -o spiralydata

# Compilation avec icône (Windows)
# Voir section "Icône de l'exécutable"
```

### Icône de l'exécutable (Windows)

1. Placer `Spiralylogo.png` dans le dossier source
2. Installer rsrc : `go install github.com/akavel/rsrc@latest`
3. Créer un fichier `app.manifest` :
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity version="1.0.0.0" processorArchitecture="*" name="Spiralydata" type="win32"/>
</assembly>
```
4. Convertir l'icône : Convertir PNG en ICO
5. Générer le fichier ressource : `rsrc -manifest app.manifest -ico Spiralylogo.ico -o rsrc.syso`
6. Compiler : `go build -ldflags="-H windowsgui" -o spiralydata.exe`

---

## Variables globales importantes

| Variable | Type | Description |
|----------|------|-------------|
| `myApp` | `fyne.App` | Instance de l'application |
| `myWindow` | `fyne.Window` | Fenêtre principale |
| `logWidget` | `*widget.Entry` | Widget des logs |
| `statusBar` | `*StatusBar` | Barre de statut |
| `globalSyncConfig` | `*SyncConfig` | Configuration sync globale |
| `globalTransferQueue` | `*TransferQueue` | File de transfert globale |
| `globalPendingActions` | `*PendingActionsManager` | Actions en attente |

---

## Logs

### Ajout de logs

```go
addLog("Message de log")
addLog(fmt.Sprintf("Message avec valeur: %d", value))
```

### Emojis standards utilisés

| Emoji | Signification |
|-------|---------------|
| ✅ | Succès |
| ❌ | Erreur |
| ⚠️ | Avertissement |
| 📤 | Envoi |
| 📥 | Réception |
| 🔍 | Scan/Recherche |
| 📁 | Dossier |
| 📄 | Fichier |
| 🔌 | Connexion |
| 👀 | Surveillance |

---

## Performance

### Optimisations implémentées

- Buffer de logs pour éviter les freezes UI
- Rate limiting sur les envois de fichiers
- Délais entre opérations pour éviter la surcharge
- Queue d'opérations pour éviter les race conditions
- Mutex pour la thread-safety

### Paramètres de timing

| Opération | Délai |
|-----------|-------|
| Entre fichiers (batch) | 20-50ms |
| Entre lots de 10 fichiers | 50-200ms |
| Refresh logs | 150ms |
| Scan périodique | 3s |

---

## Dépendances

```go
require (
    fyne.io/fyne/v2 v2.x.x
    github.com/gorilla/websocket v1.x.x
    github.com/fsnotify/fsnotify v1.x.x
)
```
