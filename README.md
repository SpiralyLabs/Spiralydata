# 🌀 Spiralydata

**Synchronisation de fichiers en temps réel via WebSocket**

---

## 🇫🇷 Français

### 📋 Description

Spiralydata est une application de synchronisation de fichiers en temps réel entre un hôte (serveur) et plusieurs clients. Elle utilise WebSocket pour une communication bidirectionnelle instantanée et offre une interface graphique moderne et intuitive.

### ✨ Fonctionnalités

#### Synchronisation
- **Temps réel** : Les modifications sont propagées instantanément
- **Bidirectionnelle** : Hôte → Clients et Clients → Hôte
- **Mode manuel ou automatique** : Choisissez votre mode de synchronisation
- **Gestion des conflits** : Détection et résolution intelligente

#### Interface utilisateur
- **Thèmes** : Clair, sombre et personnalisé
- **Explorateur de fichiers** : Navigation et téléchargement depuis le serveur
- **Logs en temps réel** : Suivi détaillé des opérations
- **Barre de statut** : État de connexion visible en permanence

#### Fonctionnalités avancées
- **Backup** : Sauvegarde complète du serveur vers un dossier local horodaté
- **Filtres** : Inclusion/exclusion par extension ou pattern
- **Prévisualisation** : Aperçu des fichiers texte 
- **Sécurité** : Authentification par identifiant hôte

### 🚀 Installation

#### Prérequis
- Go 1.21 ou supérieur
- Connexion réseau entre l'hôte et les clients

#### Compilation

**Windows :**
```batch
setup_windows.bat
```

**Linux :**
```bash
chmod +x setup_linux.sh
./setup_linux.sh
```

#### Compilation manuelle
```bash
go mod init spiralydata
go mod tidy
go build -ldflags "-H=windowsgui" -o spiralydata.exe .
```

### 📖 Utilisation

#### Mode Hôte (Serveur)
1. Lancez l'application
2. Cliquez sur **"Héberger"**
3. Configurez le port (défaut: 1212)
4. Notez l'**identifiant hôte** affiché
5. Partagez votre **IP** et l'**identifiant** aux clients

#### Mode Client (Utilisateur)
1. Lancez l'application
2. Cliquez sur **"Rejoindre"**
3. Entrez l'**adresse IP** du serveur (ex: 192.168.1.10:1212)
4. Entrez l'**identifiant hôte**
5. Choisissez le **dossier de synchronisation**
6. Cliquez sur **"Connexion"**

#### Boutons de contrôle (Client)
| Bouton | Description |
|--------|-------------|
| **ENVOYER** | Envoie vos fichiers locaux vers le serveur |
| **RECEVOIR** | Récupère les fichiers du serveur |
| **BACKUP** | Crée une sauvegarde complète dans un dossier `Backup_Spiralydata_DATE` |
| **EXPLORER** | Ouvre l'explorateur de fichiers du serveur |

### 📁 Structure des dossiers

```
Spiralydata/
├── spiralydata.exe    # Exécutable
├── Spiralydata/       # Dossier synchronisé (créé automatiquement)
├── config.json        # Configuration sauvegardée
└── logs/              # Journaux d'activité
```

### ⚙️ Configuration

Le fichier `config.json` est créé automatiquement et contient :
- Dernière adresse serveur utilisée
- Dernier identifiant hôte
- Dernier dossier de synchronisation
- Thème sélectionné
- Filtres configurés

### 🔧 Résolution de problèmes

| Problème | Solution |
|----------|----------|
| Connexion refusée | Vérifiez l'IP, le port et le pare-feu |
| Identifiant incorrect | Vérifiez l'identifiant affiché côté hôte |
| Fichiers non synchronisés | Vérifiez les filtres configurés |
| Application qui freeze | Réduisez le nombre de fichiers ou augmentez les délais |

---

## 🇬🇧 English

### 📋 Description

Spiralydata is a real-time file synchronization application between a host (server) and multiple clients. It uses WebSocket for instant bidirectional communication and offers a modern, intuitive graphical interface.

### ✨ Features

#### Synchronization
- **Real-time**: Changes are propagated instantly
- **Bidirectional**: Host → Clients and Clients → Host
- **Manual or automatic mode**: Choose your synchronization mode
- **Conflict management**: Intelligent detection and resolution

#### User Interface
- **Themes**: Light, dark and custom
- **File explorer**: Browse and download from server
- **Real-time logs**: Detailed operation tracking
- **Status bar**: Connection status always visible

#### Advanced Features
- **Backup**: Complete server backup to a local timestamped folder
- **Filters**: Include/exclude by extension or pattern
- **Preview**: Preview text files
- **Security**: Authentication by host identifier

### 🚀 Installation

#### Prerequisites
- Go 1.21 or higher
- Network connection between host and clients

#### Compilation

**Windows:**
```batch
setup_windows.bat
```

**Linux:**
```bash
chmod +x setup_linux.sh
./setup_linux.sh
```

#### Manual Compilation
```bash
go mod init spiralydata
go mod tidy
go build -ldflags "-H=windowsgui" -o spiralydata.exe .
```

### 📖 Usage

#### Host Mode (Server)
1. Launch the application
2. Click **"Host"**
3. Configure the port (default: 1212)
4. Note the displayed **host identifier**
5. Share your **IP** and **identifier** with clients

#### Client Mode (User)
1. Launch the application
2. Click **"Join"**
3. Enter the server **IP address** (e.g.: 192.168.1.10:1212)
4. Enter the **host identifier**
5. Choose the **synchronization folder**
6. Click **"Connect"**

#### Control Buttons (Client)
| Button | Description |
|--------|-------------|
| **SEND** | Sends your local files to the server |
| **RECEIVE** | Retrieves files from the server |
| **BACKUP** | Creates a complete backup in a `Backup_Spiralydata_DATE` folder |
| **EXPLORE** | Opens the server file explorer |

### 📁 Folder Structure

```
Spiralydata/
├── spiralydata.exe    # Executable
├── Spiralydata/       # Synchronized folder (created automatically)
├── config.json        # Saved configuration
└── logs/              # Activity logs
```

### ⚙️ Configuration

The `config.json` file is created automatically and contains:
- Last server address used
- Last host identifier
- Last synchronization folder
- Selected theme
- Configured filters

### 🔧 Troubleshooting

| Problem | Solution |
|---------|----------|
| Connection refused | Check IP, port and firewall |
| Incorrect identifier | Check the identifier displayed on host side |
| Files not synchronized | Check configured filters |
| Application freezing | Reduce number of files or increase delays |
