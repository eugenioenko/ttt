---
title: Extra Themes
description: Browse and install 78 extra themes for the TTT terminal text editor.
---

Beyond the built-in themes, there are 78 extra themes available covering a wide range of aesthetics, from art-inspired palettes and city vibes to retro computing experiments. These themes are not bundled in the binary but are available in the repository for download.

## Installation

To install a theme, download the `.json` file and place it in `~/.config/ttt/themes/`. TTT will pick it up automatically in the theme picker (open it from the command palette or **Options > Appearance > Theme**).

Each theme below shows a code (e.g. `19-synthwave`). Use it to download directly:

```sh
mkdir -p ~/.config/ttt/themes
curl -Lo ~/.config/ttt/themes/<theme>.json \
  https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/<theme>.json
```

Or grab all 78 at once:

```sh
git clone --depth 1 https://github.com/eugenioenko/ttt.git /tmp/ttt-themes
mkdir -p ~/.config/ttt/themes
cp /tmp/ttt-themes/config/themes/*.json ~/.config/ttt/themes/
rm -rf /tmp/ttt-themes
```

## Theme Gallery

<div class="theme-gallery">
  <div class="theme-card">
    <a href="/themes/01-vermeer.webp" class="theme-preview"><img src="/themes/01-vermeer.webp" alt="Vermeer" loading="lazy" /></a>
    <p>Vermeer</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/01-vermeer.json" class="theme-code-link"><code>01-vermeer</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/02-art-deco.webp" class="theme-preview"><img src="/themes/02-art-deco.webp" alt="Art Deco" loading="lazy" /></a>
    <p>Art Deco</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/02-art-deco.json" class="theme-code-link"><code>02-art-deco</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/03-impressionist.webp" class="theme-preview"><img src="/themes/03-impressionist.webp" alt="Impressionist" loading="lazy" /></a>
    <p>Impressionist</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/03-impressionist.json" class="theme-code-link"><code>03-impressionist</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/04-ukiyo-e.webp" class="theme-preview"><img src="/themes/04-ukiyo-e.webp" alt="Ukiyo-e" loading="lazy" /></a>
    <p>Ukiyo-e</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/04-ukiyo-e.json" class="theme-code-link"><code>04-ukiyo-e</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/05-bauhaus.webp" class="theme-preview"><img src="/themes/05-bauhaus.webp" alt="Bauhaus" loading="lazy" /></a>
    <p>Bauhaus</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/05-bauhaus.json" class="theme-code-link"><code>05-bauhaus</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/06-rothko.webp" class="theme-preview"><img src="/themes/06-rothko.webp" alt="Rothko" loading="lazy" /></a>
    <p>Rothko</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/06-rothko.json" class="theme-code-link"><code>06-rothko</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/07-deep-ocean.webp" class="theme-preview"><img src="/themes/07-deep-ocean.webp" alt="Deep Ocean" loading="lazy" /></a>
    <p>Deep Ocean</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/07-deep-ocean.json" class="theme-code-link"><code>07-deep-ocean</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/08-autumn-forest.webp" class="theme-preview"><img src="/themes/08-autumn-forest.webp" alt="Autumn Forest" loading="lazy" /></a>
    <p>Autumn Forest</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/08-autumn-forest.json" class="theme-code-link"><code>08-autumn-forest</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/09-arctic-tundra.webp" class="theme-preview"><img src="/themes/09-arctic-tundra.webp" alt="Arctic Tundra" loading="lazy" /></a>
    <p>Arctic Tundra</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/09-arctic-tundra.json" class="theme-code-link"><code>09-arctic-tundra</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/10-desert-sandstone.webp" class="theme-preview"><img src="/themes/10-desert-sandstone.webp" alt="Desert Sandstone" loading="lazy" /></a>
    <p>Desert Sandstone</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/10-desert-sandstone.json" class="theme-code-link"><code>10-desert-sandstone</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/11-tropical-rainforest.webp" class="theme-preview"><img src="/themes/11-tropical-rainforest.webp" alt="Tropical Rainforest" loading="lazy" /></a>
    <p>Tropical Rainforest</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/11-tropical-rainforest.json" class="theme-code-link"><code>11-tropical-rainforest</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/12-volcanic.webp" class="theme-preview"><img src="/themes/12-volcanic.webp" alt="Volcanic" loading="lazy" /></a>
    <p>Volcanic</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/12-volcanic.json" class="theme-code-link"><code>12-volcanic</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/13-tokyo-neon.webp" class="theme-preview"><img src="/themes/13-tokyo-neon.webp" alt="Tokyo Neon" loading="lazy" /></a>
    <p>Tokyo Neon</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/13-tokyo-neon.json" class="theme-code-link"><code>13-tokyo-neon</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/14-kyoto-zen.webp" class="theme-preview"><img src="/themes/14-kyoto-zen.webp" alt="Kyoto Zen" loading="lazy" /></a>
    <p>Kyoto Zen</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/14-kyoto-zen.json" class="theme-code-link"><code>14-kyoto-zen</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/15-havana-sunset.webp" class="theme-preview"><img src="/themes/15-havana-sunset.webp" alt="Havana Sunset" loading="lazy" /></a>
    <p>Havana Sunset</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/15-havana-sunset.json" class="theme-code-link"><code>15-havana-sunset</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/16-scandinavian.webp" class="theme-preview"><img src="/themes/16-scandinavian.webp" alt="Scandinavian" loading="lazy" /></a>
    <p>Scandinavian</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/16-scandinavian.json" class="theme-code-link"><code>16-scandinavian</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/17-marrakech.webp" class="theme-preview"><img src="/themes/17-marrakech.webp" alt="Marrakech" loading="lazy" /></a>
    <p>Marrakech</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/17-marrakech.json" class="theme-code-link"><code>17-marrakech</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/18-noir.webp" class="theme-preview"><img src="/themes/18-noir.webp" alt="Noir" loading="lazy" /></a>
    <p>Noir</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/18-noir.json" class="theme-code-link"><code>18-noir</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/19-synthwave.webp" class="theme-preview"><img src="/themes/19-synthwave.webp" alt="Synthwave" loading="lazy" /></a>
    <p>Synthwave</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/19-synthwave.json" class="theme-code-link"><code>19-synthwave</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/20-jazz-club.webp" class="theme-preview"><img src="/themes/20-jazz-club.webp" alt="Jazz Club" loading="lazy" /></a>
    <p>Jazz Club</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/20-jazz-club.json" class="theme-code-link"><code>20-jazz-club</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/21-punk-rock.webp" class="theme-preview"><img src="/themes/21-punk-rock.webp" alt="Punk Rock" loading="lazy" /></a>
    <p>Punk Rock</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/21-punk-rock.json" class="theme-code-link"><code>21-punk-rock</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/22-bossa-nova.webp" class="theme-preview"><img src="/themes/22-bossa-nova.webp" alt="Bossa Nova" loading="lazy" /></a>
    <p>Bossa Nova</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/22-bossa-nova.json" class="theme-code-link"><code>22-bossa-nova</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/23-lofi.webp" class="theme-preview"><img src="/themes/23-lofi.webp" alt="Lofi" loading="lazy" /></a>
    <p>Lofi</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/23-lofi.json" class="theme-code-link"><code>23-lofi</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/24-baroque.webp" class="theme-preview"><img src="/themes/24-baroque.webp" alt="Baroque" loading="lazy" /></a>
    <p>Baroque</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/24-baroque.json" class="theme-code-link"><code>24-baroque</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/25-sepia.webp" class="theme-preview"><img src="/themes/25-sepia.webp" alt="Sepia" loading="lazy" /></a>
    <p>Sepia</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/25-sepia.json" class="theme-code-link"><code>25-sepia</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/26-midnight-blue.webp" class="theme-preview"><img src="/themes/26-midnight-blue.webp" alt="Midnight Blue" loading="lazy" /></a>
    <p>Midnight Blue</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/26-midnight-blue.json" class="theme-code-link"><code>26-midnight-blue</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/27-emerald.webp" class="theme-preview"><img src="/themes/27-emerald.webp" alt="Emerald" loading="lazy" /></a>
    <p>Emerald</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/27-emerald.json" class="theme-code-link"><code>27-emerald</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/28-rose.webp" class="theme-preview"><img src="/themes/28-rose.webp" alt="Rose" loading="lazy" /></a>
    <p>Rose</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/28-rose.json" class="theme-code-link"><code>28-rose</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/29-slate.webp" class="theme-preview"><img src="/themes/29-slate.webp" alt="Slate" loading="lazy" /></a>
    <p>Slate</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/29-slate.json" class="theme-code-link"><code>29-slate</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/30-copper.webp" class="theme-preview"><img src="/themes/30-copper.webp" alt="Copper" loading="lazy" /></a>
    <p>Copper</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/30-copper.json" class="theme-code-link"><code>30-copper</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/31-nebula.webp" class="theme-preview"><img src="/themes/31-nebula.webp" alt="Nebula" loading="lazy" /></a>
    <p>Nebula</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/31-nebula.json" class="theme-code-link"><code>31-nebula</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/32-mars-colony.webp" class="theme-preview"><img src="/themes/32-mars-colony.webp" alt="Mars Colony" loading="lazy" /></a>
    <p>Mars Colony</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/32-mars-colony.json" class="theme-code-link"><code>32-mars-colony</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/33-cherry-blossom.webp" class="theme-preview"><img src="/themes/33-cherry-blossom.webp" alt="Cherry Blossom" loading="lazy" /></a>
    <p>Cherry Blossom</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/33-cherry-blossom.json" class="theme-code-link"><code>33-cherry-blossom</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/34-obsidian.webp" class="theme-preview"><img src="/themes/34-obsidian.webp" alt="Obsidian" loading="lazy" /></a>
    <p>Obsidian</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/34-obsidian.json" class="theme-code-link"><code>34-obsidian</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/35-jade.webp" class="theme-preview"><img src="/themes/35-jade.webp" alt="Jade" loading="lazy" /></a>
    <p>Jade</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/35-jade.json" class="theme-code-link"><code>35-jade</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/36-amethyst.webp" class="theme-preview"><img src="/themes/36-amethyst.webp" alt="Amethyst" loading="lazy" /></a>
    <p>Amethyst</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/36-amethyst.json" class="theme-code-link"><code>36-amethyst</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/37-coral-reef.webp" class="theme-preview"><img src="/themes/37-coral-reef.webp" alt="Coral Reef" loading="lazy" /></a>
    <p>Coral Reef</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/37-coral-reef.json" class="theme-code-link"><code>37-coral-reef</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/38-thunderstorm.webp" class="theme-preview"><img src="/themes/38-thunderstorm.webp" alt="Thunderstorm" loading="lazy" /></a>
    <p>Thunderstorm</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/38-thunderstorm.json" class="theme-code-link"><code>38-thunderstorm</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/39-espresso.webp" class="theme-preview"><img src="/themes/39-espresso.webp" alt="Espresso" loading="lazy" /></a>
    <p>Espresso</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/39-espresso.json" class="theme-code-link"><code>39-espresso</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/40-matcha.webp" class="theme-preview"><img src="/themes/40-matcha.webp" alt="Matcha" loading="lazy" /></a>
    <p>Matcha</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/40-matcha.json" class="theme-code-link"><code>40-matcha</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/41-polaroid.webp" class="theme-preview"><img src="/themes/41-polaroid.webp" alt="Polaroid" loading="lazy" /></a>
    <p>Polaroid</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/41-polaroid.json" class="theme-code-link"><code>41-polaroid</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/42-darkroom.webp" class="theme-preview"><img src="/themes/42-darkroom.webp" alt="Darkroom" loading="lazy" /></a>
    <p>Darkroom</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/42-darkroom.json" class="theme-code-link"><code>42-darkroom</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/43-northern-lights.webp" class="theme-preview"><img src="/themes/43-northern-lights.webp" alt="Northern Lights" loading="lazy" /></a>
    <p>Northern Lights</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/43-northern-lights.json" class="theme-code-link"><code>43-northern-lights</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/44-solar-flare.webp" class="theme-preview"><img src="/themes/44-solar-flare.webp" alt="Solar Flare" loading="lazy" /></a>
    <p>Solar Flare</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/44-solar-flare.json" class="theme-code-link"><code>44-solar-flare</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/45-vaporwave.webp" class="theme-preview"><img src="/themes/45-vaporwave.webp" alt="Vaporwave" loading="lazy" /></a>
    <p>Vaporwave</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/45-vaporwave.json" class="theme-code-link"><code>45-vaporwave</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/46-cyberpunk.webp" class="theme-preview"><img src="/themes/46-cyberpunk.webp" alt="Cyberpunk" loading="lazy" /></a>
    <p>Cyberpunk</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/46-cyberpunk.json" class="theme-code-link"><code>46-cyberpunk</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/47-art-nouveau.webp" class="theme-preview"><img src="/themes/47-art-nouveau.webp" alt="Art Nouveau" loading="lazy" /></a>
    <p>Art Nouveau</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/47-art-nouveau.json" class="theme-code-link"><code>47-art-nouveau</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/48-gotham.webp" class="theme-preview"><img src="/themes/48-gotham.webp" alt="Gotham" loading="lazy" /></a>
    <p>Gotham</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/48-gotham.json" class="theme-code-link"><code>48-gotham</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/49-mumbai-monsoon.webp" class="theme-preview"><img src="/themes/49-mumbai-monsoon.webp" alt="Mumbai Monsoon" loading="lazy" /></a>
    <p>Mumbai Monsoon</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/49-mumbai-monsoon.json" class="theme-code-link"><code>49-mumbai-monsoon</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/50-patagonia.webp" class="theme-preview"><img src="/themes/50-patagonia.webp" alt="Patagonia" loading="lazy" /></a>
    <p>Patagonia</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/50-patagonia.json" class="theme-code-link"><code>50-patagonia</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/51-glacier.webp" class="theme-preview"><img src="/themes/51-glacier.webp" alt="Glacier" loading="lazy" /></a>
    <p>Glacier</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/51-glacier.json" class="theme-code-link"><code>51-glacier</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/52-firefly.webp" class="theme-preview"><img src="/themes/52-firefly.webp" alt="Firefly" loading="lazy" /></a>
    <p>Firefly</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/52-firefly.json" class="theme-code-link"><code>52-firefly</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/53-steampunk.webp" class="theme-preview"><img src="/themes/53-steampunk.webp" alt="Steampunk" loading="lazy" /></a>
    <p>Steampunk</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/53-steampunk.json" class="theme-code-link"><code>53-steampunk</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/54-twilight.webp" class="theme-preview"><img src="/themes/54-twilight.webp" alt="Twilight" loading="lazy" /></a>
    <p>Twilight</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/54-twilight.json" class="theme-code-link"><code>54-twilight</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/55-campfire.webp" class="theme-preview"><img src="/themes/55-campfire.webp" alt="Campfire" loading="lazy" /></a>
    <p>Campfire</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/55-campfire.json" class="theme-code-link"><code>55-campfire</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/56-lavender.webp" class="theme-preview"><img src="/themes/56-lavender.webp" alt="Lavender" loading="lazy" /></a>
    <p>Lavender</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/56-lavender.json" class="theme-code-link"><code>56-lavender</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/57-submarine.webp" class="theme-preview"><img src="/themes/57-submarine.webp" alt="Submarine" loading="lazy" /></a>
    <p>Submarine</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/57-submarine.json" class="theme-code-link"><code>57-submarine</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/58-terracotta.webp" class="theme-preview"><img src="/themes/58-terracotta.webp" alt="Terracotta" loading="lazy" /></a>
    <p>Terracotta</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/58-terracotta.json" class="theme-code-link"><code>58-terracotta</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/59-experiment-double-line.webp" class="theme-preview"><img src="/themes/59-experiment-double-line.webp" alt="Experiment: Double Line" loading="lazy" /></a>
    <p>Experiment: Double Line</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/59-experiment-double-line.json" class="theme-code-link"><code>59-experiment-double-line</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/60-experiment-rounded.webp" class="theme-preview"><img src="/themes/60-experiment-rounded.webp" alt="Experiment: Rounded" loading="lazy" /></a>
    <p>Experiment: Rounded</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/60-experiment-rounded.json" class="theme-code-link"><code>60-experiment-rounded</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/61-experiment-brutalist.webp" class="theme-preview"><img src="/themes/61-experiment-brutalist.webp" alt="Experiment: Brutalist" loading="lazy" /></a>
    <p>Experiment: Brutalist</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/61-experiment-brutalist.json" class="theme-code-link"><code>61-experiment-brutalist</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/62-experiment-bbs.webp" class="theme-preview"><img src="/themes/62-experiment-bbs.webp" alt="Experiment: BBS" loading="lazy" /></a>
    <p>Experiment: BBS</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/62-experiment-bbs.json" class="theme-code-link"><code>62-experiment-bbs</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/63-experiment-dotted.webp" class="theme-preview"><img src="/themes/63-experiment-dotted.webp" alt="Experiment: Dotted" loading="lazy" /></a>
    <p>Experiment: Dotted</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/63-experiment-dotted.json" class="theme-code-link"><code>63-experiment-dotted</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/64-experiment-zen.webp" class="theme-preview"><img src="/themes/64-experiment-zen.webp" alt="Experiment: Zen" loading="lazy" /></a>
    <p>Experiment: Zen</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/64-experiment-zen.json" class="theme-code-link"><code>64-experiment-zen</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/65-experiment-monochrome-blue.webp" class="theme-preview"><img src="/themes/65-experiment-monochrome-blue.webp" alt="Experiment: Monochrome Blue" loading="lazy" /></a>
    <p>Experiment: Monochrome Blue</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/65-experiment-monochrome-blue.json" class="theme-code-link"><code>65-experiment-monochrome-blue</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/66-experiment-phosphor-green.webp" class="theme-preview"><img src="/themes/66-experiment-phosphor-green.webp" alt="Experiment: Phosphor Green" loading="lazy" /></a>
    <p>Experiment: Phosphor Green</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/66-experiment-phosphor-green.json" class="theme-code-link"><code>66-experiment-phosphor-green</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/67-experiment-phosphor-amber.webp" class="theme-preview"><img src="/themes/67-experiment-phosphor-amber.webp" alt="Experiment: Phosphor Amber" loading="lazy" /></a>
    <p>Experiment: Phosphor Amber</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/67-experiment-phosphor-amber.json" class="theme-code-link"><code>67-experiment-phosphor-amber</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/68-experiment-glitch.webp" class="theme-preview"><img src="/themes/68-experiment-glitch.webp" alt="Experiment: Glitch" loading="lazy" /></a>
    <p>Experiment: Glitch</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/68-experiment-glitch.json" class="theme-code-link"><code>68-experiment-glitch</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/69-experiment-blueprint.webp" class="theme-preview"><img src="/themes/69-experiment-blueprint.webp" alt="Experiment: Blueprint" loading="lazy" /></a>
    <p>Experiment: Blueprint</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/69-experiment-blueprint.json" class="theme-code-link"><code>69-experiment-blueprint</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/70-experiment-thermal.webp" class="theme-preview"><img src="/themes/70-experiment-thermal.webp" alt="Experiment: Thermal" loading="lazy" /></a>
    <p>Experiment: Thermal</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/70-experiment-thermal.json" class="theme-code-link"><code>70-experiment-thermal</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/71-experiment-gameboy.webp" class="theme-preview"><img src="/themes/71-experiment-gameboy.webp" alt="Experiment: Gameboy" loading="lazy" /></a>
    <p>Experiment: Gameboy</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/71-experiment-gameboy.json" class="theme-code-link"><code>71-experiment-gameboy</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/72-experiment-c64.webp" class="theme-preview"><img src="/themes/72-experiment-c64.webp" alt="Experiment: C64" loading="lazy" /></a>
    <p>Experiment: C64</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/72-experiment-c64.json" class="theme-code-link"><code>72-experiment-c64</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/73-experiment-redacted.webp" class="theme-preview"><img src="/themes/73-experiment-redacted.webp" alt="Experiment: Redacted" loading="lazy" /></a>
    <p>Experiment: Redacted</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/73-experiment-redacted.json" class="theme-code-link"><code>73-experiment-redacted</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/74-experiment-oscilloscope.webp" class="theme-preview"><img src="/themes/74-experiment-oscilloscope.webp" alt="Experiment: Oscilloscope" loading="lazy" /></a>
    <p>Experiment: Oscilloscope</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/74-experiment-oscilloscope.json" class="theme-code-link"><code>74-experiment-oscilloscope</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/75-experiment-infrared.webp" class="theme-preview"><img src="/themes/75-experiment-infrared.webp" alt="Experiment: Infrared" loading="lazy" /></a>
    <p>Experiment: Infrared</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/75-experiment-infrared.json" class="theme-code-link"><code>75-experiment-infrared</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/76-experiment-woodgrain.webp" class="theme-preview"><img src="/themes/76-experiment-woodgrain.webp" alt="Experiment: Woodgrain" loading="lazy" /></a>
    <p>Experiment: Woodgrain</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/76-experiment-woodgrain.json" class="theme-code-link"><code>76-experiment-woodgrain</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/77-experiment-neon-sign.webp" class="theme-preview"><img src="/themes/77-experiment-neon-sign.webp" alt="Experiment: Neon Sign" loading="lazy" /></a>
    <p>Experiment: Neon Sign</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/77-experiment-neon-sign.json" class="theme-code-link"><code>77-experiment-neon-sign</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
  <div class="theme-card">
    <a href="/themes/78-experiment-acid.webp" class="theme-preview"><img src="/themes/78-experiment-acid.webp" alt="Experiment: Acid" loading="lazy" /></a>
    <p>Experiment: Acid</p>
    <a href="https://raw.githubusercontent.com/eugenioenko/ttt/main/config/themes/78-experiment-acid.json" class="theme-code-link"><code>78-experiment-acid</code></a><button class="theme-copy" title="Copy theme name">&#x2398;</button>
  </div>
</div>

<script>
  (function() {
    var lb = document.createElement('div');
    lb.className = 'theme-lightbox';
    lb.id = 'theme-lightbox';
    lb.innerHTML = '<img src="" alt="" />';
    document.body.appendChild(lb);

    document.addEventListener('click', function(e) {
      var link = e.target.closest('.theme-preview');
      if (link) {
        e.preventDefault();
        lb.querySelector('img').src = link.href;
        lb.querySelector('img').alt = link.querySelector('img').alt;
        lb.classList.add('active');
      }
    });
    lb.addEventListener('click', function() {
      lb.classList.remove('active');
    });
    document.addEventListener('keydown', function(e) {
      if (e.key === 'Escape') lb.classList.remove('active');
    });

    document.addEventListener('click', function(e) {
      var btn = e.target.closest('.theme-copy');
      if (btn) {
        var code = btn.previousElementSibling.querySelector('code').textContent;
        navigator.clipboard.writeText(code);
        btn.textContent = '✓';
        setTimeout(function() { btn.innerHTML = '⎘'; }, 1500);
      }
    });
  })();
</script>
