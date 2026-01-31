package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// FilterMode représente le mode de filtrage
type FilterMode int

const (
	FilterModeBlacklist FilterMode = iota // Extensions à exclure
	FilterModeWhitelist                   // Extensions autorisées uniquement
)

// ExtensionFilter gère le filtrage par extension
type ExtensionFilter struct {
	Extensions    map[string]bool `json:"extensions"`     // Extensions actives (normalisées)
	Mode          FilterMode      `json:"mode"`           // Blacklist ou Whitelist
	Enabled       bool            `json:"enabled"`        // Filtrage actif ou non
	IgnoredCount  int             `json:"-"`              // Compteur de fichiers ignorés
	mu            sync.RWMutex    `json:"-"`
}

// SizeFilter gère le filtrage par taille
type SizeFilter struct {
	Enabled    bool  `json:"enabled"`
	MinSize    int64 `json:"min_size"`    // Taille minimale en octets (0 = pas de min)
	MaxSize    int64 `json:"max_size"`    // Taille maximale en octets (0 = pas de max)
}

// PathFilter gère le filtrage par chemin
type PathFilter struct {
	Enabled         bool     `json:"enabled"`
	ExcludedFolders []string `json:"excluded_folders"` // Dossiers à exclure
	ExcludeHidden   bool     `json:"exclude_hidden"`   // Exclure fichiers cachés
	ExcludeSymlinks bool     `json:"exclude_symlinks"` // Exclure liens symboliques
}

// FileFilters regroupe tous les filtres
type FileFilters struct {
	Extension ExtensionFilter `json:"extension"`
	Size      SizeFilter      `json:"size"`
	Path      PathFilter      `json:"path"`
}

// FilterConfig est la configuration complète des filtres
type FilterConfig struct {
	Filters        FileFilters `json:"filters"`
	configPath     string      `json:"-"`
	mu             sync.RWMutex `json:"-"`
}

// Regex pour valider les extensions
var extensionRegex = regexp.MustCompile(`^\.?[a-zA-Z0-9]+$`)

// Extensions suggérées par catégorie
var SuggestedExtensions = map[string][]string{
	"Images":      {".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff"},
	"Vidéos":      {".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
	"Audio":       {".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a"},
	"Archives":    {".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz"},
	"Temporaires": {".tmp", ".temp", ".cache", ".bak", ".swp", ".swo"},
	"Système":     {".exe", ".dll", ".so", ".dylib", ".sys", ".drv"},
	"Dev":         {".o", ".obj", ".pyc", ".class", ".pdb"},
}

// Dossiers couramment exclus
var CommonExcludedFolders = []string{
	"node_modules",
	".git",
	".svn",
	".hg",
	"__pycache__",
	".idea",
	".vscode",
	"vendor",
	"build",
	"dist",
	"bin",
	"obj",
	".cache",
	".tmp",
}

// NewFilterConfig crée une nouvelle configuration de filtres
func NewFilterConfig() *FilterConfig {
	return &FilterConfig{
		Filters: FileFilters{
			Extension: ExtensionFilter{
				Extensions: make(map[string]bool),
				Mode:       FilterModeBlacklist,
				Enabled:    false,
			},
			Size: SizeFilter{
				Enabled: false,
				MinSize: 0,
				MaxSize: 0,
			},
			Path: PathFilter{
				Enabled:         true,
				ExcludedFolders: []string{".git", "node_modules", "__pycache__"},
				ExcludeHidden:   false,
				ExcludeSymlinks: true,
			},
		},
	}
}

// NormalizeExtension normalise une extension
func NormalizeExtension(ext string) (string, error) {
	ext = strings.TrimSpace(ext)
	
	// Vérifier le format avec regex
	if !extensionRegex.MatchString(ext) {
		return "", fmt.Errorf("format d'extension invalide: %s", ext)
	}
	
	// Ajouter le point si manquant
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	
	// Convertir en minuscules
	ext = strings.ToLower(ext)
	
	return ext, nil
}

// ValidateExtension vérifie si une extension est valide
func ValidateExtension(ext string) bool {
	_, err := NormalizeExtension(ext)
	return err == nil
}

// AddExtension ajoute une extension au filtre
func (ef *ExtensionFilter) AddExtension(ext string) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	
	normalized, err := NormalizeExtension(ext)
	if err != nil {
		return err
	}
	
	// Vérifier les doublons
	if ef.Extensions[normalized] {
		return fmt.Errorf("extension déjà présente: %s", normalized)
	}
	
	// Limite de 100 extensions
	if len(ef.Extensions) >= 100 {
		return fmt.Errorf("limite de 100 extensions atteinte")
	}
	
	ef.Extensions[normalized] = true
	return nil
}

// RemoveExtension supprime une extension du filtre
func (ef *ExtensionFilter) RemoveExtension(ext string) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	
	normalized, err := NormalizeExtension(ext)
	if err != nil {
		return err
	}
	
	if !ef.Extensions[normalized] {
		return fmt.Errorf("extension non trouvée: %s", normalized)
	}
	
	delete(ef.Extensions, normalized)
	return nil
}

// GetExtensions retourne la liste triée des extensions
func (ef *ExtensionFilter) GetExtensions() []string {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	
	result := make([]string, 0, len(ef.Extensions))
	for ext := range ef.Extensions {
		result = append(result, ext)
	}
	sort.Strings(result)
	return result
}

// SetMode définit le mode de filtrage
func (ef *ExtensionFilter) SetMode(mode FilterMode) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.Mode = mode
}

// GetMode retourne le mode actuel
func (ef *ExtensionFilter) GetMode() FilterMode {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	return ef.Mode
}

// IsEnabled vérifie si le filtrage est actif
func (ef *ExtensionFilter) IsEnabled() bool {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	return ef.Enabled
}

// SetEnabled active ou désactive le filtrage
func (ef *ExtensionFilter) SetEnabled(enabled bool) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.Enabled = enabled
}

// ShouldFilter vérifie si un fichier doit être filtré
// IMPORTANT: Cette fonction ne doit être appelée que pour les FICHIERS, pas les dossiers
func (ef *ExtensionFilter) ShouldFilter(filename string) bool {
	ef.mu.RLock()
	defer ef.mu.RUnlock()

	if !ef.Enabled || len(ef.Extensions) == 0 {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		// Fichiers sans extension: ne pas filtrer (pour éviter problèmes)
		return false
	}

	hasExtension := ef.Extensions[ext]

	if ef.Mode == FilterModeBlacklist {
		// En mode blacklist, filtrer si l'extension est dans la liste
		return hasExtension
	}
	// En mode whitelist, filtrer si l'extension N'EST PAS dans la liste
	return !hasExtension
}

// IncrementIgnored incrémente le compteur de fichiers ignorés
func (ef *ExtensionFilter) IncrementIgnored() {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.IgnoredCount++
}

// ResetIgnoredCount remet le compteur à zéro
func (ef *ExtensionFilter) ResetIgnoredCount() {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.IgnoredCount = 0
}

// GetIgnoredCount retourne le nombre de fichiers ignorés
func (ef *ExtensionFilter) GetIgnoredCount() int {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	return ef.IgnoredCount
}

// AddSuggestedCategory ajoute toutes les extensions d'une catégorie
func (ef *ExtensionFilter) AddSuggestedCategory(category string) error {
	extensions, ok := SuggestedExtensions[category]
	if !ok {
		return fmt.Errorf("catégorie inconnue: %s", category)
	}
	
	for _, ext := range extensions {
		ef.AddExtension(ext) // Ignorer les erreurs de doublons
	}
	return nil
}

// Clear supprime toutes les extensions
func (ef *ExtensionFilter) Clear() {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.Extensions = make(map[string]bool)
}

// ShouldFilterBySize vérifie si un fichier doit être filtré par taille
func (sf *SizeFilter) ShouldFilter(size int64) bool {
	if !sf.Enabled {
		return false
	}
	
	if sf.MinSize > 0 && size < sf.MinSize {
		return true
	}
	if sf.MaxSize > 0 && size > sf.MaxSize {
		return true
	}
	return false
}

// ShouldFilterByPath vérifie si un chemin doit être filtré
func (pf *PathFilter) ShouldFilter(path string) bool {
	if !pf.Enabled {
		return false
	}
	
	// Normaliser le chemin
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		// Vérifier les fichiers cachés
		if pf.ExcludeHidden && strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return true
		}
		
		// Vérifier les dossiers exclus
		for _, excluded := range pf.ExcludedFolders {
			if strings.EqualFold(part, excluded) {
				return true
			}
		}
	}
	
	return false
}

// AddExcludedFolder ajoute un dossier à exclure
func (pf *PathFilter) AddExcludedFolder(folder string) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return
	}
	
	// Vérifier si déjà présent
	for _, f := range pf.ExcludedFolders {
		if strings.EqualFold(f, folder) {
			return
		}
	}
	
	pf.ExcludedFolders = append(pf.ExcludedFolders, folder)
}

// RemoveExcludedFolder supprime un dossier de la liste
func (pf *PathFilter) RemoveExcludedFolder(folder string) {
	for i, f := range pf.ExcludedFolders {
		if strings.EqualFold(f, folder) {
			pf.ExcludedFolders = append(pf.ExcludedFolders[:i], pf.ExcludedFolders[i+1:]...)
			return
		}
	}
}

// ShouldFilterFile vérifie si un fichier doit être filtré (toutes règles)
// Les dossiers ne sont JAMAIS filtrés par extension (seulement par chemin)
func (fc *FilterConfig) ShouldFilterFile(path string, size int64, isDir bool) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Les dossiers ne sont filtrés QUE par chemin (jamais par extension)
	if isDir {
		return fc.Filters.Path.ShouldFilter(path)
	}

	// Vérifier le chemin d'abord
	if fc.Filters.Path.ShouldFilter(path) {
		return true
	}

	// Vérifier l'extension (seulement pour les fichiers)
	if fc.Filters.Extension.ShouldFilter(path) {
		fc.Filters.Extension.IncrementIgnored()
		return true
	}

	// Vérifier la taille (seulement pour les fichiers)
	if fc.Filters.Size.ShouldFilter(size) {
		return true
	}

	return false
}

// Save sauvegarde la configuration
func (fc *FilterConfig) Save(path string) error {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	
	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// Load charge la configuration
func (fc *FilterConfig) Load(path string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	
	return json.Unmarshal(data, fc)
}

// ToJSON convertit en JSON pour transmission
func (fc *FilterConfig) ToJSON() ([]byte, error) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return json.Marshal(fc.Filters)
}

// FromJSON charge depuis JSON
func (fc *FilterConfig) FromJSON(data []byte) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return json.Unmarshal(data, &fc.Filters)
}

// GetSummary retourne un résumé des filtres actifs
func (fc *FilterConfig) GetSummary() string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	
	var parts []string
	
	// Extensions
	if fc.Filters.Extension.Enabled && len(fc.Filters.Extension.Extensions) > 0 {
		exts := fc.Filters.Extension.GetExtensions()
		mode := "exclues"
		if fc.Filters.Extension.Mode == FilterModeWhitelist {
			mode = "autorisées"
		}
		if len(exts) <= 3 {
			parts = append(parts, fmt.Sprintf("Extensions %s: %s", mode, strings.Join(exts, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("Extensions %s: %d types", mode, len(exts)))
		}
	}
	
	// Taille
	if fc.Filters.Size.Enabled {
		if fc.Filters.Size.MaxSize > 0 {
			parts = append(parts, fmt.Sprintf("Max: %s", formatFileSize(fc.Filters.Size.MaxSize)))
		}
		if fc.Filters.Size.MinSize > 0 {
			parts = append(parts, fmt.Sprintf("Min: %s", formatFileSize(fc.Filters.Size.MinSize)))
		}
	}
	
	// Dossiers exclus
	if fc.Filters.Path.Enabled && len(fc.Filters.Path.ExcludedFolders) > 0 {
		parts = append(parts, fmt.Sprintf("Dossiers exclus: %d", len(fc.Filters.Path.ExcludedFolders)))
	}
	
	if len(parts) == 0 {
		return "Aucun filtre actif"
	}
	
	return strings.Join(parts, " | ")
}

// formatFileSize formate une taille en octets
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// Global filter config
var globalFilterConfig *FilterConfig

// GetFilterConfig retourne la configuration globale des filtres
func GetFilterConfig() *FilterConfig {
	if globalFilterConfig == nil {
		globalFilterConfig = NewFilterConfig()
		// Charger les filtres sauvegardés depuis la config
		LoadFiltersFromConfig(globalFilterConfig)
	}
	return globalFilterConfig
}

// InitFilterConfig initialise la configuration des filtres
func InitFilterConfig() {
	globalFilterConfig = NewFilterConfig()
	// Charger les filtres sauvegardés depuis la config
	LoadFiltersFromConfig(globalFilterConfig)
}
