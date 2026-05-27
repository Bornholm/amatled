package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const appName = "amatled"

// LLMSettings contient la configuration du provider LLM.
type LLMSettings struct {
	Provider      string `json:"provider"`
	BaseURL       string `json:"baseURL"`
	APIKey        string `json:"apiKey"`
	Model         string `json:"model"`
	MaxIterations int    `json:"maxIterations"`
	MaxTokens     int    `json:"maxTokens"`
}

// Settings contient les préférences persistées de l'application.
type Settings struct {
	LastWorkspace        string      `json:"lastWorkspace"`
	NormalizeOnSave      *bool       `json:"normalizeOnSave,omitempty"`
	AutoUpdate           *bool       `json:"autoUpdate,omitempty"`
	RenderConfig         string      `json:"renderConfig,omitempty"`
	RenderConfigUsername string      `json:"renderConfigUsername,omitempty"`
	RenderConfigPassword string      `json:"renderConfigPassword,omitempty"`
	LLM                  LLMSettings `json:"llm"`
}

// IsNormalizeOnSave retourne true si la normalisation à la sauvegarde est activée (défaut : true).
func (s *Settings) IsNormalizeOnSave() bool {
	return s.NormalizeOnSave == nil || *s.NormalizeOnSave
}

// IsAutoUpdate retourne true si la vérification automatique des mises à jour est activée (défaut : true).
func (s *Settings) IsAutoUpdate() bool {
	return s.AutoUpdate == nil || *s.AutoUpdate
}

// LLMConfigured retourne true si une configuration LLM minimale est présente.
func (s *Settings) LLMConfigured() bool {
	return s.LLM.Provider != "" && s.LLM.APIKey != "" && s.LLM.Model != ""
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, appName, "settings.json"), nil
}

// Load charge les settings depuis le fichier de configuration utilisateur.
// Retourne des settings vides (sans erreur) si le fichier n'existe pas encore.
func Load() (*Settings, error) {
	path, err := configPath()
	if err != nil {
		return &Settings{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Settings{}, nil
	}
	if err != nil {
		return &Settings{}, fmt.Errorf("read settings: %w", err)
	}
	s := &Settings{}
	if err := json.Unmarshal(data, s); err != nil {
		return &Settings{}, fmt.Errorf("parse settings: %w", err)
	}
	return s, nil
}

// Save persiste les settings dans le fichier de configuration utilisateur.
func (s *Settings) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
