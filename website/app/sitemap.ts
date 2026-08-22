import type { MetadataRoute } from "next";
import { getDocSlugs } from "@/src/lib/docs";
import { absoluteUrl } from "@/src/lib/site";

export const dynamic = "force-static";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const docs = await getDocSlugs();
  const routes = [
    "/",
    "/docs/",
    "/comparison/fastlane/",
    "/changelog/",
    ...docs.map((slug) => `/docs/${slug}/`),
  ];
  return routes.map((route) => ({
    url: absoluteUrl(route),
    changeFrequency: route === "/changelog/" ? "weekly" : "monthly",
  }));
}
