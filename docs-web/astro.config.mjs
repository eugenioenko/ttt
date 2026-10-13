import { defineConfig, passthroughImageService } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://tttedit.dev',
  image: { service: passthroughImageService() },
  integrations: [
    starlight({
      components: { Header: './src/components/Header.astro' },
      customCss: ['./src/styles/custom.css'],
      title: 'TTT Editor',
      favicon: '/favicon.svg',
      logo: {
        src: './src/assets/logo.svg',
      },
      description: 'TTT Editor — a terminal text editor. A real alternative to VS Code, Zed, and Sublime that runs in your terminal. Single Go binary, zero config.',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/eugenioenko/ttt' },
      ],
      sidebar: [
        { label: 'Demo', link: '/demo/' },
        {
          label: 'Getting Started',
          items: [{ autogenerate: { directory: 'getting-started' } }],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Editor Basics', link: '/guides/editor/' },
            { label: 'File Explorer & Workspaces', link: '/guides/workspaces/' },
            { label: 'Search', link: '/guides/search/' },
            { label: 'Git Integration', link: '/guides/git/' },
            { label: 'Integrated Terminal', link: '/guides/terminal/' },
            { label: 'LSP', link: '/guides/lsp/' },
            { label: 'Themes', link: '/guides/themes/' },
            { label: 'Extra Themes', link: '/guides/extra-themes/' },
            { label: 'Keybindings', link: '/guides/keybindings/' },
            { label: 'Plugins', link: '/guides/plugins/' },
            { label: 'Plugin Authoring', link: '/guides/plugin-authoring/' },
            { label: 'Testing Plugins', link: '/guides/plugin-testing/' },
          ],
        },
        {
          label: 'Reference',
          items: [{ autogenerate: { directory: 'reference' } }],
        },
      ],
    }),
  ],
});
