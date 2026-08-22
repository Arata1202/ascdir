import type { MetadataRoute } from "next";
import { absoluteUrl, isPreviewDeployment, site } from "@/src/lib/site";

export const dynamic = "force-static";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: isPreviewDeployment ? { userAgent: "*", disallow: "/" } : { userAgent: "*", allow: "/" },
    sitemap: absoluteUrl("/sitemap.xml"),
    host: site.url,
  };
}
