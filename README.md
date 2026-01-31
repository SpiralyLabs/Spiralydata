# SpiralData

Application de synchronisation de fichiers en temps reel entre un serveur (Host) et des clients (Users).

## Presentation

SpiralData permet de synchroniser des fichiers et dossiers entre plusieurs machines a travers le reseau. Un utilisateur demarre un serveur (mode Host) et d'autres utilisateurs se connectent en tant que clients (mode User) pour synchroniser leurs fichiers.

## Modes de fonctionnement

### Mode Host (Serveur)

Le Host heberge les fichiers de reference. Il ecoute les connexions entrantes et synchronise les modifications avec tous les clients connectes.

**Configuration requise :**
- **Port** : Port TCP sur lequel le serveur ecoute (ex: 8080, 3000, etc.)
- **ID Host** : Identifiant unique (6 caracteres minimum) que les clients devront fournir pour se connecter

**Dossier de synchronisation :** Les fichiers sont stockes dans un dossier `Spiralydata` cree automatiquement a cote de l'executable.

### Mode User (Client)

Le User se connecte a un Host pour synchroniser ses fichiers.

**Configuration requise :**
- **Adresse du serveur** : IP et port du Host (ex: `192.168.1.100:8080` ou `monserveur.com:3000`)
- **ID Host** : L'identifiant defini par le Host
- **Dossier de synchronisation** : Dossier local ou les fichiers seront synchronises

## Fonctionnalites

### Synchronisation

- **Sync Auto** : Active la synchronisation automatique en temps reel. Toute modification locale est immediatement envoyee au serveur, et les modifications du serveur sont appliquees automatiquement.

- **Mode Manuel** : Permet de controler precisement quand synchroniser :
  - **RECEVOIR** : Recupere tous les fichiers du serveur
  - **ENVOYER** : Envoie toutes les modifications locales au serveur
  - **VIDER LOCAL** : Supprime tous les fichiers locaux (utile pour repartir a zero)

### File Transfert

Affiche les actions en attente d'envoi au serveur. Seules les modifications manuelles locales (creation, modification, suppression de fichiers) sont listees ici. Les fichiers recus depuis le serveur ou telecharges via l'explorateur ne sont pas ajoutes a cette liste car ils sont deja synchronises.

### Explorateur de fichiers

Permet de naviguer dans l'arborescence des fichiers du serveur, de selectionner des fichiers specifiques a telecharger, et de previsualiser certains types de fichiers.

### Telecharger une backup

Disponible dans les modes Host et User, ce bouton permet de telecharger une copie complete de tous les fichiers synchronises vers un dossier de votre choix. Utile pour creer une sauvegarde locale de vos donnees.

### Filtres

Configurez des filtres pour exclure certains fichiers de la synchronisation :
- **Par extension** : Exclure des types de fichiers (ex: `.tmp`, `.log`)
- **Par chemin** : Exclure des dossiers ou fichiers specifiques
- **Par taille** : Definir une taille minimale ou maximale

### Securite (Host uniquement)

- **Whitelist IP** : Restreindre les connexions a certaines adresses IP uniquement
- Support des plages IP (ex: `192.168.1.0/24`)

## Utilisation basique

### Demarrer un serveur

1. Lancer l'application
2. Cliquer sur "Mode Hote (Host)"
3. Entrer un port (ex: `8080`)
4. Entrer un ID Host (ex: `monid123`)
5. Cliquer sur "Demarrer le serveur"
6. Communiquer l'adresse IP et l'ID aux utilisateurs

### Se connecter en tant que client

1. Lancer l'application
2. Cliquer sur "Mode Utilisateur (User)"
3. Entrer l'adresse du serveur (ex: `192.168.1.100:8080`)
4. Entrer l'ID Host fourni par le serveur
5. Choisir le dossier de synchronisation local
6. Cliquer sur "Se connecter"

### Synchroniser des fichiers

**Avec Sync Auto :**
- Activez le bouton "SYNC AUTO"
- Toutes les modifications sont synchronisees automatiquement

**En mode manuel :**
1. Modifiez vos fichiers localement
2. Les modifications apparaissent dans "File transfert"
3. Cliquez sur "ENVOYER" pour envoyer au serveur
4. Cliquez sur "RECEVOIR" pour recuperer les fichiers du serveur

### Creer une backup

1. Cliquez sur "Telecharger une backup"
2. Choisissez le dossier de destination
3. La copie complete des fichiers sera effectuee

## Configuration

### Fichier de configuration

Les parametres sont sauvegardes dans `spiraly_config.json` a cote de l'executable.

### Parametres disponibles

- Theme (clair/sombre)
- Taille de la fenetre
- Barre de statut
- Nombre maximum de logs

## Raccourcis clavier

- **Ctrl+T** : Changer de theme
- **F11** : Plein ecran

## Notes techniques

- Communication via WebSocket pour les echanges en temps reel
- Les fichiers sont encodes en Base64 pour le transfert
- Support de la compression gzip pour les fichiers volumineux
- Deconnexion: revient au menu principal sans fermer l'application

## Depannage

### "Connexion impossible"
- Verifiez que le Host est bien demarre
- Verifiez l'adresse IP et le port
- Verifiez que le pare-feu autorise les connexions

### "ID incorrect"
- Verifiez l'ID Host aupres de l'administrateur du serveur

### Fichiers non synchronises
- Verifiez les filtres configures
- Verifiez que vous n'etes pas en mode Sync Auto (les actions ne sont pas listees en Sync Auto car elles sont envoyees immediatement)

---

Developpe avec Go et Fyne
