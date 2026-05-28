package render

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	amatlrender "github.com/Bornholm/amatl/pkg/command/cli/render"
	"github.com/Bornholm/amatl/pkg/pipeline"
	"github.com/Bornholm/amatl/pkg/resolver"
	_ "github.com/Bornholm/amatl/pkg/resolver/all"
	"gopkg.in/yaml.v3"
)

// renderConfig regroupe tous les paramètres extraits d'un fichier de config amatl.
type renderConfig struct {
	HTMLLayout             string   `yaml:"html-layout"`
	HTMLLayoutVarsURL      string   `yaml:"html-layout-vars"`
	VarsURL                string   `yaml:"vars"`
	TemplateLeftDelimiter  string   `yaml:"template-left-delimiter"`
	TemplateRightDelimiter string   `yaml:"template-right-delimiter"`
	LinkReplacements       []string `yaml:"link-replacements"`
	PDFMarginTop           float64  `yaml:"pdf-margin-top"`
	PDFMarginLeft          float64  `yaml:"pdf-margin-left"`
	PDFMarginRight         float64  `yaml:"pdf-margin-right"`
	PDFMarginBottom        float64  `yaml:"pdf-margin-bottom"`
	PDFScale               float64  `yaml:"pdf-scale"`
	PDFTimeout             string   `yaml:"pdf-timeout"`
	PDFBackground          bool     `yaml:"pdf-background"`
	PDFExecPath            string   `yaml:"pdf-exec-path"`
	PDFDisplayHeaderFooter bool     `yaml:"pdf-display-header-footer"`
	PDFHeaderTemplate      string   `yaml:"pdf-header-template"`
	PDFFooterTemplate      string   `yaml:"pdf-footer-template"`
	PDFNoSandbox           bool     `yaml:"pdf-no-sandbox"`
}

// defaultRenderConfig retourne une config pré-remplie avec les mêmes défauts que le CLI amatl.
// yaml.v3.Unmarshal ne touche que les clés présentes dans le fichier,
// donc les valeurs absentes conservent ces défauts.
func defaultRenderConfig() *renderConfig {
	return &renderConfig{
		HTMLLayout:             "amatl://document.html",
		PDFMarginTop:           amatlrender.DefaultPDFMargin,
		PDFMarginLeft:          amatlrender.DefaultPDFMargin,
		PDFMarginRight:         amatlrender.DefaultPDFMargin,
		PDFMarginBottom:        amatlrender.DefaultPDFMargin,
		PDFScale:               amatlrender.DefaultPDFScale,
		PDFTimeout:             amatlrender.DefaultPDFTimeout.String(),
		PDFBackground:          amatlrender.DefaultPDFBackground,
		PDFDisplayHeaderFooter: false,
		PDFFooterTemplate:      amatlrender.DefaultPDFFooterTemplate,
		PDFNoSandbox:           true, // toujours actif dans amatled (Linux headless)
	}
}

// NormalizeMarkdown passe le contenu Markdown par le pipeline de normalisation
// amatl (parse AST → re-rendu canonique) sans déplier les directives :include.
func NormalizeMarkdown(ctx context.Context, content []byte, filePath, workspaceRoot string) ([]byte, error) {
	sourceDir := filepath.Dir(filePath)
	if sourceDir == "." || sourceDir == "" {
		sourceDir = workspaceRoot
	} else if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(workspaceRoot, sourceDir)
	}

	sourcePath := resolver.Path(sourceDir)
	t := pipeline.Pipeline(
		amatlrender.MarkdownMiddleware(
			amatlrender.WithSourcePath(sourcePath),
		),
	)

	payload := pipeline.NewPayload(content)
	if err := t.Transform(ctx, payload); err != nil {
		return nil, err
	}
	return payload.GetData(), nil
}

// injectBasicAuth ajoute les credentials dans l'URL si elle est http/https.
func injectBasicAuth(rawURL, username, password string) string {
	if username == "" && password == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return rawURL
	}
	u.User = url.UserPassword(username, password)
	return u.String()
}

// resolveAndInjectAuth résout les chemins relatifs par rapport à baseDir,
// puis injecte Basic Auth pour les URLs http/https.
func resolveAndInjectAuth(baseDir resolver.Path, rawURL, username, password string) string {
	if rawURL == "" {
		return ""
	}
	p := resolver.Path(rawURL)
	if !p.IsURL() && !p.IsAbs() {
		rawURL = baseDir.JoinPath(rawURL).String()
	}
	return injectBasicAuth(rawURL, username, password)
}

// loadConfig charge un fichier de configuration amatl (YAML) et retourne
// tous les paramètres, avec résolution des URLs relatives et injection Basic Auth.
func loadConfig(ctx context.Context, configURL, username, password string) (*renderConfig, error) {
	authedURL := injectBasicAuth(configURL, username, password)

	reader, err := resolver.Resolve(ctx, authedURL)
	if err != nil {
		return nil, fmt.Errorf("resolve config: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := defaultRenderConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	configDir := resolver.Path(configURL).Dir()
	cfg.HTMLLayout = resolveAndInjectAuth(configDir, cfg.HTMLLayout, username, password)
	cfg.HTMLLayoutVarsURL = resolveAndInjectAuth(configDir, cfg.HTMLLayoutVarsURL, username, password)
	cfg.VarsURL = resolveAndInjectAuth(configDir, cfg.VarsURL, username, password)

	return cfg, nil
}

// loadJSONVars charge un fichier JSON depuis une URL et retourne map[string]any.
// Retourne une map vide si l'URL est vide.
func loadJSONVars(ctx context.Context, rawURL string) (map[string]any, error) {
	if rawURL == "" {
		return map[string]any{}, nil
	}
	reader, err := resolver.Resolve(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("resolve vars %s: %w", rawURL, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read vars: %w", err)
	}

	var vars map[string]any
	if err := json.Unmarshal(data, &vars); err != nil {
		return nil, fmt.Errorf("parse vars json: %w", err)
	}
	return vars, nil
}

// parseLinkReplacements convertit []string "prefix::replacement" en map[string]string.
func parseLinkReplacements(raw []string) (map[string]string, error) {
	result := make(map[string]string, len(raw))
	for _, r := range raw {
		parts := strings.SplitN(r, "::", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("format de remplacement invalide : %q (attendu prefix::remplacement)", r)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

// parsePDFTimeout parse une durée avec fallback sur le défaut.
func parsePDFTimeout(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return amatlrender.DefaultPDFTimeout
	}
	return d
}

// buildMiddlewares construit la liste de middlewares communs (template + HTML).
func buildMiddlewares(ctx context.Context, cfg *renderConfig, sourcePath resolver.Path) ([]pipeline.Middleware, error) {
	layoutVars, err := loadJSONVars(ctx, cfg.HTMLLayoutVarsURL)
	if err != nil {
		return nil, fmt.Errorf("layout vars: %w", err)
	}

	linkReplacements, err := parseLinkReplacements(cfg.LinkReplacements)
	if err != nil {
		return nil, fmt.Errorf("link replacements: %w", err)
	}

	var middlewares []pipeline.Middleware

	if cfg.VarsURL != "" || cfg.TemplateLeftDelimiter != "" || cfg.TemplateRightDelimiter != "" {
		vars, err := loadJSONVars(ctx, cfg.VarsURL)
		if err != nil {
			return nil, fmt.Errorf("template vars: %w", err)
		}
		middlewares = append(middlewares, amatlrender.TemplateMiddleware(
			amatlrender.WithVars(vars),
			amatlrender.WithDelimiters(cfg.TemplateLeftDelimiter, cfg.TemplateRightDelimiter),
		))
	}

	middlewares = append(middlewares, amatlrender.HTMLMiddleware(
		amatlrender.WithMarkdownTransformerOptions(
			amatlrender.WithSourcePath(sourcePath),
			amatlrender.WithLinkReplacements(linkReplacements),
		),
		amatlrender.WithLayoutURL(cfg.HTMLLayout),
		amatlrender.WithLayoutVars(layoutVars),
	))

	return middlewares, nil
}

// RenderPDF transforme du Markdown en PDF via le pipeline amatl (HTML + chromedp).
func RenderPDF(ctx context.Context, content []byte, filePath, workspaceRoot, configURL, configUsername, configPassword string) ([]byte, error) {
	sourceDir := filepath.Dir(filePath)
	if sourceDir == "." || sourceDir == "" {
		sourceDir = workspaceRoot
	} else if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(workspaceRoot, sourceDir)
	}

	cfg := defaultRenderConfig()
	if configURL != "" {
		var err error
		cfg, err = loadConfig(ctx, configURL, configUsername, configPassword)
		if err != nil {
			return nil, err
		}
	}

	middlewares, err := buildMiddlewares(ctx, cfg, resolver.Path(sourceDir))
	if err != nil {
		return nil, err
	}

	middlewares = append(middlewares, amatlrender.PDFMiddleware(
		amatlrender.WithMarginTop(cfg.PDFMarginTop),
		amatlrender.WithMarginLeft(cfg.PDFMarginLeft),
		amatlrender.WithMarginRight(cfg.PDFMarginRight),
		amatlrender.WithMarginBottom(cfg.PDFMarginBottom),
		amatlrender.WithScale(cfg.PDFScale),
		amatlrender.WithTimeout(parsePDFTimeout(cfg.PDFTimeout)),
		amatlrender.WithBackground(cfg.PDFBackground),
		amatlrender.WithExecPath(cfg.PDFExecPath),
		amatlrender.WithDisplayFooterHeader(cfg.PDFDisplayHeaderFooter),
		amatlrender.WithHeaderTemplate(cfg.PDFHeaderTemplate),
		amatlrender.WithFooterTemplate(cfg.PDFFooterTemplate),
		amatlrender.WithNoSandbox(true), // toujours actif dans amatled (Linux headless)
	))

	t := pipeline.Pipeline(middlewares...)
	payload := pipeline.NewPayload(content)
	if err := t.Transform(ctx, payload); err != nil {
		return nil, err
	}
	return payload.GetData(), nil
}
