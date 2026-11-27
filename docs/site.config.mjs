// Steering config for remote-astro-theme: builds docs/content with the theme below.
export default {
  //theme: "mindovermachine-dev/mom-doc-theme@main",
  theme: "mindovermachine-dev/mom-doc-theme@main",
  // Raw Astro config, merged over the theme's astro.config.mjs.
  astro: {
    site: "https://docs.mindovermachine.dev",
    base: "/gh-mkissue",
    server: { host: true },
  },
  title: "GitHub CLI mkissue",
  // null (not undefined) so the theme's default "/da/" redirect is overridden.
  rootRedirect: null,
  defaultLocale: "root",
  locales: {
    root: { label: "English", lang: "en" },
  },
  social: [
    {
      icon: "github",
      label: "GitHub",
      href: "https://github.com/lakruzz",
    },
  ],
  // One group per folder under docs/content; order pages with `sidebar.order` frontmatter.
  sidebar: [
    {
      label: "Overview",
      items: [{ autogenerate: { directory: "overview" } }],
    },
    {
      label: "Subcommands",
      items: [{ autogenerate: { directory: "commands" } }],
    },
  ],
};
