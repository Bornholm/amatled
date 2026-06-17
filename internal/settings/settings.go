package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const appName = "amatled"

// LLMSettings contient la configuration du provider LLM.
// APIKey est conservé dans le JSON uniquement pour la migration depuis l'ancien format ;
// après migration, il est toujours vide dans le fichier (stocké dans le keyring).
type LLMSettings struct {
	Provider      string `json:"provider"`
	BaseURL       string `json:"baseURL"`
	APIKey        string `json:"apiKey,omitempty"` // legacy read-only ; vidé à la sauvegarde
	Model         string `json:"model"`
	MaxIterations int    `json:"maxIterations"`
	MaxTokens     int    `json:"maxTokens"`
}

// Profile contient une configuration nommée (LLM + prompt système).
// L'APIKey n'est jamais écrite dans le JSON ; elle est peuplée depuis le keyring au runtime.
type Profile struct {
	Name         string      `json:"name"`
	LLM          LLMSettings `json:"llm"`
	SystemPrompt string      `json:"systemPrompt,omitempty"`
}

// RenderPreset contient une configuration de rendu Amatl nommée.
// RenderConfigPassword est lisible depuis le JSON (RPC entrant) mais jamais écrit sur disque
// car Save() le vide avant sérialisation.
type RenderPreset struct {
	Name                 string `json:"name"`
	RenderConfig         string `json:"renderConfig,omitempty"`
	RenderConfigUsername string `json:"renderConfigUsername,omitempty"`
	RenderConfigPassword string `json:"renderConfigPassword,omitempty"`
}

// Settings contient les préférences persistées de l'application.
type Settings struct {
	LastWorkspace   string    `json:"lastWorkspace"`
	NormalizeOnSave *bool     `json:"normalizeOnSave,omitempty"`
	AutoUpdate      *bool     `json:"autoUpdate,omitempty"`
	Profiles        []Profile `json:"profiles,omitempty"`
	ActiveProfile   string    `json:"activeProfile,omitempty"`
	RenderPresets      []RenderPreset `json:"renderPresets,omitempty"`
	ActiveRenderPreset string         `json:"activeRenderPreset,omitempty"`
	// Champs legacy (conservés pour migration one-shot ; ignorés si Profiles est non vide)
	RenderConfig         string      `json:"renderConfig,omitempty"`
	RenderConfigUsername string      `json:"renderConfigUsername,omitempty"`
	RenderConfigPassword string      `json:"renderConfigPassword,omitempty"` // legacy only
	LLM                  LLMSettings `json:"llm"`                             // APIKey vidé après migration
}

// IsNormalizeOnSave retourne true si la normalisation à la sauvegarde est activée (défaut : true).
func (s *Settings) IsNormalizeOnSave() bool {
	return s.NormalizeOnSave == nil || *s.NormalizeOnSave
}

// IsAutoUpdate retourne true si la vérification automatique des mises à jour est activée (défaut : true).
func (s *Settings) IsAutoUpdate() bool {
	return s.AutoUpdate == nil || *s.AutoUpdate
}

// GetProfile retourne le profil ayant ce nom, ou nil.
func (s *Settings) GetProfile(name string) *Profile {
	for i := range s.Profiles {
		if s.Profiles[i].Name == name {
			return &s.Profiles[i]
		}
	}
	return nil
}

// ResolveProfile retourne le profil actif en suivant la chaîne :
// name fourni → s.ActiveProfile → premier profil → profil synthétique depuis les champs legacy.
func (s *Settings) ResolveProfile(name string) *Profile {
	if name != "" {
		if p := s.GetProfile(name); p != nil {
			return p
		}
	}
	if s.ActiveProfile != "" {
		if p := s.GetProfile(s.ActiveProfile); p != nil {
			return p
		}
	}
	if len(s.Profiles) > 0 {
		return &s.Profiles[0]
	}
	// Fallback : profil synthétique depuis les champs legacy (avant migration)
	return &Profile{
		Name: "",
		LLM:  s.LLM,
	}
}

// GetRenderPreset retourne le préset de rendu ayant ce nom, ou nil.
func (s *Settings) GetRenderPreset(name string) *RenderPreset {
	for i := range s.RenderPresets {
		if s.RenderPresets[i].Name == name {
			return &s.RenderPresets[i]
		}
	}
	return nil
}

// NeedsProfileMigration retourne true si les settings doivent être migrés vers les profils.
func (s *Settings) NeedsProfileMigration() bool {
	return len(s.Profiles) == 0 && (s.LLM.Provider != "" || s.RenderConfig != "")
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

// settingsForJSON est la copie sanitisée utilisée pour la sérialisation :
// tous les champs secrets sont vidés avant écriture sur disque.
type settingsForJSON struct {
	LastWorkspace      string         `json:"lastWorkspace"`
	NormalizeOnSave    *bool          `json:"normalizeOnSave,omitempty"`
	AutoUpdate         *bool          `json:"autoUpdate,omitempty"`
	Profiles           []Profile      `json:"profiles,omitempty"`
	ActiveProfile      string         `json:"activeProfile,omitempty"`
	RenderPresets      []RenderPreset `json:"renderPresets,omitempty"`
	ActiveRenderPreset string         `json:"activeRenderPreset,omitempty"`
}

// Save persiste les settings dans le fichier de configuration utilisateur.
// Les secrets (APIKey, RenderConfigPassword) ne sont jamais écrits.
func (s *Settings) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Copie profonde en vidant les champs secrets
	profiles := make([]Profile, len(s.Profiles))
	for i, p := range s.Profiles {
		profiles[i] = p
		profiles[i].LLM.APIKey = ""
	}
	renderPresets := make([]RenderPreset, len(s.RenderPresets))
	for i, rp := range s.RenderPresets {
		renderPresets[i] = rp
		renderPresets[i].RenderConfigPassword = ""
	}

	toSave := settingsForJSON{
		LastWorkspace:      s.LastWorkspace,
		NormalizeOnSave:    s.NormalizeOnSave,
		AutoUpdate:         s.AutoUpdate,
		Profiles:           profiles,
		ActiveProfile:      s.ActiveProfile,
		RenderPresets:      renderPresets,
		ActiveRenderPreset: s.ActiveRenderPreset,
	}

	data, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
