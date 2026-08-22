import fs from "node:fs/promises";
import path from "node:path";

export type Doc = {
  slug: string;
  title: string;
  description: string;
  body: string;
};

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
      description: descriptionFromMarkdown(markdown),
      body: stripTitle(markdown),
    };
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return null;
    throw error;
  }
}

export async function getDocs() {
  const docs = await Promise.all((await getDocSlugs()).map(getDoc));
  return docs.filter((doc): doc is Doc => doc !== null);
}
