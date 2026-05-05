// Package main generates docs/assets/api-{dark,light}.svg from the OpenAPI
// spec at internal/api/docs/swagger.json. The card is a single SVG sized for
// embedding in docs/API.md as the auto-generated apicard block.
//
// Run via `go run ./cmd/genapicard`, `make docs`, or `go generate ./...`.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ── Canvas ───────────────────────────────────────────────────────────────────

const (
	canvasW   = 1000
	marginX   = 20
	marginTop = 20
	marginBot = 20

	cardPadX = 24
	cardPadY = 18

	headerH    = 54
	dividerGap = 8
	rowH       = 40
	rowGap     = 2

	badgeW = 60
	badgeH = 22
	badgeR = 4
	cardR  = 8

	pathColW   = 220
	summaryX   = marginX + cardPadX + badgeW + 14 + pathColW + 12
	pathStartX = marginX + cardPadX + badgeW + 14
	badgeX     = marginX + cardPadX
)

// ── Typography ───────────────────────────────────────────────────────────────

const (
	titleSize   = 16
	chipSize    = 11
	pathSize    = 12
	summarySize = 12
	badgeSize   = 11

	fontMono = "ui-monospace,'Cascadia Code','Source Code Pro',Menlo,Consolas,monospace"
	fontSans = "-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif"
)

// ── Theme ────────────────────────────────────────────────────────────────────

// Theme holds every color the card uses. Colors are GitHub Primer palette
// values so the card looks at home in either GH appearance mode.
type Theme struct {
	name         string
	bg           string
	cardBg       string
	cardBorder   string
	titleColor   string
	pathColor    string
	summaryColor string
	dividerColor string
	chipBg       string
	chipText     string
	rowStripeBg  string
	// per-method badge colors
	getColor    string
	getBg       string
	postColor   string
	postBg      string
	putColor    string
	putBg       string
	deleteColor string
	deleteBg    string
}

var dark = Theme{
	name:         "dark",
	bg:           "#0d1117",
	cardBg:       "#161b22",
	cardBorder:   "#30363d",
	titleColor:   "#e6edf3",
	pathColor:    "#79c0ff",
	summaryColor: "#8b949e",
	dividerColor: "#21262d",
	chipBg:       "#21262d",
	chipText:     "#8b949e",
	rowStripeBg:  "#1c2128",
	getColor:     "#79c0ff",
	getBg:        "#1f3a5c",
	postColor:    "#56d364",
	postBg:       "#1a3e2a",
	putColor:     "#d29922",
	putBg:        "#3d2e0a",
	deleteColor:  "#f85149",
	deleteBg:     "#3d1414",
}

var light = Theme{
	name:         "light",
	bg:           "#ffffff",
	cardBg:       "#f6f8fa",
	cardBorder:   "#d0d7de",
	titleColor:   "#1f2328",
	pathColor:    "#0969da",
	summaryColor: "#57606a",
	dividerColor: "#d0d7de",
	chipBg:       "#eaeef2",
	chipText:     "#57606a",
	rowStripeBg:  "#f0f2f5",
	getColor:     "#0550ae",
	getBg:        "#ddf4ff",
	postColor:    "#1a7f37",
	postBg:       "#dafbe1",
	putColor:     "#9a6700",
	putBg:        "#fff8c5",
	deleteColor:  "#cf222e",
	deleteBg:     "#ffebe9",
}

// ── Spec parsing ─────────────────────────────────────────────────────────────

type endpoint struct {
	method  string
	path    string
	summary string
}

var methodOrder = map[string]int{"get": 0, "post": 1, "put": 2, "patch": 3, "delete": 4}

type spec struct {
	Info struct {
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]map[string]struct {
		Summary string `json:"summary"`
	} `json:"paths"`
}

// parseSpec reads and decodes the swag-generated swagger.json, returning the
// flattened endpoint list and metadata for the card header.
func parseSpec(path string) (eps []endpoint, title, version, server string, err error) {
	raw, rerr := os.ReadFile(path)
	if rerr != nil {
		return nil, "", "", "", rerr
	}
	var s spec
	if jerr := json.Unmarshal(raw, &s); jerr != nil {
		return nil, "", "", "", fmt.Errorf("decode swagger.json: %w", jerr)
	}
	title = s.Info.Title
	version = s.Info.Version
	if len(s.Servers) > 0 {
		server = s.Servers[0].URL
	}

	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		methods := s.Paths[p]
		mkeys := make([]string, 0, len(methods))
		for m := range methods {
			mkeys = append(mkeys, m)
		}
		sort.Slice(mkeys, func(i, j int) bool { return methodOrder[mkeys[i]] < methodOrder[mkeys[j]] })
		for _, m := range mkeys {
			eps = append(eps, endpoint{
				method:  strings.ToUpper(m),
				path:    p,
				summary: methods[m].Summary,
			})
		}
	}
	return eps, title, version, server, nil
}

// ── SVG generation ───────────────────────────────────────────────────────────

func render(eps []endpoint, title, version, server string, th Theme) string {
	rowsH := len(eps)*rowH + max(0, len(eps)-1)*rowGap
	cardH := cardPadY + headerH + dividerGap + rowsH + cardPadY
	canvasH := marginTop + cardH + marginBot
	cardX := float64(marginX)
	cardY := float64(marginTop)
	cardW := float64(canvasW - 2*marginX)

	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		canvasW, canvasH, canvasW, canvasH)

	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, canvasW, canvasH, th.bg)
	fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="%d" fill="%s" stroke="%s" stroke-width="1"/>`,
		cardX, cardY, cardW, float64(cardH), cardR, th.cardBg, th.cardBorder)

	// ── Header ────────────────────────────────────────────────────────────────
	headerBaseY := float64(marginTop+cardPadY) + float64(headerH)/2 + 1

	fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" font-family=%q font-size="%d" font-weight="600" fill="%s">%s</text>`,
		cardX+float64(cardPadX), headerBaseY+2, fontSans, titleSize, th.titleColor, xmlEsc(title))

	titleW := float64(len(title)*titleSize) * 0.58
	vChipX := cardX + float64(cardPadX) + titleW + 10
	vChipW := float64(len(version)*chipSize)*0.72 + 14
	vChipY := headerBaseY - 10
	fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="16" rx="8" fill="%s"/>`,
		vChipX, vChipY, vChipW, th.chipBg)
	fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-family=%q font-size="%d" fill="%s" text-anchor="middle">%s</text>`,
		vChipX+vChipW/2, vChipY+11.5, fontMono, chipSize, th.chipText, xmlEsc(version))

	if server != "" {
		sChipX := vChipX + vChipW + 8
		sChipW := float64(len(server)*chipSize)*0.62 + 14
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="16" rx="8" fill="%s"/>`,
			sChipX, vChipY, sChipW, th.chipBg)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-family=%q font-size="%d" fill="%s" text-anchor="middle">%s</text>`,
			sChipX+sChipW/2, vChipY+11.5, fontMono, chipSize, th.chipText, xmlEsc(server))
	}

	divY := float64(marginTop+cardPadY+headerH) - 2
	fmt.Fprintf(&b, `<line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="1"/>`,
		cardX+4, divY, cardX+cardW-4, divY, th.dividerColor)

	// ── Endpoint rows ────────────────────────────────────────────────────────
	rowsStartY := float64(marginTop + cardPadY + headerH + dividerGap)
	for i, ep := range eps {
		ry := rowsStartY + float64(i)*(float64(rowH)+float64(rowGap))

		if i%2 == 1 {
			fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%d" fill="%s"/>`,
				cardX+2, ry, cardW-4, rowH, th.rowStripeBg)
		}

		badgeBg, badgeColor := methodColors(ep.method, th)
		bY := ry + float64(rowH-badgeH)/2
		fmt.Fprintf(&b, `<rect x="%d" y="%.0f" width="%d" height="%d" rx="%d" fill="%s"/>`,
			badgeX, bY, badgeW, badgeH, badgeR, badgeBg)
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" font-family=%q font-size="%d" font-weight="600" fill="%s" text-anchor="middle">%s</text>`,
			float64(badgeX)+float64(badgeW)/2, bY+float64(badgeH)/2+float64(badgeSize)/2-1.5,
			fontMono, badgeSize, badgeColor, ep.method)

		textMidY := ry + float64(rowH)/2 + float64(pathSize)/2 - 1.5
		fmt.Fprintf(&b, `<text x="%d" y="%.0f" font-family=%q font-size="%d" fill="%s">%s</text>`,
			pathStartX, textMidY, fontMono, pathSize, th.pathColor, xmlEsc(ep.path))

		if ep.summary != "" {
			fmt.Fprintf(&b, `<text x="%d" y="%.0f" font-family=%q font-size="%d" fill="%s">%s</text>`,
				int(summaryX), textMidY, fontSans, summarySize, th.summaryColor,
				xmlEsc(truncate(ep.summary, 58)))
		}
	}

	b.WriteString(`</svg>`)
	return b.String()
}

// methodColors returns (background, foreground) for the method badge.
func methodColors(method string, th Theme) (string, string) {
	switch method {
	case "POST":
		return th.postBg, th.postColor
	case "PUT", "PATCH":
		return th.putBg, th.putColor
	case "DELETE":
		return th.deleteBg, th.deleteColor
	default:
		return th.getBg, th.getColor
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func xmlEsc(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// mustRoot returns the repo root by walking up to the nearest go.mod, so the
// tool can be invoked from any cwd (repo root, cmd/, or via go generate).
func mustRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "genapicard: getwd: %v\n", err)
		os.Exit(1)
	}
	for dir := cwd; dir != "/" && dir != ""; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
	}
	fmt.Fprintf(os.Stderr, "genapicard: could not find go.mod walking up from %s\n", cwd)
	os.Exit(1)
	return ""
}

// ── Main ─────────────────────────────────────────────────────────────────────

func main() {
	root := mustRoot()
	specPath := filepath.Join(root, "internal", "api", "docs", "swagger.json")
	outDir := filepath.Join(root, "docs", "assets")

	eps, title, version, server, err := parseSpec(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genapicard: parse %s: %v\n", specPath, err)
		os.Exit(1)
	}
	if len(eps) == 0 {
		fmt.Fprintf(os.Stderr, "genapicard: no endpoints found in %s\n", specPath)
		os.Exit(1)
	}
	if title == "" {
		title = "API"
	}
	if version == "" {
		version = "v1"
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "genapicard: mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	for _, th := range []Theme{dark, light} {
		svg := render(eps, title, version, server, th)
		out := filepath.Join(outDir, "api-"+th.name+".svg")
		if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "genapicard: write %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Printf("  wrote %s  (%d endpoints)\n", filepath.Join("docs", "assets", "api-"+th.name+".svg"), len(eps))
	}
}
