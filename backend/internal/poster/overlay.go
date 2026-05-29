// Package poster provides image composition for sunset countdown poster overlays.
// It downloads original posters, composites a top banner with an hourglass icon
// and countdown text, and returns the modified image as JPEG bytes.
//
// Two banner styles:
//   - Sunset (leaving): warm amber background with hourglass icon
//   - Saved: emerald green background with shield-check icon
package poster

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"capacitarr/assets/fonts"
	"capacitarr/assets/icons"
)

// parsedFont caches the parsed Noto Sans Bold font. Parsed once on first use.
var (
	parsedFont     *opentype.Font
	parsedFontOnce sync.Once
	parsedFontErr  error
)

// Banner layout constants.
const (
	// BannerHeight is the fraction of poster height used for the top banner.
	BannerHeight = 0.07

	// bannerPaddingFrac is horizontal padding as a fraction of poster width.
	bannerPaddingFrac = 0.03

	// iconSizeFrac is the icon size as a fraction of banner height.
	iconSizeFrac = 0.55

	// iconGapFrac is the gap between icon and text as a fraction of banner height.
	iconGapFrac = 0.25
)

// Banner color palettes (vertical gradient top → bottom, ~95% opaque).
var (
	// Sunset: warm amber (#B45309 → #D97706)
	sunsetTop    = color.NRGBA{R: 180, G: 83, B: 9, A: 242}
	sunsetBottom = color.NRGBA{R: 217, G: 119, B: 6, A: 242}

	// Saved: emerald green (#047857 → #059669)
	savedTop    = color.NRGBA{R: 4, G: 120, B: 87, A: 242}
	savedBottom = color.NRGBA{R: 5, G: 150, B: 105, A: 242}
)

// NOTE: Icons are now pre-rendered Lucide PNGs (white on transparent) rather
// than programmatically drawn shapes. The composeBanner function accepts raw
// PNG bytes and scales them to fit the banner.

// ─── Public API ─────────────────────────────────────────────────────────────

// ComposeOverlay renders a warm amber top banner with a Lucide hourglass icon and
// countdown text. The style parameter controls the text: "countdown" shows exact
// days remaining ("Leaving in 7 days"), "simple" shows only "Leaving soon".
// Returns the composited image as JPEG bytes.
func ComposeOverlay(original []byte, daysRemaining int, style string) ([]byte, error) {
	iconPNG := selectIconSize(icons.Hourglass24, icons.Hourglass48, icons.Hourglass96, original)
	return composeBanner(original, countdownText(daysRemaining, style), sunsetTop, sunsetBottom, iconPNG)
}

// ComposeSavedOverlay renders an emerald green top banner with a Lucide shield-check
// icon and "Saved by popular demand" text. Returns the composited image as JPEG bytes.
func ComposeSavedOverlay(original []byte) ([]byte, error) {
	iconPNG := selectIconSize(icons.ShieldCheck24, icons.ShieldCheck48, icons.ShieldCheck96, original)
	return composeBanner(original, "Saved by popular demand", savedTop, savedBottom, iconPNG)
}

// selectIconSize picks the best pre-rendered icon size based on poster height.
// Uses the 96px icon for large posters, 48px for medium, 24px for small.
func selectIconSize(small, medium, large []byte, posterData []byte) []byte {
	// Quick decode just to get dimensions — don't need full pixel data
	cfg, _, err := image.DecodeConfig(bytes.NewReader(posterData))
	if err != nil {
		return medium // safe fallback
	}
	bannerH := int(math.Round(float64(cfg.Height) * BannerHeight))
	iconTarget := int(math.Round(float64(bannerH) * iconSizeFrac))

	switch {
	case iconTarget >= 72:
		return large // 96px
	case iconTarget >= 36:
		return medium // 48px
	default:
		return small // 24px
	}
}

// ContentHash returns a hex-encoded SHA-256 hash of the image data.
// Used to detect if a poster has been changed by the user since it was cached.
func ContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:16]) // 32 hex chars (128 bits) — sufficient for dedup
}

// ─── Banner Composition ─────────────────────────────────────────────────────

// composeBanner is the shared implementation for both overlay types. It draws a
// colored banner across the top of the poster with an icon on the left, text to
// the right, and a subtle drop shadow below the banner.
func composeBanner(original []byte, text string, topColor, bottomColor color.NRGBA, iconPNG []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(original))
	if err != nil {
		return nil, fmt.Errorf("decode poster image: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w < 10 || h < 10 {
		return nil, fmt.Errorf("poster too small: %dx%d", w, h)
	}

	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, src, bounds.Min, draw.Src)

	// ── Banner dimensions ────────────────────────────────────────────────
	bannerH := int(math.Round(float64(h) * BannerHeight))
	if bannerH < 24 {
		bannerH = 24
	}
	bannerTop := bounds.Min.Y
	bannerBottom := bannerTop + bannerH
	padding := int(math.Round(float64(w) * bannerPaddingFrac))

	// ── Colored gradient fill (top → bottom) ─────────────────────────────
	for y := bannerTop; y < bannerBottom; y++ {
		t := float64(y-bannerTop) / float64(bannerH)
		rc := lerpU8(topColor.R, bottomColor.R, t)
		gc := lerpU8(topColor.G, bottomColor.G, t)
		bc := lerpU8(topColor.B, bottomColor.B, t)
		ac := lerpU8(topColor.A, bottomColor.A, t)
		rowColor := color.NRGBA{R: rc, G: gc, B: bc, A: ac}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			draw.Draw(out, image.Rect(x, y, x+1, y+1),
				&image.Uniform{C: rowColor}, image.Point{}, draw.Over)
		}
	}

	// ── Subtle drop shadow below the banner (2px fade) ───────────────────
	shadowAlphas := [2]uint8{40, 20}
	for i, alpha := range shadowAlphas {
		y := bannerBottom + i
		if y >= bounds.Max.Y {
			break
		}
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			draw.Draw(out, image.Rect(x, y, x+1, y+1),
				&image.Uniform{C: color.NRGBA{A: alpha}}, image.Point{}, draw.Over)
		}
	}

	// ── Measure text ─────────────────────────────────────────────────────
	ft, err := loadFont()
	if err != nil {
		return nil, err
	}

	fontSize := float64(bannerH) * 0.55
	if fontSize < 12 {
		fontSize = 12
	}

	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size: fontSize, DPI: 72, Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create font face: %w", err)
	}
	defer func() { _ = face.Close() }()

	metrics := face.Metrics()
	textWidth := font.MeasureString(face, text).Ceil()

	// ── Layout: [icon gap text] centered in banner ───────────────────────
	iconSize := int(math.Round(float64(bannerH) * iconSizeFrac))
	iconGap := int(math.Round(float64(bannerH) * iconGapFrac))
	contentWidth := iconSize + iconGap + textWidth

	maxWidth := w - 2*padding
	if contentWidth > maxWidth {
		contentWidth = maxWidth
	}

	startX := bounds.Min.X + (w-contentWidth)/2
	iconCX := startX + iconSize/2
	iconCY := bannerTop + bannerH/2

	// ── Draw icon (decode pre-rendered PNG + scale to fit) ───────────────
	iconImg, pngErr := png.Decode(bytes.NewReader(iconPNG))
	if pngErr != nil {
		return nil, fmt.Errorf("decode icon PNG: %w", pngErr)
	}
	// Scale the icon PNG to the target iconSize using high-quality BiLinear
	iconDst := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	draw.BiLinear.Scale(iconDst, iconDst.Bounds(), iconImg, iconImg.Bounds(), draw.Over, nil)

	// Composite the scaled icon centered at (iconCX, iconCY)
	iconRect := image.Rect(
		iconCX-iconSize/2, iconCY-iconSize/2,
		iconCX+iconSize/2, iconCY+iconSize/2,
	)
	draw.Draw(out, iconRect, iconDst, image.Point{}, draw.Over)

	// ── Draw text with drop shadow ───────────────────────────────────────
	textX := startX + iconSize + iconGap
	// font.Drawer positions text by its baseline (Dot.Y), not its top or
	// center. Glyphs extend metrics.Ascent above the baseline and
	// metrics.Descent below it (Descent is reported as a positive value).
	// To vertically center the text's visual midpoint within the banner, the
	// baseline must sit at the banner center plus half the asymmetry between
	// ascent and descent: bannerCenter + (Ascent - Descent)/2. The previous
	// formula added the full text height instead, pushing text below center.
	// See GitHub issue #27.
	bannerCenter := bannerTop + bannerH/2
	baselineOffset := (metrics.Ascent - metrics.Descent).Ceil() / 2
	textY := bannerCenter + baselineOffset

	// Shadow: 1px offset, semi-transparent black
	shadowOff := max(1, int(math.Round(fontSize*0.04)))
	shadowDrw := &font.Drawer{
		Dst:  out,
		Src:  &image.Uniform{C: color.NRGBA{A: 100}},
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(textX + shadowOff), Y: fixed.I(textY + shadowOff)},
	}
	shadowDrw.DrawString(text)

	// Main text: bright white
	textDrw := &font.Drawer{
		Dst:  out,
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(textX), Y: fixed.I(textY)},
	}
	textDrw.DrawString(text)

	// ── Encode ───────────────────────────────────────────────────────────
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 92}); err != nil {
		return nil, fmt.Errorf("encode poster JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// countdownText returns the human-readable countdown string.
// When style is "simple", all values collapse to "Leaving soon".
func countdownText(daysRemaining int, style string) string {
	if style == "simple" {
		return "Leaving soon"
	}
	switch {
	case daysRemaining <= 0:
		return "Last day"
	case daysRemaining == 1:
		return "Leaving tomorrow"
	default:
		return fmt.Sprintf("Leaving in %d days", daysRemaining)
	}
}

// loadFont parses the embedded Noto Sans Bold TTF data. Called once via sync.Once.
func loadFont() (*opentype.Font, error) {
	parsedFontOnce.Do(func() {
		parsedFont, parsedFontErr = opentype.Parse(fonts.NotoSansBold)
		if parsedFontErr != nil {
			parsedFontErr = fmt.Errorf("parse Noto Sans Bold: %w", parsedFontErr)
		}
	})
	return parsedFont, parsedFontErr
}

// lerpU8 linearly interpolates between two uint8 values.
func lerpU8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a)*(1-t) + float64(b)*t))
}
