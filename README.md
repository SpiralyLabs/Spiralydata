# 🔄 Spiraly Sync

**Synchronisation de fichiers intelligente entre plusieurs ordinateurs**

---

## 📦 Installation

### 1. Télécharger le projet
```bash
git clone https://github.com/Spiralyfox/Spiralydata.git
cd Spiralydata
```

### 2. Compiler

**Windows**
```batch
setup_windows.bat
```

**Linux**
```bash
chmod +x setup_linux.sh
./setup_linux.sh
```

Le logiciel sera compilé et prêt à l'emploi dans le dossier courant.

---

## 🚀 Utilisation

### 🖥️ Mode Hôte (Serveur)

1. Lancez l'application
2. Cliquez sur **"Mode Hôte"**
3. Entrez un **port** (ex: `1234`)
4. Créez un **ID à 6 chiffres** (ex: `123456`)
5. Partagez votre **IP publique**, **port** et l'**ID** avec les utilisateurs

📁 Les fichiers seront dans le dossier `Spiralydata/`

---

### 👤 Mode Utilisateur (Client)

1. Lancez l'application
2. Cliquez sur **"Mode Utilisateur"**
3. Entrez l'**IP du serveur**
4. Entrez le **port**
5. Entrez l'**ID du host**
6. Cochez **"Sauvegarder"** pour garder la config
7. Cliquez sur **"Se connecter"**

📁 Les fichiers seront dans le dossier `Spiralydata/`

---

## ⚙️ Modes de Synchronisation

### 🔴 Mode Manuel (Par défaut)

**Contrôle total sur les transferts :**

- **📥 RECEVOIR** : Télécharge tous les fichiers du serveur
- **📤 ENVOYER** : Envoie vos modifications au serveur
- **🗑️ VIDER LOCAL** : Supprime uniquement vos fichiers locaux (ne touche pas le serveur)

### 🟢 Mode Automatique

**Synchronisation en temps réel :**

- Cliquez sur **"🔄 SYNC AUTO"** pour activer
- Tous les changements sont automatiquement synchronisés
- Les boutons manuels sont désactivés

---

## 🛠️ Dépannage

### ❌ "Connexion impossible"
- Vérifiez que le serveur est démarré
- Vérifiez votre firewall (port ouvert)
- Vérifiez l'IP et le port

### 🚫 "ID incorrect"
- L'ID doit être exactement 6 caractères
- Vérifiez avec l'hôte du serveur

### ⏳ "Opération en cours"
- Attendez la fin de l'opération en cours

---

## 🔒 Sécurité

### Protection des données
- Le bouton **"🗑️ VIDER LOCAL"** ne supprime QUE vos fichiers locaux
- Le serveur et les autres clients ne sont PAS affectés
- Uniquement disponible en mode manuel

### Délais de protection
- 100ms entre chaque transfert de fichier
- Évite la surcharge réseau (Anti auto DDOS)

---

## 📄 Licence

Projet open-source - Spiralydata
Par Spiralyfox | Merci !