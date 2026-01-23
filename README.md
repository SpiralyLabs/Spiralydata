# Spiralydata

**Real-time file synchronization application with graphical interface**

---

## English Version

### Description

Spiralydata is a cross-platform file synchronization application that allows real-time sharing of files between multiple computers. The application features an intuitive graphical interface and supports both automatic and manual synchronization modes.

### Key Features

- 🔄 **Real-time synchronization**: Automatic or manual file synchronization
- 🌐 **Server/Client architecture**: One host shares files with multiple users
- 📂 **File explorer**: Browse and download specific files from the server
- 🗑️ **Remote management**: Delete directories directly on the server
- 💾 **Configuration saving**: Auto-connect on startup
- 🎨 **Dark theme interface**: Modern and clean design

### System Requirements

- **Windows**: Windows 10/11
- **Linux**: Any modern distribution (Ubuntu, Debian, Fedora, Arch, etc.)
- **Network**: Local network or internet connection

### Installation

#### Windows

1. Download or clone the repository
2. Open a terminal in the project folder
3. Run the setup script:
```bash
setup_windows.bat
```
4. The executable `spiralydata.exe` will be created
5. Double-click to launch

#### Linux

1. Download or clone the repository
2. Navigate to the project folder:
```bash
cd /path/to/spiralydata
```
3. Make the setup script executable:
```bash
chmod +x setup_linux.sh
```
4. Run the installation:
```bash
./setup_linux.sh
```
5. Launch the application:
```bash
./spiralydata
```

### Usage

#### Host Mode (Server)

1. Launch the application
2. Click **"Host Mode"**
3. Enter:
   - Port number (example: 1234)
   - 6-digit host ID (example: 123456)
4. Click **"Start Server"**
5. Share your **public IP** and **host ID** with users

#### User Mode (Client)

1. Launch the application
2. Click **"User Mode"**
3. Enter:
   - Server IP address
   - Port number
   - Host ID (provided by the host)
   - Sync directory
4. Click **"Connect"**

#### Synchronization Modes

**Manual Mode** (default):
- Files are queued but not automatically synced
- Use **RECEIVE** to download files from server
- Use **SEND** to upload local changes
- Use **EXPLORER** to browse and download specific files

**Auto Sync Mode**:
- Automatic real-time synchronization
- All changes are immediately synced
- Manual controls are disabled during auto-sync

#### File Explorer

1. Click **EXPLORER** (in manual mode)
2. Browse the server directory structure
3. Select files/folders with checkboxes
4. Click **Download Selection**
5. Choose destination (sync folder or custom location)

**Delete Directory**:
- When inside a non-root folder, use the **Delete Directory** button
- Confirms before permanently deleting from the server

### Advanced Features

- **Clear Local**: Delete all local files while keeping server files
- **Auto-connect**: Save configuration for automatic connection on startup
- **Persistent selections**: File selections are saved when browsing folders

### Network Configuration

**Local Network**:
- Use the local IP displayed by the server (example: 192.168.1.100)

**Internet**:
- Use the public IP displayed by the server
- Configure port forwarding on your router if needed

### Troubleshooting

**Connection failed**:
- Verify IP address and port
- Check firewall settings
- Ensure the host ID is correct

**Compilation errors (Linux)**:
- Install required dependencies (see logs in `setup_crash_log/`)
- Ensure Go, GCC, and system libraries are installed

**Freeze or crash**:
- Check the logs in the graphical interface
- Disable auto-sync before performing manual operations

---

## Version Française

### Description

Spiralydata est une application de synchronisation de fichiers multiplateforme permettant le partage de fichiers en temps réel entre plusieurs ordinateurs. L'application dispose d'une interface graphique intuitive et prend en charge les modes de synchronisation automatique et manuel.

### Fonctionnalités Principales

- 🔄 **Synchronisation en temps réel**: Synchronisation automatique ou manuelle des fichiers
- 🌐 **Architecture serveur/client**: Un hôte partage des fichiers avec plusieurs utilisateurs
- 📂 **Explorateur de fichiers**: Parcourir et télécharger des fichiers spécifiques du serveur
- 🗑️ **Gestion à distance**: Supprimer des répertoires directement sur le serveur
- 💾 **Sauvegarde de configuration**: Connexion automatique au démarrage
- 🎨 **Interface thème sombre**: Design moderne et épuré

### Prérequis Système

- **Windows**: Windows 10/11
- **Linux**: Toute distribution moderne (Ubuntu, Debian, Fedora, Arch, etc.)
- **Réseau**: Réseau local ou connexion internet

### Installation

#### Windows

1. Téléchargez ou clonez le dépôt
2. Ouvrez un terminal dans le dossier du projet
3. Exécutez le script d'installation:
```bash
setup_windows.bat
```
4. L'exécutable `spiralydata.exe` sera créé
5. Double-cliquez pour lancer

#### Linux

1. Téléchargez ou clonez le dépôt
2. Naviguez vers le dossier du projet:
```bash
cd /chemin/vers/spiralydata
```
3. Rendez le script d'installation exécutable:
```bash
chmod +x setup_linux.sh
```
4. Lancez l'installation:
```bash
./setup_linux.sh
```
5. Lancez l'application:
```bash
./spiralydata
```

### Utilisation

#### Mode Hôte (Serveur)

1. Lancez l'application
2. Cliquez sur **"Mode Hôte (Host)"**
3. Entrez:
   - Numéro de port (exemple: 1234)
   - ID du serveur à 6 chiffres (exemple: 123456)
4. Cliquez sur **"Démarrer le serveur"**
5. Partagez votre **IP publique** et **l'ID hôte** avec les utilisateurs

#### Mode Utilisateur (Client)

1. Lancez l'application
2. Cliquez sur **"Mode Utilisateur (User)"**
3. Entrez:
   - Adresse IP du serveur
   - Numéro de port
   - ID du host (fourni par l'hôte)
   - Dossier de synchronisation
4. Cliquez sur **"Se connecter"**

#### Modes de Synchronisation

**Mode Manuel** (par défaut):
- Les fichiers sont mis en file d'attente mais pas automatiquement synchronisés
- Utilisez **RECEVOIR** pour télécharger les fichiers du serveur
- Utilisez **ENVOYER** pour envoyer les modifications locales
- Utilisez **EXPLORATEUR** pour parcourir et télécharger des fichiers spécifiques

**Mode Sync Auto**:
- Synchronisation automatique en temps réel
- Tous les changements sont immédiatement synchronisés
- Les contrôles manuels sont désactivés pendant la sync auto

#### Explorateur de Fichiers

1. Cliquez sur **EXPLORATEUR** (en mode manuel)
2. Parcourez la structure du répertoire du serveur
3. Sélectionnez des fichiers/dossiers avec les cases à cocher
4. Cliquez sur **Télécharger la sélection**
5. Choisissez la destination (dossier de sync ou emplacement personnalisé)

**Supprimer un Répertoire**:
- Lorsque vous êtes dans un dossier non-racine, utilisez le bouton **Delete Directory**
- Confirme avant de supprimer définitivement du serveur

### Fonctionnalités Avancées

- **Vider Local**: Supprime tous les fichiers locaux tout en conservant les fichiers du serveur
- **Connexion automatique**: Sauvegarde la configuration pour une connexion automatique au démarrage
- **Sélections persistantes**: Les sélections de fichiers sont sauvegardées lors de la navigation

### Configuration Réseau

**Réseau Local**:
- Utilisez l'IP locale affichée par le serveur (exemple: 192.168.1.100)

**Internet**:
- Utilisez l'IP publique affichée par le serveur
- Configurez la redirection de port sur votre routeur si nécessaire

### Dépannage

**Échec de connexion**:
- Vérifiez l'adresse IP et le port
- Vérifiez les paramètres du pare-feu
- Assurez-vous que l'ID hôte est correct

**Erreurs de compilation (Linux)**:
- Installez les dépendances requises (voir les logs dans `setup_crash_log/`)
- Assurez-vous que Go, GCC et les bibliothèques système sont installés

**Freeze ou crash**:
- Consultez les logs dans l'interface graphique
- Désactivez la sync auto avant d'effectuer des opérations manuelles

---

## License

This project is open source.

## Contributing

Contributions are welcome ! Feel free to open issues or submit pull requests.

## Support

For questions or issues, please open an issue on the repository.