# Lovoria — Brand & Landing Page Design System

## 1. Brand Direction

Lovoria menggunakan visual identity yang:
- Romantic
- Elegant
- Modern
- Timeless
- Premium
- Memorable

Arah visual harus terasa seperti **premium wedding brand**, bukan SaaS biasa yang kebetulan menyediakan wedding invitation.

**Brand:** Lovoria

**Tagline:**
> Your Love. Your Story. Your Forever.

## 2. Logo

### Primary Logo
**LV Monogram + LOVORIA wordmark**

Digunakan pada landing page, website header, footer, dan marketing materials.

### Logo Mark
**LV Monogram**

Digunakan untuk favicon, app icon, mobile navigation, social media profile, dan small UI surfaces.

### Wordmark
**LOVORIA**

Digunakan ketika logo mark tidak diperlukan atau ruang horizontal lebih sesuai.

### Logo Color
Primary logo menggunakan **Dusty Plum `#6B4E71`**. Untuk background gelap, gunakan versi light/white.

## 3. Typography

### Heading / Display
**Playfair Display**

Untuk hero heading, section heading, couple names, wedding names, dan large editorial statements.

Recommended weights:
- 400 Regular
- 500 Medium

### Body / UI
**Inter**

Untuk paragraph, navigation, button, form, labels, dashboard, dan UI elements.

Recommended weights:
- 400 Regular
- 500 Medium
- 600 Semibold

### Typography Rule

```text
Playfair Display → emotional / editorial content
Inter           → functional / UI content
```

## 4. Color System

| Token | Hex | Usage |
|---|---|---|
| Primary | `#6B4E71` | Brand, CTA, links |
| Deep | `#332936` | Headings, dark sections |
| Background | `#FAF7F5` | Main page background |
| Accent | `#C9A88A` | Champagne accent |
| Text | `#292529` | Primary body text |
| Muted | `#6B666B` | Secondary text |
| White | `#FFFFFF` | Cards and contrast |
| Border | `#E8DFD9` | Subtle borders |

### Color Token Naming

```text
lovoria-primary    #6B4E71
lovoria-deep       #332936
lovoria-bg         #FAF7F5
lovoria-accent     #C9A88A
lovoria-text       #292529
lovoria-muted      #6B666B
lovoria-white      #FFFFFF
lovoria-border     #E8DFD9
```

## 5. Color Usage

### Primary — Dusty Plum
`#6B4E71`

Gunakan untuk primary CTA, active navigation, links, brand elements, dan important interactive elements.

### Deep Plum
`#332936`

Gunakan untuk large headings, dark sections, footer, dan high-contrast areas.

### Warm Ivory
`#FAF7F5`

Gunakan sebagai main background dan overall page canvas.

### Champagne
`#C9A88A`

Gunakan sebagai decorative accent, highlights, borders, icons, ornaments, dan subtle emphasis.

Jangan menjadikan champagne sebagai warna utama CTA.

### Charcoal
`#292529`

Gunakan sebagai body text dan heading alternatif.

### Muted
`#6B666B`

Gunakan untuk secondary text, descriptions, metadata, dan helper text.

## 6. Optional Gradient

Gradient bersifat optional dan tidak boleh mendominasi visual.

```css
background: linear-gradient(
  135deg,
  #6B4E71 0%,
  #8A687F 45%,
  #C9A88A 100%
);
```

Gunakan terutama untuk hero decoration, CTA highlight, atau decorative background.

## 7. Landing Page Visual Direction

```text
Warm Ivory Background
        ↓
Large Editorial Typography
        ↓
Dusty Plum CTA
        ↓
Champagne Decorative Accent
        ↓
Large Romantic Photography
        ↓
Thin Borders
        ↓
Generous Whitespace
        ↓
Clean Inter UI
```

Gunakan:
- Warm ivory background
- Large typography
- Soft photography
- Elegant serif headings
- Minimal line icons
- Thin borders
- Rounded corners secukupnya
- Generous whitespace
- Subtle decorative elements
- Soft shadows

Hindari:
- Terlalu banyak gradient
- Warna pink terang
- Saturated red
- Excessive rounded cards
- Terlalu banyak animation
- Visual yang terlalu playful
- Tampilan seperti SaaS dashboard untuk landing page

## 8. Button Style

### Primary Button
```text
Background: #6B4E71
Text: #FFFFFF
```

Contoh:
`Get Started →`

### Secondary Button
```text
Background: transparent
Border: #6B4E71
Text: #6B4E71
```

Contoh:
`Learn More →`

### Text Link
```text
Color: #6B4E71
```

Contoh:
`View Story →`

## 9. Navigation

Recommended:
```text
Lovoria

Home
Features
How It Works
Pricing
About

Get Started
```

Style:
- Inter
- 14–16px
- Medium
- Minimal
- No heavy borders

Active navigation menggunakan `#6B4E71` dengan underline atau subtle indicator.

## 10. Hero Section

Hero harus langsung menyampaikan positioning Lovoria.

Recommended:

```text
THE DIGITAL WEDDING EXPERIENCE

Your Love.
Your Story.
Your Forever.

Create a beautiful wedding website,
invite your loved ones,
and keep your special moments remembered.

[ Get Started ]    [ Learn More ]
```

Heading menggunakan Playfair Display, body dan CTA menggunakan Inter.

## 11. Photography Direction

Photography harus terasa:
- Romantic
- Warm
- Natural
- Cinematic
- Intimate
- Elegant

Recommended:
- Warm sunlight
- Soft shadows
- Natural landscape
- Flowers
- Wedding details
- Couple moments
- Film-like photography
- Muted colors

Hindari overly saturated photos, generic stock-photo feeling, hard flash, dan excessive filters.

## 12. Decorative Elements

Recommended:
- Thin lines
- Small stars/sparkles
- Minimal floral ornaments
- Curved line accents
- Subtle champagne details
- Small monogram marks

> Enhance the story, not compete with the story.

## 13. Icon Style

Gunakan **minimal line icons** dengan thin stroke, rounded line caps, minimal detail, dan konsisten.

Contoh:
```text
♡  Calendar  Location  Gallery  Guestbook  Gift  People  Sparkle
```

Primary icon: `#6B4E71`  
Accent: `#C9A88A`

## 14. Border & Radius

### Border
Gunakan `#E8DFD9` atau opacity rendah.

### Border Radius
```text
Buttons: 8–12px
Cards: 12–16px
Images: 12–16px
Large containers: 16–24px
```

Hindari `rounded-full` untuk hampir semua component.

## 15. Shadow

Gunakan shadow sangat subtle:

```css
box-shadow: 0 8px 30px rgba(51, 41, 54, 0.08);
```

## 16. Responsive Design

Lovoria harus mobile-first karena mayoritas guest kemungkinan membuka invitation melalui smartphone.

```text
Mobile
  ↓
Tablet
  ↓
Desktop
```

Public wedding website harus optimal untuk mobile browser, WhatsApp link, social sharing, dan average mobile connection.

## 17. Design Personality

Lima kata:

> **Romantic. Elegant. Modern. Timeless. Personal.**

Jika desain terlihat **Cute + colorful + generic wedding template**, maka keluar dari brand direction.

Jika terlihat **Editorial + warm + intimate + elegant**, maka berada di arah yang benar.

## 18. Brand Design Rule

> **Let the couple's story be the hero.**

Lovoria menyediakan framework dan visual system. Yang menjadi pusat perhatian adalah couple, love story, wedding, dan memories.

## 19. Design Tokens — Initial

```css
:root {
  --lovoria-primary: #6B4E71;
  --lovoria-deep: #332936;
  --lovoria-bg: #FAF7F5;
  --lovoria-accent: #C9A88A;
  --lovoria-text: #292529;
  --lovoria-muted: #6B666B;
  --lovoria-white: #FFFFFF;
  --lovoria-border: #E8DFD9;
}
```

## 20. Tailwind Concept

Gunakan semantic naming:

```text
lovoria-primary
lovoria-deep
lovoria-bg
lovoria-accent
lovoria-text
lovoria-muted
lovoria-white
lovoria-border
```

Contoh:

```html
<button class="bg-lovoria-primary text-white">
  Get Started
</button>
```

```html
<section class="bg-lovoria-bg text-lovoria-text">
  ...
</section>
```

## 21. Final Brand Direction

**Logo:** LV Monogram + LOVORIA

**Heading:** Playfair Display

**Body / UI:** Inter

**Primary:** #6B4E71 — Dusty Plum

**Deep:** #332936 — Deep Plum

**Background:** #FAF7F5 — Warm Ivory

**Accent:** #C9A88A — Champagne

**Text:** #292529 — Charcoal

**Overall Style:**
> **Romantic × Elegant × Modern × Timeless**

**Brand Principle:**
> **Your Love. Your Story. Your Forever.**

**Design Principle:**
> **Let the couple's story be the hero.**
