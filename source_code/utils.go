package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// FormatFileSize formate une taille de fichier en unités lisibles
func FormatFileSize(size int64) string {
	if size < 0 {
		return "0 B"
	}
	
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// readFileWithRetry lit un fichier avec plusieurs tentatives
func readFileWithRetry(path string) ([]byte, error) {
	var data []byte
	var err error

	for i := 0; i < 5; i++ {
		data, err = os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		if os.IsNotExist(err) {
			return nil, err
		}
		time.Sleep(50 * time.Millisecond)
	}

	return nil, err
}

func getExecutableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exePath)
}

// normalizePath normalise un chemin pour une utilisation cross-platform
func normalizePath(path string) string {
	// Convertir les séparateurs en slash (format interne)
	normalized := filepath.ToSlash(path)
	// Nettoyer le chemin
	normalized = filepath.Clean(normalized)
	// Reconvertir en slash
	normalized = filepath.ToSlash(normalized)
	return normalized
}

// toLocalPath convertit un chemin interne (slash) vers le format local
func toLocalPath(path string) string {
	return filepath.FromSlash(path)
}

// isValidFileName vérifie si un nom de fichier est valide sur la plateforme actuelle
func isValidFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	// Caractères invalides sur toutes les plateformes
	invalidChars := []string{"\x00"}
	for _, char := range invalidChars {
		if strings.Contains(name, char) {
			return false
		}
	}

	// Caractères invalides spécifiques à Windows
	if runtime.GOOS == "windows" {
		windowsInvalidChars := []string{"<", ">", ":", "\"", "|", "?", "*"}
		for _, char := range windowsInvalidChars {
			if strings.Contains(name, char) {
				return false
			}
		}

		// Noms réservés Windows
		reservedNames := []string{
			"CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		}
		upperName := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
		for _, reserved := range reservedNames {
			if upperName == reserved {
				return false
			}
		}
	}

	return true
}

// sanitizeFileName nettoie un nom de fichier pour le rendre valide
func sanitizeFileName(name string) string {
	if runtime.GOOS == "windows" {
		// Remplacer les caractères invalides Windows
		replacer := strings.NewReplacer(
			"<", "_", ">", "_", ":", "_", "\"", "_",
			"|", "_", "?", "_", "*", "_",
		)
		name = replacer.Replace(name)
	}
	return name
}

// getFilePermissions retourne les permissions appropriées selon l'OS
func getFilePermissions() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0666
	}
	return 0644
}

// getDirPermissions retourne les permissions de dossier appropriées selon l'OS
func getDirPermissions() os.FileMode {
	if runtime.GOOS == "windows" {
		return 0777
	}
	return 0755
}

// isHiddenFile vérifie si un fichier est caché
func isHiddenFile(name string) bool {
	if name == "" {
		return false
	}

	// Sur Unix/Linux, les fichiers cachés commencent par un point
	if runtime.GOOS != "windows" {
		return strings.HasPrefix(name, ".")
	}

	// Sur Windows, il faudrait vérifier l'attribut du fichier
	// Pour simplifier, on vérifie juste le préfixe point
	return strings.HasPrefix(name, ".")
}

// copyDirRecursive copie récursivement un dossier vers une destination
func copyDirRecursive(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	if !srcInfo.IsDir() {
		return fmt.Errorf("source n'est pas un dossier")
	}
	
	// Créer le dossier destination
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		
		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	
	return nil
} 