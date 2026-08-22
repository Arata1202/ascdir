import fs from "node:fs/promises";
import path from "node:path";

export type Doc = {
  slug: string;
  title: string;
  description: string;
  body: string;
  category: DocCategory;
};

export type DocCategory = "Start here" | "App metadata" | "Distribution" | "Release" | "Reference";

const docMeta: Record<string, { category: DocCategory; description: string }> = {
  "getting-started": {
    category: "Start here",
    description:
      "Install ascdir, connect an existing app, preview a dry run, and apply reviewed changes.",
  },
  scope: {
    category: "Start here",
    description:
      "Understand what ascdir manages and how it complements your existing build pipeline.",
  },
  metadata: {
    category: "App metadata",
    description: "Manage localized text and long-form product-page content in reviewable files.",
  },
  screenshots: {
    category: "App metadata",
    description: "Keep App Store screenshots in a predictable, project-relative directory.",
  },
  "app-previews": {
    category: "App metadata",
    description: "Manage App Preview videos from a project-relative directory.",
  },
  accessibility: {
    category: "App metadata",
    description:
      "Configure Accessibility Nutrition Labels for every supported Apple device family.",
  },
  "age-rating": {
    category: "App metadata",
    description: "Map age-rating declarations directly to App Store Connect.",
  },
  pricing: {
    category: "App metadata",
    description: "Set exact App Store price points without locale-dependent currency formatting.",
  },
  availability: {
    category: "App metadata",
    description:
      "Control storefront availability and preorder settings from explicit configuration.",
  },
  "license-agreement": {
    category: "App metadata",
    description: "Add a custom end-user license agreement only when your app needs one.",
  },
  "testflight-distribution": {
    category: "Distribution",
    description:
      "Distribute processed builds to existing TestFlight groups and request review when needed.",
  },
  release: {
    category: "Release",
    description: "Submit a processed build for App Review and control its release after approval.",
  },
  troubleshooting: {
    category: "Reference",
    description:
      "Resolve configuration paths, credentials, validation failures, and common command errors.",
  },
};

export const docCategories: DocCategory[] = [
  "Start here",
  "App metadata",
  "Distribution",
  "Release",
  "Reference",
];

const docsDirectory = path.resolve(process.cwd(), "../docs");

function titleFromSlug(slug: string) {
  return slug
    .split("-")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function stripTitle(markdown: string) {
  return markdown.replace(/^#\s+.+\r?\n+/, "").trim();
}

function descriptionFromMarkdown(markdown: string) {
  const paragraph = stripTitle(markdown)
    .split(/\r?\n\s*\r?\n/)
    .find((block) => !block.startsWith("#") && !block.startsWith("```") && !block.startsWith("- "));

  return (paragraph ?? "Official ascdir documentation.")
    .replace(/[`*_\[\]<>]/g, "")
    .replace(/\((https?:\/\/[^)]+)\)/g, "")
    .replace(/\s+/g, " ")
    .slice(0, 160);
}

export async function getDocSlugs() {
  const files = await fs.readdir(docsDirectory);
  return files
    .filter((file) => file.endsWith(".md"))
    .map((file) => file.slice(0, -3))
    .sort();
}

export async function getDoc(slug: string): Promise<Doc | null> {
  if (!/^[a-z0-9-]+$/.test(slug)) return null;

  try {
    const markdown = await fs.readFile(path.join(docsDirectory, `${slug}.md`), "utf8");
    const heading = markdown.match(/^#\s+(.+)$/m)?.[1]?.trim();
    return {
      slug,
      title: heading ?? titleFromSlug(slug),
      description: docMeta[slug]?.description ?? descriptionFromMarkdown(markdown),
      body: stripTitle(markdown),
      category: docMeta[slug]?.category ?? "Reference",
    };
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return null;
    throw error;
  }
}

export async function getDocs() {
  const docs = await Promise.all((await getDocSlugs()).map(getDoc));
  return docs
    .filter((doc): doc is Doc => doc !== null)
    .sort((a, b) => {
      const categoryOrder = docCategories.indexOf(a.category) - docCategories.indexOf(b.category);
      if (categoryOrder !== 0) return categoryOrder;
      return Object.keys(docMeta).indexOf(a.slug) - Object.keys(docMeta).indexOf(b.slug);
    });
}
